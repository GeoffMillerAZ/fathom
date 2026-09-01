package card

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// render executes a templ.Component to a string for assertion.
func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render error: %v", err)
	}
	return buf.String()
}

func TestCard_EmptySlots(t *testing.T) {
	got := render(t, Card(Params{}))
	if !strings.Contains(got, "bg-surface-raised") {
		t.Errorf("default card missing surface-raised token class; got:\n%s", got)
	}
	if !strings.Contains(got, "rounded-lg") {
		t.Errorf("card missing rounded-lg class; got:\n%s", got)
	}
	if !strings.Contains(got, "border-line") {
		t.Errorf("default card missing border-line token class; got:\n%s", got)
	}
	// No header/footer sections when slots are nil.
	if strings.Contains(got, "border-b") {
		t.Errorf("card with nil header should not emit border-b; got:\n%s", got)
	}
	if strings.Contains(got, "border-t") {
		t.Errorf("card with nil footer should not emit border-t; got:\n%s", got)
	}
}

func TestCard_WithHeader(t *testing.T) {
	got := render(t, Card(Params{Header: bodyText("The Header")}))
	if !strings.Contains(got, "border-b") {
		t.Errorf("card with header should emit border-b separator; got:\n%s", got)
	}
	if !strings.Contains(got, "The Header") {
		t.Errorf("card header content missing; got:\n%s", got)
	}
}

func TestCard_WithFooter(t *testing.T) {
	got := render(t, Card(Params{Footer: bodyText("The Footer")}))
	if !strings.Contains(got, "border-t") {
		t.Errorf("card with footer should emit border-t separator; got:\n%s", got)
	}
	if !strings.Contains(got, "The Footer") {
		t.Errorf("card footer content missing; got:\n%s", got)
	}
}

func TestCard_WithAllSlots(t *testing.T) {
	got := render(t, Card(Params{
		Header: bodyText("H"),
		Body:   bodyText("B"),
		Footer: bodyText("F"),
	}))
	for _, want := range []string{"H", "B", "F", "border-b", "border-t"} {
		if !strings.Contains(got, want) {
			t.Errorf("all-slot card missing %q; got:\n%s", want, got)
		}
	}
}

func TestCard_Elevated(t *testing.T) {
	got := render(t, Card(Params{Elevated: true}))
	if !strings.Contains(got, "bg-surface-overlay") {
		t.Errorf("elevated card missing surface-overlay token class; got:\n%s", got)
	}
	if strings.Contains(got, "bg-surface-raised") {
		t.Errorf("elevated card should not have surface-raised; got:\n%s", got)
	}
}

func TestCard_NoBorder(t *testing.T) {
	got := render(t, Card(Params{NoBorder: true}))
	if strings.Contains(got, "border-line") {
		t.Errorf("no-border card should not have border-line class; got:\n%s", got)
	}
}

func TestCard_ExtraClass(t *testing.T) {
	got := render(t, Card(Params{ExtraClass: "shadow-md"}))
	if !strings.Contains(got, "shadow-md") {
		t.Errorf("extra class missing; got:\n%s", got)
	}
}

func TestSimpleCard_Content(t *testing.T) {
	got := render(t, SimpleCard("My Title", "My subtitle"))
	if !strings.Contains(got, "My Title") {
		t.Errorf("SimpleCard missing title; got:\n%s", got)
	}
	if !strings.Contains(got, "My subtitle") {
		t.Errorf("SimpleCard missing subtitle; got:\n%s", got)
	}
	if !strings.Contains(got, "text-ink-muted") {
		t.Errorf("SimpleCard subtitle missing text-ink-muted token class; got:\n%s", got)
	}
}

func TestSimpleCard_EmptySubtitle(t *testing.T) {
	got := render(t, SimpleCard("Title only", ""))
	if strings.Contains(got, "text-ink-muted") {
		t.Errorf("SimpleCard with empty subtitle should not emit subtitle element; got:\n%s", got)
	}
}

func TestCard_NoHexColors(t *testing.T) {
	// Regression: ensure no hardcoded hex color values appear in the output.
	got := render(t, Card(Params{
		Header: bodyText("H"),
		Body:   bodyText("B"),
		Footer: bodyText("F"),
	}))
	if strings.Contains(got, "#") {
		t.Errorf("card output contains a literal hex color; got:\n%s", got)
	}
}

func TestCardClass_TokenRoles(t *testing.T) {
	// Unit-test the class assembler directly.
	cls := cardClass(Params{})
	for _, token := range []string{"bg-surface-raised", "rounded-lg", "border-line", "text-ink"} {
		if !strings.Contains(cls, token) {
			t.Errorf("cardClass missing token role %q; got: %q", token, cls)
		}
	}
}
