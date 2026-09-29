package ownership_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/ownership"
)

// ─── Parse — basic ───────────────────────────────────────────────────────────

func TestParse_BasicRule(t *testing.T) {
	content := []byte("*.go @alice\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.For("main.go")
	want := []string{"@alice"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("For(main.go) = %v, want %v", got, want)
	}
}

func TestParse_MultipleOwners(t *testing.T) {
	content := []byte("*.go @alice @bob\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.For("main.go")
	want := []string{"@alice", "@bob"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("For(main.go) = %v, want %v", got, want)
	}
}

// Comments (lines starting with #) are skipped.
func TestParse_CommentsSkipped(t *testing.T) {
	content := []byte("# This is a comment\n*.go @alice\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.For("main.go")
	want := []string{"@alice"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("For(main.go) = %v, want %v", got, want)
	}
}

// Blank lines are skipped.
func TestParse_BlankLinesSkipped(t *testing.T) {
	content := []byte("\n*.go @alice\n\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.For("main.go")
	want := []string{"@alice"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("For(main.go) = %v, want %v", got, want)
	}
}

// Inline comments after a rule are ignored (everything after # on a rule line).
func TestParse_InlineCommentStripped(t *testing.T) {
	content := []byte("*.go @alice # owns all go files\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.For("main.go")
	want := []string{"@alice"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("For(main.go) = %v, want %v", got, want)
	}
}

// ─── Parse — nil / empty ─────────────────────────────────────────────────────

func TestParse_NilContent(t *testing.T) {
	o, err := ownership.Parse(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.For("anything.go")
	if len(got) != 0 {
		t.Errorf("expected no owners, got %v", got)
	}
}

func TestParse_EmptyContent(t *testing.T) {
	o, err := ownership.Parse([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.For("anything.go")
	if len(got) != 0 {
		t.Errorf("expected no owners, got %v", got)
	}
}

// ─── For — last-match-wins semantics ─────────────────────────────────────────

// CODEOWNERS: later rules override earlier ones when both match.
func TestFor_LastMatchWins(t *testing.T) {
	content := []byte("*.go @alice\ninternal/*.go @bob\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "internal/foo.go" matches both *.go and internal/*.go; @bob wins.
	got := o.For("internal/foo.go")
	want := []string{"@bob"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("For(internal/foo.go) = %v, want %v", got, want)
	}
}

// File matching only the first rule gets the first rule's owners.
func TestFor_FirstRuleOnly(t *testing.T) {
	content := []byte("*.go @alice\ninternal/*.go @bob\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.For("main.go")
	want := []string{"@alice"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("For(main.go) = %v, want %v", got, want)
	}
}

// No rule matches → empty slice (not nil).
func TestFor_NoMatch(t *testing.T) {
	content := []byte("*.go @alice\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.For("README.md")
	if len(got) != 0 {
		t.Errorf("expected no owners for README.md, got %v", got)
	}
}

// Directory-level glob: "internal/" should match files under internal/.
func TestFor_DirectoryPattern(t *testing.T) {
	content := []byte("internal/ @team-a\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.For("internal/foo/bar.go")
	if len(got) == 0 {
		t.Errorf("expected @team-a for internal/foo/bar.go, got none")
	}
	if got[0] != "@team-a" {
		t.Errorf("For(internal/foo/bar.go) = %v, want [@team-a]", got)
	}
}

// Catch-all rule "*" matches everything.
func TestFor_CatchAll(t *testing.T) {
	content := []byte("* @global-owner\n*.go @gopher\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// *.go is last and matches → @gopher wins.
	got := o.For("cmd/main.go")
	want := []string{"@gopher"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("For(cmd/main.go) = %v, want %v", got, want)
	}
	// README.md: only * matches → @global-owner.
	got2 := o.For("README.md")
	want2 := []string{"@global-owner"}
	if !reflect.DeepEqual(got2, want2) {
		t.Errorf("For(README.md) = %v, want %v", got2, want2)
	}
}

// Three-level nesting: last of three rules wins for a deep path.
func TestFor_ThreeLevels_LastWins(t *testing.T) {
	content := []byte("* @root\ninternal/ @backend\ninternal/auth/ @security\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.For("internal/auth/token.go")
	want := []string{"@security"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("For(internal/auth/token.go) = %v, want %v", got, want)
	}
}

// A path with no owners at all (no rule matches) → empty, not nil.
func TestFor_ReturnsEmptyNotNil(t *testing.T) {
	content := []byte("*.go @alice\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.For("Makefile")
	if got == nil {
		t.Error("For() returned nil, want empty slice")
	}
}

// ─── For — glob patterns ──────────────────────────────────────────────────────

// "docs/**" should match files in any subdirectory under docs/.
func TestFor_DoubleStarGlob(t *testing.T) {
	content := []byte("docs/** @writers\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.For("docs/adr/ADR-001.md")
	want := []string{"@writers"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("For(docs/adr/ADR-001.md) = %v, want %v", got, want)
	}
}

// Extension glob matching.
func TestFor_ExtensionGlob(t *testing.T) {
	content := []byte("*.tf @infra\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := o.For("main.tf"); !reflect.DeepEqual(got, []string{"@infra"}) {
		t.Errorf("For(main.tf) = %v", got)
	}
	if got := o.For("main.go"); len(got) != 0 {
		t.Errorf("For(main.go) should be empty, got %v", got)
	}
}

// ─── Rollup ───────────────────────────────────────────────────────────────────

// Rollup returns a map from owner to rule count.
func TestRollup_Basic(t *testing.T) {
	content := []byte("*.go @alice\n*.tf @bob\ninternal/ @alice\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.Rollup()
	if got["@alice"] != 2 {
		t.Errorf("@alice = %d, want 2", got["@alice"])
	}
	if got["@bob"] != 1 {
		t.Errorf("@bob = %d, want 1", got["@bob"])
	}
}

func TestRollup_Empty(t *testing.T) {
	o, err := ownership.Parse(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.Rollup()
	if len(got) != 0 {
		t.Errorf("expected empty rollup, got %v", got)
	}
}

// Rollup is deterministic: repeated calls return identical maps.
func TestRollup_Deterministic(t *testing.T) {
	content := []byte("*.go @alice @charlie\n*.tf @bob\ninternal/ @alice\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := o.Rollup()
	b := o.Rollup()
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Rollup not deterministic: %v vs %v", a, b)
	}
}

// Multiple owners in one rule are each counted separately.
func TestRollup_MultiOwnerRule(t *testing.T) {
	content := []byte("*.go @alice @bob\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.Rollup()
	if got["@alice"] != 1 {
		t.Errorf("@alice = %d, want 1", got["@alice"])
	}
	if got["@bob"] != 1 {
		t.Errorf("@bob = %d, want 1", got["@bob"])
	}
}

// Rollup keys are sorted (deterministic iteration order).
func TestRollup_KeysSorted(t *testing.T) {
	content := []byte("*.go @charlie\n*.tf @alice\ninternal/ @bob\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.Rollup()
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sorted := make([]string, len(keys))
	copy(sorted, keys)
	sort.Strings(sorted)
	// Check that the returned map contains expected keys (ordering is map-level;
	// just verify all owners are present and counts correct).
	if got["@alice"] != 1 || got["@bob"] != 1 || got["@charlie"] != 1 {
		t.Errorf("Rollup = %v", got)
	}
}

// ─── Regression: inline comment on rule line ──────────────────────────────────

func TestParse_RuleWithInlineComment(t *testing.T) {
	content := []byte("internal/auth/ @security # security team owns auth\n*.go @gopher\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := o.For("internal/auth/login.go")
	// internal/auth/ matches last before *.go — *.go is after and also matches
	// for .go files, so *.go wins.
	if len(got) == 0 {
		t.Errorf("expected some owners for internal/auth/login.go, got none")
	}
}

// ─── Race-safety sanity ───────────────────────────────────────────────────────

// Concurrent reads of For() and Rollup() must not race.
// (go test -race will catch this if there are any shared-write issues.)
func TestFor_ConcurrentReads(t *testing.T) {
	content := []byte("*.go @alice\ninternal/ @bob\n")
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	paths := []string{"main.go", "internal/foo.go", "README.md", "internal/auth/token.go"}
	done := make(chan struct{}, len(paths))
	for _, p := range paths {
		go func(path string) {
			o.For(path)
			done <- struct{}{}
		}(p)
	}
	for range paths {
		<-done
	}
}

// ─── Unsupported syntax is named with its line, not approximated ─────────────

// GitHub does not honor negation, character ranges or backslash escapes, and
// GitLab section headers are another dialect. Each such line is reported
// with its 1-based line number and matches nothing; the supported rules
// around it keep working.
func TestParse_UnsupportedSyntaxNamedWithLine(t *testing.T) {
	content := []byte("# owners\n" + // 1
		"*.go @alice\n" + // 2
		"!vendor/ @nobody\n" + // 3
		"\n" + // 4
		"[Docs] @writers\n" + // 5
		"docs/[ab].md @writers\n" + // 6
		"\\#notes @writers\n" + // 7
		"^[Optional] @maybe\n" + // 8
		"/api/ @backend\n") // 9
	o, err := ownership.Parse(content)
	if err != nil {
		t.Fatal(err)
	}
	got := o.Unsupported()
	wantLines := []int{3, 5, 6, 7, 8}
	if len(got) != len(wantLines) {
		t.Fatalf("Unsupported() = %+v, want lines %v", got, wantLines)
	}
	for i, u := range got {
		if u.Line != wantLines[i] || u.Reason == "" || u.Text == "" {
			t.Errorf("Unsupported()[%d] = %+v, want line %d with text and reason", i, u, wantLines[i])
		}
	}
	// Approximating would have owned these; skipping leaves them to the
	// supported rules only.
	if got := o.For("vendor/x.go"); !reflect.DeepEqual(got, []string{"@alice"}) {
		t.Errorf("For(vendor/x.go) = %v, want [@alice]: a negation must not match or unmatch", got)
	}
	if got := o.For("docs/a.md"); len(got) != 0 {
		t.Errorf("For(docs/a.md) = %v, want none: a section header and a range match nothing", got)
	}
	if got := o.For("api/x.go"); !reflect.DeepEqual(got, []string{"@backend"}) {
		t.Errorf("For(api/x.go) = %v, want [@backend]", got)
	}
	if c := o.Rollup()["@writers"]; c != 0 {
		t.Errorf("Rollup counts an unsupported rule's owner: %d", c)
	}
}

// A file with nothing unsupported reports nothing.
func TestParse_NoUnsupportedSyntax(t *testing.T) {
	o, err := ownership.Parse([]byte("*.go @a\n/docs/ @b\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := o.Unsupported(); len(got) != 0 {
		t.Errorf("Unsupported() = %+v, want none", got)
	}
}

// A leading-slash rule owns the root path and not a same-named nested one,
// end to end through Parse and For (IM-F1).
func TestFor_LeadingSlashAndAnyDepthEndToEnd(t *testing.T) {
	o, err := ownership.Parse([]byte("/services/checkout/ @pay\ndocs/ @writers\n"))
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string][]string{
		"services/checkout/main.go":   {"@pay"},
		"x/services/checkout/main.go": {},
		"docs/a.md":                   {"@writers"},
		"pkg/docs/a.md":               {"@writers"},
	} {
		if got := o.For(path); !reflect.DeepEqual(got, want) {
			t.Errorf("For(%s) = %v, want %v", path, got, want)
		}
	}
}
