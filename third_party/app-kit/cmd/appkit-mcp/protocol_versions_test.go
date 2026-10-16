package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSupportedProtocolVersionsServeFirstPartyContracts(t *testing.T) {
	versions := []string{"2025-03-26", "2025-06-18", "2025-11-25", "2026-07-28"}
	for _, version := range versions {
		t.Run(version, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			docs := loadKitDocs(kitRootRel)
			server := mcp.NewServer(&mcp.Implementation{Name: "appkit", Version: "protocol-test"}, &mcp.ServerOptions{
				Instructions: serverInstructions(nil, nil, docs),
			})
			if err := registerCatalogTools(server); err != nil {
				t.Fatal(err)
			}
			if err := registerQualityTools(server); err != nil {
				t.Fatal(err)
			}
			registerKitTools(server, docs)

			serverTransport, clientTransport := mcp.NewInMemoryTransports()
			serverSession, err := server.Connect(ctx, serverTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			serverClosed := false
			t.Cleanup(func() {
				if !serverClosed {
					if err := serverSession.Close(); err != nil {
						t.Errorf("server cleanup: %v", err)
					}
				}
			})

			client := mcp.NewClient(&mcp.Implementation{Name: "appkit-protocol-test", Version: "test"}, nil)
			clientSession, err := client.Connect(ctx, clientTransport, &mcp.ClientSessionOptions{ProtocolVersion: version})
			if err != nil {
				t.Fatal(err)
			}
			clientClosed := false
			t.Cleanup(func() {
				if !clientClosed {
					if err := clientSession.Close(); err != nil {
						t.Errorf("client cleanup: %v", err)
					}
				}
			})

			initialized := clientSession.InitializeResult()
			if initialized == nil || initialized.ProtocolVersion != version {
				t.Fatalf("negotiated protocol = %v, want %q", initialized, version)
			}

			listed, err := clientSession.ListTools(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			found := make(map[string]bool, len(listed.Tools))
			for _, tool := range listed.Tools {
				found[tool.Name] = true
			}
			for _, name := range []string{"search_capabilities", "get_capability", "get_kit_decision"} {
				if !found[name] {
					t.Errorf("protocol %s omitted tool %q", version, name)
				}
			}

			search, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
				Name: "search_capabilities", Arguments: map[string]any{"limit": 1},
			})
			if err != nil || search == nil || search.IsError {
				t.Fatal("first-party capability search failed")
			}
			var page catalog.Page
			decodeProtocolStructuredContent(t, search, &page)
			if page.Revision == "" || page.Total < 1 || len(page.Items) != 1 {
				t.Fatalf("protocol %s returned an invalid bounded capability page", version)
			}

			wantIntent, err := os.ReadFile(filepath.Join(kitRootRel, "docs", "intent", "README.md"))
			if err != nil || len(wantIntent) == 0 {
				t.Fatal("read intent router")
			}
			intent, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{URI: "kit://intent"})
			if err != nil || intent == nil || len(intent.Contents) != 1 || intent.Contents[0].Text != string(wantIntent) {
				t.Fatalf("protocol %s did not serve the exact intent resource", version)
			}

			invalid, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
				Name: "get_capability", Arguments: map[string]any{"id": "not.a.real.capability"},
			})
			if err != nil || invalid == nil || !invalid.IsError {
				t.Fatalf("protocol %s did not return a handled tool error", version)
			}
			valid, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
				Name: "get_capability", Arguments: map[string]any{"id": page.Items[0].ID},
			})
			if err != nil || valid == nil || valid.IsError {
				t.Fatalf("protocol %s session did not recover after a tool error", version)
			}
			var exact capabilityOutput
			decodeProtocolStructuredContent(t, valid, &exact)
			if exact.Revision != page.Revision || exact.Entry.ID != page.Items[0].ID {
				t.Fatalf("protocol %s exact retrieval disagreed with discovery", version)
			}

			if err := clientSession.Close(); err != nil {
				t.Fatalf("protocol %s client close: %v", version, err)
			}
			clientClosed = true
			if err := serverSession.Close(); err != nil {
				t.Fatalf("protocol %s server close: %v", version, err)
			}
			serverClosed = true
			if ctx.Err() != nil {
				t.Fatalf("protocol %s exceeded its timeout", version)
			}
		})
	}
}

func decodeProtocolStructuredContent(t *testing.T, result *mcp.CallToolResult, out any) {
	t.Helper()
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || string(body) == "null" || json.Unmarshal(body, out) != nil {
		t.Fatal("MCP tool returned invalid structured content")
	}
}
