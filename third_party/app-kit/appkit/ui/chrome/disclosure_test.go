package chrome

import (
	"strings"
	"testing"
)

func TestDisclosure(t *testing.T) {
	got := RenderDisclosure(Disclosure{Summary: "Show 3 more elements", Body: "<p>hidden</p>", Open: true})
	for _, want := range []string{
		"<details", " open", "<summary", "Show 3 more elements", "<p>hidden</p>",
		"group-open:rotate-180", // chevron animates open/closed, CSS-only
	} {
		if !strings.Contains(got, want) {
			t.Errorf("disclosure missing %q in:\n%s", want, got)
		}
	}
	// Closed by default (no open attr).
	if strings.Contains(RenderDisclosure(Disclosure{Summary: "x"}), " open>") {
		t.Error("disclosure should be closed when Open is false")
	}
}

func TestDisclosure_Escaping(t *testing.T) {
	got := RenderDisclosure(Disclosure{Summary: `<script>alert(1)</script>`})
	if strings.Contains(got, "<script>alert(1)</script>") {
		t.Errorf("disclosure did not escape summary: %s", got)
	}
}

func TestDescriptionList(t *testing.T) {
	got := RenderDescriptionList([]DescItem{
		{Term: "Value", Detail: "Security deposit of $2,400"},
		{Term: "Tier", Detail: "documentary · 92%"},
	})
	for _, want := range []string{
		"<dl", "<dt", "Value", "<dd", "Security deposit of $2,400", "Tier", "documentary",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("description list missing %q: %s", want, got)
		}
	}
	if !strings.Contains(RenderDescriptionList(nil), "<dl") {
		t.Error("empty description list should still render a <dl>")
	}
}

func TestDescriptionList_Escaping(t *testing.T) {
	got := RenderDescriptionList([]DescItem{{Term: `<b>`, Detail: `<script>x</script>`}})
	if strings.Contains(got, "<script>x</script>") || strings.Contains(got, "<b>") {
		t.Errorf("description list did not escape: %s", got)
	}
}

func TestDescriptionListCard(t *testing.T) {
	got := RenderDescriptionListCard([]DescItem{
		{Term: "Kind", Detail: "person"},
		{Term: "Roles", DetailHTML: `<span class="badge">Plaintiff</span>`},
	}, DescListOptions{Title: "Participant", Subtitle: "Case actor details"})
	for _, want := range []string{
		"rounded-lg", "border-line", "bg-surface-raised", "divide-y", "divide-line",
		">Participant<", "Case actor details", "Kind", "person",
		"Roles", `<span class="badge">Plaintiff</span>`, "sm:grid-cols-3",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("desc-list card missing %q:\n%s", want, got)
		}
	}
	// Plain detail is escaped; rich DetailHTML is not.
	esc := RenderDescriptionListCard([]DescItem{{Term: "x", Detail: "<i>plain</i>"}}, DescListOptions{})
	if strings.Contains(esc, "<i>plain</i>") {
		t.Error("plain Detail must be escaped in the card")
	}
}
