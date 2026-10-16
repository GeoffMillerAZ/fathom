package identity_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/identity"
)

// ---- HashID -----------------------------------------------------------------

func TestHashID_Stable(t *testing.T) {
	h1 := identity.HashID("salt1", "user@example.com")
	h2 := identity.HashID("salt1", "user@example.com")
	if h1 != h2 {
		t.Errorf("HashID must be stable: got %q then %q", h1, h2)
	}
}

func TestHashID_Salted(t *testing.T) {
	h1 := identity.HashID("salt1", "user@example.com")
	h2 := identity.HashID("salt2", "user@example.com")
	if h1 == h2 {
		t.Error("HashID with different salts must produce different output")
	}
}

func TestHashID_NotEqualToRaw(t *testing.T) {
	raw := "user@example.com"
	h := identity.HashID("salt", raw)
	if h == raw {
		t.Error("HashID output must not equal the raw input")
	}
}

func TestHashID_IsHex(t *testing.T) {
	h := identity.HashID("salt", "raw")
	// SHA-256 hex is 64 chars
	if len(h) != 64 {
		t.Errorf("expected 64-char hex, got len %d: %q", len(h), h)
	}
	for _, c := range h {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("non-hex character %q in HashID output %q", c, h)
		}
	}
}

func TestHashID_EmptySalt(t *testing.T) {
	// Even empty salt must produce a hex string different from raw.
	h := identity.HashID("", "abc")
	if h == "abc" {
		t.Error("HashID with empty salt must not return raw")
	}
	if len(h) != 64 {
		t.Errorf("expected 64-char hex, got %d", len(h))
	}
}

func TestHashID_DifferentRaw(t *testing.T) {
	h1 := identity.HashID("salt", "user1")
	h2 := identity.HashID("salt", "user2")
	if h1 == h2 {
		t.Error("different raw inputs must produce different hashes")
	}
}

// ---- Extract ----------------------------------------------------------------

func TestExtract_RemoteAddrNoXFF(t *testing.T) {
	cfg := identity.Config{Salt: "s", TrustedProxyHops: 0}
	got := identity.Extract(cfg, "", "203.0.113.7:12345")
	want := identity.HashID("s", "203.0.113.7")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtract_RemoteAddrStripPort(t *testing.T) {
	cfg := identity.Config{Salt: "s", TrustedProxyHops: 0}
	// Without port in remoteAddr
	got1 := identity.Extract(cfg, "", "203.0.113.7:9999")
	got2 := identity.Extract(cfg, "", "203.0.113.7:8888")
	// Both should hash the same IP (port stripped)
	if got1 != got2 {
		t.Errorf("port stripping failed: %q vs %q", got1, got2)
	}
}

func TestExtract_XFF_ZeroHops(t *testing.T) {
	// TrustedProxyHops=0 means no proxies trusted; use remoteAddr.
	cfg := identity.Config{Salt: "s", TrustedProxyHops: 0}
	got := identity.Extract(cfg, "1.2.3.4, 5.6.7.8", "9.10.11.12:80")
	want := identity.HashID("s", "9.10.11.12")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtract_XFF_OneHop(t *testing.T) {
	// TrustedProxyHops=1 means the rightmost entry in XFF is our trusted proxy;
	// the first untrusted hop is the one just to its left (or the leftmost if single).
	// XFF: "client, proxy1" — trust 1 hop → first untrusted is "client".
	cfg := identity.Config{Salt: "s", TrustedProxyHops: 1}
	got := identity.Extract(cfg, "1.2.3.4, 5.6.7.8", "9.10.11.12:80")
	// 1 trusted hop from right: rightmost trusted hop is 5.6.7.8; first untrusted = 1.2.3.4
	want := identity.HashID("s", "1.2.3.4")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtract_XFF_TwoHops(t *testing.T) {
	// XFF: "client, proxy1, proxy2" — trust 2 hops → first untrusted is "client".
	cfg := identity.Config{Salt: "s", TrustedProxyHops: 2}
	got := identity.Extract(cfg, "10.0.0.1, 10.0.0.2, 10.0.0.3", "10.0.0.4:443")
	want := identity.HashID("s", "10.0.0.1")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtract_XFF_MoreHopsThanEntries(t *testing.T) {
	// If TrustedProxyHops > entries in XFF, fall back to leftmost XFF entry.
	cfg := identity.Config{Salt: "s", TrustedProxyHops: 10}
	got := identity.Extract(cfg, "1.2.3.4", "9.10.11.12:80")
	want := identity.HashID("s", "1.2.3.4")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtract_XFF_EmptyXFF_FallbackRemoteAddr(t *testing.T) {
	cfg := identity.Config{Salt: "s", TrustedProxyHops: 1}
	// XFF is empty (whitespace only) — fall back to remoteAddr.
	got := identity.Extract(cfg, "  ", "9.10.11.12:80")
	want := identity.HashID("s", "9.10.11.12")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtract_XFF_StripSpaces(t *testing.T) {
	cfg := identity.Config{Salt: "s", TrustedProxyHops: 1}
	// Entries may have surrounding spaces.
	got := identity.Extract(cfg, "  1.2.3.4 ,  5.6.7.8 ", "9.10.11.12:80")
	want := identity.HashID("s", "1.2.3.4")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtract_IPv6_StripPort(t *testing.T) {
	cfg := identity.Config{Salt: "s", TrustedProxyHops: 0}
	// IPv6 addresses in brackets with port, as Go net.JoinHostPort produces.
	got := identity.Extract(cfg, "", "[::1]:1234")
	want := identity.HashID("s", "::1")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtract_Determinism(t *testing.T) {
	cfg := identity.Config{Salt: "testsalt", TrustedProxyHops: 1}
	xff := "192.168.1.1, 10.0.0.1"
	remote := "10.0.0.1:443"
	h1 := identity.Extract(cfg, xff, remote)
	h2 := identity.Extract(cfg, xff, remote)
	if h1 != h2 {
		t.Error("Extract is not deterministic")
	}
}
