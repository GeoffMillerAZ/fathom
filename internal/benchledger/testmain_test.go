package benchledger

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestMain runs the package under testisolate (RT-19, §10.4): its
// scenariobench import reaches daemon, state and model-CLI defaults.
func TestMain(m *testing.M) { testisolate.Main(m) }
