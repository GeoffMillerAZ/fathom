package iacfacts

// testmain_test.go — the package runs under testisolate (RT-19, §10.4): an
// isolated HOME, state dir and call log, or it does not run.
import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

func TestMain(m *testing.M) { testisolate.Main(m) }
