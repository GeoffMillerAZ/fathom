package langbench

import (
	"bytes"
	"fmt"
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
// written call site (tsWrittenCallSites inside the caller's lines) has no
// occurrence at the callee's range, counting each (CntCallsOracleBlind).
func (s oracleSites) drop(found, truth map[CallPair]bool, nodes []core.SymbolNode, srcOf func(string) [][]byte, counts map[string]int) {
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
			for _, at := range tsWrittenCallSites(lines, sp[0], sp[1], p.Callee) {
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
