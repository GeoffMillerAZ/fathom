package wiring

// testmain_test.go — mirrors cmd/fathom/testmain_test.go's hermetic-home
// mechanism: since receiptsPath (receipt.go, O63 item 5) defaults to
// internal/statedir.Resolve(root), which falls back to
// $HOME/.local/state/fathom/<repo-id>/ for a repository with no
// FATHOM_STATE_DIR override, a bare `go test` here would otherwise write
// real receipt files under the developer's actual home directory — a
// fresh one per test (each using its own t.TempDir() root, so each hashes
// to its own state dir), never cleaned up. Redirecting HOME to a hermetic,
// binary-lifetime temp dir closes that the same way cmd/fathom already
// does; tests that want their own fresh override still use t.Setenv,
// which saves/restores relative to this hermetic baseline.

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "fathom-wiring-test-home-")
	if err == nil {
		os.Setenv("HOME", home)
	}
	code := m.Run() // os.Exit below would skip a deferred cleanup, so run it inline first
	if home != "" {
		os.RemoveAll(home)
	}
	os.Exit(code)
}
