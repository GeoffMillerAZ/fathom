package rulereplay

import (
	"context"
	"testing"
)

const singleImportDirectionRule = `[{id: "a-not-b", kind: "import_direction", sentence: "a must not import b", from: ["a"], to: ["b"]}]`

// TestReplay_NewVsInherited builds a four-commit repo — clean, introduce a
// violation, leave it untouched, fix it — and proves the report classifies
// each occurrence correctly: c1 clean, c2 the violation NEW, c3 the SAME
// violation INHERITED (present at its parent), c4 clean again. A plain
// clean/introduce/fix skeleton alone never exercises "inherited" — nothing
// in that shape has a violation that survives unchanged into a second
// commit — so this fixture adds the untouched middle commit deliberately.
func TestReplay_NewVsInherited(t *testing.T) {
	repo := newTestRepo(t)
	repo.goMod("tmpmod")
	repo.guardCue(singleImportDirectionRule)
	repo.write("a/a.go", "package a\n\nfunc F() {}\n")
	repo.write("b/b.go", "package b\n\nfunc G() {}\n")
	c1 := repo.commit("c1: clean")

	repo.write("a/a.go", "package a\n\nimport \"tmpmod/b\"\n\nfunc F() { b.G() }\n")
	c2 := repo.commit("c2: introduce a-not-b violation")

	repo.write("README.md", "unrelated touch\n")
	c3 := repo.commit("c3: unrelated change, violation persists")

	repo.write("a/a.go", "package a\n\nfunc F() {}\n")
	c4 := repo.commit("c4: fix — remove the import")

	report, err := Replay(context.Background(), Options{
		RepoRoot: repo.dir,
		RuleFile: repo.guardCuePath(),
		Since:    "3", // last 3 commits: c2, c3, c4 (c1 is the baseline parent of c2)
	})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if got, want := report.TotalCommits, 3; got != want {
		t.Fatalf("TotalCommits = %d, want %d", got, want)
	}
	if len(report.Commits) != 3 {
		t.Fatalf("len(Commits) = %d, want 3", len(report.Commits))
	}

	byPrefix := map[string]CommitResult{}
	for _, c := range report.Commits {
		byPrefix[c.SHA] = c
	}
	if _, ok := byPrefix[c1]; ok {
		t.Errorf("c1 (%s) is the baseline parent, not part of the 3-commit range — should not appear as a report row", c1)
	}
	got2, ok := byPrefix[c2]
	if !ok {
		t.Fatalf("c2 (%s) missing from report; have %v", c2, keysOf(byPrefix))
	}
	if len(got2.Violations) != 1 {
		t.Fatalf("c2 violations = %+v, want exactly 1", got2.Violations)
	}
	if !got2.Violations[0].New {
		t.Errorf("c2's violation should be NEW (absent at c1), got New=false")
	}
	if got2.Violations[0].RuleID != "a-not-b" || got2.Violations[0].From != "a" || got2.Violations[0].To != "b" {
		t.Errorf("c2 violation = %+v, want {a-not-b a b ...}", got2.Violations[0])
	}

	got3, ok := byPrefix[c3]
	if !ok {
		t.Fatalf("c3 (%s) missing from report", c3)
	}
	if len(got3.Violations) != 1 {
		t.Fatalf("c3 violations = %+v, want exactly 1", got3.Violations)
	}
	if got3.Violations[0].New {
		t.Errorf("c3's violation should be INHERITED (unchanged from c2), got New=true")
	}

	got4, ok := byPrefix[c4]
	if !ok {
		t.Fatalf("c4 (%s) missing from report", c4)
	}
	if len(got4.Violations) != 0 {
		t.Errorf("c4 violations = %+v, want none (fixed)", got4.Violations)
	}

	if report.TotalViolations != 2 { // c2 + c3, one each
		t.Errorf("TotalViolations = %d, want 2", report.TotalViolations)
	}
	if report.NewViolations != 1 { // only c2's
		t.Errorf("NewViolations = %d, want 1", report.NewViolations)
	}
	if len(report.RuleIDs) != 1 || report.RuleIDs[0] != "a-not-b" {
		t.Errorf("RuleIDs = %v, want [a-not-b]", report.RuleIDs)
	}
	if len(report.SkippedRuleIDs) != 0 {
		t.Errorf("SkippedRuleIDs = %v, want none", report.SkippedRuleIDs)
	}
}

// TestReplay_ZeroViolations proves the "nothing caught" honest-reading
// path: a repo whose rule is never violated across the whole replay range
// reports TotalViolations == 0 and NewViolations == 0, with every commit
// carrying an empty Violations slice — never a fabricated finding.
func TestReplay_ZeroViolations(t *testing.T) {
	repo := newTestRepo(t)
	repo.goMod("tmpmod")
	repo.guardCue(singleImportDirectionRule)
	repo.write("a/a.go", "package a\n\nfunc F() {}\n")
	repo.write("b/b.go", "package b\n\nfunc G() {}\n")
	repo.commit("c1: clean")
	repo.write("b/b.go", "package b\n\nfunc G() {}\n\nfunc H() {}\n")
	repo.commit("c2: still clean")

	report, err := Replay(context.Background(), Options{
		RepoRoot: repo.dir,
		RuleFile: repo.guardCuePath(),
		Since:    "2",
	})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if report.TotalCommits != 2 {
		t.Fatalf("TotalCommits = %d, want 2", report.TotalCommits)
	}
	if report.TotalViolations != 0 || report.NewViolations != 0 {
		t.Fatalf("TotalViolations=%d NewViolations=%d, want 0/0", report.TotalViolations, report.NewViolations)
	}
	for _, c := range report.Commits {
		if len(c.Violations) != 0 {
			t.Errorf("commit %s has violations %+v, want none", c.SHA, c.Violations)
		}
	}
}

// TestReplay_RefSince exercises the git-ref form of --since (as opposed to
// the bare-integer form the other tests use): Since set to the first
// commit's own SHA replays everything after it, ref..HEAD.
func TestReplay_RefSince(t *testing.T) {
	repo := newTestRepo(t)
	repo.goMod("tmpmod")
	repo.guardCue(singleImportDirectionRule)
	repo.write("a/a.go", "package a\n\nfunc F() {}\n")
	c1 := repo.commit("c1: clean")
	repo.write("a/a.go", "package a\n\nimport \"tmpmod/b\"\n\nfunc F() { _ = b.G }\n")
	repo.write("b/b.go", "package b\n\nfunc G() {}\n")
	c2 := repo.commit("c2: introduce violation")

	report, err := Replay(context.Background(), Options{
		RepoRoot: repo.dir,
		RuleFile: repo.guardCuePath(),
		Since:    c1,
	})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if report.RangeSpec != c1+"..HEAD" {
		t.Errorf("RangeSpec = %q, want %q", report.RangeSpec, c1+"..HEAD")
	}
	if report.TotalCommits != 1 {
		t.Fatalf("TotalCommits = %d, want 1", report.TotalCommits)
	}
	if report.Commits[0].SHA != c2 {
		t.Errorf("commit SHA = %s, want %s", report.Commits[0].SHA, c2)
	}
	if len(report.Commits[0].Violations) != 1 || !report.Commits[0].Violations[0].New {
		t.Errorf("c2 violations = %+v, want exactly one, New=true", report.Commits[0].Violations)
	}
}

// TestReplay_RootCommitHasNoBaseline proves a range that includes the
// repository's very first commit treats it as having no parent to
// inherit from — every violation there is New.
func TestReplay_RootCommitHasNoBaseline(t *testing.T) {
	repo := newTestRepo(t)
	repo.goMod("tmpmod")
	repo.guardCue(singleImportDirectionRule)
	repo.write("a/a.go", "package a\n\nimport \"tmpmod/b\"\n\nfunc F() { _ = b.G }\n")
	repo.write("b/b.go", "package b\n\nfunc G() {}\n")
	c1 := repo.commit("c1: violation from the very first commit")

	report, err := Replay(context.Background(), Options{
		RepoRoot: repo.dir,
		RuleFile: repo.guardCuePath(),
		Since:    "1",
	})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if report.TotalCommits != 1 || report.Commits[0].SHA != c1 {
		t.Fatalf("report = %+v", report)
	}
	if len(report.Commits[0].Violations) != 1 || !report.Commits[0].Violations[0].New {
		t.Errorf("root commit's violation should be New (no parent), got %+v", report.Commits[0].Violations)
	}
}

func keysOf(m map[string]CommitResult) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
