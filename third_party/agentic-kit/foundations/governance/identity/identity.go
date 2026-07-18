package identity

import (
	"crypto/sha256"
	"fmt"
	"net"
	"strings"
)

// HashID returns a salted SHA-256 hex digest of raw. The salt is prepended
// before hashing so that outputs are not comparable across deployments using
// different salts. Raw is never stored or returned.
func HashID(salt, raw string) string {
	h := sha256.Sum256([]byte(salt + ":" + raw))
	return fmt.Sprintf("%x", h)
}

// Extract derives a privacy-preserving identifier for an HTTP request.
//
// It selects the source IP by:
//  1. If cfg.TrustedProxyHops == 0 or xff is blank, use remoteAddr.
//  2. Otherwise parse the comma-separated XFF header from right to left,
//     skip cfg.TrustedProxyHops entries (trusted infrastructure), and take
//     the next entry as the first untrusted hop. If there are fewer entries
//     than hops to skip, fall back to the leftmost XFF entry.
//
// The selected address has its port stripped (net.SplitHostPort or bracket
// removal for bare IPv6), then HashID is applied with cfg.Salt.
func Extract(cfg Config, xff, remoteAddr string) string {
	raw := selectIP(cfg, xff, remoteAddr)
	return HashID(cfg.Salt, raw)
}

// selectIP returns the raw IP string (no port) for the request.
func selectIP(cfg Config, xff, remoteAddr string) string {
	if cfg.TrustedProxyHops > 0 {
		if ip, ok := firstUntrustedXFF(xff, cfg.TrustedProxyHops); ok {
			return ip
		}
	}
	return stripPort(strings.TrimSpace(remoteAddr))
}

// firstUntrustedXFF parses a comma-separated XFF header and returns the first
// untrusted hop. Returns ("", false) if xff is blank or contains only
// whitespace after trimming.
func firstUntrustedXFF(xff string, trustedHops int) (string, bool) {
	xff = strings.TrimSpace(xff)
	if xff == "" {
		return "", false
	}

	parts := strings.Split(xff, ",")
	// Trim whitespace from each part.
	entries := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			entries = append(entries, trimmed)
		}
	}
	if len(entries) == 0 {
		return "", false
	}

	// Skip trustedHops entries from the right; clamp to leftmost.
	idx := len(entries) - 1 - trustedHops
	if idx < 0 {
		idx = 0
	}
	return stripPort(entries[idx]), true
}

// stripPort removes the port from a host:port or [ipv6]:port string.
// If addr has no port, it is returned unchanged (with bracket stripping for IPv6).
func stripPort(addr string) string {
	// Try net.SplitHostPort — handles "host:port" and "[::1]:port".
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	// No port present. Strip brackets from bare IPv6 like "[::1]".
	if strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]") {
		return addr[1 : len(addr)-1]
	}
	return addr
}
