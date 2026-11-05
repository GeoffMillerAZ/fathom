package lang

// redact.go -- the door's literals rule (ADR-092, design section 1 rule 12). This
// is not a secret scanner. It is a rule about which of the file's own text lands
// in a Result, and it fails closed: what it cannot tell apart it withholds.
//
//   - Signature, code dialect (kind "language") on quoteLed, the list of dialects
//     each of whose literal forms opens with a quote character: the Signature is
//     kept up to its first quote character (', " or a backtick) and everything
//     from the quote on reads ***. There is no pairing, no closer search and no
//     grammar of any dialect: a quote that opens no literal (a comment's
//     apostrophe, a digit separator, a quote in a regex) cuts too, and Rust's
//     lifetimes and labels are the one exception. Over-masking is accepted.
//     Prefix characters before the quote (r, b, u8, R, @, $, #, f) stay.
//   - Signature, any other code dialect: withheld (empty).
//   - Signature, any dialect that is no code: withheld (empty) unless the symbol's
//     kind is on structuralKinds, the list of kinds whose Signature is structure
//     and holds no value, and what is returned is cut at its first quote as a
//     code Signature is. The default is withheld, so a kind nobody listed fails
//     closed. A kind is selected by its Kind, never by its Canonical.
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

// quoteLed is the list of code dialects whose Signature is returned up to its first
// quote. Each entry is the language reference rule that every string and char
// literal form of the language opens with a quote character (' " or a backtick),
// possibly after identifier or sigil prefix characters (r, b, u8, R, @, $, #, f),
// so a Signature cut at its first quote holds no literal. A code dialect that is
// not here withholds its Signature, so a dialect nobody classified fails closed
// (TestEveryCodeDialectIsClassified holds the list).
var quoteLed = map[string]string{
	"c":          "C11 6.4.4.4 and 6.4.5: character constants and string literals open with ' or \", after an L, u, U or u8 prefix",
	"cpp":        "C++ [lex.string] and [lex.ccon]: character and string literals, raw ones too, open with ' or \", after an encoding prefix or R",
	"csharp":     "C# 11 lexical grammar: character, regular, verbatim (@), interpolated ($) and raw string literals open with ' or \"",
	"cuda":       "CUDA C++ takes the C++ literal grammar: every literal opens with ' or \", after an encoding prefix or R",
	"dart":       "Dart language spec, Strings: single, double, multi-line and raw (r) strings open with ' or \"",
	"go":         "Go spec, Rune and String literals: they open with ', \" or a backtick",
	"java":       "JLS 3.10.4 to 3.10.6: character literals, string literals and text blocks open with ' or \"",
	"javascript": "ECMAScript 12.9: string literals open with ' or \", template literals with a backtick",
	"kotlin":     "Kotlin spec, literal constants: character, string and raw string literals open with ' or \"",
	"python":     "Python reference 2.4.1: string and bytes literals open with ' or \", after the prefix letters r, u, b, f, t",
	"rust":       "Rust reference, Tokens: char, byte, string, raw string and C string literals open with ' or \", after b, c or r#",
	"scala":      "Scala spec 1.3: character, string, triple-quoted and interpolated string literals open with ' or \", after an identifier",
	"swift":      "The Swift Programming Language, Lexical Structure: string literals open with \", after the # of a raw string",
	"typescript": "TypeScript takes the ECMAScript literal grammar: string literals open with ' or \", template literals with a backtick",
}

// cutAtQuote is sig up to its first quote character (', " or a backtick), and ***
// from the quote on, the quote included: no pairing, no closer search, no grammar of any dialect. The
// prefix characters before the quote stay as written. A quote that opens no
// literal (a comment's apostrophe, a digit separator, a quote in a regex) cuts
// too, since over-masking is the price of a rule that cannot be fooled. In Rust a
// ' that starts a lifetime or a label stays (rustLifetime).
func cutAtQuote(sig string, rust bool) string {
	for i := 0; i < len(sig); i++ {
		switch sig[i] {
		case '"', '`':
			return sig[:i] + redacted
		case '\'':
			if rust && rustLifetime(sig, i) {
				continue
			}
			return sig[:i] + redacted
		}
	}
	return sig
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

// rustLifetime reports whether the ' at i starts a lifetime or a label: an ASCII
// identifier and then a character that is not another '. (`'a'` is a char literal;
// an identifier that ends the text has no character after it and reads as a quote.)
func rustLifetime(s string, i int) bool {
	j := i + 1
	if j >= len(s) || !(isLetter(s[j]) || s[j] == '_') {
		return false
	}
	for j < len(s) && (isLetter(s[j]) || s[j] == '_' || s[j] >= '0' && s[j] <= '9') {
		j++
	}
	return j < len(s) && s[j] != '\''
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

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
	_, quoteLedDialect := quoteLed[res.Dialect]
	rust := res.Dialect == "rust"
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
		case code && quoteLedDialect:
			s.Signature = withholdURLUserinfo(cutAtQuote(s.Signature, rust))
		case code:
			s.Signature = "" // a code dialect nobody classified as quote-led withholds
		case structuralKind(s.Kind):
			s.Signature = withholdURLUserinfo(cutAtQuote(structuralSignature(s.Kind, s.Signature), false))
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
