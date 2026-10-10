package mcp

import (
	"strings"
	"testing"
)

// module_symbol_delete_test.go — wave h3 (O742, SR-75): a module symbol is
// never dead while its file exists, and a symbol whose only callers are
// file-resolved typed edges has those callers as referents (at their
// heuristic tier), not "proven zero". moduleFixture: web/app.js and
// lib/loader.py import lib/signals.rb's module symbol; lib/signals.rb's
// module-level code calls lib/emitter.rb's emit through a file-resolved edge.

func deleteCheck(t *testing.T, srv *Server, symbol string) map[string]any {
	t.Helper()
	return callTool(t, srv, "blast_radius", map[string]any{"kind": "delete", "service": "svc", "symbol": symbol})
}

func referentFiles(got map[string]any) []string {
	var files []string
	rows, _ := got["referents"].([]any)
	for _, r := range rows {
		m := r.(map[string]any)
		files = append(files, m["file"].(string))
	}
	return files
}

func TestBlastRadiusDeleteModuleSymbolListsImportersNeverProvenZero(t *testing.T) {
	srv := moduleFixture(t)
	got := deleteCheck(t, srv, "svc::lib/signals.<module>")
	files := referentFiles(got)
	if len(files) != 2 || files[0] != "lib/loader.py" || files[1] != "web/app.js" {
		t.Fatalf("referents = %v, want the two importers lib/loader.py and web/app.js", got["referents"])
	}
	for _, r := range got["referents"].([]any) {
		if tier := r.(map[string]any)["tier"]; tier != "heuristic" {
			t.Errorf("referent tier = %v, want heuristic (a file-resolved edge)", tier)
		}
	}
	if got["module_symbol"] != true {
		t.Errorf("module_symbol = %v, want true", got["module_symbol"])
	}
	sum, _ := got["summary"].(string)
	if strings.Contains(sum, "proven zero") {
		t.Errorf("summary = %q, must not claim proven zero", sum)
	}
	for _, want := range []string{"lives as long as its file", `kind:"change"`, "lib/signals.rb"} {
		if !strings.Contains(sum, want) {
			t.Errorf("summary = %q, want it to say %q", sum, want)
		}
	}
}

func TestBlastRadiusDeleteModuleSymbolWithNoImportersIsNotProvenZero(t *testing.T) {
	srv := moduleFixture(t)
	got := deleteCheck(t, srv, "svc::web/app.<module>")
	if n, _ := got["referent_count"].(float64); n != 0 {
		t.Fatalf("referent_count = %v, want 0 importers", got["referent_count"])
	}
	sum, _ := got["summary"].(string)
	if strings.Contains(sum, "proven zero") || !strings.Contains(sum, "lives as long as its file") {
		t.Errorf("summary = %q, want a module-symbol answer, never proven zero", sum)
	}
}

func TestBlastRadiusDeleteCountsFileResolvedCallers(t *testing.T) {
	srv := moduleFixture(t)
	got := deleteCheck(t, srv, "svc::lib/emitter.emit")
	rows, _ := got["referents"].([]any)
	if len(rows) != 1 {
		t.Fatalf("referents = %v, want the file-resolved caller lib/signals.rb's module symbol", got["referents"])
	}
	row := rows[0].(map[string]any)
	if row["file"] != "lib/signals.rb" || row["tier"] != "heuristic" {
		t.Errorf("referent = %v, want lib/signals.rb at tier heuristic", row)
	}
	if got["module_symbol"] != nil {
		t.Errorf("module_symbol = %v on an ordinary function, want absent", got["module_symbol"])
	}
	if sum, _ := got["summary"].(string); strings.Contains(sum, "proven zero") {
		t.Errorf("summary = %q", sum)
	}
}

func TestFindCallersSummaryReadsTypedCallersWhenCallersEmpty(t *testing.T) {
	srv := moduleFixture(t)
	got := callTool(t, srv, "find_callers", map[string]any{"service": "svc", "symbol": "svc::lib/emitter.emit"})
	if _, ok := got["typed_callers"]; !ok {
		t.Fatalf("fixture drift: no typed_callers in %v", got)
	}
	sum, _ := got["summary"].(string)
	if strings.Contains(sum, "no direct callers") || !strings.Contains(sum, "typed") {
		t.Errorf("summary = %q, want it to read the typed_callers beside an empty callers", sum)
	}
}
