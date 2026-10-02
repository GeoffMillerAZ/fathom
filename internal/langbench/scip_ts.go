package langbench

import (
	"bytes"
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
