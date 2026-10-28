package benchprep

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestMain runs the package under testisolate (RT-19, §10.4): SplitHeldOut
// takes scenariobench tasks, and that import reaches the daemon, state and
// model-CLI defaults.
func TestMain(m *testing.M) { testisolate.Main(m) }
