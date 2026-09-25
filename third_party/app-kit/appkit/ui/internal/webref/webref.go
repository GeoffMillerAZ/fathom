// Package webref checks the destination shape shared by managed table and form
// links. It does not establish origin trust, route existence or authorization.
package webref

import (
	"net/url"
	"strings"
)

// Allowed preserves relative web references and absolute HTTP(S) destinations.
// It rejects rather than normalizes ambiguous, malformed or non-web input.
func Allowed(href string) bool {
	if href == "" || strings.TrimSpace(href) != href || strings.Contains(href, `\`) || strings.HasPrefix(href, "//") {
		return false
	}
	for _, r := range href {
		if r < ' ' || r == 0x7f {
			return false
		}
	}
	u, err := url.Parse(href)
	if err != nil {
		return false
	}
	// Parse leaves RawQuery undecoded; reject malformed escapes there too.
	if _, err := url.QueryUnescape(u.RawQuery); err != nil {
		return false
	}
	if u.Scheme == "" {
		return u.Host == "" && u.Opaque == ""
	}
	return (strings.EqualFold(u.Scheme, "https") || strings.EqualFold(u.Scheme, "http")) && u.Hostname() != "" && u.Opaque == ""
}
