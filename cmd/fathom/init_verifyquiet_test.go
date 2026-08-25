package main

// init_verifyquiet_test.go — VS-8 (docs/plans/verify-spine.md / O19 / G14):
// the lefthook pre-push verify step `fathom init` prints as MANUAL guidance
// for a consumer repo must carry --quiet, matching this repo's own
// lefthook.yml pre-push verify step — a gate run over many plans should read
// as its exceptions, not a wall of checkmarks, in every repo fathom wires,
// not just this one.

import (
	"bytes"
	"strings"
	"testing"
)

func TestInitPrintsVerifyGateWithQuiet(t *testing.T) {
	var out bytes.Buffer
	printManualSteps(&out, "myservice", "http://127.0.0.1:7575")
	got := out.String()

	if !strings.Contains(got, "bin/fathom verify --service myservice --plan docs/plans/<plan>.md --quiet") {
		t.Errorf("MANUAL verify gate line must carry --quiet:\n%s", got)
	}
}
