package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitFixtureForArchive builds a real git repo at root with CHANGELOG.md,
// ADR.md, and docs/plans/<name>.md holding planMD, all committed — the
// minimum shape `fathom plans archive`/`restore` operate against (git mv +
// the CHANGELOG/ADR lexical-absorption check).
func gitFixtureForArchive(t *testing.T, root, name, planMD, changelog, adr string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(
			os.Environ(),
			"GIT_AUTHOR_NAME=Alice", "GIT_AUTHOR_EMAIL=alice@example.com",
			"GIT_COMMITTER_NAME=Alice", "GIT_COMMITTER_EMAIL=alice@example.com",
		)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out.String())
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "plans", name), []byte(planMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(changelog), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ADR.md"), []byte(adr), 0o644); err != nil {
		t.Fatal(err)
	}
	run("init", "-q")
	run("config", "commit.gpgsign", "false")
	run("add", ".")
	run("commit", "-q", "-m", "init fixture")
}

func archiveArgs(sub, root, storePath, plan string) []string {
	return []string{
		"plans", sub, "--root", root, "--service", "payment-service", "--store", storePath, plan,
	}
}

// ─── refusal paths (E4-1) ───────────────────────────────────────────────────

func TestCmdPlansArchive_RefusesIncompleteRows(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	storePath := seedPlansFixtureStore(t)
	root := t.TempDir()
	planMD := "# Incomplete\n\n" +
		"- [x] Charge handler {id: R1-1, symbol: Charge, skip_test: true}\n" +
		"- [ ] Not started {id: R1-2, symbol: Charge, skip_test: true}\n"
	gitFixtureForArchive(t, root, "incomplete.md", planMD, "R1-1 R1-2 shipped\n", "")

	var out, errBuf bytes.Buffer
	code := run(archiveArgs("archive", root, storePath, "incomplete.md"), &out, &errBuf)
	if code == 0 {
		t.Fatalf("want refusal (non-zero exit); stdout=%s", out.String())
	}
	if !strings.Contains(errBuf.String(), "R1-2") {
		t.Errorf("refusal must name the incomplete row R1-2: %s", errBuf.String())
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "plans", "incomplete.md")); err != nil {
		t.Error("refused archive must leave the plan file in place")
	}
}

func TestCmdPlansArchive_RefusesClaimedRowsNotGreen(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	storePath := seedPlansFixtureStore(t)
	root := t.TempDir()
	planMD := "# NotGreen\n\n" +
		"- [x] Missing symbol {id: R2-1, symbol: NoSuchSymbolXYZ, skip_test: true}\n"
	gitFixtureForArchive(t, root, "notgreen.md", planMD, "R2-1 shipped\n", "")

	var out, errBuf bytes.Buffer
	code := run(archiveArgs("archive", root, storePath, "notgreen.md"), &out, &errBuf)
	if code == 0 {
		t.Fatalf("want refusal (non-zero exit); stdout=%s", out.String())
	}
	if !strings.Contains(errBuf.String(), "R2-1") {
		t.Errorf("refusal must name the failing claim R2-1: %s", errBuf.String())
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "plans", "notgreen.md")); err != nil {
		t.Error("refused archive must leave the plan file in place")
	}
}

func TestCmdPlansArchive_RefusesUnabsorbedIDs(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	storePath := seedPlansFixtureStore(t)
	root := t.TempDir()
	planMD := "# Unabsorbed\n\n" +
		"- [x] Charge handler {id: R3-1, symbol: Charge, skip_test: true}\n"
	// Neither CHANGELOG.md nor ADR.md mentions R3-1.
	gitFixtureForArchive(t, root, "unabsorbed.md", planMD, "nothing relevant here\n", "nothing relevant here either\n")

	var out, errBuf bytes.Buffer
	code := run(archiveArgs("archive", root, storePath, "unabsorbed.md"), &out, &errBuf)
	if code == 0 {
		t.Fatalf("want refusal (non-zero exit); stdout=%s", out.String())
	}
	if !strings.Contains(errBuf.String(), "R3-1") || !strings.Contains(errBuf.String(), "absorb") {
		t.Errorf("refusal must name R3-1 and explain the absorption check: %s", errBuf.String())
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "plans", "unabsorbed.md")); err != nil {
		t.Error("refused archive must leave the plan file in place")
	}
}

// ─── success path + tombstone + restore round-trip (E4-1) ──────────────────

func TestCmdPlansArchive_SuccessTombstoneAndRestore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	storePath := seedPlansFixtureStore(t)
	root := t.TempDir()
	planMD := "# Archivable\n\n" +
		"- [x] Charge handler {id: S-1, symbol: Charge, skip_test: true}\n" +
		"- [-] Extra, out of scope {id: S-2, disposition: refused, reason: \"n/a\"}\n"
	gitFixtureForArchive(t, root, "archivable.md", planMD, "S-1 and S-2 shipped\n", "")

	// ── archive ──
	var out, errBuf bytes.Buffer
	code := run(archiveArgs("archive", root, storePath, "archivable.md"), &out, &errBuf)
	if code != 0 {
		t.Fatalf("archive exit = %d, stderr: %s", code, errBuf.String())
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "plans", "archivable.md")); err == nil {
		t.Error("archived plan must no longer exist at its original path")
	}
	archivedPath := filepath.Join(root, "docs", "plans", "archive", "archivable.md")
	body, err := os.ReadFile(archivedPath)
	if err != nil {
		t.Fatalf("archived file missing at %s: %v", archivedPath, err)
	}
	doc := string(body)
	if !strings.Contains(doc, "Archived") {
		t.Errorf("archived file missing the tombstone label: %s", doc)
	}
	head, herr := gitHEAD(root)
	if herr != nil {
		t.Fatalf("gitHEAD: %v", herr)
	}
	if !strings.Contains(doc, head) {
		t.Errorf("tombstone missing HEAD sha %s: %s", head, doc)
	}
	if !strings.Contains(doc, "2 tasks total") {
		t.Errorf("tombstone missing the verify summary: %s", doc)
	}
	// The original checklist content must still be present, unmutated.
	if !strings.Contains(doc, "Charge handler") || !strings.Contains(doc, "S-1") {
		t.Errorf("archived file lost its original content: %s", doc)
	}

	// The rename (+ tombstone edit) is staged: `git status --porcelain` shows
	// nothing left unstaged for the archived file.
	statusOut, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(statusOut)), "\n") {
		if strings.Contains(line, "archivable.md") && strings.HasPrefix(line, " ") {
			t.Errorf("archived file has unstaged changes: %q", line)
		}
	}

	// ── restore ──
	var rOut, rErr bytes.Buffer
	rCode := run([]string{"plans", "restore", "--root", root, "archivable.md"}, &rOut, &rErr)
	if rCode != 0 {
		t.Fatalf("restore exit = %d, stderr: %s", rCode, rErr.String())
	}
	if _, err := os.Stat(archivedPath); err == nil {
		t.Error("restored plan must no longer exist under docs/plans/archive/")
	}
	restoredPath := filepath.Join(root, "docs", "plans", "archivable.md")
	restoredBody, err := os.ReadFile(restoredPath)
	if err != nil {
		t.Fatalf("restored file missing at %s: %v", restoredPath, err)
	}
	// Round-trip: the tombstone (historical evidence of the archival) must
	// survive the restore.
	if !bytes.Equal(body, restoredBody) {
		t.Errorf("restore must be a pure move — content changed:\narchived:\n%s\nrestored:\n%s", doc, string(restoredBody))
	}
}

// ─── critic round: absorption token boundary (finding 2) ───────────────────

func TestIDAbsorbed(t *testing.T) {
	cases := []struct {
		text, id string
		want     bool
	}{
		{"E3-10 shipped", "E3-1", false},         // the live collision class
		{"E3-1 shipped", "E3-1", true},           // plain mention
		{"(E3-1)", "E3-1", true},                 // punctuation is a boundary
		{"rows E3-1, E3-2 landed", "E3-1", true}, // comma boundary
		{"XE3-1", "E3-1", false},                 // prefix collision
		{"E3-1", "E3-1", true},                   // whole-string match
		{"CF-10 then CF-1.", "CF-1", true},       // finds the later true match
		{"", "E3-1", false},
		{"anything", "", false}, // empty ID is never absorbed by content
	}
	for _, tc := range cases {
		if got := idAbsorbed(tc.text, tc.id); got != tc.want {
			t.Errorf("idAbsorbed(%q, %q) = %v, want %v", tc.text, tc.id, got, tc.want)
		}
	}
}

// TestCmdPlansArchive_AbsorptionTokenBoundary: a CHANGELOG mentioning only
// "E3-10" must NOT absolve row E3-1 (bare substring matching would); an
// actual "E3-1" mention (in parentheses, exercising punctuation boundaries)
// then satisfies the criterion.
func TestCmdPlansArchive_AbsorptionTokenBoundary(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	storePath := seedPlansFixtureStore(t)
	root := t.TempDir()
	planMD := "# Boundary\n\n" +
		"- [x] Charge handler {id: E3-1, symbol: Charge, skip_test: true}\n"
	gitFixtureForArchive(t, root, "boundary.md", planMD, "E3-10 shipped\n", "")

	var out, errBuf bytes.Buffer
	code := run(archiveArgs("archive", root, storePath, "boundary.md"), &out, &errBuf)
	if code == 0 {
		t.Fatalf("E3-10 must not absorb E3-1 — want refusal; stdout=%s", out.String())
	}
	if !strings.Contains(errBuf.String(), "E3-1") {
		t.Errorf("refusal must name the unabsorbed row E3-1: %s", errBuf.String())
	}

	// A real, boundary-delimited mention satisfies the check. (CHANGELOG
	// dirtiness is fine — only the PLAN must be clean; the absorption corpus
	// is read from the working tree.)
	f, err := os.OpenFile(filepath.Join(root, "CHANGELOG.md"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("also (E3-1) landed\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	var out2, errBuf2 bytes.Buffer
	if code := run(archiveArgs("archive", root, storePath, "boundary.md"), &out2, &errBuf2); code != 0 {
		t.Fatalf("archive after real absorption exit = %d, stderr: %s", code, errBuf2.String())
	}
}

// ─── critic round: dirty-plan preflight + restore collision (finding 5) ────

func TestCmdPlansArchive_RefusesDirtyPlan(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	storePath := seedPlansFixtureStore(t)
	root := t.TempDir()
	planMD := "# Dirty\n\n" +
		"- [x] Charge handler {id: D-1, symbol: Charge, skip_test: true}\n"
	gitFixtureForArchive(t, root, "dirty.md", planMD, "D-1 shipped\n", "")

	// Uncommitted edit to the plan — every archival criterion still holds,
	// so the refusal below can only come from the dirty preflight.
	planPath := filepath.Join(root, "docs", "plans", "dirty.md")
	if err := os.WriteFile(planPath, []byte(planMD+"\nan uncommitted trailing note\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errBuf bytes.Buffer
	code := run(archiveArgs("archive", root, storePath, "dirty.md"), &out, &errBuf)
	if code == 0 {
		t.Fatalf("dirty plan must refuse to archive; stdout=%s", out.String())
	}
	if !strings.Contains(errBuf.String(), "uncommitted") {
		t.Errorf("refusal must say the plan has uncommitted changes: %s", errBuf.String())
	}
	if _, err := os.Stat(planPath); err != nil {
		t.Error("refused archive must leave the plan file in place")
	}
}

func TestCmdPlansRestore_RefusesWhenActivePlanExists(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "plans", "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Both the archived copy and a live plan of the same name exist — the
	// exists-check fires before any git operation, so no repo is needed.
	if err := os.WriteFile(filepath.Join(root, "docs", "plans", "archive", "clash.md"), []byte("# archived\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "plans", "clash.md"), []byte("# active\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := run([]string{"plans", "restore", "--root", root, "clash.md"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("restore must refuse when an active plan with the same name exists")
	}
	if !strings.Contains(errBuf.String(), "already exists") {
		t.Errorf("refusal must explain the collision: %s", errBuf.String())
	}
}

func TestCmdPlansArchive_MissingPlanArg(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"plans", "archive", "--service", "svc"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("want non-zero exit when the plan argument is missing")
	}
}

func TestCmdPlansRestore_UnknownPlan(t *testing.T) {
	root := t.TempDir()
	var out, errBuf bytes.Buffer
	code := run([]string{"plans", "restore", "--root", root, "nope.md"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("want non-zero exit restoring a plan that was never archived")
	}
}

// TestCmdPlansArchive_AbsorbedByReleaseNarrative: the 2026-08-24 changelog
// condensation moved row ids out of CHANGELOG.md into the per-release
// narrative under docs/changelog/<version>.md; an id absorbed ONLY there
// still satisfies criterion 3 (the narrative is part of the absorption
// corpus), otherwise every pre-0.4.0 plan became unarchivable overnight.
func TestCmdPlansArchive_AbsorbedByReleaseNarrative(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	storePath := seedPlansFixtureStore(t)
	root := t.TempDir()
	planMD := "# Narrative\n\n" +
		"- [x] Charge handler {id: N4-1, symbol: Charge, skip_test: true}\n"
	// CHANGELOG.md and ADR.md say nothing about N4-1 …
	gitFixtureForArchive(t, root, "narrative.md", planMD, "condensed: nothing here\n", "")
	// … the per-release narrative does. (Only the PLAN must be clean.)
	if err := os.MkdirAll(filepath.Join(root, "docs", "changelog"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "changelog", "0.4.0.md"), []byte("### 2026-08-20\n- N4-1: charge handler landed.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errBuf bytes.Buffer
	code := run(archiveArgs("archive", root, storePath, "narrative.md"), &out, &errBuf)
	if code != 0 {
		t.Fatalf("id absorbed by docs/changelog/0.4.0.md must archive; exit %d stderr=%s", code, errBuf.String())
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "plans", "archive", "narrative.md")); err != nil {
		t.Errorf("plan must have moved to docs/plans/archive/: %v", err)
	}
}

// TestIDNotationExpansion: the changelog's range ("SH-1..6") and slash-list
// ("IT-14/15") reference points absorb every id they denote; a lone id,
// a date, or an over-wide range does not.
func TestIDNotationExpansion(t *testing.T) {
	text := "shipped SH-1..6, IT-14/15 and GKE-W1-1..3 on 2026-08-25; ADR-025; RG-6..8; wide X-1..9000"
	corpus := text + expandIDNotation(text)
	for _, want := range []string{"SH-1", "SH-6", "IT-14", "IT-15", "GKE-W1-2", "RG-7", "ADR-025"} {
		if !idAbsorbed(corpus, want) {
			t.Errorf("%q should be absorbed by %q", want, text)
		}
	}
	for _, not := range []string{"SH-7", "IT-16", "GKE-W1-4", "X-2", "2026-08-26"} {
		if idAbsorbed(corpus, not) {
			t.Errorf("%q must NOT be absorbed by %q", not, text)
		}
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("VS-1..4 verified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readAbsorptionCorpus(root, "docs/plans/none.md")
	if err != nil {
		t.Fatal(err)
	}
	if !idAbsorbed(got, "VS-3") || idAbsorbed(got, "VS-5") {
		t.Errorf("corpus from a range must absorb VS-3 and not VS-5: %q", got)
	}
}

// TestCmdPlansArchive_AbsorbedByGitHistory (HF-1): git IS the permanent
// history (changes.md L1), so a row id named in a commit body absorbs the
// row. The corpus deliberately EXCLUDES commits that touched only
// docs/plans — a plan's own bookkeeping commit ("custodian CC-14/15") must
// not absolve its own rows, which would make criterion 3 vacuous.
func TestCmdPlansArchive_AbsorbedByGitHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	storePath := seedPlansFixtureStore(t)
	root := t.TempDir()
	planMD := "# History\n\n" +
		"- [x] Charge handler {id: GH-1, symbol: Charge, skip_test: true}\n"
	// Neither CHANGELOG.md nor ADR.md mentions GH-1.
	gitFixtureForArchive(t, root, "history.md", planMD, "nothing here\n", "")

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Alice", "GIT_AUTHOR_EMAIL=alice@example.com",
			"GIT_COMMITTER_NAME=Alice", "GIT_COMMITTER_EMAIL=alice@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// A plan-only commit naming the row must NOT absorb it.
	if err := os.WriteFile(filepath.Join(root, "docs", "plans", "history.md"), []byte(planMD+"\n<!-- touched -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "docs/plans/history.md")
	git("commit", "-q", "-m", "plans: GH-1 bookkeeping")
	if corpus, err := gitHistoryCorpus(root, "docs/plans/history.md"); err != nil {
		t.Fatalf("gitHistoryCorpus: %v", err)
	} else if idAbsorbed(corpus, "GH-1") {
		t.Error("a commit touching only docs/plans must not absorb its own row")
	}

	// A commit touching real code and naming the row DOES absorb it.
	if err := os.WriteFile(filepath.Join(root, "charge.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "charge.go")
	git("commit", "-q", "-m", "feat(charge): the handler\n\nRefs: GH-1")
	corpus, err := gitHistoryCorpus(root, "docs/plans/history.md")
	if err != nil {
		t.Fatalf("gitHistoryCorpus: %v", err)
	}
	if !idAbsorbed(corpus, "GH-1") {
		t.Errorf("a commit body naming GH-1 must absorb it; corpus=%q", corpus)
	}

	// End to end: the archive now succeeds on git history alone. (The plan
	// file itself must be clean — commit the marker edit's descendant state.)
	var out, errBuf bytes.Buffer
	code := run(archiveArgs("archive", root, storePath, "history.md"), &out, &errBuf)
	if code != 0 {
		t.Fatalf("archive absorbed by git history should succeed; exit %d stderr=%s", code, errBuf.String())
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "plans", "archive", "history.md")); err != nil {
		t.Errorf("plan must have moved to docs/plans/archive/: %v", err)
	}
}
