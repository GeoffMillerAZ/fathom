package connector_test

// connector_test.go exercises the public surface of the connector package
// through the Connector interface — not through concrete types — so that any
// compliant implementation can satisfy the same suite.
//
// Tests in this file are interface-contract tests; implementation-specific
// tests live in local_test.go and faux_test.go.

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/connector"
	"github.com/geoffmilleraz/sextant/internal/core"
)

// connectorContract runs the shared behavioural contract for any Connector.
// wantPaths may be bare filenames (e.g. "payment.go") or repo-relative paths
// (e.g. "payment-service/payment.go"). A SourceRef.Path is considered a match
// when its cleaned path ends with a cleaned wantPath segment.
func connectorContract(t *testing.T, c connector.Connector, svc core.ServiceRef, wantPaths []string) {
	t.Helper()
	ctx := context.Background()

	refs, err := c.Discover(ctx, svc)
	if err != nil {
		t.Fatalf("Discover: unexpected error: %v", err)
	}

	// Build a set of discovered paths and their base names.
	gotPaths := make(map[string]bool, len(refs))
	gotBases := make(map[string]bool, len(refs))
	for _, r := range refs {
		cleaned := filepath.ToSlash(filepath.Clean(r.Path))
		gotPaths[cleaned] = true
		gotBases[filepath.Base(r.Path)] = true
		if r.Service != svc.ID {
			t.Errorf("SourceRef.Service = %q, want %q", r.Service, svc.ID)
		}
	}

	// Accept a wantPath if any discovered path ends with the wantPath suffix
	// (handles both bare names and relative sub-paths).
	pathMatch := func(want string) bool {
		want = filepath.ToSlash(filepath.Clean(want))
		if gotPaths[want] {
			return true
		}
		// bare filename match
		if gotBases[filepath.Base(want)] {
			return true
		}
		// suffix match for repo-relative paths
		suffix := "/" + strings.TrimPrefix(want, "/")
		for p := range gotPaths {
			if strings.HasSuffix("/"+p, suffix) {
				return true
			}
		}
		return false
	}

	for _, p := range wantPaths {
		if !pathMatch(p) {
			t.Errorf("Discover: missing expected path %q", p)
		}
	}

	// Fetch each discovered ref and verify non-empty bytes are returned.
	for _, ref := range refs {
		data, err := c.Fetch(ctx, ref)
		if err != nil {
			t.Errorf("Fetch(%q): unexpected error: %v", ref.Path, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("Fetch(%q): returned empty bytes", ref.Path)
		}
	}
}

// TestConnectorInterface ensures Connector is an interface (compile-time check).
func TestConnectorInterface(t *testing.T) {
	var _ connector.Connector = (*connector.LocalConnector)(nil)
	var _ connector.Connector = (*connector.FauxConnector)(nil)
	var _ connector.Connector = (*connector.GitHubConnector)(nil)
}
