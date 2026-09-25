package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/geoffmilleraz/app-kit/internal/twplus/embed"
	"github.com/geoffmilleraz/app-kit/internal/twplus/index"
)

type recordingEmbedder struct {
	calls int
}

func (e *recordingEmbedder) Embed(context.Context, []string) ([]embed.Vector, error) {
	e.calls++
	return nil, nil
}

func (*recordingEmbedder) Model() string { return "recording-test" }
func (*recordingEmbedder) Dims() int     { return 2 }

func commercialSession(t *testing.T, sem *semantic) (context.Context, *mcp.ClientSession) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "components.sqlite")
	writeOptionalSourceFixture(t, path)
	ix, err := index.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	server := mcp.NewServer(&mcp.Implementation{Name: "commercial-contract-server", Version: "test"}, nil)
	registerComponentTools(server, ix, sem, ix)
	registerPreviewTool(server, nil, filepath.Join(root, "previews.sqlite"))
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "commercial-contract-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return ctx, session
}

func TestCommercialToolsAdvertiseAccurateBehavior(t *testing.T) {
	for _, tc := range []struct {
		name            string
		sem             *semantic
		searchOpenWorld bool
	}{
		{name: "local-bm25", searchOpenWorld: false},
		{name: "configured-embedder", sem: &semantic{}, searchOpenWorld: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, session := commercialSession(t, tc.sem)
			listed, err := session.ListTools(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(listed.Tools) != 5 {
				t.Fatalf("got %d commercial tools, want 5", len(listed.Tools))
			}
			for _, tool := range listed.Tools {
				if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || tool.Annotations.OpenWorldHint == nil {
					t.Errorf("%s must declare read-only and open-world behavior", tool.Name)
					continue
				}
				wantOpenWorld := tool.Name == "search_components" && tc.searchOpenWorld
				if *tool.Annotations.OpenWorldHint != wantOpenWorld {
					t.Errorf("%s openWorldHint=%v, want %v", tool.Name, *tool.Annotations.OpenWorldHint, wantOpenWorld)
				}
				if tool.InputSchema == nil || tool.OutputSchema == nil {
					t.Errorf("%s is missing typed input or output schema", tool.Name)
				}
			}
		})
	}
}

func TestCommercialToolValidationReturnsHandledErrors(t *testing.T) {
	ctx, session := commercialSession(t, nil)
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "search_components", args: map[string]any{"query": "overview", "limit": 21}, want: "limit"},
		{name: "list_components", args: map[string]any{"limit": 201}, want: "limit"},
		{name: "search_components", args: map[string]any{"query": "overview", "framework": "svelte"}, want: "framework"},
		{name: "get_component", args: map[string]any{"full_name": "Overview", "tailwind_version": 5}, want: "tailwind_version"},
		{name: "list_components", args: map[string]any{"mode": "sepia"}, want: "mode"},
		{name: "list_sections", args: map[string]any{"section": "Admin"}, want: "section"},
		{name: "get_component_preview", args: map[string]any{"full_name": "Overview", "framework": "svelte"}, want: "framework"},
	}
	for _, tc := range cases {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
		if err != nil {
			t.Fatalf("%s returned a protocol error: %v", tc.name, err)
		}
		var errorText strings.Builder
		for _, content := range result.Content {
			if text, ok := content.(*mcp.TextContent); ok {
				errorText.WriteString(text.Text)
			}
		}
		if !result.IsError || !strings.Contains(errorText.String(), tc.want) {
			t.Errorf("%s error=%q, want handled error containing %q", tc.name, errorText.String(), tc.want)
		}
	}

	// A handled validation error must not terminate or poison the MCP session.
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "search_components", Arguments: map[string]any{"query": "overview", "limit": 20}})
	if err != nil || result.IsError {
		t.Fatalf("valid call after errors failed: err=%v result=%+v", err, result)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(encoded), "Overview") {
		t.Fatalf("valid call omitted structured result: err=%v body=%s", err, encoded)
	}
}

func TestCommercialSearchAllowsFilterOnlyDiscovery(t *testing.T) {
	ctx, session := commercialSession(t, nil)
	for _, source := range []string{"tailwindplus", "matdash"} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "search_components", Arguments: map[string]any{"source": source, "limit": 1}})
		if err != nil || result.IsError {
			t.Fatalf("filter-only %s search failed: err=%v result=%+v", source, err, result)
		}
		encoded, err := json.Marshal(result.StructuredContent)
		var out searchOutput
		if err != nil || json.Unmarshal(encoded, &out) != nil || out.Count != 1 || len(out.Results) != 1 {
			t.Fatalf("filter-only %s search must return one structured hit", source)
		}
	}
}

func TestCommercialFilterOnlySearchBypassesEmbedder(t *testing.T) {
	embedder := &recordingEmbedder{}
	ctx, session := commercialSession(t, &semantic{embedder: embedder})
	for _, query := range []string{"", " \t\n "} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "search_components", Arguments: map[string]any{
			"query": query, "source": "tailwindplus", "limit": 1,
		}})
		if err != nil || result.IsError {
			t.Fatalf("filter-only search failed for query %q: err=%v result=%+v", query, err, result)
		}
		encoded, err := json.Marshal(result.StructuredContent)
		var out searchOutput
		if err != nil || json.Unmarshal(encoded, &out) != nil || out.Count != 1 || len(out.Results) != 1 {
			t.Fatalf("filter-only search for query %q must return one structural hit", query)
		}
	}
	if embedder.calls != 0 {
		t.Fatalf("filter-only search called the configured embedder %d times", embedder.calls)
	}
}
