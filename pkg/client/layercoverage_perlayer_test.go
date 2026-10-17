package client_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/client"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// TestLayerCoverage_AdrRulesDesignSectionStale is O1122 end to end through
// pkg/client: a docs/design section is in the adr-rules corpus and not the
// obligations corpus, so after the section changes only the adr-rules
// layer's coverage can call it stale — the coverage the obligations serve
// path must read for an adr-rules claim.
func TestLayerCoverage_AdrRulesDesignSectionStale(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	design := filepath.Join(dir, "docs", "design", "x.md")
	if err := os.MkdirAll(filepath.Dir(design), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(design, []byte("# Boundary\n\n- pkg/** never imports cmd/**.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Rules\n\n- Run tests before committing.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "foo.go"), []byte("package pkg\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}
	c.WithLLM(llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"pkg never imports cmd.","scope":["pkg/**"],"cite_start":3,"cite_end":3}]}`,
	}}), "mock-model", 0, 0)
	if run, err := c.CompileLayer(ctx, "svc", "adr-rules", false); err != nil || run.SectionsCompiled == 0 {
		t.Fatalf("compile adr-rules: run=%+v err=%v", run, err)
	}
	if cov, found, err := c.LayerCoverage(ctx, "svc", "adr-rules"); err != nil || !found || len(cov.StaleSubjects()) != 0 {
		t.Fatalf("fresh adr-rules coverage: cov=%+v found=%v err=%v, want found with nothing stale", cov, found, err)
	}

	if err := os.WriteFile(design, []byte("# Boundary\n\n- pkg/** may import cmd/** now.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cov, found, err := c.LayerCoverage(ctx, "svc", "adr-rules")
	if err != nil || !found {
		t.Fatalf("adr-rules coverage: found=%v err=%v", found, err)
	}
	var stale []string
	for subj := range cov.StaleSubjects() {
		stale = append(stale, subj)
	}
	if len(stale) != 1 || !strings.HasPrefix(stale[0], "docs/design/x.md") {
		t.Fatalf("adr-rules stale subjects = %v, want the changed docs/design/x.md section (cov %+v)", stale, cov)
	}
	oblCov, _, err := c.ObligationsLayerCoverage(ctx, "svc")
	if err != nil {
		t.Fatal(err)
	}
	for subj := range oblCov.StaleSubjects() {
		if strings.HasPrefix(subj, "docs/design/") {
			t.Fatalf("obligations coverage names a docs/design subject %q: it is not in that layer's corpus", subj)
		}
	}
}
