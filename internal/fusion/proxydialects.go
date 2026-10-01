package fusion

// proxydialects.go — the classification rules sniffedExtractor's cheap first
// test reads for the native proxy dialects (LB-Q-nginx, LB-Q-haproxy,
// LB-Q-caddyfile; LB-F101). registerProxyDialects (register.go) wires their
// extractors.

import (
	"sync"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

var (
	proxyRulesOnce sync.Once
	proxyRules     []connector.DialectRule
)

// proxyDialectRules is the native proxy dialects' manifest rules (a subset
// of dialectRules): a file none of them claims by name, glob or extension,
// and whose extension no sniff covers, skips the full classification.
func proxyDialectRules() []connector.DialectRule {
	proxyRulesOnce.Do(func() {
		for _, r := range dialectRules() {
			if _, ok := extractor.ProxyDialectExtractor(r.ID); ok {
				proxyRules = append(proxyRules, r)
			}
		}
	})
	return proxyRules
}
