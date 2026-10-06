package auth

import (
	"testing"
)

// TestBCryptCompareRoundTrip ensures a bcrypt hash verifies against the
// original password and rejects wrong ones.
func TestBCryptCompareRoundTrip(t *testing.T) {
	password := "correct-horse-battery-staple"
	hash, err := bcryptFromPassword(password)
	if err != nil {
		t.Fatalf("hash error: %v", err)
	}
	if bcryptCompare(hash, password) != nil {
		t.Fatalf("correct password should verify")
	}
	if bcryptCompare(hash, "wrong-password") == nil {
		t.Fatalf("wrong password should not verify")
	}
}

// TestBCryptHashesDiffer ensures two hashes of the same password differ (salt).
func TestBCryptHashesDiffer(t *testing.T) {
	h1, _ := bcryptFromPassword("same-pass")
	h2, _ := bcryptFromPassword("same-pass")
	if h1 == h2 {
		t.Fatalf("bcrypt hashes must differ due to salt")
	}
}

// TestHashTokenDeterministic ensures the same opaque token hashes identically
// (used for Redis key lookup).
func TestHashTokenDeterministic(t *testing.T) {
	token := "abc123opaque"
	if hashToken(token) != hashToken(token) {
		t.Fatalf("hashToken must be deterministic")
	}
	if hashToken(token) == hashToken("different") {
		t.Fatalf("distinct tokens must hash differently")
	}
}

// TestGenerateOpaqueToken ensures a random opaque token is returned with the
// requested byte length and is URL-safe.
func TestGenerateOpaqueToken(t *testing.T) {
	t1, err := generateOpaqueToken(32)
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}
	t2, _ := generateOpaqueToken(32)
	if t1 == "" || t1 == t2 {
		t.Fatalf("expected distinct non-empty tokens")
	}
	for _, r := range t1 {
		if !isURLSafe(r) {
			t.Fatalf("token contains unsafe char: %q", r)
		}
	}
}

func isURLSafe(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
}
