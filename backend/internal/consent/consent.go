// Package consent implements the Consent service: institutions request citizen
// data access, citizens approve/reject/revoke, and every lifecycle event is
// broadcast to Kafka for the audit ledger.
package consent

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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

// Repository owns the consent schema in PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository wires the postgres pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// CreateRequest inserts a new pending consent request.
func (r *Repository) CreateRequest(ctx context.Context, citizenID, institutionID string, scope []string, purpose string, expiresAt time.Time) (models.ConsentRequest, error) {
	id := uuid.NewString()
	var req models.ConsentRequest
	var authToken *string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO consent.consent_requests
			(id, citizen_id, institution_id, data_scope, purpose, status, expires_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', $6)
		RETURNING id, citizen_id, institution_id, data_scope, purpose, status, auth_token, expires_at, created_at, updated_at`,
		id, citizenID, institutionID, scope, purpose, expiresAt).
		Scan(&req.ID, &req.CitizenID, &req.InstitutionID, &req.DataScope, &req.Purpose, &req.Status,
			&authToken, &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt)
	if err != nil {
		return models.ConsentRequest{}, fmt.Errorf("insert consent request: %w", err)
	}
	if authToken != nil {
		req.AuthToken = *authToken
	}
	return req, nil
}

// RequestByID fetches a single consent request.
func (r *Repository) RequestByID(ctx context.Context, id string) (models.ConsentRequest, error) {
	var req models.ConsentRequest
	var authToken *string
	err := r.pool.QueryRow(ctx, `
		SELECT id, citizen_id, institution_id, data_scope, purpose, status, auth_token, expires_at, created_at, updated_at
		FROM consent.consent_requests WHERE id = $1`, id).
		Scan(&req.ID, &req.CitizenID, &req.InstitutionID, &req.DataScope, &req.Purpose, &req.Status,
			&authToken, &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ConsentRequest{}, ErrNotFound
	}
	if err != nil {
		return models.ConsentRequest{}, fmt.Errorf("query consent request: %w", err)
	}
	if authToken != nil {
		req.AuthToken = *authToken
	}
	return req, nil
}

// PendingByCitizen lists pending requests for a citizen, newest first.
func (r *Repository) PendingByCitizen(ctx context.Context, citizenID string) ([]models.ConsentRequest, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, citizen_id, institution_id, data_scope, purpose, status, auth_token, expires_at, created_at, updated_at
		FROM consent.consent_requests
		WHERE citizen_id = $1 AND status = 'pending'
		ORDER BY created_at DESC`, citizenID)
	if err != nil {
		return nil, fmt.Errorf("query pending requests: %w", err)
	}
	defer rows.Close()
	return scanRequests(rows)
}

// HistoryByCitizen lists all requests for a citizen, newest first.
func (r *Repository) HistoryByCitizen(ctx context.Context, citizenID string) ([]models.ConsentRequest, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, citizen_id, institution_id, data_scope, purpose, status, auth_token, expires_at, created_at, updated_at
		FROM consent.consent_requests
		WHERE citizen_id = $1
		ORDER BY created_at DESC`, citizenID)
	if err != nil {
		return nil, fmt.Errorf("query consent history: %w", err)
	}
	defer rows.Close()
	return scanRequests(rows)
}

// SetStatus updates a request's status and, for approvals, stores the
// authorization token. It returns the updated record.
func (r *Repository) SetStatus(ctx context.Context, id string, status models.ConsentStatus, authToken string) (models.ConsentRequest, error) {
	var req models.ConsentRequest
	var storedToken *string
	err := r.pool.QueryRow(ctx, `
		UPDATE consent.consent_requests
		SET status = $2, auth_token = $3, updated_at = now()
		WHERE id = $1
		RETURNING id, citizen_id, institution_id, data_scope, purpose, status, auth_token, expires_at, created_at, updated_at`,
		id, string(status), authToken).
		Scan(&req.ID, &req.CitizenID, &req.InstitutionID, &req.DataScope, &req.Purpose, &req.Status,
			&storedToken, &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ConsentRequest{}, ErrNotFound
	}
	if err != nil {
		return models.ConsentRequest{}, fmt.Errorf("update consent request: %w", err)
	}
	if storedToken != nil {
		req.AuthToken = *storedToken
	}
	return req, nil
}

// Service holds the consent business logic including token signing and events.
type Service struct {
	repo        *Repository
	producer    *kafka.Producer
	rdb         *redis.Client
	tokenSecret []byte
	tokenTTL    time.Duration
}

// NewService wires the repository, event producer and token signing.
func NewService(repo *Repository, producer *kafka.Producer, rdb *redis.Client, tokenSecret string, tokenTTL time.Duration) *Service {
	return &Service{
		repo:        repo,
		producer:    producer,
		rdb:         rdb,
		tokenSecret: []byte(tokenSecret),
		tokenTTL:    tokenTTL,
	}
}

// Request records a new consent request and publishes its event.
func (s *Service) Request(ctx context.Context, citizenID, institutionID string, scope []string, purpose string, expiresAt time.Time) (models.ConsentRequest, error) {
	req, err := s.repo.CreateRequest(ctx, citizenID, institutionID, scope, purpose, expiresAt)
	if err != nil {
		return models.ConsentRequest{}, err
	}
	evt := models.ConsentEvent{
		EventID:       uuid.NewString(),
		EventType:     models.EventConsentRequested,
		CitizenID:     citizenID,
		InstitutionID: institutionID,
		RequestID:     req.ID,
		Timestamp:     time.Now().UTC(),
		Metadata: json.RawMessage(`{"scope":` + scopeJSON(scope) +
			`,"purpose":` + strconv.Quote(purpose) +
			`,"expires_at":` + strconv.Quote(expiresAt.Format(time.RFC3339)) + `}`),
	}
	if err := s.producer.Publish(ctx, evt); err != nil {
		// Event publishing failure must not lose the consent record; surface
		// the error to the caller so it can be retried or alerted.
		return req, err
	}
	return req, nil
}

// Approve marks a request approved and generates a signed authorization token.
func (s *Service) Approve(ctx context.Context, requestID, citizenID string) (models.ConsentRequest, error) {
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
	if time.Now().After(req.ExpiresAt) {
		return models.ConsentRequest{}, ErrExpired
	}
	token, err := s.signAuthorizationToken(req)
	if err != nil {
		return models.ConsentRequest{}, err
	}
	updated, err := s.repo.SetStatus(ctx, requestID, models.StatusApproved, token)
	if err != nil {
		return models.ConsentRequest{}, err
	}
	evt := models.ConsentEvent{
		EventID:       uuid.NewString(),
		EventType:     models.EventConsentApproved,
		CitizenID:     req.CitizenID,
		InstitutionID: req.InstitutionID,
		RequestID:     req.ID,
		Timestamp:     time.Now().UTC(),
		Metadata: json.RawMessage(`{"scope":` + scopeJSON(req.DataScope) +
			`,"auth_token":` + strconv.Quote(base64.RawURLEncoding.EncodeToString([]byte("present"))) + `}`),
	}
	if err := s.producer.Publish(ctx, evt); err != nil {
		slog.Error("publish consent_approved event", "error", err, "request_id", req.ID)
		return updated, err
	}
	return updated, nil
}

// Reject marks a request rejected.
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
		Metadata:      json.RawMessage(`{}`),
	}
	if err := s.producer.Publish(ctx, evt); err != nil {
		return updated, err
	}
	return updated, nil
}

// Revoke invalidates an approved consent and its authorization token.
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
	updated, err := s.repo.SetStatus(ctx, requestID, models.StatusRevoked, "")
	if err != nil {
		return models.ConsentRequest{}, err
	}
	// Drop the cached auth token so it can no longer be used to access data.
	s.rdb.Del(ctx, "consent-token:"+req.AuthToken)
	evt := models.ConsentEvent{
		EventID:       uuid.NewString(),
		EventType:     models.EventConsentRevoked,
		CitizenID:     req.CitizenID,
		InstitutionID: req.InstitutionID,
		RequestID:     req.ID,
		Timestamp:     time.Now().UTC(),
		Metadata:      json.RawMessage(`{}`),
	}
	if err := s.producer.Publish(ctx, evt); err != nil {
		return updated, err
	}
	return updated, nil
}

// signAuthorizationToken produces an HMAC-SHA256 signed token carrying the
// request ID, citizen ID, expiry and scope. The payload is base64url JSON and
// the signature is a base64url HMAC; the two are joined with a dot.
func (s *Service) signAuthorizationToken(req models.ConsentRequest) (string, error) {
	payload := struct {
		RequestID string   `json:"request_id"`
		CitizenID string   `json:"citizen_id"`
		Expiry    string   `json:"expires_at"`
		Scope     []string `json:"scope"`
	}{
		RequestID: req.ID,
		CitizenID: req.CitizenID,
		Expiry:    req.ExpiresAt.Format(time.RFC3339),
		Scope:     req.DataScope,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal authorization payload: %w", err)
	}
	enc := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, s.tokenSecret)
	if _, err := mac.Write(body); err != nil {
		return "", fmt.Errorf("hmac write: %w", err)
	}
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return enc + "." + sig, nil
}

func scopeJSON(scope []string) string {
	b, _ := json.Marshal(scope)
	return string(b)
}

func scanRequests(rows pgx.Rows) ([]models.ConsentRequest, error) {
	out := make([]models.ConsentRequest, 0)
	for rows.Next() {
		var req models.ConsentRequest
		var authToken *string
		if err := rows.Scan(&req.ID, &req.CitizenID, &req.InstitutionID, &req.DataScope, &req.Purpose,
			&req.Status, &authToken, &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan consent request: %w", err)
		}
		if authToken != nil {
			req.AuthToken = *authToken
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

var (
	// ErrNotFound is returned when a request does not exist.
	ErrNotFound = errors.New("consent request not found")
	// ErrNotOwned is returned when a citizen acts on another citizen's request.
	ErrNotOwned = errors.New("consent request not owned by citizen")
	// ErrInvalidState is returned for a transition from an unexpected status.
	ErrInvalidState = errors.New("consent request is not in a valid state for this action")
	// ErrExpired is returned when approving an already-expired request.
	ErrExpired = errors.New("consent request has expired")
)
