package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/readout"
)

// TestReadoutRichServesThroughThePolicyTable: rich mode's enrichment goes
// through the policy table and nowhere else. A failing build whose errors
// name nothing the session edited or the working tree changed is raw (rule
// 1, contested) and its compile line is held back; one naming an edited
// file is enriched. Each row names the table's answer with decided_by
// table, and a fitted policy file is named on the row when it answers.
func TestReadoutRichServesThroughThePolicyTable(t *testing.T) {
	repo := t.TempDir()
	readoutGitRepo(t, repo, map[string]string{
		"go.mod":         "module example.com/shop\n\ngo 1.22\n",
		"money/money.go": "package money\n\nfunc Round(x int) int { return x }\n",
	})
	origSet := editSetDir
	editSetDir = filepath.Join(t.TempDir(), "edit-set")
	t.Cleanup(func() { editSetDir = origSet })
	payload := compileFailureEnvelope(t, t.TempDir(), "cd "+repo+" && go build ./...", compileErrorsOutput)

	t.Run("nothing edited: raw, the line held back", func(t *testing.T) {
		readoutHookEnv(t, "[readouts]\nmode = \"rich\"\n")
		code, out, _, _ := runReadoutHook(t, []string{"--event", "tool-failure"}, payload)
		if code != 0 || out != "" {
			t.Fatalf("exit %d, stdout %q; the table answered raw, so nothing is injected", code, out)
		}
		rows := readoutRows(t)
		if len(rows) != 1 {
			t.Fatalf("rows = %+v", rows)
		}
		m := rows[0].Meta
		for k, want := range map[string]string{
			"policy": readout.ChoiceRaw, "policy_rule": "1", "decided_by": "table", "contested": "true",
			"edited_overlap": "0", "rich_withheld": "compile_error", "records": "compile_error:8", "emitted": "false",
		} {
			if m[k] != want {
				t.Errorf("row %s = %q, want %q (meta %v)", k, m[k], want, m)
			}
		}
		if m["policy_cuts"] != "" {
			t.Errorf("no policy file, yet the row says %q", m["policy_cuts"])
		}
	})

	t.Run("an edited file named: enriched, the fitted cut points named", func(t *testing.T) {
		readoutHookEnv(t, "[readouts]\nmode = \"rich\"\n")
		if err := readout.WritePolicy(os.Getenv("FATHOM_HOME"), readout.FitResult{Seed: 3, Policy: readout.Policy{LeanFloorBytes: 5000, LongReadLines: 300}}); err != nil {
			t.Fatal(err)
		}
		markFileEdited("s-1", "cart/cart.go", time.Now())
		code, out, _, _ := runReadoutHook(t, []string{"--event", "tool-failure"}, payload)
		if code != 0 || !strings.Contains(out, "go build: 8 errors") {
			t.Fatalf("exit %d, stdout %q; want the compile line injected", code, out)
		}
		m := readoutRows(t)[0].Meta
		for k, want := range map[string]string{
			"policy": readout.ChoiceEnriched, "policy_rule": "1", "decided_by": "table", "policy_cuts": "fitted",
			"edited_overlap": "1", "rich": "compile_error",
		} {
			if m[k] != want {
				t.Errorf("row %s = %q, want %q (meta %v)", k, m[k], want, m)
			}
		}
	})
}
