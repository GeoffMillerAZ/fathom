package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// draftChangesLine is the §8.2 `; changes:` line of every draft. LB-F52 reads
// the `; source:` line above it to refuse the drafted file as the same
// dialect's oracle: a draft is a starting point, never an L1 oracle.
const draftChangesLine = "drafted by indexbench lang-draft (locals2tags): @local.definition -> @name, outer node -> @definition.<kind>; scopes, references and parameters dropped; a draft to tune, never an L1 oracle (ADR-048 amendment)"

var (
	// draftSourceRe splits `-source "<repo> <path> @ <commit> (<SPDX>)"`.
	draftSourceRe = regexp.MustCompile(`^(\S+) (\S+) @ (\S+) \(([^()\s]+)\)$`)
	draftRevRe    = regexp.MustCompile(`^[0-9a-f]{40}$`)
	// draftInheritsRe reads nvim-treesitter's `; inherits: c,cpp` directive.
	draftInheritsRe = regexp.MustCompile(`(?m)^[ \t]*;[ \t]*inherits:[ \t]*(\S.*?)[ \t]*$`)
	// draftDefRe names a definition capture, locals-style (`@local.definition`,
	// `@local.definition.function`) or the older `@definition.function` on a
	// name node. Group 1 is the locals kind, group 2 the older one.
	draftDefRe = regexp.MustCompile(`^(?:local\.definition(?:\.([a-z_]+))?|definition\.([a-z_]+))$`)
)

// cmdLangDraft is `indexbench lang-draft` (LB-F47, design §13.2 step 11): it
// drafts a dialect's tags.scm — and optionally its imports.scm — from an
// nvim-treesitter (or grammar) locals.scm. The draft is a starting point a
// worker tunes with lang-probe; it is never counted as an oracle.
//
//	indexbench lang-draft -locals <locals.scm> -source "<repo> <path> @ <commit> (<SPDX>)" -out <tags.scm> [-imports-out <imports.scm>] [-dialect <id>]
func cmdLangDraft(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-draft", flag.ContinueOnError)
	fs.SetOutput(stderr)
	locals := fs.String("locals", "", "the locals.scm to draft from (required)")
	source := fs.String("source", "", `provenance for the §8.2 header: "<repo> <path> @ <40-hex commit> (<SPDX>)" (required; the licence must be on the allowlist, MPL-2.0 excepted)`)
	out := fs.String("out", "", "write the tags.scm draft here (required)")
	importsOut := fs.String("imports-out", "", "write the imports.scm draft here")
	dialect := fs.String("dialect", "", "compile the draft against this tags-engine dialect's grammar ("+strings.Join(extractor.TagsDialects(), ", ")+"); exit 1 on a compile error, the draft still written")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *locals == "" || *source == "" || *out == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, `usage: indexbench lang-draft -locals <locals.scm> -source "<repo> <path> @ <commit> (<SPDX>)" -out <tags.scm> [-imports-out <imports.scm>] [-dialect <id>]`)
		return 2
	}
	if err := checkDraftSource(*source); err != nil {
		fmt.Fprintln(stderr, "lang-draft: -source:", err)
		return 2
	}
	if sameDraftPath(*out, *locals) || (*importsOut != "" && (sameDraftPath(*importsOut, *locals) || sameDraftPath(*importsOut, *out))) {
		fmt.Fprintln(stderr, "lang-draft: -out, -imports-out and -locals must be three different files")
		return 2
	}
	src, err := os.ReadFile(*locals)
	if err != nil {
		fmt.Fprintln(stderr, "lang-draft:", err)
		return 2
	}

	res := draftFromLocals(string(src))
	tags := renderDraft(*source, res.Inherits, res.Tags)
	if err := writeDraft(*out, tags); err != nil {
		fmt.Fprintln(stderr, "lang-draft:", err)
		return 1
	}
	imports := ""
	if *importsOut != "" {
		imports = renderDraft(*source, res.Inherits, res.Imports)
		if err := writeDraft(*importsOut, imports); err != nil {
			fmt.Fprintln(stderr, "lang-draft:", err)
			return 1
		}
	}

	fmt.Fprintf(stdout, "lang-draft: %d patterns: kept %d, imports %d, dropped %d, flagged %d\n",
		res.Kept+res.ImportCount+res.Dropped, res.Kept, res.ImportCount, res.Dropped, res.Flagged)
	for _, n := range res.Notes {
		fmt.Fprintf(stdout, "%s L%d: %s\n", n.Kind, n.Line, n.Msg)
	}
	for _, p := range res.Inherits {
		fmt.Fprintf(stdout, "inherits: %s\n", p)
	}
	fmt.Fprintf(stdout, "wrote %s\n", *out)
	switch {
	case *importsOut != "":
		fmt.Fprintf(stdout, "wrote %s\n", *importsOut)
	case res.ImportCount > 0:
		fmt.Fprintf(stdout, "%d import patterns not written (no -imports-out)\n", res.ImportCount)
	}

	if *dialect != "" {
		drafts := []namedDraft{{"tags draft", tags}}
		if *importsOut != "" {
			drafts = append(drafts, namedDraft{"imports draft", imports})
		}
		target, err := extractor.ProbeTargetFor(*dialect, "")
		if err != nil {
			fmt.Fprintln(stderr, "lang-draft:", err)
			return 2
		}
		if err := compileDrafts(target, drafts); err != nil {
			fmt.Fprintf(stderr, "lang-draft: compile against %s: %v\n", *dialect, err)
			return 1
		}
		fmt.Fprintf(stdout, "compile: %s ok\n", *dialect)
	}
	return 0
}

// checkDraftSource validates -source: its shape, a 40-hex revision (a tag or a
// branch names no fixed file) and a licence on the §8.2 allowlist. MPL-2.0 is
// on that list only for files taken unmodified, and a draft modifies the file.
func checkDraftSource(s string) error {
	m := draftSourceRe.FindStringSubmatch(s)
	if m == nil {
		return fmt.Errorf(`want "<repo> <path> @ <40-hex commit> (<SPDX>)", got %q`, s)
	}
	if !draftRevRe.MatchString(m[3]) {
		return fmt.Errorf("revision %q is not a 40-hex commit (lowercase); pin the file at a commit", m[3])
	}
	spdx := m[4]
	if spdx == "MPL-2.0" {
		return fmt.Errorf("licence MPL-2.0 is allowed only for files taken unmodified (design §8.2), and a draft modifies the file")
	}
	if !tsengine.LicenceAllowed(spdx) {
		return fmt.Errorf("licence %q is not on the allowlist (design §8.2)", spdx)
	}
	return nil
}

func sameDraftPath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return aa == bb
}

func writeDraft(path, text string) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

type namedDraft struct{ name, text string }

// compileDrafts compiles each draft as a query against the dialect's grammar,
// the way lang-probe would, and returns the first error.
func compileDrafts(target extractor.ProbeTarget, drafts []namedDraft) error {
	p, err := tsengine.NewParser(context.Background(), target.Blob, target.EntryPoint)
	if err != nil {
		return err
	}
	defer p.Close()
	for _, d := range drafts {
		q, err := p.CompileQuery(d.text)
		if err != nil {
			return fmt.Errorf("%s: %w", d.name, err)
		}
		q.Close()
	}
	return nil
}

// renderDraft writes the §8.2 header, one line per `; inherits:` the locals
// file declared, and then each block after a blank line. Blocks are in source
// order, so the output is byte-stable.
func renderDraft(source string, inherits, blocks []string) string {
	var b strings.Builder
	b.WriteString("; source: " + source + "\n")
	b.WriteString("; changes: " + draftChangesLine + "\n")
	for _, p := range inherits {
		b.WriteString("; inherits: " + p + " (not followed; run lang-draft on that locals.scm and merge by hand)\n")
	}
	for _, blk := range blocks {
		b.WriteString("\n" + blk + "\n")
	}
	return b.String()
}

// draftNote is one line of the run's report: a pattern skipped for a reason,
// or a pattern kept with predicates that need a rewrite.
type draftNote struct {
	Line int
	Kind string // "skipped" or "flagged"
	Msg  string
}

// draftResult is what draftFromLocals produced. Tags and Imports hold
// rendered blocks in source order; a skipped pattern leaves a `; SKIPPED`
// comment block in Tags.
type draftResult struct {
	Tags, Imports []string
	Inherits      []string
	Notes         []draftNote
	// Kept counts patterns drafted into tags, ImportCount those drafted into
	// imports and Dropped every other pattern; Flagged counts kept or
	// import patterns that carry a predicate needing a rewrite.
	Kept, ImportCount, Dropped, Flagged int
}

// draftKinds maps a locals kind to the capture suffix a tags query uses.
// namespace becomes module and type stays type, which a dialect manifest maps
// into the closed canonical list (§2.3). Every other kind (var, parameter,
// field, associated, …) is scope machinery a tags query has no use for.
var draftKinds = map[string]string{
	"function": "function", "method": "method", "class": "class", "interface": "interface",
	"enum": "enum", "macro": "macro", "constant": "constant",
	"namespace": "module", "type": "type",
}

// draftKindHeuristics gives an untyped @local.definition (tree-sitter-pascal's
// locals) its kind from the outer node's type, first match wins. The
// constructor and enum-member rules come first because their names contain
// "struct", "const" and "entry".
var draftKindHeuristics = []struct {
	re   *regexp.Regexp
	kind string
}{
	{regexp.MustCompile(`constructor`), "constructor"},
	{regexp.MustCompile(`enumvalue|enum_value|enum_entry|enum_member|enumerator|literal`), "enum_member"},
	{regexp.MustCompile(`proc|func|method|subprogram|entry`), "function"},
	{regexp.MustCompile(`enum`), "enum"},
	{regexp.MustCompile(`class|struct|record|object|interface|type|helper`), "type"},
	{regexp.MustCompile(`module|package|namespace|unit|program|library`), "module"},
	{regexp.MustCompile(`const`), "constant"},
	{regexp.MustCompile(`macro`), "macro"},
}

func draftHeuristicKind(nodeType string) string {
	t := strings.ToLower(nodeType)
	for _, h := range draftKindHeuristics {
		if h.re.MatchString(t) {
			return h.kind
		}
	}
	return ""
}

// draftKeptPredicates run in the engine (tsengine/predicates.go); the ignored
// ones (#set! #strip! #select-adjacent! #offset!) are accepted there and do
// nothing. Both stay as written.
var draftKeptPredicates = map[string]bool{
	"#eq?": true, "#not-eq?": true, "#match?": true, "#not-match?": true, "#any-of?": true, "#not-any-of?": true,
	"#set!": true, "#strip!": true, "#select-adjacent!": true, "#offset!": true,
}

// draftFromLocals converts a locals.scm into draft tags and imports blocks.
//
// A top-level pattern is kept when it holds exactly one definition capture
// (@local.definition[.kind], or the older @definition.kind on a name node) of
// a kept kind. That capture becomes @name and the pattern's outer node — the
// node of the pattern that holds the definition; for a grouped
// `((node …) (#pred …))` the child holding it, for an alternation the branch
// holding it — gains @definition.<kind>. Scopes, references and the other
// kinds are dropped; @local.definition.import goes to the imports draft as
// @import.name. Predicates the engine does not run are flagged in a comment
// above their pattern.
func draftFromLocals(src string) draftResult {
	var res draftResult
	for _, m := range draftInheritsRe.FindAllStringSubmatch(src, -1) {
		res.Inherits = append(res.Inherits, m[1])
	}
	toks := sxTokenize(src)
	line, at := 1, 0
	for _, it := range sxItems(src, toks) {
		line += strings.Count(src[at:it.start], "\n")
		at = it.start
		c := convertDraftItem(src, toks, it, line)
		switch {
		case c.skip != "":
			res.Dropped++
			res.Notes = append(res.Notes, draftNote{line, "skipped", c.skip})
			res.Tags = append(res.Tags, fmt.Sprintf("; SKIPPED L%d: %s", line, c.skip))
		case c.block == "":
			res.Dropped++
		case c.imports:
			res.ImportCount++
			res.Imports = append(res.Imports, c.block)
		default:
			res.Kept++
			res.Tags = append(res.Tags, c.block)
		}
		if c.block != "" && len(c.flagged) > 0 {
			res.Flagged++
			res.Notes = append(res.Notes, draftNote{line, "flagged", strings.Join(c.flagged, ", ")})
		}
	}
	return res
}

// sxTok is one token of a query: a bracket, a string or an atom (a node
// name, a field, a capture, a predicate name, a quantifier, `.`, `_`).
type sxTok struct {
	kind       byte // '(' ')' '[' ']' 's' (string, quotes included) 'a' (atom)
	start, end int
}

// sxTokenize tokenizes query source, skipping `;` comments. Unlike
// tsengine's tokenizer it keeps byte spans, because the draft is an edit of
// the source text, not a re-print of it.
func sxTokenize(src string) []sxTok {
	var toks []sxTok
	for i := 0; i < len(src); {
		switch c := src[i]; {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			i++
		case c == ';':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '(' || c == ')' || c == '[' || c == ']':
			toks = append(toks, sxTok{c, i, i + 1})
			i++
		case c == '"':
			j := i + 1
			for j < len(src) && src[j] != '"' {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			if j < len(src) {
				j++
			}
			if j > len(src) {
				j = len(src)
			}
			toks = append(toks, sxTok{'s', i, j})
			i = j
		default:
			j := i
			for j < len(src) && !strings.ContainsRune(" \t\r\n()[];\"", rune(src[j])) {
				j++
			}
			toks = append(toks, sxTok{'a', i, j})
			i = j
		}
	}
	return toks
}

// sxItem is one top-level pattern: tokens [first, last) and the source bytes
// [start, end) they cover, trailing captures and quantifiers included.
type sxItem struct {
	first, last int
	start, end  int
}

// sxItems splits the tokens into top-level patterns the way the engine
// numbers them: a bracketed group, a string or a bare atom, with the captures
// and quantifiers that follow it at depth 0.
func sxItems(src string, toks []sxTok) []sxItem {
	var items []sxItem
	for i := 0; i < len(toks); {
		if toks[i].kind == ')' || toks[i].kind == ']' {
			i++ // a stray closer
			continue
		}
		first, depth := i, 0
		for ; i < len(toks); i++ {
			switch toks[i].kind {
			case '(', '[':
				depth++
			case ')', ']':
				depth--
			}
			if depth <= 0 {
				i++
				break
			}
		}
		for i < len(toks) && toks[i].kind == 'a' && isTrailingAtom(src[toks[i].start:toks[i].end]) {
			i++
		}
		items = append(items, sxItem{first, i, toks[first].start, toks[i-1].end})
	}
	return items
}

func isTrailingAtom(a string) bool {
	return strings.HasPrefix(a, "@") || a == "*" || a == "+" || a == "?"
}

// draftEdit replaces src[start:end] with text; start == end inserts.
type draftEdit struct {
	start, end int
	text       string
}

// convertedItem is what one pattern became. skip is non-empty for a pattern
// that held definitions but could not be drafted; block is empty for a
// pattern that is dropped.
type convertedItem struct {
	block   string
	imports bool
	flagged []string
	skip    string
}

func convertDraftItem(src string, toks []sxTok, it sxItem, line int) convertedItem {
	text := func(k int) string { return src[toks[k].start:toks[k].end] }
	n := it.last - it.first
	closeOf := make([]int, n) // token index of the matching closer, -1 if none
	var stack []int
	for k := range closeOf {
		closeOf[k] = -1
	}
	for k := it.first; k < it.last; k++ {
		switch toks[k].kind {
		case '(', '[':
			stack = append(stack, k)
		case ')', ']':
			if len(stack) > 0 {
				closeOf[stack[len(stack)-1]-it.first] = k
				stack = stack[:len(stack)-1]
			}
		}
	}
	balanced := len(stack) == 0
	closer := func(k int) int { return closeOf[k-it.first] }

	// Predicate groups `(#name args…)`, whose capture arguments refer to
	// captures rather than define them.
	inPred := make([]bool, n)
	var preds [][2]int
	for k := it.first; k < it.last-1; k++ {
		if isPredicateGroup(toks, src, k) && closer(k) >= 0 {
			preds = append(preds, [2]int{k, closer(k)})
			for j := k; j <= closer(k); j++ {
				inPred[j-it.first] = true
			}
			k = closer(k)
		}
	}

	var defs []int // definition capture tokens outside predicates
	hasName := false
	for k := it.first; k < it.last; k++ {
		if toks[k].kind != 'a' || inPred[k-it.first] || !strings.HasPrefix(text(k), "@") {
			continue
		}
		name := text(k)[1:]
		if name == "name" {
			hasName = true
		}
		if draftDefRe.MatchString(name) {
			defs = append(defs, k)
		}
	}
	if len(defs) == 0 {
		return convertedItem{}
	}
	if len(defs) > 1 {
		return convertedItem{skip: "several definitions in one pattern"}
	}
	if hasName {
		return convertedItem{skip: "already carries @name (not a locals pattern)"}
	}
	if !balanced {
		return convertedItem{skip: "unbalanced brackets"}
	}
	def := defs[0]
	m := draftDefRe.FindStringSubmatch(text(def)[1:])
	kind := m[1]
	if m[2] != "" {
		kind = m[2]
	}
	untyped := kind == ""

	outer, nodeType, ok := draftOuterNode(toks, src, it, def, closer)
	if !ok {
		return convertedItem{skip: "no outer node to carry @definition.<kind>"}
	}

	isImport := kind == "import"
	var tagKind, provenance string
	switch {
	case isImport:
		provenance = text(def)
	case untyped:
		tagKind = draftHeuristicKind(nodeType)
		if tagKind == "" {
			return convertedItem{skip: "untyped definition: no kind for node type " + nodeType}
		}
		provenance = fmt.Sprintf("%s (untyped; kind %s from %s)", text(def), tagKind, nodeType)
	default:
		tagKind = draftKinds[kind]
		if tagKind == "" {
			return convertedItem{} // var, parameter, field, …: scope machinery
		}
		provenance = text(def)
	}

	var edits []draftEdit
	for k := it.first; k < it.last; k++ {
		if toks[k].kind != 'a' || !strings.HasPrefix(text(k), "@") {
			continue
		}
		name := text(k)[1:]
		switch {
		case draftDefRe.MatchString(name):
			repl := "@name"
			if isImport {
				repl = "@import.name"
			}
			edits = append(edits, draftEdit{toks[k].start, toks[k].end, repl})
		case (name == "local.scope" || name == "local.reference") && !inPred[k-it.first]:
			edits = append(edits, draftDeletion(src, it, toks[k]))
		}
	}
	if !isImport {
		p := toks[outer].end
		edits = append(edits, draftEdit{p, p, " @definition." + tagKind})
	}

	var flagged []string
	for _, pr := range preds {
		name := text(pr[0] + 1)
		if draftKeptPredicates[name] {
			continue
		}
		if e, ok := luaMatchEdits(src, toks, pr); ok {
			edits = append(edits, e...)
			continue
		}
		dup := false
		for _, f := range flagged {
			dup = dup || f == name
		}
		if !dup {
			flagged = append(flagged, name)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "; from locals L%d: %s\n", line, provenance)
	for _, f := range flagged {
		fmt.Fprintf(&b, "; UNSUPPORTED predicate %s: rewrite before use\n", f)
	}
	b.WriteString(strings.TrimRight(applyDraftEdits(src[it.start:it.end], it.start, edits), " \t\r\n"))
	return convertedItem{block: b.String(), imports: isImport, flagged: flagged}
}

// draftOuterNode finds the node the definition belongs on. For a plain
// pattern it is the whole pattern. For a grouped pattern `((node …) (#pred …))`
// or a sibling sequence `((a) . (b …))` it is the child holding the
// definition capture (a capture that trails a child belongs to that child),
// and for an alternation the branch holding it. It returns the token index of
// the outer node's last token and the node's type ("" when it has none).
func draftOuterNode(toks []sxTok, src string, it sxItem, def int, closer func(int) int) (last int, nodeType string, ok bool) {
	atom := func(k int) string {
		if k < it.last && toks[k].kind == 'a' {
			if a := src[toks[k].start:toks[k].end]; a != "" && (a == "_" || a[0] == '_' || isASCIIAlpha(a[0])) {
				return a
			}
		}
		return ""
	}
	childType := func(k int) string {
		if toks[k].kind == '(' {
			return atom(k + 1)
		}
		return ""
	}
	first := it.first
	switch toks[first].kind {
	case 's':
		return first, "", true
	case '(':
		if k := first + 1; k < it.last && toks[k].kind != '(' && toks[k].kind != '[' {
			return closer(first), atom(k), true // a plain node: the whole pattern
		}
	case '[':
	default:
		return 0, "", false
	}
	if s, e, ok := draftChildHolding(toks, src, first, def, closer); ok {
		return e, childType(s), true
	}
	// A capture trailing the whole pattern: an alternation, or a group's first child.
	if end := closer(first); def > end {
		if toks[first].kind == '[' {
			return end, "", true
		}
		for k := first + 1; k < end; k++ {
			if (toks[k].kind == '(' || toks[k].kind == '[') && !isPredicateGroup(toks, src, k) {
				return closer(k), childType(k), true
			}
			if toks[k].kind == 's' {
				return k, "", true
			}
		}
	}
	return 0, "", false
}

// draftChildHolding returns the first and last token of the child of the
// group or alternation at token container that holds the definition capture
// def, descending into an alternation to the branch that holds it.
func draftChildHolding(toks []sxTok, src string, container, def int, closer func(int) int) (start, end int, ok bool) {
	prev := [2]int{-1, -1} // the last child seen, which a capture after it belongs to
	for k := container + 1; k < closer(container); k++ {
		switch toks[k].kind {
		case '(', '[':
			e := closer(k)
			if isPredicateGroup(toks, src, k) {
				k = e
				continue
			}
			if def > k && def < e {
				if toks[k].kind == '[' {
					return draftChildHolding(toks, src, k, def, closer)
				}
				return k, e, true
			}
			prev = [2]int{k, e}
			k = e
		case 's':
			prev = [2]int{k, k}
		case 'a':
			if k == def && prev[0] >= 0 {
				return prev[0], prev[1], true
			}
		}
	}
	return 0, 0, false
}

func isPredicateGroup(toks []sxTok, src string, k int) bool {
	return toks[k].kind == '(' && k+1 < len(toks) && toks[k+1].kind == 'a' && strings.HasPrefix(src[toks[k+1].start:toks[k+1].end], "#")
}

func isASCIIAlpha(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// draftDeletion removes a capture token with the blanks before it, and its
// whole line when it stood alone on one.
func draftDeletion(src string, it sxItem, t sxTok) draftEdit {
	s, e := t.start, t.end
	for s > it.start && (src[s-1] == ' ' || src[s-1] == '\t') {
		s--
	}
	if s > it.start && src[s-1] == '\n' && e < len(src) && (src[e] == '\n' || src[e] == '\r') {
		e++
		if src[e-1] == '\r' && e < len(src) && src[e] == '\n' {
			e++
		}
	}
	return draftEdit{s, e, ""}
}

// applyDraftEdits applies edits, given in absolute offsets, to s, whose first
// byte is at offset base of the source. At one offset an insertion goes before
// a deletion; edits never overlap.
func applyDraftEdits(s string, base int, edits []draftEdit) string {
	sort.SliceStable(edits, func(i, j int) bool {
		if edits[i].start != edits[j].start {
			return edits[i].start < edits[j].start
		}
		return edits[i].start == edits[i].end && edits[j].start != edits[j].end
	})
	var b strings.Builder
	cur := 0
	for _, e := range edits {
		st, en := e.start-base, e.end-base
		if en > len(s) {
			en = len(s)
		}
		if st < cur {
			continue
		}
		b.WriteString(s[cur:st])
		b.WriteString(e.text)
		cur = en
	}
	b.WriteString(s[cur:])
	return b.String()
}

// luaMatchEdits rewrites `(#lua-match? @cap "lua-pattern")` and its `not-`
// form to `#match?` / `#not-match?` when the Lua pattern maps mechanically to
// RE2. It reports false for any other predicate, and for a lua-match that does
// not map, so the caller flags it.
func luaMatchEdits(src string, toks []sxTok, pr [2]int) ([]draftEdit, bool) {
	nameTok := toks[pr[0]+1]
	var re2Name string
	switch src[nameTok.start:nameTok.end] {
	case "#lua-match?":
		re2Name = "#match?"
	case "#not-lua-match?":
		re2Name = "#not-match?"
	default:
		return nil, false
	}
	args := toks[pr[0]+2 : pr[1]]
	if len(args) != 2 || args[0].kind != 'a' || !strings.HasPrefix(src[args[0].start:args[0].end], "@") || args[1].kind != 's' {
		return nil, false
	}
	re, ok := luaPatternToRE2(decodeQueryString(src[args[1].start:args[1].end]))
	if !ok {
		return nil, false
	}
	return []draftEdit{
		{nameTok.start, nameTok.end, re2Name},
		{args[1].start, args[1].end, quoteQueryString(re)},
	}, true
}

// decodeQueryString reads a string token the way tsengine's tokenizer does.
func decodeQueryString(tok string) string {
	var b strings.Builder
	for i := 1; i < len(tok); i++ {
		c := tok[i]
		switch {
		case c == '"':
			return b.String()
		case c == '\\' && i+1 < len(tok):
			i++
			switch tok[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '0':
				b.WriteByte(0)
			default:
				b.WriteByte(tok[i])
			}
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func quoteQueryString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// luaClasses are the Lua pattern classes with an RE2 spelling: outside a set
// and inside one. Lua's %w is alphanumeric only (no underscore) and %s
// includes the vertical tab, so RE2's \w and \s are not stand-ins.
var luaClasses = map[byte][2]string{
	'a': {"[A-Za-z]", "A-Za-z"},
	'd': {"[0-9]", "0-9"},
	'w': {"[A-Za-z0-9]", "A-Za-z0-9"},
	's': {`[ \t\n\v\f\r]`, ` \t\n\v\f\r`},
	'l': {"[a-z]", "a-z"},
	'u': {"[A-Z]", "A-Z"},
	'p': {"[[:punct:]]", "[:punct:]"},
}

// luaPatternToRE2 maps a Lua pattern (what nvim's #lua-match? takes) to RE2
// (what #match? takes) when the two mean the same thing: the classes %a %d %w
// %s %l %u %p, escaped punctuation (%.), the ^ and $ anchors, sets, and the *
// + ? repetitions of a single character or class. It refuses what has no
// mechanical form: %b, %f, back references, the complement classes, Lua's
// lazy `-`, captures, and non-ASCII text (a repetition binds to one byte in
// Lua and one rune in RE2). Characters that are plain in Lua but special in
// RE2 (| { } \) are escaped.
func luaPatternToRE2(p string) (string, bool) {
	for i := 0; i < len(p); i++ {
		if p[i] >= 0x80 {
			return "", false
		}
	}
	var b strings.Builder
	canRepeat := false // the last item is a single character or class
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch c {
		case '^':
			if i == 0 {
				b.WriteByte('^')
				canRepeat = false
			} else {
				b.WriteString(`\^`)
				canRepeat = true
			}
		case '$':
			if i == len(p)-1 {
				b.WriteByte('$')
				canRepeat = false
			} else {
				b.WriteString(`\$`)
				canRepeat = true
			}
		case '*', '+', '?':
			if !canRepeat {
				return "", false
			}
			b.WriteByte(c)
			canRepeat = false
		case '-', '(', ')':
			return "", false
		case '%':
			i++
			if i >= len(p) {
				return "", false
			}
			if cls, ok := luaClasses[p[i]]; ok {
				b.WriteString(cls[0])
			} else if isASCIIAlnum(p[i]) {
				return "", false
			} else {
				b.WriteByte('\\')
				b.WriteByte(p[i])
			}
			canRepeat = true
		case '[':
			set, end, ok := luaSetToRE2(p, i)
			if !ok {
				return "", false
			}
			b.WriteString(set)
			i = end
			canRepeat = true
		case '|', '{', '}', '\\', ']':
			b.WriteByte('\\')
			b.WriteByte(c)
			canRepeat = true
		default:
			b.WriteByte(c)
			canRepeat = true
		}
	}
	return b.String(), true
}

func isASCIIAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// luaSetToRE2 maps the Lua set that opens at p[i] == '[' and returns the RE2
// set with the index of its closing ']'.
func luaSetToRE2(p string, i int) (string, int, bool) {
	var b strings.Builder
	b.WriteByte('[')
	j := i + 1
	if j < len(p) && p[j] == '^' {
		b.WriteByte('^')
		j++
	}
	esc := func(c byte) {
		if strings.IndexByte(`\]^-[`, c) >= 0 {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	for first := true; j < len(p); first = false {
		c := p[j]
		switch {
		case c == ']' && !first:
			b.WriteByte(']')
			return b.String(), j, true
		case c == '%':
			j++
			if j >= len(p) {
				return "", 0, false
			}
			if cls, ok := luaClasses[p[j]]; ok {
				b.WriteString(cls[1])
			} else if isASCIIAlnum(p[j]) {
				return "", 0, false
			} else {
				b.WriteByte('\\')
				b.WriteByte(p[j])
			}
			j++
		case j+2 < len(p) && p[j+1] == '-' && p[j+2] != ']':
			lo, hi := c, p[j+2]
			if hi == '%' || hi < lo {
				return "", 0, false
			}
			esc(lo)
			b.WriteByte('-')
			esc(hi)
			j += 3
		default:
			esc(c)
			j++
		}
	}
	return "", 0, false
}
