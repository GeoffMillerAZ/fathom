package main

import (
	"html"
	"regexp"
	"strings"
	"testing"
)

func TestObservatoryExplorationAbsenceExplainsNextEvidence(t *testing.T) {
	for _, note := range []string{"", "   ", "No paired sessions: source <unavailable> & incomplete."} {
		m := syntheticModel()
		m.Impact.ExpNote = note
		got := renderObsImpact(m)
		for _, want := range []string{`role="note"`, "No comparable exploration sample", `href="/sessions"`, "Inspect session evidence"} {
			if !strings.Contains(got, want) {
				t.Errorf("note %q: missing %q", note, want)
			}
		}
		if strings.TrimSpace(note) == "" {
			if !strings.Contains(got, "not evidence of zero cost or savings") {
				t.Error("a missing reason must explain the evidence limit")
			}
		} else if strings.Count(got, html.EscapeString(note)) != 1 || strings.Contains(got, "source <unavailable>") {
			t.Error("the source reason must be escaped and shown once, not replaced or duplicated")
		}
	}
}

func TestObservatoryExplorationMeasuredBranchPreserved(t *testing.T) {
	m := syntheticModel()
	m.Impact.ExpSessions = 2
	m.Impact.ExpRawCalls, m.Impact.ExpFathomCalls = 4, 2
	m.Impact.ExpRawTokens, m.Impact.ExpFathomTokens = 400, 20
	m.Impact.ExpNote = "Fixture estimate, not counterfactual savings."
	got := renderObsImpact(m)
	for _, want := range []string{"~100 tok", "~10 tok", "raw exploration, per question", "indexed discovery, per question", m.Impact.ExpNote} {
		if !strings.Contains(got, want) {
			t.Errorf("measured comparison missing %q", want)
		}
	}
	if strings.Contains(got, "No comparable exploration sample") {
		t.Error("a comparable sample must not receive the missing-sample state")
	}
}

func TestObservatoryMissingConfigurationIsNotEmptyOrHealthy(t *testing.T) {
	m := syntheticModel()
	got, ok := renderObsService(m, "alpha")
	if !ok {
		t.Fatal("fixture service missing")
	}
	for _, want := range []string{"Configuration snapshot unavailable", "This snapshot does not contain configuration evidence", `role="note"`} {
		if !strings.Contains(got, want) {
			t.Errorf("nil configuration missing %q", want)
		}
	}
	if strings.Contains(got, "no semantic layers installed") || strings.Contains(got, "0 pre-edit warnings") {
		t.Error("nil configuration cannot establish no installed layers or zero guards")
	}
	m.Services[0].Config = &obsConfigPanel{GuardCount: 6, LayersNote: "layers fixture", BindingNote: "binding fixture"}
	got, _ = renderObsService(m, "alpha")
	for _, want := range []string{"6 pre-edit warnings configured", "layers fixture", "binding fixture"} {
		if !strings.Contains(got, want) {
			t.Errorf("present configuration missing %q", want)
		}
	}
	if strings.Contains(got, "Configuration snapshot unavailable") {
		t.Error("present configuration must retain its own observed state")
	}
}

func TestObservatoryAgentShareUsesReadableProvenance(t *testing.T) {
	m := syntheticModel()
	got, _ := renderObsService(m, "alpha")
	for _, want := range []string{`id="service-agent-share"`, `data-viz-source="derived"`, "7 of 10 recorded calls", "current local call-log snapshot", m.GeneratedAt} {
		if !strings.Contains(got, want) {
			t.Errorf("agent share missing %q", want)
		}
	}
	// The full explanation belongs in wrapping HTML, not a fixed SVG text run.
	const caption = "of this service&#39;s recorded calls arrived over the agent surface"
	if !strings.Contains(got, caption) {
		t.Error("share meaning must not be shortened away")
	}
	svg := regexp.MustCompile(`(?s)<svg\b[^>]*aria-label="agent-surface share"[^>]*>.*?</svg>`).FindString(got)
	if svg == "" || !strings.Contains(svg, "70%") {
		t.Error("the measured gauge and its percentage must remain")
	}
	if strings.Contains(svg, caption) {
		t.Error("the full explanation must not remain inside the clip-prone SVG")
	}
	m.CallLogNote = "call log unavailable"
	got, _ = renderObsService(m, "alpha")
	if strings.Contains(got, `id="service-agent-share"`) || !strings.Contains(got, m.CallLogNote) {
		t.Error("unreadable call data must not produce a measured share figure")
	}
}
