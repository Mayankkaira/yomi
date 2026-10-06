package consent

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yomi/yomi/internal/models"
)

// TestSignAuthorizationToken verifies the token is a valid HMAC-SHA256 signed
// payload containing request ID, citizen ID, expiry, and scope.
func TestSignAuthorizationToken(t *testing.T) {
	secret := "test-secret-value"
	svc := &Service{tokenSecret: []byte(secret), tokenTTL: 24 * time.Hour}
	expires := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)

	req := models.ConsentRequest{
		ID:            "req-abc",
		CitizenID:     "citizen-xyz",
		InstitutionID: "inst-1",
		DataScope:     []string{"account.balance", "credit.score"},
		Purpose:       "Loan",
		ExpiresAt:     expires,
	}

	token, err := svc.signAuthorizationToken(req)
	if err != nil {
		t.Fatalf("signAuthorizationToken error: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		t.Fatalf("token must be payload.signature, got: %s", token)
	}

	// Decode payload and verify claims.
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var payload struct {
		RequestID string   `json:"request_id"`
		CitizenID string   `json:"citizen_id"`
		Expiry    string   `json:"expires_at"`
		Scope     []string `json:"scope"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.RequestID != "req-abc" {
		t.Errorf("request_id mismatch: %s", payload.RequestID)
	}
	if payload.CitizenID != "citizen-xyz" {
		t.Errorf("citizen_id mismatch: %s", payload.CitizenID)
	}
	if payload.Expiry != expires.Format(time.RFC3339) {
		t.Errorf("expiry mismatch: %s", payload.Expiry)
	}
	if len(payload.Scope) != 2 || payload.Scope[0] != "account.balance" || payload.Scope[1] != "credit.score" {
		t.Errorf("scope mismatch: %v", payload.Scope)
	}

	// Verify the HMAC signature.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payloadBytes)
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		t.Fatalf("invalid signature")
	}
}

// TestSignAuthorizationTokenDeterministic ensures the same request signs to the
// same token (no random salt).
func TestSignAuthorizationTokenDeterministic(t *testing.T) {
	svc := &Service{tokenSecret: []byte("s"), tokenTTL: time.Hour}
	req := models.ConsentRequest{
		ID:        "r",
		CitizenID: "c",
		DataScope: []string{"a"},
		ExpiresAt: time.Now().Add(time.Hour),
	}
	t1, _ := svc.signAuthorizationToken(req)
	t2, _ := svc.signAuthorizationToken(req)
	if t1 != t2 {
		t.Fatalf("token should be deterministic")
	}
}

// TestSignAuthorizationTokenDistinctPerSecret ensures changing the secret
// invalidates signatures (rotation invalidates old tokens).
func TestSignAuthorizationTokenDistinctPerSecret(t *testing.T) {
	req := models.ConsentRequest{ID: "r", CitizenID: "c", DataScope: []string{"a"}, ExpiresAt: time.Now().Add(time.Hour)}
	a := &Service{tokenSecret: []byte("secret-a"), tokenTTL: time.Hour}
	b := &Service{tokenSecret: []byte("secret-b"), tokenTTL: time.Hour}
	ta, _ := a.signAuthorizationToken(req)
	tb, _ := b.signAuthorizationToken(req)
	if ta == tb {
		t.Fatalf("different secrets must produce different tokens")
	}
}
