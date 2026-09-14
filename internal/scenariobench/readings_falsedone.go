package scenariobench

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

// readings_falsedone.go is SB-21 (vision 27 §3, M2): FalseDoneRate matches
// every `[x]` the agent claimed against the INDEPENDENT oracle — the final
// tree (does the row's symbol still exist, via go/parser, exactly the way
// internal/contextbench/publicloc derives gold functions from hunks) and
// the coverage profile from the same test run (does any covered block fall
// inside the symbol's span). The fathom arm's own gate refuses a false
// claim at claim time; this reading is what an oracle fathom does not own
// says afterwards, for both arms alike.
//
// funcLoc/findFuncInTree/coverBlock/coveredInSpan below are shared with
// ContractDrift (readings_resume.go): both are "does this symbol still
// exist and is it still test-reached" checks, one over box checks, one
// over plan rows.

// funcLoc is one function's file (tree-relative) and 1-based line span, as
// found in the FINAL tree.
type funcLoc struct {
	File  string
	Start int
	End   int
}

// skipTreeDir names directories a whole-tree walk (Package == "") must not
// descend into: VCS metadata, vendored/generated trees, and fixture data —
// none of these hold the subject's own declarations.
func skipTreeDir(name string) bool {
	switch name {
	case ".git", "vendor", "node_modules", "testdata", "third_party":
		return true
	}
	return strings.HasPrefix(name, ".")
}

// isTestSymbol reports whether symbol names a Go test/benchmark/example
// function by its own convention (`go test` recognizes exactly these three
// prefixes on a top-level func) — findFuncInTree's signal to search
// _test.go files at all: a plan row or box check anchored to `TestFoo` (or
// a receiver method inside one) lives ONLY in a _test.go file, which the
// default search skips (O55 item 2 — every such row was refuted as "symbol
// not found" regardless of the work).
func isTestSymbol(symbol string) bool {
	name := symbol
	if i := strings.LastIndex(symbol, "."); i >= 0 {
		name = symbol[i+1:] // a test helper method: Recv.TestFoo is unusual but not impossible
	}
	return strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Benchmark") || strings.HasPrefix(name, "Example")
}

// splitReceiver splits a publicloc-qualified func name ("Recv.Method", or a
// bare "Method"/"Func") into its receiver and method/func parts. isMethod
// is false for a bare name (no ".").
func splitReceiver(name string) (recv, method string, isMethod bool) {
	i := strings.LastIndex(name, ".")
	if i < 0 {
		return "", name, false
	}
	return name[:i], name[i+1:], true
}

// funcSpanMatchesSymbol reports whether spanName — publicloc.FuncSpans' own
// QualifiedFuncName, "Recv.Method" for a method or a bare name for a
// top-level func — is what symbol names, accepting either spelling of a
// method on either side (O55 item 2): a plan row or box check is free to
// write a governed method as the bare method name OR the qualified
// "Recv.Method" form, and a tree symbol found under one form must still
// match a row written in the other. Two DIFFERENT receivers with the same
// method name are never conflated: the bare-vs-qualified fallback only
// fires when at least one side carries no receiver at all.
func funcSpanMatchesSymbol(spanName, symbol string) bool {
	if spanName == symbol {
		return true
	}
	spanRecv, spanMethod, spanIsMethod := splitReceiver(spanName)
	symRecv, symMethod, symIsMethod := splitReceiver(symbol)
	switch {
	case spanIsMethod && symIsMethod:
		return spanRecv == symRecv && spanMethod == symMethod
	case spanIsMethod && !symIsMethod:
		return spanMethod == symbol
	case !spanIsMethod && symIsMethod:
		return spanName == symMethod
	default:
		return false
	}
}

// findFuncInTree finds symbol's declaration in the final tree: when pkgDir
// is non-empty it searches only that directory's own .go files (Go package
// scope is non-recursive); when pkgDir is empty it walks the whole tree,
// skipping skipTreeDir. _test.go files are searched too, but ONLY when
// symbol itself looks like a test/benchmark/example function (isTestSymbol)
// — a normal source symbol has no business being "found" only inside a
// _test.go file, and the default search stays scoped to non-test sources
// exactly as before. Method symbols match either the bare method name or
// its "Recv.Method" qualification (funcSpanMatchesSymbol) on both sides.
// Unreadable or unparsable files are skipped rather than failing the
// search — this is a best-effort span finder for a bench reading, not an
// audit instrument that must account for every byte.
func findFuncInTree(tree, pkgDir, symbol string) (funcLoc, bool, error) {
	includeTests := isTestSymbol(symbol)
	var files []string
	if pkgDir != "" {
		dir := filepath.Join(tree, filepath.FromSlash(pkgDir))
		entries, err := os.ReadDir(dir)
		if err != nil {
			return funcLoc{}, false, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			if strings.HasSuffix(e.Name(), "_test.go") && !includeTests {
				continue
			}
			files = append(files, filepath.Join(dir, e.Name()))
		}
	} else {
		err := filepath.WalkDir(tree, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // unreadable entry: skip, keep walking
			}
			if d.IsDir() {
				if path != tree && skipTreeDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") && !includeTests {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			return funcLoc{}, false, err
		}
	}

	sort.Strings(files) // determinism: first match in a stable file order
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		spans, err := publicloc.FuncSpans(src, path)
		if err != nil {
			continue
		}
		for _, sp := range spans {
			if funcSpanMatchesSymbol(sp.Name, symbol) {
				rel, err := filepath.Rel(tree, path)
				if err != nil {
					rel = path
				}
				return funcLoc{File: filepath.ToSlash(rel), Start: sp.Start, End: sp.End}, true, nil
			}
		}
	}
	return funcLoc{}, false, nil
}

// Symbol kinds classifySymbolKind reports — never persisted, just the
// vocabulary FalseDoneRate/ContractDrift use to route a row.
const (
	symbolKindFunc    = "func"
	symbolKindMethod  = "method"
	symbolKindTest    = "test"
	symbolKindUnknown = "unknown"
)

// classifySymbolKind reports what shape of Go declaration symbol names, so
// FalseDoneRate/ContractDrift can tell "this detector cannot judge this
// symbol at all" (report not_adjudicable) apart from "this detector judged
// it and it is gone" (refuted/drifted) — O55 item 2: findFuncInTree only
// ever finds FUNCTION and METHOD declarations (publicloc.FuncSpans walks
// go/ast's *ast.FuncDecl exclusively), so a symbol that cannot even shape as
// one — empty, anything with more than one ".", or containing a character
// no Go identifier ever carries — is unknown to it, never merely "not
// found".
//
// A single-dot symbol ("Recv.Method", or "pkg.Func") is a SEPARATE case
// (fix-first review, 2026-09-14): its shape alone is ambiguous — the same
// spelling equally names a method, a package-qualified function, or (the
// defect this closes) a package-qualified TYPE/const/var such as
// "core.Edge", which findFuncInTree can never find because it isn't a
// function at all. Treating every single-dot name as an adjudicable method
// refuted a type reference outright, which claims a certainty this
// detector does not have. tree/pkgDir let classifySymbolKind resolve the
// ambiguity itself: adjudicable ONLY when either (a) a "Recv.Method" span
// with exactly that receiver and name exists ANYWHERE findFuncInTree(tree,
// pkgDir, symbol) would look, or (b) the left side names a real package
// directory under tree that itself contains a bare func of the right-hand
// name (findFuncInTree(tree, recv, name)). Neither resolving is
// package-qualified-non-function: unknown (not_adjudicable), never
// refuted/drifted — refuting would assert this detector knows "X.Y" was
// ever a function, which it does not.
func classifySymbolKind(tree, pkgDir, symbol string) string {
	if symbol == "" {
		return symbolKindUnknown
	}
	if isTestSymbol(symbol) {
		if _, name, isMethod := splitReceiver(symbol); !isMethod && isValidGoIdent(name) {
			return symbolKindTest
		}
	}
	recv, name, isMethod := splitReceiver(symbol)
	if !isValidGoIdent(name) {
		return symbolKindUnknown
	}
	if !isMethod {
		return symbolKindFunc
	}
	if !isValidGoIdent(recv) {
		return symbolKindUnknown
	}
	if _, found, err := findFuncInTree(tree, pkgDir, symbol); err == nil && found {
		return symbolKindMethod
	}
	if _, found, err := findFuncInTree(tree, recv, name); err == nil && found {
		return symbolKindFunc
	}
	return symbolKindUnknown
}

// isValidGoIdent reports whether s could be a Go identifier — letters,
// digits, underscore, not starting with a digit. It does not check against
// Go's keyword list: a plan row will never legitimately name a keyword as a
// symbol, and rejecting one here would only turn a clearly-wrong row into a
// silently-skipped one instead of a refuted/drifted one.
func isValidGoIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !(unicode.IsLetter(r) || r == '_') {
				return false
			}
			continue
		}
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return false
		}
	}
	return true
}

// coverBlock is one line of a `go test -coverprofile` file:
// "path/to/file.go:12.2,15.3 3 1" → File (compiler-qualified, e.g.
// "module/path/file.go"), the 1-based line span and the hit Count.
type coverBlock struct {
	File               string
	StartLine, EndLine int
	Count              int
}

var coverLineRe = regexp.MustCompile(`^(.+\.go):(\d+)\.\d+,(\d+)\.\d+ \d+ (\d+)$`)

// parseCoverProfile reads a go coverprofile (the "mode: …" header line is
// skipped; malformed lines are skipped rather than failing the read).
func parseCoverProfile(path string) ([]coverBlock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var blocks []coverBlock
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		m := coverLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start, err1 := strconv.Atoi(m[2])
		end, err2 := strconv.Atoi(m[3])
		count, err3 := strconv.Atoi(m[4])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		blocks = append(blocks, coverBlock{File: m[1], StartLine: start, EndLine: end, Count: count})
	}
	return blocks, nil
}

// coveredInSpan reports whether any coverage block for relFile (a
// tree-relative path) intersects [start, end] with Count > 0. A
// coverprofile names files by the compiler's import-path-qualified form
// (module path + subdir), so matching is by path suffix rather than
// equality.
func coveredInSpan(blocks []coverBlock, relFile string, start, end int) bool {
	for _, b := range blocks {
		if b.File != relFile && !strings.HasSuffix(b.File, "/"+relFile) {
			continue
		}
		if b.Count > 0 && b.StartLine <= end && start <= b.EndLine {
			return true
		}
	}
	return false
}

// checkLabel names a BoxCheck in FalseDoneReading.RefutedRows: its row id
// when the harness recorded one, else its symbol, else file:turn.
func checkLabel(c BoxCheck) string {
	switch {
	case c.RowID != "":
		return c.RowID
	case c.Symbol != "":
		return c.Symbol
	default:
		return fmt.Sprintf("%s:%d", c.File, c.Turn)
	}
}

// adjudicableCheck is one BoxCheck FalseDoneRate could attach a symbol to,
// either its own contract or a resolved backtick fallback.
type adjudicableCheck struct {
	check       BoxCheck
	label       string
	symbol      string
	viaBacktick bool
}

// resolveCheckSymbol returns c's symbol: its own {symbol: ...} contract
// when set, else — a plain checklist line carries no contract at all
// (public-pilot-2/reuse-pilot-1, 2026-09-13: agents wrote plain `- [x]`
// lines in NOTES.md) — the first backticked identifier in c.Text that
// resolves to an actual function in tree, via the SAME independent oracle
// (findFuncInTree) the rest of this reading uses, never taken on faith. ok
// is false when neither source names an adjudicable symbol.
func resolveCheckSymbol(c BoxCheck, tree string) (symbol string, viaBacktick bool, ok bool) {
	if c.Symbol != "" {
		return c.Symbol, false, true
	}
	for _, m := range planBacktickRe.FindAllStringSubmatch(c.Text, -1) {
		ident := m[1]
		if _, found, err := findFuncInTree(tree, c.Package, ident); err == nil && found {
			return ident, true, true
		}
	}
	return "", false, false
}

// FalseDoneRate is SB-21 (vision 27 §3, M2): for every `[x]` box check, does
// the independent oracle — the final tree plus the coverage profile from
// the SAME test run — back up the claim? Only ADJUDICABLE rows — those
// resolveCheckSymbol could attach a symbol to — count toward Claimed and
// Refuted; a row with no symbol contract and no resolvable backticked
// identifier is NotAdjudicable, never Refuted (public-pilot-2/
// reuse-pilot-1, 2026-09-13: a plain-checklist agent refuted 100% of its
// own claims under the old all-rows-are-claims reading, which was simply
// wrong — "no symbol" is not evidence of a false claim). A row whose symbol
// no longer exists in the final tree, or whose span has no covered block,
// IS refuted. `not_measurable` fires when no row is adjudicable at all (no
// claim here can be judged), or — among adjudicable rows — when no
// coverprofile was captured: refutation by "no covered block" cannot be
// judged without one, so the reading does not pretend to.
func FalseDoneRate(checks []BoxCheck, tree string, coverprofile string) FalseDoneReading {
	var adjudicable []adjudicableCheck
	var notAdjudicableRows []string
	var matchedByBacktick []string
	for _, c := range checks {
		label := checkLabel(c)
		symbol, viaBacktick, ok := resolveCheckSymbol(c, tree)
		if !ok {
			notAdjudicableRows = append(notAdjudicableRows, label+": no symbol")
			continue
		}
		if kind := classifySymbolKind(tree, c.Package, symbol); kind == symbolKindUnknown {
			// O55 item 2: this detector only ever finds functions and
			// methods (findFuncInTree/publicloc.FuncSpans) — a symbol whose
			// own shape cannot even be one is not adjudicable, never
			// refuted, regardless of what the work actually did.
			notAdjudicableRows = append(notAdjudicableRows, fmt.Sprintf("%s: symbol `%s` is not a function or method this detector can adjudicate", label, symbol))
			continue
		}
		if viaBacktick {
			matchedByBacktick = append(matchedByBacktick, fmt.Sprintf("%s: matched by backtick `%s`", label, symbol))
		}
		adjudicable = append(adjudicable, adjudicableCheck{check: c, label: label, symbol: symbol, viaBacktick: viaBacktick})
	}
	sort.Strings(notAdjudicableRows)
	sort.Strings(matchedByBacktick)

	reading := FalseDoneReading{
		Claimed:            len(adjudicable),
		NotAdjudicable:     len(notAdjudicableRows),
		NotAdjudicableRows: notAdjudicableRows,
		MatchedByBacktick:  matchedByBacktick,
	}
	if len(adjudicable) == 0 {
		reading.NotMeasurable = "no claimed row carries a symbol contract"
		return reading
	}
	if coverprofile == "" {
		reading.NotMeasurable = "no coverprofile"
		return reading
	}
	blocks, err := parseCoverProfile(coverprofile)
	if err != nil {
		reading.NotMeasurable = fmt.Sprintf("coverprofile unreadable: %s", err)
		return reading
	}

	var refuted []string
	for _, a := range adjudicable {
		loc, found, err := findFuncInTree(tree, a.check.Package, a.symbol)
		if err != nil || !found {
			refuted = append(refuted, a.label+": symbol not found in final tree")
			continue
		}
		if !coveredInSpan(blocks, loc.File, loc.Start, loc.End) {
			refuted = append(refuted, a.label+": no covered block in span")
		}
	}
	sort.Strings(refuted)
	reading.Refuted = len(refuted)
	reading.RefutedRows = refuted
	return reading
}
