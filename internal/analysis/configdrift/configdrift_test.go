package configdrift_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/analysis/configdrift"
)

func TestDrift_Basic(t *testing.T) {
	configs := map[string]map[string]string{
		"svc-a": {"log_level": "info", "timeout": "30s"},
		"svc-b": {"log_level": "info", "timeout": "30s"},
		"svc-c": {"log_level": "debug", "timeout": "30s"}, // drifts on log_level
	}

	items := configdrift.Drift(configs, []string{"log_level", "timeout"})

	if len(items) != 1 {
		t.Fatalf("expected 1 drift item, got %d: %v", len(items), items)
	}
	d := items[0]
	if d.Key != "log_level" {
		t.Errorf("Key: got %q, want %q", d.Key, "log_level")
	}
	if d.Service != "svc-c" {
		t.Errorf("Service: got %q, want %q", d.Service, "svc-c")
	}
	if d.Value != "debug" {
		t.Errorf("Value: got %q, want %q", d.Value, "debug")
	}
	if d.Majority != "info" {
		t.Errorf("Majority: got %q, want %q", d.Majority, "info")
	}
}

func TestDrift_NoDrift(t *testing.T) {
	configs := map[string]map[string]string{
		"svc-a": {"k": "v"},
		"svc-b": {"k": "v"},
		"svc-c": {"k": "v"},
	}
	items := configdrift.Drift(configs, []string{"k"})
	if len(items) != 0 {
		t.Fatalf("expected no drift, got %v", items)
	}
}

func TestDrift_MissingKey(t *testing.T) {
	// svc-c doesn't define the key — should be excluded from counting.
	configs := map[string]map[string]string{
		"svc-a": {"k": "v1"},
		"svc-b": {"k": "v1"},
		"svc-c": {},
	}
	items := configdrift.Drift(configs, []string{"k"})
	if len(items) != 0 {
		t.Fatalf("expected no drift, got %v", items)
	}
}

func TestDrift_TieBrokenLexicographically(t *testing.T) {
	// Two services with "aaa", two with "bbb" — tie, "aaa" wins (smaller).
	configs := map[string]map[string]string{
		"svc-1": {"k": "aaa"},
		"svc-2": {"k": "aaa"},
		"svc-3": {"k": "bbb"},
		"svc-4": {"k": "bbb"},
	}
	items := configdrift.Drift(configs, []string{"k"})
	// "aaa" is majority by lex tiebreak; svc-3 and svc-4 drift.
	if len(items) != 2 {
		t.Fatalf("expected 2 drift items, got %d: %v", len(items), items)
	}
	for _, d := range items {
		if d.Majority != "aaa" {
			t.Errorf("majority should be %q, got %q", "aaa", d.Majority)
		}
		if d.Value != "bbb" {
			t.Errorf("drifting value should be %q, got %q", "bbb", d.Value)
		}
	}
}

func TestDrift_Deterministic(t *testing.T) {
	configs := map[string]map[string]string{
		"alpha": {"x": "1", "y": "a"},
		"beta":  {"x": "2", "y": "a"},
		"gamma": {"x": "1", "y": "b"},
		"delta": {"x": "1", "y": "a"},
	}
	keys := []string{"x", "y"}
	a := configdrift.Drift(configs, keys)
	b := configdrift.Drift(configs, keys)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic length %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic at [%d]: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestDrift_SingleService(t *testing.T) {
	// Only one service defines the key → can't determine drift.
	configs := map[string]map[string]string{
		"svc-a": {"k": "v"},
	}
	items := configdrift.Drift(configs, []string{"k"})
	if len(items) != 0 {
		t.Fatalf("expected no drift for single service, got %v", items)
	}
}
