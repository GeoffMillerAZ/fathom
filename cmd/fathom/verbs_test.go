package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestHelpListsEveryDispatchedVerb (HF-6) is the structural fix for the
// 2026-08-25 defect: six verbs (prime, review, fleet, the session hooks)
// were dispatched but absent from `fathom help`, so the only way to find
// them was to read main.go. The dispatch switch is parsed from source, so
// adding a `case "x":` without a verbTable row fails here — the help text
// cannot drift from what the binary answers.
func TestHelpListsEveryDispatchedVerb(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	dispatched := regexp.MustCompile(`\n\tcase "([^"]+)":`).FindAllStringSubmatch(string(src), -1)
	if len(dispatched) < 20 {
		t.Fatalf("parsed only %d dispatch cases — the switch shape changed, fix this test", len(dispatched))
	}

	inTable := map[string]bool{}
	for _, n := range verbNames() {
		inTable[n] = true
	}
	seen := map[string]bool{}
	var missing []string
	for _, m := range dispatched {
		seen[m[1]] = true
		if !inTable[m[1]] {
			missing = append(missing, m[1])
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("dispatched but absent from verbTable (invisible in `fathom help`): %s", strings.Join(missing, ", "))
	}

	var phantom []string
	for _, n := range verbNames() {
		if !seen[n] {
			phantom = append(phantom, n)
		}
	}
	sort.Strings(phantom)
	if len(phantom) > 0 {
		t.Errorf("verbTable names nothing main dispatches: %s", strings.Join(phantom, ", "))
	}

	// The rendered help is the table, so every verb appears in it.
	var out bytes.Buffer
	printUsage(&out)
	for _, n := range verbNames() {
		if !strings.Contains(out.String(), "  "+n+" ") {
			t.Errorf("printUsage output omits %q", n)
		}
	}
}

// maintainedLineNames returns the snake_case tool names and CLI verb names
// a `## Maintained` line claims. Use-case docs end with such a line naming
// what the doc depends on; on 2026-08-25 four of them still named
// `set_profile`, removed months earlier (invariant I36).
func maintainedLineNames(line string) []string {
	var names []string
	for _, tok := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(line, -1) {
		t := strings.TrimSpace(tok[1])
		t = strings.TrimPrefix(t, "fathom ")
		if i := strings.IndexAny(t, " ("); i > 0 {
			t = t[:i]
		}
		if t == "" || !regexp.MustCompile(`^[a-z][a-z0-9_-]*$`).MatchString(t) {
			continue
		}
		names = append(names, t)
	}
	sort.Strings(names)
	return names
}

// TestMaintainedLinesNameRegisteredToolsAndVerbs (HF-7) turns invariant I36
// from `stated` into a test: every name a use-case doc's `## Maintained`
// line claims must be a registered MCP tool (per docs/mcp-tools.md, itself
// registry-synced by TestMcpToolsDocMatchesRegistry) or a dispatched verb.
func TestMaintainedLinesNameRegisteredToolsAndVerbs(t *testing.T) {
	root := repoRootForDocs(t)
	toolDoc, err := os.ReadFile(filepath.Join(root, "docs", "mcp-tools.md"))
	if err != nil {
		t.Skipf("docs/mcp-tools.md unreadable: %v", err)
	}
	known := map[string]bool{}
	for _, m := range regexp.MustCompile("`([a-z][a-z0-9_]*)`").FindAllStringSubmatch(string(toolDoc), -1) {
		known[m[1]] = true
	}
	for _, v := range verbNames() {
		known[v] = true
	}
	// Sub-verbs and flags a Maintained line may legitimately name.
	for _, extra := range []string{
		"status", "next", "archive", "restore", "audit", "list", "show", "add",
		"review", "brief", "note", "point", "close", "promote", "handoff", "report", "bundle", "reconcile",
		"receipts", "undo", "explain", "set", "install", "init", "sources", "map", "compile", "eval",
		"doctor", "pack", "runs", "spend", "traffic", "channels", "interventions", "series", "rollup",
		"rotate", "near", "suppress", "unsuppress", "preview", "history", "context", "lease", "leases",
		"on", "off", "render", "marketplace", "json", "quiet", "strict", "all", "fresh", "top", "apply",
	} {
		known[extra] = true
	}

	docs, err := filepath.Glob(filepath.Join(root, "docs", "use-cases", "*.md"))
	if err != nil || len(docs) == 0 {
		t.Skip("no use-case docs")
	}
	checked := 0
	for _, doc := range docs {
		body, err := os.ReadFile(doc)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(body), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "## Maintained") &&
				!strings.Contains(line, "**Maintained**") {
				continue
			}
			checked++
			for _, name := range maintainedLineNames(line) {
				if !known[name] {
					t.Errorf("%s: `## Maintained` names %q, which is neither a documented MCP tool nor a dispatched verb",
						filepath.Base(doc), name)
				}
			}
		}
	}
	if checked == 0 {
		t.Log("no `## Maintained` lines found on one line — the contract may have moved to a following line")
	}
}

// repoRootForDocs walks up from the package dir to the repo root.
func repoRootForDocs(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Skip("repo root not found")
	return ""
}
