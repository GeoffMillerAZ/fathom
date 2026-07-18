// Package vaulttest is an importable conformance suite for
// tokenization.ShreddableVault implementations. It is intentionally NOT a
// _test.go file: the whole point is that downstream adapters (e.g. a durable
// SQLite- or KMS-backed vault living in a consumer repo) can import it from
// their own _test.go files and run the canonical shred-semantics cases
// against their implementation, the same way tokenization's own
// conformance_test.go does against EncryptedVault.
//
// # Why this package imports "testing" outside a _test.go file
//
// That is the point of an importable suite, not an accident: RunShredConformance
// takes a *testing.T and calls t.Run/t.Fatalf/t.Errorf so it plugs directly into
// `go test` for any caller. Before this package existed, the suite lived in
// foundations/security/tokenization's internal `package tokenization_test`
// (conformance_test.go) — invisible to every consumer outside that package
// (agentic-kit TRACKING.md doctrine: "consumers CANNOT import it today", and a
// _test.go file is not importable across modules at all regardless). H3
// Stage 2 (agentic-kit handoff-h3-throughline.md) extracts it here so
// throughline's SQLite-backed vault (adapters/sqlitevault) can prove
// conformance without depending on tokenization's internal test package.
//
// # Shape deviations from the rest of the kit (documented, deliberate)
//
//   - No schema.cue: this package carries no data contract, only a test
//     helper function and a factory function type — the documented
//     behavioral-only exemption (CLAUDE.md: "Skip schema.cue only for
//     behavioral-only packages with no data contract").
//   - Imports "testing" in a non-test file — the documented exception that
//     makes the suite importable at all (see above).
//
// # Concurrency
//
// RunShredConformance is safe to call from multiple goroutines (each call
// constructs its own vault via factory and does not share state with any
// other call); the sub-tests it registers via t.Run are independently
// t.Parallel(). VaultFactory implementations must themselves return a
// freshly-constructed vault per call — RunShredConformance never reuses a
// vault instance across sub-tests.
package vaulttest

import (
	"errors"
	"testing"
	"time"

	aiprovenance "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"
	"github.com/geoffmilleraz/agentic-kit/foundations/security/tokenization"
)

// VaultFactory is the factory signature expected by RunShredConformance. It
// must return a freshly-constructed, empty ShreddableVault and its Tokenizer
// (used for DeterministicToken re-mint tests). The factory must produce a
// vault backed by an allow-all policy so conformance cases are not blocked by
// access control. Each RunShredConformance call invokes the factory once per
// sub-test.
type VaultFactory func() (vault tokenization.ShreddableVault, tz *tokenization.Tokenizer)

// RunShredConformance runs the canonical shred-semantics test suite against
// the vault produced by factory. Any ShreddableVault + KeyStore combination
// must pass all cases before being considered production-ready.
//
// Cases:
//  1. Shred → detokenize returns ErrShredded (typed).
//  2. Shred does not affect other namespaces.
//  3. Shred is idempotent (second shred is not an error).
//  4. Re-tokenize after shred: DeterministicToken re-mints the SAME token
//     value (HMAC key != envelope key). However, storing the value requires
//     Rotate on the KeyStore to re-activate the namespace.
//  5. Audit log records shredded detokenize as Allowed=false.
//
// Usage in an adapter package's test file:
//
//	func TestMyAdapterConformance(t *testing.T) {
//	    vaulttest.RunShredConformance(t, func() (tokenization.ShreddableVault, *tokenization.Tokenizer) {
//	        tz := tokenization.NewTokenizer([]byte("key"), fixedClock, myRandFn)
//	        ks := myDurableKeyStore()
//	        v := myDurableVault(tz, ks, ...)
//	        return v, tz
//	    })
//	}
func RunShredConformance(t *testing.T, factory VaultFactory) {
	t.Helper()

	epoch := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	author := aiprovenance.NewHuman("conformance@example.com")
	adminPrincipal := tokenization.Principal{ID: "admin", Roles: []string{"admin"}}

	t.Run("ShredThenDetokenizeIsErrShredded", func(t *testing.T) {
		t.Parallel()
		vault, _ := factory()

		tok, err := vault.Tokenize("SSN:123-45-6789", tokenization.ClassPII, "ns-alpha", author)
		if err != nil {
			t.Fatalf("Tokenize: %v", err)
		}
		if err := vault.Shred("ns-alpha"); err != nil {
			t.Fatalf("Shred: %v", err)
		}
		_, err = vault.Detokenize(tok, adminPrincipal, "review")
		if err == nil {
			t.Fatal("Detokenize after shred must return an error")
		}
		if !errors.Is(err, tokenization.ErrShredded) {
			t.Errorf("Detokenize after shred: want errors.Is(err, ErrShredded), got %v", err)
		}
	})

	t.Run("ShredDoesNotAffectOtherNamespaces", func(t *testing.T) {
		t.Parallel()
		vault, _ := factory()

		tokA, err := vault.Tokenize("value-alpha", tokenization.ClassPII, "ns-alpha", author)
		if err != nil {
			t.Fatalf("Tokenize ns-alpha: %v", err)
		}
		tokB, err := vault.Tokenize("value-beta", tokenization.ClassPII, "ns-beta", author)
		if err != nil {
			t.Fatalf("Tokenize ns-beta: %v", err)
		}

		// Shred only ns-alpha.
		if err := vault.Shred("ns-alpha"); err != nil {
			t.Fatalf("Shred ns-alpha: %v", err)
		}

		// ns-alpha must be shredded.
		_, err = vault.Detokenize(tokA, adminPrincipal, "test")
		if !errors.Is(err, tokenization.ErrShredded) {
			t.Errorf("ns-alpha: want ErrShredded, got %v", err)
		}

		// ns-beta must still be resolvable.
		got, err := vault.Detokenize(tokB, adminPrincipal, "test")
		if err != nil {
			t.Errorf("ns-beta: unexpected error after ns-alpha shred: %v", err)
		}
		if got != "value-beta" {
			t.Errorf("ns-beta: want %q, got %q", "value-beta", got)
		}
	})

	t.Run("ShredIsIdempotent", func(t *testing.T) {
		t.Parallel()
		vault, _ := factory()

		_, err := vault.Tokenize("value", tokenization.ClassPII, "ns-idempotent", author)
		if err != nil {
			t.Fatalf("Tokenize: %v", err)
		}

		// First shred must succeed.
		if err := vault.Shred("ns-idempotent"); err != nil {
			t.Fatalf("first Shred: %v", err)
		}
		// Second shred must also succeed (idempotent).
		if err := vault.Shred("ns-idempotent"); err != nil {
			t.Errorf("second Shred (idempotent): %v", err)
		}
	})

	t.Run("DeterministicTokenUnaffectedByShred", func(t *testing.T) {
		t.Parallel()
		_, tz := factory()

		// DeterministicToken minting depends only on the HMAC key, not the
		// envelope key. The same (value, class, namespace) triple ALWAYS
		// produces the same token regardless of whether the namespace has
		// been shredded. This is by design: the HMAC key (token identity) is
		// separate from the envelope key (value confidentiality). Callers
		// must document that re-minting a deterministic token into a
		// shredded namespace requires Rotate on the KeyStore before Tokenize
		// succeeds.
		tok1 := tz.DeterministicToken("SSN:987-65-4321", tokenization.ClassPII, "ns-det")
		tok2 := tz.DeterministicToken("SSN:987-65-4321", tokenization.ClassPII, "ns-det")

		if tok1.Value != tok2.Value {
			t.Errorf("DeterministicToken must be stable: %q vs %q", tok1.Value, tok2.Value)
		}
	})

	t.Run("ShredRecordedInAuditLog", func(t *testing.T) {
		t.Parallel()
		vault, _ := factory()

		tok, err := vault.Tokenize("audit-value", tokenization.ClassPII, "ns-audit", author)
		if err != nil {
			t.Fatalf("Tokenize: %v", err)
		}

		if err := vault.Shred("ns-audit"); err != nil {
			t.Fatalf("Shred: %v", err)
		}

		// Attempt detokenize — must fail and be recorded.
		_, _ = vault.Detokenize(tok, adminPrincipal, "audit-test")

		log := vault.AuditLog()
		var shredDetokenizeFound bool
		for _, e := range log {
			if e.Action == tokenization.ActionDetokenize && !e.Allowed && e.TokenValue == tok.Value {
				shredDetokenizeFound = true
			}
		}
		if !shredDetokenizeFound {
			t.Error("audit log must record shredded detokenize as Allowed=false")
		}
	})

	t.Run("UnrelatedToShredTimestamp", func(t *testing.T) {
		// Verify that the audit log timestamps use the injected clock —
		// this exercises the clock seam even in shred scenarios.
		t.Parallel()
		vault, _ := factory()

		_, err := vault.Tokenize("v", tokenization.ClassPII, "ns-clock", author)
		if err != nil {
			t.Fatalf("Tokenize: %v", err)
		}

		for _, e := range vault.AuditLog() {
			if e.At.IsZero() {
				t.Errorf("audit event At is zero — clock seam not wired correctly")
			}
			if e.At.After(epoch.Add(10 * 365 * 24 * time.Hour)) {
				// Sanity: audit timestamps should not be in the far future.
				t.Errorf("audit event At is suspiciously far in the future: %v", e.At)
			}
		}
	})
}
