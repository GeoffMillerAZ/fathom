package tenancy

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// repoRoot is the module root, two directories above this package.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

const allowlistPath = ".fathom/tenancy-allow.txt"

// rules returns the sorted "rule@line" hits (excused ones suffixed "!").
func rules(t *testing.T, rel, src string) []string {
	t.Helper()
	fs, err := CheckFile(rel, []byte(src))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	var out []string
	for _, f := range fs {
		s := f.Rule
		if f.Allowed {
			s += "!"
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func TestRuleCases(t *testing.T) {
	cases := []struct {
		name, rel, src string
		want           string // comma-joined sorted rule ids, "" for none
	}{
		{"whole-read in a handler package", "internal/mcp/x.go", `package mcp
import ("os"; "io"; fp "path/filepath")
func f() { os.ReadFile("a"); io.ReadAll(nil); fp.WalkDir(".", nil); fp.Walk(".", nil); os.ReadDir(".") }`,
			"whole-read,whole-read,whole-read,whole-read,whole-read"},
		{"a bounded ReadAll is not a whole read", "internal/mcp/x.go", `package mcp
import ("io"; "net/http")
func f(r *http.Request) { io.ReadAll(io.LimitReader(r.Body, 1)); io.ReadAll(http.MaxBytesReader(nil, r.Body, 1)) }`, ""},
		{"whole-read outside handler packages is not linted", "internal/verify/x.go", `package verify
import "os"
func f() { os.ReadFile("a") }`, ""},
		{"a cmd/fathom file that serves HTTP is a handler", "cmd/fathom/panel.go", `package main
import ("net/http"; "os")
func h(w http.ResponseWriter, r *http.Request) { os.ReadFile("a") }`, "whole-read"},
		{"a cmd/fathom CLI verb is not a handler", "cmd/fathom/verbx.go", `package main
import "os"
func f() { os.ReadFile("a") }`, ""},
		{"the ingest verbs are exempt", "cmd/fathom/ingest_x.go", `package main
import ("os"; "os/exec")
func f() { os.ReadFile("a"); exec.Command("git") }`, ""},
		{"exempt packages: fusion, refresh, corpus", "internal/fusion/x.go", `package fusion
import "os/exec"
func f() { exec.Command("git"); go f() }`, ""},
		{"sources in a handler package", "internal/mcp/x.go", `package mcp
import "github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
func f(b backend) { b.ListAllSymbols(); b.SourcesForIngest(); callgraph.Build(nil) }
type backend interface{ ListAllSymbols(); SourcesForIngest() }`, "sources,sources,sources"},
		{"callgraph.Build in the graphstore group", "cmd/fathom/console_graphstore.go", `package main
import cg "github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
func f() { cg.Build(nil) }`, ""},
		{"exec-noctx everywhere", "internal/verify/x.go", `package verify
import ("context"; "os/exec")
func f(ctx context.Context) { exec.Command("git"); exec.CommandContext(ctx, "git") }`, "exec-noctx"},
		{"bg-ctx in request-path files", "cmd/fathom/console_x.go", `package main
import "context"
func f() { _ = context.Background(); _ = context.TODO() }`, "bg-ctx,bg-ctx"},
		{"bg-ctx in a servehttp_<phase>.go file", "cmd/fathom/servehttp_loops.go", `package main
import "context"
func f() { _ = context.Background() }`, "bg-ctx"},
		{"bg-ctx in internal/mcp", "internal/mcp/y.go", `package mcp
import "context"
var _ = context.Background()`, "bg-ctx"},
		{"bg-ctx not in a CLI verb", "cmd/fathom/verbx.go", `package main
import "context"
var _ = context.Background()`, ""},
		{"bare-go outside the goroutine owners", "internal/verify/x.go", `package verify
func f() { go f() }`, "bare-go"},
		{"bare-go allowed in memo, refresh, tenancy, console", "internal/memo/x.go", `package memo
func f() { go f() }`, ""},
		{"pkg-map in cmd/fathom and internal/mcp", "cmd/fathom/verbx.go", `package main
var a = map[string]int{}
var b map[string]bool
var c = make(map[int]int)
var d = []int{}`, "pkg-map,pkg-map,pkg-map"},
		{"pkg-map not elsewhere", "internal/verify/x.go", `package verify
var a = map[string]int{}`, ""},
		{"raw-serve", "internal/verify/x.go", `package verify
import "net/http"
func f() { http.ListenAndServe(":0", nil); http.Serve(nil, nil) }`, "raw-serve,raw-serve"},
		//fathom:tenancy-allow test-live-url the rule's own fixture: a string handed to CheckFile, never dialled
		{"test-live-url", "internal/verify/x_test.go", `package verify
const u = "http://127.0.0.1:7575"`, "test-live-url"},
		//fathom:tenancy-allow test-live-url the rule's own fixture: a string handed to CheckFile, never dialled
		{"test-live-url is not a source-file rule", "internal/verify/x.go", `package verify
const u = "http://127.0.0.1:7575"`, ""},
		{"test-env from a literal or nil", "internal/verify/x_test.go", `package verify
import ("os"; "os/exec")
func f() {
	c := exec.Command("x")
	c.Env = []string{"A=1"}
	c.Env = append([]string{"A=1"}, "B=2")
	c.Env = nil
	c.Env = append(os.Environ(), "A=1")
	c.Env = os.Environ()
	c.Env = someEnv
	_ = exec.Cmd{Env: []string{"A=1"}}
	_ = Config{Env: []string{"A=1"}}
	_ = map[string]string{"FOO": "bar"}
}
var someEnv []string
type Config struct{ Env []string }`, "test-env,test-env,test-env,test-env"},
		{"test files skip the source rules", "internal/mcp/x_test.go", `package mcp
import ("os"; "os/exec")
func f() { os.ReadFile("a"); exec.Command("x"); go f() }`, ""},
		{"an excuse on the line or directly above", "internal/verify/x.go", `package verify
import "os/exec"
func f() {
	exec.Command("a") //fathom:tenancy-allow exec-noctx one-shot CLI
	//fathom:tenancy-allow exec-noctx one-shot CLI
	exec.Command("b")
	//fathom:tenancy-allow bare-go joined below
	//fathom:tenancy-allow exec-noctx one-shot CLI
	go exec.Command("c")
	//fathom:tenancy-allow bare-go wrong line

	go f()
}`, "bare-go,bare-go!,exec-noctx!,exec-noctx!,exec-noctx!"},
		{"an excuse without a reason or with an unknown rule", "internal/verify/x.go", `package verify
import "os/exec"
func f() {
	exec.Command("a") //fathom:tenancy-allow exec-noctx
	exec.Command("b") //fathom:tenancy-allow no-such-rule because
}`, "allow-noreason,allow-noreason,exec-noctx,exec-noctx"},
		{"an excuse names its own rule only", "internal/verify/x.go", `package verify
import "os/exec"
func f() {
	exec.Command("a") //fathom:tenancy-allow bare-go wrong rule
}`, "exec-noctx"},
		{"imports resolve per file: a local os is not package os", "internal/mcp/x.go", `package mcp
type fake struct{}
func (fake) ReadFile(string) {}
var os fake
func f() { os.ReadFile("a") }`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(rules(t, tc.rel, tc.src), ",")
			if got != tc.want {
				t.Errorf("rules = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTenancyRules is the gate (RT-91): no finding in cmd/, internal/ or
// pkg/ goes unexcused, and .fathom/tenancy-allow.txt lists exactly the
// excuses in the tree (regenerate with FATHOM_TENANCY_ALLOW_UPDATE=1).
func TestTenancyRules(t *testing.T) {
	root := repoRoot(t)
	fs, err := CheckTenancyRules(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range Violations(fs) {
		t.Errorf("%s — fix it, or excuse it with `//fathom:tenancy-allow %s <reason>` (design §4.7)", v, v.Rule)
	}
	want := RenderAllowlist(fs)
	path := filepath.Join(root, filepath.FromSlash(allowlistPath))
	if os.Getenv("FATHOM_TENANCY_ALLOW_UPDATE") == "1" {
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v (regenerate with FATHOM_TENANCY_ALLOW_UPDATE=1 go test ./internal/lint/tenancy)", allowlistPath, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is out of date with the tree's //fathom:tenancy-allow comments; regenerate with FATHOM_TENANCY_ALLOW_UPDATE=1 go test ./internal/lint/tenancy", allowlistPath)
	}
}

// TestTenancyAllowlistOnlyShrinks (RT-92): the excuse count never rises
// above allowlistCeiling, and when it falls the ceiling must follow it down
// in the same change, so the ratchet never leaves slack to spend.
func TestTenancyAllowlistOnlyShrinks(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(allowlistPath)))
	if err != nil {
		t.Fatal(err)
	}
	n := allowlistCount(body)
	switch {
	case n > allowlistCeiling:
		t.Errorf("%s has %d entries, above the committed ceiling %d: the allowlist may only shrink — fix the new site instead of excusing it", allowlistPath, n, allowlistCeiling)
	case n < allowlistCeiling:
		t.Errorf("%s has %d entries, below the ceiling %d: lower allowlistCeiling to %d in the same change", allowlistPath, n, allowlistCeiling, n)
	}
}

func TestAllowlistCountAndRender(t *testing.T) {
	fs := []Finding{
		{Rule: RuleExecNoCtx, Path: "b.go", Allowed: true, Reason: "why b"},
		{Rule: RuleBareGo, Path: "a.go", Allowed: true, Reason: "why a"},
		{Rule: RuleBareGo, Path: "c.go"},
	}
	body := RenderAllowlist(fs)
	if n := allowlistCount(body); n != 2 {
		t.Fatalf("count = %d, want 2:\n%s", n, body)
	}
	if !strings.Contains(string(body), "bare-go\ta.go\twhy a\nexec-noctx\tb.go\twhy b\n") {
		t.Errorf("entries not sorted rule<TAB>path<TAB>reason:\n%s", body)
	}
	if allowlistCount([]byte("# only\n\n# comments\n")) != 0 {
		t.Error("comments and blank lines are not entries")
	}
}
