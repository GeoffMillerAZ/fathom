package theme_test

import (
	"bytes"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/geoffmilleraz/app-kit/appkit/cli/theme"
	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
)

// TestPaletteCoversAllRoles asserts both schemes carry all 23 roles as
// parseable hex — the palette a terminal command actually gets, not just
// what the generator claims to have emitted.
func TestPaletteCoversAllRoles(t *testing.T) {
	const wantRoles = 23
	if len(theme.Roles) != wantRoles {
		t.Fatalf("Roles: got %d roles, want %d", len(theme.Roles), wantRoles)
	}
	for _, r := range theme.Roles {
		lh, ok := theme.LightHex[r]
		if !ok {
			t.Fatalf("LightHex missing role %q", r)
		}
		assertHex(t, "LightHex["+string(r)+"]", lh)
		if _, ok := theme.Default.Light[r]; !ok {
			t.Fatalf("Default.Light missing role %q", r)
		}

		dh, ok := theme.DarkHex[r]
		if !ok {
			t.Fatalf("DarkHex missing role %q", r)
		}
		assertHex(t, "DarkHex["+string(r)+"]", dh)
		if _, ok := theme.Default.Dark[r]; !ok {
			t.Fatalf("Default.Dark missing role %q", r)
		}
	}
}

func assertHex(t *testing.T, label, hex string) {
	t.Helper()
	if len(hex) != 7 || hex[0] != '#' {
		t.Fatalf("%s: %q is not a 6-digit #rrggbb hex color", label, hex)
	}
	for _, r := range hex[1:] {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Fatalf("%s: %q is not a 6-digit #rrggbb hex color", label, hex)
		}
	}
}

// TestPaletteRegenByteIdentical proves palette_gen.go is exactly what
// `appkit gen` would produce from the committed CUE token set right now —
// the same regeneration `task check`'s byte-identity gate runs, exercised
// here so a plain `go test` also catches a hand-edit or a stale
// generation, hermetically (no subprocess, no git, no working-tree state).
func TestPaletteRegenByteIdentical(t *testing.T) {
	root := repoRoot(t)

	src, err := os.ReadFile(filepath.Join(root, "cue", "tokens", "kit.cue"))
	if err != nil {
		t.Fatalf("read cue/tokens/kit.cue: %v", err)
	}
	tok, err := tokens.Load(src)
	if err != nil {
		t.Fatalf("tokens.Load: %v", err)
	}
	got, err := tokens.EmitTerminalPalette(tok, "theme")
	if err != nil {
		t.Fatalf("EmitTerminalPalette: %v", err)
	}
	want, err := os.ReadFile(filepath.Join(root, "appkit", "cli", "theme", "palette_gen.go"))
	if err != nil {
		t.Fatalf("read committed palette_gen.go: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("appkit/cli/theme/palette_gen.go is stale — regenerate with `task gen` (or `go run ./cmd/appkit gen`)")
	}
}

// repoRoot locates the module root from this test file's own path, so the
// test needs no subprocess and no assumption about the working directory
// `go test` was invoked from.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	// this file: appkit/cli/theme/theme_test.go — three levels up is root.
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

// TestOKLCHConversionKnownValues checks tokens.OKLCHToHex against the sRGB
// primaries' well-known OKLCH coordinates (Björn Ottosson,
// https://bottosson.github.io/posts/oklab/ — reproduced by oklch.com and
// culori's own test fixtures). These primaries sit exactly on the sRGB
// gamut boundary, which also exercises fitChroma's boundary case.
func TestOKLCHConversionKnownValues(t *testing.T) {
	cases := []struct {
		name  string
		oklch string
		want  string
	}{
		{"white", "oklch(1 0 0)", "#ffffff"},
		{"black", "oklch(0 0 0)", "#000000"},
		{"red", "oklch(0.627955 0.257683 29.2339)", "#ff0000"},
		{"green", "oklch(0.866440 0.294827 142.495)", "#00ff00"},
		{"blue", "oklch(0.452014 0.313214 264.052)", "#0000ff"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := tokens.OKLCHToHex(c.oklch)
			if err != nil {
				t.Fatalf("OKLCHToHex(%q): %v", c.oklch, err)
			}
			if got != c.want {
				t.Errorf("OKLCHToHex(%q) = %q, want %q", c.oklch, got, c.want)
			}
		})
	}
}

// TestOKLCHConversionRejectsGarbage guards the error path used by
// EmitTerminalPalette: a malformed token value must fail loudly, not
// silently become black.
func TestOKLCHConversionRejectsGarbage(t *testing.T) {
	for _, bad := range []string{"", "not a color", "#fff", "rgb(1,2,3)", "oklch(1 2)"} {
		if _, err := tokens.OKLCHToHex(bad); err == nil {
			t.Errorf("OKLCHToHex(%q): want error, got nil", bad)
		}
	}
}

// TestFangColorSchemeUsesLightDarkFunc proves FangColorScheme actually
// calls the supplied LightDarkFunc rather than hard-coding one scheme —
// the whole point of the fang.ColorSchemeFunc shape.
func TestFangColorSchemeUsesLightDarkFunc(t *testing.T) {
	schemeFn := theme.FangColorScheme()

	lightCS := schemeFn(lipgloss.LightDark(false))
	darkCS := schemeFn(lipgloss.LightDark(true))

	if colorHex(t, lightCS.Title) == colorHex(t, darkCS.Title) {
		t.Fatalf("Title resolved to the same color in both schemes: %s", colorHex(t, lightCS.Title))
	}
	if got, want := colorHex(t, lightCS.Title), theme.LightHex[theme.RoleAccent]; got != want {
		t.Errorf("light Title = %s, want accent light hex %s", got, want)
	}
	if got, want := colorHex(t, darkCS.Title), theme.DarkHex[theme.RoleAccent]; got != want {
		t.Errorf("dark Title = %s, want accent dark hex %s", got, want)
	}

	if lightCS.ErrorHeader[0] == nil || lightCS.ErrorHeader[1] == nil {
		t.Fatalf("ErrorHeader must carry both fg and bg colors, got %v", lightCS.ErrorHeader)
	}
}

func colorHex(t *testing.T, c color.Color) string {
	t.Helper()
	r, g, b, _ := c.RGBA()
	// color.Color.RGBA returns alpha-premultiplied 16-bit channels; for our
	// fully-opaque palette >>8 recovers the original 8-bit byte exactly.
	const shift = 8
	const digits = "0123456789abcdef"
	hb := func(v byte) string { return string([]byte{digits[v>>4], digits[v&0x0f]}) }
	return "#" + hb(byte(r>>shift)) + hb(byte(g>>shift)) + hb(byte(b>>shift))
}
