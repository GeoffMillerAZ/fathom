package lang

// redact.go -- the door's literals rule (ADR-092, design section 1 rule 12). This
// is not a secret scanner. It is a rule about which of the file's own text lands
// in a Result, and it fails closed: what it cannot tell apart it withholds.
//
//   - Signature, code dialect (kind "language"): every quoted literal of every
//     quote style reads ***, whatever its name or its place. Numbers, identifiers
//     and unquoted text stay.
//   - Signature, any other dialect: withheld (empty) unless the symbol's kind is
//     on structuralKinds, the list of kinds whose Signature is structure and holds
//     no value. The default is withheld, so a kind nobody listed fails closed.
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
var slashesRe = regexp.MustCompile(`(^|[^A-Za-z0-9+.\-:/])(//)([^/\\?#\s"'<>]*)@`)

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

// maskLiterals replaces what is between the quotes of every quoted literal in s
// with ***: double, single and backtick quotes, the triple quotes of Python, the
// one- or two-letter prefix of a literal (b, f, r, rb) staying where it is. An
// escape (a backslash) does not end a double or single quoted literal, and a
// literal that is never closed is masked to the end of the text, so a missing
// quote withholds more and never less. A single quote that skip says opens no
// literal (a Rust lifetime, an apostrophe in a sentence) is left as written.
func maskLiterals(s string, skip func(s string, i int) bool) string {
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
	// A quoted punctuation mark is the grammar the parser expected (`expected ',' or
	// ']'`), never a word of the file, and stays.
	s = punctuationQuoteRe.ReplaceAllStringFunc(s, func(m string) string {
		if m[0] == '\'' {
			return "\x01" + m[1:2] + "\x01"
		}
		return "\x02" + m[1:2] + "\x02"
	})
	s = maskLiterals(s, apostrophe)
	s = strings.NewReplacer("\x01", "'", "\x02", "\"").Replace(s)
	return noteTokenRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := noteTokenRe.FindStringSubmatch(m)
		if sub[1] == "EOF" || numeric(sub[1]) {
			return m
		}
		return ":" + sub[2]
	})
}

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
	var skip func(string, int) bool
	if res.Dialect == "rust" {
		skip = rustLifetime
	}
	for i := range res.Symbols {
		s := &res.Symbols[i]
		s.Name, s.Key, s.Package = withholdURLUserinfo(s.Name), withholdURLUserinfo(s.Key), withholdURLUserinfo(s.Package)
		switch {
		case code:
			s.Signature = withholdURLUserinfo(maskLiterals(s.Signature, skip))
		case structuralKind(s.Kind):
			s.Signature = withholdURLUserinfo(s.Signature)
		default:
			s.Signature = ""
		}
	}
	for i := range res.References {
		r := &res.References[i]
		r.From, r.To, r.Raw = withholdURLUserinfo(r.From), withholdURLUserinfo(r.To), withholdURLUserinfo(r.Raw)
	}
	for i := range res.Unresolved {
		u := &res.Unresolved[i]
		u.From, u.Raw = withholdURLUserinfo(u.From), withholdURLUserinfo(u.Raw)
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
