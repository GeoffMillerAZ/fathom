package langbench

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// OracleDef is one definition an oracle reports.
type OracleDef struct {
	Path   string
	Name   string
	Line   int
	End    int // 0 when the oracle gives no end
	Kind   string
	Scoped bool // the definition is a member of a type (ctags scope other than a package/namespace/module)
	// Generated marks a definition the producer found inside a macro's
	// expansion — none is written at its position (rust-analyzer). It is not
	// scored: a syntactic extractor cannot see it, and the reading counts it.
	Generated bool
}

// CtagsArgs is the tier-B invocation (§1.3): JSON output with line, kind
// name, scope and end, anonymous definitions dropped. The file list is
// passed with -L so the oracle reads exactly fathom's files (§1.4).
var CtagsArgs = []string{"--output-format=json", "--fields=+nKzSe", "--extras=-{anonymous}", "-f", "-"}

// ParseCtagsJSON reads universal-ctags JSON lines (pseudo-tags skipped).
// A malformed line is an error: a truncated oracle is never scored.
func ParseCtagsJSON(r io.Reader) ([]OracleDef, error) {
	var out []OracleDef
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var tag struct {
			Type      string `json:"_type"`
			Name      string `json:"name"`
			Path      string `json:"path"`
			Line      int    `json:"line"`
			Kind      string `json:"kind"`
			End       int    `json:"end"`
			Scope     string `json:"scope"`
			ScopeKind string `json:"scopeKind"`
		}
		if err := json.Unmarshal([]byte(line), &tag); err != nil {
			return nil, fmt.Errorf("ctags output line %d: %w", n, err)
		}
		if tag.Type != "tag" {
			continue
		}
		out = append(out, OracleDef{Path: tag.Path, Name: tag.Name, Line: tag.Line, End: tag.End, Kind: tag.Kind, Scoped: memberScope(tag.Scope, tag.ScopeKind)})
	}
	return out, sc.Err()
}

// memberScope reports whether a ctags scope makes the tag a member of a
// type: a package, namespace, module or file scope does not (a Go func's
// scope is its package).
func memberScope(scope, kind string) bool {
	if scope == "" {
		return false
	}
	switch kind {
	case "package", "packageName", "namespace", "module", "file":
		return false
	}
	return true
}

// ctagsTag is one tag a ctags parser writes for a piece of source: its name
// and its line as an offset (0-based) from the start of that source.
type ctagsTag struct {
	name string
	line int
}

// ctagsRenditions are the ctags parsers whose tags for one definition are a
// rendition of the source rather than the definition's name, keyed by the
// dialect's lang tag (K19, LB-F423). ScoreL1 compares such a dialect's
// definition with ctags in that rendition; no other language is touched.
var ctagsRenditions = map[string]func(string) []ctagsTag{
	"css": cssCtagsRendition,
}

// cssCtagsRendition is the tags universal-ctags 6.2.1's CSS parser
// (parsers/css.c at v6.2.1, 9463f1c64a4bec4b5b747440183ad44d04c4abe0) writes
// for one selector's text. The parser does not parse selectors: it reads
// tokens (a run of letters, digits and _-+>~|().:*#, a quoted string, or one
// other byte; whitespace and comments dropped) and writes one tag per run of
// selector tokens, joined by one space, each run taking an attribute selector
// that follows a token glued to it ("a[x]", "a[x] ::before"). So a comma ends
// a tag wherever it stands, also between the arguments of a functional
// pseudo-class (Selectors Level 4 :not(), :is(), :where(), :has() take a
// selector list), which makes one selector several tags; an attribute
// selector that starts a selector is not glued to anything and leaves only
// its bare words ("[hidden]" is the tag "hidden"); a byte outside ASCII ends
// a token. A tag's line is its last selector token's.
func cssCtagsRendition(sel string) []ctagsTag {
	type tok struct {
		kind byte // 's' a selector run, '"' a string, else the byte itself
		text string
		line int
	}
	isSel := func(c byte) bool {
		return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			strings.IndexByte("_-+>~|().:*#", c) >= 0
	}
	var toks []tok
	line := 0
	for i := 0; i < len(sel); {
		c := sel[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\v' || c == '\f' || c == '\r':
			i++
		case c == '/' && i+1 < len(sel) && sel[i+1] == '*':
			end := strings.Index(sel[i+2:], "*/")
			j := len(sel)
			if end >= 0 {
				j = i + 2 + end + 2
			}
			line += strings.Count(sel[i:j], "\n")
			i = j
		case c == '"' || c == '\'':
			// css.c drops a backslash and takes the byte after it, which ends
			// the string when it is the delimiter itself.
			var b strings.Builder
			b.WriteByte(c)
			j, closed := i+1, false
			for j < len(sel) {
				d := sel[j]
				j++
				if d == '\\' {
					if j == len(sel) {
						break
					}
					d = sel[j]
					j++
				}
				if d == '\n' {
					line++
				}
				if d == c {
					closed = true
					break
				}
				b.WriteByte(d)
			}
			if closed {
				b.WriteByte(c)
			}
			toks = append(toks, tok{'"', b.String(), line})
			i = j
		case isSel(c):
			j := i
			for j < len(sel) && isSel(sel[j]) {
				j++
			}
			toks = append(toks, tok{'s', sel[i:j], line})
			i = j
		default:
			toks = append(toks, tok{c, string(c), line})
			i++
		}
	}
	var out []ctagsTag
	for k := 0; k < len(toks); {
		if toks[k].kind != 's' {
			k++
			continue
		}
		var b strings.Builder
		at := 0
		for k < len(toks) && toks[k].kind == 's' {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(toks[k].text)
			at = toks[k].line
			k++
			if k < len(toks) && toks[k].kind == '[' {
				for depth := 1; depth > 0 && k < len(toks); {
					b.WriteString(toks[k].text)
					k++
					if k < len(toks) {
						switch toks[k].kind {
						case '[':
							depth++
						case ']':
							depth--
						}
					}
				}
				if k < len(toks) {
					b.WriteString(toks[k].text)
					k++
				}
			}
		}
		out = append(out, ctagsTag{b.String(), at})
	}
	return out
}
