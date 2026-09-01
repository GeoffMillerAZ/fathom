// Package motion is the kit's motion vocabulary: two-state interpolation,
// staggering by transition-delay, and prefers-reduced-motion as an invariant
// rather than an afterthought.
//
// The contribution is motion.css — hand-written CSS (it derives from no token
// set, so unlike tokens/theme.css it is authored, not generated). This Go file
// ships it three ways, none of which requires using a kit component:
//
//   - [CSS] returns the stylesheet for inlining into a <style> tag;
//   - [CSSHandler] serves it;
//   - or a Tailwind app simply @imports appkit/ui/motion/motion.css after the
//     theme layer, exactly as it @imports tokens/theme.css.
//
// The class-name constants below exist so kit components compose the vocabulary
// without stringly-typed drift, and so motion_test.go can prove the Go names
// and the CSS never diverge. They are a convenience, not a gate: the classes
// are plain CSS and a consumer writing HTML by hand is a first-class user.
//
// # The technique
//
// Both states live in the DOM at once. A class on the root ([StateB]) selects
// which one is showing, and CSS interpolates between them. 216 marks do not get
// replaced by 18 others — they are the same elements changing opacity and fill.
// It costs nothing at runtime, it prints, and it survives a theme swap.
//
//	root := `<g class="` + motion.RootClass(collapsed) + `">` +
//	        `<g class="` + motion.WhenA + " " + motion.Tint + `">…216 marks…</g>` +
//	        `<g class="` + motion.WhenB + " " + motion.Fade + " " + motion.After + `">…18 marks…</g>` +
//	        `</g>`
//
// # The invariant
//
// Both states must be legible without motion. The animation is a transition
// BETWEEN two readable states, never the thing that makes them readable. The
// escape is structural: every transition in motion.css is scoped to
// [Root], and [Root] is the selector the reduced-motion block cancels — so a
// component cannot opt into kit motion without also opting into the cancel.
// motion_test.go fails the build if a future edit breaks that.
//
// # The optional JS
//
// Two files, both optional, both L2 (ADR-0017 §1) — classic scripts, zero
// dependencies, no build step, one global each:
//
//   - motion.js ([Script] / [Handler]) is one helper: a forced style
//     recalculation, needed because a transition applied in the same frame as
//     mount silently never runs and reports nothing. A consumer that toggles
//     [StateB] its own way does not need it. See the file's header.
//   - count.js ([CountScript] / [CountHandler]) is the counting tween:
//     AppKitCount.to(from, target, onValue) with a cubic ease and a cancel,
//     jumping straight to the target under prefers-reduced-motion. Promoted
//     from two byte-identical copies in a consuming app.
package motion

import (
	_ "embed"
	"net/http"
	"strconv"
	"time"
)

//go:embed motion.css
var motionCSS string

//go:embed motion.js
var motionJS string

//go:embed count.js
var countJS string

// Class names emitted by motion.css. Kit components compose these constants so
// the Go vocabulary and the stylesheet cannot drift apart; hand-written markup
// may use the string literals directly.
const (
	// Root enables kit transitions in a subtree AND carries the
	// reduced-motion cancel for everything inside it — including transitions
	// the consumer wrote themselves. Put it on the element that owns both
	// states. Useful on its own, with none of the classes below.
	Root = "ak-motion"
	// StateB on the same element as Root means the subtree is showing state
	// B. Its absence means state A.
	StateB = "ak-state-b"

	// WhenA marks an element that belongs to state A: opaque in A, dimmed to
	// --ak-motion-dim (default 0) in B.
	WhenA = "ak-when-a"
	// WhenB marks an element that belongs to state B.
	WhenB = "ak-when-b"
	// Tone marks an element present in both states whose color/fill changes
	// between them, interpolating --ak-motion-tone-a → --ak-motion-tone-b.
	// Point those at role tokens (var(--ak-color-danger)), never at literals.
	Tone = "ak-tone"

	// Fade transitions opacity.
	Fade = "ak-fade"
	// Tint transitions opacity plus color, background-color, border-color,
	// fill and stroke.
	Tint = "ak-tint"
	// Shift transitions opacity plus transform.
	Shift = "ak-shift"
	// Anim transitions whatever --ak-motion-props names (default opacity).
	// CSS transition lists do not merge across classes, so Fade/Tint/Shift/Anim
	// are presets: pick one per element.
	Anim = "ak-anim"

	// After delays an element by one full duration, so it moves only after
	// the wave behind it has settled.
	After = "ak-after"
)

// MaxStep is the highest index stagger class motion.css ships (ak-step-1 …
// ak-step-8). Past that, set --ak-motion-delay directly with [DelayStyle] —
// eight steps of visible stagger is already a long time to wait, and a
// generated ladder of hundreds of classes is worse than one number.
const MaxStep = 8

// Step returns the index-stagger class for a 1-based position, or "" when i is
// outside 1..[MaxStep] so it can be concatenated unconditionally. Each step is
// --ak-motion-step (default 60ms) of transition-delay.
func Step(i int) string {
	if i < 1 || i > MaxStep {
		return ""
	}
	return "ak-step-" + strconv.Itoa(i)
}

// DelayStyle returns a style-attribute declaration setting this element's
// transition delay, for a stagger that is not a step index — a phase delay, or
// a value computed from data. Negative and sub-millisecond durations clamp to
// 0ms. Output is digits only, so it is safe inside a style attribute.
//
//	`<g class="` + motion.WhenA + " " + motion.Fade + `" style="` + motion.DelayStyle(350*time.Millisecond) + `">`
func DelayStyle(d time.Duration) string {
	ms := d.Milliseconds()
	if ms < 0 {
		ms = 0
	}
	return "--ak-motion-delay:" + strconv.FormatInt(ms, 10) + "ms"
}

// RootClass returns the root element's class list for a given state, so the
// server can render either state directly. That matters: the initial state is
// server-rendered and correct with JS disabled, and the transition is only ever
// the change between two states the page could have shipped on its own.
func RootClass(inStateB bool) string {
	if inStateB {
		return Root + " " + StateB
	}
	return Root
}

// CSS returns motion.css for inlining into a page (<style> tag) or for writing
// to disk. Tailwind apps normally @import the file instead; this is the path
// for apps with no CSS build at all.
func CSS() string { return motionCSS }

// CSSHandler serves motion.css with a long-lived immutable cache header. Mount
// at a stable path, e.g. mux.Handle("GET /static/motion.css", motion.CSSHandler()).
func CSSHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = w.Write([]byte(motionCSS))
	})
}

// Script returns motion.js for inlining into a page (<script> tag). Optional:
// see the package doc.
func Script() string { return motionJS }

// Handler serves motion.js with a long-lived immutable cache header. Mount at a
// stable path, e.g. mux.Handle("GET /static/motion.js", motion.Handler()).
func Handler() http.Handler { return jsHandler(motionJS) }

// CountScript returns count.js for inlining into a page (<script> tag).
//
// It is a separate file from motion.js and a separate global on purpose: the
// tween is useful to a page that uses none of the rest of this package, and a
// page that uses the class vocabulary and no counters should not carry it. Both
// are optional; motion.css is the contribution.
//
//	AppKitCount.to(from, target, onValue[, {duration, round}])  → cancel()
func CountScript() string { return countJS }

// CountHandler serves count.js with a long-lived immutable cache header. Mount
// at a stable path, e.g. mux.Handle("GET /static/count.js", motion.CountHandler()).
func CountHandler() http.Handler { return jsHandler(countJS) }

func jsHandler(src string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = w.Write([]byte(src))
	})
}
