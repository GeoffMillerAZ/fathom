package main

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// init --help must list every harness init accepts, derived from the table
// initTargetsFor parses, not a hand-kept sentence.
func TestInitHelpListsEveryAcceptedHarness(t *testing.T) {
	helpSandbox(t)
	var out, errb strings.Builder
	runWithStdin([]string{"init", "--help"}, strings.NewReader(""), &out, &errb)
	for _, id := range append(harness.IDs(), initHarnessNone) {
		if !strings.Contains(out.String(), id) {
			t.Errorf("init --help does not list accepted harness %q:\n%s", id, out.String())
		}
	}
	for _, id := range []string{"cursor", "copilot"} {
		if !strings.Contains(out.String(), id) {
			t.Errorf("init --help omits %q, which init accepts", id)
		}
	}
}
