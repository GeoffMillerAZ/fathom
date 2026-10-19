package main

// reconcile_baseline_test.go — Bug 5 of docs/plans/fleet-adoption-fixes.md
// (FA-23, FA-24, O657). The 2026-09-28 fleet pass (a binary built before
// ADR-059) rewrote about ten adopters' skills, hooks and stanzas: reconcile
// recorded the skill baselines BEFORE the apply, and the apply moved only
// the owned-file id, so the skill id kept the bytes the apply replaced and
// every later release read fathom's own render as DRIFT; `fathom init
// --skills` recorded no baseline at all. These tests rebuild that state and
// pin the fix: every writer records its baselines from the bytes it wrote,
// and --rebaseline records a skill only when fathom shipped its content.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// fleet0928Repo rebuilds an adopter as the 2026-09-28 pass left it: the
// stanza in both AGENTS.md and CLAUDE.md (the pre-ADR-059 layout), every
// hook but the two test-run rows (TR-2 shipped after), and every skill at an
// older render whose owned-file id records it and whose skill id records
// the render before that (fathom-findings, new that day, has none).
func fleet0928Repo(t *testing.T) string {
	t.Helper()
	root := legacyTwoCopyRepo(t, true)
	var keys []wiring.Key
	for _, hs := range fathomHookSpecs {
		if strings.Contains(hs.marker, "test-run") {
			continue
		}
		keys = append(keys, hookKeyFor(hs, "fathom "+hs.sub))
	}
	clock := func() time.Time { return time.Date(2026, 9, 28, 18, 50, 14, 0, time.UTC) }
	if _, err := wiring.ApplyWiring(root, wiring.Spec{Keys: keys}, wiring.ApplyOptions{Now: clock}); err != nil {
		t.Fatal(err)
	}
	ledger, err := wiring.LoadLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range skillsdist.Names {
		rel := skillRelPath(skillsTargetClaude, name)
		old := skillMarker + " -->\n# " + name + ", as the 2026-09-13 release rendered it\n"
		writeRepoFile(t, root, rel, old)
		ledger.Record(rel, wiring.OwnedKey{ID: "file", Digest: wiringDigest(old), Wrote: "2026-09-28T18:50:14Z"})
		if name != "fathom-findings" {
			ledger.Record(rel, wiring.OwnedKey{ID: "skill", Digest: wiringDigest("# " + name + ", as the 2026-08-13 release rendered it\n"), Wrote: "2026-09-28"})
		}
	}
	if err := wiring.SaveLedger(root, ledger); err != nil {
		t.Fatal(err)
	}
	return root
}

func reconcileRun(t *testing.T, args ...string) (string, int) {
	t.Helper()
	var out, errb strings.Builder
	code := cmdFleetReconcile(append([]string{"--url", "http://127.0.0.1:1"}, args...), &out, &errb)
	return out.String() + errb.String(), code
}

// TestEveryWiringWriterRecordsItsBaselineAfterTheWrite is FA-23: one
// `fleet reconcile --apply` over the 09-28 state names every file it
// changed, adds the test-run hooks from the one hook table, leaves the
// stanza in AGENTS.md alone, and records each skill's baselines from the
// bytes it wrote, so the next `fleet reconcile` reads current. `fathom init
// --skills` records the same baselines for what it writes.
func TestEveryWiringWriterRecordsItsBaselineAfterTheWrite(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := fleet0928Repo(t)

	dry, code := reconcileRun(t, "--repo", root)
	if code != 0 {
		t.Fatalf("dry-run exit %d:\n%s", code, dry)
	}
	if strings.Contains(dry, "DRIFT:   skill") {
		t.Errorf("fathom's own earlier render (the bytes its owned-file id records) read as DRIFT:\n%s", dry)
	}
	for _, want := range []string{"fathom session-hook --event test-run", "fathom session-hook --event failed-test-run", "shipped skill file(s) behind the shipped source", "CLAUDE.md still carries the stanza"} {
		if !strings.Contains(dry, want) {
			t.Errorf("dry-run lacks %q:\n%s", want, dry)
		}
	}

	applied, code := reconcileRun(t, "--apply", "--repo", root)
	if code != 0 {
		t.Fatalf("apply exit %d:\n%s", code, applied)
	}
	wantChanged := []string{".claude/settings.json", ".fathom/owned.json", "CLAUDE.md"}
	for _, name := range skillsdist.Names {
		wantChanged = append(wantChanged, skillRelPath(skillsTargetClaude, name))
	}
	for _, f := range wantChanged {
		if !strings.Contains(applied, "  changed: "+f+"\n") {
			t.Errorf("apply did not name the changed file %s:\n%s", f, applied)
		}
	}

	// The ledger holds the bytes written, under both ids, for every skill.
	ledger, err := wiring.LoadLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range skillsdist.Names {
		rel := skillRelPath(skillsTargetClaude, name)
		disk := readRepoFile(t, root, rel)
		render, _ := renderSkill(skillsTargetClaude, name)
		if disk != string(render) {
			t.Errorf("%s was not rewritten to the shipped render", rel)
		}
		for _, id := range []string{"file", "skill"} {
			if got := ledgerDigest(ledger, rel, id); got != wiringDigest(disk) {
				t.Errorf("%s %s baseline = %s, want the digest of the bytes written %s", rel, id, got, wiringDigest(disk))
			}
		}
	}
	settings := readRepoFile(t, root, ".claude/settings.json")
	for _, m := range []string{"fathom session-hook --event test-run", "fathom session-hook --event failed-test-run"} {
		if !strings.Contains(settings, m) {
			t.Errorf("apply did not add the %q hook from the hook table:\n%s", m, settings)
		}
	}
	if claude := readRepoFile(t, root, "CLAUDE.md"); ownedRegionBody(claude) != "@AGENTS.md" || strings.Contains(claude, "## fathom code intelligence") {
		t.Errorf("CLAUDE.md still carries a stanza after apply:\n%s", claude)
	}
	if agents := readRepoFile(t, root, "AGENTS.md"); !strings.Contains(agents, "## fathom code intelligence") {
		t.Errorf("AGENTS.md lost the stanza:\n%s", agents)
	}

	again, code := reconcileRun(t, "--repo", root)
	if code != 0 || !strings.Contains(again, root+": wiring current") {
		t.Errorf("reconcile after one apply does not read current (exit %d):\n%s", code, again)
	}

	// fathom init --skills: the init writer records the same baselines.
	dir := t.TempDir()
	srv := fakeInitDaemon(t, strings.ToLower(filepath.Base(dir)), 5)
	defer srv.Close()
	var out, errBuf bytes.Buffer
	if code := cmdInit([]string{"--path", dir, "--url", srv.URL, "--bin", testBinPath, "--skills=claude"}, &out, &errBuf); code != 0 {
		t.Fatalf("init exit = %d; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	ledger, err = wiring.LoadLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range skillsdist.Names {
		rel := skillRelPath(skillsTargetClaude, name)
		disk := readRepoFile(t, dir, rel)
		for _, id := range []string{"file", "skill"} {
			if got := ledgerDigest(ledger, rel, id); got != wiringDigest(disk) {
				t.Errorf("init: %s %s baseline = %q, want %s", rel, id, got, wiringDigest(disk))
			}
		}
	}
}

// TestInstallSkillsRecordsWhatItWroteBeforeARefusal: a refusal part-way
// (a hand-authored file by a skill's name) still leaves a baseline for
// every skill already written.
func TestInstallSkillsRecordsWhatItWroteBeforeARefusal(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, ".claude/skills/fathom-onboard/SKILL.md", "---\nname: fathom-onboard\n---\nmine.\n")
	var out bytes.Buffer
	if err := installSkills(nil, root, "claude", &out); err == nil {
		t.Fatal("expected the refusal")
	}
	ledger, err := wiring.LoadLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fathom-plan", "fathom-verify"} {
		rel := skillRelPath(skillsTargetClaude, name)
		if got := ledgerDigest(ledger, rel, "skill"); got != wiringDigest(readRepoFile(t, root, rel)) {
			t.Errorf("%s written before the refusal has baseline %q", rel, got)
		}
	}
	if got := ledgerDigest(ledger, ".claude/skills/fathom-onboard/SKILL.md", "skill"); got != "" {
		t.Errorf("the refused hand-authored file got a baseline %q", got)
	}
}

// TestReconcileRebaselineRecordsOnlyShippedRenders is FA-24: --apply
// --rebaseline re-records a skill only when its content is a render fathom
// shipped (this binary's or a known earlier one). A hand edit under the
// marker stays DRIFT, is named as refused, and is neither recorded nor
// rewritten.
func TestReconcileRebaselineRecordsOnlyShippedRenders(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := filepath.Join(t.TempDir(), "demo")
	if _, code := reconcileRun(t, "--apply", "--repo", root); code != 0 {
		t.Fatal("seed apply failed")
	}
	edited := skillMarker + " -->\n---\nname: fathom-plan\nhooks: [x]\n---\n# not fathom's\n"
	planRel := skillRelPath(skillsTargetClaude, "fathom-plan")
	writeRepoFile(t, root, planRel, edited)
	// fathom-verify sits at a render an earlier release shipped (added to
	// the known list for this test), its baselines lagging behind it.
	verifyRel := skillRelPath(skillsTargetClaude, "fathom-verify")
	known := skillMarker + " -->\n# fathom-verify, an earlier release's render\n"
	writeRepoFile(t, root, verifyRel, known)
	prior := shippedSkillRenderDigestList
	shippedSkillRenderDigestList = append(append([]string(nil), prior...), wiringDigest(known))
	t.Cleanup(func() { shippedSkillRenderDigestList = prior })
	ledger, _ := wiring.LoadLedger(root)
	ledger.Record(verifyRel, wiring.OwnedKey{ID: "skill", Digest: wiringDigest("older"), Wrote: "2026-09-28"})
	ledger.Record(verifyRel, wiring.OwnedKey{ID: "file", Digest: wiringDigest("older"), Wrote: "2026-09-28"})
	if err := wiring.SaveLedger(root, ledger); err != nil {
		t.Fatal(err)
	}

	dry, _ := reconcileRun(t, "--repo", root)
	if !strings.Contains(dry, "DRIFT:   skill "+planRel) || strings.Contains(dry, "DRIFT:   skill "+verifyRel) {
		t.Errorf("want only the hand edit as DRIFT:\n%s", dry)
	}
	if !strings.Contains(dry, "delete the fathom:owned line") {
		t.Errorf("a skill drift names the rebaseline remedy:\n%s", dry)
	}

	out, code := reconcileRun(t, "--apply", "--rebaseline", "--repo", root)
	if code != 1 {
		t.Errorf("--apply --rebaseline exit %d, want 1 (the hand-edited file stays BLOCKED):\n%s", code, out)
	}
	if !strings.Contains(out, "  refused: skill "+planRel) {
		t.Errorf("the hand edit is not named as refused:\n%s", out)
	}
	if got := readRepoFile(t, root, planRel); got != edited {
		t.Errorf("the hand-edited skill was rewritten:\n%s", got)
	}
	ledger, _ = wiring.LoadLedger(root)
	if ledgerDigest(ledger, planRel, "skill") == wiringDigest(edited) {
		t.Error("--rebaseline recorded content no fathom release rendered")
	}
	render, _ := renderSkill(skillsTargetClaude, "fathom-verify")
	if got := readRepoFile(t, root, verifyRel); got != string(render) {
		t.Errorf("the earlier render was not rewritten to the shipped one")
	}
	if got := ledgerDigest(ledger, verifyRel, "skill"); got != wiringDigest(string(render)) {
		t.Errorf("fathom-verify skill baseline = %s, want the render written", got)
	}
	again, _ := reconcileRun(t, "--repo", root)
	if !strings.Contains(again, "DRIFT:   skill "+planRel) {
		t.Errorf("the refused hand edit stopped reading as DRIFT:\n%s", again)
	}
}

// TestShippedSkillRenderDigestsCoverThisBinary keeps the known-render list
// complete: every render this binary ships, for every target, is in it. A
// skill edit appends the new digests (FA-24).
func TestShippedSkillRenderDigestsCoverThisBinary(t *testing.T) {
	var missing []string
	known := shippedSkillRenderDigests()
	for _, name := range skillsdist.Names {
		for _, target := range []string{skillsTargetClaude, skillsTargetAgents, skillsTargetCodexPrompts} {
			b, err := renderSkill(target, name)
			if err != nil {
				t.Fatal(err)
			}
			if d := wiringDigest(string(b)); !known[d] {
				missing = append(missing, "\t\""+d+"\", // "+name+" "+target)
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("shippedSkillRenderDigestList (skills_shipped_renders.go) lacks this binary's renders; add:\n%s", strings.Join(missing, "\n"))
	}
}

// ledgerDigest is the digest the ledger records for id in file, "" when none.
func ledgerDigest(l wiring.Ledger, file, id string) string {
	for _, k := range wiring.OwnedKeys(l, file) {
		if k.ID == id {
			return k.Digest
		}
	}
	return ""
}

// wiringDigest is the ledger's digest form: sha256 of the bytes, hex,
// "sha256:"-prefixed.
func wiringDigest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(sum[:])
}
