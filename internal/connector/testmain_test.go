package connector_test

// testmain_test.go — a package-wide safety net for scoped config
// (docs/design/scoped-config.md): LocalConnector.Discover and the exported
// LoadIgnore both default to $HOME/.fathom/ignore (userignore.go's
// UserIgnorePath) when no override is present. Critic finding F6
// (BLOCKING, phase-P fix round): this package had NO TestMain, and only
// userignore_test.go's own cases set $HOME — every OTHER pre-existing
// Discover-exercising test (local_test.go, faux_test.go, watermark_test.go)
// reads the REAL machine's ~/.fathom/ignore. On any machine where the
// operator ran `fathom init --seed-user-ignore` (this same phase's own
// feature), those fixtures could start silently losing files (an
// unexpected pattern excludes a fixture file), or — on a machine with a
// single `!` line in ~/.fathom/ignore — Discover would ERROR outright
// (the loud enforcement point, deliberately) and fail the whole suite.
//
// TestMain sets one hermetic $HOME for the whole binary before any test
// runs, exactly mirroring cmd/fathom/testmain_test.go's fix for the same
// class of problem. A test that needs its OWN fresh $HOME still calls
// t.Setenv("HOME", ...) as before — t.Setenv saves/restores relative to
// whatever HOME already is (this hermetic one, never the real one), so
// per-test overrides remain fully correct.
import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "fathom-connector-test-home-")
	if err == nil {
		os.Setenv("HOME", home)
	}
	code := m.Run() // os.Exit below would skip a deferred cleanup, so run it inline first
	if home != "" {
		os.RemoveAll(home)
	}
	os.Exit(code)
}
