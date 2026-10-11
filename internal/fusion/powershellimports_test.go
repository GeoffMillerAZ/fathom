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
		"ps::src/Tool.<module> -> ps::src/Private/Helpers.<module>",      // . $PSScriptRoot\Private\Helpers.ps1
		"ps::src/Tool.Get-Tool -> ps::src/Private/Helpers.Format-Helper", // a call into the dot-sourced file
		"ps::tests/Tool.Tests.<module> -> ps::src/Tool.<module>",         // Import-Module "$PSScriptRoot/../src/Tool.psd1"
		"ps::tests/Tool.Tests.<module> -> ps::src/Nested/Extra.<module>", // using module ../src/Nested/Extra.psm1
	} {
		if !got[w] {
			t.Errorf("missing file-resolved edge %s; have %v", w, got)
		}
	}
	// The manifest names its root and nested modules: an edge from its module
	// symbol to each. Its package is its file's name less the extension, the
	// same as Tool.psm1's (the module layout's one name), so the two module
	// symbols share a key, as a C++ foo.h and foo.cpp do (O915).
	manifestEdges := 0
	for k := range got {
		if strings.Contains(k, "Ignored") {
			t.Errorf("a dynamic path resolved: %s", k)
		}
		if strings.HasSuffix(k, "-> ps::src/Nested/Extra.<module>") && strings.HasPrefix(k, "ps::src/Tool.") {
			manifestEdges++
		}
	}
	if manifestEdges == 0 {
		t.Errorf("the manifest's NestedModules drew no edge to src/Nested/Extra.psm1; have %v", got)
	}
}
