package motion

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

// cssRule is one parsed rule and the at-rule it sits inside ("" at top level).
type cssRule struct {
	at   string
	sel  string
	body string
}

// stripComments removes /* … */ so comment prose never satisfies (or trips) an
// assertion about the rules.
func stripComments(s string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, "/*")
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		j := strings.Index(s[i:], "*/")
		if j < 0 {
			return b.String()
		}
		s = s[i+j+2:]
	}
}

// parseCSS is a deliberately small reader for the shape motion.css is written
// in: flat rules, optionally nested one level inside an at-rule. It is not a
// CSS parser; if this file ever needs one, the stylesheet has grown past what
// a motion layer should be.
func parseCSS(t *testing.T, src string) []cssRule {
	t.Helper()
	src = stripComments(src)
	var out []cssRule
	var at []string
	var sel strings.Builder
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '{':
			s := strings.Join(strings.Fields(sel.String()), " ")
			sel.Reset()
			if strings.HasPrefix(s, "@") {
				at = append(at, s)
				continue
			}
			j := strings.IndexByte(src[i:], '}')
			if j < 0 {
				t.Fatalf("motion.css: unterminated rule %q", s)
			}
			out = append(out, cssRule{at: strings.Join(at, " "), sel: s, body: src[i+1 : i+j]})
			i += j
		case '}':
			if len(at) > 0 {
				at = at[:len(at)-1]
			}
			sel.Reset()
		default:
			sel.WriteByte(src[i])
		}
	}
	if len(out) == 0 {
		t.Fatalf("motion.css: parsed no rules")
	}
	return out
}

// TestCSS_EveryTransitionIsScopedToTheWrapper is the invariant, mechanised.
//
// The reduced-motion escape is not a block bolted onto the end of the file; it
// is the same selector that grants the motion. So a transition that is NOT
// scoped to .ak-motion is a transition the escape cannot cancel — motion a
// component author got without the opt-out, silently. That is the failure this
// test exists to make impossible for a future editor.
func TestCSS_EveryTransitionIsScopedToTheWrapper(t *testing.T) {
	n := 0
	for _, r := range parseCSS(t, motionCSS) {
		if !strings.Contains(r.body, "transition-") && !strings.Contains(r.body, "animation-") {
			continue
		}
		n++
		if !strings.Contains(r.sel, "."+Root) {
			t.Errorf("rule %q declares motion but is not scoped to .%s — the reduced-motion escape cannot reach it", r.sel, Root)
		}
	}
	if n == 0 {
		t.Fatalf("no motion rules found; the parser or the stylesheet is wrong")
	}
}

// TestCSS_ReducedMotionEscape asserts the cancel exists, covers descendants and
// generated content, and neutralises delay as well as duration — a stagger left
// running under reduced motion is still motion, just later.
func TestCSS_ReducedMotionEscape(t *testing.T) {
	rules := parseCSS(t, motionCSS)
	var found []cssRule
	for _, r := range rules {
		if strings.Contains(r.at, "prefers-reduced-motion: reduce") {
			found = append(found, r)
		}
	}
	if len(found) == 0 {
		t.Fatalf("motion.css has no @media (prefers-reduced-motion: reduce) block")
	}
	for _, r := range found {
		if !strings.Contains(r.sel, "."+Root) {
			t.Errorf("reduced-motion rule %q is not keyed on .%s", r.sel, Root)
		}
		for _, want := range []string{".ak-motion *", "*::before", "*::after"} {
			if !strings.Contains(r.sel, want) {
				t.Errorf("reduced-motion selector %q does not cover %q", r.sel, want)
			}
		}
		for _, want := range []string{
			"transition-duration", "transition-delay",
			"animation-duration", "animation-delay", "animation-iteration-count",
		} {
			if !strings.Contains(r.body, want+": ") || !strings.Contains(r.body, "!important") {
				t.Errorf("reduced-motion block does not cancel %s with !important", want)
			}
		}
		// 1ms, not 0s/none: a zero-duration transition fires no
		// transitionend, so JS awaiting one would hang.
		if strings.Contains(r.body, "transition-duration: 0") || strings.Contains(r.body, "transition: none") {
			t.Errorf("reduced-motion block zeroes the duration; use 1ms so transitionend still fires")
		}
	}
}

// TestCSS_StateRulesSurviveWithoutTheWrapper proves the no-motion baseline is
// real: drop .ak-motion entirely and the two-state switch still works, instantly.
// That instant version is what a reduced-motion reader sees, which is why both
// states have to be legible on their own.
func TestCSS_StateRulesSurviveWithoutTheWrapper(t *testing.T) {
	rules := parseCSS(t, motionCSS)
	for _, cls := range []string{WhenA, WhenB, Tone} {
		ok := false
		for _, r := range rules {
			if r.at == "" && strings.Contains(r.sel, "."+cls) && !strings.Contains(r.sel, "."+Root) {
				ok = true
			}
		}
		if !ok {
			t.Errorf(".%s has no rule outside .%s — the state would only exist when motion is enabled", cls, Root)
		}
	}
}

// TestCSS_StateOverridesWinOnSpecificity guards the specificity hazard: an
// override at equal specificity that only wins because it comes later is
// invisible in review and breaks the moment a rule moves. Every state-B
// override here is a descendant selector, strictly more specific than the
// state-A rule it replaces.
func TestCSS_StateOverridesWinOnSpecificity(t *testing.T) {
	for _, want := range []string{
		"." + StateB + " ." + WhenA,
		"." + StateB + " ." + WhenB,
		"." + StateB + " ." + Tone,
	} {
		if !strings.Contains(stripComments(motionCSS), want) {
			t.Errorf("missing descendant-scoped state override %q", want)
		}
	}
	seen := map[string]bool{}
	for _, r := range parseCSS(t, motionCSS) {
		key := r.at + "|" + r.sel
		if seen[key] {
			t.Errorf("duplicate selector %q — a later rule winning at equal specificity is the exact hazard this layer avoids", r.sel)
		}
		seen[key] = true
	}
}

// TestCSS_NoColorLiteralsAndNoThemeGating applies two kit rules to this file.
// Colors are never literals; and an override must key off a role token, never
// off a theme selector, because a second shipped token set remaps the palette
// while leaving data-theme untouched.
func TestCSS_NoColorLiteralsAndNoThemeGating(t *testing.T) {
	css := stripComments(motionCSS)
	if m := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`).FindString(css); m != "" {
		t.Errorf("motion.css contains a color literal %q", m)
	}
	for _, fn := range []string{"rgb(", "rgba(", "hsl(", "oklch(", "color-mix("} {
		if strings.Contains(css, fn) {
			t.Errorf("motion.css contains a color function %q; colors belong to the token layer", fn)
		}
	}
	for _, bad := range []string{"[data-theme", "prefers-color-scheme"} {
		if strings.Contains(css, bad) {
			t.Errorf("motion.css gates on %q; motion keys off state, color keys off role tokens", bad)
		}
	}
}

// TestCSS_EveryVarHasFallback guards the worst failure mode there is: a custom
// property that is referenced but never defined makes the whole declaration
// invalid at computed-value time, and the property silently reverts with no
// error and nothing to see in review.
func TestCSS_EveryVarHasFallback(t *testing.T) {
	css := stripComments(motionCSS)
	bare := regexp.MustCompile(`var\(\s*--[A-Za-z0-9-]+\s*\)`).FindAllString(css, -1)
	for _, m := range bare {
		t.Errorf("%s is referenced with no fallback; an undefined custom property fails silently", m)
	}
}

// TestCSS_VocabularyMatchesGo keeps the Go constants and the stylesheet from
// drifting in either direction — a constant naming a class the CSS does not
// define, or a class the CSS defines that has no Go name.
func TestCSS_VocabularyMatchesGo(t *testing.T) {
	css := stripComments(motionCSS)
	known := map[string]bool{}
	for _, c := range []string{Root, StateB, WhenA, WhenB, Tone, Fade, Tint, Shift, Anim, After} {
		known[c] = true
		if !strings.Contains(css, "."+c) {
			t.Errorf("Go constant %q names a class motion.css does not define", c)
		}
	}
	for i := 1; i <= MaxStep; i++ {
		s := Step(i)
		known[s] = true
		if !strings.Contains(css, "."+s) {
			t.Errorf("Step(%d) = %q is not defined in motion.css", i, s)
		}
	}
	for _, r := range parseCSS(t, motionCSS) {
		for _, m := range regexp.MustCompile(`\.(ak-[a-z0-9-]+)`).FindAllStringSubmatch(r.sel, -1) {
			if !known[m[1]] {
				t.Errorf("motion.css defines .%s with no exported Go name", m[1])
			}
		}
	}
	// Every knob the doc comment promises must actually be read somewhere.
	for _, v := range []string{
		"--ak-motion-duration", "--ak-motion-ease", "--ak-motion-delay",
		"--ak-motion-step", "--ak-motion-dim", "--ak-motion-props",
		"--ak-motion-tone-a", "--ak-motion-tone-b",
	} {
		if !strings.Contains(css, v) {
			t.Errorf("documented knob %s is not used by motion.css", v)
		}
	}
}

func TestStep(t *testing.T) {
	if got := Step(1); got != "ak-step-1" {
		t.Errorf("Step(1) = %q", got)
	}
	if got := Step(MaxStep); got != "ak-step-8" {
		t.Errorf("Step(MaxStep) = %q", got)
	}
	for _, i := range []int{-1, 0, MaxStep + 1} {
		if got := Step(i); got != "" {
			t.Errorf("Step(%d) = %q, want \"\" so it concatenates unconditionally", i, got)
		}
	}
}

func TestDelayStyle(t *testing.T) {
	if got := DelayStyle(350 * time.Millisecond); got != "--ak-motion-delay:350ms" {
		t.Errorf("DelayStyle(350ms) = %q", got)
	}
	for _, d := range []time.Duration{0, -1 * time.Second, 400 * time.Microsecond} {
		if got := DelayStyle(d); got != "--ak-motion-delay:0ms" {
			t.Errorf("DelayStyle(%v) = %q, want a clamped 0ms", d, got)
		}
	}
	// Style-attribute safety: digits and the property name only.
	if !regexp.MustCompile(`^--ak-motion-delay:\d+ms$`).MatchString(DelayStyle(2 * time.Second)) {
		t.Errorf("DelayStyle emitted something other than a numeric declaration")
	}
}

func TestRootClass(t *testing.T) {
	if got := RootClass(false); got != "ak-motion" {
		t.Errorf("RootClass(false) = %q", got)
	}
	if got := RootClass(true); got != "ak-motion ak-state-b" {
		t.Errorf("RootClass(true) = %q", got)
	}
}

// TestScript_Contract mirrors the kit's other embedded scripts: one global, no
// build step, no dependencies — and here, the JS state class must be the same
// string the CSS and the Go constant use.
func TestScript_Contract(t *testing.T) {
	js := Script()
	if !strings.Contains(js, "w.AppKitMotion = {") {
		t.Errorf("motion.js does not register window.AppKitMotion")
	}
	if !strings.Contains(js, `var STATE = "`+StateB+`";`) {
		t.Errorf("motion.js STATE does not match the Go constant %q", StateB)
	}
	for _, fn := range []string{"flush:", "frame:", "enter:", "toggle:", "reduced:"} {
		if !strings.Contains(js, fn) {
			t.Errorf("motion.js does not export %s", fn)
		}
	}
	for _, bad := range []string{"import ", "require(", "=>"} {
		if strings.Contains(js, bad) {
			t.Errorf("motion.js uses %q; it must stay build-free ES5-flavoured vanilla", bad)
		}
	}
}

// TestScript_FlushForcesLayoutOnSVG pins the fix for the SVG defect: flush()
// must force layout with getBoundingClientRect(), never the offsetWidth
// property. offsetWidth is defined only on HTMLElement — on an SVGElement (the
// <g>/<rect> marks this package's own example at motion.go animates) it reads
// back `undefined`, forcing no layout, so flush() and enter() silently do
// nothing on exactly their primary use case. Verified in Chrome: with
// offsetWidth, flush(svgG)/enter(svgG) did not run while the HTML path did;
// with getBoundingClientRect both run (docs/plans/motion-provenance.md F-1).
//
// The old assertion here pinned `offsetWidth` in place — anchoring on Script()
// alone would have passed before the fix, which is why this is its own named
// test rather than a line folded into the contract test.
func TestScript_FlushForcesLayoutOnSVG(t *testing.T) {
	js := Script()
	if !strings.Contains(js, "getBoundingClientRect") {
		t.Error("motion.js has no forced style recalculation via getBoundingClientRect — the one thing flush() exists for, and the only form that works on SVG")
	}
	if strings.Contains(js, "offsetWidth") {
		t.Error("motion.js reads offsetWidth, undefined on SVGElement — flush()/enter() no-op on SVG marks; use getBoundingClientRect().width")
	}
}

func TestHandlers(t *testing.T) {
	for _, tc := range []struct {
		name string
		h    http.Handler
		ct   string
		want string
	}{
		{"css", CSSHandler(), "text/css; charset=utf-8", "." + Root},
		{"js", Handler(), "application/javascript; charset=utf-8", "AppKitMotion"},
	} {
		rec := httptest.NewRecorder()
		tc.h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		if got := rec.Header().Get("Content-Type"); got != tc.ct {
			t.Errorf("%s: Content-Type = %q, want %q", tc.name, got, tc.ct)
		}
		if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
			t.Errorf("%s: Cache-Control = %q", tc.name, got)
		}
		if !strings.Contains(rec.Body.String(), tc.want) {
			t.Errorf("%s: body does not contain %q", tc.name, tc.want)
		}
	}
}

func TestCSS_Deterministic(t *testing.T) {
	if CSS() != CSS() || Script() != Script() {
		t.Errorf("embedded assets are not stable")
	}
}
