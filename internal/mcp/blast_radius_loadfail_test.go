// Package mcp_test — EW-78 (O1013, I38): a call graph that cannot be loaded
// is named, never answered as "no symbols for service" for an indexed one.
package mcp_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// graphLoadFailBackend is an indexed service whose symbol listing (the nav
// graphs' load) fails, as a store read does under memory pressure.
type graphLoadFailBackend struct {
	extendedFakeBackend
}

var errGraphLoad = errors.New("store read: interrupted by the memory governor")

func (b *graphLoadFailBackend) ListAllSymbols(context.Context, string) ([]core.SymbolNode, error) {
	return nil, errGraphLoad
}

// TestBlastRadiusNamesGraphLoadFailure pins EW-78: blast_radius kinds
// change, rename and delete each answer the load failure by name, and the
// tenancy level with its budget knob when the daemon is at shed or refuse.
func TestBlastRadiusNamesGraphLoadFailure(t *testing.T) {
	fake := &graphLoadFailBackend{extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}, svcPath: t.TempDir()}}
	srv := mcp.NewServer(fake)
	t.Cleanup(func() { tenancy.PublishLevel(tenancy.LevelNormal) })
	args := map[string]map[string]any{
		"change": {"service": "svc", "changed_files": []string{"pkg/a.go"}},
		"rename": {"service": "svc", "kind": "rename", "symbol": "B"},
		"delete": {"service": "svc", "kind": "delete", "symbol": "B"},
	}
	errorText := func(t *testing.T, kind string) string {
		t.Helper()
		resp := roundtrip(t, srv, request{JSONRPC: "2.0", ID: 1, Method: "tools/call",
			Params: map[string]any{"name": "blast_radius", "arguments": args[kind]}})
		e, ok := resp["error"].(map[string]any)
		if !ok {
			t.Fatalf("kind %s: a failed graph load answered without an error: %v", kind, resp)
		}
		msg, _ := e["message"].(string)
		return msg
	}
	for _, lvl := range []tenancy.Level{tenancy.LevelNormal, tenancy.LevelShed, tenancy.LevelRefuse} {
		tenancy.PublishLevel(lvl)
		for _, kind := range []string{"change", "rename", "delete"} {
			msg := errorText(t, kind)
			if strings.Contains(msg, "no symbols") {
				t.Errorf("level %s, kind %s: a load failure read as an empty index: %q", lvl, kind, msg)
			}
			if !strings.Contains(msg, errGraphLoad.Error()) {
				t.Errorf("level %s, kind %s: the answer does not name the load failure: %q", lvl, kind, msg)
			}
			atLevel := strings.Contains(msg, "tenancy level "+lvl.String())
			knob := strings.Contains(msg, "FATHOM_TENANCY_BUDGET") && strings.Contains(msg, "[tenancy]")
			if lvl == tenancy.LevelNormal && (atLevel || knob) {
				t.Errorf("level normal, kind %s: names a tenancy level that is not in force: %q", kind, msg)
			}
			if lvl != tenancy.LevelNormal && (!atLevel || !knob) {
				t.Errorf("level %s, kind %s: the answer must name the level and the budget knob: %q", lvl, kind, msg)
			}
		}
	}
}

// TestNavGraphReadersNameGraphLoadFailure is EW-78's audit of the other
// loadNavGraphs readers: find_callers and localize keep their answer from
// the passes that did run, and say the call graph was not loaded.
func TestNavGraphReadersNameGraphLoadFailure(t *testing.T) {
	fake := &graphLoadFailBackend{extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}, svcPath: t.TempDir()}}
	srv := mcp.NewServer(fake)
	callers := callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": "B"})
	if s, _ := callers["summary"].(string); !strings.Contains(s, errGraphLoad.Error()) {
		t.Errorf("find_callers: the summary does not name the graph load failure: %q", s)
	}
	loc := callTool(t, srv, "localize", map[string]any{"service": "svc", "error": "B fails"})
	if n, _ := loc["note"].(string); !strings.Contains(n, errGraphLoad.Error()) {
		t.Errorf("localize: the note does not name the graph load failure: %q (payload %v)", n, loc)
	}
}
