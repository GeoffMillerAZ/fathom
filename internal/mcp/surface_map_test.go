package mcp

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// mapShaped reports whether a tool name belongs to the exploration-map family
// (docs/design/exploration-maps.md section 4, ADR-056): `map`, `map_show`, a
// thread verb that files entries, anything that names the model.
func mapShaped(name string) bool {
	n := strings.ToLower(name)
	return n == "map" || strings.HasPrefix(n, "map_") || strings.HasPrefix(n, "maps") ||
		strings.Contains(n, "exploration") || strings.HasPrefix(n, "thread") || strings.Contains(n, "explmap")
}

// TestDefaultSurfaceUnchangedByMap pins XM-10 (docs/plans/exploration-maps.md,
// ADR-056, vision 40): exploration maps live on the CLI and, when an MCP form is
// built, on the extended surface only. The default surface (POST /rpc/agent,
// the registration every session pays the schema cost of) stays at exactly the
// eight tools it had before maps, whatever the registry holds, and no map-shaped
// tool is ever a member of it.
func TestDefaultSurfaceUnchangedByMap(t *testing.T) {
	before := []string{
		"blast_radius", "find_symbol", "obligations", "read_source",
		"review_diff", "search", "symbol_context", "verify_plan",
	}
	if got := DefaultSurfaceTools(); !reflect.DeepEqual(got, before) {
		t.Fatalf("the default surface is %v, want the eight it had before maps: %v", got, before)
	}
	for _, name := range before {
		if !isDefaultSurfaceTool(name) {
			t.Errorf("isDefaultSurfaceTool(%q) = false", name)
		}
	}
	for _, name := range []string{"map", "map_show", "maps", "exploration_map", "thread", "thread_note", "explmap"} {
		if isDefaultSurfaceTool(name) {
			t.Errorf("isDefaultSurfaceTool(%q) = true: maps never join the default surface", name)
		}
	}

	// What the server serves on the agent path is that list and nothing else,
	// over the whole registry (bare /rpc), whatever a map-shaped tool would add.
	srv := NewServer(nil)
	var served []string
	for _, d := range srv.visibleToolsFor(SurfaceAgent) {
		served = append(served, d.Name)
	}
	sort.Strings(served)
	if !reflect.DeepEqual(served, before) {
		t.Errorf("/rpc/agent serves %v, want %v", served, before)
	}

	// A map-shaped tool that is registered (none is today: maps are CLI only) is
	// off the default surface and on the extended one, and so on its companion
	// alias: the placement the row names, checked over every registered tool.
	srv.mu.Lock()
	tools := append([]toolEntry(nil), srv.tools...)
	srv.mu.Unlock()
	for _, te := range tools {
		if !mapShaped(te.desc.Name) {
			continue
		}
		if srv.toolOnSurface(SurfaceAgent, te.desc) {
			t.Errorf("tool %q is on the default surface; a map tool belongs on the extended surface and the CLI only", te.desc.Name)
		}
		for _, s := range []string{SurfaceExtended, SurfaceCompanion} {
			if !srv.toolOnSurface(s, te.desc) {
				t.Errorf("tool %q is registered but not on the %s surface", te.desc.Name, s)
			}
		}
	}
}
