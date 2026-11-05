package lang

// redact.go -- the door withholds secret-bearing values (design section 1, ADR-069
// decision 2): a Result never carries the credential a URL, a command line or a
// default value holds. The extractors read a file's text into names, signatures
// and reference targets, so the withholding is one pass over the finished Result
// and no extractor has to remember it. It is a pattern list and says so (see the
// package documentation): what it recognises is withheld, and a credential in a
// shape it does not recognise is not promised withheld.

import (
	"regexp"
	"strings"
	"unicode"
)

// redacted replaces a withheld literal.
const redacted = "***"

// userinfoRe finds a URL's authority part up to the last @ before the path:
// scheme, one or two slashes (a path join can collapse the pair), then the
// userinfo. The scheme prefix of `git::https://` and `git+https://` is part of
// the match's scheme or sits before it.
var userinfoRe = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.\-]*):(//?)([^/\s"'<>]*)@`)

// withholdURLUserinfo drops the userinfo of every URL in s that has a password
// or a token in it. An ssh user (`ssh://git@host`) names an account, not a
// secret, and stays.
func withholdURLUserinfo(s string) string {
	if !strings.Contains(s, "@") {
		return s
	}
	return userinfoRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := userinfoRe.FindStringSubmatch(m)
		scheme, slashes, userinfo := sub[1], sub[2], sub[3]
		if !strings.Contains(userinfo, ":") && sshScheme(scheme) {
			return m
		}
		return scheme + ":" + slashes
	})
}

func sshScheme(scheme string) bool {
	s := strings.ToLower(scheme)
	return s == "ssh" || strings.HasSuffix(s, "+ssh") || s == "git"
}

// literalRe finds a name bound to a string literal: `name = "x"`, `name := "x"`,
// `name: "x"`, and a typed parameter `name: str = "x"`.
var literalRe = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)((?:\s*:\s*[A-Za-z_][\w\[\]., |]*)?\s*(?::=|=)\s*|\s*:\s*)("(?:[^"\\]|\\.)+"|'(?:[^'\\]|\\.)+')`)

// secretWords are the words of a name that say it holds a credential.
var secretWords = map[string]bool{
	"pass": true, "password": true, "passwd": true, "pwd": true, "pw": true,
	"secret": true, "secrets": true, "token": true, "tokens": true, "apikey": true,
	"credential": true, "credentials": true, "auth": true, "authorization": true,
	"bearer": true, "passphrase": true,
}

// secretName reports whether an identifier names a credential: one of its
// words (split at `_`, `-` and a lower-to-upper change) is a secret word, or
// two adjacent words are api key, private key, access key or secret key.
func secretName(id string) bool {
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	prev := rune(0)
	for _, r := range id {
		switch {
		case r == '_' || r == '-':
			flush()
		case unicode.IsUpper(r) && unicode.IsLower(prev):
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
		prev = r
	}
	flush()
	for i, w := range words {
		if secretWords[w] {
			return true
		}
		if i > 0 && w == "key" {
			switch words[i-1] {
			case "api", "private", "access", "secret", "auth":
				return true
			}
		}
	}
	return false
}

// cliSecretRe finds the value of a command-line flag that carries a credential
// (`--password x`, `--token=x`, `-u user:pass`) and of an Authorization header.
var cliSecretRe = regexp.MustCompile(
	`(?i)(--?(?:password|passwd|pass|token|api-?key|secret|auth(?:orization)?|bearer|access-?key)(?:\s+|=))([^\s'"$%-][^\s'"]*)` +
		`|(\s-u\s+)([^\s'"$%]+:[^\s'"]+)` +
		`|(authorization:\s*(?:bearer|basic|token)?\s*)([^\s'"$]+)`)

// withholdSecrets masks, in a Signature (a declaration's or a command's text),
// every string literal bound to a name that says it is a credential, the value
// of a credential flag, and an Authorization header's value.
func withholdSecrets(s string) string {
	s = withholdURLUserinfo(s)
	s = literalRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := literalRe.FindStringSubmatch(m)
		if !secretName(sub[1]) {
			return m
		}
		lit := sub[3]
		return m[:len(m)-len(lit)] + lit[:1] + redacted + lit[len(lit)-1:]
	})
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

// withhold applies the door's withholding to a finished Result in place:
// credentials in a URL everywhere a string from the file lands, and credential
// literals and flags in a Signature.
func (res *Result) withhold() {
	for i := range res.Symbols {
		s := &res.Symbols[i]
		s.Name, s.Key = withholdURLUserinfo(s.Name), withholdURLUserinfo(s.Key)
		s.Signature = withholdSecrets(s.Signature)
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
		f.Entity, f.Value = withholdURLUserinfo(f.Entity), withholdSecrets(f.Value)
	}
}
