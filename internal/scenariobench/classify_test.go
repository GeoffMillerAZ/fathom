package scenariobench

import (
	"strings"
	"testing"
	"testing/fstest"
)

func patchOf(diffs ...string) []byte {
	return []byte(strings.Join(diffs, "\n"))
}

func fileDiff(pathBoth, hunk string) string {
	return "diff --git a/" + pathBoth + " b/" + pathBoth + "\n" +
		"index 1111111..2222222 100644\n" +
		"--- a/" + pathBoth + "\n" +
		"+++ b/" + pathBoth + "\n" + hunk
}

func TestClassifyChange_GoModTouchedIsDependencyUpgrade(t *testing.T) {
	patch := patchOf(fileDiff("go.mod", "@@ -1,3 +1,3 @@\n module x\n\n-require y v1.0.0\n+require y v1.1.0\n"))
	cat, ev := ClassifyChange(patch, nil, nil)
	if cat != CategoryUpgrade {
		t.Fatalf("category = %q, want %q", cat, CategoryUpgrade)
	}
	if ev.Rule == "" {
		t.Fatalf("evidence has no rule")
	}
}

func TestClassifyChange_GoModWinsOverSignatureChange(t *testing.T) {
	// A go.mod bump alongside a real signature change: rule 1 fires first.
	before := fstest.MapFS{"pkg/a.go": {Data: []byte("package a\n\nfunc F(x int) {}\n")}}
	after := fstest.MapFS{"pkg/a.go": {Data: []byte("package a\n\nfunc F(x int, y int) {}\n")}}
	patch := patchOf(
		fileDiff("go.mod", "@@ -1,2 +1,2 @@\n module x\n-require y v1.0.0\n+require y v1.1.0\n"),
		fileDiff("pkg/a.go", "@@ -1,3 +1,3 @@\n package a\n\n-func F(x int) {}\n+func F(x int, y int) {}\n"),
	)
	cat, _ := ClassifyChange(patch, before, after)
	if cat != CategoryUpgrade {
		t.Fatalf("category = %q, want %q (rule 1 must win)", cat, CategoryUpgrade)
	}
}

func TestClassifyChange_SignatureChanged(t *testing.T) {
	before := fstest.MapFS{"pkg/a.go": {Data: []byte("package a\n\nfunc F(x int) {}\n")}}
	after := fstest.MapFS{"pkg/a.go": {Data: []byte("package a\n\nfunc F(x int, y int) {}\n")}}
	patch := patchOf(fileDiff("pkg/a.go", "@@ -1,3 +1,3 @@\n package a\n\n-func F(x int) {}\n+func F(x int, y int) {}\n"))
	cat, ev := ClassifyChange(patch, before, after)
	if cat != CategorySignature {
		t.Fatalf("category = %q, want %q", cat, CategorySignature)
	}
	if len(ev.Functions) != 1 || ev.Functions[0] != "F" {
		t.Fatalf("evidence.Functions = %v, want [F]", ev.Functions)
	}
}

func TestClassifyChange_UnexportedSignatureChangeDoesNotFire(t *testing.T) {
	before := fstest.MapFS{"pkg/a.go": {Data: []byte("package a\n\nfunc f(x int) {}\n")}}
	after := fstest.MapFS{"pkg/a.go": {Data: []byte("package a\n\nfunc f(x int, y int) {}\n")}}
	patch := patchOf(fileDiff("pkg/a.go", "@@ -1,3 +1,3 @@\n package a\n\n-func f(x int) {}\n+func f(x int, y int) {}\n"))
	cat, _ := ClassifyChange(patch, before, after)
	if cat == CategorySignature {
		t.Fatalf("category = %q, unexported func must not trigger signature_change", cat)
	}
}

func TestClassifyChange_NoTreesSkipsSignatureRuleGracefully(t *testing.T) {
	// Same content that WOULD be a signature change if trees were given —
	// with nil trees the rule must be skipped, never guessed or panicked.
	patch := patchOf(fileDiff("pkg/a.go", "@@ -1,3 +1,3 @@\n package a\n\n-func F(x int) {}\n+func F(x int, y int) {}\n"))
	cat, ev := ClassifyChange(patch, nil, nil)
	if cat == CategorySignature {
		t.Fatalf("category = %q, want anything but signature_change without trees", cat)
	}
	found := false
	for _, s := range ev.Skipped {
		if strings.Contains(s, "no before/after tree") {
			found = true
		}
	}
	if !found {
		t.Fatalf("evidence.Skipped = %v, want a note that the signature check was skipped", ev.Skipped)
	}
}

func TestClassifyChange_CrossPackageFeature(t *testing.T) {
	patch := patchOf(
		fileDiff("pkg/a/a.go", "@@ -1,2 +1,3 @@\n package a\n+var X = 1\n"),
		fileDiff("pkg/b/b.go", "@@ -1,2 +1,3 @@\n package b\n+var Y = 1\n"),
	)
	cat, ev := ClassifyChange(patch, nil, nil)
	if cat != CategoryFeature {
		t.Fatalf("category = %q, want %q", cat, CategoryFeature)
	}
	if len(ev.Packages) != 2 {
		t.Fatalf("evidence.Packages = %v, want 2 entries", ev.Packages)
	}
}

func TestClassifyChange_CrossPackageWinsOverGovernance(t *testing.T) {
	patch := patchOf(
		fileDiff("pkg/a/a.go", "@@ -1,2 +1,3 @@\n package a\n+var X = 1\n"),
		fileDiff("pkg/b/b.go", "@@ -1,2 +1,3 @@\n package b\n+var Y = 1\n"),
		fileDiff("CLAUDE.md", "@@ -1,2 +1,3 @@\n # notes\n+more notes\n"),
	)
	cat, _ := ClassifyChange(patch, nil, nil)
	if cat != CategoryFeature {
		t.Fatalf("category = %q, want %q (rule 3 precedes rule 4)", cat, CategoryFeature)
	}
}

func TestClassifyChange_GovernanceBound(t *testing.T) {
	patch := patchOf(
		fileDiff("internal/pkg/a.go", "@@ -1,2 +1,3 @@\n package pkg\n+var X = 1\n"),
		fileDiff("CLAUDE.md", "@@ -1,2 +1,3 @@\n # notes\n+more notes\n"),
	)
	cat, ev := ClassifyChange(patch, nil, nil)
	if cat != CategoryGovernance {
		t.Fatalf("category = %q, want %q", cat, CategoryGovernance)
	}
	if len(ev.Files) != 2 {
		t.Fatalf("evidence.Files = %v, want 2 entries", ev.Files)
	}
}

func TestClassifyChange_DocsOnlyIsGovernanceBound(t *testing.T) {
	patch := patchOf(
		fileDiff("internal/pkg/a.go", "@@ -1,2 +1,3 @@\n package pkg\n+var X = 1\n"),
		fileDiff("docs/usage.md", "@@ -1,2 +1,3 @@\n # usage\n+more\n"),
	)
	cat, _ := ClassifyChange(patch, nil, nil)
	if cat != CategoryGovernance {
		t.Fatalf("category = %q, want %q", cat, CategoryGovernance)
	}
}

func TestClassifyChange_GovernanceAloneIsNotGovernanceBound(t *testing.T) {
	// "touched with code": a rules-only patch has no code alongside it.
	patch := patchOf(fileDiff("CLAUDE.md", "@@ -1,2 +1,3 @@\n # notes\n+more notes\n"))
	cat, _ := ClassifyChange(patch, nil, nil)
	if cat != CategoryBugFix {
		t.Fatalf("category = %q, want %q (no code touched alongside the rules file)", cat, CategoryBugFix)
	}
}

func TestClassifyChange_SingleFileFallsBackToBugFix(t *testing.T) {
	patch := patchOf(fileDiff("internal/pkg/a.go", "@@ -1,2 +1,3 @@\n package pkg\n+var X = 1\n"))
	cat, ev := ClassifyChange(patch, nil, nil)
	if cat != CategoryBugFix {
		t.Fatalf("category = %q, want %q", cat, CategoryBugFix)
	}
	if ev.Rule == "" {
		t.Fatalf("evidence has no rule even on the fallback")
	}
}

func TestClassifyChange_Deterministic(t *testing.T) {
	patch := patchOf(
		fileDiff("pkg/a/a.go", "@@ -1,2 +1,3 @@\n package a\n+var X = 1\n"),
		fileDiff("pkg/b/b.go", "@@ -1,2 +1,3 @@\n package b\n+var Y = 1\n"),
	)
	cat1, ev1 := ClassifyChange(patch, nil, nil)
	cat2, ev2 := ClassifyChange(patch, nil, nil)
	if cat1 != cat2 {
		t.Fatalf("category not deterministic: %q vs %q", cat1, cat2)
	}
	if strings.Join(ev1.Packages, ",") != strings.Join(ev2.Packages, ",") {
		t.Fatalf("evidence.Packages not deterministic: %v vs %v", ev1.Packages, ev2.Packages)
	}
}

func TestClassifyChange_UnparseablePatchFallsBackToBugFix(t *testing.T) {
	cat, ev := ClassifyChange([]byte("@@ -bad hunk header @@\n"), nil, nil)
	if cat != CategoryBugFix {
		t.Fatalf("category = %q, want %q", cat, CategoryBugFix)
	}
	if ev.Rule == "" {
		t.Fatalf("evidence has no rule for the unparseable case")
	}
}
