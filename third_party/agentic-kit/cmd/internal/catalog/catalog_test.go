package catalog_test

import (
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/cmd/internal/catalog"
)

var update = flag.Bool("update", false, "rewrite golden files")

// TestGolden renders the fixture tree and compares it to the checked-in
// golden file. Regenerate with: go test ./cmd/internal/catalog -run Golden -update
func TestGolden(t *testing.T) {
	entries, err := catalog.Collect("../../../cmd/kit-catalog/testdata/fixture")
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	got := catalog.Render(entries)

	const golden = "../../../cmd/kit-catalog/testdata/golden.md"
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("catalog mismatch with %s\n--- got ---\n%s\n--- want ---\n%s", golden, got, string(want))
	}
}

// TestRealRepo is a smoke test over the actual module tree: the catalog must
// find well over 100 packages and contain a few known lines, and must never
// include skipped trees (tmp/, workspace/, testdata/, _reference/).
func TestRealRepo(t *testing.T) {
	entries, err := catalog.Collect("../../..")
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(entries) <= 100 {
		t.Fatalf("found %d packages, want >100", len(entries))
	}
	for _, e := range entries {
		for _, banned := range []string{"tmp/", "workspace/", "testdata", "_reference"} {
			if strings.HasPrefix(e.RelPath, banned) || strings.Contains(e.RelPath, "/"+banned) {
				t.Errorf("package %q is in a tree that must be skipped (%s)", e.RelPath, banned)
			}
		}
	}

	md := catalog.Render(entries)
	for _, want := range []string{
		// The header must explain why the source-counted total can exceed the
		// host `go list ./...` count (js/wasm-only packages are still counted).
		"Counted from source, so build-tagged packages the host `go list ./...` cannot see",
		"- `foundations/llmclient/anthropic` (`anthropic`)",
		"- `kits/agent-harness` (`agentharness`)",
		"- `cmd/endpoint-guard` (`main`)",
		"- `cmd/kit-catalog` (`main`)",
		"## kits (",
		"## foundations/evidence (",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("catalog missing expected line %q", want)
		}
	}
}

// TestSkipDir pins the directory-skipping rules.
func TestSkipDir(t *testing.T) {
	for name, want := range map[string]bool{
		"tmp":           true,
		"workspace":     true,
		"testdata":      true,
		"cue.mod":       true,
		"node_modules":  true,
		"vendor":        true,
		".git":          true,
		"_reference":    true,
		"foundations":   false,
		"kits":          false,
		"tool-registry": false,
	} {
		if got := catalog.SkipDir(name); got != want {
			t.Errorf("SkipDir(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestSectionKey pins the grouping rules.
func TestSectionKey(t *testing.T) {
	cases := map[string]string{
		"foundations/agent/bounded-exec":   "foundations/agent",
		"foundations/contracts":            "foundations/contracts",
		"foundations/llmclient":            "foundations/llmclient",
		"foundations/llmclient/replay":     "foundations/llmclient",
		"kits/agent-harness":               "kits",
		"cmd/kit-catalog":                  "cmd",
		"scenarios/governed-investigation": "scenarios",
	}
	for in, want := range cases {
		if got := catalog.SectionKey(in); got != want {
			t.Errorf("SectionKey(%q) = %q, want %q", in, got, want)
		}
	}
}
