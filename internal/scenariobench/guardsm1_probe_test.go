// guardsm1_probe_test.go — RC-50's own mechanical proof that
// testdata/scenariobench/guards-m1.json's detectors detect, not merely
// that they run (TestGuardsCorpusDetectorsExist, guards_test.go, already
// proves the latter for every guards-*.json corpus).
//
// TestGuardsM1ViolationProbesFireAndClear applies each task's own
// subject.setup (Subject.Setup — the same field PrepareSubject runs in the
// worktree BEFORE the first ingest; two of six tasks use it to materialize
// an edit-site scaffold that must never be committed to this repo itself)
// and then its violation_probe to a disposable copy of the committed
// subject tree, and checks its PRIMARY rule detector on both sides of the
// violation. guards-m1.json's six tasks split into two distinct shapes,
// and the test checks each the way that actually proves something about
// it:
//
//   - BREAKAGE tasks (a/fanout, b/org-agnostic, c/store-cgo,
//     d/served-only-core-import): once subject.setup (if any) has run, the
//     tree already HONORS the rule, so the detector is green; the probe is
//     a naive edit that breaks it, and the detector must go red.
//   - OMISSION tasks (e/fleet-index-summary-doc, f/commit-verified-line):
//     the rule is "did every governed file/step actually happen", which
//     has no truthful "untouched" reading — an untouched tree has NOT done
//     the governed thing yet, so its detector reads red for the same
//     reason a naive PARTIAL completion does. For these two, the test
//     checks the pairing that is actually meaningful: the probe (a
//     plausible partial completion — two of three files for e, a commit
//     with no trailer for f) reads RED, and completing the omitted piece
//     turns the SAME detector GREEN. Neither carries a subject.setup.
//
// Every detector command in guards-m1.json runs unmodified via `sh -c` —
// this file adds no new detector logic, only exercises the ones already
// committed to the corpus.
//
// Change-present detectors and guards-m1-fleet-index-summary-doc's
// registry<->catalog sync detector (catalog-registry-sync, an existing,
// independently pinned test — internal/mcp's TestMcpToolsDocMatchesRegistry
// — reused here rather than re-derived) are out of this file's scope: the
// former are schema-validated (allowed verb, resolvable command) by
// TestGuardsCorpusDetectorsExist already, and are a "did anything happen"
// formality orthogonal to whether a RULE was violated; the latter is
// exercised by internal/mcp's own suite, not by this corpus.
package scenariobench

import (
	"os"
	"os/exec"
	"testing"
)

const guardsM1CorpusPath = "../../testdata/scenariobench/guards-m1.json"

// guardsM1PrimaryDetector names, per task, the one oracle.detectors entry
// this file's probe exercises red/green — see the package doc comment for
// why the other detectors (change-present, catalog-registry-sync) are out
// of scope here.
var guardsM1PrimaryDetector = map[string]string{
	"guards-m1-fanout-determinism":       "fanout-deterministic",
	"guards-m1-org-agnostic-example":     "no-org-names",
	"guards-m1-store-search-accelerator": "store-purego-build",
	"guards-m1-served-only-core-import":  "layer-rule-core-no-mcp-or-cmd",
	"guards-m1-fleet-index-summary-doc":  "usecase-maintained-line",
	"guards-m1-doctor-lefthook-commit":   "verified-line",
}

// taskDetector returns the named detector from task's oracle.
func taskDetector(t *testing.T, task Task, name string) Detector {
	t.Helper()
	for _, d := range task.Oracle.Detectors {
		if d.Name == name {
			return d
		}
	}
	t.Fatalf("task %s: no detector named %q in oracle.detectors", task.ID, name)
	return Detector{}
}

// archiveHEAD extracts `git archive HEAD` of root into a fresh directory
// under t.TempDir(): a disposable, plain (no .git) copy of the committed
// subject tree, exactly as RC-50 asks — cheap enough to make one per task.
func archiveHEAD(t *testing.T, root string) string {
	t.Helper()
	dir := t.TempDir()
	archive := exec.Command("git", "-C", root, "archive", "HEAD")
	tarx := exec.Command("tar", "-x", "-C", dir)
	pipe, err := archive.StdoutPipe()
	if err != nil {
		t.Fatalf("git archive: stdout pipe: %v", err)
	}
	tarx.Stdin = pipe
	if err := tarx.Start(); err != nil {
		t.Fatalf("tar -x: start: %v", err)
	}
	if err := archive.Run(); err != nil {
		t.Fatalf("git -C %s archive HEAD: %v", root, err)
	}
	if err := tarx.Wait(); err != nil {
		t.Fatalf("tar -x: %v", err)
	}
	return dir
}

// runProbeDetector runs command (a guards-m1.json detector, verbatim) via
// `sh -c` in dir, inheriting this test process's own environment (the
// GOMODCACHE/GOCACHE/CGO_ENABLED/GOFLAGS the gate is invoked with) plus an
// explicit GOWORK=off, and returns its exit code and combined output.
func runProbeDetector(t *testing.T, dir, command string) (int, string) {
	t.Helper()
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(out)
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), string(out)
	}
	t.Fatalf("running detector %q in %s: %v\n%s", command, dir, err, out)
	return -1, string(out)
}

// applyCommands runs every command in order via `sh -c` in dir, failing
// the test immediately if any one of them errors — a subject.setup, a
// violation_probe, or this file's own "now complete the omitted piece"
// fix is meant to be unconditionally applicable to a fresh copy of the
// tree.
func applyCommands(t *testing.T, dir string, commands []string) {
	t.Helper()
	for _, c := range commands {
		cmd := exec.Command("sh", "-c", c)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("applying %q: %v\n%s", c, err, out)
		}
	}
}

// TestGuardsM1ViolationProbesFireAndClear is RC-50's anchor: every one of
// guards-m1.json's six tasks has its primary rule detector proven to
// discriminate — see the package doc comment for the breakage/omission
// split this test drives from guardsM1PrimaryDetector.
func TestGuardsM1ViolationProbesFireAndClear(t *testing.T) {
	c := loadGuardsCorpusAt(t, guardsM1CorpusPath)
	if len(guardsM1PrimaryDetector) != len(c.Tasks) {
		t.Fatalf("guardsM1PrimaryDetector has %d entries, corpus has %d tasks — keep them in lockstep", len(guardsM1PrimaryDetector), len(c.Tasks))
	}

	for _, task := range c.Tasks {
		task := task
		t.Run(task.ID, func(t *testing.T) {
			if task.ViolationProbe == nil || len(task.ViolationProbe.Commands) == 0 {
				t.Fatalf("task %s: no violation_probe", task.ID)
			}
			detectorName, ok := guardsM1PrimaryDetector[task.ID]
			if !ok {
				t.Fatalf("task %s: not registered in guardsM1PrimaryDetector", task.ID)
			}
			detector := taskDetector(t, task, detectorName)

			if task.ID == "guards-m1-doctor-lefthook-commit" {
				checkOmissionTaskCommit(t, task, detector)
				return
			}
			if task.ID == "guards-m1-fleet-index-summary-doc" {
				checkOmissionTaskMaintainedLine(t, task, detector)
				return
			}
			checkBreakageTask(t, task, detector)
		})
	}
}

// checkBreakageTask is the shape shared by fanout-determinism,
// org-agnostic-example, store-search-accelerator and
// served-only-core-import: task.Subject.Setup (if any) materializes the
// edit-site scaffold — the same step PrepareSubject runs in a real cell,
// before the first ingest — so the tree already honors the rule (detector
// green); the probe breaks it (detector red).
func checkBreakageTask(t *testing.T, task Task, detector Detector) {
	t.Helper()
	dir := archiveHEAD(t, moduleRoot)

	applyCommands(t, dir, task.Subject.Setup)

	if exit, out := runProbeDetector(t, dir, detector.Command); exit != 0 {
		t.Fatalf("detector %q must be green once subject.setup has materialized this task's edit site (this task's baseline is meant to already honor the rule), got exit %d:\n%s", detector.Name, exit, out)
	}

	applyCommands(t, dir, task.ViolationProbe.Commands)

	if exit, out := runProbeDetector(t, dir, detector.Command); exit == 0 {
		t.Fatalf("detector %q must go RED after the violation_probe (%s) — it did not fire:\n%s", detector.Name, task.ViolationProbe.Description, out)
	} else {
		t.Logf("%s: detector %q correctly red (exit %d) after: %s", task.ID, detector.Name, exit, task.ViolationProbe.Description)
	}
}

// checkOmissionTaskMaintainedLine is guards-m1-fleet-index-summary-doc's
// shape: the untouched tree has not done the governed thing at all, so
// there is no truthful "green baseline" for this detector (and this task
// carries no subject.setup — it touches files this repo already has) —
// instead, the probe's naive PARTIAL completion (registry + catalog, no
// Maintained line) must read red, and finishing the omitted piece must
// turn the same detector green.
func checkOmissionTaskMaintainedLine(t *testing.T, task Task, detector Detector) {
	t.Helper()
	dir := archiveHEAD(t, moduleRoot)

	applyCommands(t, dir, task.ViolationProbe.Commands)
	if exit, out := runProbeDetector(t, dir, detector.Command); exit == 0 {
		t.Fatalf("detector %q must be RED on the naive partial completion (%s):\n%s", detector.Name, task.ViolationProbe.Description, out)
	} else {
		t.Logf("%s: detector %q correctly red (exit %d) on the partial completion", task.ID, detector.Name, exit)
	}

	// Complete the omitted third file: add the tool name to
	// fleet-health.md's own Maintained section — the exact edit
	// docs/CLAUDE.md's rule asks for and the naive fix skipped. An awk
	// insert-after-heading, not a sed s/// against the section's long
	// prose line, so it does not depend on that paragraph's exact wording.
	fix := []string{
		`awk '1; /^## Maintained/{print "` + "`fleet_index_summary`" + ` (added by the probe test fix)."}' docs/use-cases/fleet-health.md > docs/use-cases/fleet-health.md.new && mv docs/use-cases/fleet-health.md.new docs/use-cases/fleet-health.md`,
	}
	applyCommands(t, dir, fix)

	if exit, out := runProbeDetector(t, dir, detector.Command); exit != 0 {
		t.Fatalf("detector %q must be GREEN once the Maintained line names the tool, got exit %d:\n%s", detector.Name, exit, out)
	} else {
		t.Logf("%s: detector %q correctly green once the omitted Maintained-line edit is made", task.ID, detector.Name)
	}
}

// checkOmissionTaskCommit is guards-m1-doctor-lefthook-commit's shape: the
// rule is about a COMMIT's own message, so there is no file-content
// "untouched tree" reading at all (and no subject.setup either) — the
// probe (a real `git commit` with no Verified: line, in a throwaway repo)
// must read red, and amending the same commit to carry the trailer must
// turn the same detector green.
func checkOmissionTaskCommit(t *testing.T, task Task, detector Detector) {
	t.Helper()
	dir := t.TempDir()

	applyCommands(t, dir, task.ViolationProbe.Commands)
	if exit, out := runProbeDetector(t, dir, detector.Command); exit == 0 {
		t.Fatalf("detector %q must be RED on a commit with no Verified: line:\n%s", detector.Name, out)
	} else {
		t.Logf("%s: detector %q correctly red (exit %d) on a commit with no Verified: line", task.ID, detector.Name, exit)
	}

	amend := exec.Command("git", "commit", "-q", "--amend",
		"-m", "fix: doctor lefthook path lookup",
		"-m", "Verified: go test ./cmd/fathom/... -run TestDoctor")
	amend.Dir = dir
	if out, err := amend.CombinedOutput(); err != nil {
		t.Fatalf("git commit --amend: %v\n%s", err, out)
	}

	if exit, out := runProbeDetector(t, dir, detector.Command); exit != 0 {
		t.Fatalf("detector %q must be GREEN once the commit carries a Verified: line, got exit %d:\n%s", detector.Name, exit, out)
	} else {
		t.Logf("%s: detector %q correctly green once the commit carries a Verified: line", task.ID, detector.Name)
	}
}
