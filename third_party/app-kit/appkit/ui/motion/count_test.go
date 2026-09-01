package motion

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// TestCountScript_Contract — the L2 contract (ADR-0017 §1), asserted the same
// way as motion.js and router.js: exactly one global, no module syntax, no
// dependencies, no build step. An L2 artifact that needed a bundler would be an
// artifact only a Go app could use, which is the failure the layer exists to
// prevent.
func TestCountScript_Contract(t *testing.T) {
	js := CountScript()
	if js == "" {
		t.Fatal("count.js is empty — the //go:embed did not resolve")
	}

	// Exactly one global. Count the assignments rather than checking for the
	// one we expect, so a second global added later fails here.
	globals := regexp.MustCompile(`(?m)^\s*(?:global|window|w)\.([A-Za-z_$][\w$]*)\s*=`).FindAllStringSubmatch(js, -1)
	if len(globals) != 1 {
		t.Errorf("count.js registers %d globals, want exactly 1: %v", len(globals), globals)
	}
	if len(globals) > 0 && globals[0][1] != "AppKitCount" {
		t.Errorf("count.js registers %q, want AppKitCount", globals[0][1])
	}

	for _, bad := range []string{"import ", "export ", "require(", "=>", "module.exports"} {
		if strings.Contains(js, bad) {
			t.Errorf("count.js uses %q; it must stay a build-free classic script", bad)
		}
	}
	for _, want := range []string{"to:", "requestAnimationFrame", "cancelAnimationFrame", "prefers-reduced-motion"} {
		if !strings.Contains(js, want) {
			t.Errorf("count.js is missing %q", want)
		}
	}
}

// TestCountScript_ReducedMotionJumpsRatherThanSlows — the invariant that makes
// this promotable at all. Under prefers-reduced-motion the tween must not run
// at a gentler speed: a slower interpolation is still interpolation, and
// interpolation is what was asked to stop. The reduced() check has to
// short-circuit BEFORE the first requestAnimationFrame.
func TestCountScript_ReducedMotionJumpsRatherThanSlows(t *testing.T) {
	js := CountScript()
	guard := strings.Index(js, "reduced() ||")
	firstRAF := strings.Index(js, "global.requestAnimationFrame(step)")
	if guard < 0 {
		t.Fatal("count.js has no reduced-motion guard in its tween entry")
	}
	if firstRAF < 0 || guard > firstRAF {
		t.Error("count.js schedules a frame before checking reduced motion — the tween would run anyway")
	}
	if !strings.Contains(js, "onValue(target);") {
		t.Error("count.js does not call onValue with the exact target; the reduced path must land on the real number, and the animated path must not end on an easing artifact")
	}
}

// TestCountScript_HasNoDOMAssumptions — count.js is a pure tween. The moment it
// reaches for an element, a selector or document it stops being usable by the
// React consumer it was promoted from, and stops being promotable at the
// one-consumer L2 bar.
func TestCountScript_HasNoDOMAssumptions(t *testing.T) {
	body := CountScript()
	if i := strings.Index(body, "(function (global)"); i >= 0 {
		body = body[i:] // skip the header comment, which discusses the DOM
	}
	for _, bad := range []string{"document", "querySelector", "getElementById", "addEventListener", "innerHTML", ".style"} {
		if strings.Contains(body, bad) {
			t.Errorf("count.js touches %q; it must stay a pure tween with no DOM assumptions", bad)
		}
	}
}

func TestCountHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	CountHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/count.js", nil))
	if got := rec.Header().Get("Content-Type"); got != "application/javascript; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("Cache-Control = %q", got)
	}
	if rec.Body.String() != CountScript() {
		t.Error("CountHandler body differs from CountScript()")
	}
}

// TestScriptsAreDistinctGlobals — motion.js and count.js are separately
// adoptable. A page that uses the class vocabulary and no counters should not
// have to carry the tween, and vice versa.
func TestScriptsAreDistinctGlobals(t *testing.T) {
	if strings.Contains(CountScript(), "AppKitMotion") {
		t.Error("count.js references AppKitMotion; the two scripts must be independently adoptable")
	}
	if strings.Contains(Script(), "AppKitCount") {
		t.Error("motion.js references AppKitCount; the two scripts must be independently adoptable")
	}
}
