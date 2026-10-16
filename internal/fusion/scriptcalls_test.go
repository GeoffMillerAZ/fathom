package fusion

// scriptcalls_test.go — owner ruling S1's lever (wave lf7, item shell,
// LB-F520): a bash script's top-level call (a call its <module> symbol makes)
// is stored only when its callee is a definition of the repository — in the
// script, in a file it sources, or anywhere else the service defines it;
// echo, grep and every other external command carry no navigation value and
// are not stored. A call inside a function, and every other dialect's module
// calls, are unchanged.

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_BashModuleCallsBind(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"bin/tool":       "#!/usr/bin/env bash\n. ../lib/helpers.sh\nlocal_fn() {\n  echo in\n  helper_fn\n}\nhelper_fn\nlocal_fn\nlater_fn\necho done\ngrep -q x y\n",
		"lib/helpers.sh": "helper_fn() {\n  echo hi\n}\nhelper_two() {\n  :\n}\n",
		"z/later.sh":     "later_fn() {\n  :\n}\nprintf '%s' x\n",
		"app/main.py":    "print('x')\n",
	})
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	ingest := func() map[string]core.Observation {
		t.Helper()
		if _, err := p.Ingest(ctx, core.ServiceRef{ID: "sh", Path: dir}); err != nil {
			t.Fatal(err)
		}
		obs, err := st.QueryObservations(ctx, "sh", "")
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]core.Observation{}
		for _, o := range obs {
			out[o.SourceRef+"#"+o.Symbol] = o
		}
		return out
	}
	calls := func(o core.Observation) []string {
		c := slices.Clone(o.Calls)
		slices.Sort(c)
		return c
	}

	got := ingest()
	for key, want := range map[string][]string{
		"bin/tool#<module>":    {"helper_fn", "later_fn", "local_fn"}, // sourced, defined elsewhere in the repository, in the script; echo, grep and `.` gone
		"bin/tool#local_fn":    {"echo", "helper_fn"},                 // a function's calls stay as extracted
		"z/later.sh#<module>":  nil,                                   // printf: an external command
		"app/main.py#<module>": {"print"},                             // python's module calls are not in the lever's scope
	} {
		o, ok := got[key]
		if !ok {
			t.Fatalf("no observation %s; have %d", key, len(got))
		}
		if c := calls(o); !slices.Equal(c, want) {
			t.Errorf("%s calls = %q, want %q", key, c, want)
		}
	}
	// The stored call index holds the same rows: no `echo` row from a module symbol.
	callers, indexed, err := st.ObservationsCalling(ctx, "sh", "echo")
	if err != nil || !indexed {
		t.Fatalf("ObservationsCalling: indexed %v, err %v", indexed, err)
	}
	for _, o := range callers {
		if o.Symbol == core.ModuleSymbolName {
			t.Errorf("a module symbol's echo row was stored: %s", o.SourceRef)
		}
	}
	// A held script is fingerprinted after its rows are written.
	if _, found, err := st.Fingerprint(ctx, "sh", "bin/tool"); err != nil || !found {
		t.Errorf("bin/tool fingerprint found %v, err %v", found, err)
	}
	// The top-level call into the sourced file still binds (file-resolved edge).
	edges, err := st.TypedEdges(ctx, "sh")
	if err != nil {
		t.Fatal(err)
	}
	var bound bool
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved && e.FromSymbol == "sh::bin/tool.<module>" && e.ToSymbol == "sh::lib/helpers.helper_fn" {
			bound = true
		}
	}
	if !bound {
		t.Error("missing file-resolved edge sh::bin/tool.<module> -> sh::lib/helpers.helper_fn")
	}

	// An incremental ingest reads the definitions of the files it does not
	// re-read from the store: lib/helpers.sh is unchanged, its helper_two
	// still keeps the edited script's new top-level call.
	if err := os.WriteFile(filepath.Join(dir, "bin/tool"), []byte("#!/usr/bin/env bash\n. ../lib/helpers.sh\nhelper_two\nls -la\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = ingest()
	if c := calls(got["bin/tool#<module>"]); !slices.Equal(c, []string{"helper_two"}) {
		t.Errorf("after the edit, bin/tool <module> calls = %q, want [helper_two]", c)
	}
}

// TestIngest_PowerShellModuleCallsBind (wave lf8, item powershell, owner
// ruling S5): the lever's scope widened to PowerShell. A script's or module's
// top-level calls (its <module> symbol's, which hold a Pester file's Describe
// and It blocks) are stored only when the callee is a definition of the
// repository; Write-Host, Export-ModuleMember and every other cmdlet the
// repository does not define store no row. A function's calls stay as
// extracted, and the top-level call into a dot-sourced file still binds.
func TestIngest_PowerShellModuleCallsBind(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"src/Tool.psm1":           ". $PSScriptRoot\\Private\\Helpers.ps1\nfunction Get-Tool {\n    Write-Output 'x'\n    Format-Helper\n}\nFormat-Helper\nGet-Tool\nGet-Later\nWrite-Host done\nExport-ModuleMember -Function Get-Tool\n",
		"src/Private/Helpers.ps1": "function Format-Helper {\n    'x'\n}\n",
		"z/Later.ps1":             "function Get-Later {\n}\nWrite-Verbose x\n",
		"tests/Tool.Tests.ps1":    "Import-Module $PSScriptRoot\\..\\src\\Tool.psm1 -Force\nDescribe 'Get-Tool' {\n    It 'works' {\n        Get-Tool | Should -Be 'x'\n    }\n}\n",
	})
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
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
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[o.SourceRef+"#"+o.Symbol] = o
	}
	for key, want := range map[string][]string{
		"src/Tool.psm1#<module>":        {"Format-Helper", "Get-Later", "Get-Tool"}, // dot-sourced, defined elsewhere in the repository, in the module; Write-Host and Export-ModuleMember gone
		"src/Tool.psm1#Get-Tool":        {"Format-Helper", "Write-Output"},          // a function's calls stay as extracted
		"z/Later.ps1#<module>":          nil,                                        // Write-Verbose: a cmdlet the repository does not define
		"tests/Tool.Tests.ps1#<module>": {"Get-Tool"},                               // Import-Module, Describe, It and Should are Pester's or PowerShell's, not the repository's
	} {
		o, ok := got[key]
		if !ok {
			t.Fatalf("no observation %s; have %d", key, len(got))
		}
		c := slices.Clone(o.Calls)
		slices.Sort(c)
		if !slices.Equal(c, want) {
			t.Errorf("%s calls = %q, want %q", key, c, want)
		}
	}
	callers, indexed, err := st.ObservationsCalling(ctx, "ps", "Write-Host")
	if err != nil || !indexed {
		t.Fatalf("ObservationsCalling: indexed %v, err %v", indexed, err)
	}
	if len(callers) != 0 {
		t.Errorf("a module symbol's Write-Host row was stored: %d", len(callers))
	}
	edges, err := st.TypedEdges(ctx, "ps")
	if err != nil {
		t.Fatal(err)
	}
	bound := map[string]bool{}
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved {
			bound[e.FromSymbol+" -> "+e.ToSymbol] = true
		}
	}
	for _, w := range []string{
		"ps::src/Tool.psm1.<module> -> ps::src/Private/Helpers.ps1.<module>",      // . $PSScriptRoot\Private\Helpers.ps1
		"ps::src/Tool.psm1.<module> -> ps::src/Private/Helpers.ps1.Format-Helper", // the top-level call into it binds
		"ps::tests/Tool.Tests.ps1.<module> -> ps::src/Tool.psm1.<module>",         // Import-Module $PSScriptRoot\..\src\Tool.psm1
	} {
		if !bound[w] {
			t.Errorf("missing file-resolved edge %s; have %v", w, bound)
		}
	}
}
