package decide

// securitytemplate.go — I62's security clause (ADR-056 R7): a security
// property is never judged for serving. The request plan refuses, before
// any provider sees it, an ask whose site name or question template names
// one; the site answers on its deterministic path with one `security
// property` line. The template is the site's name and each question's
// instructions, rubric, levels and option names; spans, the state and
// criteria built from repository text are data, so a rule about
// authorization can still be ranked, and a path such as
// internal/auth/authorization.go names nothing.

import (
	"fmt"
	"path"
	"strings"
	"unicode"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// securityProperties are the words and phrases that name a security
// property, lower case, matched as whole words: authentication and
// authorization (the design's authorization chain), access control and
// privilege, injection and the web attack classes, sanitization,
// encryption, vulnerability and exploitability, and leaked secrets.
var securityProperties = []string{
	"security", "secure", "securely", "insecure",
	"vulnerability", "vulnerabilities", "vulnerable", "exploit", "exploits", "exploitable",
	"injection", "xss", "csrf", "ssrf",
	"authentication", "authenticate", "authenticated", "unauthenticated", "authn",
	"authorization", "authorisation", "authorize", "authorise", "authorized", "authorised",
	"unauthorized", "unauthorised", "authz",
	"access control", "privilege escalation",
	"sanitize", "sanitise", "sanitized", "sanitised", "sanitization", "sanitisation", "unsanitized",
	"encryption", "encrypt", "encrypted", "unencrypted", "cryptographic", "cryptography",
	"secret leak", "leaks a secret", "leaks secrets", "leaks credentials", "credential leak",
}

// refuseSecurityTemplate refuses req from site when the site's name or a
// question's template names a security property; nil otherwise.
func refuseSecurityTemplate(site string, req decision.Request) error {
	if term := namedSecurityProperty(site); term != "" {
		return &planRefusedError{class: classSecurity, reason: fmt.Sprintf("site %s names a security property (%s), and a security property is never judged for serving", site, term)}
	}
	for _, q := range req.Questions {
		parts := append([]string{q.Instructions, q.Rubric}, q.Levels...)
		for _, o := range q.Options {
			parts = append(parts, o.Name)
		}
		if term := namedSecurityProperty(strings.Join(parts, "\n")); term != "" {
			return &planRefusedError{class: classSecurity, reason: fmt.Sprintf("question %s names a security property (%s), and a security property is never judged for serving", q.ID, term)}
		}
	}
	return nil
}

// namedSecurityProperty is the first security property text names as
// words, "" when none. A path-shaped token (a slash, or a file extension)
// is skipped, and `_` separates words, so a site name like
// judge_access_control reads as its words.
func namedSecurityProperty(text string) string {
	var words []string
	for _, tok := range strings.Fields(text) {
		if pathShaped(tok) {
			continue
		}
		words = append(words, strings.FieldsFunc(strings.ToLower(tok), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})...)
	}
	if len(words) == 0 {
		return ""
	}
	joined := " " + strings.Join(words, " ") + " "
	for _, p := range securityProperties {
		if strings.Contains(joined, " "+p+" ") {
			return p
		}
	}
	return ""
}

// pathShaped reports whether tok reads as a path: it holds a slash, or,
// sentence punctuation trimmed, ends in a short file extension after a
// name (authorization.go, secure.md).
func pathShaped(tok string) bool {
	if strings.Contains(tok, "/") {
		return true
	}
	tok = strings.TrimRight(tok, ".,;:!?)\"'`")
	ext := path.Ext(tok)
	if len(ext) < 2 || len(ext) > 6 || len(ext) == len(tok) {
		return false
	}
	for _, r := range ext[1:] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
