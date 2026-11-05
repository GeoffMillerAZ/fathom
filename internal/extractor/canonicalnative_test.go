package extractor

import (
	"encoding/json"
	"os"
	"testing"
)

// The native table holds values of the closed list and agrees with the Go
// incumbent manifest the benchmark grades against.
func TestNativeCanonicalMatchesGoIncumbent(t *testing.T) {
	for kind, c := range nativeCanonical {
		if !isCanonicalKind(c) {
			t.Errorf("%s: canonical %q is not in the closed list", kind, c)
		}
		if _, dup := CanonicalKind(kind); dup {
			t.Errorf("%s is declared by a manifest, so the native table must not repeat it", kind)
		}
	}
	for kind, why := range noCanonical {
		if why == "" {
			t.Errorf("%s is on the no-canonical list without a reason", kind)
		}
		if _, ok := nativeCanonical[kind]; ok {
			t.Errorf("%s is both native-canonical and exempt", kind)
		}
	}
	raw, err := os.ReadFile("../langbench/incumbents.json")
	if err != nil {
		t.Fatal(err)
	}
	var incumbents []struct {
		Manifest struct {
			ID    string `json:"id"`
			Kinds map[string]struct {
				SourceKind string `json:"source_kind"`
				Canonical  string `json:"canonical"`
			} `json:"kinds"`
		} `json:"manifest"`
	}
	if err := json.Unmarshal(raw, &incumbents); err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, inc := range incumbents {
		if inc.Manifest.ID != "go" {
			continue
		}
		for _, k := range inc.Manifest.Kinds {
			seen++
			if got := nativeCanonical[k.SourceKind]; got != k.Canonical {
				t.Errorf("%s: the native table says %q, the Go incumbent manifest %q", k.SourceKind, got, k.Canonical)
			}
		}
	}
	if seen != 7 {
		t.Fatalf("the Go incumbent manifest lists %d kinds, want 7", seen)
	}
}

func TestDoorCanonicalKind(t *testing.T) {
	if c, ok := DoorCanonicalKind("GoFunc"); !ok || c != "function" {
		t.Errorf("GoFunc = %q, %v", c, ok)
	}
	if c, ok := DoorCanonicalKind("TerraformResource"); !ok || c != "resource" {
		t.Errorf("a manifest kind = %q, %v", c, ok)
	}
	// The product's CanonicalKind is unchanged: it does not know Go.
	if _, ok := CanonicalKind("GoFunc"); ok {
		t.Error("CanonicalKind learned a native kind, which changes the product's passes")
	}
}
