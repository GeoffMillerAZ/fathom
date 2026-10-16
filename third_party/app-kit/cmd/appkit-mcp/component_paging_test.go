package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/twplus/index"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/twplus/normalize"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/twplus/parse"
)

func pagedComponentSession(t *testing.T, count int) (context.Context, *mcp.ClientSession) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "components.sqlite")
	builder, err := index.NewBuilder(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = builder.Close() })
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("Synthetic %03d", i)
		component, rows, err := normalize.Build(parse.Component{
			Path: []string{"Application UI", "Layout", "Synthetic", name},
			Raw:  parse.RawComponent{Name: name, Snippets: []parse.RawSnippet{{Code: "<div>" + name + "</div>", Language: "html", Name: "html", Version: 4}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := builder.AddComponent(component, rows); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := builder.Finalize(); err != nil {
		t.Fatal(err)
	}
	ix, err := index.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	server := mcp.NewServer(&mcp.Implementation{Name: "component-paging-server", Version: "test"}, nil)
	registerComponentTools(server, ix, nil, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "component-paging-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return ctx, session
}

func listComponentPage(t *testing.T, ctx context.Context, session *mcp.ClientSession, args map[string]any) listOutput {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_components", Arguments: args})
	if err != nil {
		t.Fatalf("list_components protocol failure: %v", err)
	}
	if result.IsError {
		t.Fatalf("list_components tool failure: %+v", result)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out listOutput
	if err := json.Unmarshal(encoded, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestListComponentsPagesCompleteUnionWithoutDuplicates(t *testing.T) {
	ctx, session := pagedComponentSession(t, 205)
	args := map[string]any{"section": "Application UI", "framework": "html", "tailwind_version": 4}
	seen := map[string]bool{}
	offset := 0
	pages := 0
	for {
		args["offset"] = offset
		page := listComponentPage(t, ctx, session, args)
		pages++
		if page.Offset != offset || page.Count != len(page.Components) {
			t.Fatalf("page metadata mismatch at offset %d: %+v", offset, page)
		}
		for _, component := range page.Components {
			if seen[component.FullName] {
				t.Fatalf("duplicate component across pages: %s", component.FullName)
			}
			seen[component.FullName] = true
		}
		if !page.HasMore {
			if page.NextOffset != 0 {
				t.Fatalf("final page next_offset=%d, want omitted/zero", page.NextOffset)
			}
			break
		}
		if page.NextOffset != offset+page.Count {
			t.Fatalf("next_offset=%d, want %d", page.NextOffset, offset+page.Count)
		}
		offset = page.NextOffset
	}
	if pages != 2 || len(seen) != 205 {
		t.Fatalf("pages=%d unique=%d, want 2 pages and 205 records", pages, len(seen))
	}
	for i := 0; i < 205; i++ {
		want := "Application UI.Layout.Synthetic." + fmt.Sprintf("Synthetic %03d", i)
		if !seen[want] {
			t.Errorf("missing %s", want)
		}
	}
}

func TestListComponentsRejectsInvalidOffsetWithoutPartialPage(t *testing.T) {
	ctx, session := pagedComponentSession(t, 2)
	for _, offset := range []int{-1, math.MaxInt} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_components", Arguments: map[string]any{"offset": offset}})
		if err != nil {
			t.Fatalf("offset %d caused protocol failure: %v", offset, err)
		}
		if !result.IsError || result.StructuredContent != nil {
			t.Errorf("offset %d did not return a handled empty error: %+v", offset, result)
		}
		for _, content := range result.Content {
			if text, ok := content.(*mcp.TextContent); ok && strings.Contains(text.Text, "Synthetic 000") {
				t.Errorf("offset %d exposed a partial component page: %q", offset, text.Text)
			}
		}
	}
}

func TestListComponentsPagingProtocolSchema(t *testing.T) {
	ctx, session := pagedComponentSession(t, 1)
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range listed.Tools {
		if tool.Name != "list_components" {
			continue
		}
		in, _ := json.Marshal(tool.InputSchema)
		out, _ := json.Marshal(tool.OutputSchema)
		if !strings.Contains(string(in), `"offset"`) {
			t.Fatalf("list_components input schema omits offset: %s", in)
		}
		for _, field := range []string{"offset", "next_offset", "has_more"} {
			if !strings.Contains(string(out), `"`+field+`"`) {
				t.Errorf("list_components output schema omits %s: %s", field, out)
			}
		}
		return
	}
	t.Fatal("list_components was not registered")
}
