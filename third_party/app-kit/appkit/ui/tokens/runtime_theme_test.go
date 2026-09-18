package tokens

import (
	"strings"
	"testing"
)

func TestThemeFontsAndRadiiResolveAtRuntime(t *testing.T) {
	tok := mustLoad(t, kitSource(t))
	css := EmitThemeCSS(tok)
	for _, pair := range append(fontValues(tok), radiusValues(tok)...) {
		prefix := "font"
		if pair[0] == "sm" || pair[0] == "md" || pair[0] == "lg" || pair[0] == "xl" || pair[0] == "full" {
			prefix = "radius"
		}
		if !strings.Contains(css, "--"+prefix+"-"+pair[0]+": var(--ak-"+prefix+"-"+pair[0]+");") || !strings.Contains(css, "--ak-"+prefix+"-"+pair[0]+": "+pair[1]+";") {
			t.Errorf("%s %s is not runtime-bound with its original value", prefix, pair[0])
		}
	}
	changed := tok
	changed.Fonts.Sans, changed.Radius.MD = "'Synthetic Brand', serif", "1.125rem"
	next := EmitThemeCSS(changed)
	if !strings.Contains(next, "--ak-font-sans: 'Synthetic Brand', serif;") || !strings.Contains(next, "--ak-radius-md: 1.125rem;") || !strings.Contains(next, "--font-sans: var(--ak-font-sans);") {
		t.Fatal("identity change lost runtime binding")
	}
}
