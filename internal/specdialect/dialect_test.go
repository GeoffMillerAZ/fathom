package specdialect

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// fakeDialect is a minimal dialect for registry tests.
type fakeDialect struct{ id string }

func (f fakeDialect) ID() string { return f.id }
func (f fakeDialect) Detect(_ string, files []string) []string {
	var out []string
	for _, p := range files {
		if strings.HasSuffix(p, ".fake") {
			out = append(out, p)
		}
	}
	return out
}
func (f fakeDialect) Claims(path string, _ []byte) bool { return strings.HasSuffix(path, ".fake") }
func (f fakeDialect) Parse(string, []byte) (verify.Plan, []Span, error) {
	return verify.Plan{}, nil, nil
}

// TestSpecDialectRegistryDetect (SI-1): the port, registration refusing an
// empty or duplicate id, and deterministic detection by path and content
// shape — foreign dialects before the native fallback, sorted output.
func TestSpecDialectRegistryDetect(t *testing.T) {
	r := NewRegistry()
	if err := RegisterSpecDialect(r, fakeDialect{id: ""}); err == nil {
		t.Fatal("an empty id must be refused")
	}
	if err := RegisterSpecDialect(r, fakeDialect{id: "fake"}); err != nil {
		t.Fatalf("register fake: %v", err)
	}
	if err := RegisterSpecDialect(r, fakeDialect{id: "fake"}); err == nil {
		t.Fatal("a duplicate id must be refused")
	}
	for _, d := range []SpecDialect{Native(), SpecKit(), OpenSpec()} {
		if err := RegisterSpecDialect(r, d); err != nil {
			t.Fatalf("register %s: %v", d.ID(), err)
		}
	}
	if got, want := r.IDs(), []string{"fake", "fathom-v2", "openspec", "speckit"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
	if got, want := Default().IDs(), []string{"fathom-v2", "gh-tasklist", "kiro", "openspec", "speckit", "taskmaster"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Default IDs = %v, want %v", got, want)
	}

	root := filepath.Join("testdata")
	files := []string{
		"speckit/specs/001-payments/tasks.md",
		"openspec/openspec/changes/add-retry/tasks.md",
		"openspec/openspec/changes/archive/2026-01-01-old/tasks.md",
		"openspec/openspec/changes/add-retry/specs/payments/spec.md",
		"docs/plans/spec-interop.md",
		"docs/plans/README.md",
		"notes/x.fake",
		"speckit/docs/plans/native.md",
	}
	got := r.Detect(root, files)
	want := []Detection{
		{Path: "docs/plans/spec-interop.md", Dialect: NativeID},
		{Path: "notes/x.fake", Dialect: "fake"},
		{Path: "openspec/openspec/changes/add-retry/tasks.md", Dialect: "openspec"},
		{Path: "speckit/specs/001-payments/tasks.md", Dialect: "speckit"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Detect =\n%v\nwant\n%v", got, want)
	}
	// Deterministic: shuffled input, same answer.
	rev := append([]string(nil), files...)
	sort.Sort(sort.Reverse(sort.StringSlice(rev)))
	if again := r.Detect(root, rev); !reflect.DeepEqual(again, got) {
		t.Fatalf("Detect is order-dependent: %v vs %v", again, got)
	}

	// Content shape, not only path: a specs/<f>/tasks.md with no T### row
	// is not Spec Kit's, and falls back to the native grammar.
	d, ok := r.ForFile("specs/002-x/tasks.md", []byte("- [ ] just a list\n"))
	if !ok || d.ID() != NativeID {
		t.Fatalf("ForFile(shape-less tasks.md) = %v, want the native fallback", d)
	}
	d, ok = r.ForFile("/abs/repo/specs/002-x/tasks.md", []byte("- [ ] T001 real task\n"))
	if !ok || d.ID() != "speckit" {
		t.Fatalf("ForFile(spec kit) = %v, want speckit", d)
	}
	d, ok = r.ForFile("openspec/changes/c1/tasks.md", []byte("- [ ] 1.1 x\n"))
	if !ok || d.ID() != "openspec" {
		t.Fatalf("ForFile(openspec) = %v, want openspec", d)
	}
	if _, ok := r.ForFile("tasks.json", []byte("{}")); ok {
		t.Fatal("nothing should claim a non-Markdown file no dialect names")
	}
}

// TestNativeDialectMatchesParseMarkdownPlan (SI-2): the native dialect is
// ParseMarkdownPlan with zero output change on every plan in docs/plans/.
func TestNativeDialectMatchesParseMarkdownPlan(t *testing.T) {
	plans, err := filepath.Glob(filepath.Join("..", "..", "docs", "plans", "*.md"))
	if err != nil || len(plans) < 10 {
		t.Fatalf("expected the repository's plans, got %d (%v)", len(plans), err)
	}
	for _, pf := range plans {
		b, err := os.ReadFile(pf)
		if err != nil {
			t.Fatal(err)
		}
		got, spans, err := Native().Parse(pf, b)
		if err != nil {
			t.Fatalf("%s: %v", pf, err)
		}
		want := verify.ParseMarkdownPlan("", b)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: native dialect output differs from ParseMarkdownPlan", pf)
		}
		direct, _ := nativeDialectParse(pf, b)
		if !reflect.DeepEqual(direct, want) {
			t.Errorf("%s: nativeDialectParse differs from ParseMarkdownPlan", pf)
		}
		if len(spans) != len(got.Tasks) {
			t.Fatalf("%s: %d spans for %d tasks", pf, len(spans), len(got.Tasks))
		}
		lines := strings.Split(string(b), "\n")
		for i, s := range spans {
			if s.TaskID != got.Tasks[i].ID || s.Line != got.Tasks[i].Line {
				t.Fatalf("%s: span %d = %+v, task %s line %d", pf, i, s, got.Tasks[i].ID, got.Tasks[i].Line)
			}
			if s.Col < 1 || lines[s.Line-1][s.Col-1] != '[' {
				t.Fatalf("%s: span %+v does not point at the checkbox", pf, s)
			}
		}
	}
}

// TestSpecKitAdapterInPlace (SI-3): T### ids, [x]/[X], anchors from named
// paths and inline annotations, positions in the foreign file, and the
// file untouched.
func TestSpecKitAdapterInPlace(t *testing.T) {
	root := filepath.Join("testdata", "speckit")
	rel := "specs/001-payments/tasks.md"
	before, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	plan, spans, read, err := ParseFile(SpecKit(), root, rel)
	if err != nil {
		t.Fatal(err)
	}
	if direct, directSpans := specKitParse(rel, before); !reflect.DeepEqual(direct, plan) || !reflect.DeepEqual(directSpans, spans) {
		t.Fatal("ParseFile and specKitParse disagree")
	}
	after, _ := os.ReadFile(filepath.Join(root, rel))
	if !bytes.Equal(before, after) || !bytes.Equal(before, read) {
		t.Fatal("the foreign file changed")
	}

	if len(plan.Tasks) != 4 {
		t.Fatalf("want 4 T### tasks (the preamble checkbox is not one), got %d: %+v", len(plan.Tasks), plan.Tasks)
	}
	byID := map[string]verify.Task{}
	for _, tk := range plan.Tasks {
		byID[tk.ID] = tk
	}
	t1, t2, t3, t4 := byID["T001"], byID["T002"], byID["T003"], byID["T004"]
	if !t1.Claimed || !t2.Claimed || t3.Claimed || !t4.Claimed {
		t.Fatalf("claimed states wrong: %v %v %v %v", t1.Claimed, t2.Claimed, t3.Claimed, t4.Claimed)
	}
	if t1.Oracle != "lexical" || t1.OraclePath != "src/payments/client.go" {
		t.Errorf("T001 anchor from its named path: %+v", t1)
	}
	if t2.Expect.Symbol != "Retry" || t2.Expect.Package != "src/payments" || t2.Oracle != "" {
		t.Errorf("T002 inline annotation: %+v", t2)
	}
	if t2.Title != "Add retry to the payment client" || len(t2.SpecIDs) != 0 {
		t.Errorf("T002 markers must be stripped and never become spec ids: %q %v", t2.Title, t2.SpecIDs)
	}
	if t3.OraclePath != "tests/contract/payments_test.go" || !reflect.DeepEqual(t3.Ctx, []string{"docs/payments.md"}) {
		t.Errorf("T003 first path is the contract, the rest ctx: %+v", t3)
	}
	if hasContract(t4) {
		t.Errorf("T004 names nothing and must carry no contract: %+v", t4)
	}

	wantSpans := []Span{{"T001", 6, 3}, {"T002", 7, 3}, {"T003", 8, 3}, {"T004", 9, 3}}
	if !reflect.DeepEqual(spans, wantSpans) {
		t.Fatalf("spans = %v, want %v", spans, wantSpans)
	}
	for _, tk := range plan.Tasks {
		if tk.Line == 0 {
			t.Fatalf("%s has no foreign line", tk.ID)
		}
	}
}

// TestOpenSpecAdapterInPlace (SI-4): numbered tasks in the change's
// tasks.md and requirement ids from its spec deltas.
func TestOpenSpecAdapterInPlace(t *testing.T) {
	root := filepath.Join("testdata", "openspec")
	rel := "openspec/changes/add-retry/tasks.md"
	before, _ := os.ReadFile(filepath.Join(root, rel))
	plan, spans, _, err := ParseFile(OpenSpec(), root, rel)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(root, rel))
	if !bytes.Equal(before, after) {
		t.Fatal("the foreign file changed")
	}
	ids := []string{}
	for _, tk := range plan.Tasks {
		ids = append(ids, tk.ID)
	}
	if want := []string{"1.1", "1.2", "1.3", "t-004"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	byID := map[string]verify.Task{}
	for _, tk := range plan.Tasks {
		byID[tk.ID] = tk
	}
	if tk := byID["1.1"]; !tk.Claimed || tk.OraclePath != "src/payments/retry.go" {
		t.Errorf("1.1: %+v", tk)
	}
	// The ADDED requirement is linked; the REMOVED one ("Retry policy",
	// which 1.1's text also names) is not.
	if got := byID["1.2"].Accept; got != "payments/Payment retries on timeout: The system SHALL retry a timed-out payment up to three times." {
		t.Errorf("1.2 accept = %q", got)
	}
	if got := byID["1.1"].Accept; got != "" {
		t.Errorf("1.1 must not link a REMOVED requirement: %q", got)
	}
	if byID["1.2"].Expect.Symbol != "Retry" {
		t.Errorf("1.2 inline anchor lost: %+v", byID["1.2"])
	}
	if hasContract(byID["1.3"]) {
		t.Errorf("1.3 names nothing: %+v", byID["1.3"])
	}
	if spans[0] != (Span{TaskID: "1.1", Line: 2, Col: 3}) {
		t.Errorf("span[0] = %+v", spans[0])
	}

	// Without deltas (the pure Parse), no requirement is linked.
	p2, _, _ := OpenSpec().Parse(rel, before)
	for _, tk := range p2.Tasks {
		if tk.Accept != "" {
			t.Errorf("%s linked a requirement without deltas: %q", tk.ID, tk.Accept)
		}
	}

	reqs := openSpecRequirements("payments", mustRead(t, filepath.Join(root, "openspec/changes/add-retry/specs/payments/spec.md")))
	if len(reqs) != 2 || reqs[0].Op != "ADDED" || reqs[1].Op != "REMOVED" || reqs[0].ID != "payments/Payment retries on timeout" {
		t.Fatalf("requirements = %+v", reqs)
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
