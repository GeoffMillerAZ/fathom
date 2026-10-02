package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestHelpListsEveryDispatchedVerb (HF-6, EW-45) is the structural fix for
// the 2026-08-25 defect: six verbs (prime, review, fleet, the session hooks)
// were dispatched but absent from `fathom help`, so the only way to find
// them was to read main.go. Every verb now registers itself (declareVerb
// from an init() in its own file) and `main` dispatches through the same
// registry `fathom help` renders, so the help text cannot drift from what
// the binary answers: this test reads the registry, routes every registered
// verb through lookupVerb, and holds every help line to a registration.
func TestHelpListsEveryDispatchedVerb(t *testing.T) {
	all := verbs()
	if len(all) < 75 {
		t.Fatalf("the registry holds only %d verbs, want at least 75 — a verb's init() registration was lost", len(all))
	}

	names := map[string]bool{}
	slots := map[string]string{}
	for _, v := range all {
		if names[v.Name] {
			t.Errorf("verb %q is registered twice", v.Name)
		}
		names[v.Name] = true
		slot := fmt.Sprintf("%s/%d", v.Group, v.Order)
		if prev, ok := slots[slot]; ok {
			t.Errorf("verbs %q and %q share help slot %s", prev, v.Name, slot)
		}
		slots[slot] = v.Name
		if v.Summary == "" || v.Run == nil {
			t.Errorf("verb %q registers no summary or no run func", v.Name)
		}
		// Dispatch: main routes the name to this very registration.
		got, ok := lookupVerb(v.Name)
		if !ok || got.Name != v.Name || got.Run != v.Run {
			t.Errorf("lookupVerb(%q) does not route to its registration (ok=%v)", v.Name, ok)
		}
		if !isVerb(v.Name) {
			t.Errorf("isVerb(%q) = false for a registered verb", v.Name)
		}
	}
	if _, ok := lookupVerb("no-such-verb"); ok {
		t.Error("lookupVerb routes a name nothing registered")
	}
	if got := verbNames(); len(got) != len(all) {
		t.Errorf("verbNames() names %d verbs, the registry holds %d", len(got), len(all))
	}

	// The rendered help is the registry: every unhidden verb is a help line,
	// in registry order, and every help line is a registered, unhidden verb
	// (a Hidden verb — SR-16/SR-18 — is dispatched and registered but
	// deliberately absent from help; TestUsageOmitsFrozenVerbs pins that).
	var out bytes.Buffer
	printUsage(&out)
	var helpLines []string
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "  ") {
			helpLines = append(helpLines, strings.Fields(line)[0])
		}
	}
	var want []string
	for _, v := range all {
		if !v.Hidden {
			want = append(want, v.Name)
		}
	}
	if strings.Join(helpLines, ",") != strings.Join(want, ",") {
		t.Errorf("printUsage lines = %v\nwant the unhidden registry in order = %v", helpLines, want)
	}
	for _, n := range helpLines {
		if v, ok := lookupVerb(n); !ok || v.Hidden {
			t.Errorf("help line %q names no registered, unhidden verb", n)
		}
	}
}

// TestRunRoutesThroughTheRegistry: runWithStdin's three outcomes — a
// registered verb runs, no verb or an unknown one prints the usage with
// its error line and exits 1 — read from the registry the help renders.
func TestRunRoutesThroughTheRegistry(t *testing.T) {
	var usage bytes.Buffer
	printUsage(&usage)

	var stdout, stderr bytes.Buffer
	if code := runWithStdin([]string{"help"}, strings.NewReader(""), &stdout, &stderr); code != 0 || stdout.String() != usage.String() || stderr.Len() != 0 {
		t.Errorf("help: exit %d, stdout matches usage %v, stderr %q", code, stdout.String() == usage.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithStdin(nil, strings.NewReader(""), &stdout, &stderr); code != 1 || stderr.String() != "error: no subcommand given\n"+usage.String() {
		t.Errorf("no args: exit %d, stderr %q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithStdin([]string{"no-such-verb"}, strings.NewReader(""), &stdout, &stderr); code != 1 || stderr.String() != "error: unknown subcommand \"no-such-verb\"\n"+usage.String() {
		t.Errorf("unknown verb: exit %d, stderr %q", code, stderr.String())
	}
}

// TestOrderVerbsRefusesAmbiguousDeclarations: a duplicate name, a duplicate
// (group, order) slot, an unknown group or a declaration with no run func
// is a programming error orderVerbs refuses (verbs() panics on it at first
// use, as defaultConsoleRegistry does for a page); a valid set sorts by
// group rank, then Order, then Name.
func TestOrderVerbsRefusesAmbiguousDeclarations(t *testing.T) {
	run := verbRunnerStub{}
	for name, decls := range map[string][]verb{
		"duplicate name": {{Name: "a", Summary: "s", Group: "meta", Order: 1, Run: run}, {Name: "a", Summary: "s", Group: "meta", Order: 2, Run: run}},
		"duplicate slot": {{Name: "a", Summary: "s", Group: "meta", Order: 1, Run: run}, {Name: "b", Summary: "s", Group: "meta", Order: 1, Run: run}},
		"unknown group":  {{Name: "a", Summary: "s", Group: "no-such-group", Order: 1, Run: run}},
		"no run func":    {{Name: "a", Summary: "s", Group: "meta", Order: 1}},
		"no name":        {{Summary: "s", Group: "meta", Order: 1, Run: run}},
	} {
		if _, err := orderVerbs(decls); err == nil {
			t.Errorf("%s: orderVerbs accepted it", name)
		}
	}
	got, err := orderVerbs([]verb{
		{Name: "z", Summary: "s", Group: verbGroups[len(verbGroups)-1], Order: 1, Run: run},
		{Name: "y", Summary: "s", Group: verbGroups[0], Order: 20, Run: run},
		{Name: "x", Summary: "s", Group: verbGroups[0], Order: 10, Run: run},
	})
	if err != nil {
		t.Fatalf("orderVerbs refused a valid set: %v", err)
	}
	var order []string
	for _, v := range got {
		order = append(order, v.Name)
	}
	if strings.Join(order, ",") != "x,y,z" {
		t.Errorf("orderVerbs order = %v, want x,y,z (group rank, then Order)", order)
	}
}

// verbRunnerStub is a do-nothing verbRunner for the ordering test.
type verbRunnerStub struct{}

func (verbRunnerStub) execVerb([]string, io.Reader, io.Writer, io.Writer) int { return 0 }

// TestUsageOmitsFrozenVerbs pins SR-16 and SR-18 (surface reckoning F-53
// "refined to freeze", F-70 "freeze"): `research` (ADR-046 decision 6, the
// only route for the retro corpus) and `assess` (ADR-046/ADR-053, an
// off-by-default capability) stay fully dispatched — verbNames()/isVerb()
// still name them, so scripts and hooks calling them keep working — but
// leave `fathom help`'s rendered output, so an agent stops discovering
// them by browsing help.
func TestUsageOmitsFrozenVerbs(t *testing.T) {
	for _, name := range []string{"research", "assess"} {
		if !isVerb(name) {
			t.Errorf("%q must stay dispatched (frozen, not deleted)", name)
		}
	}
	var out bytes.Buffer
	printUsage(&out)
	rendered := out.String()
	for _, name := range []string{"research", "assess"} {
		if strings.Contains(rendered, "  "+name+" ") {
			t.Errorf("printUsage output names frozen verb %q — it should be hidden from help", name)
		}
	}
	var found []string
	for _, v := range verbs() {
		if (v.Name == "research" || v.Name == "assess") && !v.Hidden {
			found = append(found, v.Name)
		}
	}
	if len(found) > 0 {
		t.Errorf("registered verbs not marked Hidden: %v", found)
	}
}

// TestUsageOmitsInternalVerbs pins SR-30 (surface reckoning F-44 "CUE
// shadows"): `gen-status-cue` and `gen-plan-cue` are internal dev/round-trip
// tooling (ADR-018 decision 3), not agent-facing verbs. They stay fully
// dispatched — verbNames()/isVerb() still name them, so the round-trip
// tests and any script calling them directly keep working — but leave
// `fathom help`'s rendered output.
func TestUsageOmitsInternalVerbs(t *testing.T) {
	for _, name := range []string{"gen-status-cue", "gen-plan-cue"} {
		if !isVerb(name) {
			t.Errorf("%q must stay dispatched (internal, not deleted)", name)
		}
	}
	var out bytes.Buffer
	printUsage(&out)
	rendered := out.String()
	for _, name := range []string{"gen-status-cue", "gen-plan-cue"} {
		if strings.Contains(rendered, "  "+name+" ") {
			t.Errorf("printUsage output names internal verb %q — it should be hidden from help", name)
		}
	}
	var found []string
	for _, v := range verbs() {
		if (v.Name == "gen-status-cue" || v.Name == "gen-plan-cue") && !v.Hidden {
			found = append(found, v.Name)
		}
	}
	if len(found) > 0 {
		t.Errorf("registered verbs not marked Hidden: %v", found)
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

// readMcpToolsDocSet returns docs/mcp-tools.md (the index) followed by every
// docs/mcp-tools/*.md file the index links, concatenated in index order
// (EW-32: the reference is split per group).
func readMcpToolsDocSet(root string) (string, error) {
	docs := filepath.Join(root, "docs")
	raw, err := os.ReadFile(filepath.Join(docs, "mcp-tools.md"))
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.Write(raw)
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`\]\((mcp-tools/[A-Za-z0-9._-]+\.md)\)`).FindAllStringSubmatch(string(raw), -1) {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		part, err := os.ReadFile(filepath.Join(docs, filepath.FromSlash(m[1])))
		if err != nil {
			return "", err
		}
		b.WriteString("\n")
		b.Write(part)
	}
	return b.String(), nil
}

// maintainedKnownNames is the vocabulary a `## Maintained` contract may use:
// every tool documented in docs/mcp-tools.md (itself registry-synced by
// TestMcpToolsDocMatchesRegistry), every dispatched verb, and the sub-verbs
// and flags a line may legitimately name.
func maintainedKnownNames(t *testing.T, root string) map[string]bool {
	t.Helper()
	toolDoc, err := readMcpToolsDocSet(root)
	if err != nil {
		t.Skipf("docs/mcp-tools.md unreadable: %v", err)
	}
	known := map[string]bool{}
	for _, m := range regexp.MustCompile("`([a-z][a-z0-9_]*)`").FindAllStringSubmatch(toolDoc, -1) {
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
		"on", "off", "render", "marketplace", "matrix", "json", "quiet", "strict", "all", "fresh", "top", "apply",
		"format", "sarif",
		// config blocks and ingest outcome words the lines legitimately cite
		"budgets", "degraded", "read-failed", "refused",
		// doctor rows and capability ids the module / intent docs maintain
		"binding_file", "binding_module_readiness", "binding_modules_mounted",
		"guard_dead_scopes", "guard_file", "guard_include_modules_unbound",
		"intent-corpus", "unproven", "unproven_claims", "doctrine",
	} {
		known[extra] = true
	}
	return known
}

// maintainedContractLines returns every `## Maintained` contract in body:
// the heading line itself when it carries backticked names (the inline
// form), else the first paragraph after the heading joined into one line
// (the section form, "This doc covers: `a`, `b` …", possibly wrapped).
// One entry per heading.
func maintainedContractLines(body string) []string {
	lines := strings.Split(body, "\n")
	var out []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "## Maintained") && !strings.Contains(line, "**Maintained**") {
			continue
		}
		if len(maintainedLineNames(line)) > 0 {
			out = append(out, line)
			continue
		}
		var para []string
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "" {
				if len(para) > 0 {
					break
				}
				continue
			}
			if strings.HasPrefix(lines[j], "#") {
				break
			}
			para = append(para, strings.TrimSpace(lines[j]))
		}
		out = append(out, strings.Join(para, " "))
	}
	return out
}

// TestMaintainedLinesNameRegisteredToolsAndVerbs (HF-7) turns invariant I36
// from `stated` into a test: every name a use-case doc's `## Maintained`
// contract claims must be a registered MCP tool (per docs/mcp-tools.md) or
// a dispatched verb. The contract is read through maintainedContractLines,
// so the section form ("## Maintained" then "This doc covers: …") is held
// to the same vocabulary as the inline form — before PC-14 the heading
// alone was scanned and the section form checked nothing.
func TestMaintainedLinesNameRegisteredToolsAndVerbs(t *testing.T) {
	root := repoRootForDocs(t)
	known := maintainedKnownNames(t, root)

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
		for _, line := range maintainedContractLines(string(body)) {
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
		t.Error("no `## Maintained` contracts found — I36 is unenforced")
	}
}

// TestMaintainedLinesParse (PC-14, invariant I36's rung lift): every
// use-case doc carries exactly one `## Maintained` contract and the
// contract parses to at least one tool or verb name — a doc whose contract
// is prose a machine cannot read is an unmaintained doc.
//
// certifies: invariant I36 (asserts)
func TestMaintainedLinesParse(t *testing.T) {
	root := repoRootForDocs(t)
	docs, err := filepath.Glob(filepath.Join(root, "docs", "use-cases", "*.md"))
	if err != nil || len(docs) == 0 {
		t.Skip("no use-case docs")
	}
	for _, doc := range docs {
		if filepath.Base(doc) == "README.md" {
			continue // the index describes the docs; it maintains nothing
		}
		body, err := os.ReadFile(doc)
		if err != nil {
			t.Errorf("%s: unreadable: %v", filepath.Base(doc), err)
			continue
		}
		contracts := maintainedContractLines(string(body))
		if len(contracts) != 1 {
			t.Errorf("%s: %d `## Maintained` contract(s), want exactly one", filepath.Base(doc), len(contracts))
			continue
		}
		if names := maintainedLineNames(contracts[0]); len(names) == 0 {
			t.Errorf("%s: the `## Maintained` contract names no tool or verb in backticks: %q", filepath.Base(doc), strings.TrimSpace(contracts[0]))
		}
	}

	// The parser itself, pinned on both forms, including a wrapped paragraph.
	inline := "## Maintained — `find_symbol`, `fathom verify`\n"
	if got := maintainedContractLines(inline); len(got) != 1 || len(maintainedLineNames(got[0])) != 2 {
		t.Errorf("inline form parsed wrong: %q", got)
	}
	section := "## Maintained\n\nThis doc covers: `verify_plan`,\n`fathom plans status` (`--json`). Update it when these change.\n\n## Next\n`nope`\n"
	got := maintainedContractLines(section)
	if len(got) != 1 {
		t.Fatalf("section form: %d contracts", len(got))
	}
	if names := maintainedLineNames(got[0]); strings.Join(names, ",") != "plans,verify_plan" {
		t.Errorf("section form names = %v (flags like --json are not names; the next heading ends the paragraph)", names)
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

// TestConfigHelpNamesEverySubverb: `fathom help` is where a session learns
// what `config` does, so every subverb cmdConfig dispatches (audit,
// propose, …) is named in the config row's summary — the verb-level
// HF-6 rule one level down. Parsed from source, so a new
// `args[0] == "x"` without a summary mention fails here.
func TestConfigHelpNamesEverySubverb(t *testing.T) {
	src, err := os.ReadFile("configcmd.go")
	if err != nil {
		t.Fatalf("read configcmd.go: %v", err)
	}
	subs := regexp.MustCompile(`args\[0\] == "([a-z-]+)"`).FindAllStringSubmatch(string(src), -1)
	if len(subs) < 2 {
		t.Fatalf("parsed only %d config subverbs — the dispatch shape changed, fix this test", len(subs))
	}
	summary := ""
	if v, ok := lookupVerb("config"); ok {
		summary = v.Summary
	}
	for _, m := range subs {
		if !regexp.MustCompile(`\| ` + m[1] + `\b`).MatchString(summary) {
			t.Errorf("config subverb %q is dispatched but `fathom help` does not name it: %q", m[1], summary)
		}
	}
}
