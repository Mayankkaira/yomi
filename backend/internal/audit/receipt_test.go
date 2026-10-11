package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/yomi/yomi/internal/models"
)

func sampleEntry() models.AuditEntry {
	citizen := "citizen-1"
	inst := "inst-1"
	req := "req-1"
	return models.AuditEntry{
		ID:            "evt-1",
		SequenceNo:    7,
		EventType:     models.EventConsentApproved,
		CitizenID:     &citizen,
		InstitutionID: &inst,
		RequestID:     &req,
		Metadata:      json.RawMessage(`{"scope":["a","b"],"purpose":"loans"}`),
		PrevHash:      "abc123",
		EntryHash:     "def456",
		CreatedAt:     time.Date(2026, 8, 6, 12, 0, 0, 123456789, time.UTC),
	}
}

func TestHashEntryDeterministic(t *testing.T) {
	e := sampleEntry()
	if hashEntry(e) != hashEntry(e) {
		t.Fatal("hashEntry must be deterministic")
	}
}

func TestHashEntryMicrosecondTruncation(t *testing.T) {
	ns := time.Date(2026, 8, 6, 12, 0, 0, 123456789, time.UTC)
	a := sampleEntry()
	a.CreatedAt = ns
	b := sampleEntry()
	b.CreatedAt = ns.Truncate(time.Microsecond)
	if hashEntry(a) != hashEntry(b) {
		t.Fatal("nanosecond and microsecond timestamps must hash equal")
	}
}

func TestHashEntryCanonicalizesMetadataKeyOrder(t *testing.T) {
	a := sampleEntry()
	a.Metadata = json.RawMessage(`{"purpose":"loans","scope":["a","b"]}`)
	b := sampleEntry()
	b.Metadata = json.RawMessage(`{"scope":["a","b"],"purpose":"loans"}`)
	if hashEntry(a) != hashEntry(b) {
		t.Fatal("metadata key order must not change the hash")
	}
}

func TestHashEntryChangesWhenDataChanges(t *testing.T) {
	base := sampleEntry()
	h := hashEntry(base)

	mutated := sampleEntry()
	other := "citizen-2"
	mutated.CitizenID = &other
	if hashEntry(mutated) == h {
		t.Fatal("changing citizen_id must change the hash")
	}

	mutated = sampleEntry()
	mutated.PrevHash = "zzz"
	if hashEntry(mutated) == h {
		t.Fatal("changing prev_hash must change the hash")
	}
}

func TestHashEntryMatchesReferenceDigest(t *testing.T) {
	e := sampleEntry()
	canonical := "consent_approved|citizen-1|inst-1|req-1|" +
		`{"purpose":"loans","scope":["a","b"]}` + "|" +
		"2026-08-06T12:00:00.123456Z|abc123"
	sum := sha256.Sum256([]byte(canonical))
	if hashEntry(e) != hex.EncodeToString(sum[:]) {
		t.Fatalf("hashEntry drifted from the documented canonical form: %s", hashEntry(e))
	}
}

func TestBuildReceiptIsDeterministicAndVerifiable(t *testing.T) {
	secret := []byte("receipt-secret")
	e := sampleEntry()
	first := BuildReceipt(e, secret)
	second := BuildReceipt(e, secret)
	if first.Signature != second.Signature {
		t.Fatal("receipt signature must be deterministic")
	}
	if first.ReceiptID != e.ID {
		t.Fatalf("receipt must carry the ledger event id, got %s", first.ReceiptID)
	}
	if first.Version != receiptVersion {
		t.Fatalf("unexpected receipt version %s", first.Version)
	}
	if !VerifyReceipt(first, secret) {
		t.Fatal("a freshly built receipt must verify")
	}
}

func TestVerifyReceiptRejectsTampering(t *testing.T) {
	secret := []byte("receipt-secret")
	rec := BuildReceipt(sampleEntry(), secret)

	tampered := rec
	tampered.EventType = models.EventConsentRevoked
	if VerifyReceipt(tampered, secret) {
		t.Fatal("receipt with an altered event type must not verify")
	}

	tampered = rec
	tampered.SequenceNo = 99
	if VerifyReceipt(tampered, secret) {
		t.Fatal("receipt with an altered sequence must not verify")
	}

	tampered = rec
	tampered.Metadata = json.RawMessage(`{"scope":["everything"]}`)
	if VerifyReceipt(tampered, secret) {
		t.Fatal("receipt with altered metadata must not verify")
	}
}

func TestVerifyReceiptRejectsWrongSecret(t *testing.T) {
	rec := BuildReceipt(sampleEntry(), []byte("secret-a"))
	if VerifyReceipt(rec, []byte("secret-b")) {
		t.Fatal("receipt signed with another secret must not verify")
	}
}

func TestVerifyReceiptRejectsEmptySignature(t *testing.T) {
	rec := BuildReceipt(sampleEntry(), []byte("s"))
	rec.Signature = ""
	if VerifyReceipt(rec, []byte("s")) {
		t.Fatal("receipt without a signature must not verify")
	}
}

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
	if entry.CitizenID == nil || *entry.CitizenID != "citizen-1" {
		t.Fatal("citizen ID not mapped")
	}
	if entry.InstitutionID == nil || *entry.InstitutionID != "inst-1" {
		t.Fatal("institution ID not mapped")
	}
	if entry.RequestID == nil || *entry.RequestID != "req-1" {
		t.Fatal("request ID not mapped")
	}
	if !entry.CreatedAt.Equal(ts) {
		t.Fatal("timestamp not mapped")
	}
	if string(entry.Metadata) != `{"latency_ms":5}` {
		t.Fatalf("metadata not mapped: %s", string(entry.Metadata))
	}
}

func TestFromEventDefaultsMetadataAndID(t *testing.T) {
	entry := FromEvent(models.ConsentEvent{EventType: models.EventConsentRevoked, Timestamp: time.Now().UTC()})
	if string(entry.Metadata) != "{}" {
		t.Fatalf("nil metadata should default to {}, got %s", string(entry.Metadata))
	}
	if entry.ID == "" {
		t.Fatal("expected a generated event id")
	}
}
