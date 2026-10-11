package audit

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yomi/yomi/internal/models"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Append(ctx context.Context, entry models.AuditEntry) (models.AuditEntry, error) {
	out, _, err := r.AppendWithReceipt(ctx, entry, nil)
	return out, err
}

func (r *Repository) AppendWithReceipt(ctx context.Context, entry models.AuditEntry, receiptSecret []byte) (models.AuditEntry, *Receipt, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return models.AuditEntry{}, nil, fmt.Errorf("begin append tx: %w", err)
	}
	defer tx.Rollback(ctx)

	prevHash, err := tailHashLocked(ctx, tx)
	if err != nil {
		return models.AuditEntry{}, nil, fmt.Errorf("read ledger tail: %w", err)
	}
	entry.PrevHash = prevHash
	entry.EntryHash = hashEntry(entry)

	metadata := canonicalMetadata(entry.Metadata)

	var out models.AuditEntry
	err = tx.QueryRow(ctx, `
		INSERT INTO audit.audit_log
			(id, event_type, citizen_id, institution_id, request_id, metadata, prev_hash, entry_hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, sequence_no, event_type, citizen_id, institution_id, request_id, metadata, prev_hash, entry_hash, created_at`,
		entry.ID, entry.EventType, entry.CitizenID, entry.InstitutionID, entry.RequestID,
		metadata, entry.PrevHash, entry.EntryHash, entry.CreatedAt).
		Scan(&out.ID, &out.SequenceNo, &out.EventType, &out.CitizenID, &out.InstitutionID,
			&out.RequestID, &out.Metadata, &out.PrevHash, &out.EntryHash, &out.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			existing, lookupErr := r.ByID(ctx, entry.ID)
			if lookupErr != nil {
				return models.AuditEntry{}, nil, lookupErr
			}
			var rec *Receipt
			if len(receiptSecret) > 0 {
				built := BuildReceipt(existing, receiptSecret)
				rec = &built
			}
			return existing, rec, nil
		}
		return models.AuditEntry{}, nil, fmt.Errorf("append audit entry: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return models.AuditEntry{}, nil, fmt.Errorf("commit append tx: %w", err)
	}
	if len(receiptSecret) == 0 {
		return out, nil, nil
	}
	rec := BuildReceipt(out, receiptSecret)
	return out, &rec, nil
}

func tailHashLocked(ctx context.Context, tx pgx.Tx) (string, error) {
	var hash string
	err := tx.QueryRow(ctx, `
		SELECT entry_hash FROM audit.audit_log ORDER BY sequence_no DESC LIMIT 1
		FOR UPDATE`).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return genesisHash, nil
	}
	if err != nil {
		return "", err
	}
	return hash, nil
}

func (r *Repository) ByCitizen(ctx context.Context, citizenID string) ([]models.AuditEntry, error) {
	return r.query(ctx, `WHERE citizen_id = $1 ORDER BY sequence_no DESC`, citizenID)
}

func (r *Repository) ByInstitution(ctx context.Context, institutionID string) ([]models.AuditEntry, error) {
	return r.query(ctx, `WHERE institution_id = $1 ORDER BY sequence_no DESC`, institutionID)
}

func (r *Repository) ByID(ctx context.Context, eventID string) (models.AuditEntry, error) {
	var e models.AuditEntry
	err := r.pool.QueryRow(ctx, `
		SELECT id, sequence_no, event_type, citizen_id, institution_id, request_id, metadata, prev_hash, entry_hash, created_at
		FROM audit.audit_log WHERE id = $1`, eventID).
		Scan(&e.ID, &e.SequenceNo, &e.EventType, &e.CitizenID, &e.InstitutionID,
			&e.RequestID, &e.Metadata, &e.PrevHash, &e.EntryHash, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.AuditEntry{}, ErrNotFound
	}
	if err != nil {
		return models.AuditEntry{}, fmt.Errorf("query audit entry: %w", err)
	}
	return e, nil
}

func (r *Repository) VerifyChain(ctx context.Context, limit int) ([]ChainLink, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, sequence_no, event_type, citizen_id, institution_id, request_id, metadata, prev_hash, entry_hash, created_at
		FROM audit.audit_log ORDER BY sequence_no ASC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query chain for verification: %w", err)
	}
	defer rows.Close()

	links := make([]ChainLink, 0)
	expectedPrev := genesisHash
	for rows.Next() {
		var e models.AuditEntry
		if err := rows.Scan(&e.ID, &e.SequenceNo, &e.EventType, &e.CitizenID, &e.InstitutionID,
			&e.RequestID, &e.Metadata, &e.PrevHash, &e.EntryHash, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan chain entry: %w", err)
		}
		recomputed := hashEntry(e)
		valid := e.PrevHash == expectedPrev && e.EntryHash == recomputed
		links = append(links, ChainLink{Entry: e, Valid: valid})
		expectedPrev = e.EntryHash
	}
	return links, rows.Err()
}

func (r *Repository) query(ctx context.Context, where string, args ...interface{}) ([]models.AuditEntry, error) {
	sql := `SELECT id, sequence_no, event_type, citizen_id, institution_id, request_id, metadata, prev_hash, entry_hash, created_at
		FROM audit.audit_log ` + where
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit ledger: %w", err)
	}
	defer rows.Close()
	out := make([]models.AuditEntry, 0)
	for rows.Next() {
		var e models.AuditEntry
		if err := rows.Scan(&e.ID, &e.SequenceNo, &e.EventType, &e.CitizenID, &e.InstitutionID,
			&e.RequestID, &e.Metadata, &e.PrevHash, &e.EntryHash, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

var ErrNotFound = errors.New("audit entry not found")
