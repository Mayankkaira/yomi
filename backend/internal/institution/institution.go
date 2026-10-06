// Package institution implements the Institution service: registration of data
// consuming organizations and API key management (issue, rotate, revoke).
package institution

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yomi/yomi/internal/models"
)

// Repository owns the institution schema.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository wires the postgres pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create inserts a new institution and returns it with the plaintext API key
// (shown once) alongside the stored hash.
func (r *Repository) Create(ctx context.Context, name, sector string) (models.Institution, string, error) {
	key, err := generateAPIKey()
	if err != nil {
		return models.Institution{}, "", err
	}
	id := uuid.NewString()
	_, err = r.pool.Exec(ctx, `
		INSERT INTO institution.institutions (id, name, sector, api_key_hash)
		VALUES ($1, $2, $3, $4)`,
		id, name, sector, hashAPIKey(key))
	if err != nil {
		return models.Institution{}, "", fmt.Errorf("insert institution: %w", err)
	}
	inst := models.Institution{
		ID:        id,
		Name:      name,
		Sector:    sector,
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
	}
	return inst, key, nil
}

// ByID fetches an institution by primary key.
func (r *Repository) ByID(ctx context.Context, id string) (models.Institution, error) {
	var inst models.Institution
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, sector, api_key_hash, is_active, created_at
		FROM institution.institutions WHERE id = $1`, id).
		Scan(&inst.ID, &inst.Name, &inst.Sector, &inst.APIKeyHash, &inst.IsActive, &inst.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Institution{}, ErrNotFound
	}
	if err != nil {
		return models.Institution{}, fmt.Errorf("query institution: %w", err)
	}
	return inst, nil
}

// Authenticate validates an API key and returns the matching institution.
func (r *Repository) Authenticate(ctx context.Context, apiKey string) (models.Institution, error) {
	var inst models.Institution
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, sector, api_key_hash, is_active, created_at
		FROM institution.institutions WHERE api_key_hash = $1 AND is_active = true`, hashAPIKey(apiKey)).
		Scan(&inst.ID, &inst.Name, &inst.Sector, &inst.APIKeyHash, &inst.IsActive, &inst.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Institution{}, ErrInvalidKey
	}
	if err != nil {
		return models.Institution{}, fmt.Errorf("authenticate institution: %w", err)
	}
	return inst, nil
}

// RotateKey replaces an institution's API key and returns the new plaintext.
func (r *Repository) RotateKey(ctx context.Context, id string) (models.Institution, string, error) {
	inst, err := r.ByID(ctx, id)
	if err != nil {
		return models.Institution{}, "", err
	}
	newKey, err := generateAPIKey()
	if err != nil {
		return models.Institution{}, "", err
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE institution.institutions SET api_key_hash = $2 WHERE id = $1`,
		id, hashAPIKey(newKey))
	if err != nil {
		return models.Institution{}, "", fmt.Errorf("rotate api key: %w", err)
	}
	return inst, newKey, nil
}

// Revoke deactivates an institution so its API key no longer authenticates.
func (r *Repository) Revoke(ctx context.Context, id string) (models.Institution, error) {
	inst, err := r.ByID(ctx, id)
	if err != nil {
		return models.Institution{}, err
	}
	_, err = r.pool.Exec(ctx, `UPDATE institution.institutions SET is_active = false WHERE id = $1`, id)
	if err != nil {
		return models.Institution{}, fmt.Errorf("revoke institution: %w", err)
	}
	inst.IsActive = false
	return inst, nil
}

// generateAPIKey produces a cryptographically random opaque key.
func generateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}
	return "yomi_" + base64.RawURLEncoding.EncodeToString(b), nil
}

func hashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

var (
	// ErrNotFound is returned when an institution does not exist.
	ErrNotFound = errors.New("institution not found")
	// ErrInvalidKey is returned when an API key is unknown or the institution
	// is inactive.
	ErrInvalidKey = errors.New("invalid institution api key")
)
