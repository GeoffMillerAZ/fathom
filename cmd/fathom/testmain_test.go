package main

// testmain_test.go — a package-wide safety net for user-scope config
// (docs/design/scoped-config.md) and session-instrument (production-
// tokenomics.md) code paths that default to $HOME (connector.UserIgnorePath,
// guard.go's loadUserGuardSet, sessionledger.go's sessionLedgerPath). Both
// families default to real $HOME/~/.fathom/* when no override is present —
// exactly the repo-wide "tests must not write to the real $HOME" rule.
//
// Rather than require every test in this large, pre-existing package to
// individually t.Setenv("HOME", t.TempDir()), TestMain sets one hermetic
// HOME for the whole binary before any test runs. A test that needs its
// OWN fresh $HOME still calls t.Setenv("HOME", t.TempDir()) exactly as
// before — t.Setenv saves/restores relative to whatever HOME already is
// (this hermetic one, never the real one), so per-test overrides remain
// fully correct and this change is purely additive safety.
import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "fathom-cmd-test-home-")
	if err == nil {
		os.Setenv("HOME", home)
	}
	code := m.Run() // os.Exit below would skip a deferred cleanup, so run it inline first
	if home != "" {
		os.RemoveAll(home)
	}
	os.Exit(code)
}
