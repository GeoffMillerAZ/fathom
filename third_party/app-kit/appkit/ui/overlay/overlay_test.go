package overlay

import (
	"regexp"
	"strings"
	"testing"
)

var hexColorRE = regexp.MustCompile(`(?i)(fill|stroke|color|background)[=:]["']?\s*#[0-9a-f]{3,8}\b`)

func TestRenderDialog(t *testing.T) {
	got := RenderDialog(Dialog{
		ID: "confirm", Title: "Delete case?", Size: "sm", LightDismiss: true,
		Body:   "This cannot be undone.",
		Footer: `<button data-overlay-dismiss>Cancel</button>`,
	})
	for _, want := range []string{
		`<dialog id="confirm"`, "max-w-sm", "data-overlay-light-dismiss",
		"data-overlay-panel", "Delete case?", "This cannot be undone.",
		"data-overlay-dismiss", "backdrop:bg-scrim/50",
		// Centered via inset+translate, not margin:auto (survives a host
		// `* { margin: 0 }` reset that beats Tailwind's layered utilities).
		"left-1/2", "top-1/2", "-translate-x-1/2", "-translate-y-1/2",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("dialog missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "m-auto") {
		t.Errorf("dialog must not rely on m-auto centering (host margin resets break it):\n%s", got)
	}
	// Default size when unset.
	if !strings.Contains(RenderDialog(Dialog{ID: "x"}), "max-w-md") {
		t.Error("dialog should default to max-w-md")
	}
}

// FATHOM-S0-KIT-02: the backdrop was bg-black/40, the one palette color in a
// kit component, so a consumer holding its markup to role utilities had to
// exempt it. Both modal surfaces now veil the page with the scrim role at 50%
// (appkit/ui/tokens scrim_test.go measures that opacity in every shipped set).
func TestOverlayBackdropIsTheScrimRole(t *testing.T) {
	for _, c := range []struct{ name, got string }{
		{"dialog", RenderDialog(Dialog{ID: "d", Title: "T", Body: "b"})},
		{"drawer", RenderDrawer(Drawer{ID: "r", Title: "T", Body: "b"})},
	} {
		name, got := c.name, c.got
		if n := strings.Count(got, "backdrop:"); n != 1 {
			t.Errorf("%s: %d backdrop utilities, want exactly one: %s", name, n, got)
		}
		if !strings.Contains(got, " backdrop:bg-scrim/50") {
			t.Errorf("%s: backdrop is not the scrim role at 50%%: %s", name, got)
		}
		if strings.Contains(got, "bg-black") {
			t.Errorf("%s: backdrop still paints a palette color: %s", name, got)
		}
	}
}

func TestRenderDrawer_Side(t *testing.T) {
	// Anchored with inset utilities (not margin:auto) so it survives a host
	// `* { margin: 0 }` reset — see RenderDrawer doc.
	right := RenderDrawer(Drawer{ID: "d", Title: "Details", Body: "x"})
	if !strings.Contains(right, "right-0") || !strings.Contains(right, "left-auto") {
		t.Errorf("right drawer should anchor right with inset utilities: %s", right)
	}
	left := RenderDrawer(Drawer{ID: "d", Side: "left", Body: "x"})
	if !strings.Contains(left, "left-0") || !strings.Contains(left, "right-auto") {
		t.Errorf("left drawer should anchor left with inset utilities: %s", left)
	}
	if !strings.Contains(right, "h-dvh") {
		t.Error("drawer should be full viewport height")
	}
}

func TestTrigger(t *testing.T) {
	if got := string(OpenAttr("my-dialog")); got != `data-overlay-open="my-dialog"` {
		t.Errorf("OpenAttr = %q", got)
	}
	btn := TriggerButton("my-dialog", "Open")
	if !strings.Contains(btn, `data-overlay-open="my-dialog"`) || !strings.Contains(btn, "Open") {
		t.Errorf("trigger button wrong: %s", btn)
	}
}

func TestToast_RolesAndRegion(t *testing.T) {
	if !strings.Contains(RenderToast(Toast{Message: "Saved", Role: "success"}), "border-positive") {
		t.Error("success toast should use positive role")
	}
	danger := RenderToast(Toast{Message: "Failed", Role: "danger"})
	if !strings.Contains(danger, `role="alert"`) || !strings.Contains(danger, "border-danger") {
		t.Errorf("danger toast should be an alert with danger styling: %s", danger)
	}
	if !strings.Contains(RenderToast(Toast{Message: "x"}), "border-info") {
		t.Error("default toast role should be info")
	}
	region := RenderToastRegion([]Toast{{Message: "Hi", Role: "info"}})
	for _, want := range []string{`id="ak-toast-region"`, `aria-live="polite"`, "Hi", "fixed"} {
		if !strings.Contains(region, want) {
			t.Errorf("toast region missing %q: %s", want, region)
		}
	}
}

func TestScript_RegistersAPIAndMatchesClasses(t *testing.T) {
	js := Script()
	for _, want := range []string{"AppKitOverlay", "showModal", "data-overlay-open", "data-overlay-dismiss", "ak-toast-region"} {
		if !strings.Contains(js, want) {
			t.Errorf("overlay.js missing %q", want)
		}
	}
	// The JS toast classes must match the server-rendered ones so styling is identical.
	for _, role := range []string{"border-positive bg-positive-soft", "border-danger bg-danger-soft", "border-info bg-info-soft"} {
		if !strings.Contains(js, role) {
			t.Errorf("overlay.js toast classes drifted from server (missing %q)", role)
		}
	}
}

func TestOverlay_EscapingAndTokensOnly(t *testing.T) {
	xss := `"><script>alert(1)</script>`
	all := RenderDialog(Dialog{ID: xss, Title: xss, Body: "safe"}) +
		RenderDrawer(Drawer{ID: xss, Title: xss}) +
		TriggerButton(xss, xss) +
		RenderToast(Toast{Message: xss})
	if strings.Contains(all, "<script>alert(1)</script>") {
		t.Errorf("unescaped script injected:\n%s", all)
	}
	if hexColorRE.MatchString(all) {
		t.Errorf("hardcoded hex color — must use token utilities:\n%s", all)
	}
}

func TestOverlay_Deterministic(t *testing.T) {
	f := func() string {
		return RenderDialog(Dialog{ID: "a", Title: "T", Body: "b"}) + RenderToastRegion([]Toast{{Message: "m"}})
	}
	if f() != f() {
		t.Error("overlay rendering is non-deterministic")
	}
}
