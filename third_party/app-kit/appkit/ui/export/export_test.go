package export

import (
	"strings"
	"testing"
)

func TestToolbar_DefaultsBothButtons(t *testing.T) {
	got := Toolbar(Options{})
	for _, want := range []string{
		"<script>", "</script>",
		"⤓ Save as PDF", "⤓ Save HTML",
		"print-color-adjust:exact",  // dark themes survive the PDF
		"@page{size:",               // single content-sized page
		`var AC="#4fd1c9"`,          // default accent
		"var AC=\"#4fd1c9\",W=1160", // default width
	} {
		if !strings.Contains(got, want) {
			t.Errorf("default toolbar missing %q", want)
		}
	}
}

func TestToolbar_OnlyPDF(t *testing.T) {
	got := Toolbar(Options{PDF: true})
	if !strings.Contains(got, "savePDF)") {
		t.Errorf("PDF button should be wired")
	}
	if strings.Contains(got, "Save HTML") {
		t.Errorf("HTML button should be absent when only PDF requested")
	}
}

func TestToolbar_OnlyHTML(t *testing.T) {
	got := Toolbar(Options{HTML: true})
	if !strings.Contains(got, "saveHTML)") {
		t.Errorf("HTML button should be wired")
	}
	if strings.Contains(got, "Save as PDF") {
		t.Errorf("PDF button should be absent when only HTML requested")
	}
}

func TestToolbar_CustomAccentAndWidth(t *testing.T) {
	got := Toolbar(Options{Accent: "#ff7a8a", PageWidthPx: 980})
	if !strings.Contains(got, `var AC="#ff7a8a"`) {
		t.Errorf("custom accent not applied: %s", got)
	}
	if !strings.Contains(got, "W=980") {
		t.Errorf("custom width not applied")
	}
}

// A non-hex accent must NOT reach the output (no JS injection) and must fall
// back to the default — the security-relevant case.
func TestToolbar_AccentInjectionRejected(t *testing.T) {
	got := Toolbar(Options{Accent: "red;}</script><script>alert(1)//"})
	if strings.Contains(got, "alert(1)") || strings.Contains(got, "</script><script>") {
		t.Errorf("malicious accent leaked into output: %s", got)
	}
	if !strings.Contains(got, `var AC="#4fd1c9"`) {
		t.Errorf("invalid accent should fall back to default")
	}
}

func TestToolbar_FileNameSlugified(t *testing.T) {
	got := Toolbar(Options{HTML: true, FileName: "Platform Overview!!"})
	if !strings.Contains(got, `a.download="platform-overview"+'.html'`) {
		t.Errorf("filename not slugified to a safe literal: %s", got)
	}
	// default (no FileName) derives from document.title at runtime
	def := Toolbar(Options{HTML: true})
	if !strings.Contains(def, "document.title") {
		t.Errorf("default download name should derive from document.title")
	}
}

func TestToolbar_EmptyOptionsEnablesBoth(t *testing.T) {
	// neither flag set => both buttons (an empty toolbar is never intended)
	got := Toolbar(Options{})
	if !strings.Contains(got, "Save as PDF") || !strings.Contains(got, "Save HTML") {
		t.Errorf("zero Options should enable both buttons")
	}
}

func TestToolbar_Deterministic(t *testing.T) {
	o := Options{PDF: true, HTML: true, Accent: "#a594ff", PageWidthPx: 1200}
	if Toolbar(o) != Toolbar(o) {
		t.Errorf("toolbar output is non-deterministic")
	}
}

func TestToolbar_SelfContainedNoTailwind(t *testing.T) {
	// The toolbar must inject its own inline styles, not depend on kit role
	// utilities, so it works in a standalone (non-Tailwind) doc.
	got := Toolbar(Options{})
	if !strings.Contains(got, ".__bar{position:fixed") {
		t.Errorf("toolbar should inject its own inline styles")
	}
	if strings.Contains(got, "bg-surface") || strings.Contains(got, "text-ink") {
		t.Errorf("toolbar must not rely on Tailwind role utilities")
	}
}
