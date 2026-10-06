// Package audit implements the Audit Ledger service. It consumes consent
// lifecycle events from Kafka and appends them to an append-only, hash-chained
// audit_log table. Entries are never updated or deleted.
package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yomi/yomi/internal/models"
)

// Repository owns the append-only audit schema.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository wires the postgres pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Append inserts a new audit entry, chaining it to the previous entry's hash.
// This is the only write path into the ledger. The tail read and insert run
// in a single transaction with the tail row locked, so concurrent appends
// cannot produce two entries with the same prev_hash (a chain fork). Replays
// of an already-appended event ID are idempotent: the existing entry is
// returned unchanged.
func (r *Repository) Append(ctx context.Context, entry models.AuditEntry) (models.AuditEntry, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return models.AuditEntry{}, fmt.Errorf("begin append tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	prevHash, err := tailHashLocked(ctx, tx)
	if err != nil {
		return models.AuditEntry{}, fmt.Errorf("read ledger tail: %w", err)
	}
	entry.PrevHash = prevHash
	entry.EntryHash = hashEntry(entry)

	var metadata []byte
	if entry.Metadata != nil {
		metadata = entry.Metadata
	} else {
		metadata = []byte("{}")
	}

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
		// Unique violation on the event id: the consumer already appended this
		// event (at-least-once delivery). Return the stored entry unchanged.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return r.ByID(ctx, entry.ID)
		}
		return models.AuditEntry{}, fmt.Errorf("append audit entry: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return models.AuditEntry{}, fmt.Errorf("commit append tx: %w", err)
	}
	return out, nil
}

// tailHashLocked returns the entry_hash of the most recent ledger entry,
// locking the tail row so concurrent appends serialize on it.
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

// ByCitizen lists ledger entries filtered by citizen, newest first.
func (r *Repository) ByCitizen(ctx context.Context, citizenID string) ([]models.AuditEntry, error) {
	return r.query(ctx, `WHERE citizen_id = $1 ORDER BY sequence_no DESC`, citizenID)
}

// ByInstitution lists ledger entries filtered by institution, newest first.
func (r *Repository) ByInstitution(ctx context.Context, institutionID string) ([]models.AuditEntry, error) {
	return r.query(ctx, `WHERE institution_id = $1 ORDER BY sequence_no DESC`, institutionID)
}

// ByID returns a single ledger entry by its event ID.
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

// VerifyChain recomputes every entry hash starting from the genesis marker and
// reports whether the ledger is intact. It returns a list of the chain links
// with recomputed integrity flags.
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

// ChainLink pairs a ledger entry with its recomputed integrity status.
type ChainLink struct {
	Entry models.AuditEntry `json:"entry"`
	Valid bool              `json:"valid"`
}

// query runs a ledger query with the given WHERE fragment and args, returning
// entries newest-first.
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

// genesisHash is the fixed marker used as the prev_hash of the first entry.
const genesisHash = "GENESIS-0000000000000000000000000000000000000000000000000000000000000000"

// hashEntry computes the canonical SHA-256 hash over a fixed serialization of
// the entry's identity fields plus its prev_hash. This is what binds each
// entry to its predecessor. Two normalization rules keep the hash stable
// between insert time and verification time:
//   - the timestamp is truncated to microsecond precision (PostgreSQL
//     timestamptz stores microseconds);
//   - the metadata is canonicalized through a JSON round-trip, because jsonb
//     reorders object keys on storage.
func hashEntry(e models.AuditEntry) string {
	meta := "{}"
	if e.Metadata != nil {
		var decoded interface{}
		if err := json.Unmarshal(e.Metadata, &decoded); err == nil {
			if canonical, err := json.Marshal(decoded); err == nil {
				meta = string(canonical)
			} else {
				meta = string(e.Metadata)
			}
		} else {
			meta = string(e.Metadata)
		}
	}
	canonical := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		e.EventType,
		nullStr(e.CitizenID),
		nullStr(e.InstitutionID),
		nullStr(e.RequestID),
		meta,
		e.CreatedAt.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano),
		e.PrevHash,
	)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func nullStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// FromEvent converts a Kafka consent event into a ledger entry. The entry ID
// reuses the event ID so redelivered events append idempotently.
func FromEvent(evt models.ConsentEvent) models.AuditEntry {
	var citizenID, institutionID, requestID *string
	c := evt.CitizenID
	i := evt.InstitutionID
	r := evt.RequestID
	if c != "" {
		citizenID = &c
	}
	if i != "" {
		institutionID = &i
	}
	if r != "" {
		requestID = &r
	}
	meta := evt.Metadata
	if meta == nil {
		meta = json.RawMessage(`{}`)
	}
	id := evt.EventID
	if id == "" {
		id = uuid.NewString()
	}
	return models.AuditEntry{
		ID:            id,
		EventType:     evt.EventType,
		CitizenID:     citizenID,
		InstitutionID: institutionID,
		RequestID:     requestID,
		Metadata:      meta,
		CreatedAt:     evt.Timestamp,
	}
}

// ErrNotFound is returned when an audit entry does not exist.
var ErrNotFound = errors.New("audit entry not found")
