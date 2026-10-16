package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/apps/gallery"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/adoption"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/catalogtest"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/net/html"
)

func catalogSession(t *testing.T) (context.Context, *mcp.ClientSession) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	server := mcp.NewServer(&mcp.Implementation{Name: "catalog-server-test", Version: "test"}, nil)
	if err := registerCatalogTools(server); err != nil {
		t.Fatal(err)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "catalog-client-test", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return ctx, session
}

func catalogJSONEqual(t *testing.T, got, want []byte) {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(got, &a); err != nil {
		t.Fatalf("decode actual JSON: %v", err)
	}
	if err := json.Unmarshal(want, &b); err != nil {
		t.Fatalf("decode expected JSON: %v", err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("catalog transport JSON differs\ngot: %s\nwant: %s", got, want)
	}
}

func catalogHTTP(t *testing.T, handler http.Handler, path string, status int) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Host, r.RemoteAddr = "127.0.0.1:8765", "127.0.0.1:12345"
	handler.ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("GET %s: status=%d want=%d body=%s", path, w.Code, status, w.Body.String())
	}
	return w
}

func catalogCall(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, input any) []byte {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: input})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || len(result.Content) != 1 {
		t.Fatalf("%s failed: %+v", name, result)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("%s did not return one text content block", name)
	}
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	catalogJSONEqual(t, []byte(text.Text), structured)
	return []byte(text.Text)
}

func TestCatalogMCPResourceExactGetAndGalleryParity(t *testing.T) {
	ctx, session := catalogSession(t)
	handler := gallery.New()
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Entries) == 0 {
		t.Fatal("embedded catalog contains no capabilities")
	}
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, tool := range tools.Tools {
		found[tool.Name] = true
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
			t.Errorf("%s must declare local, read-only behavior", tool.Name)
		}
		if tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Errorf("%s is missing typed input or output schema", tool.Name)
		}
	}
	if !found["search_capabilities"] || !found["get_capability"] || len(found) != 2 {
		t.Fatalf("unexpected catalog tools: %v", found)
	}
	resources, err := session.ListResources(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources.Resources) != 1 || resources.Resources[0].URI != "kit://catalog" {
		t.Fatalf("catalog resource not advertised: %+v", resources.Resources)
	}
	resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "kit://catalog"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resource.Contents) != 1 || resource.Contents[0].URI != "kit://catalog" || resource.Contents[0].MIMEType != "application/json" {
		t.Fatalf("invalid catalog resource: %+v", resource.Contents)
	}
	body := []byte(resource.Contents[0].Text)
	metadata := resources.Resources[0]
	if metadata.MIMEType != "application/json" || metadata.Size != int64(len(body)) || metadata.Annotations == nil || !slices.Contains(metadata.Annotations.Audience, mcp.Role("assistant")) || !slices.Contains(metadata.Annotations.Audience, mcp.Role("user")) {
		t.Fatalf("catalog resource metadata differs from its payload: %+v", metadata)
	}
	catalogJSONEqual(t, body, catalogHTTP(t, handler, "/api/catalog", http.StatusOK).Body.Bytes())
	want, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	catalogJSONEqual(t, body, want)
	for _, entry := range c.Entries {
		t.Run(entry.ID, func(t *testing.T) {
			got := catalogCall(t, ctx, session, "get_capability", capabilityGetInput{ID: entry.ID})
			catalogJSONEqual(t, got, catalogHTTP(t, handler, "/api/catalog/"+url.PathEscape(entry.ID), http.StatusOK).Body.Bytes())
			var exact capabilityOutput
			if err := json.Unmarshal(got, &exact); err != nil {
				t.Fatal(err)
			}
			if exact.Revision != c.Revision || !reflect.DeepEqual(exact.Entry, entry) {
				t.Fatalf("exact contract differs from source catalog: %+v", exact)
			}
			// Decode the wire field independently: absent pins must fail even if
			// both transports accidentally omit them in the same way.
			var metadata struct {
				Pin adoption.CapabilityRevision `json:"capability_revision"`
			}
			if err := json.Unmarshal(got, &metadata); err != nil {
				t.Fatal(err)
			}
			pin, err := adoption.PinCapability(c.SchemaVersion, entry)
			if err != nil {
				t.Fatal(err)
			}
			if metadata.Pin != pin {
				t.Fatalf("exact get must expose the canonical v2 metadata pin: got %+v, want %+v", metadata.Pin, pin)
			}
			if entry.Preview != "" {
				preview := catalogHTTP(t, handler, entry.Preview, http.StatusOK)
				if !strings.Contains(preview.Header().Get("Content-Type"), "text/html") || preview.Body.Len() == 0 {
					t.Fatalf("declared preview is not a rendered HTML specimen: %s", entry.Preview)
				}
			}
		})
	}
}

func TestGetCapabilityExpectedRevisionGuardsExactReview(t *testing.T) {
	ctx, session := catalogSession(t)
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	entry := c.Entries[0]
	omitted := catalogCall(t, ctx, session, "get_capability", capabilityGetInput{ID: entry.ID})
	current := catalogCall(t, ctx, session, "get_capability", map[string]any{"id": entry.ID, "expected_revision": c.Revision})
	if string(current) != string(omitted) {
		t.Fatalf("current expected_revision changed exact output\nwith guard: %s\nomitted: %s", current, omitted)
	}

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	for _, tool := range tools.Tools {
		if tool.Name != "get_capability" {
			continue
		}
		body, err := json.Marshal(tool.InputSchema)
		if err != nil || json.Unmarshal(body, &schema) != nil {
			t.Fatalf("decode get_capability input schema: %v: %s", err, body)
		}
	}
	if _, ok := schema.Properties["expected_revision"]; !ok {
		t.Fatal("get_capability input schema omits expected_revision")
	}
	if slices.Contains(schema.Required, "expected_revision") {
		t.Fatal("expected_revision must remain optional")
	}

	stale := "0" + c.Revision[1:]
	if c.Revision[0] == '0' {
		stale = "1" + c.Revision[1:]
	}
	for name, revision := range map[string]string{
		"short":      "abc",
		"uppercase":  strings.Repeat("A", 64),
		"nonhex":     strings.Repeat("g", 64),
		"whitespace": " " + c.Revision,
		"stale":      stale,
	} {
		t.Run(name, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_capability", Arguments: map[string]any{"id": entry.ID, "expected_revision": revision}})
			if err != nil {
				t.Fatalf("expected a tool-level refusal: %v", err)
			}
			if !result.IsError {
				t.Fatalf("invalid expected revision silently succeeded: %+v", result)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			if result.StructuredContent != nil {
				t.Fatalf("revision refusal leaked capability output: %s", encoded)
			}
			for _, content := range result.Content {
				text, ok := content.(*mcp.TextContent)
				if ok && (strings.Contains(text.Text, `"entry"`) || strings.Contains(text.Text, `"capability_revision"`)) {
					t.Fatalf("revision refusal leaked capability output in text: %q", text.Text)
				}
			}
			if name == "stale" && !strings.Contains(string(encoded), "restart search_capabilities") {
				t.Fatalf("stale-revision refusal lacks recovery guidance: %s", encoded)
			}
		})
	}

	// A refused request must not poison the session or the immutable catalog.
	after := catalogCall(t, ctx, session, "get_capability", map[string]any{"id": entry.ID, "expected_revision": c.Revision})
	if string(after) != string(omitted) {
		t.Fatalf("post-refusal exact output changed\nafter: %s\nomitted: %s", after, omitted)
	}
}

func TestCatalogDiscoveryEvaluationAcrossMCPAndGallery(t *testing.T) {
	ctx, session := catalogSession(t)
	handler := gallery.New()
	cases := []struct {
		name  string
		input capabilitySearchInput
		first string
		empty bool
	}{
		{"portable bar comparison", capabilitySearchInput{Query: "bar chart", Layer: "L3a", Profile: "portable"}, "analytics.bars", false},
		{"CUE config", capabilitySearchInput{Query: "CUE config", Profile: "cli"}, "cli.config", false},
		{"forms validation", capabilitySearchInput{Query: "forms validation", Profile: "go-web"}, "interaction.forms", false},
		{"Wails unavailable", capabilitySearchInput{Query: "Wails"}, "", true},
		{"NATS unavailable", capabilitySearchInput{Query: "NATS"}, "", true},
		{"full settings backend unavailable", capabilitySearchInput{Query: "full settings backend"}, "", true},
		{"no proven workflow claim", capabilitySearchInput{Maturity: "proven"}, "", true},
		{"incompatible terminal SVG", capabilitySearchInput{Layer: "L3a", Profile: "cli"}, "", true},
		{"React forms not Go wrappers", capabilitySearchInput{Query: "form validation", Profile: "react-web"}, "", true},
		{"default budget", capabilitySearchInput{Limit: 10}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := catalogCall(t, ctx, session, "search_capabilities", tc.input)
			v := url.Values{"q": {tc.input.Query}, "layer": {tc.input.Layer}, "profile": {tc.input.Profile}, "kind": {tc.input.Kind}, "maturity": {tc.input.Maturity}}
			if tc.input.Limit != 0 {
				v.Set("limit", strconv.Itoa(tc.input.Limit))
			}
			catalogJSONEqual(t, body, catalogHTTP(t, handler, "/api/catalog/search?"+v.Encode(), http.StatusOK).Body.Bytes())
			var page catalog.Page
			if err := json.Unmarshal(body, &page); err != nil {
				t.Fatal(err)
			}
			if tc.empty && (page.Total != 0 || len(page.Items) != 0 || page.NextCursor != "") {
				t.Fatalf("unavailable/incompatible capability recommended: %+v", page)
			}
			if !tc.empty && len(page.Items) == 0 {
				t.Fatal("expected a discoverable capability, got none")
			}
			if tc.first != "" && page.Items[0].ID != tc.first {
				t.Errorf("top recommendation %s, want %s", page.Items[0].ID, tc.first)
			}
			if len(body) > 16*1024 || len(page.Items) > 10 {
				t.Errorf("search exceeds discovery budget: %d bytes, %d items", len(body), len(page.Items))
			}
			for _, forbidden := range []string{`"source"`, `"usage"`, `"proof"`, `"dependencies"`, "package ", "func "} {
				if strings.Contains(string(body), forbidden) {
					t.Errorf("bounded search leaked full contract/source: %q", forbidden)
				}
			}
			if page.NextCursor != "" {
				next := tc.input
				next.Cursor = page.NextCursor
				v.Set("cursor", page.NextCursor)
				catalogJSONEqual(t, catalogCall(t, ctx, session, "search_capabilities", next), catalogHTTP(t, handler, "/api/catalog/search?"+v.Encode(), http.StatusOK).Body.Bytes())
			}
		})
	}
}

func TestCatalogTaskCorpusAcrossMCPAndGallery(t *testing.T) {
	ctx, session := catalogSession(t)
	handler := gallery.New()
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	catalogtest.Run(t, c, func(t *testing.T, q catalog.Query) []byte {
		t.Helper()
		input := capabilitySearchInput{Query: q.Text, Layer: q.Layer, Profile: q.Profile, Kind: q.Kind, Maturity: q.Maturity, Limit: q.Limit, Cursor: q.Cursor}
		body := catalogCall(t, ctx, session, "search_capabilities", input)
		v := url.Values{"q": {q.Text}, "layer": {q.Layer}, "profile": {q.Profile}, "kind": {q.Kind}, "maturity": {q.Maturity}, "cursor": {q.Cursor}}
		if q.Limit != 0 {
			v.Set("limit", strconv.Itoa(q.Limit))
		}
		catalogJSONEqual(t, body, catalogHTTP(t, handler, "/api/catalog/search?"+v.Encode(), http.StatusOK).Body.Bytes())
		var page catalog.Page
		if err := json.Unmarshal(body, &page); err != nil {
			t.Fatal(err)
		}
		rendered := catalogHTTP(t, handler, "/catalog?"+v.Encode(), http.StatusOK)
		doc, err := html.Parse(rendered.Body)
		if err != nil {
			t.Fatal(err)
		}
		var ids, want []string
		var visit func(*html.Node)
		visit = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "a" {
				for _, a := range n.Attr {
					if a.Key == "href" {
						u, err := url.Parse(a.Val)
						if err == nil && strings.HasPrefix(u.Path, "/catalog/") {
							ids = append(ids, strings.TrimPrefix(u.Path, "/catalog/"))
						}
					}
				}
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				visit(child)
			}
		}
		visit(doc)
		for _, item := range page.Items {
			want = append(want, item.ID)
		}
		if !slices.Equal(ids, want) {
			t.Fatalf("rendered Gallery recommendations %v differ from protocol %v", ids, want)
		}
		return body
	})
}

func TestCatalogTransportRejectsUnknownRequests(t *testing.T) {
	ctx, session := catalogSession(t)
	handler := gallery.New()
	for _, tc := range []struct {
		name, path string
		input      any
		status     int
	}{
		{"get_capability", "/api/catalog/missing", capabilityGetInput{ID: "missing"}, http.StatusNotFound},
		{"search_capabilities", "/api/catalog/search?profile=desktop", capabilitySearchInput{Profile: "desktop"}, http.StatusBadRequest},
		{"search_capabilities", "/api/catalog/search?layer=L9", capabilitySearchInput{Layer: "L9"}, http.StatusBadRequest},
		{"search_capabilities", "/api/catalog/search?kind=framework", capabilitySearchInput{Kind: "framework"}, http.StatusBadRequest},
		{"search_capabilities", "/api/catalog/search?maturity=complete", capabilitySearchInput{Maturity: "complete"}, http.StatusBadRequest},
		{"search_capabilities", "/api/catalog/search?limit=51", capabilitySearchInput{Limit: 51}, http.StatusBadRequest},
		{"search_capabilities", "/api/catalog/search?cursor=bad", capabilitySearchInput{Cursor: "bad"}, http.StatusBadRequest},
	} {
		t.Run(tc.path, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: tc.input})
			if err != nil {
				t.Fatalf("expected a tool-level error response: %v", err)
			}
			if !result.IsError {
				t.Fatalf("invalid request silently succeeded: %+v", result)
			}
			catalogHTTP(t, handler, tc.path, tc.status)
		})
	}
}
