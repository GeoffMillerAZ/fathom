package conserve

// residue_commentonly_test.go — a comment-only or blank-line edit INSIDE a
// function body is not a behaviour change. The header (doc) comment case
// already cleared, because a funcSite's span starts at the `func` keyword;
// a comment or blank line inside the braces did not, so the reading set
// listed such a function "modified" and a reviewer had to open it
// (docs/bench/readings/cv-m12-reading-set-2026-09-25.md: runLocalize,
// whose only change was a comment inside its body).

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestChangedFunctions_CommentOrBlankLineInBodyIsNotModified(t *testing.T) {
	base := map[string]string{
		"pkg/a.go": "package pkg\n\n" +
			"// Doc says one thing.\n" +
			"func HeaderDoc() int { return 1 }\n\n" +
			"func AddedComment() int {\n\tx := 1\n\treturn x\n}\n\n" +
			"func AddedBlank() int {\n\tx := 1\n\treturn x\n}\n\n" +
			"func EditedComment() int {\n\t// old wording\n\tx := 1\n\treturn x\n}\n\n" +
			"func TrailingComment() int {\n\tx := 1 // old\n\treturn x\n}\n\n" +
			"func BlockComment() int {\n\tx := /* one */ 1\n\treturn x\n}\n\n" +
			"func RealChange() int {\n\tx := 1\n\treturn x\n}\n\n" +
			"func SlashesInString() string {\n\treturn \"a // b\"\n}\n\n" +
			"func Reflowed() int {\n\treturn 1 +\n\t\t2\n}\n",
	}
	head := map[string]string{
		"pkg/a.go": "package pkg\n\n" +
			"// Doc says another thing.\n" +
			"func HeaderDoc() int { return 1 }\n\n" +
			"func AddedComment() int {\n\t// why x is one\n\tx := 1\n\treturn x\n}\n\n" +
			"func AddedBlank() int {\n\tx := 1\n\n\treturn x\n}\n\n" +
			"func EditedComment() int {\n\t// new wording\n\tx := 1\n\treturn x\n}\n\n" +
			"func TrailingComment() int {\n\tx := 1 // new\n\treturn x\n}\n\n" +
			"func BlockComment() int {\n\tx := /* two */ 1\n\treturn x\n}\n\n" +
			"func RealChange() int {\n\t// and a comment too\n\tx := 2\n\treturn x\n}\n\n" +
			"func SlashesInString() string {\n\treturn \"a // c\"\n}\n\n" +
			"func Reflowed() int {\n\treturn 1 + 2\n}\n",
	}
	in := Input{
		ChangedFiles: []string{"pkg/a.go"},
		BaseFile:     mapFile(base),
		HeadFile:     mapFile(head),
	}
	funcs, unanalyzed, missing := ChangedFunctions(in)
	if len(unanalyzed) != 0 || len(missing) != 0 {
		t.Fatalf("unanalyzed=%v missing=%v, want none", unanalyzed, missing)
	}
	got := map[string]string{}
	for _, f := range funcs {
		got[f.Symbol] = f.Change
	}
	want := map[string]string{"RealChange": "modified", "SlashesInString": "modified"}
	for name, change := range want {
		if got[name] != change {
			t.Errorf("%s = %q, want %q (funcs=%+v)", name, got[name], change, funcs)
		}
	}
	for _, name := range []string{"HeaderDoc", "AddedComment", "AddedBlank", "EditedComment", "TrailingComment", "BlockComment", "Reflowed"} {
		if c, ok := got[name]; ok {
			t.Errorf("%s reported %q — a comment, blank-line or line-break-only edit is not a behaviour change", name, c)
		}
	}
	if len(funcs) != len(want) {
		t.Errorf("ChangedFunctions = %+v, want exactly %v", funcs, want)
	}
}

// A comment added inside a block that never ran must not make the function
// a "partial" reading-set item: git diff names the comment's line as
// changed, and that line sits inside the unexecuted block's span.
func TestResidue_CommentOnlyBodyEditIsNeverPartial(t *testing.T) {
	ctx := context.Background()
	const svc = "svc"
	base := map[string]string{
		"pkg/f.go": "package pkg\n\nfunc F(a bool) {\n\tx := 1\n\t_ = x\n\tif a {\n\t\tprintln(1)\n\t}\n}\n",
	}
	head := map[string]string{
		"pkg/f.go": "package pkg\n\nfunc F(a bool) {\n\tx := 1\n\t_ = x\n\tif a {\n\t\t// only reached when a is set\n\t\tprintln(1)\n\t}\n}\n",
	}
	st := store.NewMemoryStore()
	if err := st.PutObservations(ctx, []core.Observation{
		{ServiceID: svc, Package: "pkg", Symbol: "F", SourceKind: core.KindGoFunc, SourceRef: "pkg/f.go", Line: 3},
	}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	cov := &CoverageProfile{Mode: "set", Blocks: map[string][]CoverageBlock{
		"pkg/f.go": {
			{StartLine: 3, StartCol: 16, EndLine: 6, EndCol: 7, NumStmt: 3, Count: 5},
			{StartLine: 6, StartCol: 7, EndLine: 9, EndCol: 3, NumStmt: 1, Count: 0},
		},
	}}
	in := Input{
		Service: svc, ChangedFiles: []string{"pkg/f.go"},
		BaseFile: mapFile(base), HeadFile: mapFile(head),
		Store: st, IndexAtHead: true,
		Coverage:     cov,
		CoverageMeta: CoverageMeta{Commit: "c1", Passed: true},
		ReachUnchangedSince: func(commit string, files []string) (bool, error) {
			return commit == "c1", nil
		},
		ChangedLines: func(path string) ([]LineRange, error) {
			return []LineRange{{Start: 7, End: 7}}, nil // the added comment line
		},
	}
	rep := Residue(ctx, in, 0)
	if rep.Changed != 0 || rep.Partial != 0 || len(rep.Items) != 0 {
		t.Errorf("comment-only body edit reached the reading set: changed=%d partial=%d items=%+v", rep.Changed, rep.Partial, rep.Items)
	}
}
