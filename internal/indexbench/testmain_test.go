package indexbench

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestMain runs the package under testisolate (RT-19, §10.4): its tests
// reach the operator's intent ledger by default (internal/intent).
func TestMain(m *testing.M) { testisolate.Main(m) }
