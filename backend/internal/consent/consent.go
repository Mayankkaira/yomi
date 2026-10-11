package consent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/yomi/yomi/internal/kafka"
	"github.com/yomi/yomi/internal/models"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const requestColumns = `id, citizen_id, institution_id, institution_role, data_scope, purpose, status, access_token_hash, expires_at, created_at, updated_at`

func (r *Repository) CreateRequest(ctx context.Context, citizenID, institutionID string, role models.InstitutionRole, scope []string, purpose string, expiresAt time.Time) (models.ConsentRequest, error) {
	id := uuid.NewString()
	var req models.ConsentRequest
	var tokenHash *string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO consent.consent_requests
			(id, citizen_id, institution_id, institution_role, data_scope, purpose, status, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7)
		RETURNING `+requestColumns,
		id, citizenID, institutionID, string(role), scope, purpose, expiresAt).
		Scan(&req.ID, &req.CitizenID, &req.InstitutionID, &req.InstitutionRole, &req.DataScope, &req.Purpose, &req.Status,
			&tokenHash, &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt)
	if err != nil {
		return models.ConsentRequest{}, fmt.Errorf("insert consent request: %w", err)
	}
	if tokenHash != nil {
		req.AccessTokenHash = *tokenHash
	}
	return req, nil
}

func (r *Repository) RequestByID(ctx context.Context, id string) (models.ConsentRequest, error) {
	var req models.ConsentRequest
	var tokenHash *string
	err := r.pool.QueryRow(ctx, `
		SELECT `+requestColumns+`
		FROM consent.consent_requests WHERE id = $1`, id).
		Scan(&req.ID, &req.CitizenID, &req.InstitutionID, &req.InstitutionRole, &req.DataScope, &req.Purpose, &req.Status,
			&tokenHash, &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ConsentRequest{}, ErrNotFound
	}
	if err != nil {
		return models.ConsentRequest{}, fmt.Errorf("query consent request: %w", err)
	}
	if tokenHash != nil {
		req.AccessTokenHash = *tokenHash
	}
	return req, nil
}

func (r *Repository) PendingByCitizen(ctx context.Context, citizenID string) ([]models.ConsentRequest, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+requestColumns+`
		FROM consent.consent_requests
		WHERE citizen_id = $1 AND status = 'pending'
		ORDER BY created_at DESC`, citizenID)
	if err != nil {
		return nil, fmt.Errorf("query pending requests: %w", err)
	}
	defer rows.Close()
	return scanRequests(rows)
}

func (r *Repository) HistoryByCitizen(ctx context.Context, citizenID string) ([]models.ConsentRequest, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+requestColumns+`
		FROM consent.consent_requests
		WHERE citizen_id = $1
		ORDER BY created_at DESC`, citizenID)
	if err != nil {
		return nil, fmt.Errorf("query consent history: %w", err)
	}
	defer rows.Close()
	return scanRequests(rows)
}

func (r *Repository) SetStatus(ctx context.Context, id string, status models.ConsentStatus, accessTokenHash string) (models.ConsentRequest, error) {
	var req models.ConsentRequest
	var tokenHash *string
	err := r.pool.QueryRow(ctx, `
		UPDATE consent.consent_requests
		SET status = $2, access_token_hash = $3, updated_at = now()
		WHERE id = $1
		RETURNING `+requestColumns,
		id, string(status), accessTokenHash).
		Scan(&req.ID, &req.CitizenID, &req.InstitutionID, &req.InstitutionRole, &req.DataScope, &req.Purpose, &req.Status,
			&tokenHash, &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ConsentRequest{}, ErrNotFound
	}
	if err != nil {
		return models.ConsentRequest{}, fmt.Errorf("update consent request: %w", err)
	}
	if tokenHash != nil {
		req.AccessTokenHash = *tokenHash
	}
	return req, nil
}

func (r *Repository) ApproveWithAccessToken(ctx context.Context, id string, accessToken string) (models.ConsentRequest, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return models.ConsentRequest{}, fmt.Errorf("begin approve tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var req models.ConsentRequest
	var tokenHash *string
	err = tx.QueryRow(ctx, `
		SELECT `+requestColumns+`
		FROM consent.consent_requests WHERE id = $1 FOR UPDATE`, id).
		Scan(&req.ID, &req.CitizenID, &req.InstitutionID, &req.InstitutionRole, &req.DataScope, &req.Purpose, &req.Status,
			&tokenHash, &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ConsentRequest{}, ErrNotFound
	}
	if err != nil {
		return models.ConsentRequest{}, fmt.Errorf("load request for approval: %w", err)
	}
	if req.Status != string(models.StatusPending) {
		return models.ConsentRequest{}, ErrInvalidState
	}
	if time.Now().After(req.ExpiresAt) {
		return models.ConsentRequest{}, ErrExpired
	}

	stored := hashAccessToken(accessToken)
	err = tx.QueryRow(ctx, `
		UPDATE consent.consent_requests
		SET status = $2, access_token_hash = $3, updated_at = now()
		WHERE id = $1
		RETURNING `+requestColumns,
		id, string(models.StatusApproved), stored).
		Scan(&req.ID, &req.CitizenID, &req.InstitutionID, &req.InstitutionRole, &req.DataScope, &req.Purpose, &req.Status,
			&tokenHash, &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt)
	if err != nil {
		return models.ConsentRequest{}, fmt.Errorf("store access token hash: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return models.ConsentRequest{}, fmt.Errorf("commit approve tx: %w", err)
	}
	req.AccessTokenHash = stored
	return req, nil
}

func (r *Repository) ByAccessTokenHash(ctx context.Context, hash string) (models.ConsentRequest, error) {
	if hash == "" {
		return models.ConsentRequest{}, ErrNotFound
	}
	var req models.ConsentRequest
	var tokenHash *string
	err := r.pool.QueryRow(ctx, `
		SELECT `+requestColumns+`
		FROM consent.consent_requests WHERE access_token_hash = $1`, hash).
		Scan(&req.ID, &req.CitizenID, &req.InstitutionID, &req.InstitutionRole, &req.DataScope, &req.Purpose, &req.Status,
			&tokenHash, &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ConsentRequest{}, ErrNotFound
	}
	if err != nil {
		return models.ConsentRequest{}, fmt.Errorf("query consent request by token: %w", err)
	}
	if tokenHash != nil {
		req.AccessTokenHash = *tokenHash
	}
	return req, nil
}

type Service struct {
	repo     *Repository
	producer *kafka.Producer
	rdb      *redis.Client
	tokenTTL time.Duration
}

func NewService(repo *Repository, producer *kafka.Producer, rdb *redis.Client, tokenTTL time.Duration) *Service {
	return &Service{
		repo:     repo,
		producer: producer,
		rdb:      rdb,
		tokenTTL: tokenTTL,
	}
}

type RequestInput struct {
	CitizenID     string
	InstitutionID string
	Role          models.InstitutionRole
	Scope         []string
	Purpose       string
	ExpiresAt     time.Time
}

func (s *Service) Request(ctx context.Context, in RequestInput) (models.ConsentRequest, error) {
	scope, err := NormalizeScope(in.Role, in.Scope)
	if err != nil {
		return models.ConsentRequest{}, err
	}
	req, err := s.repo.CreateRequest(ctx, in.CitizenID, in.InstitutionID, in.Role, scope, in.Purpose, in.ExpiresAt)
	if err != nil {
		return models.ConsentRequest{}, err
	}
	evt := models.ConsentEvent{
		EventID:       uuid.NewString(),
		EventType:     models.EventConsentRequested,
		CitizenID:     in.CitizenID,
		InstitutionID: in.InstitutionID,
		RequestID:     req.ID,
		Timestamp:     time.Now().UTC(),
		Metadata: json.RawMessage(`{"scope":` + scopeJSON(scope) +
			`,"purpose":` + strconv.Quote(in.Purpose) +
			`,"institution_role":` + strconv.Quote(string(in.Role)) +
			`,"expires_at":` + strconv.Quote(in.ExpiresAt.Format(time.RFC3339)) + `}`),
	}
	if err := s.producer.Publish(ctx, evt); err != nil {
		return req, err
	}
	return req, nil
}

func (s *Service) Approve(ctx context.Context, requestID, citizenID string) (models.ConsentRequest, string, error) {
	req, err := s.repo.RequestByID(ctx, requestID)
	if err != nil {
		return models.ConsentRequest{}, "", err
	}
	if req.CitizenID != citizenID {
		return models.ConsentRequest{}, "", ErrNotOwned
	}
	if req.Status != string(models.StatusPending) {
		return models.ConsentRequest{}, "", ErrInvalidState
	}
	if time.Now().After(req.ExpiresAt) {
		return models.ConsentRequest{}, "", ErrExpired
	}
	raw, err := generateAccessToken()
	if err != nil {
		return models.ConsentRequest{}, "", err
	}
	updated, err := s.repo.ApproveWithAccessToken(ctx, requestID, raw)
	if err != nil {
		return models.ConsentRequest{}, "", err
	}
	evt := models.ConsentEvent{
		EventID:       uuid.NewString(),
		EventType:     models.EventConsentApproved,
		CitizenID:     req.CitizenID,
		InstitutionID: req.InstitutionID,
		RequestID:     req.ID,
		Timestamp:     time.Now().UTC(),
		Metadata: json.RawMessage(`{"scope":` + scopeJSON(req.DataScope) +
			`,"institution_role":` + strconv.Quote(string(req.InstitutionRole)) +
			`,"token_fingerprint":` + strconv.Quote(fingerprint(updated.AccessTokenHash)) + `}`),
	}
	if err := s.producer.Publish(ctx, evt); err != nil {
		slog.Error("publish consent_approved event", "error", err, "request_id", req.ID)
		return updated, raw, err
	}
	return updated, raw, nil
}

func (s *Service) Reject(ctx context.Context, requestID, citizenID string) (models.ConsentRequest, error) {
	req, err := s.repo.RequestByID(ctx, requestID)
	if err != nil {
		return models.ConsentRequest{}, err
	}
	if req.CitizenID != citizenID {
		return models.ConsentRequest{}, ErrNotOwned
	}
	if req.Status != string(models.StatusPending) {
		return models.ConsentRequest{}, ErrInvalidState
	}
	updated, err := s.repo.SetStatus(ctx, requestID, models.StatusRejected, "")
	if err != nil {
		return models.ConsentRequest{}, err
	}
	evt := models.ConsentEvent{
		EventID:       uuid.NewString(),
		EventType:     models.EventConsentRejected,
		CitizenID:     req.CitizenID,
		InstitutionID: req.InstitutionID,
		RequestID:     req.ID,
		Timestamp:     time.Now().UTC(),
		Metadata:      json.RawMessage(`{"scope":` + scopeJSON(req.DataScope) + `}`),
	}
	if err := s.producer.Publish(ctx, evt); err != nil {
		return updated, err
	}
	return updated, nil
}

func (s *Service) Revoke(ctx context.Context, requestID, citizenID string) (models.ConsentRequest, error) {
	req, err := s.repo.RequestByID(ctx, requestID)
	if err != nil {
		return models.ConsentRequest{}, err
	}
	if req.CitizenID != citizenID {
		return models.ConsentRequest{}, ErrNotOwned
	}
	if req.Status != string(models.StatusApproved) {
		return models.ConsentRequest{}, ErrInvalidState
	}
	revokedHash := req.AccessTokenHash
	updated, err := s.repo.SetStatus(ctx, requestID, models.StatusRevoked, "")
	if err != nil {
		return models.ConsentRequest{}, err
	}

	if revokedHash != "" {
		if err := s.rdb.Set(ctx, accessTokenKey(revokedHash), revokedMarker, s.tokenTTL).Err(); err != nil {
			slog.Error("write revocation marker", "error", err, "request_id", req.ID)
		}
	}
	evt := models.ConsentEvent{
		EventID:       uuid.NewString(),
		EventType:     models.EventConsentRevoked,
		CitizenID:     req.CitizenID,
		InstitutionID: req.InstitutionID,
		RequestID:     req.ID,
		Timestamp:     time.Now().UTC(),
		Metadata: json.RawMessage(`{"scope":` + scopeJSON(req.DataScope) +
			`,"revoked_token_fingerprint":` + strconv.Quote(fingerprint(revokedHash)) + `}`),
	}
	if err := s.producer.Publish(ctx, evt); err != nil {
		return updated, err
	}
	return updated, nil
}

type Authorization struct {
	RequestID     string
	InstitutionID string
	Scope         []string
	ExpiresAt     time.Time
}

func (s *Service) VerifyAccessToken(ctx context.Context, token string) (Authorization, error) {
	if token == "" {
		return Authorization{}, ErrInvalidToken
	}
	hash := hashAccessToken(token)
	denied, err := s.rdb.Exists(ctx, accessTokenKey(hash)).Result()
	if err == nil && denied > 0 {
		return Authorization{}, ErrTokenRevoked
	}
	req, err := s.repo.ByAccessTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Authorization{}, ErrInvalidToken
		}
		return Authorization{}, err
	}
	switch req.Status {
	case string(models.StatusApproved):
	case string(models.StatusRevoked):
		return Authorization{}, ErrTokenRevoked
	default:
		return Authorization{}, ErrTokenRevoked
	}
	if req.AccessTokenHash != hash {
		return Authorization{}, ErrTokenRevoked
	}
	if time.Now().After(req.ExpiresAt) {
		return Authorization{}, ErrTokenExpired
	}
	return Authorization{
		RequestID:     req.ID,
		InstitutionID: req.InstitutionID,
		Scope:         req.DataScope,
		ExpiresAt:     req.ExpiresAt,
	}, nil
}

func generateAccessToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate access token: %w", err)
	}
	return "yomi_access_" + base64.RawURLEncoding.EncodeToString(b), nil
}

func hashAccessToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func accessTokenKey(hash string) string {
	return "consent-token:" + hash
}

const revokedMarker = "revoked"

func fingerprint(hash string) string {
	if len(hash) < 12 {
		return hash
	}
	return hash[:12]
}

func scopeJSON(scope []string) string {
	b, _ := json.Marshal(scope)
	return string(b)
}

func scanRequests(rows pgx.Rows) ([]models.ConsentRequest, error) {
	out := make([]models.ConsentRequest, 0)
	for rows.Next() {
		var req models.ConsentRequest
		var tokenHash *string
		if err := rows.Scan(&req.ID, &req.CitizenID, &req.InstitutionID, &req.InstitutionRole, &req.DataScope, &req.Purpose,
			&req.Status, &tokenHash, &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan consent request: %w", err)
		}
		if tokenHash != nil {
			req.AccessTokenHash = *tokenHash
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

var (
	ErrNotFound     = errors.New("consent request not found")
	ErrNotOwned     = errors.New("consent request not owned by citizen")
	ErrInvalidState = errors.New("consent request is not in a valid state for this action")
	ErrExpired      = errors.New("consent request has expired")
	ErrInvalidToken = errors.New("invalid access token")
	ErrTokenExpired = errors.New("access token has expired")
	ErrTokenRevoked = errors.New("access token was revoked")
)
