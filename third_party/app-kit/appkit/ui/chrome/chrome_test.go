package chrome

import (
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
