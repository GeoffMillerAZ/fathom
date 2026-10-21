package testevidence

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestMain isolates daemon, state and call-log defaults reached by package tests.
func TestMain(m *testing.M) { testisolate.Main(m) }
