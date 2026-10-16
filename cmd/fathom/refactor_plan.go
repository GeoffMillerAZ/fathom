package main

// refactor_plan.go — cmdRefactorPlan (CV-33, ADR-044 §4): "fathom refactor
// plan <symbol|package|import>" writes a grammar-v2, gate-ready plan for
// retiring one target:
//
//   - a "Baseline: <HEAD sha>" header line (docs/plans/README.md's plan-
//     level pin, read by `fathom refactor status` and `fathom conserve`);
//   - one `gone: pattern` ROW PER PACKAGE that still references the target
//     TODAY, each a ratchet (`max:` at today's own count, ADR-044 §3.1/§4);
//   - a `certified_by:` row for every LIVE-certified test that reaches the
//     target, so the refactor cannot silently break a certified behaviour;
//   - the FINAL absence row: `gone: callers <Symbol>` (a resolved symbol
//     target) or `gone: import <path>` (an import/package target), proving
//     the whole corpus, unscoped;
//   - a closing instruction line naming `fathom conserve --since <baseline>
//     --strict`.
//
// Store-direct (mirrors cmd/fathom/conserve.go and cmd/fathom/certify_
// orphans.go): reads the store directly, never the daemon, and reuses
// internal/analysis/callgraph the same way cmd/fathom's other symbol-
// impact verbs already do (RenameImpact/TestImpact — "use rename_impact/
// safe_delete_check-equivalent store queries for symbols" per the design).
//
// Target disambiguation (design §4's "<symbol|package|import>"): the
// target is tried as a SYMBOL first (it must resolve in the service's own
// call graph); if it doesn't, it is tried as an IMPORT PATH directly, or —
// when it names an existing directory in the corpus — as a PACKAGE,
// converted to its import path via go.mod's own module line and handled
// identically to an import target from there. A target that is neither is
// refused with a named reason, never a guess.
import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func cmdRefactorPlan(args []string, stdout, stderr io.Writer) int {
	target, flagArgs, terr := extractSinglePositional(args, "service", "store", "out")
	if terr != nil {
		fmt.Fprintf(stderr, "error: %v\n", terr)
		fmt.Fprintln(stderr, "usage: fathom refactor plan <symbol|package|import> [--out FILE] [--service S] [--store DB]")
		return 2
	}

	fs := newFlagSet("refactor plan", stderr)
	service := fs.String("service", "", "service ID (default: resolved from the repo directory name)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	out := fs.String("out", "", "output file for the generated plan (default: stdout)")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}

	svc := *service
	if svc == "" {
		svc = defaultTestsCoverageService()
		if svc == "" {
			fmt.Fprintln(stderr, "error: refactor plan requires --service (could not guess one from the current directory)")
			return 2
		}
	}

	root := verifyRepoRoot()
	shaOut, gerr := certify.ExecGit(root, "rev-parse", "HEAD")
	if gerr != nil {
		fmt.Fprintf(stderr, "error: resolve HEAD in %s: %v\n", root, gerr)
		return 2
	}
	baseline := strings.TrimSpace(string(shaOut))

	// Every ratchet's max: is seeded from c.Sources(), which reads the
	// LIVE WORKING TREE (client.Sources -> LocalConnector -> os.ReadFile),
	// while Baseline: above is the last COMMIT. A dirty tree at
	// plan-creation time would let those disagree for the very same
	// pinned Baseline sha: `fathom refactor status`'s own baseline count
	// (git show <baseline>:path) reads the committed content, so an
	// uncommitted edit already present when the plan was written can make
	// max: and the baseline count contradict each other from the first
	// status check, before any real drift has landed. Refusing on a dirty
	// tree keeps the two reads of "today" identical by construction.
	if dirty, derr := gitPathDirty(root, "."); derr != nil {
		fmt.Fprintf(stderr, "error: check working tree cleanliness in %s: %v\n", root, derr)
		return 2
	} else if dirty {
		fmt.Fprintln(stderr, "error: refactor plan requires a clean working tree: "+
			"an uncommitted change would seed max: from content that disagrees with "+
			"the pinned Baseline commit — commit or stash first")
		return 2
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 2
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	ctx := context.Background()

	doc, berr := buildRefactorPlan(ctx, c, svc, root, baseline, target)
	if berr != nil {
		fmt.Fprintf(stderr, "error: %v\n", berr)
		return 2
	}

	if *out == "" {
		fmt.Fprint(stdout, doc)
		return 0
	}
	if werr := os.WriteFile(*out, []byte(doc), 0o644); werr != nil {
		fmt.Fprintf(stderr, "error: write %q: %v\n", *out, werr)
		return 2
	}
	fmt.Fprintf(stdout, "wrote %s\n", *out)
	return 0
}

// buildRefactorPlan renders the whole plan document for target, trying the
// SYMBOL shape first and falling back to IMPORT/PACKAGE.
func buildRefactorPlan(ctx context.Context, c *client.Client, svc, root, baseline, target string) (string, error) {
	nodes, err := c.ListAllSymbols(ctx, svc)
	if err != nil {
		return "", fmt.Errorf("list symbols for %q: %w", svc, err)
	}
	typed, _ := c.TypedEdges(ctx, svc) // best effort: empty for a service with no typed pass yet

	byKey := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byKey[n.Key()] = n
	}
	g := callgraph.Build(nodes)
	callgraph.AddTypedEdges(g, typed)

	// A Sources() failure must NEVER degrade to an all-zero-referent plan:
	// every ratchet's max: comes from countPatternInDir over sources
	// (below), so silently substituting nil here would seed max: 0 for a
	// package the SAME command's own typed call graph (byPkg/
	// CallersDetail, built without touching Sources at all) may already
	// prove has live, resolved references today — a manufactured "proven
	// gone" baseline the plan never actually measured (I44's own rule for
	// verify-time evidence applies just as much at plan-generation time).
	// Fail loud instead, naming what could not be read.
	sources, srcErr := c.Sources(ctx, svc)
	if srcErr != nil {
		return "", fmt.Errorf("read corpus sources for %q: %w (refusing to seed a ratchet's max: from a partial read)", svc, srcErr)
	}

	slug := slugify(target)
	var b strings.Builder
	fmt.Fprintf(&b, "# Refactor: %s\n\nBaseline: %s\n\n", target, baseline)

	if canon := callgraph.Canonical(g, target); canon != "" {
		if node, ok := byKey[canon]; ok {
			claims, _ := c.ClaimsForService(ctx, svc, certify.Layer) // best effort: no certifications layer yet just means no certified_by rows
			renderSymbolRefactorPlan(&b, g, byKey, sources, claims, baseline, slug, canon, node)
			return b.String(), nil
		}
	}

	importPath, ierr := resolveImportTarget(root, target)
	if ierr != nil {
		return "", fmt.Errorf("target %q is not a known symbol in %q, and %v", target, svc, ierr)
	}
	renderImportRefactorPlan(&b, sources, baseline, slug, importPath)
	return b.String(), nil
}

// renderSymbolRefactorPlan writes every row for a resolved symbol target:
// one gone: pattern ratchet per referencing package (grouped from typed,
// resolved caller edges — RenameImpact's own resolution, detailed via
// CallersDetail so each caller's Kind is known), a certified_by row per
// live-certified test TestImpact says reaches the target, the final gone:
// callers row (package: disambiguates the target's own bare name), and the
// closing conserve instruction.
func renderSymbolRefactorPlan(b *strings.Builder, g *callgraph.Graph, byKey map[string]core.SymbolNode, sources []core.SourceFile, claims []core.SemanticClaim, baseline, slug, canon string, node core.SymbolNode) {
	bareTarget := node.Name // "Name" for a func, "Recv.Method" for a method (goextract.go's own convention)
	targetPkg := path.Dir(node.FilePath)
	patternWord := bareTarget
	if i := strings.LastIndex(patternWord, "."); i >= 0 {
		patternWord = patternWord[i+1:] // a call site reads "recv.Method(", never "Recv.Method(" — scan for the trailing name only
	}
	wordRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(patternWord) + `\b`)

	byPkg := map[string]bool{}
	for _, ci := range callgraph.CallersDetail(g, canon) {
		if !isResolvedKind(ci.Kind) {
			continue // heuristic/unresolved callers are not a typed reference (design §4's own store-query basis)
		}
		if cn, ok := byKey[ci.Key]; ok {
			byPkg[path.Dir(cn.FilePath)] = true
		}
	}
	pkgs := make([]string, 0, len(byPkg))
	for p := range byPkg {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)

	n := 0
	for _, dir := range pkgs {
		n++
		count := countPatternInDir(sources, dir, wordRe)
		fmt.Fprintf(b, "- [ ] %s: no more references to %s from %s {id: %s-g%d, gone: pattern \"\\b%s\\b\", paths: \"%s/\", max: %d}\n",
			dir, bareTarget, dir, slug, n, regexp.QuoteMeta(patternWord), dir, count)
	}

	n = 0
	for _, test := range liveReachingTests(g, canon, claims) {
		n++
		fmt.Fprintf(b, "- [ ] %s stays certified through the retirement of %s {id: %s-c%d, symbol: %s, certified_by: \"%s\"}\n",
			test, bareTarget, slug, n, canon, test)
	}

	fmt.Fprintf(b, "- [ ] %s has zero typed referents anywhere in the corpus {id: %s-gone, gone: callers %s, package: %s}\n",
		bareTarget, slug, bareTarget, targetPkg)
	fmt.Fprintf(b, "\n> Run `fathom conserve --since %s --strict` before merging this refactor.\n", baseline)
}

// renderImportRefactorPlan writes every row for an import/package target: a
// gone: pattern ratchet per importing package directory (a plain Go import
// scan over the corpus' own file content, design §4's "the Go import scan
// for imports/packages" — never re-reading disk, the SAME content c.Sources
// already fetched), and the final unscoped gone: import row.
//
// The lexical pattern matches the import path's own quoted literal in
// source text using \x22 hex escapes rather than a literal '"' byte: the
// plan grammar's annotation block is quote-AWARE (internal/verify/
// markdown.go's splitPairs toggles on every literal '"' it sees), so
// embedding a real quote character inside a value would corrupt parsing of
// the rest of that row's block — and any row after it, since brace pairing
// is quote-aware too.
func renderImportRefactorPlan(b *strings.Builder, sources []core.SourceFile, baseline, slug, importPath string) {
	byPkg := map[string]bool{}
	fset := token.NewFileSet()
	for _, f := range sources {
		if !strings.HasSuffix(f.Path, ".go") || strings.HasSuffix(f.Path, "_test.go") {
			continue
		}
		af, perr := parser.ParseFile(fset, f.Path, f.Content, parser.ImportsOnly)
		if perr != nil {
			continue
		}
		for _, imp := range af.Imports {
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr == nil && p == importPath {
				byPkg[path.Dir(f.Path)] = true
			}
		}
	}
	pkgs := make([]string, 0, len(byPkg))
	for p := range byPkg {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)

	pattern := `\x22` + regexp.QuoteMeta(importPath) + `\x22`
	re := regexp.MustCompile(pattern)

	n := 0
	for _, dir := range pkgs {
		n++
		count := countPatternInDir(sources, dir, re)
		fmt.Fprintf(b, "- [ ] %s: no more files import %s {id: %s-g%d, gone: pattern \"%s\", paths: \"%s/\", max: %d}\n",
			dir, importPath, slug, n, pattern, dir, count)
	}

	fmt.Fprintf(b, "- [ ] %s is imported nowhere at head {id: %s-gone, gone: import %s}\n", importPath, slug, importPath)
	fmt.Fprintf(b, "\n> Run `fathom conserve --since %s --strict` before merging this refactor.\n", baseline)
}

// liveReachingTests returns, sorted, every Test* symbol TestImpact says
// reaches canon whose compiled certifications-layer claim is currently
// live (certify.StateLive) — design §4's "certified_by rows for certified
// targets whose tests reach the target". A service with no certifications
// layer compiled at all (claims empty) simply yields no rows: certified_by
// rows are additive, never required for a plan to be usable.
func liveReachingTests(g *callgraph.Graph, canon string, claims []core.SemanticClaim) []string {
	tests := callgraph.TestImpact(g, canon)
	if len(tests) == 0 || len(claims) == 0 {
		return nil
	}
	byTest := certify.ByTest(claims, tests)
	var live []string
	for _, test := range tests {
		for _, cert := range byTest[test] {
			if cert.State == string(certify.StateLive) {
				live = append(live, test)
				break
			}
		}
	}
	sort.Strings(live)
	return live
}

// isResolvedKind reports whether kind came from the typed pass with real
// confirmation — mirrors core.EdgeConfidenceOf's own two-word vocabulary
// (EdgeUnresolved and EdgeFileResolved are both heuristic, never a proof
// of an actual reference the ratchet should count).
func isResolvedKind(kind core.EdgeKind) bool {
	return kind != "" && kind != core.EdgeUnresolved && kind != core.EdgeFileResolved
}

// countPatternInDir sums re's matches across every source file whose path
// is dir itself or lies under "dir/" (design's own "paths: dir/ prefix
// matching every file under it" semantics) — the same universe a future
// GoneEvidence.CorpusFiles(paths) adapter would scan for this row, so
// today's seeded max: is the count that row would ALREADY satisfy the
// moment such an adapter exists.
func countPatternInDir(sources []core.SourceFile, dir string, re *regexp.Regexp) int {
	prefix := dir + "/"
	total := 0
	for _, f := range sources {
		if f.Path != dir && !strings.HasPrefix(f.Path, prefix) {
			continue
		}
		total += len(re.FindAll(f.Content, -1))
	}
	return total
}

// resolveImportTarget turns target into a Go import path: as-is when it is
// not an existing directory under root (already an import path — a
// refactor's whole point may be to retire one nothing in the corpus
// resolves to anymore), or via go.mod's own module line when it names one
// (a PACKAGE target, ADR-044 §4's "<package>" shape).
func resolveImportTarget(root, target string) (string, error) {
	clean := strings.Trim(filepath.ToSlash(target), "/")
	if clean == "" {
		return "", fmt.Errorf("empty target")
	}
	if fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(clean))); err == nil && fi.IsDir() {
		mod, merr := moduleImportPath(root)
		if merr != nil {
			return "", fmt.Errorf("resolve module path for package %q: %w", target, merr)
		}
		return mod + "/" + clean, nil
	}
	return clean, nil
}

// moduleImportPath reads root/go.mod's "module <path>" line.
func moduleImportPath(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", fmt.Errorf("no module line in go.mod")
}

// extractSinglePositional splits args into exactly one positional argument
// (the target, in ANY position — before, after, or between flags, matching
// the doc'd usage "refactor plan <target> [--out FILE] …") and the
// remaining flag tokens, so a caller can hand the latter straight to
// flag.FlagSet.Parse. Go's flag package stops parsing at the first
// non-flag token, so a positional written BEFORE any flag (the documented
// order) would otherwise swallow every flag after it as unparsed
// arguments — this walks args itself instead, recognizing exactly the
// named long-flag names (each consuming its own next token as a value,
// or an inline "--name=value" form) and treating every other token as the
// one positional. Returns an error naming the count when there is not
// exactly one.
func extractSinglePositional(args []string, flagNames ...string) (positional string, flagArgs []string, err error) {
	known := make(map[string]bool, len(flagNames))
	for _, n := range flagNames {
		known["--"+n] = true
	}
	var positionals []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if known[a] {
			flagArgs = append(flagArgs, a)
			if i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
			continue
		}
		isInlineFlag := false
		for _, n := range flagNames {
			if strings.HasPrefix(a, "--"+n+"=") {
				isInlineFlag = true
				break
			}
		}
		if isInlineFlag {
			flagArgs = append(flagArgs, a)
			continue
		}
		positionals = append(positionals, a)
	}
	if len(positionals) != 1 {
		return "", nil, fmt.Errorf("expected exactly one target argument, got %d", len(positionals))
	}
	return positionals[0], flagArgs, nil
}

// slugify reduces s to a stable, plan-id-safe token: lowercase letters and
// digits, every other rune collapsed to a single '-', leading/trailing '-'
// trimmed — so a symbol like "Recv.Method" or an import path like
// "internal/old/pkg" becomes a legal, readable id prefix.
func slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// runRefactor dispatches `fathom refactor plan|status`.
func runRefactor(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "plan" {
		return cmdRefactorPlan(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "status" {
		return cmdRefactorStatus(args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, "usage: fathom refactor plan <symbol|package|import> [--out FILE] [--service S] [--store DB]\n       fathom refactor status --plan FILE [--json]\n       fathom refactor status --migration \"<kind> <from> -> <to>\" [--store DB] [--services a,b,c] [--json]")
	return 2
}

// refactorRunner runs `fathom refactor` (registered below; EW-45).
type refactorRunner struct{}

func (refactorRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return runRefactor(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "refactor", Group: "review", Order: 50, Run: refactorRunner{},
		Summary: "refactor plan <symbol|package|import> [--out FILE] [--service S] [--store DB]: writes a gate-ready grammar-v2 plan (Baseline header, one ratcheted gone: row per referencing package, certified_by rows, the final absence row, a conserve instruction) | refactor status --plan FILE [--json]: each ratchet's live count vs its max and vs the baseline, plus drift landed on the branch since baseline | refactor status --migration \"<kind> <from> -> <to>\" [--store DB] [--services a,b,c] [--json]: the fleet-wide migration view — remaining/done sites per service, typed where available, lexical otherwise; read-only, never rewrites a site (design change-and-compliance.md)"})
}
