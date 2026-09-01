package callout

import (
	"strings"
	"testing"
)

func TestRender_KindRolesAndAria(t *testing.T) {
	cases := []struct {
		kind Kind
		role string
		aria string
	}{
		{Info, "info", "note"},
		{Success, "positive", "status"},
		{Caution, "caution", "alert"},
		{Danger, "danger", "alert"},
		{Note, "accent", "note"},
	}
	for _, c := range cases {
		got := Render(Callout{Kind: c.kind, Title: "T", Body: "B"})
		if !strings.Contains(got, "bg-"+c.role+"-soft") || !strings.Contains(got, "border-"+c.role) {
			t.Errorf("%s: missing role utilities for %q: %s", c.kind, c.role, got)
		}
		if !strings.Contains(got, `role="`+c.aria+`"`) {
			t.Errorf("%s: want aria role %q", c.kind, c.aria)
		}
	}
}

func TestRender_UnknownKindFallsBack(t *testing.T) {
	got := Render(Callout{Kind: "bogus", Body: "x"})
	if !strings.Contains(got, "bg-accent-soft") {
		t.Errorf("unknown kind should fall back to note/accent: %s", got)
	}
}

func TestRender_Escaping(t *testing.T) {
	got := Render(Callout{Kind: Info, Title: `<script>t</script>`, Body: `<img src=x>`})
	for _, bad := range []string{"<script>", "<img src=x>"} {
		if strings.Contains(got, bad) {
			t.Errorf("callout did not escape %q: %s", bad, got)
		}
	}
}

func TestRender_BodyOnly(t *testing.T) {
	got := Render(Callout{Kind: Info, Body: "just a line"})
	if strings.Contains(got, "font-medium") {
		t.Errorf("body-only callout should not render a title heading")
	}
	if !strings.Contains(got, "just a line") {
		t.Errorf("body missing")
	}
}

func TestRender_BodyHTML(t *testing.T) {
	// Trusted rich body is emitted verbatim (links/emphasis preserved).
	got := Render(Callout{Kind: Info, Title: "T", BodyHTML: `see <a href="/x">settings</a>`})
	if !strings.Contains(got, `<a href="/x">settings</a>`) {
		t.Errorf("BodyHTML not rendered verbatim: %s", got)
	}
	// BodyHTML takes precedence over Body when both set.
	both := Render(Callout{Kind: Info, Body: "plain", BodyHTML: `<em>rich</em>`})
	if strings.Contains(both, "plain") || !strings.Contains(both, "<em>rich</em>") {
		t.Errorf("BodyHTML should take precedence over Body: %s", both)
	}
}

func TestRender_BlockBody(t *testing.T) {
	// BodyHTML uses a <div> wrapper (not <p>) so block content like a list is
	// valid; plain Body stays in a <p>.
	block := Render(Callout{Kind: Info, BodyHTML: `<ol><li>one</li></ol>`})
	if !strings.Contains(block, `<div class="text-sm text-ink"><ol>`) {
		t.Errorf("BodyHTML should render inside a div wrapper: %s", block)
	}
	plain := Render(Callout{Kind: Info, Body: "hi"})
	if !strings.Contains(plain, `<p class="text-sm text-ink">hi</p>`) {
		t.Errorf("plain Body should stay in a <p>: %s", plain)
	}
}

func TestRender_HideIcon(t *testing.T) {
	with := Render(Callout{Kind: Info, Body: "x"})
	without := Render(Callout{Kind: Info, Body: "x", HideIcon: true})
	if !strings.Contains(with, "<svg") {
		t.Errorf("default callout should include an icon")
	}
	if strings.Contains(without, "<svg") {
		t.Errorf("HideIcon should omit the icon")
	}
}

func TestRender_Deterministic(t *testing.T) {
	a := Render(Callout{Kind: Danger, Title: "T", Body: "B"})
	b := Render(Callout{Kind: Danger, Title: "T", Body: "B"})
	if a != b {
		t.Errorf("callout non-deterministic")
	}
}

func TestDisclaimer(t *testing.T) {
	got := Disclaimer("")
	if !strings.Contains(got, "bg-caution-soft") {
		t.Errorf("disclaimer should be a caution callout")
	}
	if !strings.Contains(got, "not professional advice") {
		t.Errorf("default disclaimer wording missing: %s", got)
	}
	custom := Disclaimer("Custom caveat.")
	if !strings.Contains(custom, "Custom caveat.") {
		t.Errorf("custom disclaimer body not used")
	}
}

func TestBadge(t *testing.T) {
	got := Badge("Strong", "positive")
	if !strings.Contains(got, "bg-positive-soft") || !strings.Contains(got, "text-positive") {
		t.Errorf("badge role utilities wrong: %s", got)
	}
	if !strings.Contains(got, ">Strong<") {
		t.Errorf("badge text missing")
	}
}

func TestBadge_NeutralFallback(t *testing.T) {
	got := Badge("Unknown", "ink")
	if !strings.Contains(got, "bg-surface-sunken") || !strings.Contains(got, "text-ink-muted") {
		t.Errorf("unsupported role should render neutral pill: %s", got)
	}
	if strings.Contains(got, "soft") {
		t.Errorf("neutral badge should not reference a -soft role: %s", got)
	}
}

func TestBadge_Escaping(t *testing.T) {
	got := Badge(`<b>x</b>`, "danger")
	if strings.Contains(got, "<b>") {
		t.Errorf("badge did not escape text: %s", got)
	}
}
