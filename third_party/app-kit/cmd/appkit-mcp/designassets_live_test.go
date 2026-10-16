package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestDesignAssetsLivePurchaseProbe is deliberately opt-in. Ordinary tests use
// synthetic fixtures and never require, discover, or modify a purchased corpus.
// This probe reads an already imported cache; it never imports or extracts ZIPs.
func TestDesignAssetsLivePurchaseProbe(t *testing.T) {
	root := os.Getenv("APPKIT_PROBE_DESIGN_ASSETS_ROOT")
	if root == "" {
		t.Skip("opt-in private corpus probe: set APPKIT_PROBE_DESIGN_ASSETS_ROOT to an existing imported archive cache")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("APPKIT_PROBE_DESIGN_ASSETS_ROOT must be an absolute existing cache path")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	started := time.Now()
	c, err := designassets.Load(root)
	if err != nil {
		t.Fatal("private corpus manifest failed validation; source bodies are not logged")
	}
	archives := c.List()
	if len(archives) < 1 || len(archives) > 64 {
		t.Fatalf("live probe requires 1..64 archive revisions; found %d", len(archives))
	}
	stats := map[string]*liveAssetTiming{}
	for _, class := range []string{"discovery", "exact_metadata", "file_listing", "exact_read"} {
		stats[class] = &liveAssetTiming{}
	}
	var verifiedCount, selectedBytes int
	defer func() {
		files, projects, archiveBytes := 0, 0, int64(0)
		for _, a := range archives {
			files += len(a.Files)
			projects += len(a.Projects)
			archiveBytes += a.Size
		}
		t.Logf("private corpus probe: revision=%s archives=%d files=%d projects=%d archive_bytes=%d verified_archives=%d selected_text_bytes=%d elapsed=%s",
			c.Revision, len(archives), files, projects, archiveBytes, verifiedCount, selectedBytes, time.Since(started))
		for _, class := range []string{"discovery", "exact_metadata", "file_listing", "exact_read"} {
			s := stats[class]
			t.Logf("tool_class=%s calls=%d total=%s max=%s max_archive=%s max_response_bytes=%d", class, s.calls, s.total, s.max, s.maxArchive, s.maxBytes)
		}
	}()
	cmd := optionalSourceCommand(ctx, t.TempDir(), "-design-assets", root)
	// Error responses and stderr can contain vendor text. Do not print either.
	cmd.Stderr = io.Discard
	client := mcp.NewClient(&mcp.Implementation{Name: "private-design-assets-live-probe", Version: "test"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal("real MCP subprocess startup failed; stderr intentionally not logged")
	}
	defer session.Close()
	call := func(class, name, id string, args, out any) {
		t.Helper()
		liveAssetCall(t, ctx, session, stats[class], name, id, args, out)
	}
	seen := map[string]bool{}
	query := designassets.Query{Limit: 10}
	complete := false
	for pageNo := 0; pageNo < 7; pageNo++ {
		var page designassets.Page
		call("discovery", "search_design_assets", "", query, &page)
		want, err := c.Search(query)
		if err != nil || !reflect.DeepEqual(page, want) || page.Revision != c.Revision || page.Total != len(archives) || len(page.Items) > 10 {
			t.Fatal("live discovery differs from validated local catalog; response bodies omitted")
		}
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatal("discovery returned a duplicate immutable archive ID")
			}
			seen[item.ID] = true
		}
		if page.NextCursor == "" {
			complete = true
			break
		}
		query.Cursor = page.NextCursor
	}
	if !complete || len(seen) != len(archives) {
		t.Fatal("bounded discovery did not enumerate the entire supplied catalog")
	}
	for _, archived := range archives {
		if ctx.Err() != nil {
			t.Fatal("90-second live probe context expired")
		}
		if !seen[archived.ID] {
			t.Fatal("archive absent from real server discovery")
		}
		verified, err := designassets.Verify(root, archived.ID)
		if err != nil || !reflect.DeepEqual(verified, archived) {
			t.Fatalf("local archive/inventory verification failed for %s; source bodies omitted", archived.ID)
		}
		var detail assetDetail
		call("exact_metadata", "get_design_asset", archived.ID, assetGetInput{ID: archived.ID}, &detail)
		if !reflect.DeepEqual(detail.Asset, designassets.Summarize(verified)) || !reflect.DeepEqual(detail.Projects, verified.Projects) || detail.Notice == "" {
			t.Fatalf("exact metadata differs from verified original for %s", archived.ID)
		}
		selected := liveAssetMember(t, verified)
		fileQuery := designassets.FileQuery{ID: archived.ID, Prefix: selected.Path, Kind: selected.Kind, Limit: 50}
		var files designassets.FilePage
		call("file_listing", "list_design_asset_files", archived.ID, fileQuery, &files)
		wantFiles, err := c.FilesPage(fileQuery)
		if err != nil || !reflect.DeepEqual(files, wantFiles) {
			t.Fatalf("member discovery differs from local inventory for %s", archived.ID)
		}
		found := false
		for _, file := range files.Items {
			if reflect.DeepEqual(file, selected) {
				found = true
			}
		}
		if !found {
			t.Fatalf("selected README/source not retrievable through file discovery for %s", archived.ID)
		}
		expected, err := designassets.ReadFile(root, archived.ID, selected.Path)
		if err != nil {
			t.Fatalf("selected README/source failed bounded local read for %s", archived.ID)
		}
		original := liveAssetOriginalMember(t, root, verified, selected)
		if !bytes.Equal(original, []byte(expected.Content)) {
			t.Fatalf("library text differs from original ZIP bytes for %s", archived.ID)
		}
		var assembled strings.Builder
		start, finished := 1, false
		// Selected text is <=16 KiB. Even one-byte lines need at most 41
		// 400-line pages; the fixed 64-iteration cap prevents cursor loops.
		for pageNo := 0; pageNo < 64; pageNo++ {
			var page assetTextPage
			call("exact_read", "get_design_asset_file", archived.ID, assetReadInput{ID: archived.ID, Path: selected.Path, StartLine: start, Limit: 400}, &page)
			if page.ArchiveID != archived.ID || page.Path != selected.Path || page.SHA256 != selected.SHA256 || page.Size != selected.Size || page.StartLine != start || len(page.Content) > 32*1024 || page.TotalLines < 1 {
				t.Fatalf("source page metadata/size differs from original for %s", archived.ID)
			}
			assembled.WriteString(page.Content)
			if int64(assembled.Len()) > selected.Size {
				t.Fatalf("source paging exceeded original size for %s", archived.ID)
			}
			if page.NextLine == 0 {
				finished = true
				break
			}
			if page.NextLine <= start {
				t.Fatalf("source paging did not advance for %s", archived.ID)
			}
			start = page.NextLine
		}
		if !finished || assembled.String() != expected.Content {
			t.Fatalf("MCP source bytes differ from complete original ZIP member for %s", archived.ID)
		}
		selectedBytes += assembled.Len()
		verifiedCount++
	}
	if ctx.Err() != nil {
		t.Fatal("90-second live probe context expired")
	}
}

type liveAssetTiming struct {
	calls      int
	total, max time.Duration
	maxArchive string
	maxBytes   int
}

func liveAssetCall(t *testing.T, ctx context.Context, session *mcp.ClientSession, timing *liveAssetTiming, name, id string, input, output any) {
	t.Helper()
	start := time.Now()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: input})
	elapsed := time.Since(start)
	timing.calls++
	timing.total += elapsed
	if elapsed > timing.max {
		timing.max, timing.maxArchive = elapsed, id
	}
	if err != nil || result == nil || result.IsError || len(result.Content) != 1 {
		t.Fatalf("%s failed for %s; transport/tool response intentionally not logged", name, id)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("%s did not return one text content block", name)
	}
	timing.maxBytes = max(timing.maxBytes, len(text.Text))
	if len(text.Text) > 64*1024 {
		t.Fatalf("%s exceeded the 64 KiB result budget", name)
	}
	var textJSON any
	if err := json.Unmarshal([]byte(text.Text), &textJSON); err != nil {
		t.Fatalf("%s returned invalid JSON; body intentionally omitted", name)
	}
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("%s returned invalid structured content", name)
	}
	var structuredJSON any
	if json.Unmarshal(structured, &structuredJSON) != nil || !reflect.DeepEqual(textJSON, structuredJSON) {
		t.Fatalf("%s text/structured payloads differ; bodies intentionally omitted", name)
	}
	if err := json.Unmarshal([]byte(text.Text), output); err != nil {
		t.Fatalf("%s response shape differs from local API", name)
	}
}

func liveAssetMember(t *testing.T, a designassets.Archive) designassets.File {
	t.Helper()
	for _, kind := range []string{"readme", "source"} {
		var best designassets.File
		for _, f := range a.Files {
			if f.Kind == kind && f.Size > 0 && f.Size <= 16*1024 && (best.Path == "" || f.Size < best.Size) {
				best = f
			}
		}
		if best.Path != "" {
			return best
		}
	}
	t.Fatalf("archive %s has no nonempty README/source member within this probe's 16 KiB selection bound", a.ID)
	return designassets.File{}
}

func liveAssetOriginalMember(t *testing.T, root string, a designassets.Archive, file designassets.File) []byte {
	t.Helper()
	z, err := zip.OpenReader(filepath.Join(root, "archives", a.SHA256+".zip"))
	if err != nil {
		t.Fatalf("cannot reopen verified original ZIP for %s", a.ID)
	}
	defer z.Close()
	var body []byte
	found := 0
	for _, f := range z.File {
		if f.Name != file.Path {
			continue
		}
		found++
		r, err := f.Open()
		if err != nil {
			t.Fatalf("cannot read original ZIP member for %s", a.ID)
		}
		body, err = io.ReadAll(io.LimitReader(r, 16*1024+1))
		closeErr := r.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("original member CRC/read failed for %s", a.ID)
		}
	}
	hash := sha256.Sum256(body)
	if found != 1 || int64(len(body)) != file.Size || hex.EncodeToString(hash[:]) != file.SHA256 {
		t.Fatalf("original member hash/size differs from inventory for %s", a.ID)
	}
	return body
}
