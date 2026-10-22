package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

const citationSource = "package sample\n\nfunc Alpha() { println(1) }\n"

func citationFixture(t *testing.T) (*client.Client, string) {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}} {
		citationGit(t, root, args...)
	}
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(citationSource), 0o600); err != nil {
		t.Fatal(err)
	}
	citationGit(t, root, "add", ".")
	citationGit(t, root, "commit", "-qm", "fixture")
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "index.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), st)
	t.Cleanup(func() { _ = c.Close() })
	if err := c.IngestServiceAt(context.Background(), "sample", root); err != nil {
		t.Fatal(err)
	}
	if _, err := loadNavGraphsErr(context.Background(), c, "sample"); err != nil {
		t.Fatal(err)
	}
	return c, root
}

func citationGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}

func citationCall(t *testing.T, b QueryInterface, tool string, args map[string]any) (map[string]any, string) {
	t.Helper()
	p, _ := json.Marshal(map[string]any{"name": tool, "arguments": args})
	r, e := NewServer(b).handleToolsCall(context.Background(), p)
	if e != nil {
		t.Fatalf("%s: %+v", tool, e)
	}
	env := r.(map[string]any)
	raw := env["content"].([]any)[0].(map[string]any)["text"].(string)
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	return out, raw
}

func citationContext(t *testing.T, b QueryInterface) (map[string]any, string) {
	t.Helper()
	return citationCall(t, b, "symbol_context", map[string]any{"service": "sample", "symbol": "Alpha"})
}

func requireCitationState(t *testing.T, obj map[string]any, state string) {
	t.Helper()
	mark, _ := obj["citation_status"].(map[string]any)
	if mark["state"] != state || mark["reason"] == "" || mark["reason"] == nil {
		t.Fatalf("citation_status = %v, want %s with reason; object=%v", mark, state, obj)
	}
}

func TestCitationOnDirtyFileIsMarkedStale(t *testing.T) {
	c, root := citationFixture(t)
	path := filepath.Join(root, "sample.go")
	fi, _ := os.Stat(path)
	// Same HEAD, size and mtime: none proves the indexed bytes still exist.
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(citationSource, "Alpha", "Bravo")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, fi.ModTime(), fi.ModTime()); err != nil {
		t.Fatal(err)
	}
	p, _ := citationContext(t, c)
	requireCitationState(t, p["def"].(map[string]any), "stale")
	if _, ok := p["snippet"]; ok {
		requireCitationState(t, p, "stale")
	}
	found, _ := citationCall(t, c, "find_symbol", map[string]any{"service": "sample", "name": "Alpha"})
	requireCitationState(t, found["symbols"].([]any)[0].(map[string]any), "stale")
}

type citationProbe struct {
	*client.Client
	unknown          bool
	refresh          bool
	afterFingerprint func()
	afterFind        func()
	ingestStarted    int64
	fingerprintReads int
}

func (b *citationProbe) FindSymbol(ctx context.Context, service, name string) ([]core.SymbolNode, error) {
	nodes, err := b.Client.FindSymbol(ctx, service, name)
	if b.afterFind != nil {
		b.afterFind()
	}
	return nodes, err
}

func (b *citationProbe) Obligations(context.Context, string, string, int) (semantic.Obligations, error) {
	return semantic.Obligations{Service: "sample", File: "sample.go", Rules: []semantic.Obligation{{Rule: "Keep Alpha stable.", Src: "sample.go#L3-L3", Tier: "doc-claimed", Layer: "rules"}}}, nil
}

func (b *citationProbe) ObligationsLayer(ctx context.Context, service, file string, max int, _ semantic.ObligationLayerFilter) (semantic.Obligations, error) {
	return b.Obligations(ctx, service, file, max)
}

func (b *citationProbe) SourceFingerprint(ctx context.Context, service, path string) (string, bool, error) {
	b.fingerprintReads++
	if b.unknown {
		return "", false, nil
	}
	h, ok, err := b.Client.SourceFingerprint(ctx, service, path)
	if b.afterFingerprint != nil {
		f := b.afterFingerprint
		b.afterFingerprint = nil
		f()
	}
	return h, ok, err
}

func (b *citationProbe) FileStats(context.Context, string) ([]store.FileStat, bool, error) {
	panic("a citation point check must not load the file_stats table")
}

func (b *citationProbe) IngestState(context.Context, string) (store.IngestState, bool, error) {
	if b.refresh {
		return store.IngestState{State: store.IngestInProgress}, true, nil
	}
	return store.IngestState{State: store.IngestComplete, StartedAt: b.ingestStarted}, true, nil
}

func TestCitationDuringRefreshIsMarkedRefreshing(t *testing.T) {
	c, _ := citationFixture(t)
	p, _ := citationContext(t, &citationProbe{Client: c, refresh: true})
	requireCitationState(t, p["def"].(map[string]any), "refreshing")
}

func TestCitationWithUnknownIdentityIsUnknown(t *testing.T) {
	c, _ := citationFixture(t)
	p, _ := citationContext(t, &citationProbe{Client: c, unknown: true})
	requireCitationState(t, p["def"].(map[string]any), "unknown")
}

func TestCleanCitationIsUnchanged(t *testing.T) {
	c, _ := citationFixture(t)
	_, raw := citationContext(t, c)
	baseline := "testdata/citation_clean.golden.json"
	want, err := os.ReadFile(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if raw != string(want) {
		t.Fatalf("clean response changed\ngot:  %s\nbase: %s", raw, want)
	}
}

func TestReadSourceReportsReadIdentity(t *testing.T) {
	c, root := citationFixture(t)
	dirty := strings.ReplaceAll(citationSource, "Alpha", "Bravo")
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(dirty), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"", "HEAD"} {
		p, _ := citationCall(t, c, "read_source", map[string]any{"service": "sample", "file": "sample.go", "ref": ref})
		id, _ := p["source_identity"].(map[string]any)
		if ref == "" {
			sum := sha256.Sum256([]byte(dirty))
			if id["state"] != "working_tree" || id["content_sha256"] != hex.EncodeToString(sum[:]) {
				t.Fatalf("working-tree identity = %v", id)
			}
		} else if id["state"] != "commit" || id["commit"] != citationGit(t, root, "rev-parse", "HEAD") {
			t.Fatalf("ref identity = %v", id)
		}
	}
}

func TestCitationChecksOnlyCitedFiles(t *testing.T) {
	c, root := citationFixture(t)
	// An unrelated file changing is neither read nor hashed for this answer.
	if err := os.WriteFile(filepath.Join(root, "unrelated.go"), []byte("package sample\nfunc Other() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"find_symbol", "symbol_context"} {
		b := &citationProbe{Client: c}
		_, _ = citationCall(t, b, tool, map[string]any{"service": "sample", "symbol": "Alpha", "name": "Alpha"})
		if b.fingerprintReads != 2 {
			t.Fatalf("%s made %d point fingerprint reads, want before/after for the single cited file", tool, b.fingerprintReads)
		}
	}
}

func TestCitationReadRaceUsesHashedBytes(t *testing.T) {
	c, root := citationFixture(t)
	b := &citationProbe{Client: c, afterFingerprint: func() {
		if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(strings.ReplaceAll(citationSource, "Alpha", "Bravo")), 0o600); err != nil {
			t.Fatal(err)
		}
	}}
	p, _ := citationContext(t, b)
	raw, _ := json.Marshal(p["snippet"])
	if strings.Contains(string(raw), "Bravo") {
		requireCitationState(t, p, "stale")
	}
	// Even when the snippet uses the original hashed bytes, its location
	// must be marked if the current file moved before the response.
	requireCitationState(t, p["def"].(map[string]any), "stale")
}

func TestCitationRefreshCompletedDuringAnswerIsMarkedRefreshing(t *testing.T) {
	c, _ := citationFixture(t)
	b := &citationProbe{Client: c, ingestStarted: 1}
	b.afterFind = func() { b.ingestStarted = 2 }
	p, _ := citationCall(t, b, "find_symbol", map[string]any{"service": "sample", "name": "Alpha"})
	requireCitationState(t, p["symbols"].([]any)[0].(map[string]any), "refreshing")
}

func TestCitationObligationsSnippetIsMarkedStale(t *testing.T) {
	c, root := citationFixture(t)
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(strings.ReplaceAll(citationSource, "Alpha", "Bravo")), 0o600); err != nil {
		t.Fatal(err)
	}
	p, _ := citationCall(t, &citationProbe{Client: c}, "obligations", map[string]any{"service": "sample", "file": "sample.go"})
	rules, _ := p["rules"].([]any)
	if len(rules) == 0 {
		t.Fatalf("no obligations: %v", p)
	}
	requireCitationState(t, rules[0].(map[string]any), "stale")
}

func TestCitationReviewLocationsAreMarkedStale(t *testing.T) {
	c, root := citationFixture(t)
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(strings.ReplaceAll(citationSource, "Alpha", "Bravo")), 0o600); err != nil {
		t.Fatal(err)
	}
	p, raw := citationCall(t, &citationProbe{Client: c}, "review_diff", map[string]any{"service": "sample", "changed_files": []string{"sample.go"}})
	if !strings.Contains(raw, `"citation_status":{"state":"stale"`) && !strings.Contains(raw, `"state":"stale"`) {
		t.Fatalf("no marked review location: %v", p)
	}
}

func TestCitationUnknownIdentityDoesNotReadSource(t *testing.T) {
	c, _ := citationFixture(t)
	b := &citationProbe{Client: c, unknown: true}
	gate := tenancy.NewGate(tenancy.GateConfig{})
	ticket, refusal := gate.Admit(context.Background(), tenancy.Request{Tool: "obligations", Service: "sample", Class: tenancy.ClassPoint})
	if refusal != nil {
		t.Fatal(refusal)
	}
	defer ticket.Release(tenancy.OutcomeOK)
	status := citationForFile(ticket.Context(), b, "sample", "sample.go")
	if status.State != "unknown" {
		t.Fatalf("citation status = %v, want unknown", status)
	}
	if got := ticket.BytesRead(); got != 0 {
		t.Fatalf("read %d bytes without an indexed identity to compare", got)
	}
}
