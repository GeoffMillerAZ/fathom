package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/app-kit/apps/gallery"
	"github.com/geoffmilleraz/app-kit/internal/qualityplan"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func qualitySession(t *testing.T) (context.Context, *mcp.ClientSession) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	server := mcp.NewServer(&mcp.Implementation{Name: "quality-test", Version: "test"}, nil)
	if err := registerQualityTools(server); err != nil {
		t.Fatal(err)
	}
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "quality-client", Version: "test"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return ctx, cs
}

func TestQualityToolsExactAuthoredProjection(t *testing.T) {
	ctx, session := qualitySession(t)
	plan, err := qualityplan.Default()
	if err != nil {
		t.Fatal(err)
	}
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 2 {
		t.Fatalf("unexpected tools: %+v", tools.Tools)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "list_quality_work" && tool.Name != "get_quality_work" {
			t.Fatal(tool.Name)
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint || tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Fatalf("missing local/read-only typed contract: %+v", tool)
		}
	}
	for _, scope := range []string{"", "core", "on-demand", "all"} {
		t.Run("scope="+scope, func(t *testing.T) {
			want, err := plan.List("", scope)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := json.Marshal(want)
			got := catalogCall(t, ctx, session, "list_quality_work", qualityListInput{Scope: scope})
			catalogJSONEqual(t, got, body)
			if len(got) > 64<<10 {
				t.Fatal("list exceeds byte budget")
			}
			for _, item := range want.Items {
				detail, err := plan.Get(item.ID, want.Revision)
				if err != nil {
					t.Fatal(err)
				}
				detailBody, _ := json.Marshal(detail)
				actual := catalogCall(t, ctx, session, "get_quality_work", qualityGetInput{ID: item.ID, ExpectedRevision: want.Revision})
				catalogJSONEqual(t, actual, detailBody)
				if len(actual) > 128<<10 {
					t.Fatal("detail exceeds byte budget")
				}
				var wire map[string]json.RawMessage
				if err := json.Unmarshal(actual, &wire); err != nil {
					t.Fatal(err)
				}
				if len(wire["claim_boundary"]) == 0 || len(wire["backlog_policy"]) == 0 || len(wire["standards"]) == 0 {
					t.Fatal("missing provenance/standard context")
				}
			}
		})
	}
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	empty := false
	for _, entry := range c.Entries {
		want, err := plan.List(entry.ID, "all")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(want)
		got := catalogCall(t, ctx, session, "list_quality_work", qualityListInput{CapabilityID: entry.ID, Scope: "all"})
		catalogJSONEqual(t, got, body)
		empty = empty || len(want.Items) == 0
	}
	if !empty {
		t.Fatal("expected an unrepresented capability to prove empty is not a pass")
	}
	// Preserve exact consumer states without freezing today's authored status.
	shell := catalogCall(t, ctx, session, "get_quality_work", qualityGetInput{ID: "core.shell-settings"})
	var observed qualityplan.DetailResult
	if err := json.Unmarshal(shell, &observed); err != nil {
		t.Fatal(err)
	}
	wantShell, err := plan.Get("core.shell-settings", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(observed.Item.Consumers) != len(wantShell.Item.Consumers) {
		t.Fatal("consumer evidence was lost")
	}
	for i, consumer := range observed.Item.Consumers {
		if consumer.IntegrationStatus != wantShell.Item.Consumers[i].IntegrationStatus {
			t.Fatal("consumer state was silently promoted")
		}
	}
}

func TestQualityToolsRefuseInvalidOrStaleSelection(t *testing.T) {
	ctx, session := qualitySession(t)
	cases := []struct {
		name, tool string
		input      any
	}{
		{"unknown scope", "list_quality_work", qualityListInput{Scope: "ready"}},
		{"unknown capability", "list_quality_work", qualityListInput{CapabilityID: "not.a.capability"}},
		{"overlong capability", "list_quality_work", qualityListInput{CapabilityID: strings.Repeat("x", 97)}},
		{"capability whitespace", "list_quality_work", qualityListInput{CapabilityID: " shell.layout"}},
		{"unknown item", "get_quality_work", qualityGetInput{ID: "not.a.work-item"}},
		{"empty item", "get_quality_work", qualityGetInput{}},
		{"missing item", "get_quality_work", map[string]any{}},
		{"wrong input type", "get_quality_work", map[string]any{"id": 42}},
		{"path is not an ID", "get_quality_work", qualityGetInput{ID: "../../inventory.cue"}},
		{"stale revision", "get_quality_work", qualityGetInput{ID: "core.analytics", ExpectedRevision: strings.Repeat("0", 64)}},
		{"malformed revision", "get_quality_work", qualityGetInput{ID: "core.analytics", ExpectedRevision: "current"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tc.tool, Arguments: tc.input})
			if err == nil && (r == nil || !r.IsError) {
				t.Fatalf("invalid selection was not refused: %+v", r)
			}
		})
	}
}

func TestQualityGuidanceUsesBoundedLookupBeforeRawInventory(t *testing.T) {
	text := loadKitDocs("../..").guidance()
	for _, fragment := range []string{"list_quality_work", "get_quality_work", "embedded", "quality-inventory", "schema validity never certifies quality"} {
		if !strings.Contains(text, fragment) {
			t.Errorf("missing guidance %q", fragment)
		}
	}
	startup := serverInstructions(nil, nil, nil)
	for _, fragment := range []string{"list_quality_work", "get_quality_work", "embedded authored", "not live evidence checks or certification"} {
		if !strings.Contains(startup, fragment) {
			t.Errorf("missing startup instruction %q", fragment)
		}
	}
}

func TestQualityMCPAndGalleryShareAuthoredSnapshot(t *testing.T) {
	ctx, session := qualitySession(t)
	handler := gallery.New()
	plan, err := qualityplan.Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"", "core", "on-demand", "all"} {
		input := qualityListInput{Scope: scope}
		got := catalogCall(t, ctx, session, "list_quality_work", input)
		catalogJSONEqual(t, got, catalogHTTP(t, handler, "/api/quality?scope="+url.QueryEscape(scope), http.StatusOK).Body.Bytes())
	}
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range c.Entries {
		got := catalogCall(t, ctx, session, "list_quality_work", qualityListInput{Scope: "all", CapabilityID: entry.ID})
		query := url.Values{"scope": {"all"}, "capability_id": {entry.ID}}
		catalogJSONEqual(t, got, catalogHTTP(t, handler, "/api/quality?"+query.Encode(), http.StatusOK).Body.Bytes())
	}
	all, err := plan.List("", "all")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range all.Items {
		got := catalogCall(t, ctx, session, "get_quality_work", qualityGetInput{ID: item.ID, ExpectedRevision: all.Revision})
		catalogJSONEqual(t, got, catalogHTTP(t, handler, "/api/quality/"+url.PathEscape(item.ID)+"?expected_revision="+all.Revision, http.StatusOK).Body.Bytes())
	}
}
