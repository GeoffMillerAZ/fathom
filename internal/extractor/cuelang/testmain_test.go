package cuelang

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestMain runs the package under testisolate: only its tests reach the daemon defaults, through fusion and support.
func TestMain(m *testing.M) { testisolate.Main(m) }
