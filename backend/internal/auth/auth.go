// Package auth implements the Auth service domain: citizen registration,
// credential verification, and JWT/refresh token lifecycle.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/yomi/yomi/internal/middleware"
	"github.com/yomi/yomi/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// Repository owns the auth schema and Redis refresh-token store.
type Repository struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
}

// NewRepository wires the postgres pool and redis client.
func NewRepository(pool *pgxpool.Pool, rdb *redis.Client) *Repository {
	return &Repository{pool: pool, rdb: rdb}
}

// CreateCitizen inserts a new citizen after hashing their password.
func (r *Repository) CreateCitizen(ctx context.Context, name, aadhaar, email, phone, password string) (models.Citizen, error) {
	hash, err := bcryptFromPassword(password)
	if err != nil {
		return models.Citizen{}, fmt.Errorf("hash password: %w", err)
	}
	id := uuid.NewString()
	_, err = r.pool.Exec(ctx, `
		INSERT INTO auth.citizens (id, aadhaar_id, full_name, email, phone, password_hash)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		id, aadhaar, name, email, phone, string(hash))
	if err != nil {
		return models.Citizen{}, fmt.Errorf("insert citizen: %w", err)
	}
	return models.Citizen{
		ID:        id,
		AadhaarID: aadhaar,
		FullName:  name,
		Email:     email,
		Phone:     phone,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// CitizenByAadhaar fetches a citizen (including password hash) by Aadhaar ID.
func (r *Repository) CitizenByAadhaar(ctx context.Context, aadhaar string) (models.Citizen, error) {
	var c models.Citizen
	err := r.pool.QueryRow(ctx, `
		SELECT id, aadhaar_id, full_name, email, phone, password_hash, created_at
		FROM auth.citizens WHERE aadhaar_id = $1`, aadhaar).
		Scan(&c.ID, &c.AadhaarID, &c.FullName, &c.Email, &c.Phone, &c.PasswordHash, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Citizen{}, ErrNotFound
	}
	if err != nil {
		return models.Citizen{}, fmt.Errorf("query citizen: %w", err)
	}
	return c, nil
}

// CitizenByID fetches a citizen by primary key.
func (r *Repository) CitizenByID(ctx context.Context, id string) (models.Citizen, error) {
	var c models.Citizen
	err := r.pool.QueryRow(ctx, `
		SELECT id, aadhaar_id, full_name, email, phone, password_hash, created_at
		FROM auth.citizens WHERE id = $1`, id).
		Scan(&c.ID, &c.AadhaarID, &c.FullName, &c.Email, &c.Phone, &c.PasswordHash, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Citizen{}, ErrNotFound
	}
	if err != nil {
		return models.Citizen{}, fmt.Errorf("query citizen: %w", err)
	}
	return c, nil
}

// TokenService issues and validates access and refresh tokens.
type TokenService struct {
	priv    *rsa.PrivateKey
	pub     *rsa.PublicKey
	rdb     *redis.Client
	access  time.Duration
	refresh time.Duration
}

// NewTokenService builds the token service with signing and verification keys.
func NewTokenService(priv *rsa.PrivateKey, pub *rsa.PublicKey, rdb *redis.Client, access, refresh time.Duration) *TokenService {
	return &TokenService{priv: priv, pub: pub, rdb: rdb, access: access, refresh: refresh}
}

// IssueAccessToken creates a short-lived RS256 JWT for a subject.
func (s *TokenService) IssueAccessToken(subjectID string, role models.Role) (string, error) {
	now := time.Now()
	claims := middleware.Claims{
		SubjectID: subjectID,
		Role:      string(role),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "yomi",
			Subject:   subjectID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.access)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return tok.SignedString(s.priv)
}

// IssueRefreshToken generates a random opaque token, stores it in Redis bound
// to the subject, and returns both the plaintext token and its hash.
func (s *TokenService) IssueRefreshToken(ctx context.Context, subjectID string) (string, error) {
	raw, err := generateOpaqueToken(32)
	if err != nil {
		return "", err
	}
	hash := hashToken(raw)
	key := refreshKey(hash)
	if err := s.rdb.Set(ctx, key, subjectID, s.refresh).Err(); err != nil {
		return "", fmt.Errorf("store refresh token: %w", err)
	}
	return raw, nil
}

// ValidateRefreshToken checks a refresh token exists in Redis and returns its
// subject. It atomically rotates the token: the old one is deleted and a new
// one issued, matching the refresh endpoint contract. GETDEL makes
// consumption atomic — under concurrent refreshes with the same token exactly
// one call wins; the losers see the token already gone and fail with
// ErrInvalidRefresh.
func (s *TokenService) ValidateRefreshToken(ctx context.Context, raw string) (string, error) {
	hash := hashToken(raw)
	key := refreshKey(hash)
	subject, err := s.rdb.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrInvalidRefresh
	}
	if err != nil {
		return "", fmt.Errorf("get refresh token: %w", err)
	}
	if subject == "" {
		return "", ErrInvalidRefresh
	}
	return subject, nil
}

// RevokeRefreshToken invalidates a stored refresh token.
func (s *TokenService) RevokeRefreshToken(ctx context.Context, raw string) error {
	key := refreshKey(hashToken(raw))
	if err := s.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

// PublicKey exposes the verification key so other services can validate tokens.
func (s *TokenService) PublicKey() *rsa.PublicKey { return s.pub }

func refreshKey(hash string) string { return "refresh:" + hash }

func hashToken(raw string) string {
	sum := sha256sum(raw)
	return base64.RawURLEncoding.EncodeToString(sum)
}

func generateOpaqueToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// sha256sum returns the SHA-256 digest of the input.
func sha256sum(data string) []byte {
	sum := sha256.Sum256([]byte(data))
	return sum[:]
}

// bcryptFromPassword hashes a plaintext password with bcrypt. Kept as a
// package-level helper so it is unit-testable; CreateCitizen uses it.
func bcryptFromPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

var (
	// ErrNotFound is returned when a citizen does not exist.
	ErrNotFound = errors.New("citizen not found")
	// ErrInvalidRefresh is returned when a refresh token is unknown or expired.
	ErrInvalidRefresh = errors.New("invalid refresh token")
)
