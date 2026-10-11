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

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const institutionColumns = `id, name, sector, role, api_key_hash, is_active, created_at`

func (r *Repository) Create(ctx context.Context, name, sector string, role models.InstitutionRole) (models.Institution, string, error) {
	if !ValidRole(role) {
		return models.Institution{}, "", ErrInvalidRole
	}
	key, err := generateAPIKey()
	if err != nil {
		return models.Institution{}, "", err
	}
	id := uuid.NewString()
	_, err = r.pool.Exec(ctx, `
		INSERT INTO institution.institutions (id, name, sector, role, api_key_hash)
		VALUES ($1, $2, $3, $4, $5)`,
		id, name, sector, string(role), hashAPIKey(key))
	if err != nil {
		return models.Institution{}, "", fmt.Errorf("insert institution: %w", err)
	}
	inst := models.Institution{
		ID:        id,
		Name:      name,
		Sector:    sector,
		Role:      role,
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
	}
	return inst, key, nil
}

func (r *Repository) ByID(ctx context.Context, id string) (models.Institution, error) {
	var inst models.Institution
	var role string
	err := r.pool.QueryRow(ctx, `
		SELECT `+institutionColumns+`
		FROM institution.institutions WHERE id = $1`, id).
		Scan(&inst.ID, &inst.Name, &inst.Sector, &role, &inst.APIKeyHash, &inst.IsActive, &inst.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Institution{}, ErrNotFound
	}
	if err != nil {
		return models.Institution{}, fmt.Errorf("query institution: %w", err)
	}
	inst.Role = models.InstitutionRole(role)
	return inst, nil
}

func (r *Repository) Authenticate(ctx context.Context, apiKey string) (models.Institution, error) {
	var inst models.Institution
	var role string
	err := r.pool.QueryRow(ctx, `
		SELECT `+institutionColumns+`
		FROM institution.institutions WHERE api_key_hash = $1 AND is_active = true`, hashAPIKey(apiKey)).
		Scan(&inst.ID, &inst.Name, &inst.Sector, &role, &inst.APIKeyHash, &inst.IsActive, &inst.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Institution{}, ErrInvalidKey
	}
	if err != nil {
		return models.Institution{}, fmt.Errorf("authenticate institution: %w", err)
	}
	inst.Role = models.InstitutionRole(role)
	return inst, nil
}

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

func (r *Repository) SetRole(ctx context.Context, id string, role models.InstitutionRole) (models.Institution, error) {
	if !ValidRole(role) {
		return models.Institution{}, ErrInvalidRole
	}
	tag, err := r.pool.Exec(ctx, `UPDATE institution.institutions SET role = $2 WHERE id = $1`, id, string(role))
	if err != nil {
		return models.Institution{}, fmt.Errorf("update institution role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return models.Institution{}, ErrNotFound
	}
	return r.ByID(ctx, id)
}

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

func ValidRole(role models.InstitutionRole) bool {
	switch role {
	case models.InstitutionRoleViewer, models.InstitutionRoleAnalyst, models.InstitutionRoleAdmin:
		return true
	}
	return false
}

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
	ErrNotFound    = errors.New("institution not found")
	ErrInvalidKey  = errors.New("invalid institution api key")
	ErrInvalidRole = errors.New("invalid institution role")
)
