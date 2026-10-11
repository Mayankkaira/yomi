package audit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yomi/yomi/internal/models"
)

const (
	genesisHash    = "GENESIS-0000000000000000000000000000000000000000000000000000000000000000"
	headHash       = "HEAD"
	receiptVersion = "yomi-receipt/1"
)

type ChainLink struct {
	Entry models.AuditEntry `json:"entry"`
	Valid bool              `json:"valid"`
}

type Receipt struct {
	ReceiptID     string          `json:"receiptId"`
	Version       string          `json:"version"`
	SequenceNo    int64           `json:"sequenceNo"`
	EventType     string          `json:"eventType"`
	CitizenID     *string         `json:"citizenId,omitempty"`
	InstitutionID *string         `json:"institutionId,omitempty"`
	RequestID     *string         `json:"requestId,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	RecordedAt    time.Time       `json:"recordedAt"`
	PrevHash      string          `json:"prevHash"`
	EntryHash     string          `json:"entryHash"`
	Content       string          `json:"content"`
	Signature     string          `json:"signature"`
}

func BuildReceipt(e models.AuditEntry, secret []byte) Receipt {
	rec := Receipt{
		ReceiptID:     e.ID,
		Version:       receiptVersion,
		SequenceNo:    e.SequenceNo,
		EventType:     e.EventType,
		CitizenID:     e.CitizenID,
		InstitutionID: e.InstitutionID,
		RequestID:     e.RequestID,
		Metadata:      canonicalMetadata(e.Metadata),
		RecordedAt:    e.CreatedAt.UTC(),
		PrevHash:      e.PrevHash,
		EntryHash:     e.EntryHash,
	}
	rec.Content = receiptContent(rec)
	rec.Signature = signContent(rec.Content, secret)
	return rec
}

func VerifyReceipt(rec Receipt, secret []byte) bool {
	if rec.ReceiptID == "" || rec.Signature == "" {
		return false
	}
	expected := signContent(receiptContent(rec), secret)
	return hmac.Equal([]byte(expected), []byte(rec.Signature))
}

func receiptContent(rec Receipt) string {
	return strings.Join([]string{
		rec.Version,
		strconv.FormatInt(rec.SequenceNo, 10),
		rec.EventType,
		deref(rec.CitizenID),
		deref(rec.InstitutionID),
		deref(rec.RequestID),
		string(rec.Metadata),
		rec.RecordedAt.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano),
		rec.PrevHash,
		rec.EntryHash,
	}, "|")
}

func signContent(content string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(content))
	return hex.EncodeToString(mac.Sum(nil))
}

func hashEntry(e models.AuditEntry) string {
	canonical := strings.Join([]string{
		e.EventType,
		deref(e.CitizenID),
		deref(e.InstitutionID),
		deref(e.RequestID),
		string(canonicalMetadata(e.Metadata)),
		e.CreatedAt.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano),
		e.PrevHash,
	}, "|")
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func canonicalMetadata(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	var decoded interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return raw
	}
	canonical, err := json.Marshal(decoded)
	if err != nil {
		return raw
	}
	return canonical
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func FromEvent(evt models.ConsentEvent) models.AuditEntry {
	var citizenID, institutionID, requestID *string
	if evt.CitizenID != "" {
		c := evt.CitizenID
		citizenID = &c
	}
	if evt.InstitutionID != "" {
		i := evt.InstitutionID
		institutionID = &i
	}
	if evt.RequestID != "" {
		r := evt.RequestID
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
