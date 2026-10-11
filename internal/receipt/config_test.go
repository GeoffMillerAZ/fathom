package receipt

// config_test.go — the configured repository URI ([receipts]
// repository_uri) reaches a SARIF run's provenance; fathom never derives
// one from a git remote (ADR-011).

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

func TestConfiguredRepositoryURI(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir())
	r := newFixtureRepo(t)
	r.write("a.md", "one\n")
	r.git("remote", "add", "origin", "https://example.invalid/never-derived.git")
	r.commitAll("a")
	if got := ConfiguredRepositoryURI(r.dir); got != "" {
		t.Fatalf("no [receipts] key: got %q, want none (never derived from a remote)", got)
	}
	prov, _ := AnchorRun(context.Background(), Run{Dir: r.dir, RepositoryURIAt: ConfiguredRepositoryURI}, nil)
	if prov.RepositoryURI != "" {
		t.Fatalf("unconfigured run names %q", prov.RepositoryURI)
	}

	r.write(".fathom/config.toml", "[receipts]\nrepository_uri = \"https://example.invalid/acme/widgets.git\"\n")
	if got := ConfiguredRepositoryURI(r.dir); got != "https://example.invalid/acme/widgets.git" {
		t.Fatalf("configured: got %q", got)
	}
	prov, _ = AnchorRun(context.Background(), Run{Dir: r.dir, RepositoryURIAt: ConfiguredRepositoryURI}, []verify.SARIFResult{{RuleID: "x", File: "a.md", Line: 1}})
	if prov.RepositoryURI != "https://example.invalid/acme/widgets.git" || prov.Commit == "" {
		t.Fatalf("configured run: %+v", prov)
	}

	r.write(".fathom/config.toml", "[receipts]\nrepository_uri = \"no-scheme\"\n")
	if got := ConfiguredRepositoryURI(r.dir); got != "" {
		t.Fatalf("a refused config names %q, want none", got)
	}
}
