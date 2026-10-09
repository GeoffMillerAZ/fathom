package extractor

import "strings"

// tsextract_cppmacros.go — the C++ dialect's pre-parse step (O455, the cpp
// dialect.json's preprocess key "cpp-macros"). A C++ library writes its
// export, inline and noexcept spellings as object-like macros the compiler
// expands and tree-sitter-cpp cannot: `class LIB_API logger {` parses as a
// class named LIB_API, and `LIB_INLINE std::string name() LIB_NOEXCEPT {` as a
// function returning the type LIB_INLINE whose name runs into its return type.
// Its try and catch are macros too where exceptions may be off (`LIB_TRY {`
// and `LIB_CATCH(...) {`), and a catch parses as a function named LIB_CATCH.
//
// The step finds those macros by where they are written, never by a project's
// list of names (an all-capitals word: a capital, then capitals, digits and
// underscores, with an underscore or more than three letters), and rewrites
// them at equal byte length, so every line, column and byte offset of the
// file is unchanged:
//   - between `class`, `struct` or `union` and the name, with its argument
//     list when it takes one, when a class head follows the name (`{`, `:`,
//     `<`, `::`, final): `class LIB_API logger final : public sink {`;
//   - before a declaration: where one starts (the file's start, after `;`,
//     `{`, `}`, a template's `>`, an access label's `:`, an attribute's `]]`
//     or a keyword such as static, inline or template), followed on its line
//     by a word or `~`, unless that word is a variable the macro is the type
//     of (`HANDLE h{nullptr};`): `LIB_CONSTEXPR LIB_INLINE auto g()`;
//   - between a declaration's type and its declarator's name, when the
//     parameter list opens on that line: `std::string LIB_INLINE name(int)`;
//   - after a declarator's parameter list or array bound, before what ends
//     it (`{`, `;`, `=`, `:`, `->`, override, final, const, another macro):
//     `f(int x) LIB_NOEXCEPT {`, `names[] LIB_LEVEL_NAMES;`;
//   - alone on its line where a declaration or statement starts, with its
//     argument list when it takes one and no `;` after (a namespace's
//     opening and closing, a pragma, an attribute over the next line);
//   - between `if` and its condition (`if LIB_CONSTEXPR (x)`);
//   - a function-like macro standing as a whole template parameter, with the
//     comma that separates it (`template <typename T, LIB_ENABLE_IF(c)>`);
//   - a word ending `_TRY` before a block reads as `try` when a word ending
//     `_CATCH` with a handler follows the block (`LIB_CATCH(...) {` reads as
//     `catch`); when the catch macro is the whole handler
//     (`LIB_LOGGER_CATCH(where)`), both are blanked and the block stays a
//     block;
//   - a word naming an access specifier (PRIVATE, PROTECTED, PUBLIC) before
//     a label's `:` reads as that specifier.
//
// The conditional directive lines are then blanked too, every branch kept
// (cppBlankConditionals). A blanked byte becomes a space. Comments, string
// and character literals and other directive lines are never touched. The
// rewrite is taken only when it parses with no error (tsPreprocessed), so a
// file the step cannot mend is read as written; one that parses as written
// is never rewritten.

// cppMacroTexts is the one text a C++ file that does not parse as written is
// parsed as instead (tsPreprocessors["cpp-macros"]): its macros rewritten
// (cppBlankMacros) and its conditional directive lines blanked
// (cppBlankConditionals); nil when neither changes anything. One text, not
// one per step: over the three C++ subjects the combined text parses
// cleanly wherever the macro text alone does, and a second candidate is a
// second parse of every file neither mends (O455).
func cppMacroTexts(src []byte) [][]byte {
	toks, dirs := cppLex(src)
	text, changed := cppBlankMacros(src, toks)
	if !changed {
		text = src
	}
	if out, ok := cppBlankConditionals(text, dirs); ok {
		text, changed = out, true
	}
	if !changed {
		return nil
	}
	return [][]byte{text}
}

// cppBlankConditionals is src with every conditional directive line (#if,
// #ifdef, #ifndef, #elif, #elifdef, #elifndef, #else, #endif, with its
// continuation lines) blanked and every branch kept, so a group whose
// branches each open or close part of one construct (a template head over
// two alternative alias declarations, a branch inside an initializer list)
// reads as its branches in a row; ok is false when src has none. dirs are the
// directive spans cppLex found.
func cppBlankConditionals(src []byte, dirs [][2]int) (out []byte, ok bool) {
	for _, d := range dirs {
		i := d[0] + 1
		for i < d[1] && (src[i] == ' ' || src[i] == '\t') {
			i++
		}
		j := i
		for j < d[1] && isIdentByte(src[j]) {
			j++
		}
		switch string(src[i:j]) {
		case "if", "ifdef", "ifndef", "elif", "elifdef", "elifndef", "else", "endif":
		default:
			continue
		}
		if out == nil {
			out = append([]byte(nil), src...)
		}
		for k := d[0]; k < d[1]; k++ {
			if out[k] != '\n' && out[k] != '\r' {
				out[k] = ' '
			}
		}
	}
	return out, out != nil
}

// cppTok is one token of cppLex: an identifier or keyword (word), a
// punctuator, or a literal (string, character or number).
type cppTok struct {
	start, end int
	kind       byte // 'w' word, 'p' punctuator, 'l' literal
}

// cppLex tokenizes src as the C++ lexer would for this purpose: comments and
// whitespace are skipped, string, raw string and character literals and
// pp-numbers are one literal token each, `::` and `->` are one punctuator,
// and a preprocessor directive line (with its continuations) is no token:
// its span, from its `#` to the end of its last line, is in dirs.
func cppLex(src []byte) (toks []cppTok, dirs [][2]int) {
	n := len(src)
	lineStart := true // only whitespace since the last newline
	for i := 0; i < n; {
		c := src[i]
		switch {
		case c == '\n':
			lineStart = true
			i++
			continue
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			i++
			continue
		case c == '/' && i+1 < n && src[i+1] == '/':
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		case c == '/' && i+1 < n && src[i+1] == '*':
			i += 2
			for i+1 < n && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i = min(i+2, n)
			continue
		case c == '#' && lineStart:
			end := cppSkipDirective(src, i)
			dirs = append(dirs, [2]int{i, end})
			i = end
			continue
		}
		lineStart = false
		switch {
		case cppIdentStart(c):
			j := i + 1
			for j < n && isIdentByte(src[j]) {
				j++
			}
			if j < n && (src[j] == '"' || src[j] == '\'') && cppLiteralPrefix(string(src[i:j])) {
				end := cppSkipLiteral(src, i, j)
				toks = append(toks, cppTok{i, end, 'l'})
				i = end
				continue
			}
			toks = append(toks, cppTok{i, j, 'w'})
			i = j
		case c >= '0' && c <= '9' || c == '.' && i+1 < n && src[i+1] >= '0' && src[i+1] <= '9':
			j := i + 1
			for j < n {
				d := src[j]
				switch {
				case isIdentByte(d) || d == '.':
					j++
				case d == '\'' && j+1 < n && isIdentByte(src[j+1]): // a digit separator
					j += 2
				case (d == '+' || d == '-') && strings.ContainsRune("eEpP", rune(src[j-1])):
					j++
				default:
					goto num
				}
			}
		num:
			toks = append(toks, cppTok{i, j, 'l'})
			i = j
		case c == '"' || c == '\'':
			end := cppSkipLiteral(src, i, i)
			toks = append(toks, cppTok{i, end, 'l'})
			i = end
		case c == ':' && i+1 < n && src[i+1] == ':', c == '-' && i+1 < n && src[i+1] == '>':
			toks = append(toks, cppTok{i, i + 2, 'p'})
			i += 2
		default:
			toks = append(toks, cppTok{i, i + 1, 'p'})
			i++
		}
	}
	return toks, dirs
}

func cppIdentStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// cppLiteralPrefix reports whether p is an encoding or raw-string prefix of a
// string or character literal (u8"..", L'..', R"(..)", u8R"x(..)x").
func cppLiteralPrefix(p string) bool {
	switch p {
	case "L", "u", "U", "u8", "R", "LR", "uR", "UR", "u8R":
		return true
	}
	return false
}

// cppSkipLiteral returns the end of the string or character literal whose
// prefix starts at start and whose quote is at q.
func cppSkipLiteral(src []byte, start, q int) int {
	n := len(src)
	quote := src[q]
	if quote == '"' && q > start && src[q-1] == 'R' {
		// R"delim( ... )delim"
		open := q + 1
		for open < n && src[open] != '(' && src[open] != '\n' && open-q <= 17 {
			open++
		}
		if open >= n || src[open] != '(' {
			return min(q+1, n)
		}
		closer := ")" + string(src[q+1:open]) + `"`
		if k := strings.Index(string(src[open+1:]), closer); k >= 0 {
			return open + 1 + k + len(closer)
		}
		return n
	}
	for i := q + 1; i < n; i++ {
		switch src[i] {
		case '\\':
			i++
		case quote:
			return i + 1
		case '\n':
			return i // an unterminated literal ends at its line
		}
	}
	return n
}

// cppSkipDirective returns the offset just past the preprocessor directive
// starting at the '#' at i: the end of its line, following backslash
// continuations, with a block comment that spans lines read through.
func cppSkipDirective(src []byte, i int) int {
	n := len(src)
	for i < n {
		switch {
		case src[i] == '\\' && i+1 < n && src[i+1] == '\n':
			i += 2
		case src[i] == '\\' && i+2 < n && src[i+1] == '\r' && src[i+2] == '\n':
			i += 3
		case src[i] == '/' && i+1 < n && src[i+1] == '*':
			i += 2
			for i+1 < n && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i = min(i+2, n)
		case src[i] == '/' && i+1 < n && src[i+1] == '/':
			for i < n && src[i] != '\n' {
				i++
			}
		case src[i] == '\n':
			return i
		default:
			i++
		}
	}
	return n
}

// cppAllCaps reports whether w is an all-capitals macro-style word: a capital
// letter, then capitals, digits and underscores, at least three bytes long,
// and holding an underscore or more than three letters, so a type parameter
// (T, U) or an acronym type (DIR) is never read as a macro.
func cppAllCaps(w []byte) bool {
	if len(w) < 3 || w[0] < 'A' || w[0] > 'Z' {
		return false
	}
	under := false
	for _, c := range w {
		switch {
		case c == '_':
			under = true
		case c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		default:
			return false
		}
	}
	return under || len(w) > 3
}

// cppDeclLead are the keywords after which a declaration's decorator macro
// may stand (`static LIB_INLINE int f()`).
var cppDeclLead = map[string]bool{
	"static": true, "inline": true, "extern": true, "virtual": true, "friend": true,
	"constexpr": true, "consteval": true, "constinit": true, "explicit": true, "export": true,
	"template": true, // an explicit instantiation, `template LIB_API void f<int>(int);`
}

// cppDeclEnd are the words that may follow a declarator's trailing macro.
var cppDeclEnd = map[string]bool{"override": true, "final": true, "const": true, "noexcept": true, "try": true}

// cppStmtWord are the keywords a word after which is an expression, not a
// declarator: a macro after one is no decorator.
var cppStmtWord = map[string]bool{
	"return": true, "case": true, "goto": true, "throw": true, "new": true, "delete": true,
	"sizeof": true, "else": true, "do": true, "typedef": true, "using": true, "co_return": true,
	"co_yield": true, "co_await": true, "alignof": true, "decltype": true, "typeid": true,
	"class": true, "struct": true, "union": true, "enum": true, "namespace": true, "template": true,
	"typename": true, "operator": true,
}

// cppDeclaratorAt reports whether a function declarator's name starts at
// token i and its parameter list opens on the same line: a word, then any of
// `::`, `~`, words and balanced template arguments, then `(`.
func cppDeclaratorAt(i int, word func(int) []byte, punct func(int, string) bool, sameLine func(a, b int) bool) bool {
	if word(i) == nil {
		return false
	}
	for j := i + 1; sameLine(i, j); j++ {
		switch {
		case punct(j, "("):
			return true
		case punct(j, "::"), punct(j, "~"), word(j) != nil && !punct(j-1, ">") && !(word(j-1) != nil):
		case punct(j, "<"):
			depth := 0
			for ; sameLine(i, j); j++ {
				if punct(j, "<") {
					depth++
				} else if punct(j, ">") {
					if depth--; depth == 0 {
						break
					}
				} else if punct(j, ";") || punct(j, "{") || punct(j, "(") {
					return false
				}
			}
			if depth != 0 {
				return false
			}
		default:
			return false
		}
	}
	return false
}

// cppAccessWord is the access specifier a macro named for one stands for
// (its name holds PRIVATE, PROTECTED or PUBLIC), padded to nothing; "" when
// none.
func cppAccessWord(name string) string {
	for _, kw := range []string{"private", "protected", "public"} {
		if strings.Contains(name, strings.ToUpper(kw)) {
			return kw
		}
	}
	return ""
}

// cppLoneNext reports whether the token at i, on the line after a macro that
// stands alone, starts a declaration or closes a block: a word, `}`, `[` (an
// attribute) or `~`.
func cppLoneNext(i int, word func(int) []byte, punct func(int, string) bool) bool {
	return word(i) != nil || punct(i, "}") || punct(i, "[") || punct(i, "~")
}

// cppBlankMacros is src with the decorator macros cppMacroTexts describes
// blanked and the try and catch macros spelled as keywords; ok is false when
// it finds none.
func cppBlankMacros(src []byte, toks []cppTok) (out []byte, ok bool) {
	word := func(i int) []byte {
		if i < 0 || i >= len(toks) || toks[i].kind != 'w' {
			return nil
		}
		return src[toks[i].start:toks[i].end]
	}
	punct := func(i int, p string) bool {
		return i >= 0 && i < len(toks) && toks[i].kind == 'p' && string(src[toks[i].start:toks[i].end]) == p
	}
	sameLine := func(a, b int) bool {
		return b < len(toks) && !strings.ContainsRune(string(src[toks[a].end:toks[b].start]), '\n')
	}
	// cppClassHead reports whether the token at i continues a class head
	// after its name: a body, a base clause, final or template arguments.
	cppClassHead := func(i int) bool {
		return punct(i, "{") || punct(i, ":") || punct(i, "<") || punct(i, "::") || string(word(i)) == "final"
	}
	blanked := make([]bool, len(toks))
	edit := func(start, end int, with string) {
		if out == nil {
			out = append([]byte(nil), src...)
		}
		for i := start; i < end; i++ {
			if out[i] != '\n' && out[i] != '\r' {
				out[i] = ' '
			}
		}
		copy(out[start:], with)
	}
	// prev is the index of the last token before i that is not blanked.
	prev := func(i int) int {
		for i--; i >= 0 && blanked[i]; i-- {
		}
		return i
	}
	// closeParen returns the index of the `)` matching the `(` at i, or -1.
	closeParen := func(i int) int {
		depth := 0
		for j := i; j < len(toks); j++ {
			switch {
			case punct(j, "("):
				depth++
			case punct(j, ")"):
				depth--
				if depth == 0 {
					return j
				}
			case punct(j, ";"), punct(j, "{"), punct(j, "}"):
				return -1
			}
		}
		return -1
	}
	// closeBrace returns the index of the `}` matching the `{` at i, or -1.
	closeBrace := func(i int) int {
		depth := 0
		for j := i; j < len(toks); j++ {
			switch {
			case punct(j, "{"):
				depth++
			case punct(j, "}"):
				depth--
				if depth == 0 {
					return j
				}
			}
		}
		return -1
	}
	blank := func(from, to int) { // tokens [from, to]
		edit(toks[from].start, toks[to].end, "")
		for k := from; k <= to; k++ {
			blanked[k] = true
		}
	}
	// catchAt reads a catch macro at i: the index of its `)` (or i itself when
	// it takes no arguments), and whether a handler body `{` follows it.
	catchAt := func(i int) (last int, body bool) {
		if !punct(i+1, "(") {
			return i, punct(i+1, "{")
		}
		c := closeParen(i + 1)
		if c < 0 {
			return -1, false
		}
		return c, punct(c+1, "{")
	}
	for i := 0; i < len(toks); i++ {
		w := word(i)
		if w == nil || blanked[i] || !cppAllCaps(w) {
			continue
		}
		s := string(w)
		if strings.HasSuffix(s, "_TRY") && punct(i+1, "{") {
			// A try macro reads as try when a catch macro with a handler
			// follows its block; a catch macro that is the whole handler
			// (`LIB_CATCH(where)`, expanding to every catch clause) leaves the
			// block a plain block and the handler blank.
			c := closeBrace(i+1) + 1
			if cw := word(c); c > 0 && cw != nil && cppAllCaps(cw) && strings.Contains(string(cw), "CATCH") {
				if last, body := catchAt(c); last >= 0 && body {
					edit(toks[i].start, toks[i].end, "try")
				} else if last >= 0 {
					blank(i, i)
					blank(c, last)
				}
				continue
			}
			blank(i, i)
			continue
		}
		if strings.HasSuffix(s, "_CATCH") {
			if last, body := catchAt(i); last > i && body {
				edit(toks[i].start, toks[i].end, "catch")
			}
			continue
		}
		p := prev(i)
		pw := string(word(p))
		atDeclStart := p < 0 || punct(p, ";") || punct(p, "{") || punct(p, "}") || punct(p, ">") || punct(p, ":") || cppDeclLead[pw] ||
			punct(p, "]") && punct(prev(p), "]") // after an attribute, [[noreturn]]
		// a macro alone on its line where a declaration or statement starts:
		// an object-like one (a namespace's opening or closing, a pragma), or
		// a function-like one with its arguments and no `;` after
		// (`LIB_PRAGMA(push)`, an attribute `LIB_NON_NULL(1)` over the
		// declaration below it).
		if atDeclStart && (p < 0 || !sameLine(p, i)) {
			last := i
			if punct(i+1, "(") {
				last = closeParen(i + 1)
			}
			if last >= 0 && (last+1 >= len(toks) || !sameLine(last, last+1) && cppLoneNext(last+1, word, punct)) {
				blank(i, last)
				continue
			}
		}
		// `if LIB_CONSTEXPR (cond)`, a C++17 `if constexpr` spelled as a macro
		if pw == "if" && punct(i+1, "(") {
			blank(i, i)
			continue
		}
		// between class/struct/union and the name
		if pw == "class" || pw == "struct" || pw == "union" {
			next := i + 1
			if punct(next, "(") {
				if c := closeParen(next); c > 0 {
					next = c + 1
				} else {
					continue
				}
			}
			if nw := word(next); nw != nil && sameLine(i, next) && !cppAllCaps(nw) && !cppDeclEnd[string(nw)] && cppClassHead(next+1) {
				blank(i, next-1)
			}
			continue
		}
		// before a declaration: the macro, not the next word, is what starts
		// it; a word followed by an initializer, a terminator or a subscript
		// is a variable the macro is the type of (`HANDLE h{nullptr};`).
		// an access label spelled as a macro (`LIB_PRIVATE_UNLESS_TESTED:`)
		if kw := cppAccessWord(s); kw != "" && atDeclStart && punct(i+1, ":") {
			edit(toks[i].start, toks[i].end, kw)
			continue
		}
		// a function-like macro standing as a whole template parameter (a
		// SFINAE constraint, `template <typename T, LIB_ENABLE_IF(cond)>`):
		// blanked with the comma that separates it
		if punct(i+1, "(") && (punct(p, ",") || punct(p, "<")) {
			if c := closeParen(i + 1); c > 0 {
				switch {
				case punct(p, ",") && (punct(c+1, ">") || punct(c+1, ",")):
					blank(p, c)
				case punct(p, "<") && punct(c+1, ","):
					blank(i, c+1)
				}
			}
			continue
		}
		if atDeclStart && (word(i+1) != nil || punct(i+1, "~")) && sameLine(i, i+1) {
			if !(punct(i+2, "{") || punct(i+2, "=") || punct(i+2, ";") || punct(i+2, "[") || punct(i+2, ",") || punct(i+2, ")")) {
				blank(i, i)
			}
			continue
		}
		// between a declaration's type and its declarator name
		if (p >= 0 && toks[p].kind == 'w' && !cppStmtWord[pw] || punct(p, "*") || punct(p, "&")) && cppDeclaratorAt(i+1, word, punct, sameLine) {
			blank(i, i)
			continue
		}
		// after a declarator's parameter list or array bound
		if punct(p, ")") || punct(p, "]") || pw == "const" && punct(prev(p), ")") {
			nx := i + 1
			if punct(nx, "{") || punct(nx, ";") || punct(nx, "=") || punct(nx, ":") || punct(nx, "->") || cppDeclEnd[string(word(nx))] || cppAllCaps(word(nx)) {
				blank(i, i)
			}
		}
	}
	return out, out != nil
}
