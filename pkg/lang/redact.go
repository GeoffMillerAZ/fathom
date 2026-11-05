package lang

// redact.go -- the door's literals rule (ADR-092, design section 1 rule 12). This
// is not a secret scanner. It is a rule about which of the file's own text lands
// in a Result, and it fails closed: what it cannot tell apart it withholds.
//
//   - Signature, code dialect (kind "language"): every quoted literal of every
//     quote style reads ***, whatever its name or its place, in the spelling of
//     the dialect (literalRules: raw, verbatim and escaped-backtick literals): a literal that
//     interpolates is one literal with the quotes inside its interpolation, and
//     percent literals, sigils and heredocs are quote styles. Numbers,
//     identifiers and unquoted text stay.
//   - Signature, any other dialect: withheld (empty) unless the symbol's kind is
//     on structuralKinds, the list of kinds whose Signature is structure and holds
//     no value, and what is returned has its quoted literals masked as a code
//     Signature does (a class header holds calls). The default is withheld, so a
//     kind nobody listed fails closed.
//     A kind is selected by its Kind, never by its Canonical.
//   - Fact values: a value under a key whose last segment looks like a credential
//     reads ***, and a value bound to a literal by = := or : is masked. Other Fact
//     values are returned as written.
//   - The userinfo of a URL is dropped wherever a string from the file lands, and
//     so is the userinfo of a scheme-less authority (user:pass@host, //u:p@host).
//   - Notes carry a reason and a line, never source text.
//
// A symbol name, a reference target, a reference's raw spelling and any value the
// rule does not name are returned as written.

import (
	"regexp"
	"strings"
	"unicode"
)

// redacted replaces a withheld literal.
const redacted = "***"

// userinfoRe finds a URL's authority part up to the last @ before the path:
// scheme, one or two slashes (a path join can collapse the pair), then the
// userinfo. The userinfo stops at the first `/`, `\`, `?` or `#`, where the
// authority ends: past one of them an @ belongs to the path, the query or the
// fragment, and the host is the text before it (`https://evil.example?x=@host`
// is evil.example). The scheme prefix of `git::https://` and `git+https://` is
// part of the match's scheme or sits before it.
var userinfoRe = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.\-]*):(//?)([^/\\?#\s"'<>]*)@`)

// slashesRe finds the authority of a URL whose scheme is not written
// (`//u:S@host/path`).
var slashesRe = regexp.MustCompile(`(^|[^A-Za-z0-9+\-:/])(//)([^/\\?#\s"'<>]*)@`)

// bareRe finds user:password@host written with no scheme and no slashes (a
// Dockerfile FROM, a Kubernetes image, a compose service): the userinfo starts
// after a space, a quote or a delimiter and holds a colon.
var bareRe = regexp.MustCompile(`(^|[\s"'=,(\[{>|;])([A-Za-z0-9._~%+\-]+:[^\s/\\?#"'<>@]*)@([A-Za-z0-9\[])`)

// digestRe is what follows the @ of an image reference pinned by digest.
var digestRe = regexp.MustCompile(`^(?:sha256|sha384|sha512):[0-9A-Fa-f]+`)

// withholdURLUserinfo drops the userinfo of every URL in s. The one account that
// stays is `git` in an ssh URL (`ssh://git@github.com/o/r.git`, the name of the
// service's account, not a secret), and the image of a docker or oci reference
// whose @ is a digest pin (`docker://alpine@sha256:...`, a GitHub Actions
// `uses: docker://alpine@sha256:...`).
func withholdURLUserinfo(s string) string {
	if !strings.Contains(s, "@") {
		return s
	}
	var b strings.Builder
	last := 0
	for _, ix := range userinfoRe.FindAllStringSubmatchIndex(s, -1) {
		scheme, slashes, userinfo := s[ix[2]:ix[3]], s[ix[4]:ix[5]], s[ix[6]:ix[7]]
		if keepUserinfo(scheme, userinfo, s[ix[1]:]) {
			continue
		}
		b.WriteString(s[last:ix[0]])
		b.WriteString(scheme + ":" + slashes)
		last = ix[1]
	}
	if last > 0 {
		b.WriteString(s[last:])
		s = b.String()
	}
	if !strings.Contains(s, "@") {
		return s
	}
	s = slashesRe.ReplaceAllString(s, "${1}${2}")
	return withholdBareUserinfo(s)
}

// withholdBareUserinfo drops user:password@ from text that has no scheme. A
// scheme written as `mailto:dev@example.com`, and an image pinned by digest
// (`alpine:3.19@sha256:...`), are not a user and a password.
func withholdBareUserinfo(s string) string {
	var b strings.Builder
	last := 0
	for _, ix := range bareRe.FindAllStringSubmatchIndex(s, -1) {
		userinfo := s[ix[4]:ix[5]]
		scheme := userinfo[:strings.IndexByte(userinfo, ':')]
		if bareScheme(scheme) || digestRe.MatchString(s[ix[5]+1:]) {
			continue
		}
		b.WriteString(s[last:ix[4]])
		last = ix[5] + 1 // past the @
	}
	if last == 0 {
		return s
	}
	b.WriteString(s[last:])
	return b.String()
}

// bareScheme reports a URI scheme whose rest is an account and a host, not a
// user and a password.
func bareScheme(s string) bool {
	switch strings.ToLower(s) {
	case "mailto", "sip", "sips", "xmpp", "im", "acct", "tel", "callto", "skype":
		return true
	}
	return false
}

// keepUserinfo reports whether the userinfo before an @ is no credential: the
// ssh account `git`, or the name of an image in a docker or oci reference that a
// digest follows (`docker://alpine:3.19@sha256:...`).
func keepUserinfo(scheme, userinfo, rest string) bool {
	switch {
	case len(scheme) == 1:
		return true // a drive letter: C:/me@corp/m is a path
	case sshScheme(scheme):
		return userinfo == "git"
	case imageScheme(scheme):
		return digestRe.MatchString(rest)
	}
	return false
}

func sshScheme(scheme string) bool {
	s := strings.ToLower(scheme)
	return s == "ssh" || strings.HasSuffix(s, "+ssh") || s == "git"
}

func imageScheme(scheme string) bool {
	s := strings.ToLower(scheme)
	return s == "docker" || s == "oci"
}

// literalRules is how a dialect spells the literals the plain quote rule reads
// wrong: a literal with no escapes (a C# verbatim string, a C++ or Rust raw
// string, a Kotlin or Scala triple quote, a Dart `r'...'`, a Go raw string), a
// backtick that honours a backslash (a JS or TS template, a Ruby command) and a
// Ruby percent literal with no type letter. The reading of a dialect that names
// none of them is the plain rule.
type literalRules struct {
	skip        func(s string, i int) bool // a single quote that opens no literal
	verbatim    bool                       // C#: @"..." has no escapes
	rawTriple   bool                       // C#, Kotlin, Scala: """...""" has no escapes
	cppRaw      bool                       // C and C++: R"delim(...)delim"
	rustRaw     bool                       // Rust: r"..." and r#"..."#, with b and c prefixes
	swiftRaw    bool                       // Swift: #"..."# and #"""..."""#
	dartRaw     bool                       // Dart: r'...' and r"..."
	backtickEsc bool                       // JS, TS, Ruby, PHP: a backslash escapes in a backtick literal
	bareRuby    bool                       // Ruby: %(...), %[...], %{...} and %<...> are %Q literals
}

// rulesFor is the literal rules of the dialect that read the file, "" and an
// unknown dialect reading by the plain rule.
func rulesFor(dialect string) literalRules {
	switch dialect {
	case "rust":
		return literalRules{skip: rustLifetime, rustRaw: true}
	case "csharp":
		return literalRules{verbatim: true, rawTriple: true}
	case "kotlin", "scala":
		return literalRules{rawTriple: true}
	case "c", "cpp", "cuda":
		return literalRules{cppRaw: true}
	case "swift":
		return literalRules{swiftRaw: true}
	case "dart":
		return literalRules{dartRaw: true}
	case "javascript", "typescript", "vue", "svelte", "astro", "php":
		return literalRules{backtickEsc: true}
	case "ruby":
		return literalRules{backtickEsc: true, bareRuby: true}
	}
	return literalRules{}
}

// maskLiterals is the plain rule (rulesFor("")) with a skip function.
func maskLiterals(s string, skip func(s string, i int) bool) string {
	return literalRules{skip: skip}.mask(s)
}

// mask replaces what is between the delimiters of every literal in a code
// signature with ***, whatever its quote style:
//
//   - double, single and backtick quotes and the triple quotes of Python, the
//     one- or two-letter prefix of a literal (b, f, r, rb) staying where it is;
//   - the literals the dialect reads with no escapes or with its own delimiter
//     (literalRules): a C# `@"C:\"`, a C++ `R"x(...)x"`, a Rust `r#"..."#`;
//   - the interpolation of a literal (`${...}` in JS, TS, Kotlin, Scala and Dart,
//     `#{...}` in Ruby and Elixir, `\(...)` in Swift, `{...}` in a Python f-string
//     or a C# `$"..."`), which may hold quoted literals of its own: they are part
//     of the outer literal, and the outer literal ends at its own closing quote;
//   - a percent literal (Ruby `%w[...]`, `%q(...)`, `%i[...]`, `%(...)`), an
//     Elixir sigil (`~S|...|`, `~w(...)`) and a heredoc (PHP `<<<EOT`, Ruby
//     `<<~EOS`).
//
// An escape (a backslash) does not end a literal that has escapes, and a literal
// that is never closed is masked to the end of the text, so a missing delimiter
// withholds more and never less. A single quote that skip says opens no literal
// (a Rust lifetime, an apostrophe in a sentence) is left as written.
func (r literalRules) mask(s string) string {
	if !strings.ContainsAny(s, "\"'`%~<") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		head, bodyEnd, next, ok := r.literalAt(s, i)
		if !ok {
			b.WriteByte(s[i])
			i++
			continue
		}
		b.WriteString(s[i : i+head])
		b.WriteString(redacted)
		if bodyEnd < 0 {
			break // never closed: the rest of the text is the literal
		}
		b.WriteString(s[bodyEnd:next])
		i = next
	}
	return b.String()
}

// literalAt reports the literal that opens at s[i]. head is the length of its
// opener (the quote, `%w[`, `<<<EOT`), bodyEnd the index where its closer
// starts (-1 when it is never closed) and next the index after the closer. next
// is 0 when no literal opens at i.
func (r literalRules) literalAt(s string, i int) (head, bodyEnd, next int, ok bool) {
	switch c := s[i]; c {
	case '"', '\'', '`':
		if c == '\'' && r.skip != nil && r.skip(s, i) {
			return 0, 0, 0, false
		}
		head, spec := r.quoteAt(s, i)
		end := r.closeLiteral(s, i+head, spec)
		return head, end, end + len(spec.closer), true
	case '%':
		head := r.percentHead(s, i)
		if head == 0 {
			return 0, 0, 0, false
		}
		open, closer := delimiters(s[i+head-1])
		end := r.closeLiteral(s, i+head, literalSpec{open: open, closer: closer, escapes: true, interp: interpHash})
		return head, end, end + len(closer), true
	case '~':
		if !sigilOpens(s, i) {
			return 0, 0, 0, false
		}
		j := i + 2
		for j < len(s) && s[j] >= 'A' && s[j] <= 'Z' {
			j++
		}
		open, closer := delimiters(s[j])
		end := r.closeLiteral(s, j+1, literalSpec{open: open, closer: closer, escapes: true, interp: interpHash})
		return j + 1 - i, end, end + len(closer), true
	case '<':
		id, headLen := heredocAt(s, i)
		if id == "" {
			return 0, 0, 0, false
		}
		end := indexWord(s, i+headLen, id)
		return headLen, end, end + len(id), true
	}
	return 0, 0, 0, false
}

// A literal's interpolation, by the characters that open it.
const (
	interpNone  = 0
	interpHash  = 1 // #{...}, ${...} and \(...)
	interpBrace = 2 // {...}: a Python f-string, a C# $"..." string
)

// literalSpec describes one literal's body for closeLiteral.
type literalSpec struct {
	open    byte   // a nesting opener (the `[` of `%w[`), 0 for none
	closer  string // what ends the literal
	escapes bool
	interp  int
}

// quoteAt is the opener length and the body of the literal that the quote at
// s[i] opens. The characters before the quote have been written already, so the
// prefix of a raw or verbatim literal (`@`, `R`, `r#`) is not part of head.
func (r literalRules) quoteAt(s string, i int) (head int, spec literalSpec) {
	c := s[i]
	if c == '"' {
		switch {
		case r.cppRaw:
			if delim, ok := cppRawDelim(s, i); ok {
				return len(delim) + 2, literalSpec{closer: ")" + delim + `"`}
			}
		case r.rustRaw:
			if hashes, ok := rustRawHashes(s, i); ok {
				return 1, literalSpec{closer: `"` + hashes}
			}
		}
	}
	n := quoteLen(s, i)
	spec = literalSpec{closer: strings.Repeat(string(c), n), escapes: c != '`' || r.backtickEsc, interp: interpAt(s, i)}
	switch {
	case r.swiftRaw && c == '"':
		if hashes := hashesBefore(s, i); hashes != "" {
			spec.closer += hashes
			spec.escapes, spec.interp = false, interpNone
		}
	case r.dartRaw && (c == '"' || c == '\'') && i > 0 && s[i-1] == 'r' && (i == 1 || !identChar(s[i-2])):
		spec.escapes, spec.interp = false, interpNone
	case r.verbatim && c == '"' && verbatimPrefix(s, i):
		spec.escapes = false
	case r.rawTriple && c == '"' && n == 3:
		spec.escapes = false
	}
	return n, spec
}

// verbatimPrefix reports a C# `@` in the prefix of the quote at i: `@"`, `$@"`
// or `@$"`.
func verbatimPrefix(s string, i int) bool {
	for j := i - 1; j >= 0 && i-j <= 2 && (s[j] == '@' || s[j] == '$'); j-- {
		if s[j] == '@' {
			return true
		}
	}
	return false
}

// hashesBefore is the run of # right before i.
func hashesBefore(s string, i int) string {
	j := i
	for j > 0 && s[j-1] == '#' {
		j--
	}
	return s[j:i]
}

// cppRawDelim is the delimiter of the C++ raw string whose opening quote is at
// s[i]: `R"x(`, with an optional u8, u, U or L before the R. The delimiter is at
// most 16 characters and holds no space, quote, parenthesis or backslash.
func cppRawDelim(s string, i int) (string, bool) {
	j := i - 1
	if j < 0 || s[j] != 'R' {
		return "", false
	}
	switch {
	case j >= 2 && s[j-2:j] == "u8":
		j -= 2
	case j >= 1 && strings.ContainsRune("uUL", rune(s[j-1])):
		j--
	}
	if j > 0 && identChar(s[j-1]) {
		return "", false
	}
	for k := i + 1; k < len(s) && k <= i+17; k++ {
		switch s[k] {
		case '(':
			return s[i+1 : k], true
		case ' ', '"', ')', '\\', '\t', '\n', '\r':
			return "", false
		}
	}
	return "", false
}

// rustRawHashes is the hashes of the Rust raw string whose opening quote is at
// s[i]: `r"`, `r#"`, `br#"`, `cr##"`.
func rustRawHashes(s string, i int) (string, bool) {
	hashes := hashesBefore(s, i)
	j := i - len(hashes) - 1
	if j < 0 || s[j] != 'r' {
		return "", false
	}
	if j > 0 && (s[j-1] == 'b' || s[j-1] == 'c') {
		j--
	}
	if j > 0 && identChar(s[j-1]) {
		return "", false
	}
	return hashes, true
}

// closeLiteral is the index of the closer of a literal whose body starts at
// from, -1 for none. A quote inside an interpolation belongs to the literal.
func (r literalRules) closeLiteral(s string, from int, spec literalSpec) int {
	depth := 0
	for j := from; j < len(s); j++ {
		c := s[j]
		switch {
		case c == '\\' && spec.escapes:
			if spec.interp != interpNone && j+1 < len(s) && s[j+1] == '(' {
				k := r.skipCode(s, j+2, '(', ')')
				if k < 0 {
					return -1
				}
				j = k
				continue
			}
			j++
		case spec.interp != interpNone && (c == '$' || c == '#') && j+1 < len(s) && s[j+1] == '{':
			k := r.skipCode(s, j+2, '{', '}')
			if k < 0 {
				return -1
			}
			j = k
		case c == '{' && spec.interp&interpBrace != 0:
			if j+1 < len(s) && s[j+1] == '{' {
				j++ // an escaped brace
				continue
			}
			k := r.skipCode(s, j+1, '{', '}')
			if k < 0 {
				return -1
			}
			j = k
		case spec.open != 0 && c == spec.open:
			depth++
		case strings.HasPrefix(s[j:], spec.closer):
			if depth == 0 {
				return j
			}
			depth--
		}
	}
	return -1
}

// skipCode is the index of the bracket that closes code whose opener was just
// before from, -1 when it is never closed. A quote in the code opens a literal
// that is read whole.
func (r literalRules) skipCode(s string, from int, open, closeCh byte) int {
	depth := 0
	for j := from; j < len(s); j++ {
		switch c := s[j]; c {
		case open:
			depth++
		case closeCh:
			if depth == 0 {
				return j
			}
			depth--
		case '"', '\'', '`':
			head, spec := r.quoteAt(s, j)
			end := r.closeLiteral(s, j+head, spec)
			if end < 0 {
				return -1
			}
			j = end + len(spec.closer) - 1
		}
	}
	return -1
}

// quoteLen is 3 for the triple quote of Python at i, 1 otherwise.
func quoteLen(s string, i int) int {
	if s[i] != '`' && strings.HasPrefix(s[i:], strings.Repeat(string(s[i]), 3)) {
		return 3
	}
	return 1
}

// interpAt is how the literal opened by the quote at i interpolates. Every quote
// takes `${`, `#{` and `\(`, which fails closed (a balanced pair of braces in a
// plain string stays inside it, an unbalanced one withholds the rest). Only a
// literal with the prefix of an f-string (f, rf, t) or of C# ($, $@) takes `{`.
func interpAt(s string, i int) int {
	j := i
	for j > 0 && (isLetter(s[j-1]) || s[j-1] == '$' || s[j-1] == '@') {
		j--
	}
	run := s[j:i]
	if len(run) > 3 || (j > 0 && (isLetter(s[j-1]) || s[j-1] == '_' || s[j-1] >= '0' && s[j-1] <= '9')) {
		return interpHash
	}
	if strings.ContainsAny(run, "fFtT$") {
		return interpHash | interpBrace
	}
	return interpHash
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// identChar reports a byte an identifier holds.
func identChar(c byte) bool { return isLetter(c) || c == '_' || c >= '0' && c <= '9' }

// percentHead is the length of the opener of the Ruby percent literal at i: `%`
// then one of q Q w W i I r s x, then a delimiter (3), or, in Ruby, `%` and a
// bracket (2). The `%` is no modulo: it follows no identifier, number or closing
// bracket. 0 for none.
func (r literalRules) percentHead(s string, i int) int {
	if i > 0 && (identChar(s[i-1]) || s[i-1] == ')' || s[i-1] == ']') {
		return 0
	}
	switch {
	case i+2 < len(s) && strings.ContainsRune("qQwWiIrsx", rune(s[i+1])) && strings.ContainsRune("([{<|!/^~", rune(s[i+2])):
		return 3
	case r.bareRuby && i+1 < len(s) && strings.ContainsRune("([{<", rune(s[i+1])):
		return 2
	}
	return 0
}

// sigilOpens reports an Elixir sigil at i: `~`, one letter or a run of capitals,
// then a delimiter that is no quote (a quote is read as a quoted literal).
func sigilOpens(s string, i int) bool {
	j := i + 1
	if j >= len(s) || !isLetter(s[j]) {
		return false
	}
	j++
	if s[j-1] >= 'A' && s[j-1] <= 'Z' {
		for j < len(s) && s[j] >= 'A' && s[j] <= 'Z' {
			j++
		}
	}
	return j < len(s) && strings.ContainsRune("/|([{<", rune(s[j]))
}

// delimiters is the nesting opener (0 when the delimiter does not nest) and the
// closer of a literal opened by delim.
func delimiters(delim byte) (open byte, closer string) {
	switch delim {
	case '(':
		return '(', ")"
	case '[':
		return '[', "]"
	case '{':
		return '{', "}"
	case '<':
		return '<', ">"
	}
	return 0, string(delim)
}

// heredocAt reports the identifier of a heredoc that opens at i (PHP `<<<EOT`,
// `<<<'EOT'`, Ruby `<<~EOS`, `<<-EOS`) and the length of its opener.
func heredocAt(s string, i int) (id string, headLen int) {
	rest := s[i:]
	switch {
	case strings.HasPrefix(rest, "<<<"):
		headLen = 3
	case strings.HasPrefix(rest, "<<~"), strings.HasPrefix(rest, "<<-"):
		headLen = 3
	default:
		return "", 0
	}
	j := i + headLen
	for j < len(s) && s[j] == ' ' {
		j++
	}
	quote := byte(0)
	if j < len(s) && (s[j] == '\'' || s[j] == '"') {
		quote = s[j]
		j++
	}
	start := j
	for j < len(s) && identChar(s[j]) {
		j++
	}
	if j == start || !isLetter(s[start]) && s[start] != '_' {
		return "", 0
	}
	id = s[start:j]
	if quote != 0 {
		if j >= len(s) || s[j] != quote {
			return "", 0
		}
		j++
	}
	return id, j - i
}

// indexWord is the index of the first occurrence of id as a whole word at or
// after from, -1 for none.
func indexWord(s string, from int, id string) int {
	for j := from; j+len(id) <= len(s); j++ {
		if strings.HasPrefix(s[j:], id) && (j == 0 || !identChar(s[j-1])) && (j+len(id) == len(s) || !identChar(s[j+len(id)])) {
			return j
		}
	}
	return -1
}

// maskQuoted replaces what is between the quotes of every quoted literal in s
// with ***: double, single and backtick quotes, the triple quotes of Python. It
// is the Notes rule, which reads a parser's message, not code. An escape does
// not end a double or single quoted literal, and a literal that is never closed
// is masked to the end of the text. A single quote that skip says opens no
// literal is left as written.
func maskQuoted(s string, skip func(s string, i int) bool) string {
	if !strings.ContainsAny(s, "\"'`") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		q := s[i]
		if q != '"' && q != '\'' && q != '`' {
			b.WriteByte(q)
			i++
			continue
		}
		if q == '\'' && skip != nil && skip(s, i) {
			b.WriteByte(q)
			i++
			continue
		}
		n := 1
		if q != '`' && strings.HasPrefix(s[i:], strings.Repeat(string(q), 3)) {
			n = 3
		}
		end := closingQuote(s, i+n, q, n)
		b.WriteString(strings.Repeat(string(q), n))
		b.WriteString(redacted)
		if end < 0 {
			break // never closed: the rest of the text is the literal
		}
		b.WriteString(strings.Repeat(string(q), n))
		i = end + n
	}
	return b.String()
}

// closingQuote is the index of the quote that closes a literal whose body starts
// at from, -1 for none.
func closingQuote(s string, from int, q byte, n int) int {
	closer := strings.Repeat(string(q), n)
	for j := from; j < len(s); j++ {
		switch {
		case s[j] == '\\' && q != '`':
			j++
		case strings.HasPrefix(s[j:], closer):
			return j
		}
	}
	return -1
}

// rustLifetime reports whether the single quote at i opens a lifetime: an
// identifier that is not closed by another quote (`'a'` is a character).
func rustLifetime(s string, i int) bool {
	j := i + 1
	if j >= len(s) || !(unicode.IsLetter(rune(s[j])) || s[j] == '_') {
		return false
	}
	for j < len(s) && (unicode.IsLetter(rune(s[j])) || unicode.IsDigit(rune(s[j])) || s[j] == '_') {
		j++
	}
	return j >= len(s) || s[j] != '\''
}

// apostrophe reports whether the single quote at i sits inside a word
// (`couldn't`), where it is no quote.
func apostrophe(s string, i int) bool {
	return i > 0 && i+1 < len(s) && unicode.IsLetter(rune(s[i-1])) && unicode.IsLetter(rune(s[i+1]))
}

// scrubNote reduces a parser's message to the reason and the line it gives. A
// parser quotes the text it stopped at (`found "S"`, `setting "S" has no = or :`)
// and some name it unquoted after a colon (`unknown directive: S`); a Note never
// carries the file's own text, so quoted text reads *** and a single word after a
// colon that ends the message or a clause is dropped.
func scrubNote(s string) string {
	// A parser that found a literal token names its class and then the literal
	// itself, unquoted (CUE: `found 'IDENT' S`): the class stays, the text goes.
	s = literalTokenRe.ReplaceAllString(s, "${1}${2}")
	// go/parser names an identifier or a number it stopped at bare, after a comma
	// (`expected ';', found S`), where a keyword or an operator is quoted and a
	// string is quoted by its own quotes: the bare word is the file's text.
	s = foundBareRe.ReplaceAllStringFunc(s, func(m string) string {
		if word := m[len(", found "):]; word == "newline" || word == "EOF" {
			return m
		}
		return ", found " + redacted
	})
	// A quoted punctuation mark is the grammar the parser expected (`expected ',' or
	// ']'`), never a word of the file, and stays.
	s = punctuationQuoteRe.ReplaceAllStringFunc(s, func(m string) string {
		if m[0] == '\'' {
			return "\x01" + m[1:2] + "\x01"
		}
		return "\x02" + m[1:2] + "\x02"
	})
	s = maskQuoted(s, apostrophe)
	s = strings.NewReplacer("\x01", "'", "\x02", "\"").Replace(s)
	return noteTokenRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := noteTokenRe.FindStringSubmatch(m)
		if sub[1] == "EOF" || numeric(sub[1]) {
			return m
		}
		return ":" + sub[2]
	})
}

// literalTokenRe is a token class in capitals (CUE's IDENT, INT, STRING,
// ATTRIBUTE) after `found`, with the text of the file that follows it to the end
// of the message or to a trailing `(and N more errors)`.
var literalTokenRe = regexp.MustCompile(`(\bfound '[A-Z_]{3,}')[^\n]*?( \(and \d+ more errors?\))?$`)

// foundBareRe is the unquoted word after `, found `: go/parser's identifier or
// number (`expected ';', found S`).
var foundBareRe = regexp.MustCompile(`, found [^\s'"]+`)

// punctuationQuoteRe is a quote around one character that is no letter, digit or
// space.
var punctuationQuoteRe = regexp.MustCompile(`'[^\w\s'"\x01\x02]'|"[^\w\s'"\x01\x02]"`)

// noteTokenRe is a single word after a colon that ends the message or a clause
// (`unknown directive: S`, `unknown directive: S; the line was skipped`).
var noteTokenRe = regexp.MustCompile(`: ([^\s:;,"'()]+)(;|,|\)|$)`)

func numeric(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

// literalRe finds a string literal in a fact's value: double, single or triple
// quoted, a backtick template, and the one- or two-letter prefix of a Python or
// Rust literal (b, f, r, rb, u).
var literalRe = regexp.MustCompile("[A-Za-z]{0,2}(?:\"\"\"(?s:.*?)\"\"\"|'''(?s:.*?)'''|\"(?:[^\"\\\\]|\\\\.)*\"|'(?:[^'\\\\]|\\\\.)*'|`[^`]*`)")

// bound reports whether the text before a literal binds it to a name or a key: it
// ends in `=` (not `==`, `!=`, `<=` or `>=`), `:=` or `:`.
func bound(before string) bool {
	before = strings.TrimRight(before, " \t")
	switch {
	case strings.HasSuffix(before, ":=") || strings.HasSuffix(before, ":"):
		return true
	case strings.HasSuffix(before, "="):
		return len(before) < 2 || !strings.ContainsRune("=!<>", rune(before[len(before)-2]))
	}
	return false
}

// maskLiteral is lit with what is between its quotes replaced.
func maskLiteral(lit string) string {
	i := strings.IndexAny(lit, "\"'`")
	q := lit[i:]
	n := 1
	if len(q) >= 6 && (strings.HasPrefix(q, `"""`) || strings.HasPrefix(q, "'''")) {
		n = 3
	}
	return lit[:i+n] + redacted + lit[len(lit)-n:]
}

// maskBoundLiterals masks the string literals in s that are bound to a name or a
// key by `=`, `:=` or `:`, whatever the name is: a fact's value `password = "x"`,
// a dictionary entry or a connection string.
func maskBoundLiterals(s string) string {
	var b strings.Builder
	last := 0
	for _, ix := range literalRe.FindAllStringIndex(s, -1) {
		lit := s[ix[0]:ix[1]]
		if ix[0] > 0 && unicode.IsLetter(rune(s[ix[0]-1])) && unicode.IsLetter(rune(lit[0])) {
			continue // the letters belong to a longer word, not to a prefix
		}
		if !bound(s[:ix[0]]) {
			continue
		}
		b.WriteString(s[last:ix[0]])
		b.WriteString(maskLiteral(lit))
		last = ix[1]
	}
	if last == 0 {
		return s
	}
	b.WriteString(s[last:])
	return b.String()
}

// cliSecretRe finds the value of a command-line flag that carries a credential
// (`--password x`, `--token=x`, `-u user:pass`) and of an Authorization header,
// in a fact's value.
var cliSecretRe = regexp.MustCompile(
	`(?i)(--?(?:password|passwd|pass|token|api-?key|secret|auth(?:orization)?|bearer|access-?key)(?:\s+|=))([^\s'"$%-][^\s'"]*)` +
		`|(\s-u\s+)([^\s'"$%]+:[^\s'"]+)` +
		`|(authorization:\s*(?:bearer|basic|token)?\s*)([^\s'"$]+)`)

// withholdFactValue masks, in a fact's value, the userinfo of a URL, the string
// literals bound to a name or a key, the value of a credential flag and an
// Authorization header's value. Any other fact value is returned as written.
func withholdFactValue(s string) string {
	s = withholdURLUserinfo(s)
	s = maskBoundLiterals(s)
	if strings.ContainsAny(s, "-:") {
		s = cliSecretRe.ReplaceAllStringFunc(s, func(m string) string {
			sub := cliSecretRe.FindStringSubmatch(m)
			for _, pair := range [][2]int{{1, 2}, {3, 4}, {5, 6}} {
				if sub[pair[0]] != "" {
					return sub[pair[0]] + redacted
				}
			}
			return m
		})
	}
	return s
}

// credentialWords are what a fact key's last segment holds when its value is a
// credential, after the case and the separators (`_`, `-`) are dropped.
var credentialWords = []string{"password", "passwd", "secret", "token", "apikey", "credential", "privatekey"}

// credentialKey reports whether the last segment of a fact's key looks like a
// credential: `labels.password`, `env.DB_PASSWORD`, `clientSecret`, `api-key`.
func credentialKey(key string) bool {
	if i := strings.LastIndexAny(key, "./"); i >= 0 {
		key = key[i+1:]
	}
	key = strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(key))
	for _, w := range credentialWords {
		if strings.Contains(key, w) {
			return true
		}
	}
	return false
}

// withhold applies the door's literals rule to a finished Result in place.
// dialectKind is the Kind ("language" or "tooling") of the dialect that read the
// file, and "" when it is not known, which reads as a dialect that is no code.
func (res *Result) withhold(dialectKind string) {
	code := dialectKind == "language"
	rules := rulesFor(res.Dialect)
	// A Key is Package.Name (design rule 9), so it is rebuilt from the scrubbed
	// parts: scrubbing the joined text separately can eat the package prefix
	// (`pod.u:S@reg/i` reads as the user `pod.u`) or keep a userinfo the Name lost.
	keys := map[string]string{}
	for i := range res.Symbols {
		s := &res.Symbols[i]
		old := s.Key
		s.Name, s.Package = withholdURLUserinfo(s.Name), withholdURLUserinfo(s.Package)
		s.Key = symbolKey(s.Package, s.Name)
		if _, seen := keys[old]; !seen {
			keys[old] = s.Key
		}
		switch {
		case code:
			s.Signature = withholdURLUserinfo(rules.mask(s.Signature))
		case structuralKind(s.Kind):
			s.Signature = withholdURLUserinfo(rules.mask(structuralSignature(s.Kind, s.Signature)))
		default:
			s.Signature = ""
		}
	}
	for i := range res.References {
		r := &res.References[i]
		if k, ok := keys[r.From]; ok {
			r.From = k
		} else {
			r.From = withholdURLUserinfo(r.From)
		}
		r.To, r.Raw = withholdURLUserinfo(r.To), withholdURLUserinfo(r.Raw)
	}
	for i := range res.Unresolved {
		u := &res.Unresolved[i]
		if k, ok := keys[u.From]; ok {
			u.From = k
		} else {
			u.From = withholdURLUserinfo(u.From)
		}
		u.Raw = withholdURLUserinfo(u.Raw)
	}
	for i := range res.Facts {
		f := &res.Facts[i]
		f.Entity = withholdURLUserinfo(f.Entity)
		if credentialKey(f.Key) && f.Value != "" {
			f.Value = redacted
		} else {
			f.Value = withholdFactValue(f.Value)
		}
	}
}
