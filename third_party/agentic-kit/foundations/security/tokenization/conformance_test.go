package tokenization_test

// RunShredConformance's real implementation now lives in the importable
// foundations/security/tokenization/vaulttest package (H3 Stage 2, agentic-kit
// handoff-h3-throughline.md) so downstream adapters (e.g. throughline's
// SQLite-backed vault) can import it from their own test files. The previous
// doc comment here claimed this _test.go file itself was "exported so
// downstream consumers can run it" — that was wrong: a _test.go file compiles
// only for this package's own `go test` and cannot be imported across module
// boundaries at all, which is exactly the friction that motivated extracting
// vaulttest. This file is now the "thin caller" the spec asks for: a
// same-package alias + delegation so existing internal callers
// (encrypted_vault_test.go's bare `RunShredConformance(t, factory)` calls)
// keep compiling unchanged.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/tokenization"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/tokenization/vaulttest"
)

// VaultFactory aliases vaulttest.VaultFactory so existing callers in this
// package need no changes.
type VaultFactory = vaulttest.VaultFactory

// RunShredConformance delegates to vaulttest.RunShredConformance.
func RunShredConformance(t *testing.T, factory VaultFactory) {
	t.Helper()
	vaulttest.RunShredConformance(t, factory)
}

// compile-time check that the alias round-trips through the real factory
// signature (defensive: catches a signature drift between the two packages
// at build time rather than at first test invocation).
var _ vaulttest.VaultFactory = VaultFactory(func() (tokenization.ShreddableVault, *tokenization.Tokenizer) {
	return nil, nil
})
