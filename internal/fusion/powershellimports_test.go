package fusion

// powershellimports_test.go — LB-Q-powershell L3 (wave lf6, item shell):
// dot-sourcing, Import-Module, `using module` and a manifest's RootModule and
// NestedModules resolve to files by the manifest's file-symbol strategy: a
// $PSScriptRoot path against the script's own directory (dir_vars), `\` a
// separator, any other relative path against the script's directory and then
// the repository root. The script's top level is its module symbol (K18), so
// each resolved statement is a module edge and a call into the target binds
// its function. A Join-Path argument or another variable resolves nothing.

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_PowerShellImportsResolve(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"src/Tool.psd1":           "@{\n    RootModule    = 'Tool.psm1'\n    NestedModules = @('Nested\\Extra.psm1')\n    ModuleVersion = '1.0.0'\n}\n",
		"src/Tool.psm1":           ". $PSScriptRoot\\Private\\Helpers.ps1\nfunction Get-Tool {\n    Format-Helper\n}\n",
		"src/Private/Helpers.ps1": "function Format-Helper {\n    'x'\n}\n",
		"src/Nested/Extra.psm1":   "function Get-Extra {\n    1\n}\n",
		"tests/Tool.Tests.ps1":    "Import-Module \"$PSScriptRoot/../src/Tool.psd1\" -Force\nusing module ../src/Nested/Extra.psm1\nGet-Tool\nImport-Module (Join-Path $PSScriptRoot 'Ignored.psm1')\n. $here\\Ignored.ps1\n",
		"tests/Ignored.psm1":      "function Get-Ignored {\n}\n",
		"tests/Ignored.ps1":       "function Get-IgnoredScript {\n}\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "ps", Path: dir}); err != nil {
		t.Fatal(err)
	}
	all, err := st.TypedEdges(ctx, "ps")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			got[e.FromSymbol+" -> "+e.ToSymbol] = true
		}
	}
	for _, w := range []string{
		"ps::src/Tool.psm1.<module> -> ps::src/Private/Helpers.ps1.<module>",      // . $PSScriptRoot\Private\Helpers.ps1
		"ps::src/Tool.psm1.Get-Tool -> ps::src/Private/Helpers.ps1.Format-Helper", // a call into the dot-sourced file
		"ps::tests/Tool.Tests.ps1.<module> -> ps::src/Tool.psd1.<module>",         // Import-Module "$PSScriptRoot/../src/Tool.psd1"
		"ps::tests/Tool.Tests.ps1.<module> -> ps::src/Nested/Extra.psm1.<module>", // using module ../src/Nested/Extra.psm1
		"ps::src/Tool.psd1.<module> -> ps::src/Tool.psm1.<module>",                // the manifest's RootModule
		"ps::src/Tool.psd1.<module> -> ps::src/Nested/Extra.psm1.<module>",        // the manifest's NestedModules
	} {
		if !got[w] {
			t.Errorf("missing file-resolved edge %s; have %v", w, got)
		}
	}
	// The manifest names its root and nested modules: an edge from its module
	// symbol to each. Its package is its file's base name with the extension
	// (package strategy "file", LB-F519), so Tool.psd1 and Tool.psm1 each key
	// a module symbol of their own (O915, TestIngest_PowerShellModuleKeysPerFile).
	for k := range got {
		if strings.Contains(k, "Ignored") {
			t.Errorf("a dynamic path resolved: %s", k)
		}
	}
}

// TestIngest_PowerShellModuleKeysPerFile (O915, LB-F522, owner ruling S2): a
// module's manifest, its script module and a script sharing their name in one
// directory (X.psd1, X.psm1, X.ps1) mint three module symbols with three keys
// (package strategy "file", LB-F519: the package is the file's base name with
// its extension), so the manifest's RootModule and NestedModules edges and an
// `Import-Module $PSScriptRoot\..\mod\X.psd1` land on the file they name, never
// on a sibling sharing its stem.
func TestIngest_PowerShellModuleKeysPerFile(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"mod/X.psd1":        "@{\n    RootModule    = 'X.psm1'\n    NestedModules = @('X.ps1')\n    ModuleVersion = '1.0.0'\n}\n",
		"mod/X.psm1":        "function Get-X {\n    1\n}\n",
		"mod/X.ps1":         "function Get-XScript {\n    2\n}\n",
		"tests/X.Tests.ps1": "Import-Module $PSScriptRoot\\..\\mod\\X.psd1 -Force\nGet-X\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "ps", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "ps", "")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]string{} // module key -> the files declaring it
	for _, o := range obs {
		if o.Symbol == core.ModuleSymbolName {
			k := o.ToSymbolNode().Key()
			files[k] = append(files[k], o.SourceRef)
		}
	}
	for key, file := range map[string]string{
		"ps::mod/X.psd1.<module>":        "mod/X.psd1",
		"ps::mod/X.psm1.<module>":        "mod/X.psm1",
		"ps::mod/X.ps1.<module>":         "mod/X.ps1",
		"ps::tests/X.Tests.ps1.<module>": "tests/X.Tests.ps1",
	} {
		if got := files[key]; len(got) != 1 || got[0] != file {
			t.Errorf("module key %s declared by %v, want [%s] (keys: %v)", key, got, file, files)
		}
	}
	all, err := st.TypedEdges(ctx, "ps")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			got[e.FromSymbol+" -> "+e.ToSymbol] = true
		}
	}
	for _, w := range []string{
		"ps::mod/X.psd1.<module> -> ps::mod/X.psm1.<module>",        // RootModule = 'X.psm1'
		"ps::mod/X.psd1.<module> -> ps::mod/X.ps1.<module>",         // NestedModules = @('X.ps1')
		"ps::tests/X.Tests.ps1.<module> -> ps::mod/X.psd1.<module>", // Import-Module $PSScriptRoot\..\mod\X.psd1
	} {
		if !got[w] {
			t.Errorf("missing file-resolved edge %s; have %v", w, got)
		}
	}
	for k := range got {
		if strings.HasPrefix(k, "ps::tests/X.Tests.ps1.<module> -> ps::mod/X.ps") && !strings.HasSuffix(k, "X.psd1.<module>") {
			t.Errorf("the import of the manifest landed on a sibling: %s", k)
		}
	}
}
