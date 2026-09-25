package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/geoffmilleraz/app-kit/internal/twplus/index"
)

func previewSession(t *testing.T, html string) (context.Context, *mcp.ClientSession) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "previews.sqlite")
	builder, err := index.NewPreviewBuilder(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = builder.Close() })
	if err := builder.Add("Synthetic.Preview", "html", 4, "light", html); err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Finalize(); err != nil {
		t.Fatal(err)
	}
	pv, err := index.OpenPreviews(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pv.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	server := mcp.NewServer(&mcp.Implementation{Name: "preview-paging-server", Version: "test"}, nil)
	registerPreviewTool(server, pv, path)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "preview-paging-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return ctx, session
}

func callPreviewPage(t *testing.T, ctx context.Context, session *mcp.ClientSession, args map[string]any) (string, previewMeta) {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_component_preview", Arguments: args})
	if err != nil {
		t.Fatalf("preview protocol failure: %v", err)
	}
	if result.IsError || len(result.Content) != 1 {
		t.Fatalf("preview tool failure: %+v", result)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("preview returned %T, want text", result.Content[0])
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var meta previewMeta
	if err := json.Unmarshal(encoded, &meta); err != nil {
		t.Fatal(err)
	}
	return text.Text, meta
}

func TestPreviewPagingReassemblesExactUTF8(t *testing.T) {
	html := "<main>αβγ🙂-exact-preview-日本語</main>"
	ctx, session := previewSession(t, html)
	args := map[string]any{"full_name": "Synthetic.Preview", "framework": "html", "tailwind_version": 4, "mode": "light", "max_bytes": 7}
	var assembled strings.Builder
	var expectedHash string
	offset := 0
	for {
		args["offset"] = offset
		if expectedHash != "" {
			args["expected_sha256"] = expectedHash
		}
		body, meta := callPreviewPage(t, ctx, session, args)
		if !utf8.ValidString(body) || meta.Offset != offset || meta.ReturnedBytes != len(body) || meta.Bytes != len(html) {
			t.Fatalf("invalid page at offset %d: body=%q meta=%+v", offset, body, meta)
		}
		if expectedHash == "" {
			expectedHash = meta.SHA256
		} else if meta.SHA256 != expectedHash {
			t.Fatalf("page hash changed: got %s want %s", meta.SHA256, expectedHash)
		}
		assembled.WriteString(body)
		if !meta.HasMore {
			if meta.NextOffset != 0 {
				t.Fatalf("final page next_offset=%d, want omitted/zero", meta.NextOffset)
			}
			break
		}
		if meta.NextOffset <= offset {
			t.Fatalf("paging did not advance: %+v", meta)
		}
		offset = meta.NextOffset
	}
	if assembled.String() != html {
		t.Fatalf("reassembled preview differs\ngot:  %q\nwant: %q", assembled.String(), html)
	}
	wantHash := fmt.Sprintf("%x", sha256.Sum256([]byte(html)))
	if expectedHash != wantHash {
		t.Fatalf("sha256=%s want %s", expectedHash, wantHash)
	}
}

func TestPreviewPagingDefaultAndSmallPreview(t *testing.T) {
	large := strings.Repeat("a", defaultPreviewPageBytes-1) + "🙂" + strings.Repeat("z", 32)
	ctx, session := previewSession(t, large)
	body, meta := callPreviewPage(t, ctx, session, map[string]any{"full_name": "Synthetic.Preview"})
	if !utf8.ValidString(body) || len(body) > defaultPreviewPageBytes || !meta.HasMore || meta.NextOffset != len(body) {
		t.Fatalf("default page is not a bounded UTF-8 page: body=%d meta=%+v", len(body), meta)
	}
	if body != large[:len(body)] {
		t.Fatal("default page changed source bytes")
	}

	small := "<p>small 🙂 preview</p>"
	ctx, session = previewSession(t, small)
	body, meta = callPreviewPage(t, ctx, session, map[string]any{"full_name": "Synthetic.Preview"})
	if body != small || meta.HasMore || meta.ReturnedBytes != len(small) || meta.Bytes != len(small) {
		t.Fatalf("small preview should remain a full response: body=%q meta=%+v", body, meta)
	}
}

func TestPreviewPagingRefusesInvalidRequestsWithoutPreviewContent(t *testing.T) {
	html := "SECRET-PREVIEW-αβγ🙂"
	ctx, session := previewSession(t, html)
	insideRune := strings.Index(html, "α") + 1
	cases := []map[string]any{
		{"full_name": "Synthetic.Preview", "max_bytes": maxPreviewPageBytes + 1},
		{"full_name": "Synthetic.Preview", "max_bytes": -1},
		{"full_name": "Synthetic.Preview", "offset": -1},
		{"full_name": "Synthetic.Preview", "offset": len(html)},
		{"full_name": "Synthetic.Preview", "offset": insideRune},
		{"full_name": "Synthetic.Preview", "expected_sha256": strings.Repeat("0", 64)},
		{"full_name": "Synthetic.Preview", "expected_sha256": "not-a-hash"},
		{"full_name": "Synthetic.Preview", "offset": strings.Index(html, "🙂"), "max_bytes": 1},
	}
	for _, args := range cases {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_component_preview", Arguments: args})
		if err != nil {
			t.Fatalf("invalid request caused protocol failure: %v", err)
		}
		if !result.IsError {
			t.Errorf("invalid request succeeded: %v", args)
			continue
		}
		for _, content := range result.Content {
			if text, ok := content.(*mcp.TextContent); ok && strings.Contains(text.Text, "SECRET-PREVIEW") {
				t.Errorf("invalid request exposed preview content: args=%v text=%q", args, text.Text)
			}
		}
		if result.StructuredContent != nil {
			t.Errorf("invalid request returned structured preview metadata: args=%v", args)
		}
	}
}

func TestPreviewPagingProtocolSchemas(t *testing.T) {
	ctx, session := previewSession(t, "<p>schema</p>")
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(listed.Tools))
	}
	tool := listed.Tools[0]
	if tool.InputSchema == nil || tool.OutputSchema == nil {
		t.Fatal("preview tool must publish input and output schemas")
	}
	in, _ := json.Marshal(tool.InputSchema)
	out, _ := json.Marshal(tool.OutputSchema)
	for _, field := range []string{"offset", "max_bytes", "expected_sha256"} {
		if !strings.Contains(string(in), `"`+field+`"`) {
			t.Errorf("input schema omits %s: %s", field, in)
		}
	}
	for _, field := range []string{"bytes", "offset", "returned_bytes", "next_offset", "has_more", "sha256"} {
		if !strings.Contains(string(out), `"`+field+`"`) {
			t.Errorf("output schema omits %s: %s", field, out)
		}
	}
}
