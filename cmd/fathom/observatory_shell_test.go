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
		// The shared shell renders the same nav in desktop and mobile regions.
		if strings.Count(got, `aria-current="page"`) != 2 {
			t.Errorf("%s must mark one current destination in each navigation region", active)
		}
	}
}

func TestObservatoryNavigationRoutesAndUnknownActive(t *testing.T) {
	want := []struct {
		label string
		href  string
	}{
		{label: "home", href: "/"},
		{label: "fleet", href: "/observatory"},
		{label: "wiring", href: "/observatory/wiring/"},
		{label: "impact", href: "/observatory/impact"},
		{label: "story", href: "/observatory/story"},
		{label: "brief", href: "/observatory/brief"},
		{label: "statusz", href: "/statusz"},
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
		activeNav := obsNavigation(item.label)
		activeAnchor := regexp.QuoteMeta(`<a href="`+item.href+`"`) + `[^>]*aria-current="page"`
		if strings.Count(activeNav, `aria-current="page"`) != 1 || !regexp.MustCompile(activeAnchor).MatchString(activeNav) {
			t.Errorf("active key %q must select only %q", item.label, item.href)
		}
	}
}
