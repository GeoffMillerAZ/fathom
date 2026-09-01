package tokens

import (
	"bytes"
	"go/format"
	"os"
	"strings"
	"testing"
)

func kitSource(t *testing.T) []byte {
	t.Helper()
	src, err := os.ReadFile("../../../cue/tokens/kit.cue")
	if err != nil {
		t.Fatalf("read kit.cue: %v", err)
	}
	return src
}

func TestLoad_KitTokens(t *testing.T) {
	tok, err := Load(kitSource(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tok.Name != "appkit" {
		t.Errorf("Name = %q, want appkit", tok.Name)
	}
	for _, role := range []string{"surface", "ink", "line", "accent", "focus", "danger-soft"} {
		pair, ok := tok.Colors[role]
		if !ok {
			t.Errorf("missing required color role %q", role)
			continue
		}
		if pair.Light == "" || pair.Dark == "" {
			t.Errorf("role %q has empty scheme value: %+v", role, pair)
		}
	}
}

func TestLoad_RejectsIncompleteSet(t *testing.T) {
	// Missing all required roles — the schema must refuse it.
	src := []byte(`
tokens: {
	name: "broken"
	fonts: {sans: "x", mono: "y"}
	radius: {sm: "1px", md: "2px", lg: "3px", xl: "4px", full: "9999px"}
	colors: {
		surface: {light: "oklch(1 0 0)", dark: "oklch(0 0 0)"}
	}
}`)
	if _, err := Load(src); err == nil {
		t.Fatal("Load accepted a token set missing required roles")
	}
}

func TestLoad_RejectsBadName(t *testing.T) {
	src := bytes.Replace(kitSource(t), []byte(`name:        "appkit"`), []byte(`name:        "Not Valid!"`), 1)
	if _, err := Load(src); err == nil {
		t.Fatal("Load accepted an invalid token-set name")
	}
}

func TestEmit_Deterministic(t *testing.T) {
	tok, err := Load(kitSource(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a, b := EmitThemeCSS(tok), EmitThemeCSS(tok); a != b {
		t.Error("EmitThemeCSS not deterministic across calls")
	}
	if a, b := EmitDTCG(tok), EmitDTCG(tok); !bytes.Equal(a, b) {
		t.Error("EmitDTCG not deterministic across calls")
	}
	if a, b := EmitGoNames(tok, "tokens"), EmitGoNames(tok, "tokens"); !bytes.Equal(a, b) {
		t.Error("EmitGoNames not deterministic across calls")
	}
}

func TestEmitThemeCSS_Contract(t *testing.T) {
	tok, err := Load(kitSource(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	css := EmitThemeCSS(tok)

	for _, want := range []string{
		"@theme inline {",
		"--color-surface: var(--ak-color-surface);",
		"[data-theme=\"dark\"]",
		"@media (prefers-color-scheme: dark)",
		"@custom-variant dark",
		"--ak-font-scale",
		"color-scheme: dark;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("theme CSS missing %q", want)
		}
	}
	// Light and dark values for the same role must both be present.
	if !strings.Contains(css, tok.Colors["surface"].Light) || !strings.Contains(css, tok.Colors["surface"].Dark) {
		t.Error("theme CSS missing light or dark surface value")
	}
}

// TestRadiusFontNamesMatchCSS ties the generated RadiusNames/FontNames
// vocabulary to the utilities EmitThemeCSS actually defines: every name must
// have a matching --radius-<name> / --font-<name> declaration, so adding a
// scale entry without wiring its CSS (or vice versa) fails here.
func TestRadiusFontNamesMatchCSS(t *testing.T) {
	tok, err := Load(kitSource(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	css := EmitThemeCSS(tok)
	for _, n := range RadiusNames {
		if !strings.Contains(css, "--radius-"+n+":") {
			t.Errorf("RadiusNames has %q but CSS defines no --radius-%s", n, n)
		}
	}
	for _, n := range FontNames {
		if !strings.Contains(css, "--font-"+n+":") {
			t.Errorf("FontNames has %q but CSS defines no --font-%s", n, n)
		}
	}
}

func TestEmitGoNames_GofmtClean(t *testing.T) {
	tok, err := Load(kitSource(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	out := EmitGoNames(tok, "tokens")
	formatted, err := format.Source(out)
	if err != nil {
		t.Fatalf("emitted Go does not parse: %v", err)
	}
	if !bytes.Equal(out, formatted) {
		t.Error("EmitGoNames output is not gofmt-clean")
	}
}

func TestGoName(t *testing.T) {
	cases := map[string]string{
		"surface":        "Surface",
		"surface-raised": "SurfaceRaised",
		"ink-muted":      "InkMuted",
	}
	for in, want := range cases {
		if got := goName(in); got != want {
			t.Errorf("goName(%q) = %q, want %q", in, got, want)
		}
	}
}
