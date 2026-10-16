// rules.go — the `fathom rules` verb family (VB-14, docs/vision/29-verbs-
// not-lookups.md §3 wave C; ADR-035 decision 3): `gen-test` compiles a
// repo's `.fathom/guard.cue` layer_rules (VB-13) to a Go test file so the
// SAME rule text obligations serves live at the edit also fails `task
// validate` and any CI on an actual regression — "governance moves from
// advice to enforcement." `replay` (VB-16, §5 bullet 3) answers whether
// that enforcement has ever actually caught anything: it evaluates
// today's rules against a historical commit range and reports which
// commits would have failed the gate.
package main

import (
	"context"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/geoffmilleraz/fathom/internal/analysis/rulereplay"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/guardspec/guardsarif"
	"github.com/geoffmilleraz/fathom/internal/receipt"
)

// cmdRules dispatches to the rules verb named by args[0].
func cmdRules(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: rules requires a verb: gen-test, replay")
		return 1
	}
	switch args[0] {
	case "gen-test":
		return cmdRulesGenTest(args[1:], stdout, stderr)
	case "replay":
		return cmdRulesReplay(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown rules verb %q (want gen-test, replay)\n", args[0])
		return 1
	}
}

// cmdRulesReplay evaluates --rules' layer_rules (read once) against every
// commit in --since's range under --root, reporting which commits would
// have failed the gate and, of those, which were a new regression at that
// commit versus one merely inherited from its parent (VB-16). --format sarif
// (SR-47w, F-29) emits the violations the range introduced as SARIF 2.1.0.
func cmdRulesReplay(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("rules replay", stderr)
	root := fs.String("root", ".", "git repository to replay")
	rulesPath := fs.String("rules", ".fathom/guard.cue", "layer-rule file to evaluate, read once (relative to --root unless absolute)")
	since := fs.String("since", "", "a git ref (replay ref..HEAD) or a bare positive integer N (replay the last N commits)")
	jsonOut := fs.Bool("json", false, "emit the report as JSON instead of a table")
	format := fs.String("format", "", "output format: table (default) | json | sarif — SARIF 2.1.0 of the violations the range introduced, one result per violation at the commit that introduced it (an inherited repeat is not a second finding), one rule per rule id, located at the offending package; overrides --json")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *jsonOut && *format == "" {
		*format = "json"
	}
	switch *format {
	case "", "table", "json", "sarif":
	default:
		fmt.Fprintf(stderr, "error: --format %q: want table, json or sarif\n", *format)
		return 1
	}
	if *since == "" {
		fmt.Fprintln(stderr, "error: --since is required (a git ref or a bare commit count)")
		return 1
	}

	rootAbs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	ruleFile := *rulesPath
	if !filepath.IsAbs(ruleFile) {
		ruleFile = filepath.Join(rootAbs, ruleFile)
	}

	report, err := rulereplay.Replay(context.Background(), rulereplay.Options{
		RepoRoot: rootAbs,
		RuleFile: ruleFile,
		Since:    *since,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	switch *format {
	case "json":
		if err := report.WriteJSON(stdout); err != nil {
			fmt.Fprintf(stderr, "error: encoding report: %v\n", err)
			return 1
		}
	case "sarif":
		// EVX-3: the run's provenance names the range's end commit.
		_, _ = stdout.Write(cliSARIF(receipt.Run{Dir: rootAbs}, "fathom rules replay", guardsarif.GuardVerdictsSARIF(report.IntroducedViolations())))
	default:
		report.WriteTable(stdout)
	}
	return 0
}

// cmdRulesGenTest emits <out>/layer_rules_test.go: one subtest per rule
// declared in <root>/.fathom/guard.cue's layer_rules, each evaluated LIVE
// (at `go test` time, via internal/guardspec/layerrules' go/packages
// loader) against the checkout it runs in — so a later edit to guard.cue's
// rule text changes what the generated test checks without regenerating.
func cmdRulesGenTest(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("rules gen-test", stderr)
	root := fs.String("root", ".", "repo root containing .fathom/guard.cue")
	out := fs.String("out", "rules", "output directory for layer_rules_test.go (its basename is the generated package name)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	rootAbs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	guardCue := filepath.Join(rootAbs, ".fathom", "guard.cue")
	if _, err := os.Stat(guardCue); err != nil {
		fmt.Fprintf(stderr, "error: no %s — nothing to generate a test from\n", filepath.Join(*root, ".fathom", "guard.cue"))
		return 1
	}
	rules, err := guardspec.LoadLayerRules(guardCue)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if len(rules.Rules) == 0 {
		fmt.Fprintf(stdout, "no layer_rules configured in %s — nothing to generate\n", filepath.Join(*root, ".fathom", "guard.cue"))
		return 0
	}

	outAbs, err := filepath.Abs(*out)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(outAbs, 0o755); err != nil {
		fmt.Fprintf(stderr, "error: creating %s: %v\n", *out, err)
		return 1
	}

	relRoot, err := filepath.Rel(outAbs, rootAbs)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	relGuard, err := filepath.Rel(outAbs, guardCue)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	src, err := renderLayerRulesTest(layerRulesTestData{
		Package:     packageNameFor(outAbs),
		RootRel:     filepath.ToSlash(relRoot),
		GuardCueRel: filepath.ToSlash(relGuard),
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: rendering generated test: %v\n", err)
		return 1
	}

	target := filepath.Join(outAbs, "layer_rules_test.go")
	if err := os.WriteFile(target, src, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: writing %s: %v\n", target, err)
		return 1
	}

	ids := make([]string, 0, len(rules.Rules))
	for _, r := range rules.Rules {
		ids = append(ids, r.ID)
	}
	sort.Strings(ids)
	fmt.Fprintf(stdout, "wrote %s (%d rule(s): %s)\n", target, len(ids), strings.Join(ids, ", "))
	return 0
}

// packageNameFor derives a valid Go package identifier from a directory's
// basename ("rules" for "./rules", "layer_rules" for "./layer-rules"),
// falling back to "rules" when the basename yields nothing usable — the
// generated file must compile standalone regardless of --out.
func packageNameFor(dir string) string {
	base := strings.ToLower(filepath.Base(dir))
	base = regexp.MustCompile(`[^a-z0-9_]+`).ReplaceAllString(base, "_")
	base = strings.Trim(base, "_")
	if base == "" || !regexp.MustCompile(`^[a-z]`).MatchString(base) {
		return "rules"
	}
	return base
}

type layerRulesTestData struct {
	Package     string
	RootRel     string
	GuardCueRel string
}

var layerRulesTestTmpl = template.Must(template.New("layer_rules_test").Parse(`// Code generated by ` + "`fathom rules gen-test`" + `. DO NOT EDIT.
//
// One subtest per layer rule declared in .fathom/guard.cue (VB-13/VB-14,
// docs/vision/29-verbs-not-lookups.md §3 wave C; ADR-035 decision 3),
// evaluated LIVE at go test time against this checkout's TRUE go/packages
// import graph — no fathom daemon, no fathom index. Editing guard.cue's
// rule text changes what this test checks without regenerating; re-run
// ` + "`fathom rules gen-test`" + ` only if guard.cue's location moves
// relative to this file.
package {{.Package}}

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/layerrules"
)

// Paths are relative to THIS file's own directory, resolved at test run
// time — portable across any checkout location.
const (
	layerRuleRootRel  = "{{.RootRel}}"
	layerRuleGuardRel = "{{.GuardCueRel}}"
)

func layerRulePaths(t *testing.T) (root, ruleFile string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	return filepath.Join(dir, filepath.FromSlash(layerRuleRootRel)),
		filepath.Join(dir, filepath.FromSlash(layerRuleGuardRel))
}

func TestLayerRules(t *testing.T) {
	root, ruleFile := layerRulePaths(t)
	rules, err := layerrules.LoadLayerRules(ruleFile)
	if err != nil {
		t.Fatalf("layerrules.LoadLayerRules(%s): %v", ruleFile, err)
	}
	if len(rules.Rules) == 0 {
		t.Skip("no layer_rules configured in " + ruleFile)
	}

	violations, skipped, err := layerrules.Check(root, ruleFile)
	if err != nil {
		t.Fatalf("layerrules.Check(%s, %s): %v", root, ruleFile, err)
	}
	skippedSet := make(map[string]bool, len(skipped))
	for _, id := range skipped {
		skippedSet[id] = true
	}
	byRule := map[string][]layerrules.Violation{}
	for _, v := range violations {
		byRule[v.RuleID] = append(byRule[v.RuleID], v)
	}

	for _, rule := range rules.Rules {
		rule := rule
		t.Run(rule.ID, func(t *testing.T) {
			if skippedSet[rule.ID] {
				t.Skip("cross_service_direction is a live-obligations-only check (needs the fleet's typed edges); fathom rules gen-test cannot evaluate it from a single checkout")
			}
			for _, v := range byRule[rule.ID] {
				t.Errorf("%s", v.Message)
			}
		})
	}
}
`))

func renderLayerRulesTest(data layerRulesTestData) ([]byte, error) {
	var buf strings.Builder
	if err := layerRulesTestTmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	formatted, err := format.Source([]byte(buf.String()))
	if err != nil {
		return nil, fmt.Errorf("generated source does not parse: %w\n---\n%s", err, buf.String())
	}
	return formatted, nil
}

// rulesRunner runs `fathom rules` (registered below; EW-45).
type rulesRunner struct{}

func (rulesRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdRules(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "rules", Group: "govern", Order: 60, Run: rulesRunner{},
		Summary: "Compiled governance: gen-test [--root DIR] [--out DIR] emits a Go test from .fathom/guard.cue's layer_rules | replay --since <ref|N> [--rules FILE] [--json]: evaluate today's rules against past commits"})
}
