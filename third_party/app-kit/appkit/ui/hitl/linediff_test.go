package hitl

import (
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func kinds(d LineDiff) string {
	var b strings.Builder
	for _, line := range d.Lines {
		switch line.Kind {
		case DiffContext:
			b.WriteByte(' ')
		case DiffAdded:
			b.WriteByte('+')
		case DiffRemoved:
			b.WriteByte('-')
		}
		b.WriteString(line.Text)
		b.WriteByte('|')
	}
	return b.String()
}

func mustDiff(t *testing.T, a, b string) LineDiff {
	t.Helper()
	d, err := DiffText(a, b)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// rebuild reconstructs both texts from the script: the script is valid only if
// it reproduces them exactly, including the final newline.
func rebuild(d LineDiff) (string, string) {
	var oldText, newText strings.Builder
	for _, line := range d.Lines {
		suffix := "\n"
		if line.NoNewline {
			suffix = ""
		}
		if line.Kind != DiffAdded {
			oldText.WriteString(line.Text + suffix)
		}
		if line.Kind != DiffRemoved {
			newText.WriteString(line.Text + suffix)
		}
	}
	return oldText.String(), newText.String()
}

func lcs(a, b []string) int {
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else {
				dp[i][j] = max(dp[i+1][j], dp[i][j+1])
			}
		}
	}
	return dp[0][0]
}

func TestDiffTextSmallCases(t *testing.T) {
	cases := []struct{ old, new, want string }{
		{"", "", ""},
		{"a\nb\nc\n", "a\nb\nc\n", " a| b| c|"},
		{"a\nb\nc\n", "a\nc\nd\n", " a|-b| c|+d|"},
		{"", "x\ny\n", "+x|+y|"},
		{"x\ny\n", "", "-x|-y|"},
		// A changed block lists removals before additions.
		{"a\nb\nc\nz\n", "a\nB\nC\nz\n", " a|-b|-c|+B|+C| z|"},
		// "a" and "a\n" differ, as in git.
		{"a", "a\n", "-a|+a|"},
	}
	for _, c := range cases {
		d := mustDiff(t, c.old, c.new)
		if got := kinds(d); got != c.want {
			t.Errorf("DiffText(%q, %q) = %q, want %q", c.old, c.new, got, c.want)
		}
		if o, n := rebuild(d); o != c.old || n != c.new {
			t.Errorf("script does not rebuild %q/%q: got %q/%q", c.old, c.new, o, n)
		}
		if d.Replaced {
			t.Errorf("small diff %q/%q reported the budget fallback", c.old, c.new)
		}
	}
	d := mustDiff(t, "a", "a\n")
	if !d.Lines[0].NoNewline || d.Lines[1].NoNewline {
		t.Errorf("final-newline flags wrong: %#v", d.Lines)
	}
}

func TestDiffTextNumbersEverySide(t *testing.T) {
	d := mustDiff(t, "a\nb\nc\n", "a\nc\nd\n")
	want := []DiffLine{
		{Kind: DiffContext, OldNumber: 1, NewNumber: 1, Text: "a"},
		{Kind: DiffRemoved, OldNumber: 2, Text: "b"},
		{Kind: DiffContext, OldNumber: 3, NewNumber: 2, Text: "c"},
		{Kind: DiffAdded, NewNumber: 3, Text: "d"},
	}
	if !reflect.DeepEqual(d.Lines, want) {
		t.Fatalf("lines = %#v", d.Lines)
	}
}

// TestDiffTextIsMinimalAndValid checks the Myers script against an
// independent LCS on a deterministic random corpus: it must rebuild both texts
// and keep exactly LCS-many lines unchanged.
func TestDiffTextIsMinimalAndValid(t *testing.T) {
	rng := rand.New(rand.NewSource(20260925))
	alphabet := []string{"a", "b", "c", "d", "{", "}", ""}
	gen := func() []string {
		lines := make([]string, rng.Intn(40))
		for i := range lines {
			lines[i] = alphabet[rng.Intn(len(alphabet))]
		}
		return lines
	}
	for trial := 0; trial < 400; trial++ {
		a, b := gen(), gen()
		oldText, newText := strings.Join(a, "\n"), strings.Join(b, "\n")
		if len(a) > 0 {
			oldText += "\n"
		}
		if len(b) > 0 && trial%3 != 0 {
			newText += "\n"
		}
		d := mustDiff(t, oldText, newText)
		if o, n := rebuild(d); o != oldText || n != newText {
			t.Fatalf("trial %d: invalid script for %q -> %q", trial, oldText, newText)
		}
		if trial%3 != 0 || len(b) == 0 {
			kept := 0
			for _, line := range d.Lines {
				if line.Kind == DiffContext {
					kept++
				}
			}
			if want := lcs(a, b); kept != want {
				t.Fatalf("trial %d: kept %d lines, LCS is %d (%q -> %q)", trial, kept, want, oldText, newText)
			}
		}
		if again := mustDiff(t, oldText, newText); !reflect.DeepEqual(again, d) {
			t.Fatalf("trial %d: nondeterministic", trial)
		}
		// Every hunk set DiffText produces, and a subset of it, passes the
		// precomputed path's consistency checks.
		for _, context := range []int{NoContext, 0, 1, 5} {
			hunks := d.Hunks(context)
			var odd []Hunk
			for i, h := range hunks {
				if i%2 == 1 {
					odd = append(odd, h)
				}
			}
			for _, set := range [][]Hunk{hunks, odd} {
				if len(set) == 0 {
					continue
				}
				if _, err := RenderUnifiedDiff(UnifiedDiff{Label: "x", Hunks: set}); err != nil {
					t.Fatalf("trial %d context %d: DiffText hunks refused: %v", trial, context, err)
				}
			}
		}
	}
}

func TestDiffBudgetFallsBackToOneReplacementBlock(t *testing.T) {
	a := keys("keep\nx1\nx2\nx3\ntail\n")
	b := keys("keep\ny1\ny2\ntail\n")
	ops, replaced := editScript(a, b, 2)
	if !replaced {
		t.Fatal("budget of 2 edits accepted a 5-edit script")
	}
	want := []opKind{opEqual, opDelete, opDelete, opDelete, opInsert, opInsert, opEqual}
	if !reflect.DeepEqual(ops, want) {
		t.Fatalf("fallback ops = %v, want %v", ops, want)
	}
	if ops, replaced := editScript(a, b, 5); replaced || len(ops) != 7 {
		t.Fatalf("exact budget refused: %v %v", ops, replaced)
	}
}

func TestDiffTextRefusesOversizedInput(t *testing.T) {
	big := strings.Repeat("x\n", MaxDiffLines+1)
	if _, err := DiffText(big, ""); err == nil {
		t.Fatal("oversized old text accepted")
	}
	if _, err := DiffText("", big); err == nil {
		t.Fatal("oversized new text accepted")
	}
	if _, err := DiffText(strings.Repeat("x\n", MaxDiffLines), strings.Repeat("y\n", MaxDiffLines)); err != nil {
		t.Fatalf("bounded input refused: %v", err)
	}
}

func TestHunksMatchUnifiedConventions(t *testing.T) {
	var oldLines, newLines []string
	for i := 1; i <= 20; i++ {
		oldLines = append(oldLines, "line"+strconv.Itoa(i))
		newLines = append(newLines, "line"+strconv.Itoa(i))
	}
	newLines[4] = "changed5"                                                            // line 5
	newLines = append(newLines[:16], append([]string{"inserted"}, newLines[16:]...)...) // after 16
	d := mustDiff(t, strings.Join(oldLines, "\n")+"\n", strings.Join(newLines, "\n")+"\n")

	var headers []string
	for _, h := range d.Hunks(0) {
		headers = append(headers, h.Header())
	}
	// Matches `git diff -U3` for the same edit.
	if want := []string{"@@ -2,7 +2,7 @@", "@@ -14,6 +14,7 @@"}; !reflect.DeepEqual(headers, want) {
		t.Fatalf("default context headers = %v, want %v", headers, want)
	}
	headers = headers[:0]
	for _, h := range d.Hunks(NoContext) {
		headers = append(headers, h.Header())
	}
	if want := []string{"@@ -5 +5 @@", "@@ -16,0 +17 @@"}; !reflect.DeepEqual(headers, want) {
		t.Fatalf("no-context headers = %v, want %v", headers, want)
	}
	if hunks := d.Hunks(6); len(hunks) != 1 || hunks[0].Header() != "@@ -1,20 +1,21 @@" {
		t.Fatalf("wide context should merge into one hunk: %+v", hunks)
	}
	if got := mustDiff(t, "", "a\nb\n").Hunks(0); len(got) != 1 || got[0].Header() != "@@ -0,0 +1,2 @@" {
		t.Fatalf("new-file header: %+v", got)
	}
	if got := mustDiff(t, "a\n", "").Hunks(0); len(got) != 1 || got[0].Header() != "@@ -1 +0,0 @@" {
		t.Fatalf("deleted-file header: %+v", got)
	}
	if got := mustDiff(t, "same\n", "same\n").Hunks(0); got != nil {
		t.Fatalf("identical texts produced hunks: %+v", got)
	}
}
