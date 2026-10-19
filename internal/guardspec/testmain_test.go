package guardspec

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestMain runs this package's tests isolated from the operator: a
// private HOME, state dir, call log and an unroutable daemon URL, or not
// at all (RT-19, docs/design/responsible-tenant.md §10.4). The FD-5
// project-value test reaches capability and custodian, which carry the
// daemon defaults.
func TestMain(m *testing.M) { testisolate.Main(m) }
