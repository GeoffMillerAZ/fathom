package tui_test

import (
	"testing"
	"time"

	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/tui"
)

// TestNewTestQuitsOnStart exercises testsupport.go's NewTest directly (not
// just Run): a model that quits from Init should finish promptly under
// the kit's pinned 80x24/Ascii defaults.
func TestNewTestQuitsOnStart(t *testing.T) {
	tm := tui.NewTest(t, quitOnStartModel{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	final := tm.FinalModel(t)
	if _, ok := final.(quitOnStartModel); !ok {
		t.Fatalf("final model type = %T, want quitOnStartModel", final)
	}
}
