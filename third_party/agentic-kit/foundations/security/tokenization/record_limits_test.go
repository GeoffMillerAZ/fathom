package tokenization_test

import (
	"errors"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/tokenization"
)

func TestEncryptedRecordsBoundedAndKeyed(t *testing.T) {
	for _, limits := range [][2]int{{0, 1}, {1, 0}, {-1, 1}} {
		if _, err := tokenization.NewMemoryRecordStoreWithLimits(limits[0], limits[1]); err == nil {
			t.Fatal("invalid limits accepted")
		}
	}
	store, err := tokenization.NewMemoryRecordStoreWithLimits(1, 4096)
	if err != nil {
		t.Fatal(err)
	}
	keys := tokenization.NewMemoryKeyStoreWithRandom(counterRandom())
	v := recordVault(keys, store)
	first, err := v.TokenizeDeterministic("first", tokenization.ClassPII, "ns", encVaultAuthor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.TokenizeDeterministic("second", tokenization.ClassPII, "ns", encVaultAuthor); !errors.Is(err, tokenization.ErrRecordLimit) {
		t.Fatalf("count limit: %v", err)
	}
	if again, err := v.TokenizeDeterministic("first", tokenization.ClassPII, "ns", encVaultAuthor); err != nil || again != first {
		t.Fatal("existing binding lost at capacity")
	}
	bytes, err := tokenization.NewMemoryRecordStoreWithLimits(100, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recordVault(keys, bytes).Tokenize("first", tokenization.ClassPII, "ns", encVaultAuthor); !errors.Is(err, tokenization.ErrRecordLimit) {
		t.Fatalf("byte limit: %v", err)
	}
	clock := fixedClock(time.Unix(123, 0))
	tz := tokenization.NewTokenizer(nil, clock, counterRandom())
	policy := tokenization.NewRolePolicy(map[string][]tokenization.TokenClass{"admin": {tokenization.ClassPII}})
	unkeyed := tokenization.NewEncryptedVault(tz, keys, policy, clock)
	if _, err := unkeyed.TokenizeDeterministic("guessable", tokenization.ClassPII, "ns", encVaultAuthor); err == nil {
		t.Fatal("empty HMAC key accepted")
	}
	if log := unkeyed.AuditLog(); len(log) != 1 || log[0].Allowed {
		t.Fatal("failed mint not audited denied")
	}
}
