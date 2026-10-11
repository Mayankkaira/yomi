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

func testService(secret string) *Service {
	return &Service{tokenSecret: []byte(secret), receiptSecret: []byte("receipt"), tokenTTL: 24 * time.Hour}
}

func sampleRequest() models.ConsentRequest {
	return models.ConsentRequest{
		ID:              "req-abc",
		CitizenID:       "citizen-xyz",
		InstitutionID:   "inst-1",
		InstitutionRole: models.InstitutionRoleAnalyst,
		DataScope:       []string{"account.balance", "credit.score"},
		Purpose:         "Loan",
		ExpiresAt:       time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
	}
}

func TestIssueAccessTokenIsSignedAndParsable(t *testing.T) {
	svc := testService("test-secret-value")
	req := sampleRequest()

	token, err := svc.IssueAccessToken(req)
	if err != nil {
		t.Fatalf("IssueAccessToken error: %v", err)
	}
	payloadPart, sig, ok := strings.Cut(token, ".")
	if !ok {
		t.Fatalf("token must be payload.signature, got %s", token)
	}
	body, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var claims struct {
		RequestID string   `json:"request_id"`
		CitizenID string   `json:"citizen_id"`
		Expiry    string   `json:"expires_at"`
		Scope     []string `json:"scope"`
	}
	if err := json.Unmarshal(body, &claims); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if claims.RequestID != "req-abc" || claims.CitizenID != "citizen-xyz" {
		t.Fatalf("claims mismatch: %+v", claims)
	}
	if claims.Expiry != req.ExpiresAt.Format(time.RFC3339) {
		t.Fatalf("expiry mismatch: %s", claims.Expiry)
	}
	if len(claims.Scope) != 2 || claims.Scope[0] != "account.balance" {
		t.Fatalf("scope mismatch: %v", claims.Scope)
	}

	mac := hmac.New(sha256.New, []byte("test-secret-value"))
	mac.Write(body)
	if !hmac.Equal([]byte(sig), []byte(base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))) {
		t.Fatal("invalid signature")
	}
}

func TestIssueAccessTokenIsDeterministic(t *testing.T) {
	svc := testService("s")
	req := sampleRequest()
	first, _ := svc.IssueAccessToken(req)
	second, _ := svc.IssueAccessToken(req)
	if first != second {
		t.Fatal("signing must be deterministic for the same request")
	}
}

func TestIssueAccessTokenDiffersPerSecret(t *testing.T) {
	req := sampleRequest()
	a, _ := testService("secret-a").IssueAccessToken(req)
	b, _ := testService("secret-b").IssueAccessToken(req)
	if a == b {
		t.Fatal("different secrets must produce different signatures")
	}
}

func TestHashAccessTokenIsStableAndNotReversible(t *testing.T) {
	token, err := generateAccessToken()
	if err != nil {
		t.Fatalf("generateAccessToken error: %v", err)
	}
	if !strings.HasPrefix(token, "yomi_access_") {
		t.Fatalf("unexpected token prefix: %s", token)
	}
	if hashAccessToken(token) != hashAccessToken(token) {
		t.Fatal("hash must be deterministic")
	}
	if hashAccessToken(token) == token {
		t.Fatal("stored value must never be the token itself")
	}
	other, _ := generateAccessToken()
	if hashAccessToken(token) == hashAccessToken(other) {
		t.Fatal("distinct tokens must hash distinctly")
	}
}

func TestFingerprintIsShortButStable(t *testing.T) {
	hash := hashAccessToken("yomi_access_abc")
	if len(fingerprint(hash)) != 12 {
		t.Fatalf("fingerprint must be 12 chars, got %d", len(fingerprint(hash)))
	}
	if fingerprint("short") != "short" {
		t.Fatal("short input must pass through unchanged")
	}
}

func TestSplitTokenRejectsMalformed(t *testing.T) {
	for _, bad := range []string{"", ".", "abc.", ".sig", "noseparator"} {
		if _, _, ok := splitToken(bad); ok {
			t.Fatalf("malformed token accepted: %q", bad)
		}
	}
	head, tail, ok := splitToken("a.b")
	if !ok || head != "a" || tail != "b" {
		t.Fatal("well-formed token must split into two parts")
	}
}

func TestNormalizeScopeRejectsUnknownKey(t *testing.T) {
	if _, err := NormalizeScope(models.InstitutionRoleAdmin, []string{"financial.credit", "telepathy"}); err == nil {
		t.Fatal("unknown scope key must be rejected")
	}
}

func TestNormalizeScopeEnforcesRoleCeiling(t *testing.T) {
	if _, err := NormalizeScope(models.InstitutionRoleViewer, []string{"health.records"}); err == nil {
		t.Fatal("a viewer must not be able to request health records")
	}
	if _, err := NormalizeScope(models.InstitutionRoleAnalyst, []string{"financial.credit"}); err == nil {
		t.Fatal("an analyst must not be able to request credit reports")
	}
	if _, err := NormalizeScope(models.InstitutionRoleAdmin, []string{"financial.credit"}); err != nil {
		t.Fatalf("an admin must be able to request credit reports: %v", err)
	}
}

func TestNormalizeScopeReturnsCatalogOrderAndDeduplicates(t *testing.T) {
	got, err := NormalizeScope(models.InstitutionRoleAdmin, []string{"identity.contact", "identity.basic", "identity.contact"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "identity.basic" || got[1] != "identity.contact" {
		t.Fatalf("scope must be deduplicated and ordered by catalog, got %v", got)
	}
}

func TestNormalizeScopeRejectsUnknownRole(t *testing.T) {
	if _, err := NormalizeScope("superuser", []string{"identity.basic"}); err == nil {
		t.Fatal("unknown role must be rejected")
	}
}

func TestDescribeScopesLabelsKnownKeys(t *testing.T) {
	out := DescribeScopes([]string{"identity.basic", "mystery.key"})
	if out[0].Label != "Name and address" {
		t.Fatalf("known scope must carry its human label, got %q", out[0].Label)
	}
	if out[1].Label != "mystery.key" {
		t.Fatalf("unknown scope must fall back to its key, got %q", out[1].Label)
	}
}

func TestCatalogForRoleHidesHigherScopes(t *testing.T) {
	viewer := CatalogForRole(models.InstitutionRoleViewer)
	for _, s := range viewer {
		if s.Key == "health.records" || s.Key == "financial.credit" {
			t.Fatalf("viewer catalog leaked %s", s.Key)
		}
	}
	admin := CatalogForRole(models.InstitutionRoleAdmin)
	if len(admin) != len(ScopeCatalog()) {
		t.Fatalf("admin catalog must expose every scope, got %d of %d", len(admin), len(ScopeCatalog()))
	}
}
