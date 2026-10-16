package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func importedDesignFixture(t *testing.T) (string, *designassets.Catalog) {
	t.Helper()
	src, root := t.TempDir(), t.TempDir()
	f, err := os.Create(filepath.Join(src, "catalyst-ui-kit.zip"))
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for _, item := range []struct{ path, body string }{
		{"README.md", "# Synthetic reference\nSetup instructions.\nThird line.\n"},
		{"typescript/button.tsx", "export const button = 'LITERAL_SOURCE';\n"},
		{"demo/package.json", `{"name":"synthetic-reference","version":"1.0.0","dependencies":{"react":"^19"}}`},
		{"image.png", "\x89PNG\x00\xff"},
		{".env", "SYNTHETIC_SECRET=fixture-only"},
	} {
		w, err := z.Create(item.path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(item.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	c, err := designassets.Import(context.Background(), src, root)
	if err != nil {
		t.Fatal(err)
	}
	return root, c
}

func TestDesignAssetsRealMCPStartupAndRetrieval(t *testing.T) {
	root, c := importedDesignFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := optionalSourceCommand(ctx, t.TempDir(), "-design-assets", root)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "design-asset-test", Version: "test"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("startup: %v\n%s", err, stderr.String())
	}
	defer session.Close()
	if !strings.Contains(session.InitializeResult().Instructions, "complete visual reference") {
		t.Fatal("actual host instructions omit reference-led discovery")
	}
	var page designassets.Page
	b := catalogCall(t, ctx, session, "search_design_assets", map[string]any{"query": "typescript", "limit": 1})
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if page.Revision != c.Revision || page.Total != 1 || strings.Contains(string(b), "LITERAL_SOURCE") {
		t.Fatalf("search metadata/parity: %s", b)
	}
	id := page.Items[0].ID
	var detail assetDetail
	b = catalogCall(t, ctx, session, "get_design_asset", map[string]any{"id": id})
	if err := json.Unmarshal(b, &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Projects) != 1 || detail.Projects[0].Dependencies["react"] != "^19" {
		t.Fatalf("dependency context lost: %s", b)
	}
	b = catalogCall(t, ctx, session, "list_design_asset_files", map[string]any{"id": id, "prefix": "typescript/"})
	var files designassets.FilePage
	if err := json.Unmarshal(b, &files); err != nil {
		t.Fatal(err)
	}
	if files.Total != 1 || files.Items[0].Path != "typescript/button.tsx" {
		t.Fatalf("file discovery: %s", b)
	}
	var text assetTextPage
	b = catalogCall(t, ctx, session, "get_design_asset_file", map[string]any{"id": id, "path": "README.md", "limit": 1})
	if err := json.Unmarshal(b, &text); err != nil {
		t.Fatal(err)
	}
	if text.Content != "# Synthetic reference\n" || text.NextLine != 2 || text.TotalLines != 3 {
		t.Fatalf("source paging: %s", b)
	}
	b = catalogCall(t, ctx, session, "get_design_asset_file", map[string]any{"id": id, "path": "README.md", "start_line": 2})
	text = assetTextPage{}
	if err := json.Unmarshal(b, &text); err != nil {
		t.Fatal(err)
	}
	if text.Content != "Setup instructions.\nThird line.\n" || text.NextLine != 0 {
		t.Fatalf("source continuation: %s", b)
	}
	for _, args := range []map[string]any{{"id": id, "path": "../README.md"}, {"id": id, "path": "image.png"}, {"id": id, "path": ".env"}, {"id": id, "path": "README.md", "limit": 401}} {
		r, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_design_asset_file", Arguments: args})
		if err != nil || !r.IsError {
			t.Errorf("unsafe read must be a tool error: %+v %+v %v", args, r, err)
		}
	}
	archive := c.List()[0]
	if err := os.WriteFile(filepath.Join(root, "archives", archive.SHA256+".zip"), []byte("corrupt fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_design_asset", Arguments: map[string]any{"id": id}})
	if err != nil || !r.IsError {
		t.Fatalf("corrupt archive accepted: %+v %v", r, err)
	}
}

func TestDesignAssetsExplicitBrokenConfigurationFails(t *testing.T) {
	for _, env := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		root := t.TempDir()
		var args []string
		if !env {
			args = []string{"-design-assets", root}
		}
		cmd := optionalSourceCommand(ctx, t.TempDir(), args...)
		if env {
			cmd.Env = append(cmd.Env, "APPKIT_DESIGN_ASSETS="+root)
		}
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "open design assets") || !strings.Contains(string(out), "assets-import") {
			t.Fatalf("explicit configuration env=%v did not fail actionably: %v %s", env, err, out)
		}
	}
	if err := checkDesignAssetsRoot(t.TempDir(), false); err != nil {
		t.Fatalf("optional missing assets blocked startup: %v", err)
	}
}

func TestAssetTextWindowBoundsAndExactBytes(t *testing.T) {
	f := &designassets.TextFile{Content: "a\r\nb\nfinal"}
	one, err := assetTextWindow(f, 1, 1)
	if err != nil || one.Content != "a\r\n" || one.NextLine != 2 {
		t.Fatalf("CRLF changed: %+v %v", one, err)
	}
	two, err := assetTextWindow(f, one.NextLine, 20)
	if err != nil || one.Content+two.Content != f.Content || two.NextLine != 0 {
		t.Fatalf("bytes lost: %+v %v", two, err)
	}
	for _, bounds := range [][2]int{{-1, 1}, {1, -1}, {1, 401}, {4, 1}} {
		if _, err := assetTextWindow(f, bounds[0], bounds[1]); err == nil {
			t.Errorf("bad bounds accepted: %v", bounds)
		}
	}
	if _, err := assetTextWindow(&designassets.TextFile{Content: strings.Repeat("x", 32*1024+1)}, 1, 1); err == nil {
		t.Fatal("oversized line silently truncated or returned")
	}
}
