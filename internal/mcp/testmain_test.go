package mcp

// testmain_test.go — a package-wide safety net for scoped config
// (docs/design/scoped-config.md): read_source (readsource.go) calls
// connector.LoadIgnore, which defaults to $HOME/.fathom/ignore when no
// override is present. Critic finding F6 (BLOCKING, phase-P fix round):
// mirrors internal/connector/testmain_test.go's own fix for the identical
// class of problem — without this, read_source's tests read the REAL
// machine's ~/.fathom/ignore, which could silently change which fixture
// files a test sees on any machine where the operator ran `fathom init
// --seed-user-ignore`.
import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "fathom-mcp-test-home-")
	if err == nil {
		os.Setenv("HOME", home)
	}
	code := m.Run() // os.Exit below would skip a deferred cleanup, so run it inline first
	if home != "" {
		os.RemoveAll(home)
	}
	os.Exit(code)
}
