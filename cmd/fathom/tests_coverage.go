package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/coverage"
	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// cmdTestsCoverage implements "fathom tests coverage" (RC-40, extended for
// true per-layer attribution): per-symbol Go statement coverage from one or
// more `go test -coverprofile` files, each labeled with the test LAYER
// whose run produced it, joined against a service's indexed symbols
// (internal/analysis/coverage.ParseProfile + JoinLayeredSymbols, via
// review.ResolveCoverage — the SAME composition the review_diff MCP tool's
// coverage dimension uses, so the two surfaces never drift). With --diff
// <base>, narrows to the changed symbols (the same changed-files-to-symbols
// mapping internal/analysis/review's dimensions use: a symbol whose
// FilePath is in the diff) that have no covering test IN ANY LAYER — the
// coverage GAP a reviewer needs named, not the full per-symbol table.
//
// "By layer" is a CALLER-declared label, never guessed: --profile is
// repeatable and accepts `[layer=]path` — `--profile
// unit=workspace/cov/unit.out --profile
// integration=workspace/cov/integration.out` — because layers are run
// SEPARATELY in practice (`go test -coverprofile=unit.out ./...`, `go test
// -tags integration -coverprofile=integration.out ./...`, an e2e suite
// writing its own); a standard Go cover profile carries no per-test data to
// recover this from after the fact (see docs/use-cases/test-coverage.md).
// A bare `--profile <path>` (no `layer=` prefix) defaults to layer "unit" —
// internal/testlayer.Classify's own default for an unlabeled test file;
// there is no profile-side signal to say otherwise.
//
// Store-direct, mirroring cmdReview's own "works without the daemon"
// discipline (cmd/fathom/review.go).
//
// TQ-14w: --foreign <format>=<file> (repeatable) joins a non-Go coverage report
// (the formats `fathom coverage ingest --format` accepts) to the same indexed
// symbols — per symbol where a file's symbols carry line ranges, per file,
// named as file-level, where they do not — through
// coverage.JoinForeignBlocks. --profile is optional when --foreign is given.
//
//	fathom tests coverage --profile [layer=]<file> [--profile ...] [--foreign <format>=<file> ...] [--service S] [--store DB] [--diff <base>] [--json]
func cmdTestsCoverage(args []string, stdout, stderr io.Writer) int {
	return runTestsCoverage(args, defaultGitRunner, stdout, stderr)
}

// profileFlagRe matches an explicit `<layer>=<path>` --profile argument;
// anything else is treated as a bare path defaulting to layer "unit" (see
// defaultProfileLayer).
var profileFlagRe = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_-]*)=(.+)$`)

// defaultProfileLayer is the layer a bare, unlabeled --profile path is
// classified as — internal/testlayer.Classify's own default for a plain
// `_test.go` file with no distinguishing build tag or directory. A
// coverage profile carries no metadata naming which tests produced it, so
// this is the caller's honest fallback, never fathom's inference.
const defaultProfileLayer = "unit"

// parseProfileArg splits one --profile argument into (layer, path):
// `<layer>=<path>` when layer looks like a bare identifier (letters,
// digits, -, _, starting with a letter — never a path separator, so a
// Windows drive-letter path like "C:\cov.out" is never misread as a
// label), else the whole string is the path and the layer defaults to
// defaultProfileLayer.
func parseProfileArg(raw string) (layer, path string) {
	if m := profileFlagRe.FindStringSubmatch(raw); m != nil {
		return m[1], m[2]
	}
	return defaultProfileLayer, raw
}

// profileFlagValues collects a repeatable --profile flag (flag.Value).
type profileFlagValues []string

func (p *profileFlagValues) String() string { return strings.Join(*p, ",") }
func (p *profileFlagValues) Set(v string) error {
	*p = append(*p, v)
	return nil
}

// foreignFlagValues collects a repeatable --foreign flag (flag.Value).
type foreignFlagValues []string

func (f *foreignFlagValues) String() string { return strings.Join(*f, ",") }
func (f *foreignFlagValues) Set(v string) error {
	*f = append(*f, v)
	return nil
}

// foreignInput is one --foreign report, parsed to the shared block model.
type foreignInput struct {
	Format string
	File   string
	Blocks []coverage.ForeignBlock
}

// parseForeignArg splits one --foreign argument at its first "=" into
// (format, path); both halves are required.
func parseForeignArg(raw string) (format, path string, err error) {
	format, path, ok := strings.Cut(raw, "=")
	if !ok || format == "" || path == "" {
		return "", "", fmt.Errorf("--foreign %q: want <format>=<file> (format: lcov, cobertura, jacoco, coverage-py, istanbul or llvm-cov)", raw)
	}
	return format, path, nil
}

// loadForeignInputs reads and parses every raw --foreign argument the way
// `fathom coverage ingest` reads its one file: the same parser switch
// (parseForeignCoverage), under the same size cap (coverageIngestMaxBytes)
// enforced by the same boundedReader. The first unreadable, oversize or
// malformed report is a hard error — a report the caller named is never
// silently skipped.
func loadForeignInputs(raw []string) ([]foreignInput, error) {
	out := make([]foreignInput, 0, len(raw))
	for _, r := range raw {
		format, path, err := parseForeignArg(r)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("--foreign %q: %w", r, err)
		}
		if info.Size() > coverageIngestMaxBytes {
			return nil, fmt.Errorf("--foreign %q: %s is %d bytes, over the %d MiB coverage cap", r, path, info.Size(), coverageIngestMaxBytes/(1<<20))
		}
		f, err := os.Open(path) //nolint:gosec — an operator-supplied CLI flag
		if err != nil {
			return nil, fmt.Errorf("--foreign %q: %w", r, err)
		}
		blocks, perr := parseForeignCoverage(format, &boundedReader{r: f, max: coverageIngestMaxBytes})
		f.Close()
		if perr != nil {
			return nil, fmt.Errorf("--foreign %q: %w", r, perr)
		}
		out = append(out, foreignInput{Format: format, File: path, Blocks: blocks})
	}
	return out, nil
}

// foreignView is the --foreign reports and what they joined to: per symbol
// where the file's symbols carry line ranges, per file where they do not.
type foreignView struct {
	Inputs []foreignInput
	Join   coverage.ForeignJoinResult
}

// joinForeignInputs joins every report's blocks, in flag order, to nodes.
func joinForeignInputs(inputs []foreignInput, nodes []core.SymbolNode) *foreignView {
	if len(inputs) == 0 {
		return nil
	}
	var blocks []coverage.ForeignBlock
	for _, in := range inputs {
		blocks = append(blocks, in.Blocks...)
	}
	return &foreignView{Inputs: inputs, Join: coverage.JoinForeignBlocks(blocks, nodes)}
}

// renderForeignTable prints the foreign view under the Go table: one section
// per granularity, each named, and the report files no indexed file matched.
func renderForeignTable(w io.Writer, v *foreignView) {
	parts := make([]string, len(v.Inputs))
	for i, in := range v.Inputs {
		parts[i] = fmt.Sprintf("%s=%s (%d block(s))", in.Format, in.File, len(in.Blocks))
	}
	fmt.Fprintf(w, "\nFOREIGN COVERAGE — %s\n", strings.Join(parts, ", "))
	if len(v.Join.BySymbol) == 0 && len(v.Join.ByFile) == 0 {
		fmt.Fprintln(w, "  no block joined to an indexed file")
	}
	if len(v.Join.BySymbol) > 0 {
		fmt.Fprintf(w, "%-60s %10s %10s %10s\n", "SYMBOL (by line range)", "STATEMENTS", "COVERED", "PERCENT")
		for _, s := range v.Join.BySymbol {
			fmt.Fprintf(w, "%-60s %10d %10d %9.1f%%\n", fmt.Sprintf("%s (%s:%d)", s.Symbol, s.File, s.Line), s.Statements, s.Covered, s.Percent)
		}
	}
	if len(v.Join.ByFile) > 0 {
		fmt.Fprintln(w, "FILE-LEVEL — these files' symbols carry no line ranges, so coverage is named by file, never guessed per symbol")
		fmt.Fprintf(w, "%-60s %10s %10s %10s\n", "FILE", "STATEMENTS", "COVERED", "PERCENT")
		for _, f := range v.Join.ByFile {
			fmt.Fprintf(w, "%-60s %10d %10d %9.1f%%\n", f.File, f.Statements, f.Covered, f.Percent)
		}
	}
	if len(v.Join.Unmatched) > 0 {
		fmt.Fprintf(w, "UNMATCHED — in a report, in no indexed file (omitted, not guessed): %s\n", strings.Join(v.Join.Unmatched, ", "))
	}
}

// foreignJSON is the foreign view's --json shape: the reports, both
// granularities (each row's "level" names it) and the unmatched files.
func foreignJSON(v *foreignView) map[string]any {
	inputs := make([]any, len(v.Inputs))
	for i, in := range v.Inputs {
		inputs[i] = map[string]any{"format": in.Format, "file": in.File, "blocks": len(in.Blocks)}
	}
	bySymbol := make([]any, len(v.Join.BySymbol))
	for i, s := range v.Join.BySymbol {
		bySymbol[i] = map[string]any{
			"symbol": s.Symbol, "file": s.File, "line": s.Line, "end_line": s.EndLine, "level": "symbol",
			"statements": s.Statements, "covered": s.Covered, "percent": s.Percent, "tests": nonNilStrList(s.Tests),
		}
	}
	byFile := make([]any, len(v.Join.ByFile))
	for i, f := range v.Join.ByFile {
		byFile[i] = map[string]any{
			"file": f.File, "level": "file",
			"statements": f.Statements, "covered": f.Covered, "percent": f.Percent, "tests": nonNilStrList(f.Tests),
		}
	}
	return map[string]any{"inputs": inputs, "by_symbol": bySymbol, "by_file": byFile, "unmatched_files": nonNilStrList(v.Join.Unmatched)}
}

// loadLabeledProfiles opens and parses every raw --profile argument
// (parseProfileArg for the `[layer=]path` split), returning one
// coverage.LabeledProfile per argument. Fails loudly on the first
// unreadable or malformed file — a caller who named a profile explicitly
// gets a hard error, never a silently-skipped layer.
func loadLabeledProfiles(raw []string) ([]coverage.LabeledProfile, error) {
	out := make([]coverage.LabeledProfile, 0, len(raw))
	for _, r := range raw {
		layer, path := parseProfileArg(r)
		f, err := os.Open(path) //nolint:gosec — an operator-supplied CLI flag
		if err != nil {
			return nil, fmt.Errorf("open --profile %q: %w", r, err)
		}
		profile, perr := coverage.ParseProfile(f)
		f.Close()
		if perr != nil {
			return nil, fmt.Errorf("parse --profile %q: %w", r, perr)
		}
		out = append(out, coverage.LabeledProfile{Layer: layer, Profile: profile})
	}
	return out, nil
}

// profileModes returns the distinct coverage modes across profiles, for
// the JSON payload's "modes" field (plural: each layer's profile may have
// been run with its own -covermode).
func profileModes(profiles []coverage.LabeledProfile) []string {
	seen := map[string]bool{}
	var out []string
	for _, lp := range profiles {
		m := string(lp.Profile.Mode)
		if m != "" && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out
}

func runTestsCoverage(args []string, gitRunner func(args ...string) ([]byte, error), stdout, stderr io.Writer) int {
	fs := newFlagSet("tests coverage", stderr)
	var profileFlags profileFlagValues
	fs.Var(&profileFlags, "profile", "Go cover profile from `go test -coverprofile`, as `[layer=]path` (repeatable — one per test layer, e.g. --profile unit=workspace/cov/unit.out --profile integration=workspace/cov/integration.out); a bare path defaults to layer \"unit\" (required, at least one)")
	var foreignFlags foreignFlagValues
	fs.Var(&foreignFlags, "foreign", "TQ-14w: a non-Go coverage report as `<format>=<file>` (repeatable; format: lcov, cobertura, jacoco, coverage-py, istanbul or llvm-cov — what `fathom coverage ingest --format` accepts), joined to the service's symbols by line range where its symbols carry them, else per file, named as file-level; makes --profile optional")
	service := fs.String("service", "", "service ID (default: resolved from the repo directory name, the same guess the Claude Code hooks use)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	diffBase := fs.String("diff", "", "git ref: narrow the report to changed symbols (vs this ref) with no covering test in any layer")
	mutationPath := fs.String("mutation", "", "VB-34: mutation-testing report (go-mutesting log output, or fathom's own JSON shape — see internal/analysis/coverage.ParseMutationReport); adds a MUTATION score column")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a table")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if len(profileFlags) == 0 && len(foreignFlags) == 0 {
		fmt.Fprintln(stderr, "error: tests coverage requires at least one --profile or --foreign")
		fs.Usage()
		return 1
	}
	if len(profileFlags) == 0 && (*diffBase != "" || *mutationPath != "") {
		fmt.Fprintln(stderr, "error: --diff and --mutation read the Go --profile view; give a --profile (the foreign join is neither narrowed by --diff nor scored by --mutation)")
		return 1
	}
	if len(foreignFlags) > 0 && *diffBase != "" {
		fmt.Fprintln(stderr, "error: --diff narrows the Go --profile view only; drop --foreign to use it")
		return 1
	}

	var mutationReport coverage.MutationReport
	haveMutation := *mutationPath != ""
	if haveMutation {
		mf, ferr := os.Open(*mutationPath) //nolint:gosec — an operator-supplied CLI flag
		if ferr != nil {
			fmt.Fprintf(stderr, "error: open --mutation %q: %v\n", *mutationPath, ferr)
			return 1
		}
		mrep, perr := coverage.ParseMutationReport(mf)
		mf.Close()
		if perr != nil {
			fmt.Fprintf(stderr, "error: parse --mutation %q: %v\n", *mutationPath, perr)
			return 1
		}
		mutationReport = mrep
	}

	profiles, err := loadLabeledProfiles(profileFlags)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	foreignInputs, err := loadForeignInputs(foreignFlags)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	svc := *service
	if svc == "" {
		svc = defaultTestsCoverageService()
		if svc == "" {
			fmt.Fprintln(stderr, "error: tests coverage requires --service (could not guess one from the current directory)")
			return 1
		}
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	ctx := context.Background()

	nodes, err := c.ListAllSymbols(ctx, svc)
	if err != nil {
		fmt.Fprintf(stderr, "error: list symbols for service %q: %v\n", svc, err)
		return 1
	}

	symbols := review.ResolveCoverage(profiles, nodes)
	foreign := joinForeignInputs(foreignInputs, nodes)

	var mutationsBySymbol map[string]coverage.SymbolMutation
	if haveMutation {
		joined := coverage.JoinMutationSymbols(mutationReport, nodes)
		mutationsBySymbol = make(map[string]coverage.SymbolMutation, len(joined))
		for _, m := range joined {
			mutationsBySymbol[m.Symbol] = m
		}
	}

	var changedFiles []string
	if *diffBase != "" {
		out, gerr := gitRunner("diff", "--name-only", *diffBase)
		if gerr != nil {
			fmt.Fprintf(stderr, "error: git diff --name-only %s: %v\n", *diffBase, gerr)
			return 1
		}
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				changedFiles = append(changedFiles, line)
			}
		}
	}

	if *jsonOut {
		return renderTestsCoverageJSON(stdout, profiles, symbols, nodes, changedFiles, *diffBase, mutationsBySymbol, mutationReport, foreign)
	}
	if len(profiles) > 0 {
		renderTestsCoverageTable(stdout, profiles, symbols, nodes, changedFiles, *diffBase, mutationsBySymbol, mutationReport)
	}
	if foreign != nil {
		renderForeignTable(stdout, foreign)
	}
	return 0
}

// defaultTestsCoverageService guesses a service ID from the current
// directory — the same dir-basename-or-git-toplevel candidates the Claude
// Code hooks resolve against the daemon's index (cmd/fathom/serviceident.go
// serviceCandidates), lowercased to match `fathom init`'s own convention.
// Returns "" when the current directory cannot be determined at all.
func defaultTestsCoverageService() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	cands := serviceCandidates(cwd)
	if len(cands) == 0 {
		return ""
	}
	return strings.ToLower(cands[0].name)
}

// coverageGap is one changed symbol (via --diff) with no covering test in
// ANY supplied layer: either no layer's profile has data for it at all, or
// every layer that does reports 0% (uncoveredAt names which).
type coverageGap struct {
	symbol      string
	file        string
	line        int
	inAnyLayer  bool // true when at least one layer's profile has data for this symbol
	uncoveredAt []string
}

// changedCoverageGap computes --diff's gap list: function/method symbols
// defined in changedFiles (the same changed-files-to-symbols mapping
// internal/analysis/review's dimensions use throughout) with no covering
// test in the union of every supplied layer, sorted by (file, line).
func changedCoverageGap(nodes []core.SymbolNode, changedFiles []string, symbols []coverage.SymbolLayerCoverage) []coverageGap {
	changedSet := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changedSet[f] = true
	}
	byKey := make(map[string]coverage.SymbolLayerCoverage, len(symbols))
	for _, s := range symbols {
		byKey[s.Symbol] = s
	}

	var gaps []coverageGap
	for _, n := range nodes {
		if !changedSet[n.FilePath] || (n.Kind != core.KindGoFunc && n.Kind != core.KindGoMethod) {
			continue
		}
		key := n.Key()
		if s, ok := byKey[key]; ok {
			if s.Any.Covered > 0 {
				continue
			}
			gaps = append(gaps, coverageGap{symbol: key, file: n.FilePath, line: n.Line, inAnyLayer: true, uncoveredAt: s.UncoveredAt()})
			continue
		}
		gaps = append(gaps, coverageGap{symbol: key, file: n.FilePath, line: n.Line})
	}
	sort.Slice(gaps, func(i, j int) bool {
		if gaps[i].file != gaps[j].file {
			return gaps[i].file < gaps[j].file
		}
		return gaps[i].line < gaps[j].line
	})
	return gaps
}

// layerColumns returns the sorted, distinct set of layer labels across
// every supplied profile — the table/JSON column set (plus "any").
func layerColumns(profiles []coverage.LabeledProfile) []string {
	seen := map[string]bool{}
	var out []string
	for _, lp := range profiles {
		if !seen[lp.Layer] {
			seen[lp.Layer] = true
			out = append(out, lp.Layer)
		}
	}
	sort.Strings(out)
	return out
}

func renderTestsCoverageTable(w io.Writer, profiles []coverage.LabeledProfile, symbols []coverage.SymbolLayerCoverage, nodes []core.SymbolNode, changedFiles []string, diffBase string, mutations map[string]coverage.SymbolMutation, mutationReport coverage.MutationReport) {
	if diffBase != "" {
		gaps := changedCoverageGap(nodes, changedFiles, symbols)
		fmt.Fprintf(w, "tests coverage --diff %s: %d changed symbol(s) with no covering test in any layer\n", diffBase, len(gaps))
		for _, g := range gaps {
			note := "no profile data"
			if g.inAnyLayer {
				note = fmt.Sprintf("0%% covered (uncovered at: %s)", strings.Join(g.uncoveredAt, ", "))
			}
			fmt.Fprintf(w, "  - %s (%s:%d): %s\n", g.symbol, g.file, g.line, note)
		}
		return
	}

	layers := layerColumns(profiles)
	fmt.Fprintf(w, "%-60s", "SYMBOL")
	for _, l := range layers {
		fmt.Fprintf(w, " %10s", strings.ToUpper(l))
	}
	fmt.Fprintf(w, " %10s", "ANY")
	if mutations != nil {
		fmt.Fprintf(w, " %10s", "MUTATION")
	}
	fmt.Fprintln(w)
	for _, s := range symbols {
		fmt.Fprintf(w, "%-60s", s.Symbol)
		for _, l := range layers {
			if st, ok := s.ByLayer[l]; ok {
				fmt.Fprintf(w, " %9.1f%%", st.Percent)
			} else {
				fmt.Fprintf(w, " %10s", "-")
			}
		}
		fmt.Fprintf(w, " %9.1f%%", s.Any.Percent)
		if mutations != nil {
			if m, ok := mutations[s.Symbol]; ok {
				fmt.Fprintf(w, " %9.2f", m.Score)
			} else {
				fmt.Fprintf(w, " %10s", "-")
			}
		}
		fmt.Fprintln(w)
	}
	if mutations != nil && (mutationReport.Skipped > 0 || mutationReport.Unattributed > 0) {
		fmt.Fprintf(w, "mutation report: %d skipped (did not compile), %d unattributed (no line recovered — see docs/use-cases/test-coverage.md's --debug note)\n",
			mutationReport.Skipped, mutationReport.Unattributed)
	}
}

// nonNilStrList normalizes a nil string slice to an empty (never-null)
// JSON array — the same never-null-array convention internal/mcp/review's
// nonNilStrings established for every other list field.
func nonNilStrList(ss []string) []string {
	if ss == nil {
		return []string{}
	}
	return ss
}

func renderTestsCoverageJSON(w io.Writer, profiles []coverage.LabeledProfile, symbols []coverage.SymbolLayerCoverage, nodes []core.SymbolNode, changedFiles []string, diffBase string, mutations map[string]coverage.SymbolMutation, mutationReport coverage.MutationReport, foreign *foreignView) int {
	symItems := make([]any, len(symbols))
	for i, s := range symbols {
		byLayer := make(map[string]any, len(s.ByLayer))
		for layer, st := range s.ByLayer {
			byLayer[layer] = map[string]any{"statements": st.Statements, "covered": st.Covered, "percent": st.Percent}
		}
		item := map[string]any{
			"symbol":       s.Symbol,
			"file":         s.File,
			"line":         s.Line,
			"layers":       nonNilStrList(s.CoveringLayers()),
			"uncovered_at": nonNilStrList(s.UncoveredAt()),
			"by_layer":     byLayer,
			"any":          map[string]any{"statements": s.Any.Statements, "covered": s.Any.Covered, "percent": s.Any.Percent},
		}
		if m, ok := mutations[s.Symbol]; ok {
			item["mutation"] = map[string]any{"mutants": m.Mutants, "killed": m.Killed, "survived": m.Survived, "score": m.Score}
		}
		symItems[i] = item
	}
	payload := map[string]any{
		"modes":   nonNilStrList(profileModes(profiles)),
		"symbols": symItems,
	}
	if foreign != nil {
		payload["foreign"] = foreignJSON(foreign)
	}
	if diffBase != "" {
		gaps := changedCoverageGap(nodes, changedFiles, symbols)
		gapItems := make([]any, len(gaps))
		for i, g := range gaps {
			gapItems[i] = map[string]any{
				"symbol":       g.symbol,
				"file":         g.file,
				"line":         g.line,
				"in_profile":   g.inAnyLayer,
				"uncovered_at": nonNilStrList(g.uncoveredAt),
			}
		}
		payload["diff_base"] = diffBase
		payload["gap"] = gapItems
	}
	if mutations != nil {
		payload["mutation_summary"] = map[string]any{
			"skipped":      mutationReport.Skipped,
			"unattributed": mutationReport.Unattributed,
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return 1
	}
	return 0
}

// runTests dispatches `fathom tests coverage|select|ingest|status`.
func runTests(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "coverage" {
		return cmdTestsCoverage(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "select" {
		return cmdTestsSelect(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "ingest" {
		return cmdTestsIngest(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "status" {
		return cmdTestsStatus(args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, "usage: fathom tests coverage --profile <file> [--service S] [--diff <base>] [--json]\n       fathom tests select --since <ref> [--profile <file>]... [--service S] [--json]\n       fathom tests ingest --manifest <dir> --results <file|dir> [--service S] [--store DB]\n       fathom tests status [--service S] [--store DB] [--json]")
	return 1
}

// testsRunner runs `fathom tests` (registered below; EW-45).
type testsRunner struct{}

func (testsRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return runTests(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "tests", Group: "review", Order: 80, Run: testsRunner{},
		Summary: "tests coverage --profile <file>: per-symbol Go statement coverage by test layer (--mutation <report>: adds a mutation score column); --diff <base> narrows to changed symbols with no covering test | tests select --since <ref>: ranks Test* functions for a diff by reachability/coverage/layer with a test that already certifies a changed-file target promoted first, emits go test invocations grouped per package plus the test gap | tests ingest --manifest <dir> --results <file|dir>: loads declared tests/*.cue manifests + attested #TestResult envelopes into additive test evidence tables, joining cases to Test* symbols and scanning `// certifies:` markers (vision 33 / ADR-043 stage 0) | tests status: per suite, class/tier/last-run-verdict/totals and the case-to-symbol join rate"})
}
