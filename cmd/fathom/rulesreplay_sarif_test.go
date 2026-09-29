package main

// rulesreplay_sarif_test.go — SR-47w (docs/plans/surface-reckoning.md, F-29):
// `fathom rules replay --format sarif` emits the layer-rule violations a
// replayed range introduced as SARIF 2.1.0, through guardsarif and the
// shared emitter, and leaves the table and --json alone.

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/rulereplay"
)

// replayRepo is a throwaway git repository the replay reads history from.
type replayRepo struct {
	t   *testing.T
	dir string
	n   int
}

func newReplayRepo(t *testing.T) *replayRepo {
	t.Helper()
	requireGit(t)
	r := &replayRepo{t: t, dir: t.TempDir()}
	r.git(nil, "init", "-q", "-b", "main")
	return r
}

func (r *replayRepo) git(env []string, args ...string) {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.dir, "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func (r *replayRepo) write(rel, content string) {
	r.t.Helper()
	full := filepath.Join(r.dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// commit commits everything at a strictly increasing date.
func (r *replayRepo) commit(msg string) {
	r.t.Helper()
	r.n++
	date := "2026-01-01T0" + string(rune('0'+r.n)) + ":00:00Z"
	env := []string{
		"GIT_AUTHOR_NAME=replay-test", "GIT_AUTHOR_EMAIL=replay-test@example.com",
		"GIT_COMMITTER_NAME=replay-test", "GIT_COMMITTER_EMAIL=replay-test@example.com",
		"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date,
	}
	r.git(env, "add", "-A")
	r.git(env, "commit", "-q", "-m", msg)
}

// sarifLog is the slice of a SARIF 2.1.0 log the test reads.
type sarifLog struct {
	Version string `json:"version"`
	Runs    []struct {
		Tool struct {
			Driver struct {
				Name  string `json:"name"`
				Rules []struct {
					ID string `json:"id"`
				} `json:"rules"`
			} `json:"driver"`
		} `json:"tool"`
		Results []struct {
			RuleID  string `json:"ruleId"`
			Level   string `json:"level"`
			Message struct {
				Text string `json:"text"`
			} `json:"message"`
			Locations []struct {
				PhysicalLocation struct {
					ArtifactLocation struct {
						URI string `json:"uri"`
					} `json:"artifactLocation"`
				} `json:"physicalLocation"`
			} `json:"locations"`
		} `json:"results"`
	} `json:"runs"`
}

// TestCmdRulesReplayEmitsSARIF is SR-47w: over a range that introduces two
// violations (one error rule, one warn rule) and then carries them
// unchanged through a third commit, `--format sarif` emits SARIF 2.1.0 with
// one result per violation, each at the commit that introduced it — the
// inherited repeat is not a second finding — leveled by the rule's severity
// and located at the offending package; the table and --json are unchanged.
func TestCmdRulesReplayEmitsSARIF(t *testing.T) {
	r := newReplayRepo(t)
	r.write("go.mod", "module tmpmod\n\ngo 1.24\n")
	r.write(".fathom/guard.cue", `guards: []
layer_rules: [
	{id: "a-not-b", kind: "import_direction", sentence: "a must not import b", from: ["a"], to: ["b"]},
	{id: "a-not-c", kind: "import_direction", sentence: "a should not import c", from: ["a"], to: ["c"], severity: "warn"},
]
`)
	r.write("a/a.go", "package a\n\nfunc F() {}\n")
	r.write("b/b.go", "package b\n\nfunc G() {}\n")
	r.write("c/c.go", "package c\n\nfunc H() {}\n")
	r.commit("c1: clean")
	r.write("a/a.go", "package a\n\nimport (\n\t\"tmpmod/b\"\n\t\"tmpmod/c\"\n)\n\nfunc F() { b.G(); c.H() }\n")
	r.commit("c2: a reaches into b and c")
	r.write("README.md", "unrelated\n")
	r.commit("c3: unrelated, violations persist")

	var out, errOut bytes.Buffer
	if code := cmdRulesReplay([]string{"--root", r.dir, "--since", "3", "--format", "sarif"}, &out, &errOut); code != 0 {
		t.Fatalf("rules replay --format sarif exit %d: %s", code, errOut.String())
	}
	var log sarifLog
	if err := json.Unmarshal(out.Bytes(), &log); err != nil {
		t.Fatalf("--format sarif did not emit JSON: %v\n%s", err, out.String())
	}
	if log.Version != "2.1.0" || len(log.Runs) != 1 {
		t.Fatalf("version %q with %d runs, want SARIF 2.1.0 with one run", log.Version, len(log.Runs))
	}
	run := log.Runs[0]
	if run.Tool.Driver.Name != "fathom rules replay" {
		t.Errorf("driver name = %q, want \"fathom rules replay\"", run.Tool.Driver.Name)
	}
	if len(run.Results) != 2 {
		t.Fatalf("%d results, want 2 (each violation once, at the commit that introduced it): %s", len(run.Results), out.String())
	}
	byRule := map[string]int{}
	for i, res := range run.Results {
		byRule[res.RuleID] = i
	}
	errRes, warnRes := run.Results[byRule["a-not-b"]], run.Results[byRule["a-not-c"]]
	if errRes.Level != "error" || warnRes.Level != "warning" {
		t.Errorf("levels = %q / %q, want error / warning from the rules' severities", errRes.Level, warnRes.Level)
	}
	if !strings.Contains(errRes.Message.Text, "a must not import b") || !strings.Contains(errRes.Message.Text, "c2: a reaches into b and c") {
		t.Errorf("a-not-b message %q must carry the rule's sentence and the commit that introduced it", errRes.Message.Text)
	}
	if len(errRes.Locations) != 1 || errRes.Locations[0].PhysicalLocation.ArtifactLocation.URI != "a" {
		t.Errorf("a-not-b location = %+v, want the offending package \"a\"", errRes.Locations)
	}
	if len(run.Tool.Driver.Rules) != 2 || run.Tool.Driver.Rules[0].ID != "a-not-b" || run.Tool.Driver.Rules[1].ID != "a-not-c" {
		t.Errorf("driver rules = %+v, want a-not-b, a-not-c", run.Tool.Driver.Rules)
	}

	// The table and --json are what they were: no SARIF in either, and the
	// JSON is still the replay report (every commit, inherited rows included).
	out.Reset()
	if code := cmdRulesReplay([]string{"--root", r.dir, "--since", "3"}, &out, &errOut); code != 0 {
		t.Fatalf("rules replay exit %d: %s", code, errOut.String())
	}
	table := out.String()
	if !strings.HasPrefix(table, "fathom rules replay — since 3") || !strings.Contains(table, "SHA") || strings.Contains(table, "2.1.0") {
		t.Errorf("the table changed:\n%s", table)
	}
	out.Reset()
	if code := cmdRulesReplay([]string{"--root", r.dir, "--since", "3", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("rules replay --json exit %d: %s", code, errOut.String())
	}
	var report rulereplay.Report
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.TotalCommits != 3 || len(report.Commits) != 3 || len(report.Commits[2].Violations) != 2 {
		t.Fatalf("--json changed: %v\n%s", err, out.String())
	}
	if strings.Contains(out.String(), "2.1.0") {
		t.Error("--json carries SARIF")
	}

	// An unknown format is refused, not rendered as a table.
	out.Reset()
	errOut.Reset()
	if code := cmdRulesReplay([]string{"--root", r.dir, "--since", "3", "--format", "xml"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), `--format "xml"`) || out.Len() != 0 {
		t.Errorf("--format xml: exit %d, stdout %q, stderr %q", code, out.String(), errOut.String())
	}
}
