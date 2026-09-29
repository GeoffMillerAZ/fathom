package chrome

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestEmptyState(t *testing.T) {
	got := RenderEmptyState(EmptyState{Heading: "No sources", Body: "Register one.", Action: `<a>Add</a>`, Icon: `<svg></svg>`})
	for _, want := range []string{"No sources", "Register one.", "<a>Add</a>", "<svg></svg>", "text-center"} {
		if !strings.Contains(got, want) {
			t.Errorf("empty state missing %q", want)
		}
	}
}

func TestEmptyState_Escaping(t *testing.T) {
	got := RenderEmptyState(EmptyState{Heading: `<script>`, Body: `<img src=x>`})
	for _, bad := range []string{"<script>", "<img src=x>"} {
		if strings.Contains(got, bad) {
			t.Errorf("empty state did not escape %q", bad)
		}
	}
}

func TestBreadcrumbs(t *testing.T) {
	got := Breadcrumbs([]Crumb{{Label: "Cases", Href: "/cases"}, {Label: "Doe v. Roe", Href: "/cases/1"}, {Label: "Timeline"}})
	if !strings.Contains(got, `href="/cases"`) || !strings.Contains(got, `aria-current="page"`) {
		t.Errorf("breadcrumbs structure wrong: %s", got)
	}
	// Last crumb is current (no link).
	if strings.Contains(got, `>Timeline</a>`) {
		t.Errorf("last crumb should not be a link")
	}
	if Breadcrumbs(nil) != "" {
		t.Errorf("empty breadcrumbs should be empty string")
	}
}

// FATHOM-F0-KIT-03: a crumb link is a navigation control, so it gets the kit's
// 44px minimum target and a visible keyboard focus outline (the bare link
// measured 17px high with no focus treatment in Fathom's console). The current
// page stays plain text; the separator stays decorative.
func TestBreadcrumbs_LinksHaveTargetSizeAndVisibleFocus(t *testing.T) {
	got := Breadcrumbs([]Crumb{{Label: "A", Href: "/a"}, {Label: "Cases", Href: "/cases"}, {Label: "Timeline"}})
	links := regexp.MustCompile(`<a [^>]*>`).FindAllString(got, -1)
	if len(links) != 2 {
		t.Fatalf("want 2 crumb links, got %d: %s", len(links), got)
	}
	for _, link := range links {
		class := regexp.MustCompile(`class="([^"]*)"`).FindStringSubmatch(link)
		if class == nil {
			t.Fatalf("crumb link has no class: %s", link)
		}
		fields := strings.Fields(class[1])
		for _, want := range []string{
			"inline-flex", "items-center", "min-h-11", "min-w-11",
			"focus-visible:outline-2", "focus-visible:outline-offset-2", "focus-visible:outline-focus",
		} {
			if !slices.Contains(fields, want) {
				t.Errorf("crumb link %s lacks %q", link, want)
			}
		}
		// Tailwind v4: focus:outline-none would cancel the focus-visible ring.
		for _, bad := range []string{"outline-none", "focus:outline-none"} {
			if slices.Contains(fields, bad) {
				t.Errorf("crumb link %s must not carry %q", link, bad)
			}
		}
	}
	if !strings.Contains(got, `<li class="text-ink" aria-current="page">Timeline</li>`) {
		t.Errorf("current crumb must stay plain text: %s", got)
	}
}

func TestBreadcrumbs_Escaping(t *testing.T) {
	got := Breadcrumbs([]Crumb{{Label: `<script>`, Href: `"x`}})
	if strings.Contains(got, "<script>") {
		t.Errorf("breadcrumbs did not escape label")
	}
}

func TestPagination(t *testing.T) {
	got := RenderPagination(Pagination{Page: 5, TotalPages: 20})
	for _, want := range []string{"Prev", "Next", "?page=4", "?page=6", `aria-current="page"`, "…"} {
		if !strings.Contains(got, want) {
			t.Errorf("pagination missing %q: %s", want, got)
		}
	}
	// Single page → nothing.
	if RenderPagination(Pagination{Page: 1, TotalPages: 1}) != "" {
		t.Errorf("single-page pagination should be empty")
	}
}

// FATHOM-F0-KIT-05: pagination links were px-2 py-1, about 28px high with only the browser's
// focus ring — the gap FATHOM-F0-KIT-03 left open after fixing breadcrumbs,
// which Fathom's console wrapped with [&_a]:min-h-11 on every pager. Every page
// link, Prev and Next is now a 44px target with the kit's focus-visible
// outline; the current page and the disabled ends keep the same height, and
// the row wraps rather than overflowing a phone.
func TestPagination_ControlsHaveTargetSizeAndVisibleFocus(t *testing.T) {
	classOf := func(tag string) []string {
		m := regexp.MustCompile(`class="([^"]*)"`).FindStringSubmatch(tag)
		if m == nil {
			t.Fatalf("no class on %s", tag)
		}
		return strings.Fields(m[1])
	}
	for _, p := range []Pagination{{Page: 5, TotalPages: 20}, {Page: 1, TotalPages: 3}, {Page: 3, TotalPages: 3}} {
		got := RenderPagination(p)
		nav := regexp.MustCompile(`<nav [^>]*>`).FindString(got)
		if !slices.Contains(classOf(nav), "flex-wrap") {
			t.Errorf("page %d/%d: the pager row does not wrap: %s", p.Page, p.TotalPages, nav)
		}
		links := regexp.MustCompile(`<a [^>]*>`).FindAllString(got, -1)
		if len(links) == 0 {
			t.Fatalf("page %d/%d: no links: %s", p.Page, p.TotalPages, got)
		}
		for _, link := range links {
			fields := classOf(link)
			for _, want := range []string{
				"inline-flex", "items-center", "justify-center", "min-h-11", "min-w-11",
				"focus-visible:outline-2", "focus-visible:outline-offset-2", "focus-visible:outline-focus",
			} {
				if !slices.Contains(fields, want) {
					t.Errorf("pager link %s lacks %q", link, want)
				}
			}
			for _, bad := range []string{"py-1", "outline-none", "focus:outline-none"} {
				if slices.Contains(fields, bad) {
					t.Errorf("pager link %s must not carry %q", link, bad)
				}
			}
		}
		// The current page and a disabled end are not targets, but share
		// the row's height so the row does not jump between pages.
		for _, span := range regexp.MustCompile(`<span (?:aria-current|aria-disabled)[^>]*>`).FindAllString(got, -1) {
			if !slices.Contains(classOf(span), "min-h-11") {
				t.Errorf("pager span %s is shorter than the links", span)
			}
		}
	}
}

func TestPagination_EdgesDisabled(t *testing.T) {
	first := RenderPagination(Pagination{Page: 1, TotalPages: 3})
	if !strings.Contains(first, `aria-disabled="true"`) || strings.Contains(first, `rel="prev"`) {
		t.Errorf("page 1 should disable Prev")
	}
	last := RenderPagination(Pagination{Page: 3, TotalPages: 3})
	if strings.Contains(last, `rel="next"`) {
		t.Errorf("last page should disable Next")
	}
}

func TestPagination_CustomHref(t *testing.T) {
	got := RenderPagination(Pagination{Page: 2, TotalPages: 4, HrefFor: func(n int) string { return "/x/p/" + itoa(n) }})
	if !strings.Contains(got, "/x/p/3") {
		t.Errorf("custom HrefFor not used: %s", got)
	}
}

func TestPagination_OutOfRangeClamps(t *testing.T) {
	got := RenderPagination(Pagination{Page: 99, TotalPages: 5})
	if strings.Contains(got, "NaN") || got == "" {
		t.Errorf("out-of-range page should clamp and render")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	if neg {
		d = append([]byte{'-'}, d...)
	}
	return string(d)
}
