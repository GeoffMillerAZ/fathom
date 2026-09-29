package tokens

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scrimAlpha is the opacity appkit/ui/overlay draws the scrim at
// (`backdrop:bg-scrim/50`); TestOverlayBackdropIsTheScrimRole in
// appkit/ui/overlay pins the class to the same number.
const scrimAlpha = 0.5

type tokenSource struct {
	name string
	src  []byte
}

// shippedTokenSets returns every token set the kit ships: the default set
// first, then each named theme under cue/tokens/<name>/ in path order.
func shippedTokenSets(t *testing.T) []tokenSource {
	t.Helper()
	paths, err := filepath.Glob("../../../cue/tokens/*/*.cue")
	if err != nil {
		t.Fatal(err)
	}
	paths = append([]string{"../../../cue/tokens/kit.cue"}, paths...)
	var out []tokenSource
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		out = append(out, tokenSource{name: filepath.Base(p), src: src})
	}
	if len(out) < 7 {
		t.Fatalf("found %d shipped token sets, want at least 7", len(out))
	}
	return out
}

// FATHOM-S0-KIT-02: the dialog and drawer backdrop was `bg-black/40`, a
// palette color outside the role vocabulary every other kit class uses. The
// backdrop is now the `scrim` role. A scrim exists to push the page back, so in
// every shipped set, in both schemes, drawing it at the overlay's opacity must
// darken that scheme's own surface — a scrim that lightens a dark ground washes
// the page out instead — and must dim it at least as much as the black/40 it
// replaces did (within rounding), so no theme's dialogs got quieter.
func TestScrimRoleDimsEveryShippedSurface(t *testing.T) {
	for _, set := range shippedTokenSets(t) {
		name := set.name
		tok, err := Load(set.src)
		if err != nil {
			t.Fatalf("%s: Load: %v", name, err)
		}
		if _, ok := tok.Colors["scrim"]; !ok {
			t.Errorf("%s: no scrim role — the overlay backdrop class would never be generated", name)
			continue
		}
		css := EmitThemeCSS(tok)
		if !strings.Contains(css, "--color-scrim: var(--ak-color-scrim);") {
			t.Errorf("%s: theme CSS does not map the scrim utility", name)
		}
		if n := strings.Count(css, "--ak-color-scrim: "); n != 4 {
			t.Errorf("%s: --ak-color-scrim defined in %d branches, want 4 (root, dark, light, OS dark)", name, n)
		}
		for _, scheme := range []string{"light", "dark"} {
			surface, _ := roleValue(tok, "surface", scheme)
			scrim, _ := roleValue(tok, "scrim", scheme)
			ground, err := parseColor(surface)
			if err != nil {
				t.Fatalf("%s %s surface: %v", name, scheme, err)
			}
			veil, err := parseColor(scrim)
			if err != nil {
				t.Fatalf("%s %s scrim %q: %v", name, scheme, scrim, err)
			}
			black, _ := parseColor("#000000")
			dimmed := composite(veil, scrimAlpha, ground)
			before := composite(black, 0.4, ground)
			if relLuminance(dimmed) >= relLuminance(ground) {
				t.Errorf("%s %s: scrim %s at %.0f%% lightens the surface %s — it must recede the page, not wash it out",
					name, scheme, scrim, scrimAlpha*100, surface)
			}
			now, was := ratio(ground, dimmed), ratio(ground, before)
			if now < was-0.01 {
				t.Errorf("%s %s: scrim dims the surface %.3f:1, weaker than the black/40 it replaced (%.3f:1)", name, scheme, now, was)
			}
		}
	}
}

// Every app-authored tokens.cue written before the role existed omits scrim.
// Load must still accept it and fill the role, or those apps' next `appkit
// gen` would drop the backdrop utility (H4: a class whose role is missing is
// never generated). The default follows each branch's MEASURED polarity, not
// its name (H1): synthwave-style sets paint their "light" branch dark.
func TestLoad_DefaultsAnOmittedScrimByBranchPolarity(t *testing.T) {
	kit := mustLoad(t, kitSource(t))
	authored, ok := kit.Colors["scrim"]
	if !ok {
		t.Fatal("kit.cue does not author the scrim role")
	}
	if authored.Light != defaultScrimOnLight || authored.Dark != defaultScrimOnDark {
		t.Errorf("kit.cue scrim %+v disagrees with the loader defaults %q/%q — the default set and the fallback must be one value",
			authored, defaultScrimOnLight, defaultScrimOnDark)
	}

	omitted := dropScrimLine(t, kitSource(t))
	tok, err := Load(omitted)
	if err != nil {
		t.Fatalf("a token set without scrim no longer loads: %v", err)
	}
	if got := tok.Colors["scrim"]; got != authored {
		t.Errorf("omitted scrim defaulted to %+v, want the kit's %+v", got, authored)
	}

	darkOnly := []byte(`
tokens: {
	name: "dark-only"
	fonts: {sans: "x", mono: "y"}
	radius: {sm: "1px", md: "2px", lg: "3px", xl: "4px", full: "9999px"}
	colors: {
		surface: {light: "#0e0b1a", dark: "#0e0b1a"}
		"surface-raised": {light: "#1b1640", dark: "#1b1640"}
		"surface-sunken": {light: "#141029", dark: "#141029"}
		"surface-overlay": {light: "#221b52", dark: "#221b52"}
		ink: {light: "#ece9ff", dark: "#ece9ff"}
		"ink-muted": {light: "#9b93c9", dark: "#9b93c9"}
		"ink-faint": {light: "#6f679b", dark: "#6f679b"}
		"ink-inverse": {light: "#0e0b1a", dark: "#0e0b1a"}
		line: {light: "#2c2552", dark: "#2c2552"}
		"line-strong": {light: "#3a3270", dark: "#3a3270"}
		accent: {light: "#7c4dff", dark: "#7c4dff"}
		"accent-hover": {light: "#8f66ff", dark: "#8f66ff"}
		"accent-ink": {light: "#f4f1ff", dark: "#f4f1ff"}
		"accent-soft": {light: "#2a2150", dark: "#2a2150"}
		positive: {light: "#36e0a0", dark: "#36e0a0"}
		"positive-soft": {light: "#103a30", dark: "#103a30"}
		caution: {light: "#ffd166", dark: "#ffd166"}
		"caution-soft": {light: "#3a3010", dark: "#3a3010"}
		danger: {light: "#ff5c8a", dark: "#ff5c8a"}
		"danger-soft": {light: "#3a1022", dark: "#3a1022"}
		info: {light: "#4dc9ff", dark: "#4dc9ff"}
		"info-soft": {light: "#10304a", dark: "#10304a"}
		focus: {light: "#8f66ff", dark: "#8f66ff"}
	}
}`)
	dark, err := Load(darkOnly)
	if err != nil {
		t.Fatalf("Load dark-only set: %v", err)
	}
	if got := dark.Colors["scrim"]; got.Light != defaultScrimOnDark || got.Dark != defaultScrimOnDark {
		t.Errorf("dark-only set's scrim = %+v, want %q in both branches (its light branch paints dark)", got, defaultScrimOnDark)
	}

	// An authored scrim is never overridden.
	custom := bytes.Replace(kitSource(t),
		[]byte(`scrim: {light: "`+defaultScrimOnLight+`", dark: "`+defaultScrimOnDark+`"}`),
		[]byte(`scrim: {light: "oklch(0.25 0.02 60)", dark: "oklch(0.08 0.02 60)"}`), 1)
	own, err := Load(custom)
	if err != nil {
		t.Fatalf("Load custom scrim: %v", err)
	}
	if got := own.Colors["scrim"]; got.Light != "oklch(0.25 0.02 60)" || got.Dark != "oklch(0.08 0.02 60)" {
		t.Errorf("an authored scrim was replaced: %+v", got)
	}
}

// dropScrimLine removes the scrim role (and only it) from a token source.
func dropScrimLine(t *testing.T, src []byte) []byte {
	t.Helper()
	var kept [][]byte
	removed := 0
	for _, line := range bytes.Split(src, []byte("\n")) {
		if bytes.HasPrefix(bytes.TrimSpace(line), []byte("scrim:")) {
			removed++
			continue
		}
		kept = append(kept, line)
	}
	if removed != 1 {
		t.Fatalf("expected exactly one scrim line in the source, found %d", removed)
	}
	return bytes.Join(kept, []byte("\n"))
}
