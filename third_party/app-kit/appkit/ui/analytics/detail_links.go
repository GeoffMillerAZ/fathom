package analytics

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

// validateDetailHref admits only app-owned, root-relative detail destinations.
// It deliberately performs no normalization, fetching, authorization, or host
// policy; those remain the adopting application's responsibility.
func validateDetailHref(href string) error {
	if href == "" {
		return nil
	}
	if err := checkText("detail href", href, true); err != nil {
		return err
	}
	for _, r := range href {
		if r == '\\' || unicode.IsControl(r) || r == ' ' {
			return fmt.Errorf("analytics: detail href contains whitespace or control characters")
		}
	}
	if !strings.HasPrefix(href, "/") || strings.HasPrefix(href, "//") {
		return fmt.Errorf("analytics: detail href must be a single root-relative path")
	}
	u, err := url.Parse(href)
	if err != nil || u.Host != "" || u.User != nil || u.Opaque != "" || u.Scheme != "" {
		return fmt.Errorf("analytics: detail href must be a valid root-relative URL")
	}
	decoded, err := url.PathUnescape(href)
	if err != nil {
		return fmt.Errorf("analytics: detail href contains malformed escaping")
	}
	for _, r := range decoded {
		if r == '\\' || unicode.IsControl(r) {
			return fmt.Errorf("analytics: detail href contains a decoded control character")
		}
	}
	// Parse already decoded the path once. Do not recursively decode escapes.
	if strings.HasPrefix(u.Path, "//") {
		return fmt.Errorf("analytics: detail href path cannot decode to a protocol-relative path")
	}
	return nil
}
