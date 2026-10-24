package extractor

import "regexp"

// jinja2's pre-parse step (tsPreprocessors["jinja-templates"], LB-Q-jinja2).
// cathaysia/tree-sitter-jinja reads two things Jinja itself does not:
//   - a text line that opens with `# ` is a line statement (`# if x`), which
//     Jinja has off unless the environment sets a prefix; a shell or Dockerfile
//     comment in the surrounding text is an ERROR;
//   - `in` is an operator only in a for loop's header, so `a in b` and
//     `a not in b` anywhere else in an expression are an ERROR.
//
// A template that does not parse as written is parsed as the text this gives
// instead: each such `#` is a space, and each such operator is `==` / `!=`,
// padded to its own length. A byte keeps its offset, so lines are the source's;
// names, imports and calls are never inside what is rewritten. The rewrite is
// taken only when it parses with no error (tsPreprocessed); one that parses as
// written is never rewritten.

var jinjaLineStatement = regexp.MustCompile(`(?m)^[ \t]*(#)[ \t]`)

// jinjaTexts is the one text a template that does not parse as written is
// parsed as instead; nil when nothing changes.
func jinjaTexts(src []byte) [][]byte {
	out := append([]byte(nil), src...)
	changed := false
	for _, m := range jinjaLineStatement.FindAllSubmatchIndex(out, -1) {
		out[m[2]] = ' '
		changed = true
	}
	for i := 0; i+1 < len(out); i++ {
		if out[i] != '{' || (out[i+1] != '{' && out[i+1] != '%') {
			continue
		}
		end := jinjaTagEnd(out, i)
		if out[i+1] == '%' && jinjaBlankIn(out, i+2, end, jinjaIsFor(out, i+2, end)) {
			changed = true
		} else if out[i+1] == '{' && jinjaBlankIn(out, i+2, end, false) {
			changed = true
		}
		i = end
	}
	if !changed {
		return nil
	}
	return [][]byte{out}
}

// jinjaTagEnd is the index of the last byte of the `{{ }}` or `{% %}` tag opening at i
// (len-1 when it never closes), string literals skipped.
func jinjaTagEnd(b []byte, i int) int {
	closer := byte('}')
	if b[i+1] == '%' {
		closer = '%'
	}
	for j := i + 2; j+1 < len(b); j++ {
		switch b[j] {
		case '\'', '"':
			q := b[j]
			for j++; j < len(b) && b[j] != q; j++ {
				if b[j] == '\\' {
					j++
				}
			}
		case closer:
			if b[j+1] == '}' {
				return j + 1
			}
		}
	}
	return len(b) - 1
}

// jinjaIsFor is whether the statement tag whose body starts at i is `for`.
func jinjaIsFor(b []byte, i, end int) bool {
	for i < end && (b[i] == '-' || b[i] == '+' || b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r') {
		i++
	}
	return i+3 < end && string(b[i:i+3]) == "for" && !isIdentByte(b[i+3])
}

// jinjaBlankIn rewrites the `in` / `not in` operators of the expression text
// b[from:end] (string literals skipped); in a for header the first `in` word is
// the loop's own and stays. It reports whether it changed anything.
func jinjaBlankIn(b []byte, from, end int, forHeader bool) bool {
	changed := false
	keep := forHeader
	prevWord := [2]int{-1, -1} // the previous word, for `not in`
	for j := from; j < end; j++ {
		c := b[j]
		switch {
		case c == '\'' || c == '"':
			for j++; j < end && b[j] != c; j++ {
				if b[j] == '\\' {
					j++
				}
			}
			prevWord = [2]int{-1, -1}
		case isIdentByte(c):
			s := j
			for j < end && isIdentByte(b[j]) {
				j++
			}
			word := string(b[s:j])
			j--
			if word == "in" && !jinjaAfterDot(b, from, s) {
				if keep {
					keep = false
				} else if jinjaIsNot(b, prevWord) {
					for k := prevWord[0]; k < s; k++ {
						b[k] = ' '
					}
					b[s], b[s+1] = '!', '='
					changed = true
				} else {
					b[s], b[s+1] = '=', '='
					changed = true
				}
			}
			prevWord = [2]int{s, j + 1}
		}
	}
	return changed
}

func jinjaIsNot(b []byte, w [2]int) bool { return w[0] >= 0 && string(b[w[0]:w[1]]) == "not" }

// jinjaAfterDot is whether the word at s is an attribute (`x.in`), never an operator.
func jinjaAfterDot(b []byte, from, s int) bool { return s > from && b[s-1] == '.' }
