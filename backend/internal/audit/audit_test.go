package audit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yomi/yomi/internal/models"
)

// TestHashEntryDeterministic ensures the same entry always hashes identically,
// so the chain is stable between insert and verification.
func TestHashEntryDeterministic(t *testing.T) {
	ts := time.Date(2026, 8, 6, 12, 0, 0, 123456789, time.UTC)
	e := models.AuditEntry{
		EventType:     models.EventConsentApproved,
		CitizenID:     strPtr("citizen-1"),
		InstitutionID: strPtr("inst-1"),
		RequestID:     strPtr("req-1"),
		Metadata:      json.RawMessage(`{"scope":["a","b"],"purpose":"loans"}`),
		CreatedAt:     ts,
		PrevHash:      "prev-hash",
	}
	h1 := hashEntry(e)
	h2 := hashEntry(e)
	if h1 != h2 {
		t.Fatalf("hashEntry not deterministic: %s != %s", h1, h2)
	}
	ld := copyAudit(e)
	different := hashEntry(ld)
	if h1 != different {
		t.Fatalf("hashEntry changed despite equal semantic value: %s != %s", h1, different)
	}
}

// TestHashEntryMicrosecondTruncation ensures the timestamp is normalized to
// microsecond precision, matching what PostgreSQL timestamptz stores.
func TestHashEntryMicrosecondTruncation(t *testing.T) {
	ns := time.Date(2026, 8, 6, 12, 0, 0, 123456789, time.UTC) // nanosecond precision
	micro := ns.Truncate(time.Microsecond)

	e1 := models.AuditEntry{EventType: "x", CreatedAt: ns, PrevHash: "p"}
	e2 := models.AuditEntry{EventType: "x", CreatedAt: micro, PrevHash: "p"}

	if hashEntry(e1) != hashEntry(e2) {
		t.Fatalf("nanosecond vs microsecond timestamps must hash equal")
	}
}

// TestHashEntryChangesWhenDataChanges ensures any field change alters the hash.
func TestHashEntryChangesWhenDataChanges(t *testing.T) {
	base := models.AuditEntry{
		EventType:     models.EventConsentRequested,
		CitizenID:     strPtr("c"),
		InstitutionID: strPtr("i"),
		RequestID:     strPtr("r"),
		Metadata:      json.RawMessage(`{"k":"v"}`),
		CreatedAt:     time.Now().UTC(),
		PrevHash:      "p",
	}
	h := hashEntry(base)

	mutated := copyAudit(base)
	*mutated.CitizenID = "c2"
	if hashEntry(mutated) == h {
		t.Fatalf("changing citizen_id must change the hash")
	}

	mutated2 := copyAudit(base)
	mutated2.PrevHash = "p2"
	if hashEntry(mutated2) == h {
		t.Fatalf("changing prev_hash must change the hash")
	}
}

// TestFromEventMapsAllFields ensures a Kafka event converts to a complete
// ledger entry with the same event ID (for idempotent append).
func TestFromEventMapsAllFields(t *testing.T) {
	ts := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	evt := models.ConsentEvent{
		EventID:       "evt-123",
		EventType:     models.EventConsentApproved,
		CitizenID:     "citizen-1",
		InstitutionID: "inst-1",
		RequestID:     "req-1",
		Timestamp:     ts,
		Metadata:      json.RawMessage(`{"latency_ms":5}`),
	}
	entry := FromEvent(evt)
	if entry.ID != "evt-123" {
		t.Fatalf("entry ID must reuse event ID for idempotency, got %s", entry.ID)
	}
	if entry.EventType != models.EventConsentApproved {
		t.Fatalf("event type mismatch: %s", entry.EventType)
	}
	if entry.CitizenID == nil || *entry.CitizenID != "citizen-1" {
		t.Fatalf("citizen ID not mapped")
	}
	if entry.InstitutionID == nil || *entry.InstitutionID != "inst-1" {
		t.Fatalf("institution ID not mapped")
	}
	if entry.RequestID == nil || *entry.RequestID != "req-1" {
		t.Fatalf("request ID not mapped")
	}
	if !entry.CreatedAt.Equal(ts) {
		t.Fatalf("timestamp not mapped")
	}
	if string(entry.Metadata) != `{"latency_ms":5}` {
		t.Fatalf("metadata not mapped: %s", string(entry.Metadata))
	}
}

// TestFromEventDefaultsMetadata ensures nil metadata becomes an empty object.
func TestFromEventDefaultsMetadata(t *testing.T) {
	evt := models.ConsentEvent{EventID: "e", EventType: "consent_revoked", Timestamp: time.Now().UTC()}
	entry := FromEvent(evt)
	if string(entry.Metadata) != "{}" {
		t.Fatalf("nil metadata should default to {}, got %s", string(entry.Metadata))
	}
}

// TestFromEventGeneratesID ensures a missing event ID still yields an entry.
func TestFromEventGeneratesID(t *testing.T) {
	evt := models.ConsentEvent{EventType: "x", Timestamp: time.Now().UTC()}
	entry := FromEvent(evt)
	if entry.ID == "" {
		t.Fatalf("expected a generated ID")
	}
}

func strPtr(s string) *string { return &s }

func copyAudit(e models.AuditEntry) models.AuditEntry {
	c := e
	if c.CitizenID != nil {
		v := *c.CitizenID
		c.CitizenID = &v
	}
	if c.InstitutionID != nil {
		v := *c.InstitutionID
		c.InstitutionID = &v
	}
	if c.RequestID != nil {
		v := *c.RequestID
		c.RequestID = &v
	}
	return c
}
