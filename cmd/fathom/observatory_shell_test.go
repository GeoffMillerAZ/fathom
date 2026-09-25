package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestObservatoryUsesSharedAccessibleShell(t *testing.T) {
	model := syntheticModel()
	model.DegradedNote = "Fixture read unavailable; counts are incomplete."
	body := `<section id="fixture-evidence">Unknown is not zero.</section>`
	for _, active := range []string{"home", "fleet", "wiring", "impact", "story", "brief", "statusz"} {
		got := obsPage("Fixture view", active, body, model)
		for _, want := range []string{`<html lang="en">`, `href="#observatory-main"`, `<main id="observatory-main" tabindex="-1"`, `aria-label="Primary"`, `aria-label="Primary (mobile)"`, "Menu", body, model.DegradedNote} {
			if !strings.Contains(got, want) {
				t.Errorf("%s missing %q", active, want)
			}
		}
		if strings.Count(got, "<main ") != 1 || strings.Count(got, "<h1 ") != 1 {
			t.Errorf("%s must have one main landmark and one page heading", active)
		}
		if strings.Count(got, `href="#observatory-main"`) != 1 || strings.Count(got, `id="observatory-main"`) != 1 || strings.Contains(got, "observatory-content") {
			t.Errorf("%s must have one skip link and one explicit focus target", active)
		}
		// The shared shell renders the same nav in desktop and mobile
		// regions; each marks exactly one current destination. (The
		// breadcrumb trail marks the current page too, in its own region.)
		for _, region := range []string{`aria-label="Primary"`, `aria-label="Primary (mobile)"`} {
			if n := strings.Count(navRegion(got, region), `aria-current="page"`); n != 1 {
				t.Errorf("%s must mark one current destination in the %s navigation region, got %d", active, region, n)
			}
		}
		if !strings.Contains(got, `aria-label="Breadcrumb"`) {
			t.Errorf("%s must carry the breadcrumb trail", active)
		}
	}
}

func TestObservatoryNavigationRoutesAndUnknownActive(t *testing.T) {
	// The registry's navigable pages, at their console routes (OC-3/OC-4);
	// legacy active keys ("story") resolve through the registry's aliases.
	want := []struct {
		key   string
		label string
		href  string
	}{
		{key: "home", label: "Home", href: "/"},
		{key: "fleet", label: "Fleet", href: "/fleet"},
		{key: "wiring", label: "Wiring", href: "/wiring"},
		{key: "impact", label: "Impact", href: "/impact"},
		{key: "story", label: "Sessions", href: "/sessions"},
		{key: "brief", label: "Brief", href: "/brief"},
		{key: "statusz", label: "Statusz", href: "/statusz"},
	}

	nav := obsNavigation("unknown")
	if strings.Contains(nav, `aria-current="page"`) {
		t.Fatal("an unknown active key must not highlight any destination")
	}
	for _, item := range want {
		anchor := regexp.QuoteMeta(`<a href="`+item.href+`"`) + `[^>]*><span[^>]*>` +
			regexp.QuoteMeta(item.label+`</span></a>`)
		if !regexp.MustCompile(anchor).MatchString(nav) {
			t.Errorf("navigation does not bind label %q to exact route %q", item.label, item.href)
		}
		activeNav := obsNavigation(item.key)
		activeAnchor := regexp.QuoteMeta(`<a href="`+item.href+`"`) + `[^>]*aria-current="page"`
		if strings.Count(activeNav, `aria-current="page"`) != 1 || !regexp.MustCompile(activeAnchor).MatchString(activeNav) {
			t.Errorf("active key %q must select only %q", item.key, item.href)
		}
	}
}

// navRegion returns the markup of the <nav> whose attributes contain label.
func navRegion(doc, label string) string {
	i := strings.Index(doc, label)
	if i < 0 {
		return ""
	}
	start := strings.LastIndex(doc[:i], "<nav")
	end := strings.Index(doc[i:], "</nav>")
	if start < 0 || end < 0 {
		return ""
	}
	return doc[start : i+end]
}
