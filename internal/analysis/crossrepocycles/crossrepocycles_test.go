package crossrepocycles_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/analysis/crossrepocycles"
)

func TestDetect_NoCycle(t *testing.T) {
	imports := map[string][]string{
		"frontend": {"backend"},
		"backend":  {"database"},
		"database": {},
	}
	got := crossrepocycles.Detect(imports)
	if len(got) != 0 {
		t.Errorf("expected no cycles, got %v", got)
	}
}

func TestDetect_SimpleCycle(t *testing.T) {
	// a → b → a
	imports := map[string][]string{
		"a": {"b"},
		"b": {"a"},
	}
	got := crossrepocycles.Detect(imports)
	if len(got) != 1 {
		t.Fatalf("expected 1 cycle, got %d: %v", len(got), got)
	}
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("cycle = %v; want %v", got[0], want)
	}
}

func TestDetect_ThreeNodeCycle(t *testing.T) {
	// a → b → c → a
	imports := map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"a"},
	}
	got := crossrepocycles.Detect(imports)
	if len(got) != 1 {
		t.Fatalf("expected 1 cycle, got %d: %v", len(got), got)
	}
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("cycle = %v; want %v", got[0], want)
	}
}

func TestDetect_MultipleCycles(t *testing.T) {
	// Two independent cycles: {a,b} and {x,y}
	imports := map[string][]string{
		"a": {"b"},
		"b": {"a"},
		"x": {"y"},
		"y": {"x"},
	}
	got := crossrepocycles.Detect(imports)
	if len(got) != 2 {
		t.Fatalf("expected 2 cycles, got %d: %v", len(got), got)
	}
	if !reflect.DeepEqual(got[0], []string{"a", "b"}) {
		t.Errorf("cycle[0] = %v; want [a b]", got[0])
	}
	if !reflect.DeepEqual(got[1], []string{"x", "y"}) {
		t.Errorf("cycle[1] = %v; want [x y]", got[1])
	}
}

func TestDetect_IsolatedServiceNoCycle(t *testing.T) {
	// "standalone" has no imports and nothing imports it
	imports := map[string][]string{
		"standalone": {},
	}
	got := crossrepocycles.Detect(imports)
	if len(got) != 0 {
		t.Errorf("isolated service should produce no cycle, got %v", got)
	}
}

func TestDetect_EmptyInput(t *testing.T) {
	got := crossrepocycles.Detect(map[string][]string{})
	if got == nil || len(got) != 0 {
		t.Errorf("empty input should return empty non-nil slice, got %v", got)
	}
}

func TestDetect_NilInput(t *testing.T) {
	got := crossrepocycles.Detect(nil)
	if got == nil || len(got) != 0 {
		t.Errorf("nil input should return empty non-nil slice, got %v", got)
	}
}

func TestDetect_DiamondNoCycle(t *testing.T) {
	// Diamond: top → left, top → right; left → bottom; right → bottom
	imports := map[string][]string{
		"top":    {"left", "right"},
		"left":   {"bottom"},
		"right":  {"bottom"},
		"bottom": {},
	}
	got := crossrepocycles.Detect(imports)
	if len(got) != 0 {
		t.Errorf("diamond has no cycle, got %v", got)
	}
}

func TestDetect_LargeCycle(t *testing.T) {
	// Chain: svc0→svc1→svc2→...→svc9→svc0
	const n = 10
	imports := make(map[string][]string, n)
	names := make([]string, n)
	for i := 0; i < n; i++ {
		names[i] = string(rune('a' + i)) // "a" through "j"
	}
	for i := 0; i < n; i++ {
		imports[names[i]] = []string{names[(i+1)%n]}
	}
	got := crossrepocycles.Detect(imports)
	if len(got) != 1 {
		t.Fatalf("expected 1 cycle of length %d, got %d: %v", n, len(got), got)
	}
	if len(got[0]) != n {
		t.Errorf("cycle length = %d; want %d", len(got[0]), n)
	}
}

func TestDetect_DuplicateImports(t *testing.T) {
	// Duplicate entries in imports list must not affect result.
	imports := map[string][]string{
		"p": {"q", "q", "q"},
		"q": {"p", "p"},
	}
	got := crossrepocycles.Detect(imports)
	if len(got) != 1 {
		t.Fatalf("expected 1 cycle, got %d: %v", len(got), got)
	}
	want := []string{"p", "q"}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("cycle = %v; want %v", got[0], want)
	}
}

func TestDetect_Determinism(t *testing.T) {
	imports := map[string][]string{
		"alpha":   {"beta", "gamma"},
		"beta":    {"delta"},
		"gamma":   {"delta"},
		"delta":   {"alpha"}, // creates cycle alpha→beta→delta→alpha and alpha→gamma→delta→alpha
		"epsilon": {},
	}
	r1 := crossrepocycles.Detect(imports)
	r2 := crossrepocycles.Detect(imports)
	if !reflect.DeepEqual(r1, r2) {
		t.Errorf("non-deterministic: %v vs %v", r1, r2)
	}
}

func TestDetect_UnreferencedImportee(t *testing.T) {
	// "ext-lib" is only mentioned as an import target, not a key in the map.
	imports := map[string][]string{
		"myservice": {"ext-lib"},
	}
	got := crossrepocycles.Detect(imports)
	if len(got) != 0 {
		t.Errorf("no cycle expected, got %v", got)
	}
}
