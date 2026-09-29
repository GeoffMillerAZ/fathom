package client_test

// obligations_agentsmd_test.go — AG-17 (docs/plans/agents-md.md, ADR-059):
// an edit under a directory whose governing guide is a nested AGENTS.md is
// served that file's rule raw before any compile, including a guide written
// after the last ingest (probeAncestorGuides) and a root `.claude/AGENTS.md`
// the connector never indexes.

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestProbeAncestorGuidesFindsNestedAgentsMD(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"AGENTS.md":             "# Repo\n\n## Rules\n\n- Keep the gate green.\n",
		"internal/AGENTS.md":    "# internal\n\n## Determinism\n\n- Sort all outputs before returning them.\n",
		"internal/a.go":         "package internal\n\nfunc A() {}\n",
		"internal/sub/b.go":     "package sub\n\nfunc B() {}\n",
		".claude/settings.json": "{}\n",
	})
	c := client.New(connector.NewLocalConnector(), store.NewMemoryStore())
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal(err)
	}

	srcs := func(file string) string {
		t.Helper()
		rows, _, ok, err := c.RawFallbackObligations(ctx, "svc", file, 10)
		if err != nil || !ok {
			t.Fatalf("%s: rows=%+v ok=%v err=%v", file, rows, ok, err)
		}
		var out []string
		for _, r := range rows {
			out = append(out, r.Src)
		}
		return strings.Join(out, " ")
	}
	if got := srcs("internal/a.go"); !strings.Contains(got, "internal/AGENTS.md#") {
		t.Errorf("an edit under internal/ was not served internal/AGENTS.md's rule: %s", got)
	}

	// A guide written after the last ingest, and a root .claude/AGENTS.md.
	writeTree(t, dir, map[string]string{
		"internal/sub/AGENTS.md": "# sub\n\n## Sub rule\n\n- Never write outside workspace/.\n",
		".claude/AGENTS.md":      "# claude-dir guide\n\n## Root rule\n\n- Always name the plan row.\n",
	})
	got := srcs("internal/sub/b.go")
	if !strings.Contains(got, "internal/sub/AGENTS.md#") {
		t.Errorf("a nested AGENTS.md written since the ingest was not served: %s", got)
	}
	if !strings.Contains(got, ".claude/AGENTS.md#") {
		t.Errorf("the root guide under .claude/ was not served: %s", got)
	}
}
