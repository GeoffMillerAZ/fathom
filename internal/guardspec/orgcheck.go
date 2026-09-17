// orgcheck.go — the ADR-011 / I19 org-agnostic-content scanner, lifted out
// of cmd/fathom/orgnames_test.go (ADR-038 item 2) so the enforced "org_token"
// guard check (enforce.go) and TestNoOrgSpecificNames share exactly one
// rule set: a hook denying an edit and a gate test rejecting the same
// commit must never be able to disagree about what counts as org-specific.
//
// The denylist is itself org-agnostic by construction (see
// cmd/fathom/orgnames_test.go's own doc comment) — it scans for the SHAPES
// org content takes (an internal-TLD host, a non-neutral email domain, a
// real cloud account or bucket), not for any literal name.
package guardspec

import (
	"regexp"
	"strings"
)

// OrgInternalHostSuffixes are the private-network TLDs a host is denied
// under — the one list OrgHits is built from, and what
// TestADR011EnforcedPatternsMatchOrgNames checks ADR-011's own served text
// against.
var OrgInternalHostSuffixes = []string{"corp", "internal", "intranet", "lan"}

var (
	// orgInternalHostRe: a name under one of OrgInternalHostSuffixes.
	orgInternalHostRe = regexp.MustCompile(`(?i)\b[a-z0-9][a-z0-9-]*(?:\.[a-z0-9-]+)*\.(?:` + strings.Join(OrgInternalHostSuffixes, "|") + `)\b`)
	// orgEmailRe: an email address; the domain is checked against the
	// neutral set (OrgNeutralEmailDomain).
	orgEmailRe = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@((?:[A-Za-z0-9-]+\.)+[A-Za-z]{2,})\b`)
	// orgARNRe: a cloud account in an ARN (12 digits) other than a
	// documented placeholder account.
	orgARNRe = regexp.MustCompile(`arn:aws:[a-z0-9-]*:[a-z0-9-]*:(\d{12}):`)
	// orgBucketRe: a storage bucket URI.
	orgBucketRe = regexp.MustCompile(`\b(?:gs|s3)://([a-z0-9][a-z0-9._-]{2,})`)
)

// OrgNeutralEmailDomain reports whether an email domain is a documented
// placeholder (RFC 2606/6761 names) or tooling identity, not an org.
func OrgNeutralEmailDomain(d string) bool {
	d = strings.ToLower(d)
	switch {
	case d == "example.com", d == "example.net", d == "example.org",
		strings.HasSuffix(d, ".example.com"), strings.HasSuffix(d, ".example.net"), strings.HasSuffix(d, ".example.org"),
		strings.HasSuffix(d, ".example"), strings.HasSuffix(d, ".test"), strings.HasSuffix(d, ".invalid"), strings.HasSuffix(d, ".localhost"),
		d == "localhost", d == "github.com", d == "anthropic.com":
		return true
	}
	return false
}

// OrgPlaceholderAccount reports whether acct is a documented AWS
// placeholder account id.
func OrgPlaceholderAccount(acct string) bool {
	switch acct {
	case "123456789012", "000000000000", "111111111111", "999999999999":
		return true
	}
	return false
}

// OrgNeutralBucket reports whether a bucket name reads as a placeholder
// rather than a real org's storage bucket.
func OrgNeutralBucket(b string) bool {
	b = strings.ToLower(b)
	return strings.Contains(b, "example") || strings.HasPrefix(b, "my-") || strings.HasPrefix(b, "bucket") || strings.Contains(b, "fauxtown")
}

// OrgHits scans one line and returns the org-shaped tokens on it: an
// internal-TLD host, a non-neutral email, a real AWS account in an ARN, a
// non-neutral bucket URI, or any denylist token (case-insensitive
// substring match — the caller's own FATHOM_ORG_DENYLIST, lower-cased).
func OrgHits(line string, denylist []string) []string {
	var hits []string
	lower := strings.ToLower(line)
	// Cheap substring gates before each regexp: a caller scanning every
	// tracked line of a repo needs this to stay well under a second.
	if strings.Contains(lower, ".corp") || strings.Contains(lower, ".internal") || strings.Contains(lower, ".intranet") || strings.Contains(lower, ".lan") {
		hits = append(hits, orgInternalHostRe.FindAllString(line, -1)...)
	}
	if strings.Contains(line, "@") {
		for _, m := range orgEmailRe.FindAllStringSubmatch(line, -1) {
			if !OrgNeutralEmailDomain(m[1]) && !strings.HasPrefix(strings.ToLower(m[0]), "noreply@") {
				hits = append(hits, m[0])
			}
		}
	}
	if strings.Contains(line, "arn:aws") {
		for _, m := range orgARNRe.FindAllStringSubmatch(line, -1) {
			if !OrgPlaceholderAccount(m[1]) {
				hits = append(hits, m[0])
			}
		}
	}
	if strings.Contains(line, "://") {
		for _, m := range orgBucketRe.FindAllStringSubmatch(line, -1) {
			if !OrgNeutralBucket(m[1]) {
				hits = append(hits, m[0])
			}
		}
	}
	for _, tok := range denylist {
		if tok != "" && strings.Contains(lower, tok) {
			hits = append(hits, tok)
		}
	}
	return hits
}
