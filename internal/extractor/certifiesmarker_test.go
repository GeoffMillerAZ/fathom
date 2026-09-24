package extractor

import (
	"strings"
	"testing"
)

func TestParseCertifiesMarker_GrammarAndMalformed(t *testing.T) {
	t.Run("single entry, default strength", func(t *testing.T) {
		got, err := ParseCertifiesMarker(" invariant I35")
		if err != nil {
			t.Fatalf("ParseCertifiesMarker: %v", err)
		}
		want := []CertifyEntry{{Kind: CertifyKindInvariant, TargetID: "I35", Strength: StrengthAsserts}}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("explicit strength", func(t *testing.T) {
		got, err := ParseCertifiesMarker(" invariant I35 (golden)")
		if err != nil {
			t.Fatalf("ParseCertifiesMarker: %v", err)
		}
		if len(got) != 1 || got[0].Strength != StrengthGolden {
			t.Fatalf("got %+v, want strength golden", got)
		}
	})

	t.Run("multiple entries, mixed strengths", func(t *testing.T) {
		got, err := ParseCertifiesMarker(" invariant I35 (asserts), incident INC-21, port billing/ports.Ledger (conformance)")
		if err != nil {
			t.Fatalf("ParseCertifiesMarker: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("got %d entries, want 3: %+v", len(got), got)
		}
		if got[0] != (CertifyEntry{Kind: CertifyKindInvariant, TargetID: "I35", Strength: StrengthAsserts}) {
			t.Errorf("entry 0 = %+v", got[0])
		}
		if got[1] != (CertifyEntry{Kind: CertifyKindIncident, TargetID: "INC-21", Strength: StrengthAsserts}) {
			t.Errorf("entry 1 = %+v", got[1])
		}
		if got[2] != (CertifyEntry{Kind: CertifyKindPort, TargetID: "billing/ports.Ledger", Strength: StrengthConformance}) {
			t.Errorf("entry 2 = %+v", got[2])
		}
	})

	t.Run("id with special characters (rule target)", func(t *testing.T) {
		got, err := ParseCertifiesMarker(" rule CLAUDE.md#deploy")
		if err != nil {
			t.Fatalf("ParseCertifiesMarker: %v", err)
		}
		if len(got) != 1 || got[0].TargetID != "CLAUDE.md#deploy" {
			t.Fatalf("got %+v", got)
		}
	})

	malformed := []struct {
		name string
		body string
	}{
		{"empty body", "   "},
		{"kind with no id", " invariant"},
		{"unknown kind", " nonsense I35"},
		{"unknown strength", " invariant I35 (whenever)"},
		{"empty strength parens", " invariant I35 ()"},
		{"unbalanced parens", " invariant I35 asserts)"},
		{"stray trailing comma", " invariant I35,"},
		{"stray leading comma", ", invariant I35"},
	}
	for _, tc := range malformed {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseCertifiesMarker(tc.body); err == nil {
				t.Fatalf("ParseCertifiesMarker(%q) succeeded, want an error", tc.body)
			}
		})
	}
}

func TestScanCertifiesMarkers_FindsAndWarns(t *testing.T) {
	src := []byte(`package cart

// certifies: invariant I35 (asserts), incident INC-21
func TestAdd(t *T) {}

// not a certifies marker at all
func TestSubtract(t *T) {}

// certifies: not-a-kind I99
func TestBroken(t *T) {}

func helperNotATest(t *T) {}
`)
	decls, warnings, err := ScanCertifiesMarkers("cart-svc", "cart/cart_test.go", src)
	if err != nil {
		t.Fatalf("ScanCertifiesMarkers: %v", err)
	}
	if len(decls) != 2 {
		t.Fatalf("want 2 declarations from TestAdd's marker, got %d: %+v", len(decls), decls)
	}
	for _, d := range decls {
		if !strings.HasSuffix(d.TestSymbolKey, "cart.TestAdd") {
			t.Errorf("declaration TestSymbolKey = %q, want it to name TestAdd", d.TestSymbolKey)
		}
		if d.File != "cart/cart_test.go" || d.Line != 3 {
			t.Errorf("declaration file/line = %s:%d, want cart/cart_test.go:3", d.File, d.Line)
		}
	}
	if decls[0].Kind != CertifyKindInvariant || decls[0].TargetID != "I35" {
		t.Errorf("decl 0 = %+v", decls[0])
	}
	if decls[1].Kind != CertifyKindIncident || decls[1].TargetID != "INC-21" {
		t.Errorf("decl 1 = %+v", decls[1])
	}

	if len(warnings) != 1 {
		t.Fatalf("want 1 warning for TestBroken's malformed marker, got %d: %+v", len(warnings), warnings)
	}
	if warnings[0].File != "cart/cart_test.go" || warnings[0].Line != 9 {
		t.Errorf("warning file/line = %s:%d, want cart/cart_test.go:9", warnings[0].File, warnings[0].Line)
	}
}
