package migrationtargets_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/migrationtargets"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// ─── fixtures ─────────────────────────────────────────────────────────────────

func nodes() []core.SymbolNode {
	return []core.SymbolNode{
		// Uses deprecated io/ioutil.
		{Service: "svc", Package: "io/ioutil", Name: "ReadAll", Kind: core.KindGoFunc},
		// Uses deprecated io/ioutil.
		{Service: "svc", Package: "io/ioutil", Name: "WriteFile", Kind: core.KindGoFunc},
		// Uses deprecated golang.org/x/net/context.
		{Service: "svc", Package: "golang.org/x/net/context", Name: "Background", Kind: core.KindGoFunc},
		// Modern — should not be flagged.
		{Service: "svc", Package: "context", Name: "Background", Kind: core.KindGoFunc},
		// Modern — should not be flagged.
		{Service: "svc", Package: "io", Name: "ReadAll", Kind: core.KindGoFunc},
	}
}

// ─── Find ─────────────────────────────────────────────────────────────────────

func TestFind_DeprecatedIoutil(t *testing.T) {
	// deprecatedPkgs is additive (ADR: merge, not replace — the tool's own
	// parameter description promises "default list is always included").
	// "io/ioutil" is already in DefaultDeprecatedPkgs, so this caller-supplied
	// list is a no-op merge — but the OTHER default entries (notably
	// golang.org/x/net/context, which the fixture also uses) still apply.
	got := migrationtargets.Find(nodes(), []string{"io/ioutil"})
	// Two ioutil symbols + the golang.org/x/net/context default symbol.
	if len(got) != 3 {
		t.Fatalf("Find(io/ioutil) = %v (len %d), want 3 targets (2 ioutil + 1 default x/net/context)", got, len(got))
	}
	var sawIoutil, sawContext int
	for _, tgt := range got {
		switch {
		case contains(tgt.Symbol, "io/ioutil"):
			sawIoutil++
		case contains(tgt.Symbol, "golang.org/x/net/context"):
			sawContext++
		default:
			t.Errorf("unexpected target symbol %q", tgt.Symbol)
		}
	}
	if sawIoutil != 2 {
		t.Errorf("got %d io/ioutil targets, want 2", sawIoutil)
	}
	if sawContext != 1 {
		t.Errorf("got %d default-list x/net/context targets, want 1 (default list must always be included)", sawContext)
	}
}

func TestFind_DeprecatedContext(t *testing.T) {
	// Same additive contract: caller supplies only x/net/context, but the
	// default list's io/ioutil entries are still checked.
	got := migrationtargets.Find(nodes(), []string{"golang.org/x/net/context"})
	if len(got) != 3 {
		t.Fatalf("Find(x/net/context) = %v (len %d), want 3 targets (1 x/net/context + 2 default ioutil)", got, len(got))
	}
	var foundContext bool
	for _, tgt := range got {
		if tgt.Symbol == "svc::golang.org/x/net/context.Background" {
			foundContext = true
		}
	}
	if !foundContext {
		t.Errorf("expected svc::golang.org/x/net/context.Background among targets: %v", got)
	}
}

func TestFind_CallerPkgsMergeWithDefaults(t *testing.T) {
	// A caller-supplied package that is NOT in the default list must be
	// checked in ADDITION to the defaults — the defaults are never dropped
	// just because the caller passed a custom, non-empty list.
	custom := []core.SymbolNode{
		{Service: "svc", Package: "example.com/old/custompkg", Name: "Do", Kind: core.KindGoFunc},
	}
	allNodes := append(append([]core.SymbolNode{}, nodes()...), custom...)

	got := migrationtargets.Find(allNodes, []string{"example.com/old/custompkg"})

	var sawCustom, sawDefaultIoutil bool
	for _, tgt := range got {
		if contains(tgt.Symbol, "custompkg") {
			sawCustom = true
		}
		if contains(tgt.Symbol, "io/ioutil") {
			sawDefaultIoutil = true
		}
	}
	if !sawCustom {
		t.Errorf("caller-supplied package was not checked: %v", got)
	}
	if !sawDefaultIoutil {
		t.Errorf("default list was dropped when caller supplied a custom list (should always be included): %v", got)
	}
}

func TestFind_MergeDeduplicates(t *testing.T) {
	// Passing a duplicate of a default entry must not produce duplicate
	// targets for the same symbol.
	got := migrationtargets.Find(nodes(), []string{"io/ioutil", "io/ioutil"})
	var ioutilReadAllCount int
	for _, tgt := range got {
		if tgt.Symbol == "svc::io/ioutil.ReadAll" {
			ioutilReadAllCount++
		}
	}
	if ioutilReadAllCount != 1 {
		t.Errorf("got %d targets for svc::io/ioutil.ReadAll, want 1 (merge must dedupe)", ioutilReadAllCount)
	}
}

func TestFind_ModernPackagesNotFlagged(t *testing.T) {
	got := migrationtargets.Find(nodes(), []string{"io/ioutil", "golang.org/x/net/context"})
	// "context" and "io" should NOT be in results.
	for _, tgt := range got {
		if tgt.Symbol == "svc::context.Background" {
			t.Errorf("modern 'context' package was incorrectly flagged: %v", tgt)
		}
		if tgt.Symbol == "svc::io.ReadAll" {
			t.Errorf("modern 'io' package was incorrectly flagged: %v", tgt)
		}
	}
}

func TestFind_DefaultDeprecatedPkgs(t *testing.T) {
	// Passing nil should use DefaultDeprecatedPkgs (which includes io/ioutil).
	got := migrationtargets.Find(nodes(), nil)
	// At minimum, io/ioutil and x/net/context nodes should be flagged.
	if len(got) < 3 {
		t.Errorf("Find(nil) = %v, expected ≥3 targets using defaults", got)
	}
}

func TestFind_NoDeprecated(t *testing.T) {
	modern := []core.SymbolNode{
		{Service: "svc", Package: "context", Name: "Background", Kind: core.KindGoFunc},
		{Service: "svc", Package: "io", Name: "ReadAll", Kind: core.KindGoFunc},
	}
	got := migrationtargets.Find(modern, []string{"io/ioutil"})
	if len(got) != 0 {
		t.Errorf("Find(modern nodes) = %v, want empty", got)
	}
}

func TestFind_EmptyNodes(t *testing.T) {
	got := migrationtargets.Find(nil, []string{"io/ioutil"})
	if len(got) != 0 {
		t.Errorf("Find(nil nodes) = %v, want empty", got)
	}
}

func TestFind_Sorted(t *testing.T) {
	got := migrationtargets.Find(nodes(), []string{"io/ioutil", "golang.org/x/net/context"})
	for i := 1; i < len(got); i++ {
		a, b := got[i-1], got[i]
		if a.Symbol > b.Symbol || (a.Symbol == b.Symbol && a.Reason > b.Reason) {
			t.Errorf("results not sorted at index %d: %+v, %+v", i, a, b)
		}
	}
}

func TestFind_Determinism(t *testing.T) {
	a := migrationtargets.Find(nodes(), []string{"io/ioutil", "golang.org/x/net/context"})
	b := migrationtargets.Find(nodes(), []string{"io/ioutil", "golang.org/x/net/context"})
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Find not deterministic:\n  %v\n  %v", a, b)
	}
}

func TestFind_ReasonContainsPackage(t *testing.T) {
	// deprecatedPkgs is merged with the defaults, so passing "io/ioutil"
	// alone still produces a target for the default-list x/net/context
	// entry; each target's Reason must name the package that actually
	// matched, not necessarily the caller-supplied one.
	got := migrationtargets.Find(nodes(), []string{"io/ioutil"})
	for _, tgt := range got {
		if !contains(tgt.Reason, "io/ioutil") && !contains(tgt.Reason, "golang.org/x/net/context") {
			t.Errorf("Reason %q does not mention a deprecated package", tgt.Reason)
		}
	}
}

func TestFind_MultipleDeprecatedPkgs(t *testing.T) {
	got := migrationtargets.Find(nodes(), []string{"io/ioutil", "golang.org/x/net/context"})
	// 2 ioutil + 1 x/net/context = 3.
	if len(got) != 3 {
		t.Errorf("Find(two deprecated pkgs) = %v (len %d), want 3", got, len(got))
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func contains(s, sub string) bool {
	return len(sub) <= len(s) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
