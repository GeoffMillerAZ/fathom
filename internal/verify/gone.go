package verify

// gone.go — CV-27/CV-28 (ADR-044 §3.1, I44, docs/design/refactor-
// conservation.md §3.1): EvaluateGone proves — or, honestly, refuses to
// prove — a `gone:` row's absence contract: an import, a module
// dependency, a lexical pattern, or a kept-but-unused symbol (callers).
//
// A row is Unknown, NEVER falsely proven, whenever any of design §3.1's
// three preconditions is unmet: the index is not at HEAD, the typed pass
// failed for packages in scope, or a file a build tag excluded from that
// pass still references the target (a heuristic scan of excluded files).
// "Proven gone" never comes from a stale or partial index (I44, CV-28).
//
// EvaluateGone is wired into verifyTask (verify.go) the same way the
// lexical/certified_by/ledger row types above it are: a `gone:` row is
// evaluated in place of the ordinary symbol/test_reaches path, never
// combined with `symbol:`/`oracle:` (one contract per row).

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// goneCriterion is the criterion name every gone: row reports under.
const goneCriterion = "gone"

// GoFile is one Go source file GoneEvidence hands EvaluateGone for the
// import (`gone: import`) form's AST scan and the callers (`gone:
// callers`) form's build-tag precondition. Excluded is "" for a file the
// typed pass actually scanned; otherwise the reason (e.g. a `//go:build`
// constraint) a build tag left it out of that pass — design §3.1's "no
// file excluded by build tags references the target (a heuristic scan of
// excluded files)" precondition needs both sets, scanned separately.
type GoFile struct {
	Path     string
	Content  []byte
	Excluded string
}

// CorpusFile is one file GoneEvidence hands EvaluateGone for the pattern
// (`gone: pattern`) form's lexical scan — any file kind, not only Go
// source: a gone: pattern row proves a string (a retired flag name, a
// deprecated config key) is gone from the corpus generally, the same
// scope the lexical oracle's own single-file `path:` reads from, widened
// to many.
type CorpusFile struct {
	Path    string
	Content []byte
}

// GoneReferent is one call-graph referent GoneEvidence reports for the
// callers (`gone: callers`) form — the same file:line shape
// safe_delete_check's own referent listing already uses.
type GoneReferent struct {
	File string
	Line int
}

// GoneEvidence is an optional capability — the same pattern FileEvidence/
// DetailedEvidence/CertificationEvidence already use (verify.go): an
// Evidence that can additionally answer a `gone:` row's absence questions.
// An Evidence without it leaves every gone: row Unknown with a reason,
// never a fabricated pass (verifyTask type-asserts for it inline, exactly
// where it type-asserts for CertificationEvidence).
type GoneEvidence interface {
	// IndexAtHead reports whether the index reflects the exact working
	// tree HEAD verify is reading the plan from (design §3.1's first
	// precondition). atHead=false names why not in reason (a stale
	// ingest, an index built from a different commit) — never silently
	// empty when false.
	IndexAtHead() (atHead bool, reason string)
	// FailedTypedPackages returns the repo-relative packages whose typed
	// pass failed to build/type-check (design §3.1's second precondition:
	// "the typed pass has zero failed packages in scope"). Empty means
	// the typed pass is clean.
	FailedTypedPackages() []string
	// GoFiles returns every Go source file in the corpus at HEAD — both
	// the ones the typed pass scanned and the ones a build tag excluded
	// from it (Excluded names why).
	GoFiles() []GoFile
	// ModuleFile returns go.mod's content at HEAD, for `gone: module`. ok
	// is false when the corpus has no go.mod at all.
	ModuleFile() ([]byte, bool)
	// CorpusFiles returns every file in the corpus at HEAD matching paths
	// (an exact repo-relative path, or a "dir/" prefix matching every
	// file under it) — every file when paths is empty.
	CorpusFiles(paths []string) []CorpusFile
	// TypedCallers returns symbol's typed-tier referents, split into
	// ordinary and Test* callers (design §3.1's callers form). pkg, when
	// non-empty, is the row's package: filter (Task.Expect.Package),
	// disambiguating a bare name that exists in more than one package —
	// the same role `package:` plays for an ordinary symbol row.
	TypedCallers(symbol, pkg string) (nonTest, test []GoneReferent)
}

// GoneReadEvidence is an optional GoneEvidence extension (O1068, I38): an
// evidence source whose corpus reads can fail or stop short names each
// failure, so a file it could not read is never scored as a file without
// the target. EvaluateGone asks for it after the form's reads: a non-empty
// answer turns what would have been a Pass into Unknown with the failures
// named, and never weakens a Fail the files it did read already prove.
type GoneReadEvidence interface {
	GoneReadFailures() []string
}

// goneReadFailures returns ev's read failures, sorted, when ev can name
// them; nil otherwise.
func goneReadFailures(ev GoneEvidence) []string {
	re, ok := ev.(GoneReadEvidence)
	if !ok {
		return nil
	}
	out := append([]string(nil), re.GoneReadFailures()...)
	sort.Strings(out)
	return out
}

// EvaluateGone evaluates one `gone:` row's absence contract against ev
// (CV-27/CV-28). Returns the Criterion under criterion name "gone" and
// the Fail/Unknown verdict label (empty on Pass) — the same (Criterion,
// label) shape every other row-level evaluator in this package uses.
func EvaluateGone(t Task, ev GoneEvidence) (Criterion, string) {
	if msg := goneContractError(t); msg != "" {
		return Criterion{goneCriterion, Fail, msg}, "contract-invalid"
	}
	form, arg, _ := parseGoneForm(t.Gone) // shape already validated above

	switch form {
	case "import":
		return evaluateGoneImport(ev, arg)
	case "module":
		return evaluateGoneModule(ev, arg)
	case "pattern":
		return evaluateGonePattern(t, ev, arg)
	case "callers":
		return evaluateGoneCallers(t, ev, arg)
	default:
		// Unreachable: goneContractError refuses every other form.
		return Criterion{goneCriterion, Fail, fmt.Sprintf("unrecognized gone: form %q", t.Gone)}, "contract-invalid"
	}
}

// goneContractError returns the static reason a gone: row's contract is
// malformed, or "" when its shape is sound — the same role
// lexicalContractError plays for the lexical oracle, shared so a caller
// (a future LintPlan check) can never disagree with EvaluateGone about
// what is refused.
func goneContractError(t Task) string {
	switch {
	case t.Expect.Symbol != "":
		return "one contract per row: symbol and gone both set"
	case t.Oracle != "":
		return "one contract per row: oracle and gone both set"
	}
	form, arg, ok := parseGoneForm(t.Gone)
	if !ok {
		return fmt.Sprintf("unrecognized gone: form %q (want import <path> / module <path> / pattern \"<regex>\" / callers <Symbol>)", t.Gone)
	}
	switch form {
	case "import":
		if arg == "" {
			return "gone: import without an import path"
		}
	case "module":
		if arg == "" {
			return "gone: module without a module path"
		}
	case "pattern":
		if arg == "" {
			return "gone: pattern without a regexp"
		}
		if _, err := regexp.Compile("(?m)" + arg); err != nil {
			return fmt.Sprintf("gone: pattern does not compile: %v", err)
		}
	case "callers":
		if arg == "" {
			return "gone: callers without a symbol"
		}
		if t.Tests != "" && t.Tests != "allowed" {
			return fmt.Sprintf("tests: %q not recognized (want \"allowed\" or unset)", t.Tests)
		}
	}
	if form != "pattern" && len(t.GonePaths) > 0 {
		return "paths: is only meaningful on gone: pattern"
	}
	if t.Eq != nil && (t.Max != nil || t.Min != nil) {
		return "eq cannot be combined with max or min (CV-29): one bound shape per row"
	}
	return ""
}

// parseGoneForm splits raw (Task.Gone's raw text) into its form
// ("import"/"module"/"pattern"/"callers") and argument. ok is false when
// raw does not start with one of the four recognized keywords. The
// pattern form's argument is additionally unquoted (`pattern "<regex>"`
// carries its regexp double-quoted, the same convention every other
// quoted plan value uses).
func parseGoneForm(raw string) (form, arg string, ok bool) {
	raw = strings.TrimSpace(raw)
	for _, f := range []string{"import", "module", "pattern", "callers"} {
		if rest, cut := strings.CutPrefix(raw, f+" "); cut {
			arg = strings.TrimSpace(rest)
			if f == "pattern" {
				arg = unquote(arg)
			}
			return f, arg, true
		}
	}
	return "", "", false
}

// gonePreconditionReasons returns design §3.1's first two preconditions'
// failure reasons (index not at HEAD, typed pass failed for packages in
// scope) — the ones every gone: form shares. The third (build-tag-
// excluded files referencing the target) is form-specific and appended by
// each evaluator.
func gonePreconditionReasons(ev GoneEvidence) []string {
	var reasons []string
	if atHead, reason := ev.IndexAtHead(); !atHead {
		if reason == "" {
			reason = "the index is not at HEAD"
		}
		reasons = append(reasons, reason)
	}
	if failed := ev.FailedTypedPackages(); len(failed) > 0 {
		sorted := append([]string(nil), failed...)
		sort.Strings(sorted)
		detail := fmt.Sprintf("the typed pass failed for %s", sorted[0])
		if len(sorted) > 1 {
			detail += fmt.Sprintf(" (+%d more)", len(sorted)-1)
		}
		reasons = append(reasons, detail)
	}
	return reasons
}

// goneUnknown renders the shared Unknown verdict every precondition
// failure reports — sorted reasons joined into one detail string, label
// "gone-unknown" throughout (CV-28: never a form-specific label, so a
// caller can recognize "not proven because of missing input" uniformly).
func goneUnknown(reasons []string) (Criterion, string) {
	sorted := append([]string(nil), reasons...)
	sort.Strings(sorted)
	return Criterion{Name: goneCriterion, Status: Unknown, Detail: "gone: not proven — " + strings.Join(sorted, "; ")}, "gone-unknown"
}

// IndexStaleMarker ends the detail of a gone: Unknown whose only reason is
// that the index is not at HEAD. It rides the detail so the daemon wire,
// which carries name/status/detail, needs no new field.
const IndexStaleMarker = "[index-stale]"

// DirtyPathMarker introduces the first uncommitted path inside such a reason.
const DirtyPathMarker = " (first dirty path: "

// goneUnknownFor is goneUnknown that also names the cause when the ONLY
// reason is the first precondition (the index is not at HEAD, O1160): the
// verdict stays Unknown (I44), and IndexStaleMarker (ending the detail) lets a gate tell an
// environment that failed every gone: row at once from a row that failed.
func goneUnknownFor(ev GoneEvidence, reasons []string) (Criterion, string) {
	c, label := goneUnknown(reasons)
	if atHead, _ := ev.IndexAtHead(); !atHead && len(reasons) == 1 {
		c.Detail += " " + IndexStaleMarker
	}
	return c, label
}

// --- import ---

func evaluateGoneImport(ev GoneEvidence, importPath string) (Criterion, string) {
	reasons := gonePreconditionReasons(ev)
	files := ev.GoFiles()

	if path, line, found := scanImports(files, importPath, false); found {
		return Criterion{goneCriterion, Fail, fmt.Sprintf("%s still imports %q at line %d", path, importPath, line)}, "gone-not-proven"
	}
	if path, line, found := scanImports(files, importPath, true); found {
		reasons = append(reasons, fmt.Sprintf("build-tag-excluded file %s imports %q at line %d", path, importPath, line))
	}
	reasons = append(reasons, goneReadFailures(ev)...)
	if len(reasons) > 0 {
		return goneUnknownFor(ev, reasons)
	}
	return Criterion{goneCriterion, Pass, fmt.Sprintf("no Go file at head imports %q (%d file(s) scanned)", importPath, len(files))}, ""
}

// scanImports finds every import of importPath among files whose Excluded
// status matches excludedOnly (false: the ordinary, typed-pass-scanned
// set; true: the build-tag-excluded set) and returns the first hit in
// deterministic (path, then line) order. A file that fails to parse is
// skipped, never fatal — this is a heuristic scan over already-read
// content, not a compile.
func scanImports(files []GoFile, importPath string, excludedOnly bool) (path string, line int, found bool) {
	type hit struct {
		path string
		line int
	}
	var hits []hit
	fset := token.NewFileSet()
	for _, f := range files {
		if (f.Excluded != "") != excludedOnly {
			continue
		}
		af, err := parser.ParseFile(fset, f.Path, f.Content, parser.ImportsOnly)
		if err != nil {
			continue
		}
		for _, imp := range af.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil || p != importPath {
				continue
			}
			hits = append(hits, hit{f.Path, fset.Position(imp.Pos()).Line})
		}
	}
	if len(hits) == 0 {
		return "", 0, false
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].path != hits[j].path {
			return hits[i].path < hits[j].path
		}
		return hits[i].line < hits[j].line
	})
	return hits[0].path, hits[0].line, true
}

// --- module ---

func evaluateGoneModule(ev GoneEvidence, modPath string) (Criterion, string) {
	reasons := gonePreconditionReasons(ev)
	body, ok := ev.ModuleFile()
	if !ok {
		reasons = append(reasons, "go.mod not found at head")
	}
	if len(reasons) > 0 {
		return goneUnknownFor(ev, reasons)
	}
	if line, found := goModRequireLine(body, modPath); found {
		return Criterion{goneCriterion, Fail, fmt.Sprintf("go.mod still requires %q at line %d", modPath, line)}, "gone-not-proven"
	}
	if failed := goneReadFailures(ev); len(failed) > 0 {
		return goneUnknown(failed)
	}
	return Criterion{goneCriterion, Pass, fmt.Sprintf("go.mod at head has no require for %q", modPath)}, ""
}

// goModRequireLine reports the 1-based line of a `require` entry for
// modPath in go.mod's raw body (single-line "require <path> <version>" or
// block "require (\n\t<path> <version>\n)" form) — a small, self-
// contained scanner rather than a new module-graph dependency, since the
// only question EvaluateGone needs answered is "does go.mod name this
// module at all," never version resolution.
func goModRequireLine(body []byte, modPath string) (line int, ok bool) {
	inBlock := false
	for i, raw := range strings.Split(string(body), "\n") {
		l := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(l, "require ("):
			inBlock = true
		case inBlock && l == ")":
			inBlock = false
		case inBlock:
			if requireLineNamesModule(l, modPath) {
				return i + 1, true
			}
		case strings.HasPrefix(l, "require "):
			if requireLineNamesModule(strings.TrimPrefix(l, "require "), modPath) {
				return i + 1, true
			}
		}
	}
	return 0, false
}

func requireLineNamesModule(l, modPath string) bool {
	if idx := strings.Index(l, "//"); idx >= 0 {
		l = l[:idx]
	}
	fields := strings.Fields(l)
	return len(fields) > 0 && fields[0] == modPath
}

// --- pattern ---

func evaluateGonePattern(t Task, ev GoneEvidence, pattern string) (Criterion, string) {
	reasons := gonePreconditionReasons(ev)
	if len(reasons) > 0 {
		return goneUnknownFor(ev, reasons)
	}
	re := regexp.MustCompile("(?m)" + pattern) // already compiled once in goneContractError
	files := ev.CorpusFiles(t.GonePaths)
	sorted := append([]CorpusFile(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	var total int
	var firstPath string
	var firstLine int
	for _, f := range sorted {
		locs := re.FindAllIndex(f.Content, -1)
		if len(locs) > 0 && firstPath == "" {
			firstPath = f.Path
			firstLine = 1 + strings.Count(string(f.Content[:locs[0][0]]), "\n")
		}
		total += len(locs)
	}
	c, label := goneBoundsVerdict(t, pattern, total, firstPath, firstLine, len(sorted))
	// A file that could not be read may hold matches: a Pass, or a Fail
	// for too FEW matches (min/eq), is then not proven either way; a count
	// already over the bound stays a Fail, since more files only add.
	if failed := goneReadFailures(ev); len(failed) > 0 && (c.Status == Pass || t.Min != nil || t.Eq != nil) {
		return goneUnknown(failed)
	}
	return c, label
}

// goneBoundsVerdict decides a gone: pattern row's pass/fail from its
// regexp's TOTAL match count across the scanned corpus (design §3.1). With
// none of Max/Min/Eq set, the default is the OPPOSITE of the ordinary
// lexical oracle's (lexicalBoundsVerdict): a gone: row proves ABSENCE, so
// count == 0 is the unbounded default, not >= 1. Any of Max/Min/Eq being
// set switches to the same range/exact semantics lexicalBoundsVerdict
// already implements — a `max: N` alone reads as a ratchet ceiling
// (design §4: "a max: N on a gone: row makes it a ratchet"), so the two
// functions only disagree on their unbounded default.
func goneBoundsVerdict(t Task, pattern string, count int, firstPath string, firstLine, scanned int) (Criterion, string) {
	switch {
	case t.Eq != nil:
		if count != *t.Eq {
			return Criterion{goneCriterion, Fail, fmt.Sprintf("pattern %q matched %d time(s) across %d file(s), want exactly %d", pattern, count, scanned, *t.Eq)}, "gone-not-proven"
		}
	case t.Max != nil || t.Min != nil:
		if t.Min != nil && count < *t.Min {
			return Criterion{goneCriterion, Fail, fmt.Sprintf("pattern %q matched %d time(s) across %d file(s), want at least %d", pattern, count, scanned, *t.Min)}, "gone-not-proven"
		}
		if t.Max != nil && count > *t.Max {
			return Criterion{goneCriterion, Fail, fmt.Sprintf("pattern %q matched %d time(s) across %d file(s), want at most %d", pattern, count, scanned, *t.Max)}, "gone-not-proven"
		}
	default:
		if count != 0 {
			return Criterion{goneCriterion, Fail, fmt.Sprintf("pattern %q still matches at %s:%d (%d total across %d file(s))", pattern, firstPath, firstLine, count, scanned)}, "gone-not-proven"
		}
	}
	return Criterion{goneCriterion, Pass, fmt.Sprintf("pattern %q matched %d time(s) across %d file(s) scanned, within bounds", pattern, count, scanned)}, ""
}

// --- callers ---

func evaluateGoneCallers(t Task, ev GoneEvidence, symbol string) (Criterion, string) {
	reasons := gonePreconditionReasons(ev)
	if path, line, found := excludedFilesReference(ev.GoFiles(), symbol); found {
		reasons = append(reasons, fmt.Sprintf("build-tag-excluded file %s references %s at line %d", path, symbol, line))
	}
	reasons = append(reasons, goneReadFailures(ev)...)
	if len(reasons) > 0 {
		return goneUnknownFor(ev, reasons)
	}
	nonTest, test := ev.TypedCallers(symbol, t.Expect.Package)
	if len(nonTest) > 0 {
		r := firstReferent(nonTest)
		return Criterion{goneCriterion, Fail, fmt.Sprintf("%s still has %d typed referent(s), e.g. %s:%d", symbol, len(nonTest), r.File, r.Line)}, "gone-not-proven"
	}
	if t.Tests != "allowed" && len(test) > 0 {
		r := firstReferent(test)
		return Criterion{goneCriterion, Fail, fmt.Sprintf("%s still has %d typed TEST referent(s), e.g. %s:%d (add tests: allowed to permit)", symbol, len(test), r.File, r.Line)}, "gone-not-proven"
	}
	return Criterion{goneCriterion, Pass, fmt.Sprintf("%s is kept but has zero typed referents (%d test referent(s))", symbol, len(test))}, ""
}

// excludedFilesReference heuristically scans build-tag-excluded files for
// an identifier matching symbol's bare name (the part after the last
// "." — a `package:`-qualified row still names its target as a bare
// method/function name in `gone: callers <Symbol>`). A false positive
// from an unrelated same-named identifier is the accepted cost of a
// heuristic-tier scan over content the typed pass never compiled in the
// first place; it can only ever push a row from Pass to Unknown, never
// fabricate a Fail.
func excludedFilesReference(files []GoFile, symbol string) (path string, line int, found bool) {
	bare := symbol
	if i := strings.LastIndex(bare, "."); i >= 0 {
		bare = bare[i+1:]
	}
	type hit struct {
		path string
		line int
	}
	var hits []hit
	fset := token.NewFileSet()
	for _, f := range files {
		if f.Excluded == "" {
			continue
		}
		af, err := parser.ParseFile(fset, f.Path, f.Content, 0)
		if err != nil {
			continue
		}
		ast.Inspect(af, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok || id.Name != bare {
				return true
			}
			hits = append(hits, hit{f.Path, fset.Position(id.Pos()).Line})
			return true
		})
	}
	if len(hits) == 0 {
		return "", 0, false
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].path != hits[j].path {
			return hits[i].path < hits[j].path
		}
		return hits[i].line < hits[j].line
	})
	return hits[0].path, hits[0].line, true
}

// firstReferent returns rs's first entry in deterministic (file, then
// line) order — rs is never mutated (a fresh sorted copy).
func firstReferent(rs []GoneReferent) GoneReferent {
	sorted := append([]GoneReferent(nil), rs...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].File != sorted[j].File {
			return sorted[i].File < sorted[j].File
		}
		return sorted[i].Line < sorted[j].Line
	})
	return sorted[0]
}
