package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/readout"
)

func runReadoutFit(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := cmdReadout(append([]string{"fit"}, args...), strings.NewReader(""), &out, &errb)
	return code, out.String(), errb.String()
}

// TestReadoutFitLabelsTheReplayAndWritesOnlyWithWrite: `fathom readout fit`
// labels the hook's served readouts from the transcripts (a drop the next
// calls name: lean was wrong; none named: lean was right), counts a row
// whose transcript is gone apart, gives the same answer for the same seed,
// prints both halves with decided_by table, writes the policy file only
// with --write, and doctor names the fitted file, or an invalid one with
// the built-ins standing.
func TestReadoutFitLabelsTheReplayAndWritesOnlyWithWrite(t *testing.T) {
	fh := readoutHookEnv(t, "[readouts]\nmode = \"observe\"\n")
	cwd, root := t.TempDir(), t.TempDir()
	useReadoutReplayEnv(t, root, time.Now().Add(time.Hour))
	if err := os.WriteFile(filepath.Join(fh, "readouts.toml"), []byte(readoutTestRule), 0o644); err != nil {
		t.Fatal(err)
	}
	listing := readoutListing("")
	for _, id := range []string{"toolu_ref", "toolu_clean", "toolu_gone"} {
		if code, out, _, _ := runReadoutHook(t, []string{"--event", "post-tool"}, reportEnvelope(t, cwd, id, "ls -1", listing)); code != 0 || out != "" {
			t.Fatalf("readout-hook %s: exit %d, stdout %q", id, code, out)
		}
	}
	tr := &replayFixture{session: "s-1"}
	tr.bash(t, "2026-10-18T10:00:01Z", "toolu_ref", "ls -1", listing, nil)
	tr.use(t, "2026-10-18T10:00:02Z", "toolu_r1", "Read", map[string]any{"file_path": "/repo/internal/some/longer/path/file_07.go"})
	tr.bash(t, "2026-10-18T10:00:03Z", "toolu_clean", "ls -1", listing, nil)
	tr.use(t, "2026-10-18T10:00:04Z", "toolu_r2", "Read", map[string]any{"file_path": "/repo/internal/some/longer/path/file_00.go"})
	tr.write(t, filepath.Join(root, "-repo", "s-1.jsonl"))

	code, out, errOut := runReadoutFit(t, "--json", "--seed", "3")
	if code != 0 {
		t.Fatalf("readout fit --json: exit %d, stderr %q", code, errOut)
	}
	var f roFit
	if err := json.Unmarshal([]byte(out), &f); err != nil {
		t.Fatalf("readout fit --json: %v\n%s", err, out)
	}
	if f.Rows != 3 || f.Fit.Labels != 2 || f.Skipped.TranscriptGone != 1 || f.DecidedBy != "table" || f.Written {
		t.Fatalf("fit %+v: want 3 rows, 2 labels, 1 transcript gone, decided_by table, not written", f)
	}
	if f.Fit.Fit.Labels+f.Fit.HeldOut.Labels != 2 || f.Fit.Unscored != 0 || f.Fit.Seed != 3 {
		t.Fatalf("halves %+v / %+v, unscored %d, seed %d", f.Fit.Fit, f.Fit.HeldOut, f.Fit.Unscored, f.Fit.Seed)
	}
	if _, again, _ := runReadoutFit(t, "--json", "--seed", "3"); again != out {
		t.Fatalf("the same seed gave another fit:\n%s\n%s", again, out)
	}
	if _, err := os.Stat(readout.PolicyPath(fh)); !os.IsNotExist(err) {
		t.Fatalf("a fit without --write left a policy file (%v)", err)
	}

	code, text, _ := runReadoutFit(t)
	for _, want := range []string{"readout fit (seed 1): 2 labels from 3 readout rows", "transcript gone 1", "fitted half:", "held-out half:", "95% Wilson", "decided_by: table", "not written: --write writes " + readout.PolicyPath(fh)} {
		if code != 0 || !strings.Contains(text, want) {
			t.Fatalf("exit %d, text lacks %q:\n%s", code, want, text)
		}
	}

	if code, text, errOut := runReadoutFit(t, "--write"); code != 0 || !strings.Contains(text, "written: "+readout.PolicyPath(fh)) {
		t.Fatalf("--write: exit %d, stderr %q\n%s", code, errOut, text)
	}
	if _, found, err := readout.LoadPolicyFile(fh); !found || err != nil {
		t.Fatalf("written policy: found %v, %v", found, err)
	}
	repo := t.TempDir()
	if c := checkReadouts(repo); !strings.Contains(c.Reason, "policy: fitted cut points from "+readout.PolicyPath(fh)) {
		t.Fatalf("doctor %q lacks the fitted policy file", c.Reason)
	}
	if err := os.WriteFile(readout.PolicyPath(fh), []byte("version = 1\nlong_read_lines = 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if c := checkReadouts(repo); c.Status != doctorWarn || !strings.Contains(c.Reason, "policy file invalid, the built-in cut points stand") {
		t.Fatalf("doctor %+v: want a warn naming the invalid policy file", c)
	}
}
