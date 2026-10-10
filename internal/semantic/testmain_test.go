package semantic

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestMain runs the suite under the isolating entry point (TQ-22/TQ-23):
// eval_test.go imports the claudecli client, so a stubbed PATH must stand
// between these tests and a real claude/codex.
func TestMain(m *testing.M) { testisolate.Main(m) }
