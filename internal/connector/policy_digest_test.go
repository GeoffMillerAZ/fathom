package connector_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// These rules change without changing the discovered members. The policy
// receipt must still move because a later file could match the new rule.
func TestDiscoveryPolicyDigestGitIgnoreSources(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T, string) (core.ServiceRef, string)
	}{
		{"nested", func(t *testing.T, root string) (core.ServiceRef, string) {
			return core.ServiceRef{ID: "svc", Path: root}, filepath.Join(root, "pkg", ".gitignore")
		}},
		{"parent", func(t *testing.T, root string) (core.ServiceRef, string) {
			p := filepath.Join(root, "pkg", ".fathom", "config.toml")
			if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte("[connector]\nrespect_gitignore=true\n"), 0644); err != nil {
				t.Fatal(err)
			}
			return core.ServiceRef{ID: "svc", Path: filepath.Join(root, "pkg")}, filepath.Join(root, ".gitignore")
		}},
		{"global", func(t *testing.T, root string) (core.ServiceRef, string) {
			xdg := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", xdg)
			t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			p := filepath.Join(xdg, "git", "ignore")
			if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
				t.Fatal(err)
			}
			return core.ServiceRef{ID: "svc", Path: root}, p
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := gitRepo(t, "[connector]\nrespect_gitignore=true\n")
			svc, path := tc.setup(t, root)
			if err := os.WriteFile(path, []byte("never-one-*.generated\n"), 0644); err != nil {
				t.Fatal(err)
			}
			c := connector.NewLocalConnector()
			before, err := c.Discover(context.Background(), svc)
			if err != nil {
				t.Fatal(err)
			}
			d1, err := c.DiscoveryPolicyDigest(context.Background(), svc)
			if err != nil || d1 == "" {
				t.Fatalf("first digest=%q err=%v", d1, err)
			}
			if err := os.WriteFile(path, []byte("never-two-*.generated\n"), 0644); err != nil {
				t.Fatal(err)
			}
			after, err := c.Discover(context.Background(), svc)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("membership changed: before=%v after=%v", before, after)
			}
			d2, err := c.DiscoveryPolicyDigest(context.Background(), svc)
			if err != nil || d2 == d1 {
				t.Fatalf("policy drift undetected: before=%q after=%q err=%v", d1, d2, err)
			}
		})
	}
}
