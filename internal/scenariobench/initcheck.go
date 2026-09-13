// initcheck.go — validates the `fathom init --json` envelope wireScratchDaemon
// runs against a scratch cell (subject.go), so a wiring failure the process
// exit code alone would not surface (init fails open to exit 2 on an
// operational condition, and a law-15 BLOCKED file still exits 1 with
// output the caller must read to understand) is caught before the cell
// proceeds to a session that would silently run unwired.
package scenariobench

import (
	"encoding/json"
	"fmt"
	"strings"
)

// initWriteEnvelope mirrors cmd/fathom's initWriteJSON (init_json.go) —
// duplicated rather than imported because that type lives in package main.
// Only the fields checkInitJSON reads are declared.
type initWriteEnvelope struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Detail string `json:"detail,omitempty"`
}

// initManualStepEnvelope mirrors cmd/fathom's initManualStepJSON.
type initManualStepEnvelope struct {
	Text string `json:"text"`
}

// initEnvelope mirrors cmd/fathom's initJSON envelope (init_json.go) —
// only the fields checkInitJSON reads.
type initEnvelope struct {
	ExitCode    int                      `json:"exit_code"`
	Writes      []initWriteEnvelope      `json:"writes"`
	Blocked     []initWriteEnvelope      `json:"blocked"`
	ManualSteps []initManualStepEnvelope `json:"manual_steps"`
}

// checkInitJSON parses the `fathom init --json` envelope out of combined
// (stdout+stderr) subprocess output and fails unless the run was clean:
// exit_code 0, OR exit_code 2 whose manual steps include init's own
// daemon-down line (printDaemonDownInstructions' "is unreachable" text;
// wireScratchDaemon already waited for the daemon to answer before calling
// init, so this should not fire, but if it does it is init's documented
// operational exit, not a wiring defect). A non-empty `blocked` list or
// any write/blocked entry with action == "error" fails regardless of
// exit_code, since those name a real file init could not wire.
func checkInitJSON(combinedOutput string) error {
	i := strings.IndexByte(combinedOutput, '{')
	if i < 0 {
		return fmt.Errorf("no JSON object found in `fathom init --json` output")
	}
	var env initEnvelope
	dec := json.NewDecoder(strings.NewReader(combinedOutput[i:]))
	if err := dec.Decode(&env); err != nil {
		return fmt.Errorf("parsing `fathom init --json` output: %w", err)
	}

	for _, w := range append(append([]initWriteEnvelope{}, env.Writes...), env.Blocked...) {
		if w.Action == "error" {
			return fmt.Errorf("fathom init reported action \"error\" on %s: %s", w.Path, w.Detail)
		}
	}
	if len(env.Blocked) > 0 {
		names := make([]string, len(env.Blocked))
		for i, b := range env.Blocked {
			names[i] = b.Path
		}
		return fmt.Errorf("fathom init reported %d blocked file(s): %s", len(env.Blocked), strings.Join(names, ", "))
	}

	if env.ExitCode == 0 {
		return nil
	}
	if env.ExitCode == 2 && hasDaemonUnreachableStep(env.ManualSteps) {
		return nil
	}
	return fmt.Errorf("fathom init exited %d (not the daemon-unreachable operational case)", env.ExitCode)
}

// hasDaemonUnreachableStep reports whether steps names init's own
// daemon-down manual step (printDaemonDownInstructions, cmd/fathom/init.go
// — "fathom daemon at %s is unreachable …") — the one exit-2 shape
// wireScratchDaemon accepts, since it already confirmed the daemon answers
// before running `init`, so a genuine report of that at this point is
// init's documented fail-open, not a wiring defect. manual_steps also
// always carries the standard MCP-registration/lefthook lines
// (printManualSteps runs unconditionally, success or not), so this checks
// for the presence of the daemon-down line rather than requiring every
// step to be it. The OTHER exit-2 shape ("ingest ran but service … has no
// symbols") carries no such line and is correctly rejected.
func hasDaemonUnreachableStep(steps []initManualStepEnvelope) bool {
	for _, s := range steps {
		if strings.Contains(s.Text, "is unreachable") {
			return true
		}
	}
	return false
}
