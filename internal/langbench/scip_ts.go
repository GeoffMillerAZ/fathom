package langbench

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// CntCallsOracleBlind counts fathom's pairs the truth lacks whose every
// written call site has no occurrence of the index at its callee's range
// (SCIPOptions.OracleBlindSites, LB-F175): calls the oracle cannot judge,
// left out of both counts (design §1.4 amendment 2026-10-02).
const CntCallsOracleBlind = "calls_oracle_blind"

// tsIdentRE is a JavaScript identifier as a call names it ($ is a letter).
var tsIdentRE = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

func isTSIdentByte(c byte) bool { return c == '$' || isIdentByte(c) }

// tsDefName is a scip-typescript definition's leaf in the name the language
// gives it, SCIPDefs' rule for L1: `<constructor>` is "constructor", an
// accessor `<get>x` or `<set>x` is x.
func tsDefName(name string) string {
	switch {
	case name == "<constructor>":
		return "constructor"
	case strings.HasPrefix(name, "<get>"), strings.HasPrefix(name, "<set>"):
		return name[len("<get>"):]
	}
	return name
}

// tsCalleeName is a callable reference's callee: a constructor's is the
// class written at the reference (`new Foo(` references Foo#`<constructor>`
// at "Foo"), an accessor's its name; "<constructor>" stays when what is
// written there is no identifier (and so is never a written call).
func tsCalleeName(callee string, lines [][]byte, r [4]int32) string {
	switch {
	case callee == "<constructor>":
		if r[0] == r[2] && int(r[0]) < len(lines) && r[1] >= 0 && r[1] <= r[3] && int(r[3]) <= len(lines[r[0]]) {
			if w := string(lines[r[0]][r[1]:r[3]]); tsIdentRE.MatchString(w) {
				return w
			}
		}
	case strings.HasPrefix(callee, "<get>"), strings.HasPrefix(callee, "<set>"):
		return callee[len("<get>"):]
	}
	return callee
}

// tsCallAt reports whether the source at range r is the callee's own name
// written as a call: followed on its line by `(`, after an optional non-null
// `!`, type arguments and optional-call `?.`, or by a template literal (a
// tagged template is a call).
func tsCallAt(lines [][]byte, r [4]int32, callee string) bool {
	if r[0] != r[2] || r[1] < 0 || int(r[0]) >= len(lines) || int(r[3]) > len(lines[r[0]]) || r[1] > r[3] {
		return false
	}
	line := lines[r[0]]
	if string(line[r[1]:r[3]]) != callee {
		return false
	}
	rest := bytes.TrimLeft(line[r[3]:], " \t")
	if len(rest) > 0 && rest[0] == '!' && (len(rest) == 1 || rest[1] != '=') {
		rest = bytes.TrimLeft(rest[1:], " \t")
	}
	if len(rest) > 0 && rest[0] == '<' {
		depth, closed := 0, false
		for i, c := range rest {
			switch {
			case c == '<':
				depth++
			case c == '>' && (i == 0 || rest[i-1] != '='):
				depth--
			}
			if depth == 0 {
				rest, closed = bytes.TrimLeft(rest[i+1:], " \t"), true
				break
			}
		}
		if !closed {
			return false
		}
	}
	rest = bytes.TrimPrefix(rest, []byte("?."))
	return len(rest) > 0 && (rest[0] == '(' || rest[0] == '`')
}

// utf16Range is r with its columns, UTF-16 code units (scip-typescript's
// position encoding), turned into byte offsets of the source's lines.
func utf16Range(lines [][]byte, r [4]int32) [4]int32 {
	if int(r[0]) < len(lines) && r[0] >= 0 {
		r[1] = utf16Col(lines[r[0]], r[1])
	}
	if int(r[2]) < len(lines) && r[2] >= 0 {
		r[3] = utf16Col(lines[r[2]], r[3])
	}
	return r
}

func utf16Col(line []byte, col int32) int32 {
	units, i := int32(0), 0
	for i < len(line) && units < col {
		if c := line[i]; c < utf8.RuneSelf {
			units++
			i++
			continue
		}
		cr, size := utf8.DecodeRune(line[i:])
		if cr >= 0x10000 {
			units += 2
		} else {
			units++
		}
		i += size
	}
	return int32(i) + (col - units)
}

// oracleSites are the (line, start byte) positions, per document, at which
// the index writes any occurrence (OracleBlindSites).
type oracleSites map[string]map[[2]int32]bool

func (s oracleSites) mark(doc string, r [4]int32) {
	if s[doc] == nil {
		s[doc] = map[[2]int32]bool{}
	}
	s[doc][[2]int32{r[0], r[1]}] = true
}

// drop removes from found each pair the truth does not hold whose every
// written call site (tsWrittenCallSites inside the caller's lines; with jsx, in
// a JSX variant, also each element whose tag names the callee, ruling M2) has
// no occurrence at the callee's range, counting each (CntCallsOracleBlind).
func (s oracleSites) drop(found, truth map[CallPair]bool, nodes []core.SymbolNode, srcOf func(string) [][]byte, jsx bool, counts map[string]int) {
	spans := map[[2]string][][2]int{}
	for _, n := range nodes {
		hi := max(n.EndLine, n.Line)
		k := [2]string{n.FilePath, BareName(n.Name)}
		spans[k] = append(spans[k], [2]int{n.Line, hi})
	}
	for p := range found {
		if truth[p] {
			continue
		}
		lines := srcOf(p.Path)
		n, judged := 0, false
		for _, sp := range spans[[2]string{p.Path, p.Caller}] {
			sites := tsWrittenCallSites(lines, sp[0], sp[1], p.Callee)
			if jsx && tsJSXVariant(p.Path) {
				sites = append(sites, tsJSXElementSites(lines, sp[0], sp[1], p.Callee)...) // ruling M2
			}
			for _, at := range sites {
				n++
				if s[p.Path][[2]int32{int32(at[0] - 1), int32(at[1])}] {
					judged = true
				}
			}
		}
		if n > 0 && !judged {
			delete(found, p)
			counts[CntCallsOracleBlind]++
		}
	}
}

// tsWrittenCallSites are the (1-based line, 0-based byte column) positions in
// lines lo..hi where callee is written as a call (tsCallAt).
func tsWrittenCallSites(lines [][]byte, lo, hi int, callee string) [][2]int {
	if !tsIdentRE.MatchString(callee) {
		return nil
	}
	var out [][2]int
	for ln := max(lo, 1); ln <= hi && ln <= len(lines); ln++ {
		line := lines[ln-1]
		for from := 0; from < len(line); {
			i := bytes.Index(line[from:], []byte(callee))
			if i < 0 {
				break
			}
			i += from
			end := i + len(callee)
			from = end
			if (i > 0 && isTSIdentByte(line[i-1])) || (end < len(line) && isTSIdentByte(line[end])) {
				continue
			}
			if tsCallAt(lines, [4]int32{int32(ln - 1), int32(i), int32(ln - 1), int32(end)}, callee) {
				out = append(out, [2]int{ln, i})
			}
		}
	}
	return out
}

// CntScipDefsObjectLiteral counts scip-typescript's object-literal property
// definitions (LB-F413, owner ruling D2, K19, O747): a property assignment or
// shorthand property, in any expression, is a global symbol of its file whose
// last descriptor is the meta `<name><n>:`, n a counter per name and file
// (scip-typescript's FileIndexer: ts.isPropertyAssignment and
// ts.isShorthandPropertyAssignment). No extractor names such a definition,
// so def_mapping leaves it out of its denominator, counted, as LB-F239
// leaves parameters; typed_recall's references to it stay.
const CntScipDefsObjectLiteral = "scip_defs_object_literal"

// tsObjectLiteralProperty reports whether sym is a scip-typescript object-literal
// property: its last descriptor is the meta `<name><n>:` (LB-F413).
func tsObjectLiteralProperty(sym string) bool {
	return strings.HasPrefix(sym, "scip-typescript ") && strings.HasSuffix(sym, ":")
}

// tsObjectLiteralNote is the reading's disclosure of D2's object-literal rule for a
// subject, "" when the rule moved nothing.
func tsObjectLiteralNote(name string, counts map[string]int) string {
	n := counts[CntScipDefsObjectLiteral]
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%s: %d object-literal properties scip-typescript defines (a property assignment or shorthand property in any expression, named `<name><n>:` per file: no definition an extractor names) are out of def_mapping's denominator (owner ruling D2, K19, LB-F413: %s, as LB-F239 leaves parameters); typed_recall's references to them stay", name, n, CntScipDefsObjectLiteral)
}

// CntScipDefsTypeLiteral counts scip-typescript's anonymous type-literal member
// definitions (owner ruling S3, K19, wave lf7, LB-F511; SCIPOptions.TypeLiteralMembers):
// scip-typescript's descriptor grammar gives every type literal (`{ x?: T }`
// written as a type) the meta descriptor `typeLiteral<n>:`, n its file's local
// counter, under the symbol of the node that holds it (FileIndexer.descriptor:
// ts.isTypeLiteralNode), so each member is a global symbol of its file named
// through an anonymous, counter-numbered container (`Props:typeLiteral1:disabled.`
// for `Props<..., { disabled?: boolean }>`, `ButtonFn().(`{ value }`)typeLiteral7:value.`
// for a destructured parameter's annotation). TypeScript itself names no such
// type (its checker's symbol for a type literal is the anonymous `__type`), and
// nothing but the literal declares the member: D2's object-literal principle on
// the type side, structural and no named declaration, so def_mapping leaves it
// out of its denominator, counted, and typed_recall's references to it stay. A
// type alias's own body (`type Own = { open: boolean }`: `Own#typeLiteral0:open.`,
// the literal directly under the alias's type descriptor) is the declared body
// of a named type, written interchangeably with `interface Own { … }`, and the
// dialect names its members (LB-F377's field pattern, `Own.open`), so it stays.
const CntScipDefsTypeLiteral = "scip_defs_type_literal"

// CntScipDefsTypeLiteralNamed counts, of CntScipDefsTypeLiteral, the members a
// fathom definition maps (a method signature, which the dialect names in any
// object type): out of def_mapping's numerator and denominator alike, disclosed.
const CntScipDefsTypeLiteralNamed = "scip_defs_type_literal_named"

// tsAnonymousTypeLiteralMember reports whether sym is a scip-typescript member
// of an anonymous type literal (owner ruling S3, LB-F511): its innermost
// `typeLiteral<n>:` meta descriptor has a member descriptor after it and does
// not stand directly under a type descriptor (`Alias#typeLiteral<n>:`, a type
// alias's own body: a type literal's parent is a type descriptor only there,
// since a class, an interface or an enum holds none of its own). A symbol the
// grammar cannot read is none.
func tsAnonymousTypeLiteralMember(sym string) bool {
	if !strings.HasPrefix(sym, "scip-typescript ") {
		return false
	}
	ds, ok := scipDescriptors(sym)
	if !ok {
		return false
	}
	for i := len(ds) - 2; i >= 0; i-- {
		if ds[i].suffix == ':' && tsTypeLiteralRE.MatchString(ds[i].name) {
			return i == 0 || ds[i-1].suffix != '#'
		}
	}
	return false
}

var tsTypeLiteralRE = regexp.MustCompile(`^typeLiteral[0-9]+$`)

// scipDescriptor is one descriptor of a SCIP symbol: its name and its suffix
// ('/' namespace, '#' type, '.' term, ':' meta, '!' macro, 'm' method, '['
// type parameter, '(' parameter).
type scipDescriptor struct {
	name   string
	suffix byte
}

// scipDescriptors reads a global SCIP symbol's descriptors by SCIP's grammar:
// past the scheme, manager, package name and version (a double space escapes a
// space), each descriptor a simple identifier or a backtick-escaped one (a
// double backtick escapes a backtick) and its suffix. ok is false for a local
// symbol or text the grammar cannot read.
func scipDescriptors(sym string) (out []scipDescriptor, ok bool) {
	i := 0
	for field := 0; field < 4; field++ {
		for {
			if i >= len(sym) {
				return nil, false
			}
			if sym[i] == ' ' {
				if i+1 < len(sym) && sym[i+1] == ' ' {
					i += 2
					continue
				}
				i++
				break
			}
			i++
		}
	}
	name := func() (string, bool) {
		if i < len(sym) && sym[i] == '`' {
			var b strings.Builder
			for i++; i < len(sym); i++ {
				if sym[i] == '`' {
					if i+1 < len(sym) && sym[i+1] == '`' {
						b.WriteByte('`')
						i++
						continue
					}
					i++
					return b.String(), true
				}
				b.WriteByte(sym[i])
			}
			return "", false
		}
		start := i
		for i < len(sym) && (isTSIdentByte(sym[i]) || sym[i] == '+' || sym[i] == '-') {
			i++
		}
		return sym[start:i], i > start
	}
	for i < len(sym) {
		switch sym[i] {
		case '[', '(':
			open, close := sym[i], byte(']')
			if open == '(' {
				close = ')'
			}
			i++
			n, ok := name()
			if !ok || i >= len(sym) || sym[i] != close {
				return nil, false
			}
			i++
			out = append(out, scipDescriptor{n, open})
			continue
		}
		n, ok := name()
		if !ok || i >= len(sym) {
			return nil, false
		}
		switch c := sym[i]; c {
		case '/', '#', '.', ':', '!':
			i++
			out = append(out, scipDescriptor{n, c})
		case '(':
			j := strings.IndexByte(sym[i:], ')')
			if j < 0 || i+j+1 >= len(sym) || sym[i+j+1] != '.' {
				return nil, false
			}
			i += j + 2
			out = append(out, scipDescriptor{n, 'm'})
		default:
			return nil, false
		}
	}
	return out, len(out) > 0
}

// tsTypeLiteralNote is the reading's disclosure of ruling S3's type-literal rule
// for a subject, "" when the rule moved nothing.
func tsTypeLiteralNote(name string, counts map[string]int) string {
	n := counts[CntScipDefsTypeLiteral]
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%s: %d members of an anonymous type literal scip-typescript defines (a `{ x?: T }` written as a type argument's, a parameter's, a variable's or a member's type, named under a counter-numbered `typeLiteral<n>:` descriptor: structural, no named declaration) are out of def_mapping's denominator (owner ruling S3, K19, LB-F511: %s, D2's object-literal principle on the type side), %d of them a definition the dialect names as well (%s: a method signature), out of the numerator alike; a type alias's own members stay, and typed_recall's references to them stay", name, n, CntScipDefsTypeLiteral, counts[CntScipDefsTypeLiteralNamed], CntScipDefsTypeLiteralNamed)
}

// scipDefOutOfScope reports whether a definition in document doc is outside
// the documents the reading measures under owner ruling L5 (K19, LB-F467):
// def_mapping leaves it out of its denominator, counted
// (scip_defs_out_of_scope), as ClangScope does for C and C++ (LB-F292). A
// TypeScript declaration file beside JavaScript (eslint's lib/types/*.d.ts,
// O788), a file -only leaves out or one ingest's gates skip is a document of
// the index the dialect is never given, so no extractor of the measured files
// could name its definitions.
func scipDefOutOfScope(opts SCIPOptions, doc string) bool {
	return opts.DefScoped && opts.DefScope != nil && !opts.DefScope[doc]
}

// DefScopeNote is the reading's disclosure of ruling L5 for a subject, "" when
// the rule moved nothing.
func DefScopeNote(name string, counts map[string]int) string {
	n := counts[CntScipDefsOutOfScope]
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%s: %d definitions the index writes in documents outside the files the reading measures (a TypeScript declaration file beside JavaScript, a file -only leaves out, one ingest's gates skip) are out of def_mapping's denominator (owner ruling L5, K19, LB-F467: %s, the out-of-scope principle of LB-F292 and §1.4); typed_recall's references in and to them stay", name, n, CntScipDefsOutOfScope)
}

// CntScipJSXElementCalls counts the references scip-typescript writes at a JSX
// element's tag that the call truth reads as a call (SCIPOptions.JSXElementCalls,
// owner ruling M2, O860, K19).
const CntScipJSXElementCalls = "scip_jsx_element_calls"

// tsJSXVariant reports whether TypeScript parses doc with JSX: its language
// variant is JSX for a .tsx, .jsx, .js, .mjs or .cjs file and standard for .ts,
// .mts and .cts (TypeScript's getLanguageVariant), where `<Foo>x` is a type
// assertion and no element can be written.
func tsJSXVariant(doc string) bool {
	switch strings.ToLower(path.Ext(doc)) {
	case ".tsx", ".jsx", ".js", ".mjs", ".cjs":
		return true
	}
	return false
}

// tsJSXElementAt reports whether the identifier at range r is the tag of a JSX
// opening or self-closing element naming a component, and returns it (owner
// ruling M2). `<Foo .../>` compiles to jsx(Foo, props) and the renderer calls
// Foo, so the tag is the call site; a member tag `<Foo.Bar>` calls its last
// segment (the receivers are reads, as `foo.bar()` calls bar), which is how the
// dialects record a member tag (LB-F377). TypeScript's rule: an identifier tag
// starting a to z, or holding `-` or `:`, is an intrinsic element (a string,
// never a call). A closing tag `</Foo>` is the element already counted at its
// opening; a fragment `<>` names nothing. What is written decides: the tag
// follows a `<` that opens an expression or a child (never one an identifier,
// `)` or `]` stands against, which is type arguments, `useState<Foo>(`, or a
// comparison), and is followed by its own type arguments, then `>`, `/>`, or
// a space and an attribute, a spread or the end of the line (a comparison
// `i < max;` is followed by none of them).
func tsJSXElementAt(lines [][]byte, r [4]int32) (string, bool) {
	if r[0] != r[2] || r[0] < 0 || int(r[0]) >= len(lines) || r[1] < 0 || r[1] >= r[3] || int(r[3]) > len(lines[r[0]]) {
		return "", false
	}
	line := lines[r[0]]
	name := string(line[r[1]:r[3]])
	if !tsIdentRE.MatchString(name) {
		return "", false
	}
	start, member := int(r[1]), false
	for start > 0 && line[start-1] == '.' {
		j := start - 1
		for j > 0 && isTSIdentByte(line[j-1]) {
			j--
		}
		if j == start-1 {
			return "", false
		}
		start, member = j, true
	}
	lt := start
	for lt > 0 && (line[lt-1] == ' ' || line[lt-1] == '\t') {
		lt--
	}
	if lt == 0 || line[lt-1] != '<' {
		return "", false // `</Foo`: a closing tag; `:rect`, `-x`: an intrinsic name
	}
	if lt--; lt > 0 {
		if c := line[lt-1]; isTSIdentByte(c) || c == ')' || c == ']' || c == '<' || c == '.' {
			return "", false
		}
	}
	operand := tsOperandBefore(line[:lt])
	if c := name[0]; !member && c >= 'a' && c <= 'z' {
		return "", false
	}
	rest := line[r[3]:]
	if len(rest) > 0 && (rest[0] == '.' || rest[0] == '-' || rest[0] == ':') {
		return "", false
	}
	if len(rest) > 0 && rest[0] == '<' {
		depth, closed := 0, false
		for i, c := range rest {
			switch c {
			case '<':
				depth++
			case '>':
				depth--
			}
			if depth == 0 {
				rest, closed = rest[i+1:], true
				break
			}
		}
		if !closed {
			return "", false
		}
	}
	trimmed := bytes.TrimLeft(rest, " \t\r")
	spaced := len(trimmed) < len(rest)
	switch {
	case len(trimmed) == 0 && !operand, len(trimmed) > 0 && trimmed[0] == '>':
		return name, true
	case len(trimmed) > 0 && trimmed[0] == '/':
		if after := bytes.TrimLeft(trimmed[1:], " \t"); len(after) > 0 && after[0] == '>' {
			return name, true
		}
	case len(trimmed) > 0 && spaced && (trimmed[0] == '{' || trimmed[0] == '_' || trimmed[0] == '$' || isLetterByte(trimmed[0])):
		return name, true
	}
	return "", false
}

func isLetterByte(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

// tsJSXKeywords may stand before an element, so a `<` after one opens it.
var tsJSXKeywords = map[string]bool{"return": true, "yield": true, "await": true, "case": true, "default": true,
	"else": true, "do": true, "throw": true, "void": true, "typeof": true, "delete": true, "in": true, "of": true}

// tsOperandBefore reports whether the text before a `<` ends in an operand (an
// identifier other than a keyword, a literal, `)`, `]`): the `<` may then be a
// comparison, so tsJSXElementAt needs the tag's own delimiter on its line, never
// the end of it (`ok = i < items.length` in a file without semicolons); JSX text
// before an element (`Click <Link>`) reads the same way.
func tsOperandBefore(text []byte) bool {
	text = bytes.TrimRight(text, " \t")
	if len(text) == 0 {
		return false
	}
	switch c := text[len(text)-1]; {
	case c == ')' || c == ']' || c == '"' || c == '\'' || c == '`':
		return true
	case isTSIdentByte(c):
		i := len(text)
		for i > 0 && isTSIdentByte(text[i-1]) {
			i--
		}
		return !tsJSXKeywords[string(text[i:])]
	}
	return false
}

// tsJSXElementSites are the (1-based line, 0-based byte column) positions in
// lines lo..hi where callee is written as a JSX element's tag (tsJSXElementAt):
// the oracle-blind rule's written call sites in a JSX variant (ruling M2).
func tsJSXElementSites(lines [][]byte, lo, hi int, callee string) [][2]int {
	if !tsIdentRE.MatchString(callee) {
		return nil
	}
	var out [][2]int
	for ln := max(lo, 1); ln <= hi && ln <= len(lines); ln++ {
		line := lines[ln-1]
		for from := 0; from < len(line); {
			i := bytes.Index(line[from:], []byte(callee))
			if i < 0 {
				break
			}
			i += from
			end := i + len(callee)
			from = end
			if (i > 0 && isTSIdentByte(line[i-1])) || (end < len(line) && isTSIdentByte(line[end])) {
				continue
			}
			if _, ok := tsJSXElementAt(lines, [4]int32{int32(ln - 1), int32(i), int32(ln - 1), int32(end)}); ok {
				out = append(out, [2]int{ln, i})
			}
		}
	}
	return out
}

// tsJSXElementNote is the reading's disclosure of ruling M2 for a subject, ""
// when the rule moved nothing.
func tsJSXElementNote(name string, counts map[string]int) string {
	n := counts[CntScipJSXElementCalls]
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%s: %d references at a JSX element's tag naming a component (`<Foo/>`, the last segment of `<Foo.Bar>`; never an intrinsic `<div>`, a closing tag or a fragment) are counted as calls to the name written there (owner ruling M2, K19, LB-F481: %s; `<Foo/>` compiles to jsx(Foo, props) and the renderer calls Foo), and an element is a written call site for the oracle-blind rule", name, n, CntScipJSXElementCalls)
}
