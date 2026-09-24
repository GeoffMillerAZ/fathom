package verify

// gone_test.go — CV-27/CV-28 (ADR-044 §3.1, I44): EvaluateGone proves
// every gone: form (import, module, pattern, callers) and stays Unknown,
// never falsely proven, when the index is stale, a typed package failed,
// or a build-tag-excluded file still references the target.

import (
	"strings"
	"testing"
)

// fakeGoneEvidence is a fully-scripted GoneEvidence for EvaluateGone's
// tests: every field defaults to "clean" (index at head, nothing failed,
// no excluded files, no referents) so a test only sets what it needs to
// exercise.
type fakeGoneEvidence struct {
	atHead    bool
	atHeadWhy string
	failed    []string
	goFiles   []GoFile
	modBody   []byte
	modOK     bool
	corpus    []CorpusFile
	nonTest   []GoneReferent
	test      []GoneReferent
}

func (f fakeGoneEvidence) IndexAtHead() (bool, string)   { return f.atHead, f.atHeadWhy }
func (f fakeGoneEvidence) FailedTypedPackages() []string { return f.failed }
func (f fakeGoneEvidence) GoFiles() []GoFile             { return f.goFiles }
func (f fakeGoneEvidence) ModuleFile() ([]byte, bool)    { return f.modBody, f.modOK }
func (f fakeGoneEvidence) CorpusFiles([]string) []CorpusFile {
	return f.corpus
}
func (f fakeGoneEvidence) TypedCallers(symbol, pkg string) ([]GoneReferent, []GoneReferent) {
	return f.nonTest, f.test
}

func cleanGone() fakeGoneEvidence {
	return fakeGoneEvidence{atHead: true, modOK: true}
}

// TestEvaluateGone_EveryForm (CV-27): each of the four gone: forms proves
// absence when the fact holds, and fails honestly when it does not.
func TestEvaluateGone_EveryForm(t *testing.T) {
	t.Run("import gone", func(t *testing.T) {
		ev := cleanGone()
		ev.goFiles = []GoFile{
			{Path: "a.go", Content: []byte("package a\n\nimport \"fmt\"\n\nfunc F() { fmt.Println(1) }\n")},
		}
		task := Task{Gone: `import github.com/foo/bar`}
		c, label := EvaluateGone(task, ev)
		if c.Status != Pass || label != "" {
			t.Fatalf("import gone: status=%s label=%q detail=%q", c.Status, label, c.Detail)
		}
	})

	t.Run("import still present fails", func(t *testing.T) {
		ev := cleanGone()
		ev.goFiles = []GoFile{
			{Path: "a.go", Content: []byte("package a\n\nimport \"github.com/foo/bar\"\n\nfunc F() { bar.Do() }\n")},
		}
		task := Task{Gone: `import github.com/foo/bar`}
		c, label := EvaluateGone(task, ev)
		if c.Status != Fail || label != "gone-not-proven" {
			t.Fatalf("import still present: status=%s label=%q detail=%q", c.Status, label, c.Detail)
		}
		if !strings.Contains(c.Detail, "a.go") {
			t.Errorf("detail should cite a.go: %q", c.Detail)
		}
	})

	t.Run("module gone", func(t *testing.T) {
		ev := cleanGone()
		ev.modBody = []byte("module foo\n\ngo 1.26\n\nrequire (\n\tgithub.com/other/thing v1.0.0\n)\n")
		task := Task{Gone: `module github.com/foo/bar`}
		c, label := EvaluateGone(task, ev)
		if c.Status != Pass || label != "" {
			t.Fatalf("module gone: status=%s label=%q detail=%q", c.Status, label, c.Detail)
		}
	})

	t.Run("module still required fails, single-line form", func(t *testing.T) {
		ev := cleanGone()
		ev.modBody = []byte("module foo\n\ngo 1.26\n\nrequire github.com/foo/bar v1.2.3\n")
		task := Task{Gone: `module github.com/foo/bar`}
		c, label := EvaluateGone(task, ev)
		if c.Status != Fail || label != "gone-not-proven" {
			t.Fatalf("module still required: status=%s label=%q detail=%q", c.Status, label, c.Detail)
		}
	})

	t.Run("module still required fails, block form", func(t *testing.T) {
		ev := cleanGone()
		ev.modBody = []byte("module foo\n\ngo 1.26\n\nrequire (\n\tgithub.com/foo/bar v1.2.3 // indirect\n)\n")
		task := Task{Gone: `module github.com/foo/bar`}
		c, label := EvaluateGone(task, ev)
		if c.Status != Fail || label != "gone-not-proven" {
			t.Fatalf("module still required (block): status=%s label=%q detail=%q", c.Status, label, c.Detail)
		}
	})

	t.Run("pattern gone (default eq-zero)", func(t *testing.T) {
		ev := cleanGone()
		ev.corpus = []CorpusFile{
			{Path: "a.txt", Content: []byte("nothing interesting here")},
		}
		task := Task{Gone: `pattern "old_flag_name"`}
		c, label := EvaluateGone(task, ev)
		if c.Status != Pass || label != "" {
			t.Fatalf("pattern gone: status=%s label=%q detail=%q", c.Status, label, c.Detail)
		}
	})

	t.Run("pattern still present fails", func(t *testing.T) {
		ev := cleanGone()
		ev.corpus = []CorpusFile{
			{Path: "a.txt", Content: []byte("uses old_flag_name here")},
		}
		task := Task{Gone: `pattern "old_flag_name"`}
		c, label := EvaluateGone(task, ev)
		if c.Status != Fail || label != "gone-not-proven" {
			t.Fatalf("pattern still present: status=%s label=%q detail=%q", c.Status, label, c.Detail)
		}
	})

	t.Run("pattern with paths: scopes the scan", func(t *testing.T) {
		ev := cleanGone()
		ev.corpus = []CorpusFile{
			{Path: "keep/only.txt", Content: []byte("no match here")},
		}
		task := Task{Gone: `pattern "old_flag_name"`, GonePaths: []string{"keep/"}}
		c, label := EvaluateGone(task, ev)
		if c.Status != Pass || label != "" {
			t.Fatalf("pattern scoped by paths: status=%s label=%q detail=%q", c.Status, label, c.Detail)
		}
	})

	t.Run("pattern with max ratchet allows a bounded count", func(t *testing.T) {
		ev := cleanGone()
		ev.corpus = []CorpusFile{
			{Path: "a.txt", Content: []byte("old_flag_name old_flag_name")}, // 2 matches
		}
		two := 2
		task := Task{Gone: `pattern "old_flag_name"`, Max: &two}
		c, label := EvaluateGone(task, ev)
		if c.Status != Pass || label != "" {
			t.Fatalf("pattern under ratchet: status=%s label=%q detail=%q", c.Status, label, c.Detail)
		}
	})

	t.Run("callers gone: zero typed referents", func(t *testing.T) {
		ev := cleanGone()
		task := Task{Gone: `callers OldFunc`, Expect: Expect{Package: "internal/old"}}
		c, label := EvaluateGone(task, ev)
		if c.Status != Pass || label != "" {
			t.Fatalf("callers gone: status=%s label=%q detail=%q", c.Status, label, c.Detail)
		}
	})

	t.Run("callers with a non-test referent fails", func(t *testing.T) {
		ev := cleanGone()
		ev.nonTest = []GoneReferent{{File: "b.go", Line: 10}}
		task := Task{Gone: `callers OldFunc`}
		c, label := EvaluateGone(task, ev)
		if c.Status != Fail || label != "gone-not-proven" {
			t.Fatalf("callers non-test referent: status=%s label=%q detail=%q", c.Status, label, c.Detail)
		}
		if !strings.Contains(c.Detail, "b.go:10") {
			t.Errorf("detail should cite b.go:10: %q", c.Detail)
		}
	})

	t.Run("callers with a test referent fails unless tests: allowed", func(t *testing.T) {
		ev := cleanGone()
		ev.test = []GoneReferent{{File: "b_test.go", Line: 20}}
		task := Task{Gone: `callers OldFunc`}
		c, label := EvaluateGone(task, ev)
		if c.Status != Fail || label != "gone-not-proven" {
			t.Fatalf("callers test referent, tests: unset: status=%s label=%q", c.Status, label)
		}

		task.Tests = "allowed"
		c, label = EvaluateGone(task, ev)
		if c.Status != Pass || label != "" {
			t.Fatalf("callers test referent, tests: allowed: status=%s label=%q detail=%q", c.Status, label, c.Detail)
		}
	})
}

// TestEvaluateGone_UnknownOnStaleOrFailedIndex (CV-28): a gone: row is
// Unknown, never proven, when the index is not at HEAD, a typed package
// failed in scope, or a build-tag-excluded file references the target —
// even when the "clean" evidence would otherwise prove absence.
func TestEvaluateGone_UnknownOnStaleOrFailedIndex(t *testing.T) {
	t.Run("index not at head", func(t *testing.T) {
		ev := cleanGone()
		ev.atHead = false
		ev.atHeadWhy = "ingest is 3 commits behind HEAD"
		task := Task{Gone: `import github.com/foo/bar`}
		c, label := EvaluateGone(task, ev)
		if c.Status != Unknown || label != "gone-unknown" {
			t.Fatalf("status=%s label=%q, want unknown/gone-unknown", c.Status, label)
		}
		if !strings.Contains(c.Detail, "3 commits behind") {
			t.Errorf("detail should carry the reason: %q", c.Detail)
		}
	})

	t.Run("index not at head, no reason given still reports unknown", func(t *testing.T) {
		ev := cleanGone()
		ev.atHead = false
		task := Task{Gone: `import github.com/foo/bar`}
		c, label := EvaluateGone(task, ev)
		if c.Status != Unknown || label != "gone-unknown" {
			t.Fatalf("status=%s label=%q, want unknown/gone-unknown", c.Status, label)
		}
		if !strings.Contains(c.Detail, "not at HEAD") {
			t.Errorf("detail should carry a default reason: %q", c.Detail)
		}
	})

	t.Run("typed packages failed in scope", func(t *testing.T) {
		ev := cleanGone()
		ev.failed = []string{"internal/foo", "internal/bar"}
		task := Task{Gone: `module github.com/foo/bar`}
		c, label := EvaluateGone(task, ev)
		if c.Status != Unknown || label != "gone-unknown" {
			t.Fatalf("status=%s label=%q, want unknown/gone-unknown", c.Status, label)
		}
		if !strings.Contains(c.Detail, "internal/bar") { // sorted: bar before foo
			t.Errorf("detail should cite the first failed package alphabetically: %q", c.Detail)
		}
	})

	t.Run("build-tag-excluded file references the import target", func(t *testing.T) {
		ev := cleanGone()
		ev.goFiles = []GoFile{
			{Path: "a.go", Content: []byte("package a\n\nfunc F() {}\n")}, // clean, in-scope
			{Path: "darwin.go", Excluded: "//go:build darwin", Content: []byte(
				"package a\n\nimport \"github.com/foo/bar\"\n\nfunc G() { bar.Do() }\n")},
		}
		task := Task{Gone: `import github.com/foo/bar`}
		c, label := EvaluateGone(task, ev)
		if c.Status != Unknown || label != "gone-unknown" {
			t.Fatalf("status=%s label=%q detail=%q, want unknown/gone-unknown", c.Status, label, c.Detail)
		}
		if !strings.Contains(c.Detail, "darwin.go") {
			t.Errorf("detail should name the excluded file: %q", c.Detail)
		}
	})

	t.Run("build-tag-excluded file references the callers target", func(t *testing.T) {
		ev := cleanGone()
		ev.goFiles = []GoFile{
			{Path: "windows.go", Excluded: "//go:build windows", Content: []byte(
				"package a\n\nfunc G() { OldFunc() }\n")},
		}
		task := Task{Gone: `callers OldFunc`}
		c, label := EvaluateGone(task, ev)
		if c.Status != Unknown || label != "gone-unknown" {
			t.Fatalf("status=%s label=%q detail=%q, want unknown/gone-unknown", c.Status, label, c.Detail)
		}
		if !strings.Contains(c.Detail, "windows.go") {
			t.Errorf("detail should name the excluded file: %q", c.Detail)
		}
	})

	t.Run("module: go.mod not found at head is unknown, not a pass", func(t *testing.T) {
		ev := cleanGone()
		ev.modOK = false
		task := Task{Gone: `module github.com/foo/bar`}
		c, label := EvaluateGone(task, ev)
		if c.Status != Unknown || label != "gone-unknown" {
			t.Fatalf("status=%s label=%q, want unknown/gone-unknown", c.Status, label)
		}
	})

	t.Run("missing input still beats an otherwise-clean pattern scan", func(t *testing.T) {
		ev := cleanGone()
		ev.atHead = false
		ev.corpus = []CorpusFile{{Path: "a.txt", Content: []byte("nothing here")}}
		task := Task{Gone: `pattern "old_flag_name"`}
		c, label := EvaluateGone(task, ev)
		if c.Status != Unknown || label != "gone-unknown" {
			t.Fatalf("status=%s label=%q, want unknown/gone-unknown", c.Status, label)
		}
	})
}

// TestEvaluateGone_ContractRefusals pins the shape refusals shared between
// verifyTask's early bail and a future lint check.
func TestEvaluateGone_ContractRefusals(t *testing.T) {
	cases := []struct {
		name string
		task Task
		want string
	}{
		{"symbol and gone both set", Task{Gone: "import x", Expect: Expect{Symbol: "Foo"}}, "symbol and gone"},
		{"oracle and gone both set", Task{Gone: "import x", Oracle: "lexical"}, "oracle and gone"},
		{"unrecognized form", Task{Gone: "frobnicate x"}, "unrecognized gone"},
		{"import without a path", Task{Gone: "import"}, "unrecognized gone"},
		{"pattern without a regexp", Task{Gone: `pattern ""`}, "without a regexp"},
		{"pattern does not compile", Task{Gone: `pattern "("`}, "does not compile"},
		{"paths on a non-pattern form", Task{Gone: "import x", GonePaths: []string{"a/"}}, "only meaningful on gone: pattern"},
		{"tests value not recognized", Task{Gone: "callers Foo", Tests: "sometimes"}, "not recognized"},
		{"eq combined with max", Task{Gone: `pattern "x"`, Eq: intp(0), Max: intp(1)}, "cannot be combined"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := goneContractError(c.task)
			if got == "" {
				t.Fatalf("goneContractError returned no error, want one containing %q", c.want)
			}
			if !strings.Contains(got, c.want) {
				t.Errorf("goneContractError = %q, want it to contain %q", got, c.want)
			}
		})
	}
}

func intp(n int) *int { return &n }
