package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// selectOut is one `tests select --json` run's selection, the reach it
// ranked, what it took out as standard-library dispatch only, the index it
// read, and its stderr.
type selectOut struct {
	Tests        []string // the selection's tests, bare names, sorted
	Ranked       []string // ranked_tests, bare names, sorted
	DispatchOnly struct {
		Methods  []string `json:"methods"`
		Tests    []string `json:"tests"`
		Selected *bool    `json:"selected"`
	}
	Index struct {
		Commit    string `json:"commit"`
		Head      string `json:"head"`
		HoldsHead *bool  `json:"holds_head"`
	}
	HasIndex bool
	Stderr   string
}

func bareTestNames(keys []string) []string {
	var names []string
	for _, k := range keys {
		names = append(names, k[strings.LastIndex(k, ".")+1:])
	}
	sort.Strings(names)
	return names
}

// ingestTo ingests the working tree into a fresh store and returns its path.
func (r *selectRepo) ingestTo(service string) string {
	r.t.Helper()
	storePath := filepath.Join(r.t.TempDir(), "s.sqlite")
	var ing bytes.Buffer
	if code := run([]string{"ingest", "--service", service, "--path", r.root, "--store", storePath}, &ing, &ing); code != 0 {
		r.t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}
	return storePath
}

// head is the work tree's HEAD commit.
func (r *selectRepo) head() string {
	r.t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = r.root
	out, err := cmd.Output()
	if err != nil {
		r.t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// selectWith runs `tests select --since HEAD --json` plus extra against
// storePath; changed answers `git diff --name-only`, and `git rev-parse`
// runs in the work tree.
func (r *selectRepo) selectWith(service, storePath string, extra []string, changed ...string) selectOut {
	r.t.Helper()
	old := testsSelectRoot
	testsSelectRoot = r.root
	r.t.Cleanup(func() { testsSelectRoot = old })
	setEconomicsLedger(r.t, filepath.Join(r.t.TempDir(), "econ.jsonl"))
	diff := fakeGitRunner(changed)
	gitRunner := func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "rev-parse" {
			cmd := exec.Command("git", args...)
			cmd.Dir = r.root
			return cmd.Output()
		}
		return diff(args...)
	}
	var out, errBuf bytes.Buffer
	args := append([]string{"--since", "HEAD", "--service", service, "--store", storePath, "--json"}, extra...)
	if code := runTestsSelect(args, gitRunner, &out, &errBuf); code != 0 {
		r.t.Fatalf("exit %d: %s", code, errBuf.String())
	}
	var payload struct {
		Selection struct {
			Tests []string `json:"tests"`
		} `json:"selection"`
		Ranked []struct {
			Test string `json:"test"`
		} `json:"ranked_tests"`
		DispatchOnly *json.RawMessage `json:"stdlib_dispatch_only"`
		Index        *json.RawMessage `json:"index"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		r.t.Fatalf("unmarshal: %v\n%s", err, out.String())
	}
	res := selectOut{Tests: bareTestNames(payload.Selection.Tests), Stderr: errBuf.String()}
	var ranked []string
	for _, rt := range payload.Ranked {
		ranked = append(ranked, rt.Test)
	}
	res.Ranked = bareTestNames(ranked)
	if payload.DispatchOnly != nil {
		if err := json.Unmarshal(*payload.DispatchOnly, &res.DispatchOnly); err != nil {
			r.t.Fatalf("stdlib_dispatch_only: %v", err)
		}
	}
	if payload.Index != nil {
		res.HasIndex = true
		if err := json.Unmarshal(*payload.Index, &res.Index); err != nil {
			r.t.Fatalf("index: %v", err)
		}
	}
	return res
}

// TestTestsSelectSafeSelectsDispatchOnlyTestsByDefault is O887's second
// half as the --safe contract has it (TQ-M6: --safe is never shipped less
// safe): a test that reaches a changed symbol only through a dispatch into
// error's Error method is still selected by default, and one line counts it
// and names the method (not fmt.Stringer's String, which it also dispatches
// to but which reaches no change). --skip-stdlib-dispatch is the opt-out
// that leaves it out. A test that calls the method directly stays selected
// either way, and so does the helper that method calls: the reach is read as
// a whole, not only at the seed.
func TestTestsSelectSafeSelectsDispatchOnlyTestsByDefault(t *testing.T) {
	files := map[string]string{
		"go.mod":     "module example.com/sd\n\ngo 1.22\n",
		"use/use.go": "package use\n\nimport \"fmt\"\n\nfunc Describe(err error) string { return err.Error() }\n\nfunc Label(s fmt.Stringer) string { return s.String() }\n",
		"q/q.go":     "package q\n\ntype QErr struct{}\n\nfunc (*QErr) Error() string { return msg() }\n\nfunc msg() string { return \"q\" }\n\nfunc NewQ() error { return &QErr{} }\n\ntype QName struct{}\n\nfunc (QName) String() string { return \"name\" }\n",
		"q/q_test.go": `package q_test

import (
	"testing"

	"example.com/sd/q"
	"example.com/sd/use"
)

func TestQ(t *testing.T) { _ = use.Describe(q.NewQ()) + use.Label(q.QName{}) }

func TestQDirect(t *testing.T) { _ = (&q.QErr{}).Error() }
`,
	}
	r := newSelectRepo(t, files)
	r.write("q/q.go", strings.Replace(files["q/q.go"], `return "q"`, `return "Q"`, 1))
	storePath := r.ingestTo("sd")

	got := r.selectWith("sd", storePath, []string{"--safe"}, "q/q.go")
	if strings.Join(got.Tests, ",") != "TestQ,TestQDirect" {
		t.Errorf("--safe selected %v, want [TestQ TestQDirect] (a dispatch-only reach is still a reach)", got.Tests)
	}
	if strings.Join(got.Ranked, ",") != "TestQ,TestQDirect" {
		t.Errorf("--safe ranked %v, want [TestQ TestQDirect]", got.Ranked)
	}
	const line = "note: tests select: 1 of these tests reach the change only through error.Error dispatch; --skip-stdlib-dispatch leaves them out"
	if strings.Count(got.Stderr, line) != 1 {
		t.Errorf("stderr does not carry the dispatch line once; want %q in:\n%s", line, got.Stderr)
	}
	if strings.Join(got.DispatchOnly.Methods, ",") != "error.Error" || strings.Join(bareTestNames(got.DispatchOnly.Tests), ",") != "TestQ" || got.DispatchOnly.Selected == nil || !*got.DispatchOnly.Selected {
		t.Errorf("stdlib_dispatch_only = %+v, want methods [error.Error], tests [TestQ], selected true", got.DispatchOnly)
	}

	skip := r.selectWith("sd", storePath, []string{"--safe", "--skip-stdlib-dispatch"}, "q/q.go")
	if strings.Join(skip.Tests, ",") != "TestQDirect" || strings.Join(skip.Ranked, ",") != "TestQDirect" {
		t.Errorf("--skip-stdlib-dispatch selected %v and ranked %v, want [TestQDirect] (TestQ reaches msg only through error.Error dispatch)", skip.Tests, skip.Ranked)
	}
	const skipLine = "note: tests select: 1 test(s) reach the change only through error.Error dispatch and are left out (--skip-stdlib-dispatch)"
	if strings.Count(skip.Stderr, skipLine) != 1 {
		t.Errorf("stderr does not carry the opt-out line once; want %q in:\n%s", skipLine, skip.Stderr)
	}
	if strings.Join(bareTestNames(skip.DispatchOnly.Tests), ",") != "TestQ" || skip.DispatchOnly.Selected == nil || *skip.DispatchOnly.Selected {
		t.Errorf("stdlib_dispatch_only = %+v, want tests [TestQ], selected false", skip.DispatchOnly)
	}

	// Without --safe the reach reads as before: the ranked list keeps TestQ
	// and nothing is counted.
	plain := r.selectWith("sd", storePath, nil, "q/q.go")
	if strings.Join(plain.Ranked, ",") != "TestQ,TestQDirect" || strings.Contains(plain.Stderr, "only through") {
		t.Errorf("without --safe ranked %v, want [TestQ TestQDirect] and no dispatch line:\n%s", plain.Ranked, plain.Stderr)
	}
}

// TestTestsSelectSaysWhenTheIndexIsNotTheDiffHead is O1199: a store whose
// index holds an older commit than HEAD selects from that index, and the
// run says so in one line and in the JSON's index field; an index of HEAD
// says nothing.
func TestTestsSelectSaysWhenTheIndexIsNotTheDiffHead(t *testing.T) {
	r := newSelectRepo(t, map[string]string{
		"go.mod":     "module example.com/ix\n\ngo 1.22\n",
		"lib/lib.go": bodyLibV1,
		"lib/lib_test.go": `package lib

import "testing"

func TestStable(t *testing.T) { _ = Stable() }
`,
	})
	base := r.head()
	stale := r.ingestTo("ix")
	r.write("lib/lib.go", strings.Replace(bodyLibV1, "return 1", "return 3", 1))
	r.commit()
	head := r.head()
	r.write("lib/lib.go", strings.Replace(bodyLibV1, "return 1", "return 4", 1))

	got := r.selectWith("ix", stale, []string{"--safe"}, "lib/lib.go")
	want := "note: tests select: the index holds commit " + shortSHA(base) + ", not HEAD " + shortSHA(head) + ", and this selection reads that index; ingest HEAD into a --store to select for it"
	if strings.Count(got.Stderr, want) != 1 {
		t.Errorf("stderr does not name the index's commit once; want %q in:\n%s", want, got.Stderr)
	}
	if !got.HasIndex || got.Index.Commit != base || got.Index.Head != head || got.Index.HoldsHead == nil || *got.Index.HoldsHead {
		t.Errorf("index = %+v (present %v), want commit %s, head %s, holds_head false", got.Index, got.HasIndex, base, head)
	}

	fresh := r.ingestTo("ix")
	again := r.selectWith("ix", fresh, []string{"--safe"}, "lib/lib.go")
	if strings.Contains(again.Stderr, "the index holds commit") {
		t.Errorf("an index of HEAD still printed the note:\n%s", again.Stderr)
	}
	if !again.HasIndex || again.Index.Commit != head || again.Index.HoldsHead == nil || !*again.Index.HoldsHead {
		t.Errorf("index = %+v, want commit %s, holds_head true", again.Index, head)
	}
}
