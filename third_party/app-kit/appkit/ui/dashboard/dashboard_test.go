package dashboard

import (
	"html/template"
	"strings"
	"testing"
)

func TestRenderShell_StructureAndSlots(t *testing.T) {
	got := RenderShell(Shell{
		Title:   "Overview",
		Nav:     template.HTML(`<a href="/x">Home</a>`),
		Actions: template.HTML(`<button>New</button>`),
		Content: template.HTML(`<div id="body">hi</div>`),
	})
	for _, want := range []string{
		`<aside`, `md:flex`, // sidebar, desktop-only
		`<header`, "Overview", // top bar + title
		`<a href="/x">Home</a>`, // nav slot
		`<button>New</button>`,  // actions slot
		`<main`, `id="body"`,    // content
		`md:hidden`,                                    // mobile nav disclosure
		"border-line", "bg-surface-raised", "text-ink", // tokens, no hex
	} {
		if !strings.Contains(got, want) {
			t.Errorf("shell missing %q; got:\n%s", want, got)
		}
	}
}

func TestRenderShell_TitleEscapedBrandFallback(t *testing.T) {
	got := RenderShell(Shell{Title: `<x>`})
	if strings.Contains(got, "<x>") || !strings.Contains(got, "&lt;x&gt;") {
		t.Errorf("title must be escaped; got:\n%s", got)
	}
	// No nav → no mobile disclosure.
	if strings.Contains(got, "md:hidden") {
		t.Errorf("no nav should mean no mobile menu; got:\n%s", got)
	}
}

func TestRenderWidget_TitleActionBody(t *testing.T) {
	got := RenderWidget(Widget{Title: "Revenue", Action: template.HTML(`<a>more</a>`), Body: template.HTML(`<svg></svg>`), Span: 2})
	for _, want := range []string{"Revenue", "<a>more</a>", "<svg></svg>", "lg:col-span-2", "border-line"} {
		if !strings.Contains(got, want) {
			t.Errorf("widget missing %q; got:\n%s", want, got)
		}
	}
}

func TestRenderWidgetGrid_ColsAndContent(t *testing.T) {
	got := RenderWidgetGrid([]string{RenderWidget(Widget{Title: "A", Body: "1"}), RenderWidget(Widget{Title: "B", Body: "2"})}, WidgetGridOptions{Cols: 4, AriaLabel: "kpis"})
	if !strings.Contains(got, "lg:grid-cols-4") || !strings.Contains(got, "sm:grid-cols-2") {
		t.Errorf("grid cols classes missing; got:\n%s", got)
	}
	if !strings.Contains(got, "A") || !strings.Contains(got, "B") || !strings.Contains(got, `aria-label="kpis"`) {
		t.Errorf("grid content/aria missing; got:\n%s", got)
	}
}

func TestGridAndSpanClasses(t *testing.T) {
	if gridColsClass(0) != "lg:grid-cols-3" || gridColsClass(2) != "lg:grid-cols-2" {
		t.Error("gridColsClass defaults wrong")
	}
	if spanClass(0) != "lg:col-span-1" || spanClass(4) != "lg:col-span-4" {
		t.Error("spanClass defaults wrong")
	}
}
