package main

// tests_select_inputs_test.go — TQ-33 and TQ-34 (docs/plans/test-intelligence.md,
// ADR-090): a test that reads a plan, a changelog or the source tree through a
// path computed at run time is invisible to the call graph, so an edit there
// selected nothing and the push gate found the failure. `tests select` now
// joins the files Go's testlog observed each test reading, and selects a new
// test function in a changed test file even before the index knows it.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func inputsFixture(t *testing.T) (root, storePath string) {
	t.Helper()
	root = t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/inputsfix\n\ngo 1.22\n")
	write("docs/guide.md", "guide\n")
	write("p/p.go", "package p\n\nfunc P() int { return 1 }\n")
	write("p/p_test.go", `package p

import (
	"os"
	"path/filepath"
	"testing"
)

func docsPath(name string) string { return filepath.Join("..", "docs", name) }

func TestReadsGuide(t *testing.T) { _, _ = os.ReadFile(docsPath("guide.md")) }

func TestPlain(t *testing.T) { _ = P() }
`)
	write("q/q.go", "package q\n\nfunc Q() int { return 2 }\n")
	write("q/q_test.go", "package q\n\nimport \"testing\"\n\nfunc TestListsDocs(t *testing.T) { _ = Q() }\n")
	storePath = filepath.Join(t.TempDir(), "s.sqlite")
	var ing bytes.Buffer
	if code := run([]string{"ingest", "--service", "in-svc", "--path", root, "--store", storePath}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}
	return root, storePath
}

// writeInputs lays out the observation scripts/test-inputs.sh writes: p run
// test by test, q as a package that lists docs/.
func writeInputs(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "test-inputs")
	files := map[string]string{
		"meta.json": `{"commit":"0123456789abcdef0123456789abcdef01234567","at":"2026-10-31T20:00:00Z","root":"` + root + `","packages":{` +
			`"p":{"exit":0,"per_test":true,"tests":{"TestReadsGuide":0,"TestPlain":0}},"q":{"exit":0}}}`,
		"pkg/p/_package.log":             "# test log\nopen ../docs/guide.md\n",
		"pkg/p/_test.TestReadsGuide.log": "# test log\nopen ../docs/guide.md\n",
		"pkg/p/_test.TestPlain.log":      "# test log\n",
		"pkg/q/_package.log":             "# test log\nopen ../docs\n",
	}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

type selectJSON struct {
	Invocations []struct {
		Package string   `json:"package"`
		Tests   []string `json:"tests"`
		Whole   bool     `json:"whole"`
		Command string   `json:"command"`
	} `json:"invocations"`
	Inputs struct {
		Dir   string `json:"dir"`
		Hits  []struct{ File, Package, Test, Via string }
		Whole []string `json:"whole_packages"`
	} `json:"inputs"`
	NewTests map[string][]string `json:"new_tests"`
}

func runSelectJSON(t *testing.T, root, storePath string, extra []string, changed ...string) (selectJSON, string) {
	t.Helper()
	old := testsSelectRoot
	testsSelectRoot = root
	t.Cleanup(func() { testsSelectRoot = old })
	setEconomicsLedger(t, filepath.Join(t.TempDir(), "econ.jsonl"))
	var out, errBuf bytes.Buffer
	args := append([]string{"--since", "HEAD~1", "--service", "in-svc", "--store", storePath, "--safe", "--json"}, extra...)
	if code := runTestsSelect(args, fakeGitRunner(changed), &out, &errBuf); code != 0 {
		t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	var got selectJSON
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json: %v\n%s", err, out.String())
	}
	return got, errBuf.String()
}

func invocationFor(got selectJSON, pkg string) (tests []string, whole bool, ok bool) {
	for _, inv := range got.Invocations {
		if inv.Package == pkg {
			return inv.Tests, inv.Whole, true
		}
	}
	return nil, false, false
}

func TestTestsSelectSelectsTheTestThatReadAChangedFile(t *testing.T) {
	root, storePath := inputsFixture(t)
	dir := writeInputs(t, root)
	got, _ := runSelectJSON(t, root, storePath, []string{"--inputs", dir}, "docs/guide.md")
	tests, whole, ok := invocationFor(got, "p")
	if !ok || whole || strings.Join(tests, ",") != "TestReadsGuide" {
		t.Fatalf("p invocation = %v whole=%v ok=%v; want TestReadsGuide alone (TestPlain read nothing):\n%+v", tests, whole, ok, got.Invocations)
	}
	if len(got.Inputs.Hits) == 0 || got.Inputs.Hits[0].File != "docs/guide.md" {
		t.Fatalf("inputs hits = %+v, want docs/guide.md", got.Inputs.Hits)
	}
}

func TestTestsSelectRunsAWholePackageThatListedAChangedDirectory(t *testing.T) {
	root, storePath := inputsFixture(t)
	dir := writeInputs(t, root)
	// fakeGitRunner answers every git call with the same list, so docs/new.md
	// reads as added (a listing change) as well as changed.
	got, _ := runSelectJSON(t, root, storePath, []string{"--inputs", dir}, "docs/new.md")
	_, whole, ok := invocationFor(got, "q")
	if !ok || !whole {
		t.Fatalf("q must run whole: its package log opened docs/, whose listing changed:\n%+v", got.Invocations)
	}
	for _, inv := range got.Invocations {
		if inv.Package == "q" && inv.Command != "go test ./q" {
			t.Fatalf("whole-package command = %q, want %q", inv.Command, "go test ./q")
		}
	}
}

func TestTestsSelectSelectsANewTestTheIndexDoesNotKnow(t *testing.T) {
	root, storePath := inputsFixture(t)
	p := filepath.Join(root, "p", "p_test.go")
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(body, []byte("\nfunc TestBrandNew(t *testing.T) {}\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	got, _ := runSelectJSON(t, root, storePath, nil, "p/p_test.go")
	tests, _, ok := invocationFor(got, "p")
	if !ok || !strings.Contains(strings.Join(tests, ","), "TestBrandNew") {
		t.Fatalf("a new test in a changed test file must be selected by name: %+v", got.Invocations)
	}
	if names := got.NewTests["p"]; len(names) != 1 || names[0] != "TestBrandNew" {
		t.Fatalf("new_tests = %v, want p: [TestBrandNew]", got.NewTests)
	}
}

func TestTestsSelectSaysWhenNoTestInputsWereObserved(t *testing.T) {
	root, storePath := inputsFixture(t)
	_, stderr := runSelectJSON(t, root, storePath, []string{"--inputs", filepath.Join(t.TempDir(), "none")}, "docs/guide.md")
	if !strings.Contains(stderr, "no observed test inputs") || !strings.Contains(stderr, "task test:inputs") {
		t.Fatalf("stderr must say no inputs were observed and how to observe them:\n%s", stderr)
	}
}

// TQ-36: with no --store, the selection reads the store the daemon records,
// not the workspace file an earlier ingest left behind, and says which.
func TestTestsSelectReadsTheDaemonsRecordedStoreByDefault(t *testing.T) {
	f := newConsoleFixture(t)
	t.Setenv("FATHOM_HTTP", f.ts.URL)
	setEconomicsLedger(t, filepath.Join(t.TempDir(), "econ.jsonl"))
	var out, errBuf bytes.Buffer
	args := []string{"--since", "HEAD~1", "--service", "payment-service", "--json", "--inputs", filepath.Join(t.TempDir(), "none")}
	if code := runTestsSelect(args, fakeGitRunner(nil), &out, &errBuf); code != 0 {
		t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	var got struct {
		Store struct{ Path, Rule string } `json:"store"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Store.Rule != "daemon" || got.Store.Path != consoleFixtureStorePath(t) {
		t.Fatalf("store = %+v, want the daemon's recorded store %s", got.Store, consoleFixtureStorePath(t))
	}
}

// F2 and F10 (review of ADR-090): an added file's listing change belongs to
// the deepest directory that existed at --since, and a test file under
// testdata/, _x/ or .x/ is a fixture go list never runs.
func TestTestsSelectChangesListingDirAndFixtureTests(t *testing.T) {
	git := func(args ...string) ([]byte, error) {
		switch {
		case args[0] == "cat-file":
			if args[2] == "HEAD:docs/plans" {
				return nil, nil
			}
			return nil, os.ErrNotExist
		case args[0] == "ls-files":
			return []byte("docs/plans/new/deeper/b.md\x00"), nil
		default:
			return []byte("docs/plans/new/deeper/b.md\x00"), nil
		}
	}
	changes, err := testsSelectChanges(git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || !changes[0].Listing || changes[0].ListingDir != "docs/plans" {
		t.Fatalf("changes = %+v, want docs/plans/new/deeper/b.md listing docs/plans", changes)
	}
	for rel, want := range map[string]bool{
		"internal/fusion/testdata/reachfix/a_test.go": true,
		"internal/_old/a_test.go":                     true,
		".claude/worktrees/w/a_test.go":               true,
		"internal/fusion/a_test.go":                   false,
		"a_test.go":                                   false,
	} {
		if got := ignoredByGoList(rel); got != want {
			t.Errorf("ignoredByGoList(%q) = %v, want %v", rel, got, want)
		}
	}
}
