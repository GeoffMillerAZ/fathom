package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/apps/gallery"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/qualityplan"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestQualityCatalogRevisionAdvertisedAndTransportParity(t *testing.T) {
	ctx, session := qualitySession(t)
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, tool := range tools.Tools {
		if tool.Name != "list_quality_work" && tool.Name != "get_quality_work" {
			continue
		}
		found[tool.Name] = true
		body, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var schema map[string]any
		if err := json.Unmarshal(body, &schema); err != nil {
			t.Fatal(err)
		}
		properties, _ := schema["properties"].(map[string]any)
		if _, ok := properties["expected_catalog_revision"]; !ok {
			t.Errorf("%s input schema omits expected_catalog_revision", tool.Name)
		}
		if required, ok := schema["required"].([]any); ok {
			for _, field := range required {
				if field == "expected_catalog_revision" {
					t.Errorf("%s requires expected_catalog_revision", tool.Name)
				}
			}
		}
	}
	if !found["list_quality_work"] || !found["get_quality_work"] {
		t.Fatalf("quality tools not advertised: %v", found)
	}

	listInput := map[string]any{"scope": "all"}
	listOmitted := catalogCall(t, ctx, session, "list_quality_work", listInput)
	var list map[string]any
	if err := json.Unmarshal(listOmitted, &list); err != nil {
		t.Fatal(err)
	}
	if list["catalog_revision"] != c.Revision {
		t.Fatalf("list catalog_revision=%v want=%s", list["catalog_revision"], c.Revision)
	}
	qualityRevision, ok := list["revision"].(string)
	if !ok || qualityRevision == "" {
		t.Fatalf("list revision missing: %s", listOmitted)
	}
	for name, guard := range map[string]string{"current": c.Revision, "empty": ""} {
		got := catalogCall(t, ctx, session, "list_quality_work", map[string]any{"scope": "all", "expected_catalog_revision": guard})
		if !bytes.Equal(got, listOmitted) {
			t.Errorf("list %s guard changed bytes\ngot: %s\nwant: %s", name, got, listOmitted)
		}
	}

	getInput := map[string]any{"id": "core.shell-settings", "expected_revision": qualityRevision}
	getOmitted := catalogCall(t, ctx, session, "get_quality_work", getInput)
	var detail map[string]any
	if err := json.Unmarshal(getOmitted, &detail); err != nil {
		t.Fatal(err)
	}
	if detail["catalog_revision"] != c.Revision {
		t.Fatalf("get catalog_revision=%v want=%s", detail["catalog_revision"], c.Revision)
	}
	for name, guard := range map[string]string{"current": c.Revision, "empty": ""} {
		got := catalogCall(t, ctx, session, "get_quality_work", map[string]any{"id": "core.shell-settings", "expected_revision": qualityRevision, "expected_catalog_revision": guard})
		if !bytes.Equal(got, getOmitted) {
			t.Errorf("get %s guard changed bytes\ngot: %s\nwant: %s", name, got, getOmitted)
		}
	}

	handler := gallery.New()
	listPath := "/api/quality?scope=all"
	if got := catalogHTTP(t, handler, listPath, http.StatusOK).Body.Bytes(); !bytes.Equal(got, listOmitted) {
		t.Errorf("Gallery list differs from MCP\ngot: %s\nwant: %s", got, listOmitted)
	}
	getPath := "/api/quality/core.shell-settings?expected_revision=" + url.QueryEscape(qualityRevision)
	if got := catalogHTTP(t, handler, getPath, http.StatusOK).Body.Bytes(); !bytes.Equal(got, getOmitted) {
		t.Errorf("Gallery get differs from MCP\ngot: %s\nwant: %s", got, getOmitted)
	}
}

func TestQualityCatalogRevisionRejectsInvalidBeforePayload(t *testing.T) {
	ctx, session := qualitySession(t)
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	stale := "0" + c.Revision[1:]
	if stale == c.Revision {
		stale = "1" + c.Revision[1:]
	}
	invalid := map[string]string{
		"short":     "abc",
		"uppercase": strings.Repeat("A", 64),
		"nonhex":    strings.Repeat("g", 64),
		"overlong":  strings.Repeat("0", 65),
		"stale":     stale,
	}
	for name, guard := range invalid {
		for _, tc := range []struct {
			name string
			tool string
			args map[string]any
		}{
			{name: "list", tool: "list_quality_work", args: map[string]any{"capability_id": "analytics.comparison", "scope": "all", "expected_catalog_revision": guard}},
			{name: "get", tool: "get_quality_work", args: map[string]any{"id": "core.shell-settings", "expected_catalog_revision": guard}},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tc.tool, Arguments: tc.args})
				if err != nil {
					t.Fatal(err)
				}
				if result == nil || !result.IsError {
					t.Fatalf("%s accepted invalid catalog guard %q: %+v", tc.tool, guard, result)
				}
				if result.StructuredContent != nil {
					t.Fatalf("%s returned structured records after rejected guard: %+v", tc.tool, result.StructuredContent)
				}
				content, err := json.Marshal(result.Content)
				if err != nil {
					t.Fatal(err)
				}
				for _, field := range []string{"claim_boundary", "backlog_policy", "enhancement", "capability_ids"} {
					if bytes.Contains(content, []byte(field)) {
						t.Errorf("%s leaked quality payload field %q: %s", tc.tool, field, content)
					}
				}
			})
		}
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_quality_work", Arguments: map[string]any{"scope": "all", "expected_catalog_revision": stale}})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || !result.IsError || result.StructuredContent != nil {
		t.Fatalf("unfiltered list ignored stale catalog guard: %+v", result)
	}
}

func TestQualityCatalogRevisionPreservesInventoryGuard(t *testing.T) {
	ctx, session := qualitySession(t)
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_quality_work",
		Arguments: map[string]any{
			"id":                        "core.shell-settings",
			"expected_revision":         strings.Repeat("0", 64),
			"expected_catalog_revision": c.Revision,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || !result.IsError || result.StructuredContent != nil {
		t.Fatalf("get accepted stale quality revision with current catalog guard: %+v", result)
	}
}

func TestQualityCatalogRevisionRefusesCatalogOnlyDriftAcrossServers(t *testing.T) {
	catalogA, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	entries := append([]catalog.Entry(nil), catalogA.Entries...)
	changed := false
	for i := range entries {
		if entries[i].ID == "analytics.comparison" {
			entries[i].Usage += " Synthetic changed MCP integration contract."
			changed = true
		}
	}
	if !changed {
		t.Fatal("analytics.comparison fixture capability missing")
	}
	body, err := json.Marshal(map[string]any{"schema_version": catalogA.SchemaVersion, "entries": entries})
	if err != nil {
		t.Fatal(err)
	}
	catalogB, err := catalog.Load(body)
	if err != nil {
		t.Fatal(err)
	}
	if catalogA.Revision == catalogB.Revision {
		t.Fatal("catalog-only fixture did not change revision")
	}
	planA, err := qualityplan.ForCatalog(catalogA)
	if err != nil {
		t.Fatal(err)
	}
	planB, err := qualityplan.ForCatalog(catalogB)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	connect := func(name string, plan *qualityplan.Snapshot) *mcp.ClientSession {
		t.Helper()
		server := mcp.NewServer(&mcp.Implementation{Name: name + "-server", Version: "test"}, nil)
		registerQualitySnapshot(server, plan)
		serverTransport, clientTransport := mcp.NewInMemoryTransports()
		serverSession, err := server.Connect(ctx, serverTransport, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = serverSession.Close() })
		client := mcp.NewClient(&mcp.Implementation{Name: name + "-client", Version: "test"}, nil)
		session, err := client.Connect(ctx, clientTransport, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = session.Close() })
		return session
	}
	sessionA, sessionB := connect("catalog-a", planA), connect("catalog-b", planB)

	listABytes := catalogCall(t, ctx, sessionA, "list_quality_work", map[string]any{"scope": "all"})
	listBBytes := catalogCall(t, ctx, sessionB, "list_quality_work", map[string]any{"scope": "all"})
	var listA, listB map[string]any
	if err := json.Unmarshal(listABytes, &listA); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(listBBytes, &listB); err != nil {
		t.Fatal(err)
	}
	if listA["revision"] != listB["revision"] {
		t.Fatalf("catalog-only drift changed inventory revision: A=%v B=%v", listA["revision"], listB["revision"])
	}
	if listA["catalog_revision"] != catalogA.Revision || listB["catalog_revision"] != catalogB.Revision {
		t.Fatalf("servers did not expose their catalog identity: A=%v B=%v", listA["catalog_revision"], listB["catalog_revision"])
	}
	guardedListB := catalogCall(t, ctx, sessionB, "list_quality_work", map[string]any{"scope": "all", "expected_catalog_revision": catalogB.Revision})
	if !bytes.Equal(guardedListB, listBBytes) {
		t.Fatalf("current catalog guard changed list bytes\ngot: %s\nwant: %s", guardedListB, listBBytes)
	}
	qualityRevision := listA["revision"].(string)
	getB := catalogCall(t, ctx, sessionB, "get_quality_work", map[string]any{"id": "core.analytics", "expected_revision": qualityRevision})
	guardedGetB := catalogCall(t, ctx, sessionB, "get_quality_work", map[string]any{"id": "core.analytics", "expected_revision": qualityRevision, "expected_catalog_revision": catalogB.Revision})
	if !bytes.Equal(guardedGetB, getB) {
		t.Fatalf("current catalog guard changed get bytes\ngot: %s\nwant: %s", guardedGetB, getB)
	}

	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"list_quality_work", map[string]any{"scope": "all", "expected_catalog_revision": catalogA.Revision}},
		{"get_quality_work", map[string]any{"id": "core.analytics", "expected_revision": qualityRevision, "expected_catalog_revision": catalogA.Revision}},
	} {
		result, err := sessionB.CallTool(ctx, &mcp.CallToolParams{Name: tc.tool, Arguments: tc.args})
		if err != nil {
			t.Fatal(err)
		}
		if result == nil || !result.IsError || result.StructuredContent != nil {
			t.Errorf("%s did not refuse catalog-only drift before payload: %+v", tc.tool, result)
		}
	}

	for source, guidance := range map[string]string{
		"served":  loadKitDocs("../..").guidance(),
		"startup": serverInstructions(nil, nil, nil),
	} {
		if !strings.Contains(guidance, "expected_catalog_revision") {
			t.Errorf("%s guidance omits expected_catalog_revision", source)
		}
	}
}
