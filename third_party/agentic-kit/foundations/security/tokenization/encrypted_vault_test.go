package tokenization_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/tokenization"
)

// newEncryptedTestVault constructs an EncryptedVault backed by an allow-all
// policy, a deterministic counter-based random source, and a fixed clock.
func newEncryptedTestVault(t *testing.T) (*tokenization.EncryptedVault, *tokenization.Tokenizer, *tokenization.MemoryKeyStore) {
	t.Helper()
	clock := func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	var (
		mu  sync.Mutex
		ctr byte
	)
	randFn := func(n int) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		ctr++
		b := make([]byte, n)
		for i := range b {
			b[i] = ctr + byte(i)
		}
		return b, nil
	}
	ks := tokenization.NewMemoryKeyStore()
	policy := tokenization.NewRolePolicy(map[string][]tokenization.TokenClass{
		"admin": {
			tokenization.ClassPII,
			tokenization.ClassFinancial,
			tokenization.ClassPrivileged,
			tokenization.ClassSensitive,
			tokenization.ClassOther,
		},
	})
	tz := tokenization.NewTokenizer([]byte("conf-hmac-key"), clock, randFn)
	vault := tokenization.NewEncryptedVault(tz, ks, policy, clock)
	return vault, tz, ks
}

var (
	encVaultAuthor    = aiprovenance.NewHuman("ev-test@example.com")
	encAdminPrincipal = tokenization.Principal{ID: "admin", Roles: []string{"admin"}}
)

// ---- RunShredConformance via EncryptedVault -----------------------------------

// TestEncryptedVaultShredConformance runs the canonical conformance suite against
// the EncryptedVault + MemoryKeyStore combination that ships in this package.
func TestEncryptedVaultShredConformance(t *testing.T) {
	RunShredConformance(t, func() (tokenization.ShreddableVault, *tokenization.Tokenizer) {
		vault, tz, _ := newEncryptedTestVault(t)
		return vault, tz
	})
}

// ---- EncryptedVault unit tests -----------------------------------------------

func TestEncryptedVaultRoundTrip(t *testing.T) {
	t.Parallel()

	vault, _, _ := newEncryptedTestVault(t)

	tok, err := vault.Tokenize("SSN:123-45-6789", tokenization.ClassPII, "ns1", encVaultAuthor)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	got, err := vault.Detokenize(tok, encAdminPrincipal, "test")
	if err != nil {
		t.Fatalf("Detokenize: %v", err)
	}
	if got != "SSN:123-45-6789" {
		t.Errorf("round-trip: want %q, got %q", "SSN:123-45-6789", got)
	}
}

func TestEncryptedVaultUnauthorizedDenied(t *testing.T) {
	t.Parallel()

	vault, _, _ := newEncryptedTestVault(t)
	tok, err := vault.Tokenize("secret", tokenization.ClassPII, "ns", encVaultAuthor)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	intruder := tokenization.Principal{ID: "eve", Roles: []string{"guest"}}
	_, err = vault.Detokenize(tok, intruder, "probe")
	if err == nil {
		t.Fatal("unauthorized Detokenize must return an error")
	}
	if errors.Is(err, tokenization.ErrShredded) {
		t.Error("unauthorized access must not return ErrShredded")
	}
}

func TestEncryptedVaultShredThenDetokenize(t *testing.T) {
	t.Parallel()

	vault, _, _ := newEncryptedTestVault(t)
	tok, err := vault.Tokenize("sensitive-value", tokenization.ClassPII, "ns-shred", encVaultAuthor)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	if err := vault.Shred("ns-shred"); err != nil {
		t.Fatalf("Shred: %v", err)
	}

	_, err = vault.Detokenize(tok, encAdminPrincipal, "after-shred")
	if !errors.Is(err, tokenization.ErrShredded) {
		t.Errorf("Detokenize after shred: want ErrShredded, got %v", err)
	}
}

func TestEncryptedVaultShredDoesNotAffectOtherNamespaces(t *testing.T) {
	t.Parallel()

	vault, _, _ := newEncryptedTestVault(t)

	tokA, err := vault.Tokenize("value-A", tokenization.ClassPII, "ns-A", encVaultAuthor)
	if err != nil {
		t.Fatalf("Tokenize ns-A: %v", err)
	}
	tokB, err := vault.Tokenize("value-B", tokenization.ClassPII, "ns-B", encVaultAuthor)
	if err != nil {
		t.Fatalf("Tokenize ns-B: %v", err)
	}

	if err := vault.Shred("ns-A"); err != nil {
		t.Fatalf("Shred ns-A: %v", err)
	}

	_, err = vault.Detokenize(tokA, encAdminPrincipal, "test")
	if !errors.Is(err, tokenization.ErrShredded) {
		t.Errorf("ns-A: want ErrShredded, got %v", err)
	}

	got, err := vault.Detokenize(tokB, encAdminPrincipal, "test")
	if err != nil {
		t.Errorf("ns-B: unexpected error: %v", err)
	}
	if got != "value-B" {
		t.Errorf("ns-B round-trip: want %q, got %q", "value-B", got)
	}
}

func TestEncryptedVaultAuditLogNeverContainsRawValue(t *testing.T) {
	t.Parallel()

	vault, _, _ := newEncryptedTestVault(t)
	rawValue := "SUPERSECRET-SSN-123-45-6789"

	tok, err := vault.Tokenize(rawValue, tokenization.ClassPII, "ns-audit", encVaultAuthor)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	_, _ = vault.Detokenize(tok, encAdminPrincipal, "audit-test")

	for _, e := range vault.AuditLog() {
		fields := []string{e.TokenValue, e.PrincipalID, e.Purpose, string(e.Action), string(e.Class)}
		for _, f := range fields {
			if f == rawValue || len(f) > 0 && f == rawValue {
				t.Errorf("audit event field contains raw secret: %q", f)
			}
		}
	}
}

func TestEncryptedVaultMultipleTokensIndependent(t *testing.T) {
	t.Parallel()

	vault, _, _ := newEncryptedTestVault(t)

	values := []string{"alpha", "beta", "gamma"}
	tokens := make([]tokenization.Token, len(values))
	for i, v := range values {
		tok, err := vault.Tokenize(v, tokenization.ClassPII, "ns-multi", encVaultAuthor)
		if err != nil {
			t.Fatalf("Tokenize %q: %v", v, err)
		}
		tokens[i] = tok
	}

	// Verify all round-trip correctly.
	for i, tok := range tokens {
		got, err := vault.Detokenize(tok, encAdminPrincipal, "test")
		if err != nil {
			t.Errorf("Detokenize %d: %v", i, err)
			continue
		}
		if got != values[i] {
			t.Errorf("round-trip %d: want %q, got %q", i, values[i], got)
		}
	}
}

func TestEncryptedVaultShredIdempotent(t *testing.T) {
	t.Parallel()

	vault, _, _ := newEncryptedTestVault(t)
	_, err := vault.Tokenize("v", tokenization.ClassPII, "ns-idem", encVaultAuthor)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	if err := vault.Shred("ns-idem"); err != nil {
		t.Fatalf("first Shred: %v", err)
	}
	if err := vault.Shred("ns-idem"); err != nil {
		t.Errorf("second Shred (idempotent): %v", err)
	}
}

func TestEncryptedVaultNilPanics(t *testing.T) {
	assertPanic := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s: expected panic, got none", name)
			}
		}()
		fn()
	}

	clock := func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	tz := tokenization.NewTokenizer(nil, clock, func(n int) ([]byte, error) { return make([]byte, n), nil })
	ks := tokenization.NewMemoryKeyStore()
	policy := tokenization.NewRolePolicy(nil)

	assertPanic("nil tz", func() {
		tokenization.NewEncryptedVault(nil, ks, policy, clock)
	})
	assertPanic("nil ks", func() {
		tokenization.NewEncryptedVault(tz, nil, policy, clock)
	})
	assertPanic("nil policy", func() {
		tokenization.NewEncryptedVault(tz, ks, nil, clock)
	})
	assertPanic("nil clock", func() {
		tokenization.NewEncryptedVault(tz, ks, policy, nil)
	})
}

// TestEncryptedVaultMemoryKeyStoreShredded tests that MemoryKeyStore.Key returns
// ErrShredded after Shred, and that Rotate re-activates it.
func TestMemoryKeyStoreLifecycle(t *testing.T) {
	t.Parallel()

	ks := tokenization.NewMemoryKeyStore()
	const ns = "test-ns"

	// First Key call mints a new key.
	k1, err := ks.Key(ns)
	if err != nil {
		t.Fatalf("Key (first): %v", err)
	}
	if len(k1) != 32 {
		t.Errorf("key length: want 32, got %d", len(k1))
	}

	// Second Key call returns the same key (stable).
	k2, err := ks.Key(ns)
	if err != nil {
		t.Fatalf("Key (second): %v", err)
	}
	for i := range k1 {
		if k1[i] != k2[i] {
			t.Errorf("key not stable: k1[%d]=%02x k2[%d]=%02x", i, k1[i], i, k2[i])
			break
		}
	}

	// Shred destroys the key.
	if err := ks.Shred(ns); err != nil {
		t.Fatalf("Shred: %v", err)
	}
	_, err = ks.Key(ns)
	if !errors.Is(err, tokenization.ErrShredded) {
		t.Errorf("Key after Shred: want ErrShredded, got %v", err)
	}

	// Rotate re-activates with a new key.
	if err := ks.Rotate(ns); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	k3, err := ks.Key(ns)
	if err != nil {
		t.Fatalf("Key after Rotate: %v", err)
	}
	if len(k3) != 32 {
		t.Errorf("rotated key length: want 32, got %d", len(k3))
	}
}
