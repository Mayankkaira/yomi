package consent

import (
	"strings"
	"testing"
	"time"

	"github.com/yomi/yomi/internal/models"
)

func TestHashAccessTokenIsStableAndNotReversible(t *testing.T) {
	token, err := generateAccessToken()
	if err != nil {
		t.Fatalf("generateAccessToken error: %v", err)
	}
	if !strings.HasPrefix(token, "yomi_access_") {
		t.Fatalf("unexpected token prefix: %s", token)
	}
	if strings.Contains(token, ".") {
		t.Fatalf("an opaque token must not carry a readable payload: %s", token)
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
	if len(token) < 40 {
		t.Fatalf("token is too short to be unguessable: %d chars", len(token))
	}
}

func TestGenerateAccessTokenIsNotPredictable(t *testing.T) {
	seen := make(map[string]bool, 64)
	for i := 0; i < 64; i++ {
		token, err := generateAccessToken()
		if err != nil {
			t.Fatalf("generateAccessToken error: %v", err)
		}
		if seen[token] {
			t.Fatal("generateAccessToken repeated a value")
		}
		seen[token] = true
	}
}

func TestFingerprintIsShortButStable(t *testing.T) {
	hash := hashAccessToken("yomi_access_abc")
	if len(fingerprint(hash)) != 12 {
		t.Fatalf("fingerprint must be 12 chars, got %d", len(fingerprint(hash)))
	}
	if fingerprint(hash) == hash {
		t.Fatal("fingerprint must not expose the whole hash")
	}
	if fingerprint("short") != "short" {
		t.Fatal("short input must pass through unchanged")
	}
}

func TestAccessTokenKeyNamespacesByHash(t *testing.T) {
	if accessTokenKey("abc") != "consent-token:abc" {
		t.Fatalf("unexpected redis key: %s", accessTokenKey("abc"))
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
	for _, s := range CatalogForRole(models.InstitutionRoleViewer) {
		if s.Key == "health.records" || s.Key == "financial.credit" {
			t.Fatalf("viewer catalog leaked %s", s.Key)
		}
	}
	if len(CatalogForRole(models.InstitutionRoleAdmin)) != len(ScopeCatalog()) {
		t.Fatal("admin catalog must expose every scope")
	}
}

func TestScopeJSONIsStable(t *testing.T) {
	if scopeJSON([]string{"identity.basic"}) != `["identity.basic"]` {
		t.Fatalf("unexpected scope json: %s", scopeJSON([]string{"identity.basic"}))
	}
	if scopeJSON(nil) != "null" {
		t.Fatalf("nil scope must marshal deterministically, got %s", scopeJSON(nil))
	}
}

func TestRevokedHashIsRememberedNotForgotten(t *testing.T) {
	req := models.ConsentRequest{ID: "r", CitizenID: "c", DataScope: []string{"a"}, ExpiresAt: time.Now().Add(time.Hour)}
	if req.AccessTokenHash != "" {
		t.Fatal("a request must not carry a token hash until it is approved")
	}
}
