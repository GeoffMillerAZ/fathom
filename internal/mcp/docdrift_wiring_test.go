// Package mcp_test — TestStalenessReportDocLinkDrift is KR-7's anchor
// through the real ingest path (docs/design/knowledge-retrieval.md §6):
// an ingest links a doc span to a symbol (KR-6), a commit changes the
// symbol, the re-ingest leaves the unchanged doc's link-time hash alone,
// and staleness_report's doc_link_drift names the span and the commit
// that changed it.
//
// TestStalenessReportDocLinkDriftShape pins the key's shape: a backend implementing
// the optional docDriftBackend capability (pkg/client.Client.DocLinkDrift,
// backed by docindex.StalenessDocDrift) populates staleness_report's
// doc_link_drift key -- additive to, never replacing, SR-28's own
// pre-existing doc_drift key (internal/analysis/docdrift's unrelated
// symbol/path-existence scan), which TestStalenessReportDocDrift
// (new_tools_test.go) already pins on its own shape.
package mcp_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/docindex"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestStalenessReportDocLinkDrift(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	// Both commits land in one second, as a fast ingest-commit-ingest does:
	// the changing commit must still be named over its same-dated parent.
	const sameSecond = "2026-09-29T12:00:00Z"
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+sameSecond, "GIT_COMMITTER_DATE="+sameSecond)
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out.String())
		}
		return strings.TrimSpace(out.String())
	}
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	git("config", "commit.gpgsign", "false")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.com")
	write("go.mod", "module example.com/drift\n\ngo 1.22\n")
	write("crank.go", "package drift\n\n// Frobnicate turns the crank once.\nfunc Frobnicate() int { return 1 }\n")
	write("docs/design.md", "# Design\n\n## Cranking\n\n`Frobnicate` turns the crank once per call.\n")
	git("add", "-A")
	git("commit", "-q", "-m", "initial")

	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "drift.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	c := client.New(connector.NewLocalConnector(), sq)
	t.Cleanup(func() { _ = c.Close() })
	ctx := context.Background()
	svc := client.ServiceRef{ID: "drift", Path: root}
	if _, err := c.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	srv := mcp.NewServer(c)
	payload := func() map[string]any {
		return callTool(t, srv, "staleness_report", map[string]any{"service": "drift"})
	}
	if rows, ok := payload()["doc_link_drift"].([]any); !ok || len(rows) != 0 {
		t.Fatalf("before any change doc_link_drift = %v, want present and empty (the link holds)", payload()["doc_link_drift"])
	}

	// The linked symbol changes in a commit; the doc does not.
	write("crank.go", "package drift\n\n// Frobnicate turns the crank twice.\nfunc Frobnicate() int { return 2 }\n")
	git("commit", "-q", "-am", "crank twice")
	changed := git("rev-parse", "HEAD")
	if _, err := c.Ingest(ctx, svc); err != nil {
		t.Fatalf("re-ingest: %v", err)
	}

	rows, ok := payload()["doc_link_drift"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("doc_link_drift = %v, want the one drifted span", payload()["doc_link_drift"])
	}
	row := rows[0].(map[string]any)
	if row["path"] != "docs/design.md" || row["target"] != "Frobnicate" || row["heading"] != "Design > Cranking" {
		t.Errorf("drifted span = %v, want docs/design.md's Cranking section naming Frobnicate", row)
	}
	if row["changed_commit"] != changed {
		t.Errorf("changed_commit = %v, want the commit that changed the symbol, %s", row["changed_commit"], changed)
	}
}

// docDriftFakeBackend adds the optional docLinkDrift capability
// (structural: internal/mcp's docDriftBackend interface is unexported, so
// this satisfies it by having the same method signature, the same way
// pkg/client.Client does for real) on top of the package's existing
// extendedFakeBackend.
type docDriftFakeBackend struct {
	extendedFakeBackend
	drifted []docindex.DriftedDocLink
}

func (d *docDriftFakeBackend) DocLinkDrift(_ context.Context, _ string) ([]docindex.DriftedDocLink, error) {
	return d.drifted, nil
}

var _ mcp.QueryInterface = (*docDriftFakeBackend)(nil)

func TestStalenessReportDocLinkDriftShape(t *testing.T) {
	srv := mcp.NewServer(&docDriftFakeBackend{
		extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}},
		drifted: []docindex.DriftedDocLink{{
			Path: "docs/design/x.md", HeadingPath: "X", TargetKind: "path", Target: "a/b.go",
			Line: 5, LinkedHash: "h1", CurrentHash: "h2",
			ChangedCommit: "deadbeef", ChangedAt: "2026-09-27T00:00:00Z",
		}},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 900, Method: "tools/call",
		Params: map[string]any{
			"name":      "staleness_report",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	rows, ok := p["doc_link_drift"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("staleness_report: doc_link_drift missing or wrong shape: %v", p)
	}
	row, ok := rows[0].(map[string]any)
	if !ok || row["path"] != "docs/design/x.md" || row["target"] != "a/b.go" || row["changed_commit"] != "deadbeef" {
		t.Errorf("doc_link_drift[0] = %v, want path docs/design/x.md target a/b.go changed_commit deadbeef", rows[0])
	}
	// SR-28's own key must still be present -- KR-7's addition is
	// additive, never a replacement.
	if _, ok := p["doc_drift"]; !ok {
		t.Error("staleness_report: doc_drift (SR-28) key missing after KR-7's addition")
	}
}

// TestStalenessReportNoDocLinkDrift pins that a backend without the
// docDriftBackend capability gets no doc_link_drift key at all -- never an
// empty list standing in for "checked, found nothing" (I38).
func TestStalenessReportNoDocLinkDrift(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 901, Method: "tools/call",
		Params: map[string]any{
			"name":      "staleness_report",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	if _, ok := p["doc_link_drift"]; ok {
		t.Errorf("staleness_report: doc_link_drift present without a docDriftBackend: %v", p)
	}
}
