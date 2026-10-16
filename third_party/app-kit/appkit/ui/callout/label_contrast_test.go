package callout

import (
	"os"
	"regexp"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/tokens"
)

// FATHOM-F0-KIT-06: the Details affordance ("Show both versions" on Fathom's
// blocked-change preview, a hitl.DiffView inside a danger callout) was
// text-ink-faint, measured 3.16:1 light and 2.88:1 dark on danger-soft — below
// the 4.5:1 text floor. It is the only way to reveal the elaboration, so it is
// text a reader needs, not scenery (docs/theming-hazards.md H6). The label role
// is read from the rendered summary and measured over every kind's soft ground
// in both schemes of the kit token set.
func TestDetailsLabelClearsTheTextFloorOnEveryKind(t *testing.T) {
	src, err := os.ReadFile("../../../cue/tokens/kit.cue")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := tokens.Load(src)
	if err != nil {
		t.Fatal(err)
	}
	labelRe := regexp.MustCompile(`<span class="ml-auto flex shrink-0 items-center gap-1 text-xs text-([a-z-]+)"><span>Show both versions</span>`)
	for _, kind := range []Kind{Info, Success, Caution, Danger, Note} {
		got := Render(Callout{Kind: kind, Title: "t", Body: "b", Details: "<p>d</p>", DetailsLabel: "Show both versions"})
		m := labelRe.FindStringSubmatch(got)
		if m == nil {
			t.Fatalf("%s: no details label found in:\n%s", kind, got)
		}
		ground := kinds[kind].role + "-soft"
		for _, scheme := range []string{"light", "dark"} {
			ratio, err := tokens.ContrastRoles(tok, m[1], ground, scheme)
			if err != nil {
				t.Fatalf("%s %s: %v", kind, scheme, err)
			}
			if ratio < tokens.TextFloor {
				t.Errorf("%s (%s): details label text-%s on %s is %.2f:1, below the %.1f:1 text floor",
					kind, scheme, m[1], ground, ratio, tokens.TextFloor)
			}
		}
	}
}
