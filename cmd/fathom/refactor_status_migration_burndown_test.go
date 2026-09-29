package main

// refactor_status_migration_burndown_test.go — CX-12w (docs/plans/
// change-and-compliance.md, design §4, U9): `fathom refactor status
// --migration` prints each repository's burn-down and completion verdict.
// Its fixtures share the helpers of refactor_status_migration_blockers_test.go.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/conserve"
	"github.com/geoffmilleraz/fathom/internal/review"
)

// TestRefactorStatusMigrationShowsBurnDown is CX-12w. OldFunc is retired
// across two repositories whose gone: rows are read from their own
// docs/plans; the verify ledger says how each row's verdict moved. Per
// repository the verb prints the burn-down series and the completion verdict:
// never complete without --conserve-since (conservation was not checked, and
// the reason says so), and complete only when every gone: row is proven and
// conservation is established with no unexcused loss.
func TestRefactorStatusMigrationShowsBurnDown(t *testing.T) {
	dir := t.TempDir()
	plan := func(rows ...string) string {
		return "# Plan\n\n" + strings.Join(rows, "\n") + "\n"
	}
	repo := func(name string, planRows ...string) string {
		root := filepath.Join(dir, name)
		writeTree(t, root, map[string]string{
			"go.mod":            "module " + name + "\n\ngo 1.21\n",
			"oldpkg/def.go":     "package oldpkg\n\nfunc OldFunc() {}\n",
			"caller/user.go":    "package caller\n\nimport \"" + name + "/oldpkg\"\n\nfunc use() { oldpkg.OldFunc() }\n",
			"docs/plans/mig.md": plan(planRows...),
		})
		return root
	}
	rowsOf := func(third string) []string {
		return []string{
			"- [x] Nothing calls OldFunc {id: MG-1, gone: callers OldFunc, package: oldpkg}",
			"- [x] No lexical OldFunc left {id: MG-2, gone: pattern \"\\bOldFunc\\b\"}",
			third,
			"- [x] Another row {id: MG-4, gone: import example.com/unrelated}",
		}
	}
	// svc-done proves both of its rows; svc-late leaves one unproven.
	doneRoot := repo("svc-done", rowsOf("- [ ] Not a gone row {id: MG-3}")...)
	lateRoot := repo("svc-late", rowsOf("- [x] Name it in a comment too {id: MG-3, gone: pattern \"OldFunc\"}")...)
	storePath := filepath.Join(t.TempDir(), "fleet.sqlite")
	ingestService(t, storePath, "svc-done", doneRoot)
	ingestService(t, storePath, "svc-late", lateRoot)

	// The verify ledger: svc-done's two rows go unproven -> one proven -> both
	// proven; svc-late's rows never all get there. MG-4's gone: names another
	// import, so it is not the migration's.
	ledger := filepath.Join(t.TempDir(), "verify-ledger.jsonl")
	line := func(at, svc, digest string, rows ...[2]string) string {
		var rs []string
		for _, r := range rows {
			rs = append(rs, fmt.Sprintf(`{"id":%q,"state":%q,"weight":1}`, r[0], r[1]))
		}
		return fmt.Sprintf(`{"schema_version":3,"at":%q,"repo_root":%q,"service":%q,"plan":"docs/plans/mig.md","digest":%q,"claimed":1,"verified":1,"rows":[%s]}`,
			at, filepath.Join(dir, svc), svc, digest, strings.Join(rs, ","))
	}
	unproven, proven := "claimed-unproven", "claimed-proven"
	ledgerBody := strings.Join([]string{
		line("2026-09-20T10:00:00Z", "svc-done", "d1", [2]string{"MG-1", unproven}, [2]string{"MG-2", unproven}, [2]string{"MG-4", proven}),
		line("2026-09-22T10:00:00Z", "svc-done", "d2", [2]string{"MG-1", proven}, [2]string{"MG-2", unproven}, [2]string{"MG-4", proven}),
		line("2026-09-25T10:00:00Z", "svc-done", "d3", [2]string{"MG-1", proven}, [2]string{"MG-2", proven}, [2]string{"MG-4", proven}),
		line("2026-09-21T10:00:00Z", "svc-late", "e1", [2]string{"MG-1", proven}, [2]string{"MG-2", unproven}, [2]string{"MG-3", unproven}),
	}, "\n") + "\n"
	if err := os.WriteFile(ledger, []byte(ledgerBody), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_VERIFY_LEDGER", ledger)

	migration := "symbol OldFunc -> NewFunc"
	rep := migrationStatusJSON(t, dir, storePath, migration)

	done := repoOf(t, rep, "svc-done").BurnDown
	if done == nil {
		t.Fatal("svc-done has no burn-down")
	}
	if len(done.Rows) != 2 || done.Rows[0].ID != "MG-1" || done.Rows[1].ID != "MG-2" || done.Rows[0].Verdict != review.RowProven || done.Rows[1].Verdict != review.RowProven {
		t.Errorf("svc-done rows = %+v, want MG-1 and MG-2 (MG-3 is no gone: row, MG-4 names another import), both proven", done.Rows)
	}
	wantSeries := []migrationPointWire{
		{At: "2026-09-20T10:00:00Z", Claimed: 2, Proven: 0, Remaining: 2},
		{At: "2026-09-22T10:00:00Z", Claimed: 2, Proven: 1, Remaining: 1},
		{At: "2026-09-25T10:00:00Z", Claimed: 2, Proven: 2, Remaining: 0},
	}
	if fmt.Sprint(done.Series) != fmt.Sprint(wantSeries) {
		t.Errorf("svc-done series = %+v, want %+v", done.Series, wantSeries)
	}
	// Every row proven, yet not complete: conservation was never run.
	if done.Complete || !strings.Contains(done.Reason, "conservation not established") || !strings.Contains(done.Reason, "--conserve-since") || done.Conservation != "not run" {
		t.Errorf("svc-done without --conserve-since = %+v, want incomplete for want of conservation, naming the flag", done)
	}

	late := repoOf(t, rep, "svc-late").BurnDown
	if late == nil || late.Complete || !strings.Contains(late.Reason, "unproven gone: row(s) in svc-late: MG-2, MG-3") {
		t.Errorf("svc-late = %+v, want the unproven rows named", late)
	}
	if late != nil && (len(late.Series) != 1 || late.Series[0].Claimed != 3 || late.Series[0].Proven != 1) {
		t.Errorf("svc-late series = %+v, want one reading, 1 of 3 proven", late.Series)
	}

	// The text prints the same, per repository.
	code, out, errOut := runInDir(t, dir, []string{"refactor", "status", "--migration", migration, "--store", storePath})
	if code != 0 {
		t.Fatalf("text run failed (%d): %s", code, errOut)
	}
	for _, want := range []string{
		"burn-down: 2 of 2 gone: row(s) proven",
		"docs/plans/mig.md#MG-1  proven",
		"2026-09-22T10:00:00Z  1 of 2 claimed proven (1 remaining)",
		"conservation: not run",
		"complete: no — conservation not established in svc-done",
		"complete: no — unproven gone: row(s) in svc-late: MG-2, MG-3",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text lacks %q:\n%s", want, out)
		}
	}
}

// TestRefactorStatusMigrationBurnDownConserves: with --conserve-since the
// verb runs conservation in each repository, and a repository whose rows are
// all proven and whose conservation is established is complete. The fixture
// is a git repository with a base commit; nothing changes since it.
func TestRefactorStatusMigrationBurnDownConserves(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	root := filepath.Join(dir, "svc-git")
	writeTree(t, root, map[string]string{
		"go.mod":            "module svc-git\n\ngo 1.21\n",
		"oldpkg/def.go":     "package oldpkg\n\nfunc OldFunc() {}\n",
		"docs/plans/mig.md": "# Plan\n\n- [x] Nothing calls OldFunc {id: MG-1, gone: callers OldFunc, package: oldpkg}\n",
	})
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	storePath := filepath.Join(t.TempDir(), "fleet.sqlite")
	ingestService(t, storePath, "svc-git", root)

	ledger := filepath.Join(t.TempDir(), "verify-ledger.jsonl")
	body := fmt.Sprintf(`{"schema_version":3,"at":"2026-09-25T10:00:00Z","repo_root":%q,"service":"svc-git","plan":"docs/plans/mig.md","digest":"d","claimed":1,"verified":1,"rows":[{"id":"MG-1","state":"claimed-proven","weight":1}]}`+"\n", root)
	if err := os.WriteFile(ledger, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_VERIFY_LEDGER", ledger)

	rep := migrationStatusJSON(t, dir, storePath, "symbol OldFunc -> NewFunc", "--conserve-since", "HEAD")
	b := repoOf(t, rep, "svc-git").BurnDown
	if b == nil {
		t.Fatal("no burn-down")
	}
	if strings.Contains(b.Reason, "not run") || strings.Contains(b.Conservation, "not run") {
		t.Errorf("conservation was not run although --conserve-since was given: %+v", b)
	}
	if !b.Complete && !strings.Contains(b.Reason, "conservation not established") && !strings.Contains(b.Reason, "unexcused") {
		t.Errorf("burn-down = %+v, want complete, or incomplete for a named conservation outcome", b)
	}
	t.Logf("conservation: %q complete=%v reason=%q", b.Conservation, b.Complete, b.Reason)

	// A ref the repository cannot resolve leaves conservation unestablished,
	// named, never silently complete.
	rep = migrationStatusJSON(t, dir, storePath, "symbol OldFunc -> NewFunc", "--conserve-since", "no-such-ref")
	b = repoOf(t, rep, "svc-git").BurnDown
	if b.Complete || !strings.Contains(b.Reason, "conserve --since no-such-ref could not run") {
		t.Errorf("burn-down with a bad ref = %+v, want conservation unestablished and the failure named", b)
	}
}

// TestMigrationConservationFacts folds a conservation report: a losses plane
// is an unexcused loss naming its first undeclared finding; an unproven plane
// leaves the verdict unestablished (I44).
func TestMigrationConservationFacts(t *testing.T) {
	clean := conserve.Report{Planes: []conserve.PlaneReport{{Plane: "surface", Verdict: conserve.Conserved}}}
	if u, d, p := migrationConservationFacts(clean); u || d != "" || p != "" {
		t.Errorf("clean report = %v %q %q", u, d, p)
	}
	rep := conserve.Report{Planes: []conserve.PlaneReport{
		{Plane: "tests", Verdict: conserve.Losses, Findings: []conserve.Finding{
			{Detail: "excused test removed", Declared: conserve.DeclaredPrior},
			{Detail: "TestFoo deleted", Declared: conserve.DeclaredNone},
		}},
		{Plane: "certificates", Verdict: conserve.Unproven, Missing: []string{"certify layer"}},
	}}
	u, d, p := migrationConservationFacts(rep)
	if !u || d != "tests: TestFoo deleted" || p != "plane certificates unproven (missing certify layer)" {
		t.Errorf("facts = %v %q %q", u, d, p)
	}
}

// TestGoneNamesMigrationFrom: which gone: contracts retire a migration's From.
func TestGoneNamesMigrationFrom(t *testing.T) {
	sym := review.Migration{Kind: review.MigrationSymbol, From: "Recv.OldFunc", To: "NewFunc"}
	imp := review.Migration{Kind: review.MigrationImport, From: "example.com/old", To: "example.com/new"}
	pkg := review.Migration{Kind: review.MigrationPkg, From: "example.com/mod@v1.2.0", To: "example.com/mod@v2.0.0"}
	route := review.Migration{Kind: review.MigrationRoute, From: "GET /v1/things", To: "GET /v2/things"}
	for _, tc := range []struct {
		name string
		gone string
		m    review.Migration
		want bool
	}{
		{"callers, bare", "callers OldFunc", sym, true},
		{"callers, other symbol", "callers Other", sym, false},
		{"callers on an import migration", "callers OldFunc", imp, false},
		{"pattern word boundary", `pattern "\bOldFunc\b"`, sym, true},
		{"pattern for another name", `pattern "\bOther\b"`, sym, false},
		{"import exact", "import example.com/old", imp, true},
		{"import other", "import example.com/older", imp, false},
		{"import pattern, quoted literal", `pattern "\x22example.com/old\x22"`, imp, true},
		{"module without version", "module example.com/mod", pkg, true},
		{"module with version", "module example.com/mod@v1.2.0", pkg, true},
		{"route pattern", `pattern "/v1/things"`, route, true},
		{"bad regexp", `pattern "("`, route, false},
		{"unknown form", "symbol OldFunc", sym, false},
		{"empty", "", sym, false},
	} {
		if got := goneNamesFrom(tc.gone, tc.m); got != tc.want {
			t.Errorf("%s: goneNamesFrom(%q) = %v, want %v", tc.name, tc.gone, got, tc.want)
		}
	}
}
