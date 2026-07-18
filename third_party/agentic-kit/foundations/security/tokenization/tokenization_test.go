package tokenization_test

import (
	"strings"
	"testing"
	"time"

	aiprovenance "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"
	"github.com/geoffmilleraz/agentic-kit/foundations/security/tokenization"
)

// ---- helpers ---------------------------------------------------------------

// fixedClock returns a clock pinned to t.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// fixedRand returns a RandFunc that always emits the same byte slice.
// It rotates through bytes to ensure unique tokens when called multiple times.
func fixedRand(seed byte) tokenization.RandFunc {
	calls := 0
	return func(n int) ([]byte, error) {
		calls++
		b := make([]byte, n)
		for i := range b {
			b[i] = seed + byte(calls)
		}
		return b, nil
	}
}

// uniqueRand returns a RandFunc that increments a counter so each call
// produces distinct bytes — needed when creating multiple tokens.
func uniqueRand() tokenization.RandFunc {
	var counter byte
	return func(n int) ([]byte, error) {
		counter++
		b := make([]byte, n)
		for i := range b {
			b[i] = counter + byte(i)
		}
		return b, nil
	}
}

var epoch = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// humanAuthor is a reusable human Author for tests.
var humanAuthor = aiprovenance.NewHuman("test-user@example.com")

// ---- TokenClass enum validation --------------------------------------------

func TestTokenClassValidation(t *testing.T) {
	t.Parallel()

	valid := []tokenization.TokenClass{
		tokenization.ClassPII,
		tokenization.ClassFinancial,
		tokenization.ClassPrivileged,
		tokenization.ClassSensitive,
		tokenization.ClassOther,
	}
	for _, c := range valid {
		if err := c.Validate(); err != nil {
			t.Errorf("class %q should be valid, got: %v", c, err)
		}
	}

	invalid := []tokenization.TokenClass{"", "secret", "PII", "FINANCIAL"}
	for _, c := range invalid {
		if err := c.Validate(); err == nil {
			t.Errorf("class %q should be invalid but Validate returned nil", c)
		}
	}
}

// ---- Token Parse / String round-trip ---------------------------------------

func TestTokenParseRoundTrip(t *testing.T) {
	t.Parallel()

	tz := tokenization.NewTokenizer(
		[]byte("hmac-key"),
		fixedClock(epoch),
		uniqueRand(),
	)
	tok, err := tz.RandomToken(tokenization.ClassPII, "ns1")
	if err != nil {
		t.Fatalf("RandomToken: %v", err)
	}

	s := tok.String()
	parsed, err := tokenization.Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	if parsed.Value != tok.Value {
		t.Errorf("Value mismatch: want %q, got %q", tok.Value, parsed.Value)
	}
	if parsed.Class != tok.Class {
		t.Errorf("Class mismatch: want %q, got %q", tok.Class, parsed.Class)
	}
	// Namespace is NOT encoded in the opaque token string and is intentionally
	// left empty by Parse. Callers must supply namespace from context.
	if parsed.Namespace != "" {
		t.Errorf("Parse must leave Namespace empty (not encoded in token string), got %q", parsed.Namespace)
	}
}

func TestTokenParseInvalidFormat(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"notaprefix_pii_abc",
		"tok__abc",
		"tok_badclass_hex",
	}
	for _, s := range cases {
		if _, err := tokenization.Parse(s); err == nil {
			t.Errorf("Parse(%q) should fail, got nil error", s)
		}
	}
}

// ---- Deterministic token stability ----------------------------------------

func TestDeterministicTokenStable(t *testing.T) {
	t.Parallel()

	key := []byte("test-hmac-key-1234")
	tz1 := tokenization.NewTokenizer(key, fixedClock(epoch), fixedRand(0))
	tz2 := tokenization.NewTokenizer(key, fixedClock(epoch), fixedRand(0))

	tok1 := tz1.DeterministicToken("SSN:123-45-6789", tokenization.ClassPII, "acme")
	tok2 := tz2.DeterministicToken("SSN:123-45-6789", tokenization.ClassPII, "acme")

	if tok1.Value != tok2.Value {
		t.Errorf("same value+namespace+key must produce same token: %q vs %q", tok1.Value, tok2.Value)
	}
}

func TestDeterministicTokenDiffersAcrossNamespace(t *testing.T) {
	t.Parallel()

	key := []byte("test-hmac-key-1234")
	tz := tokenization.NewTokenizer(key, fixedClock(epoch), uniqueRand())

	tok1 := tz.DeterministicToken("SSN:123-45-6789", tokenization.ClassPII, "ns-alpha")
	tok2 := tz.DeterministicToken("SSN:123-45-6789", tokenization.ClassPII, "ns-beta")

	if tok1.Value == tok2.Value {
		t.Error("different namespace must produce different deterministic token")
	}
}

func TestDeterministicTokenDiffersAcrossClass(t *testing.T) {
	t.Parallel()

	key := []byte("test-hmac-key-1234")
	tz := tokenization.NewTokenizer(key, fixedClock(epoch), uniqueRand())

	tok1 := tz.DeterministicToken("4111111111111111", tokenization.ClassPII, "ns")
	tok2 := tz.DeterministicToken("4111111111111111", tokenization.ClassFinancial, "ns")

	if tok1.Value == tok2.Value {
		t.Error("different class must produce different deterministic token")
	}
}

// ---- Random token uniqueness ----------------------------------------------

func TestRandomTokensUnique(t *testing.T) {
	t.Parallel()

	tz := tokenization.NewTokenizer(nil, fixedClock(epoch), uniqueRand())

	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		tok, err := tz.RandomToken(tokenization.ClassSensitive, "ns")
		if err != nil {
			t.Fatalf("RandomToken iteration %d: %v", i, err)
		}
		if seen[tok.Value] {
			t.Fatalf("duplicate token at iteration %d: %q", i, tok.Value)
		}
		seen[tok.Value] = true
	}
}

// ---- Format-preserving helper ---------------------------------------------

func TestFormatPreservingShape(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input   string
		mask    byte
		last4   int
		wantLen int
		suffix  string
	}{
		{"123-45-6789", '*', 4, 11, "6789"},
		{"4111111111111111", '*', 4, 16, "1111"},
		{"secret", '*', 4, 6, "cret"},
	}

	for _, tc := range cases {
		got := tokenization.FormatPreserving(tc.input, tc.mask, tc.last4)
		if len(got) != tc.wantLen {
			t.Errorf("FormatPreserving(%q): want len %d, got %d (%q)", tc.input, tc.wantLen, len(got), got)
		}
		if !strings.HasSuffix(got, tc.suffix) {
			t.Errorf("FormatPreserving(%q): want suffix %q, got %q", tc.input, tc.suffix, got)
		}
		// Verify the non-suffix portion is all mask chars (ignoring separator chars)
		prefix := got[:len(got)-tc.last4]
		for _, r := range prefix {
			if r != rune(tc.mask) && r != '-' && r != ' ' {
				t.Errorf("FormatPreserving(%q): non-suffix char %q not mask or separator", tc.input, r)
			}
		}
	}
}

func TestFormatPreservingShortInput(t *testing.T) {
	t.Parallel()

	// Input shorter than last4 — should not panic, return something sane.
	got := tokenization.FormatPreserving("ab", '*', 4)
	if len(got) != 2 {
		t.Errorf("short input: want len 2, got %d", len(got))
	}
}

// ---- Principal / RolePolicy -----------------------------------------------

func TestRolePolicyAllow(t *testing.T) {
	t.Parallel()

	policy := tokenization.NewRolePolicy(map[string][]tokenization.TokenClass{
		"analyst": {tokenization.ClassPII, tokenization.ClassSensitive},
		"finance": {tokenization.ClassFinancial},
	})

	principal := tokenization.Principal{ID: "u1", Roles: []string{"analyst"}}

	if !policy.Allow(principal, tokenization.ClassPII, "investigation") {
		t.Error("analyst should be allowed to detokenize PII")
	}
	if policy.Allow(principal, tokenization.ClassFinancial, "investigation") {
		t.Error("analyst should NOT be allowed to detokenize financial")
	}
}

func TestRolePolicyDenyUnknownRole(t *testing.T) {
	t.Parallel()

	policy := tokenization.NewRolePolicy(map[string][]tokenization.TokenClass{
		"admin": {tokenization.ClassPrivileged},
	})

	principal := tokenization.Principal{ID: "u2", Roles: []string{"guest"}}
	if policy.Allow(principal, tokenization.ClassPrivileged, "any") {
		t.Error("unknown role must be denied")
	}
}

func TestRolePolicyMultipleRoles(t *testing.T) {
	t.Parallel()

	policy := tokenization.NewRolePolicy(map[string][]tokenization.TokenClass{
		"analyst": {tokenization.ClassPII},
		"finance": {tokenization.ClassFinancial},
	})

	// Principal with both roles must access both classes.
	principal := tokenization.Principal{ID: "u3", Roles: []string{"analyst", "finance"}}
	if !policy.Allow(principal, tokenization.ClassPII, "x") {
		t.Error("multi-role principal should access PII via analyst role")
	}
	if !policy.Allow(principal, tokenization.ClassFinancial, "x") {
		t.Error("multi-role principal should access financial via finance role")
	}
}

// ---- MemoryVault round-trip -----------------------------------------------

func TestMemoryVaultRoundTrip(t *testing.T) {
	t.Parallel()

	policy := tokenization.NewRolePolicy(map[string][]tokenization.TokenClass{
		"admin": {tokenization.ClassPII, tokenization.ClassFinancial, tokenization.ClassPrivileged, tokenization.ClassSensitive, tokenization.ClassOther},
	})
	tz := tokenization.NewTokenizer([]byte("vault-key"), fixedClock(epoch), uniqueRand())
	vault := tokenization.NewMemoryVault(tz, policy, fixedClock(epoch))

	principal := tokenization.Principal{ID: "alice", Roles: []string{"admin"}}

	tok, err := vault.Tokenize("SSN:987-65-4321", tokenization.ClassPII, "case-001", humanAuthor)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	got, err := vault.Detokenize(tok, principal, "legal review")
	if err != nil {
		t.Fatalf("Detokenize: %v", err)
	}
	if got != "SSN:987-65-4321" {
		t.Errorf("round-trip mismatch: want %q, got %q", "SSN:987-65-4321", got)
	}
}

func TestMemoryVaultUnauthorizedDenied(t *testing.T) {
	t.Parallel()

	policy := tokenization.NewRolePolicy(map[string][]tokenization.TokenClass{
		"admin": {tokenization.ClassPII},
	})
	tz := tokenization.NewTokenizer([]byte("vault-key"), fixedClock(epoch), uniqueRand())
	vault := tokenization.NewMemoryVault(tz, policy, fixedClock(epoch))

	tok, err := vault.Tokenize("secret-value", tokenization.ClassPII, "ns", humanAuthor)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	intruder := tokenization.Principal{ID: "eve", Roles: []string{"guest"}}
	_, err = vault.Detokenize(tok, intruder, "snooping")
	if err == nil {
		t.Fatal("unauthorized Detokenize must return an error")
	}
}

// ---- Audit log completeness -----------------------------------------------

func TestAuditLogRecordsBothActions(t *testing.T) {
	t.Parallel()

	policy := tokenization.NewRolePolicy(map[string][]tokenization.TokenClass{
		"admin": {tokenization.ClassPII},
	})
	tz := tokenization.NewTokenizer([]byte("vault-key"), fixedClock(epoch), uniqueRand())
	vault := tokenization.NewMemoryVault(tz, policy, fixedClock(epoch))

	principal := tokenization.Principal{ID: "bob", Roles: []string{"admin"}}
	tok, _ := vault.Tokenize("raw-secret", tokenization.ClassPII, "ns", humanAuthor)
	_, _ = vault.Detokenize(tok, principal, "review")

	log := vault.AuditLog()
	if len(log) < 2 {
		t.Fatalf("expected at least 2 audit events, got %d", len(log))
	}

	var hasTokenize, hasDetokenize bool
	for _, e := range log {
		if e.Action == tokenization.ActionTokenize {
			hasTokenize = true
		}
		if e.Action == tokenization.ActionDetokenize {
			hasDetokenize = true
		}
	}
	if !hasTokenize {
		t.Error("audit log missing tokenize event")
	}
	if !hasDetokenize {
		t.Error("audit log missing detokenize event")
	}
}

func TestAuditLogNeverContainsRawValue(t *testing.T) {
	t.Parallel()

	rawValue := "SUPERSECRET-SSN-123-45-6789"
	policy := tokenization.NewRolePolicy(map[string][]tokenization.TokenClass{
		"admin": {tokenization.ClassPII},
	})
	tz := tokenization.NewTokenizer([]byte("vault-key"), fixedClock(epoch), uniqueRand())
	vault := tokenization.NewMemoryVault(tz, policy, fixedClock(epoch))

	principal := tokenization.Principal{ID: "carol", Roles: []string{"admin"}}
	tok, _ := vault.Tokenize(rawValue, tokenization.ClassPII, "ns", humanAuthor)
	_, _ = vault.Detokenize(tok, principal, "audit-test")

	for _, e := range vault.AuditLog() {
		// Serialize the event fields we control and check none contain raw value.
		fields := []string{e.TokenValue, e.PrincipalID, e.Purpose, string(e.Action), string(e.Class)}
		for _, f := range fields {
			if strings.Contains(f, rawValue) {
				t.Errorf("audit event field contains raw secret value: %q", f)
			}
		}
	}
}

func TestAuditLogRecordsDeniedDetokenize(t *testing.T) {
	t.Parallel()

	policy := tokenization.NewRolePolicy(map[string][]tokenization.TokenClass{
		"admin": {tokenization.ClassPII},
	})
	tz := tokenization.NewTokenizer([]byte("vault-key"), fixedClock(epoch), uniqueRand())
	vault := tokenization.NewMemoryVault(tz, policy, fixedClock(epoch))

	tok, _ := vault.Tokenize("value", tokenization.ClassPII, "ns", humanAuthor)
	intruder := tokenization.Principal{ID: "mallory", Roles: []string{}}
	_, _ = vault.Detokenize(tok, intruder, "probe")

	var deniedFound bool
	for _, e := range vault.AuditLog() {
		if e.Action == tokenization.ActionDetokenize && !e.Allowed {
			deniedFound = true
			if e.PrincipalID != "mallory" {
				t.Errorf("denied event principal want %q, got %q", "mallory", e.PrincipalID)
			}
		}
	}
	if !deniedFound {
		t.Error("no denied detokenize audit event found")
	}
}

// ---- Injected clock determinism -------------------------------------------

func TestInjectedClockDeterministic(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
	policy := tokenization.NewRolePolicy(map[string][]tokenization.TokenClass{
		"r": {tokenization.ClassPII},
	})
	tz := tokenization.NewTokenizer([]byte("k"), fixedClock(fixedTime), uniqueRand())
	vault := tokenization.NewMemoryVault(tz, policy, fixedClock(fixedTime))

	vault.Tokenize("x", tokenization.ClassPII, "ns", humanAuthor) //nolint:errcheck

	for _, e := range vault.AuditLog() {
		if !e.At.Equal(fixedTime) {
			t.Errorf("audit event At want %v, got %v", fixedTime, e.At)
		}
	}
}

// ---- AuditEvent validate ---------------------------------------------------

func TestAuditEventValidate(t *testing.T) {
	t.Parallel()

	good := tokenization.AuditEvent{
		At:          epoch,
		Action:      tokenization.ActionTokenize,
		TokenValue:  "tok_pii_abc123",
		Class:       tokenization.ClassPII,
		PrincipalID: "user1",
		Purpose:     "test",
		Allowed:     true,
	}
	if err := good.Validate(); err != nil {
		t.Errorf("valid AuditEvent: %v", err)
	}

	// Zero At must fail.
	bad := good
	bad.At = time.Time{}
	if err := bad.Validate(); err == nil {
		t.Error("zero At must be invalid")
	}

	// Empty TokenValue must fail.
	bad = good
	bad.TokenValue = ""
	if err := bad.Validate(); err == nil {
		t.Error("empty TokenValue must be invalid")
	}

	// Invalid action must fail.
	bad = good
	bad.Action = "unknown"
	if err := bad.Validate(); err == nil {
		t.Error("invalid Action must be invalid")
	}
}

// ---- Token Validate --------------------------------------------------------

func TestTokenValidate(t *testing.T) {
	t.Parallel()

	good := tokenization.Token{
		Value:     "tok_pii_abc123",
		Class:     tokenization.ClassPII,
		Namespace: "ns",
	}
	if err := good.Validate(); err != nil {
		t.Errorf("valid Token: %v", err)
	}

	bad := good
	bad.Value = ""
	if err := bad.Validate(); err == nil {
		t.Error("empty Value must be invalid")
	}

	bad = good
	bad.Namespace = ""
	if err := bad.Validate(); err == nil {
		t.Error("empty Namespace must be invalid")
	}

	bad = good
	bad.Class = "bad"
	if err := bad.Validate(); err == nil {
		t.Error("invalid Class must be invalid")
	}
}

// ---- Audit log is defensive copy ------------------------------------------

func TestAuditLogIsDefensiveCopy(t *testing.T) {
	t.Parallel()

	policy := tokenization.NewRolePolicy(nil)
	tz := tokenization.NewTokenizer(nil, fixedClock(epoch), uniqueRand())
	vault := tokenization.NewMemoryVault(tz, policy, fixedClock(epoch))

	vault.Tokenize("v", tokenization.ClassOther, "ns", humanAuthor) //nolint:errcheck

	log1 := vault.AuditLog()
	log1[0].PrincipalID = "MUTATED"

	log2 := vault.AuditLog()
	if log2[0].PrincipalID == "MUTATED" {
		t.Error("AuditLog must return defensive copy; mutation leaked back")
	}
}
