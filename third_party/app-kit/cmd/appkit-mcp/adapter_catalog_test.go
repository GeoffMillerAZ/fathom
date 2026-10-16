package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPrivateAdapterCatalogMCPUsesSharedMetadataWithoutLicensedCorpora(t *testing.T) {
	ctx, session := catalogSession(t)
	wantCatalog, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 2 {
		t.Fatalf("adapter metadata added an MCP tool: %+v", tools.Tools)
	}
	wantTools := map[string]bool{"search_capabilities": true, "get_capability": true}
	for _, tool := range tools.Tools {
		if !wantTools[tool.Name] {
			t.Errorf("unexpected catalog tool %q", tool.Name)
		}
	}

	resources, err := session.ListResources(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources.Resources) != 1 || resources.Resources[0].URI != "kit://catalog" {
		t.Fatalf("unexpected first-party catalog resources: %+v", resources.Resources)
	}
	resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "kit://catalog"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resource.Contents) != 1 {
		t.Fatalf("catalog resource returned %d contents", len(resource.Contents))
	}
	var gotCatalog catalog.Catalog
	if err := json.Unmarshal([]byte(resource.Contents[0].Text), &gotCatalog); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotCatalog.Entries, wantCatalog.Entries) || gotCatalog.Revision != wantCatalog.Revision || gotCatalog.SchemaVersion != wantCatalog.SchemaVersion {
		t.Fatal("MCP catalog resource differs from catalog.Default")
	}

	adapters := []struct {
		id, query string
	}{
		{"reference.pricing-adapter", "pricing private adapter"},
		{"reference.directory-adapter", "directory table private adapter"},
		{"reference.form-adapter", "form email field private adapter"},
		{"reference.navigation-adapter", "page navigation private adapter"},
		{"reference.faq-adapter", "FAQ help private adapter"},
		{"reference.hero-adapter", "hero private adapter"},
		{"reference.menu-adapter", "dropdown menu private adapter"},
		{"reference.drawer-adapter", "profile drawer private adapter"},
		{"reference.navbar-adapter", "responsive navbar private adapter"},
	}
	if len(adapters) != 9 {
		t.Fatalf("private adapter MCP inventory count = %d, want 9", len(adapters))
	}
	for _, adapter := range adapters {
		t.Run(adapter.id, func(t *testing.T) {
			for _, input := range []capabilitySearchInput{
				{Query: adapter.query},
				{Query: adapter.query, Profile: "cli"},
			} {
				body := catalogCall(t, ctx, session, "search_capabilities", input)
				var got catalog.Page
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatal(err)
				}
				want, err := wantCatalog.Search(catalog.Query{Text: input.Query, Profile: input.Profile})
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) || got.Revision != wantCatalog.Revision || got.Total != 1 || len(got.Items) != 1 || got.Items[0].ID != adapter.id {
					t.Fatalf("MCP search differs from shared catalog\ngot:  %+v\nwant: %+v", got, want)
				}
				if len(body) > 4<<10 {
					t.Fatalf("MCP search exceeds bounded summary budget: %d bytes", len(body))
				}
				for _, forbidden := range []string{`"source"`, `"usage"`, `"proof"`, `"provenance"`, "WritePricingDelivery", "WriteDirectoryDelivery", "WriteFormDelivery", "WriteNavigationDelivery", "WriteMenuDelivery"} {
					if strings.Contains(string(body), forbidden) {
						t.Errorf("MCP discovery exposed exact contract/source material %q", forbidden)
					}
				}
			}
			for _, input := range []capabilitySearchInput{
				{Query: adapter.query, Profile: "go-web"},
				{Query: adapter.query, Profile: "static-web"},
				{Query: adapter.query, Profile: "react-web"},
				{Query: adapter.query, Maturity: "available"},
				{Query: adapter.query, Maturity: "proven"},
				{Query: adapter.query, Kind: "recipe"},
				{Query: adapter.query, Layer: "L4"},
			} {
				body := catalogCall(t, ctx, session, "search_capabilities", input)
				var got catalog.Page
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatal(err)
				}
				want, err := wantCatalog.Search(catalog.Query{
					Text: input.Query, Layer: input.Layer, Profile: input.Profile,
					Kind: input.Kind, Maturity: input.Maturity,
				})
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) || got.Revision != wantCatalog.Revision || got.Total != 0 || len(got.Items) != 0 || got.NextCursor != "" {
					t.Errorf("MCP promoted unsupported adapter fit for %+v: %+v", input, got)
				}
			}

			body := catalogCall(t, ctx, session, "get_capability", capabilityGetInput{ID: adapter.id})
			var got capabilityOutput
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatal(err)
			}
			want, ok := wantCatalog.Get(adapter.id)
			if !ok {
				t.Fatalf("catalog.Default is missing %q", adapter.id)
			}
			if got.Revision != wantCatalog.Revision || !reflect.DeepEqual(got.Entry, want) {
				t.Fatalf("MCP exact get differs from shared record\ngot:  %+v\nwant: %+v", got, capabilityOutput{Revision: wantCatalog.Revision, Entry: want})
			}
			if got.Entry.Usage == "" || len(got.Entry.Proof) == 0 || got.Entry.Provenance.Source == "" || got.Entry.Provenance.License == "" || got.Entry.Provenance.Audience == "" {
				t.Fatalf("MCP exact get omitted usage, evidence, or provenance: %+v", got.Entry)
			}
		})
	}
}
