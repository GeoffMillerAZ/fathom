package mcp

import (
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// module_symbol_qualifier_test.go — wave h3 (O743, SR-74): a qualified exact
// query for a module symbol matches only the module symbol of the file the
// qualifier names (its package, or its file stem path), never every file's.
// A bare `<module>` still answers every file's (h2's contract). The fixture
// is moduleFixture's JavaScriptModule, RubyModule and PythonModule files, so
// the rule is pinned on the name and any dialect turning module_symbol on
// inherits it.

func moduleFiles(t *testing.T, payload map[string]any) []string {
	t.Helper()
	raw, _ := payload["symbols"].([]any)
	var files []string
	for _, r := range raw {
		if m, ok := r.(map[string]any); ok {
			if m["name"] != core.ModuleSymbolName {
				continue
			}
			f, _ := m["file_path"].(string)
			files = append(files, f)
		}
	}
	slices.Sort(files)
	return files
}

func TestFindSymbolQualifiedModuleNamesOneFile(t *testing.T) {
	srv := moduleFixture(t)
	for q, want := range map[string]string{
		"signals." + core.ModuleSymbolName:     "lib/signals.rb",
		"app." + core.ModuleSymbolName:         "web/app.js",
		"loader." + core.ModuleSymbolName:      "lib/loader.py",
		"lib/signals." + core.ModuleSymbolName: "lib/signals.rb",
		"web/app." + core.ModuleSymbolName:     "web/app.js",
	} {
		got := findSymbolIn(t, srv, q)
		if files := moduleFiles(t, got); !slices.Equal(files, []string{want}) {
			t.Errorf("find_symbol(%q) answered module symbols of %v (matched_via %v), want only %s", q, files, got["matched_via"], want)
		}
	}
}

func TestFindSymbolQualifiedModuleWrongQualifierIsNoExactMatch(t *testing.T) {
	srv := moduleFixture(t)
	for _, q := range []string{"zzz." + core.ModuleSymbolName, "ignals." + core.ModuleSymbolName, "lib/zzz." + core.ModuleSymbolName} {
		got := findSymbolIn(t, srv, q)
		if files := moduleFiles(t, got); len(files) != 0 {
			t.Errorf("find_symbol(%q) answered module symbols of %v, want none (a miss falls to the lower tiers)", q, files)
		}
		if got["matched_via"] == "exact" {
			t.Errorf("find_symbol(%q) matched_via exact, want a lower tier or a miss", q)
		}
	}
}

func TestFindSymbolBareModuleStillAnswersEveryFile(t *testing.T) {
	srv := moduleFixture(t)
	got := findSymbolIn(t, srv, core.ModuleSymbolName)
	want := []string{"lib/emitter.rb", "lib/loader.py", "lib/signals.rb", "web/app.js"}
	if files := moduleFiles(t, got); !slices.Equal(files, want) {
		t.Errorf("find_symbol(<module>) = %v, want %v", files, want)
	}
}

func TestFindSymbolQualifiedOrdinaryNameUnchanged(t *testing.T) {
	// Recorded open (O770): an ordinary bare-stored name still matches by
	// suffix alone, so verify's plan-row resolution is untouched.
	srv := moduleFixture(t)
	got := findSymbolIn(t, srv, "zzz.emit")
	raw, _ := got["symbols"].([]any)
	if len(raw) != 1 {
		t.Errorf("find_symbol(zzz.emit) = %v, want the ordinary-name qualifier rule unchanged (1 row)", got["symbols"])
	}
}
