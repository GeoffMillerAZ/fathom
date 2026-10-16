package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designattempts"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designconvert"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func conversionFixture(t *testing.T) (string, map[string]designconvert.Translation, map[string]designattempts.Entry) {
	t.Helper()
	src, root := t.TempDir(), t.TempDir()
	var body bytes.Buffer
	z := zip.NewWriter(&body)
	for _, member := range []struct{ name, content string }{
		{"html/static.html", "<section>\r\n<p class=\"dark:text-white\">SYNTHETIC_ONLY α &amp; β</p>\n<p>Next line</p>\n</section>\n"},
		{"html/unsupported.html", strings.Repeat("<el-dialog></el-dialog>", 70)},
		{"html/long.html", "<p>" + strings.Repeat("X", (32<<10)+1) + "</p>"},
	} {
		w, err := z.Create(member.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(member.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "marketing-v4.zip"), body.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := designassets.Import(context.Background(), src, root)
	if err != nil {
		t.Fatal(err)
	}
	results, entries := map[string]designconvert.Translation{}, map[string]designattempts.Entry{}
	for _, name := range []string{"static", "unsupported", "long"} {
		original, err := designassets.ReadFile(root, c.List()[0].ID, "html/"+name+".html")
		if err != nil {
			t.Fatal(err)
		}
		result, err := designconvert.Translate(designconvert.Source{ArchiveID: original.ArchiveID, Path: original.Path,
			SHA256: original.SHA256, Size: len(original.Content)}, []byte(original.Content))
		if err != nil {
			t.Fatal(err)
		}
		entry, err := designattempts.Publish(root, result)
		if err != nil {
			t.Fatal(err)
		}
		results[name], entries[name] = result, entry
	}
	if results["static"].Artifact == nil || results["long"].Artifact == nil || results["unsupported"].Artifact != nil || len(results["unsupported"].Diagnostics) != 64 || results["unsupported"].OmittedDiagnostics == 0 {
		t.Fatal("synthetic fixture did not exercise translated, unsupported and omitted diagnostics")
	}
	return root, results, entries
}

func conversionProcess(t *testing.T, workingRoot string, args ...string) (context.Context, *mcp.ClientSession) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	t.Cleanup(cancel)
	cmd := optionalSourceCommand(ctx, workingRoot, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "design-conversion-test", Version: "test"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("startup: %v\n%s", err, stderr.String())
	}
	t.Cleanup(func() { _ = session.Close() })
	return ctx, session
}

func conversionToolError(t *testing.T, ctx context.Context, s *mcp.ClientSession, name string, input any, contains string) {
	t.Helper()
	result, err := s.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: input})
	if err != nil || !result.IsError {
		t.Fatalf("%s must refuse as a tool error: %v", name, err)
	}
	var text strings.Builder
	for _, item := range result.Content {
		if value, ok := item.(*mcp.TextContent); ok {
			text.WriteString(value.Text)
		}
	}
	if contains != "" && !strings.Contains(text.String(), contains) {
		t.Fatalf("%s refusal missing %q", name, contains)
	}
}

func TestDesignConversionsRealMCPParityAndPaging(t *testing.T) {
	root, results, entries := conversionFixture(t)
	ctx, session := conversionProcess(t, t.TempDir(), "-design-assets", root)
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, tool := range tools.Tools {
		if tool.Name != "list_design_conversions" && tool.Name != "get_design_conversion" {
			continue
		}
		found[tool.Name] = true
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint || tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Fatalf("%s missing typed, local, read-only contract", tool.Name)
		}
	}
	if len(found) != 2 {
		t.Fatal("actual server did not register both conversion tools")
	}
	catalog, err := designattempts.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []map[string]any{{}, {"limit": 50}} {
		body := catalogCall(t, ctx, session, "list_design_conversions", input)
		var page designattempts.Page
		if err := json.Unmarshal(body, &page); err != nil || page.Total != 3 || len(page.Entries) != 3 {
			t.Fatal("optional MCP discovery fields became required", err)
		}
	}
	var ids []string
	query := designattempts.Query{Limit: 1}
	for n := 0; n < 3; n++ {
		body := catalogCall(t, ctx, session, "list_design_conversions", query)
		want, err := catalog.Search(query)
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(want)
		catalogJSONEqual(t, body, encoded)
		if len(body) > 64<<10 || bytes.Contains(body, []byte("SYNTHETIC_ONLY")) || bytes.Contains(body, []byte(`"content"`)) {
			t.Fatal("metadata listing leaked source or exceeded bound")
		}
		var page designattempts.Page
		if err := json.Unmarshal(body, &page); err != nil || len(page.Entries) != 1 || page.Revision != catalog.Revision || page.Total != 3 {
			t.Fatal("wrong metadata page or revision", err)
		}
		ids = append(ids, page.Entries[0].ID)
		query.Cursor = page.NextCursor
	}
	if query.Cursor != "" || len(slices.Compact(ids)) != 3 {
		t.Fatal("missing or duplicate attempt during continuation")
	}
	for _, query := range []designattempts.Query{
		{ArchiveID: entries["static"].Source.ArchiveID, Path: entries["static"].Source.Path},
		{Status: "unsupported"}, {Path: "html/missing.html"},
	} {
		got := catalogCall(t, ctx, session, "list_design_conversions", query)
		want, _ := catalog.Search(query)
		body, _ := json.Marshal(want)
		catalogJSONEqual(t, got, body)
	}
	for _, query := range []designattempts.Query{{Limit: 51}, {Status: "verified"}, {Path: "../escape"}, {Cursor: "bad"}} {
		conversionToolError(t, ctx, session, "list_design_conversions", query, "")
	}
	var assembled strings.Builder
	start := 1
	for pageNumber := 0; pageNumber < 100; pageNumber++ {
		body := catalogCall(t, ctx, session, "get_design_conversion", conversionGetInput{ID: entries["static"].ID, StartLine: start, Limit: 2})
		var got conversionDetail
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if got.Revision != catalog.Revision || !reflect.DeepEqual(got.Entry, entries["static"]) || got.Replay != "matches-current" || got.Entry.Verification != "not-verified" || got.Artifact == nil {
			t.Fatal("exact metadata, original identity or replay changed")
		}
		artifact := results["static"].Artifact
		if got.Artifact.SHA256 != artifact.SHA256 || got.Artifact.Filename != artifact.Filename || got.Artifact.MappingCount != len(artifact.Mappings) || got.Artifact.StartLine != start {
			t.Fatal("artifact lineage or window metadata changed")
		}
		if !strings.Contains(got.Notice, "get_design_asset_file") || !strings.Contains(got.Notice, "not runtime compatibility") || bytes.Contains(body, []byte(`"mappings":`)) || len(body) > 64<<10 || len(got.Artifact.Content) > 32<<10 {
			t.Fatal("exact output omitted limits/context or embedded mappings")
		}
		if got.Diagnostics.Total != 0 || got.Diagnostics.Omitted != 0 || got.Diagnostics.Items == nil || len(got.Diagnostics.Items) != 0 {
			t.Fatal("translated diagnostics changed")
		}
		assembled.WriteString(got.Artifact.Content)
		if got.Artifact.NextLine == 0 {
			break
		}
		start = got.Artifact.NextLine
	}
	if assembled.String() != results["static"].Artifact.Content || fmt.Sprintf("%x", sha256.Sum256([]byte(assembled.String()))) != entries["static"].ArtifactSHA256 {
		t.Fatal("artifact pagination changed bytes or failed to terminate")
	}
	originalBody := catalogCall(t, ctx, session, "get_design_asset_file", assetReadInput{ID: entries["static"].Source.ArchiveID, Path: entries["static"].Source.Path})
	var original assetTextPage
	if err := json.Unmarshal(originalBody, &original); err != nil || original.SHA256 != entries["static"].Source.SHA256 || !strings.Contains(original.Content, "SYNTHETIC_ONLY") {
		t.Fatal("exact vendor original no longer reachable", err)
	}
	diagnosticStart := 0
	var diagnostics []designconvert.Diagnostic
	for n := 0; n < 4; n++ {
		body := catalogCall(t, ctx, session, "get_design_conversion", conversionGetInput{ID: entries["unsupported"].ID, DiagnosticStart: diagnosticStart})
		var got conversionDetail
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if got.Artifact != nil || bytes.Contains(body, []byte(`"artifact":`)) || got.Diagnostics.Start != diagnosticStart || got.Diagnostics.Total != 64 || got.Diagnostics.Omitted != results["unsupported"].OmittedDiagnostics || len(got.Diagnostics.Items) != 16 || got.Entry.Source != entries["unsupported"].Source {
			t.Fatal("unsupported diagnostics/origin lost or artifact fabricated")
		}
		diagnostics = append(diagnostics, got.Diagnostics.Items...)
		diagnosticStart = got.Diagnostics.NextDiagnostic
	}
	if diagnosticStart != 0 || !reflect.DeepEqual(diagnostics, results["unsupported"].Diagnostics) {
		t.Fatal("diagnostic paging lost original order, offsets or reasons")
	}
	for _, input := range []conversionGetInput{
		{ID: entries["unsupported"].ID, Limit: 1}, {ID: entries["unsupported"].ID, StartLine: 2},
		{ID: entries["unsupported"].ID, DiagnosticStart: 64}, {ID: entries["static"].ID, DiagnosticStart: 1},
		{ID: entries["static"].ID, StartLine: 100000},
	} {
		conversionToolError(t, ctx, session, "get_design_conversion", input, "")
	}
	// Explicit defaults are meaningful, and are allowed even without an artifact.
	catalogCall(t, ctx, session, "get_design_conversion", conversionGetInput{ID: entries["unsupported"].ID, StartLine: 1, Limit: 100, DiagnosticLimit: 64})
	longLine := 0
	for i, line := range strings.SplitAfter(results["long"].Artifact.Content, "\n") {
		if len(line) > 32<<10 {
			longLine = i + 1
			break
		}
	}
	if longLine == 0 {
		t.Fatal("missing oversized line canary")
	}
	conversionToolError(t, ctx, session, "get_design_conversion", conversionGetInput{ID: entries["long"].ID, StartLine: longLine, Limit: 1}, "assets-conversion")
}

func TestDesignConversionsMissingStoreAndEarlyBounds(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			workingRoot := t.TempDir()
			root := filepath.Join(workingRoot, designAssetsRel)
			var args []string
			if existing {
				root, _ = importedDesignFixture(t)
				args = []string{"-design-assets", root}
			}
			ctx, session := conversionProcess(t, workingRoot, args...)
			if existing {
				var page designattempts.Page
				if err := json.Unmarshal(catalogCall(t, ctx, session, "list_design_conversions", map[string]any{}), &page); err != nil || page.Total != 0 || page.Entries == nil {
					t.Fatal("absent optional attempt store must be empty", err)
				}
			} else {
				conversionToolError(t, ctx, session, "list_design_conversions", map[string]any{}, "unavailable")
			}
			id := "syntax@" + strings.Repeat("a", 64)
			for _, tc := range []struct {
				in   conversionGetInput
				want string
			}{
				{conversionGetInput{ID: "../escape"}, "id must"},
				{conversionGetInput{ID: "syntax@" + strings.Repeat("A", 64)}, "id must"},
				{conversionGetInput{ID: id, StartLine: -1}, "start_line"},
				{conversionGetInput{ID: id, StartLine: math.MaxInt}, "start_line"},
				{conversionGetInput{ID: id, Limit: 401}, "limit"},
				{conversionGetInput{ID: id, Limit: -1}, "limit"},
				{conversionGetInput{ID: id, DiagnosticStart: -1}, "diagnostic_start"},
				{conversionGetInput{ID: id, DiagnosticStart: 65}, "diagnostic_start"},
				{conversionGetInput{ID: id, DiagnosticLimit: 65}, "diagnostic_limit"},
				{conversionGetInput{ID: id, DiagnosticLimit: -1}, "diagnostic_limit"},
			} {
				conversionToolError(t, ctx, session, "get_design_conversion", tc.in, tc.want)
			}
			conversionToolError(t, ctx, session, "get_design_conversion", conversionGetInput{ID: id}, "")
			catalogCall(t, ctx, session, "get_capability", capabilityGetInput{ID: "analytics.bars"})
			if _, err := os.Stat(filepath.Join(root, "translations")); !os.IsNotExist(err) {
				t.Fatal("read-only tools created an optional attempt store")
			}
			if !existing {
				if _, err := os.Stat(root); !os.IsNotExist(err) {
					t.Fatal("read-only tools created the private root")
				}
			}
		})
	}
}

func TestDesignConversionsCorruptionIsolatedFromFirstParty(t *testing.T) {
	root, _, entries := conversionFixture(t)
	const wantOriginal = "<section>\r\n<p class=\"dark:text-white\">SYNTHETIC_ONLY α &amp; β</p>\n<p>Next line</p>\n</section>\n"
	wantOriginalHash := fmt.Sprintf("%x", sha256.Sum256([]byte(wantOriginal)))
	ctx, session := conversionProcess(t, t.TempDir(), "-design-assets", root)
	id := entries["static"].ID
	archiveHash := strings.Split(entries["static"].Source.ArchiveID, "@")[1]
	for _, filename := range []string{
		filepath.Join(root, "translations", "entries", strings.TrimPrefix(id, "syntax@")+".json"),
		filepath.Join(root, "archives", archiveHash+".zip"),
		filepath.Join(root, "translations", "catalog.json"),
	} {
		original, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte("corrupt synthetic fixture"), 0600); err != nil {
			t.Fatal(err)
		}
		conversionToolError(t, ctx, session, "get_design_conversion", conversionGetInput{ID: id}, "")
		catalogCall(t, ctx, session, "get_capability", capabilityGetInput{ID: "analytics.bars"})
		if filepath.Base(filename) == "catalog.json" {
			conversionToolError(t, ctx, session, "list_design_conversions", map[string]any{}, "unavailable")
			// A broken conversion index must not gate the independent original
			// archive path. Expected bytes/hash come from the synthetic input,
			// not from either the conversion or original-file response.
			body := catalogCall(t, ctx, session, "get_design_asset_file", assetReadInput{
				ID: entries["static"].Source.ArchiveID, Path: "html/static.html",
			})
			var got assetTextPage
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatal(err)
			}
			if got.ArchiveID != entries["static"].Source.ArchiveID || got.Path != "html/static.html" ||
				got.Content != wantOriginal || got.SHA256 != wantOriginalHash ||
				fmt.Sprintf("%x", sha256.Sum256([]byte(got.Content))) != wantOriginalHash || got.NextLine != 0 {
				t.Fatal("corrupt conversion index hid or changed the exact vendor original")
			}
		} else {
			catalogCall(t, ctx, session, "list_design_conversions", map[string]any{})
		}
		after, err := os.ReadFile(filename)
		if err != nil || string(after) != "corrupt synthetic fixture" {
			t.Fatal("exact failure attempted to repair or replace storage", err)
		}
		if err := os.WriteFile(filename, original, 0600); err != nil {
			t.Fatal(err)
		}
	}
	catalogCall(t, ctx, session, "get_design_conversion", conversionGetInput{ID: id})
}

func TestConversionArtifactWindowAndStaleNotice(t *testing.T) {
	for _, content := range []string{"a\r\nb\nlast", "a\n\n", "\n", "α\r\nβ", ""} {
		artifact := &designconvert.Artifact{Filename: "snippet.templ", SHA256: strings.Repeat("a", 64), Content: content}
		var joined strings.Builder
		for line := 1; line < 10; {
			page, err := conversionArtifactWindow(artifact, line, 1)
			if err != nil {
				t.Fatal(err)
			}
			joined.WriteString(page.Content)
			if page.NextLine == 0 {
				break
			}
			line = page.NextLine
		}
		if joined.String() != content {
			t.Fatal("line window normalized source whitespace or lost bytes")
		}
	}
	for _, bounds := range [][2]int{{0, 1}, {-1, 1}, {1, 0}, {1, 401}, {math.MaxInt, 1}} {
		if _, err := conversionArtifactWindow(&designconvert.Artifact{Content: "x"}, bounds[0], bounds[1]); err == nil {
			t.Fatal("invalid artifact window accepted")
		}
	}
	if _, err := conversionArtifactWindow(&designconvert.Artifact{Content: strings.Repeat("x", (32<<10)+1)}, 1, 1); err == nil || !strings.Contains(err.Error(), "assets-conversion") {
		t.Fatal("long line was truncated or failed without local fallback", err)
	}
	detail := designattempts.Detail{Entry: designattempts.Entry{Verification: "not-verified"}, Replay: "stale-transformer",
		Translation: designconvert.Translation{Diagnostics: []designconvert.Diagnostic{}, Artifact: &designconvert.Artifact{Content: "x"}}}
	out, err := conversionOutput(detail, conversionGetInput{StartLine: 1, Limit: 100, DiagnosticLimit: 16})
	if err != nil || out.Replay != "stale-transformer" || out.Entry.Verification != "not-verified" || !strings.Contains(out.Notice, "not runtime compatibility") {
		t.Fatal("adapter promoted stale replay into compatibility", err)
	}
}
