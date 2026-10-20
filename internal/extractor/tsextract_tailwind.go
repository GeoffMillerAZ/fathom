package extractor

import "strings"

// tsextract_tailwind.go — the css dialect's Tailwind CSS v4 variant's
// preprocess key "tailwind-at-rules" (LB-Q-tailwind). tree-sitter-css reads a
// v4 stylesheet nearly whole, but some v4 spellings are outside its grammar, and
// the first of them drags the statements after it into an ERROR node: a
// parenthesised selector after `@custom-variant name` (the shorthand
// definition), a `*` in `@utility name-*`, a `--ns-*: initial;` namespace
// reset, `@source`, `@plugin`, `@config` or `@reference` with a string argument,
// the options after an `@import "x"` (`theme(static)`, `layer(base)`,
// `source(none)`), the options after `@theme` (`default inline reference`), the
// importance flag and arbitrary values of an `@apply` candidate, the range
// syntax of a media query, an unquoted `url(@scope/file)`, and the descendant
// combinator before a nesting `&`. The rewrite replaces each with spaces (or `_`,
// `:`, `>` where the grammar needs a token there), so every other byte keeps its
// offset and its line, and parses the result. It adds nothing and moves
// nothing, and the extractor cuts every name from the file as written
// (tsDialectRules.preprocessSameLength): the descendant combinator of
// `.parent &`, which the parse reads as `>`, still reads as a space, and
// `@utility tab-*` reads `tab-` because the symbol ends where the star starts.

// tailwindTexts returns the file with the unparsable v4 spellings blanked, or
// nothing when it holds none.
func tailwindTexts(src []byte) [][]byte {
	out := append([]byte(nil), src...)
	changed := false
	fill := func(from, to int, c byte) {
		for i := from; i < to && i < len(out); i++ {
			if out[i] != '\n' && out[i] != c {
				out[i] = c
				changed = true
			}
		}
	}
	blank := func(from, to int) { fill(from, to, ' ') }
	n := len(src)
	// prevSignificant is the last non-space byte outside a comment or string
	// before i, 0 at the start: a declaration starts after `{`, `;` or `}`.
	var prev byte
	for i := 0; i < n; {
		c := src[i]
		switch {
		case c == '/' && i+1 < n && src[i+1] == '*':
			i = tailwindSkipComment(src, i)
		case c == '"' || c == '\'':
			i = tailwindSkipString(src, i)
			prev = c
		case c == '@':
			j := i + 1
			for j < n && tailwindIdent(src[j]) {
				j++
			}
			name := string(src[i+1 : j])
			i = tailwindAtRule(src, name, j, blank, fill)
			prev = ';'
			if name == "utility" {
				prev = '{'
			}
		case c == 'u' && i+4 < n && src[i+1] == 'r' && src[i+2] == 'l' && src[i+3] == '(' && src[i+4] != '"' && src[i+4] != '\'' && (i == 0 || !tailwindIdent(src[i-1])):
			// An unquoted url(@scope/pkg/file.woff2): the `@` reads as an at-keyword.
			k := i + 4
			for k < n && src[k] != ')' && src[k] != '\n' {
				if src[k] == '@' {
					fill(k, k+1, '_')
				}
				k++
			}
			i = k
			prev = ')'
		case c == '&' && i > 0 && (src[i-1] == ' ' || src[i-1] == '\t' || src[i-1] == '\n') && prev != 0 && !strings.ContainsRune(">+~,({;}", rune(prev)):
			// `.parent &` (a descendant combinator before the nesting selector):
			// the grammar drops the combinator and the `&` into an ERROR node.
			// The combinator reads as `>` for the parse; names are cut from
			// the file as written (tailwindSameLength).
			for k := i - 1; k >= 0 && (src[k] == ' ' || src[k] == '\t' || src[k] == '\n'); k-- {
				if src[k] != '\n' {
					fill(k, k+1, '>')
					break
				}
			}
			prev = '&'
			i++
		case c == '-' && i+1 < n && src[i+1] == '-' && (prev == 0 || prev == '{' || prev == ';' || prev == '}'):
			j := i + 2
			star := false
			for j < n && (tailwindIdent(src[j]) || src[j] == '*') {
				star = star || src[j] == '*'
				j++
			}
			k := j
			for k < n && (src[k] == ' ' || src[k] == '\t') {
				k++
			}
			if star && k < n && src[k] == ':' {
				end := k
				for end < n && src[end] != ';' && src[end] != '}' && src[end] != '\n' {
					end++
				}
				if end < n && src[end] == ';' {
					end++ // a lone `;` opening a block is no statement to the grammar
				}
				blank(i, end)
				i = end
			} else {
				i = j
			}
			prev = '-'
		default:
			if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
				prev = c
			}
			i++
		}
	}
	if !changed {
		return nil
	}
	return [][]byte{out}
}

// tailwindAtRule handles the at-rule whose name ends at j and returns where the
// scan resumes.
func tailwindAtRule(src []byte, name string, j int, blank func(from, to int), fill func(from, to int, c byte)) int {
	n := len(src)
	skipSpace := func(k int) int {
		for k < n && (src[k] == ' ' || src[k] == '\t' || src[k] == '\n' || src[k] == '\r') {
			k++
		}
		return k
	}
	switch name {
	case "custom-variant", "variant":
		k := skipSpace(j)
		for k < n && !tailwindSpace(src[k]) && src[k] != '(' && src[k] != '{' && src[k] != ';' {
			k++
		}
		k = skipSpace(k)
		if k < n && src[k] == '(' {
			if end := tailwindBalanced(src, k); end > k {
				blank(k, end)
				return end
			}
		}
		return j
	case "utility":
		k := j
		for k < n && src[k] != '{' && src[k] != ';' && src[k] != '\n' {
			if src[k] == '*' {
				blank(k, k+1)
			}
			k++
		}
		return k
	case "apply":
		// A candidate's arbitrary value or variant (`h-[calc(100%-1rem)]`,
		// `has-[:checked]:p-2`, `[&_img]:size-4`, `bg-(--brand)`), a child
		// variant (`*:p-2`, `**:p-2`) and the importance flag (`p-4!`) are outside
		// the grammar's value syntax: each reads as the letter `x`, a candidate
		// keeping its length and the name before the first of them.
		depth := 0
		k := j
		for k < n && (depth > 0 || (src[k] != ';' && src[k] != '}' && src[k] != '{')) {
			switch src[k] {
			case '[', '(':
				depth++
				fill(k, k+1, 'x')
			case ']', ')':
				fill(k, k+1, 'x')
				depth--
			case '!', '*':
				fill(k, k+1, 'x')
			default:
				if depth > 0 {
					fill(k, k+1, 'x')
				}
			}
			k++
		}
		if k < n && src[k] == '}' {
			// The last statement of a block may omit its `;` (CSS allows it, the
			// grammar inserts a MISSING one): the blank before `}` reads as `;`.
			for m := k - 1; m >= 0 && (src[m] == ' ' || src[m] == '\t' || src[m] == '\n'); m-- {
				if src[m] != '\n' {
					fill(m, m+1, ';')
					break
				}
			}
		}
		return k
	case "theme":
		// `@theme default inline reference {`: the options are no definition.
		k := j
		for k < n && src[k] != '{' && src[k] != ';' && src[k] != '}' {
			k++
		}
		blank(j, k)
		return k
	case "media", "container":
		// Media Queries 4 range syntax, `(width >= 48rem)`, reads as `(width : 48rem)`.
		depth := 0
		k := j
		for k < n && src[k] != '{' && src[k] != ';' && src[k] != '}' {
			switch c := src[k]; {
			case c == '(':
				depth++
			case c == ')':
				depth--
			case (c == '>' || c == '<') && depth > 0:
				if k+1 < n && src[k+1] == '=' {
					fill(k, k+1, ':')
					fill(k+1, k+2, ' ')
					k++
				} else {
					fill(k, k+1, ':')
				}
			}
			k++
		}
		return k
	case "source", "plugin", "config", "reference":
		k := j
		for k < n && src[k] != ';' && src[k] != '{' {
			k++
		}
		blank(j, k)
		return k
	case "import":
		k := skipSpace(j)
		if k < n && (src[k] == '"' || src[k] == '\'') {
			k = tailwindSkipString(src, k)
			end := k
			for end < n && src[end] != ';' {
				end++
			}
			blank(k, end)
			return end
		}
		return j
	}
	return j
}

func tailwindIdent(c byte) bool {
	return c == '-' || c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func tailwindSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// tailwindSkipComment returns the offset just past the comment starting at i.
func tailwindSkipComment(src []byte, i int) int {
	for j := i + 2; j+1 < len(src); j++ {
		if src[j] == '*' && src[j+1] == '/' {
			return j + 2
		}
	}
	return len(src)
}

// tailwindSkipString returns the offset just past the string starting at i.
func tailwindSkipString(src []byte, i int) int {
	q := src[i]
	for j := i + 1; j < len(src); j++ {
		switch src[j] {
		case '\\':
			j++
		case q:
			return j + 1
		case '\n':
			return j
		}
	}
	return len(src)
}

// tailwindBalanced returns the offset just past the parenthesis matching the
// one at i (strings and comments skipped), or i when it never closes.
func tailwindBalanced(src []byte, i int) int {
	depth := 0
	for j := i; j < len(src); j++ {
		switch src[j] {
		case '"', '\'':
			j = tailwindSkipString(src, j) - 1
		case '/':
			if j+1 < len(src) && src[j+1] == '*' {
				j = tailwindSkipComment(src, j) - 1
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return j + 1
			}
		}
	}
	return i
}

// tailwindCandidateRoot is the utility a Tailwind candidate names: the text after
// its last top-level `:` (the variants before it, `hover:`, `md:`, `has-[:x]:`,
// are not part of the utility), with the importance flag (`!` before or after)
// removed. A name with neither is returned as it is: a `var()` name, an
// animation name, a plain utility.
func tailwindCandidateRoot(s string) string {
	if !strings.ContainsAny(s, ":!") {
		return s
	}
	depth, cut := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '[', '(':
			depth++
		case ']', ')':
			depth--
		case ':':
			if depth == 0 {
				cut = i + 1
			}
		}
	}
	r := strings.TrimSuffix(strings.TrimPrefix(s[cut:], "!"), "!")
	if r == "" {
		return s
	}
	return r
}
