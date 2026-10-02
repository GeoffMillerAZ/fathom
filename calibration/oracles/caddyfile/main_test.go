package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestComposeProjectRule (LB-F169): a compose project is a directory, a route
// joins the nearest project that declares its host, one pair per compose file
// of that directory that declares the service.
func TestComposeProjectRule(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"docker-compose.yml", "docker-compose.override.yml", "docker-compose.prod.yml", "app/compose.yaml", "app/conf/x.cfg", "other/notes.yml", "caddy/c.cfg"} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	names := map[string]map[string][]string{
		"docker-compose.yml":          {"web": {"web"}, "db": {"db"}},
		"docker-compose.override.yml": {"web": {"web"}},
		"docker-compose.prod.yml":     {"web": {"web"}, "cache": {"cache"}},
		"app/compose.yaml":            {"web": {"web"}},
	}
	of := func(c string) map[string][]string { return names[c] }
	lookup := func(rel string) func(string) []composeHit {
		return func(h string) []composeHit { return resolveCompose(root, rel, h, of) }
	}
	if got := composeFilesIn(root, ""); strings.Join(got, ",") != "docker-compose.override.yml,docker-compose.prod.yml,docker-compose.yml" {
		t.Errorf("composeFilesIn(root) = %v", got)
	}
	if got := composeFilesIn(root, "other"); len(got) != 0 {
		t.Errorf("composeFilesIn(other) = %v", got)
	}
	for _, c := range []struct {
		raw, rel, svc, files, reason string
	}{
		{"web:80", "caddy/c.cfg", "web", "docker-compose.override.yml,docker-compose.prod.yml,docker-compose.yml", ""},
		{"db:5432", "caddy/c.cfg", "db", "docker-compose.yml", ""},
		{"web:80", "app/conf/x.cfg", "web", "app/compose.yaml", ""},
		{"db:5432", "app/conf/x.cfg", "db", "docker-compose.yml", ""},
		{"nothing:80", "caddy/c.cfg", "", "", "undeclared"},
		{"api.example.com:443", "caddy/c.cfg", "", "", "external"},
	} {
		svc, files, reason := classify(c.raw, lookup(c.rel))
		if svc != c.svc || strings.Join(files, ",") != c.files || reason != c.reason {
			t.Errorf("classify(%q) from %s = %q %v %q; want %q %q %q", c.raw, c.rel, svc, files, reason, c.svc, c.files, c.reason)
		}
	}
	names["docker-compose.yml"] = map[string][]string{"alias": {"a", "b"}}
	if _, _, reason := classify("alias:1", lookup("caddy/c.cfg")); reason != "ambiguous" {
		t.Errorf("one name for two services: reason %q, want ambiguous", reason)
	}
}
