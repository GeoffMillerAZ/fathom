package migrationtargets_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/analysis/migrationtargets"
	"github.com/geoffmilleraz/sextant/internal/core"
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
	got := migrationtargets.Find(nodes(), []string{"io/ioutil"})
	// Two ioutil symbols should be flagged.
	if len(got) != 2 {
		t.Fatalf("Find(io/ioutil) = %v (len %d), want 2 targets", got, len(got))
	}
	for _, tgt := range got {
		if !contains(tgt.Symbol, "io/ioutil") {
			t.Errorf("unexpected target symbol %q for io/ioutil", tgt.Symbol)
		}
	}
}

func TestFind_DeprecatedContext(t *testing.T) {
	got := migrationtargets.Find(nodes(), []string{"golang.org/x/net/context"})
	if len(got) != 1 {
		t.Fatalf("Find(x/net/context) = %v, want 1 target", got)
	}
	if got[0].Symbol != "svc::golang.org/x/net/context.Background" {
		t.Errorf("wrong symbol: %q", got[0].Symbol)
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
	got := migrationtargets.Find(nodes(), []string{"io/ioutil"})
	for _, tgt := range got {
		if !contains(tgt.Reason, "io/ioutil") {
			t.Errorf("Reason %q does not mention deprecated package", tgt.Reason)
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
