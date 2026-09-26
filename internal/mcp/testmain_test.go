package mcp

// testmain_test.go — the package runs under testisolate (RT-19, §10.4):
// an isolated HOME (read_source calls connector.LoadIgnore, which defaults
// to $HOME/.fathom/ignore — critic finding F6), state dir and call log,
// or it does not run. Before, a failed MkdirTemp silently kept the real
// HOME.
import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

func TestMain(m *testing.M) { testisolate.Main(m) }
