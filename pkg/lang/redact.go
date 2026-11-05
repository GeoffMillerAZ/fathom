package lang

// redact.go -- the door withholds secret-bearing values (design section 1, ADR-069
// decision 2): a Result never carries the credential a URL, a command line or a
// default value holds. The extractors read a file's text into names, signatures
// and reference targets, so the withholding is one pass over the finished Result
// and no extractor has to remember it.
//
// What it withholds is chosen by where a value sits, not by what the value looks
// like, because a list of credential-shaped names and flags is never complete (a
// `dbpassword` default, `curl --user` and `mysql -pS` each read through the list
// that preceded this one): the userinfo of a URL, every string literal a
// signature binds to a name or a key where a value is not the point of the
// declaration (a parameter's default, a rule's value, a fact's value, a
// dictionary entry), and the text of a script, of which only the program is
// returned. What is in none of those places (a name, a label, a literal that is an
// argument, a block's own attributes) is returned as written, so a credential in
// a place the door does not look is not promised withheld.

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

// digestRe is what follows the @ of an image reference pinned by digest.
var digestRe = regexp.MustCompile(`^(?:sha256|sha384|sha512):[0-9A-Fa-f]+`)

// withholdURLUserinfo drops the userinfo of every URL in s that has a password
// or a token in it. An ssh user (`ssh://git@host`) names an account, not a
// secret, and stays, and so does the image of a `docker://` reference (a GitHub
// Actions `uses: docker://alpine@sha256:...`), where the @ is a digest pin.
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
	if last == 0 {
		return s
	}
	b.WriteString(s[last:])
	return b.String()
}

// keepUserinfo reports whether the userinfo before an @ is no credential: an ssh
// account (no password), or the name of an image in a docker reference, which has
// no colon, or has one and is followed by a digest (`docker://alpine:3.19@sha256:...`).
func keepUserinfo(scheme, userinfo, rest string) bool {
	colon := strings.Contains(userinfo, ":")
	switch {
	case sshScheme(scheme):
		return !colon
	case imageScheme(scheme):
		return !colon || digestRe.MatchString(rest)
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

// literalRe finds a string literal: double, single or triple quoted, a backtick
// template, and the one- or two-letter prefix of a Python or Rust literal (b, f,
// r, rb, u).
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
// key, whatever the name is: a default (`def f(dbpassword="x")`), a rule value
// (`adminpass := "x"`), an entry (`{"password": "x"}`) or a connection string.
// Unless whole is set, only a literal inside parentheses is masked (a parameter's
// default): a block's attribute (`module "m" { source = "x" }`) is the value the
// declaration is read for, and its credentials are the userinfo of its URLs.
func maskBoundLiterals(s string, whole bool) string {
	var b strings.Builder
	last, seen, depth := 0, 0, 0
	for _, ix := range literalRe.FindAllStringIndex(s, -1) {
		depth += strings.Count(s[seen:ix[0]], "(") - strings.Count(s[seen:ix[0]], ")")
		seen = ix[1]
		lit := s[ix[0]:ix[1]]
		if ix[0] > 0 && unicode.IsLetter(rune(s[ix[0]-1])) && unicode.IsLetter(rune(lit[0])) {
			continue // the letters belong to a longer word, not to a prefix
		}
		if !bound(s[:ix[0]]) || !(whole || depth > 0) {
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
// (`--password x`, `--token=x`, `-u user:pass`) and of an Authorization header.
// It covers what a Signature that is not a script holds (a task's text is not
// returned, see scriptProgram), so it is not where a script is made safe.
var cliSecretRe = regexp.MustCompile(
	`(?i)(--?(?:password|passwd|pass|token|api-?key|secret|auth(?:orization)?|bearer|access-?key)(?:\s+|=))([^\s'"$%-][^\s'"]*)` +
		`|(\s-u\s+)([^\s'"$%]+:[^\s'"]+)` +
		`|(authorization:\s*(?:bearer|basic|token)?\s*)([^\s'"$]+)`)

// withholdSecrets masks, in a Signature (a declaration's text) or a fact's value,
// the userinfo of a URL, the string literals bound to a name or a key (all of
// them when whole is set, as in a rule or a fact; those in parentheses otherwise,
// see maskBoundLiterals), the value of a credential flag, and an Authorization
// header's value.
func withholdSecrets(s string, whole bool) string {
	s = withholdURLUserinfo(s)
	s = maskBoundLiterals(s, whole)
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

// scriptProgram is the program a script's text runs first, and "" when that is
// not a plain name. A script (a package.json or composer.json script, a task's
// command) is free text in which a credential can sit in a flag, a header, an
// environment assignment or a password glued to its option (`mysql -pS`), in more
// shapes than a pattern covers, so the door returns the program and not the text.
// Leading `NAME=value` assignments are skipped, and are never returned.
func scriptProgram(s string) string {
	for _, w := range strings.Fields(s) {
		if i := strings.Index(w, "="); i > 0 && !strings.ContainsAny(w[:i], "/.") {
			continue // an environment assignment
		}
		for _, r := range w {
			if !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_-./", r)) {
				return ""
			}
		}
		return w
	}
	return ""
}

// withhold applies the door's withholding to a finished Result in place:
// credentials in a URL everywhere a string from the file lands, bound literals and
// credential flags in a Signature, and a script's text in place of its program.
func (res *Result) withhold() {
	for i := range res.Symbols {
		s := &res.Symbols[i]
		s.Name, s.Key = withholdURLUserinfo(s.Name), withholdURLUserinfo(s.Key)
		if s.Canonical == "task" {
			s.Signature = scriptProgram(s.Signature)
		} else {
			s.Signature = withholdSecrets(s.Signature, s.Canonical == "rule")
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
		f.Entity, f.Value = withholdURLUserinfo(f.Entity), withholdSecrets(f.Value, true)
	}
}
