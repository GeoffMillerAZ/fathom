package reviewbench

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestMain runs the package under testisolate (RT-19, §10.4): it imports pkg/client, which reaches the operator's intent ledger by default (internal/intent, through internal/certify).
func TestMain(m *testing.M) { testisolate.Main(m) }
