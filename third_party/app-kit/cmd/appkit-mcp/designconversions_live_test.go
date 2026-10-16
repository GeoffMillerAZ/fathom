package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designattempts"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Read-only private proof. No receipt/source body or raw protocol failure is
// logged. Normal tests use synthetic data; this probe requires the saved pilot.
func TestSavedConversionsPrivateMCPReadLive(t *testing.T) {
	kitRoot := os.Getenv("APPKIT_PROBE_SAVED_CONVERSIONS_KIT_ROOT")
	if kitRoot == "" {
		t.Skip("set APPKIT_PROBE_SAVED_CONVERSIONS_KIT_ROOT to the existing private kit checkout")
	}
	if !filepath.IsAbs(kitRoot) {
		t.Fatal("private probe root must be absolute")
	}
	root := filepath.Join(kitRoot, "data/design-assets")
	catalog, err := designattempts.Load(root)
	if err != nil {
		t.Fatal("private saved catalog unavailable; error body omitted")
	}
	expected, err := catalog.Search(designattempts.Query{Limit: 50})
	if err != nil || expected.Total == 0 || expected.NextCursor != "" {
		t.Fatal("probe expects a nonempty bounded private pilot page")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := optionalSourceCommand(ctx, t.TempDir(), "-design-assets", root)
	client := mcp.NewClient(&mcp.Implementation{Name: "saved-private-read-proof", Version: "test"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal("private protocol startup failed; output omitted")
	}
	defer session.Close()
	call := func(name string, args map[string]any, out any) {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || result.IsError || len(result.Content) != 1 {
			t.Fatalf("%s failed; private response omitted", name)
		}
		text, ok := result.Content[0].(*mcp.TextContent)
		if !ok {
			t.Fatal("expected one text result")
		}
		encoded, err := json.Marshal(result.StructuredContent)
		if err != nil || len(encoded) > 64<<10 || len(text.Text) > 64<<10 {
			t.Fatal("protocol response encoding or size failed")
		}
		var a, b any
		if json.Unmarshal(encoded, &a) != nil || json.Unmarshal([]byte(text.Text), &b) != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("protocol text/structured parity failed; private body omitted")
		}
		if json.Unmarshal(encoded, out) != nil {
			t.Fatal("protocol decode failed; private body omitted")
		}
	}
	var page designattempts.Page
	call("list_design_conversions", map[string]any{"limit": 50}, &page)
	if !reflect.DeepEqual(page, expected) {
		t.Fatal("CLI/library and MCP catalog disagree")
	}
	translated, unsupported := 0, 0
	for _, entry := range page.Entries {
		var got conversionDetail
		call("get_design_conversion", map[string]any{"id": entry.ID}, &got)
		if !reflect.DeepEqual(got.Entry, entry) || got.Revision != page.Revision || got.Replay != "matches-current" || got.Entry.Verification != "not-verified" {
			t.Fatal("saved pilot identity/replay mismatch; body omitted")
		}
		if entry.Status == "translated" {
			translated++
			if got.Artifact == nil || got.Artifact.SHA256 != entry.ArtifactSHA256 || got.Artifact.Content == "" {
				t.Fatal("saved output unavailable; body omitted")
			}
		} else {
			unsupported++
			if got.Artifact != nil || got.Diagnostics.Total == 0 {
				t.Fatal("unsupported attempt lost diagnostics or gained output")
			}
		}
		var original assetTextPage
		call("get_design_asset_file", map[string]any{"id": entry.Source.ArchiveID, "path": entry.Source.Path, "limit": 1}, &original)
		if original.SHA256 != entry.Source.SHA256 || original.ArchiveID != entry.Source.ArchiveID || original.Path != entry.Source.Path {
			t.Fatal("original fallback identity drifted; body omitted")
		}
	}
	t.Logf("saved_attempts=%d translated=%d unsupported=%d current_replays=%d original_reads=%d verified_targets=0 revision=%s", page.Total, translated, unsupported, page.Total, page.Total, page.Revision)
}
