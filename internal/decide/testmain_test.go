package decide_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestMain runs the package under testisolate (RT-19, §10.4): its tests
// read decisions.jsonl rows through internal/console's decide view, which
// reaches the daemon and call-log defaults.
func TestMain(m *testing.M) { testisolate.Main(m) }
