package verify

// lexical_test.go — PC-17 / PC-18 (ADR-030): the lexical oracle verifies a
// non-code row from the working tree, and refuses the shapes that would
// let existence stand in for proof.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// memFiles is a FileEvidence over an in-memory tree.
type memFiles map[string]string

func (m memFiles) ReadRepoFile(p string) ([]byte, error) {
	if s, ok := m[p]; ok {
		return []byte(s), nil
	}
	return nil, os.ErrNotExist
}

// lexEvidence pairs an empty index with a file tree, as the CLI does.
type lexEvidence struct {
	noEvidence
	files memFiles
}

func (e lexEvidence) ReadRepoFile(p string) ([]byte, error) { return e.files.ReadRepoFile(p) }

const lexicalMD = `# plan
- [x] Measurement recorded {id: L-1, oracle: lexical, path: docs/measurements/m.md, pattern: "p95 ≈ 329"}
- [x] Ignore line present {id: L-2, oracle: lexical, path: .gitignore, pattern: "^\.fathom/memories\.jsonl$"}
- [x] Exists only {id: L-3, oracle: lexical, path: notes.txt}
- [x] Pattern absent {id: L-4, oracle: lexical, path: docs/measurements/m.md, pattern: "never written"}
- [x] Path absent {id: L-5, oracle: lexical, path: docs/nope.md, pattern: x}
- [x] Markdown without pattern {id: L-6, oracle: lexical, path: docs/measurements/m.md}
- [x] Two contracts {id: L-7, symbol: Foo, oracle: lexical, path: notes.txt, pattern: x}
- [x] Empty file {id: L-8, oracle: lexical, path: empty.txt}
- [x] Escapes {id: L-9, oracle: lexical, path: ../secrets, pattern: x}
- [x] Bad regexp {id: L-10, oracle: lexical, path: notes.txt, pattern: "("}
- [x] Unknown class {id: L-11, oracle: vibes, path: notes.txt, pattern: x}
`

func lexicalFixture() memFiles {
	return memFiles{
		"docs/measurements/m.md": "| post-edit | 1,918 | 131 | 315 | 329 |\np95 ≈ 329 (2.2×)\n",
		".gitignore":             "bin/\n.fathom/memories.jsonl\nworkspace/\n",
		"notes.txt":              "hello\n",
		"empty.txt":              "  \n",
	}
}

func TestParseMarkdownPlan_LexicalOracleAndLine(t *testing.T) {
	p := ParseMarkdownPlan("svc", []byte(lexicalMD))
	l1 := p.Tasks[0]
	if l1.Oracle != "lexical" || l1.OraclePath != "docs/measurements/m.md" || l1.OraclePattern != "p95 ≈ 329" {
		t.Fatalf("oracle keys parsed wrong: %+v", l1)
	}
	if l1.Line != 2 {
		t.Errorf("Line = %d, want 2 (1-based row line)", l1.Line)
	}
	if p.Tasks[10].Line != 12 {
		t.Errorf("last row Line = %d, want 12", p.Tasks[10].Line)
	}
	// A stray pattern without oracle stays inert (no half-contract).
	q := ParseMarkdownPlan("svc", []byte("- [x] X {id: A, symbol: Foo, pattern: bar}\n"))
	if q.Tasks[0].Oracle != "" || q.Tasks[0].OraclePattern != "" {
		t.Errorf("pattern without oracle must be ignored: %+v", q.Tasks[0])
	}
}

func TestEvalLexicalOracle_VerifiesAndRefuses(t *testing.T) {
	p := ParseMarkdownPlan("svc", []byte(lexicalMD))
	rep := VerifyPlan(p, lexEvidence{files: lexicalFixture()})
	want := map[string]struct {
		status CriterionStatus
		label  string
		detail string
	}{
		"L-1":  {Pass, "verified", "matches at docs/measurements/m.md:2"},
		"L-2":  {Pass, "verified", ".gitignore:2"},
		"L-3":  {Pass, "verified", "non-empty"},
		"L-4":  {Fail, "pattern-unmatched", "matches nothing"},
		"L-5":  {Fail, "path-absent", "not readable"},
		"L-6":  {Fail, "contract-invalid", "without a pattern"},
		"L-7":  {Fail, "contract-invalid", "one contract per row"},
		"L-8":  {Fail, "path-empty", "is empty"},
		"L-9":  {Fail, "contract-invalid", "repo-relative"},
		"L-10": {Fail, "contract-invalid", "does not compile"},
		"L-11": {Fail, "contract-invalid", "unknown oracle"},
	}
	for _, tv := range rep.Tasks {
		w, ok := want[tv.TaskID]
		if !ok {
			t.Fatalf("unexpected task %s", tv.TaskID)
		}
		if tv.Status != w.status || tv.Label != w.label {
			t.Errorf("%s: status/label = %s/%s, want %s/%s (%+v)", tv.TaskID, tv.Status, tv.Label, w.status, w.label, tv.Criteria)
		}
		var lex *Criterion
		for i := range tv.Criteria {
			if tv.Criteria[i].Name == lexicalCriterion {
				lex = &tv.Criteria[i]
			}
		}
		if lex == nil {
			t.Errorf("%s: no %s criterion", tv.TaskID, lexicalCriterion)
			continue
		}
		if !strings.Contains(lex.Detail, w.detail) {
			t.Errorf("%s: detail %q lacks %q", tv.TaskID, lex.Detail, w.detail)
		}
	}
	// The verified rows are exactly the ones that leave U2: Pass, never Unknown.
	if rep.Summary.Pass != 3 || rep.Summary.Fail != 8 || rep.Summary.Unknown != 0 {
		t.Errorf("summary = %+v, want 3 pass / 8 fail / 0 unknown", rep.Summary)
	}
}

func TestEvalLexicalOracle_NoFileEvidenceIsUnknownNeverPass(t *testing.T) {
	p := ParseMarkdownPlan("svc", []byte("- [x] Row {id: L-1, oracle: lexical, path: notes.txt, pattern: hello}\n"))
	rep := VerifyPlan(p, noEvidence{})
	tv := rep.Tasks[0]
	if tv.Status != Unknown || tv.Label != "oracle-unevaluated" {
		t.Fatalf("without FileEvidence the row must be Unknown/oracle-unevaluated, got %s/%s", tv.Status, tv.Label)
	}
	if g := Gate(rep, true); g.Pass {
		t.Error("--strict must refuse an unevaluated oracle row")
	}
	if g := Gate(rep, false); !g.Pass {
		t.Error("non-strict must fail open on an unevaluated oracle row")
	}
}

func TestLintLexicalOracle_RefusesTheSameShapesVerifyFails(t *testing.T) {
	p := ParseMarkdownPlan("svc", []byte(lexicalMD))
	findings := LintPlan(p)
	joined := strings.Join(findings, "\n")
	for _, id := range []string{"L-6", "L-7", "L-9", "L-10", "L-11"} {
		if !strings.Contains(joined, id+": lexical oracle refused") {
			t.Errorf("lint must refuse %s; got:\n%s", id, joined)
		}
	}
	for _, id := range []string{"L-1", "L-2", "L-3", "L-4", "L-5", "L-8"} {
		if strings.Contains(joined, id+": lexical oracle refused") {
			t.Errorf("lint must not refuse a sound contract %s (its truth is verify's job); got:\n%s", id, joined)
		}
	}
	if !strings.Contains(joined, "markdown path") || !strings.Contains(joined, "one contract per row") {
		t.Errorf("the two ADR-030 refusals must be named in words:\n%s", joined)
	}
}

func TestDirFiles_ReadsWorkingTreeAndRefusesEscape(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "a.md"), []byte("## Evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d := DirFiles(root)
	if b, err := d.ReadRepoFile("docs/a.md"); err != nil || !strings.Contains(string(b), "Evidence") {
		t.Errorf("read = %q, %v", b, err)
	}
	if _, err := d.ReadRepoFile("../outside"); err != ErrOutsideRepo {
		t.Errorf("escape must be refused, got %v", err)
	}
	if _, err := d.ReadRepoFile("docs/../../outside"); err != ErrOutsideRepo {
		t.Errorf("dotted escape must be refused, got %v", err)
	}
}

func TestOverlayLocal_ReplacesOnlyLexicalRowsAndStampsLines(t *testing.T) {
	md := "- [x] Code {id: C-1, symbol: Foo}\n- [x] Doc {id: D-1, oracle: lexical, path: notes.txt, pattern: hello}\n"
	p := ParseMarkdownPlan("svc", []byte(md))
	// What a daemon without the working tree returns: the code row judged,
	// the lexical row unknown, no line numbers on the wire.
	rep := Report{Service: "svc", Tasks: []TaskVerdict{
		{TaskID: "C-1", Title: "Code", Status: Fail, Label: "claimed-but-absent", Claimed: true},
		{TaskID: "D-1", Title: "Doc", Status: Unknown, Label: "oracle-unevaluated", Claimed: true},
	}, Summary: Summary{Total: 2, Fail: 1, Unknown: 1}}
	OverlayLocal(&rep, p, lexicalFixture())
	if rep.Tasks[0].Status != Fail || rep.Tasks[0].Line != 1 {
		t.Errorf("code row must keep the daemon's verdict and gain its line: %+v", rep.Tasks[0])
	}
	if rep.Tasks[1].Status != Pass || rep.Tasks[1].Label != "verified" || rep.Tasks[1].Line != 2 || !rep.Tasks[1].Claimed {
		t.Errorf("lexical row must be re-evaluated locally: %+v", rep.Tasks[1])
	}
	if rep.Summary != (Summary{Total: 2, Pass: 1, Fail: 1}) {
		t.Errorf("summary not recounted: %+v", rep.Summary)
	}
}
