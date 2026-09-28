package main

import (
	"fmt"
	"html"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"
)

func TestObservatoryObservationAbsenceUsesReadableRole(t *testing.T) {
	for _, note := range []string{"", "registry unreadable"} {
		got := fleetOpenObservations(observationCoverageModel(nil, note))
		if got.ValueRole != "ink-muted" {
			t.Fatalf("observation absence role = %q; browser contrast target requires ink-muted", got.ValueRole)
		}
	}
}

func TestObservatoryObservationCoverageAcrossViews(t *testing.T) {
	zero, two, three := 0, 2, 3
	tests := []struct {
		name     string
		services []obsServiceRow
		value    string
		sub      string
	}{
		{
			name:  "no services is absent",
			value: "—",
			sub:   "no observation counts available",
		},
		{
			name: "all unknown is absent",
			services: []obsServiceRow{
				observationService("alpha", nil),
				observationService("beta", nil),
			},
			value: "—",
			sub:   "no observation counts available",
		},
		{
			name: "partial known zero is a lower bound",
			services: []obsServiceRow{
				observationService("alpha", &zero),
				observationService("beta", nil),
			},
			value: "≥ 0",
			sub:   "1 of 2 repos reporting; lower bound",
		},
		{
			name: "partial known positive is a lower bound",
			services: []obsServiceRow{
				observationService("alpha", &three),
				observationService("beta", nil),
			},
			value: "≥ 3",
			sub:   "1 of 2 repos reporting; lower bound",
		},
		{
			name: "complete measured zero is exact",
			services: []obsServiceRow{
				observationService("alpha", &zero),
				observationService("beta", &zero),
			},
			value: "0",
			sub:   "2 of 2 repos reporting",
		},
		{
			name: "complete positive is exact",
			services: []obsServiceRow{
				observationService("alpha", &two),
				observationService("beta", &three),
			},
			value: "5",
			sub:   "2 of 2 repos reporting",
		},
	}

	for _, tc := range tests {
		for _, renderer := range observationCoverageRenderers() {
			t.Run(tc.name+"/"+renderer.name, func(t *testing.T) {
				m := observationCoverageModel(tc.services, "")
				assertRenderedObservationCoverage(t, renderer.render(m), renderer, tc.value, tc.sub)
			})
		}
		t.Run(tc.name+"/landing", func(t *testing.T) {
			m := observationCoverageModel(tc.services, "")
			assertLandingQuestion(t, renderObsLanding(m), "Q3", "where does the agent still flounder?",
				tc.value+" open observations", tc.sub+"; plus per-tool miss rates from 0 logged calls")
		})
	}
}

func TestObservatoryObservationCoverageLandingCallLogContext(t *testing.T) {
	tests := []struct {
		name        string
		model       observatoryModel
		wantReason  string
		wantZeroLog bool
	}{
		{
			name: "unreadable call log is not zero",
			model: func() observatoryModel {
				m := observationCoverageModel([]obsServiceRow{observationService("alpha", nil)}, "")
				m.CallLogNote = `<call log> & unreadable`
				return m
			}(),
			wantReason: `<call log> & unreadable`,
		},
		{
			name:       "degraded registry is not zero",
			model:      observationCoverageModel(nil, `<registry> & unreadable`),
			wantReason: `<registry> & unreadable`,
		},
		{
			name: "both sources keep their own diagnostic",
			model: func() observatoryModel {
				m := observationCoverageModel(nil, `<registry> & unreadable`)
				m.CallLogNote = `<call log> & unreadable`
				return m
			}(),
			wantReason: `<call log> & unreadable`,
		},
		{
			name:        "readable zero remains measured zero",
			model:       observationCoverageModel([]obsServiceRow{observationService("alpha", nil)}, ""),
			wantZeroLog: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			widget := landingQuestionWidget(t, renderObsLanding(tc.model), "Q3", "where does the agent still flounder?")
			if got := strings.Contains(widget, "0 logged calls"); got != tc.wantZeroLog {
				t.Errorf("Q3 zero logged calls present = %t, want %t: %s", got, tc.wantZeroLog, widget)
			}
			if tc.wantReason != "" {
				if strings.Contains(widget, tc.wantReason) || !strings.Contains(widget, html.EscapeString(tc.wantReason)) {
					t.Errorf("Q3 did not render the source diagnostic escaped: %s", widget)
				}
			}
		})
	}
}

func TestObservatoryObservationCoverageLandingVerifyCatches(t *testing.T) {
	zero, seven := 0, 7
	tests := []struct {
		name  string
		value *int
		note  string
		want  string
	}{
		{name: "unmeasured is absent", note: `<verify ledger> & unreadable`, want: "— verify catches"},
		{name: "measured zero is exact", value: &zero, note: "verify ran in 2 sessions", want: "0 verify catches"},
		{name: "measured positive is preserved", value: &seven, note: "verify ran in 9 sessions", want: "7 verify catches"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := observationCoverageModel(nil, "")
			m.Impact.VerifyCatchesTotal = tc.value
			m.Impact.VerifyCatchesNote = tc.note
			body := renderObsLanding(m)
			assertLandingQuestion(t, body, "Q2", "what did fathom prevent?", tc.want, tc.note)
			widget := landingQuestionWidget(t, body, "Q2", "what did fathom prevent?")
			escaped := html.EscapeString(tc.note)
			if escaped != tc.note && (strings.Contains(widget, tc.note) || !strings.Contains(widget, escaped)) {
				t.Errorf("Q2 note was not rendered escaped: %s", widget)
			}
		})
	}
}

func TestObservatoryObservationCoverageDegradedRegistryIsNeverExact(t *testing.T) {
	two, three := 2, 3
	t.Run("known rows remain a lower bound", func(t *testing.T) {
		m := observationCoverageModel([]obsServiceRow{
			observationService("alpha", &two),
			observationService("beta", &three),
		}, "registry unreadable")
		for _, renderer := range observationCoverageRenderers() {
			t.Run(renderer.name, func(t *testing.T) {
				assertRenderedObservationCoverage(t, renderer.render(m), renderer, "≥ 5",
					"2 of 2 listed repos reporting; lower bound; fleet coverage unavailable: registry unreadable")
			})
		}
	})

	t.Run("no known rows is absent", func(t *testing.T) {
		m := observationCoverageModel(nil, "registry unreadable")
		for _, renderer := range observationCoverageRenderers() {
			t.Run(renderer.name, func(t *testing.T) {
				assertRenderedObservationCoverage(t, renderer.render(m), renderer, "—",
					"registry unreadable")
			})
		}
	})

	t.Run("diagnostic is escaped", func(t *testing.T) {
		const diagnostic = `<script>alert("coverage")</script> & unreadable`
		m := observationCoverageModel(nil, diagnostic)
		for _, renderer := range observationCoverageRenderers() {
			t.Run(renderer.name, func(t *testing.T) {
				body := renderer.render(m)
				assertRenderedObservationCoverage(t, body, renderer, "—",
					diagnostic)
				if err := checkEscapedDiagnostic(body, diagnostic); err != nil {
					t.Error(err)
				}
			})
		}
	})
}

// These canaries protect the test helper itself. A class-string update may
// change the renderer's wrapper utilities, but it must not make the assertion
// accept a missing aggregate, a value from another widget, or unsafe text.
func TestObservatoryObservationCoverageAssertionRegressionCanaries(t *testing.T) {
	two := 2
	renderer := observationCoverageRenderers()[0] // fleet aggregate: semantic dt/dd shape
	model := observationCoverageModel([]obsServiceRow{observationService("alpha", &two)}, "")
	body := renderer.render(model)

	if err := checkRenderedObservationCoverage(body, renderer, "2", "1 of 1 repo reporting"); err != nil {
		t.Fatalf("valid aggregate rejected: %v", err)
	}
	if err := checkRenderedObservationCoverage("<section></section>", renderer, "2", "1 of 1 repo reporting"); err == nil {
		t.Fatal("missing aggregate unexpectedly passed")
	}
	wrongValue := strings.Replace(body, ">2</span>", ">99</span>", 1)
	if err := checkRenderedObservationCoverage(wrongValue, renderer, "2", "1 of 1 repo reporting"); err == nil {
		t.Fatal("wrong aggregate value unexpectedly passed")
	}
	decoy := kv("open observations", `<span>2</span> <span class="text-ink-muted">1 of 1 repo reporting</span>`)
	if err := checkRenderedObservationCoverage(decoy, renderer, "2", "1 of 1 repo reporting"); err == nil {
		t.Fatal("per-service wrong-widget decoy unexpectedly passed as the fleet aggregate")
	}
	if err := checkRenderedObservationCoverage(decoy+body, renderer, "2", "1 of 1 repo reporting"); err != nil {
		t.Fatalf("same-label wrong-shape decoy hid the valid aggregate: %v", err)
	}
	if err := checkRenderedObservationCoverage(body+body, renderer, "2", "1 of 1 repo reporting"); err == nil {
		t.Fatal("duplicate valid aggregates unexpectedly passed as unambiguous")
	}
	missingCaption := `<dl><div><dt>open observations</dt><dd><div><span>2</span></div></dd></div></dl>`
	if err := checkRenderedObservationCoverage(missingCaption, renderer, "2", "1 of 1 repo reporting"); err == nil {
		t.Fatal("aggregate with a missing caption unexpectedly passed")
	}

	const diagnostic = `<script>alert("coverage")</script> & unreadable`
	escapedBody := renderer.render(observationCoverageModel(nil, diagnostic))
	if err := checkRenderedObservationCoverage(escapedBody, renderer, "—", diagnostic); err != nil {
		t.Fatalf("escaped diagnostic value/caption rejected: %v", err)
	}
	if err := checkRenderedObservationCoverage(escapedBody, renderer, "—", "wrong reason"); err == nil {
		t.Fatal("wrong absence reason unexpectedly passed")
	}
	if err := checkEscapedDiagnostic(escapedBody, diagnostic); err != nil {
		t.Fatalf("valid escaped diagnostic rejected: %v", err)
	}
	unescapedBody := strings.Replace(escapedBody, html.EscapeString(diagnostic), diagnostic, 1)
	if err := checkEscapedDiagnostic(unescapedBody, diagnostic); err == nil {
		t.Fatal("unescaped diagnostic unexpectedly passed")
	}
}

func TestObservatoryObservationCoverageKVAssertionRegressionCanaries(t *testing.T) {
	two := 2
	renderer := observationCoverageRenderers()[1] // impact aggregate: semantic same-row span shape
	model := observationCoverageModel([]obsServiceRow{observationService("alpha", &two)}, "")
	body := renderer.render(model)
	const caption = "1 of 1 repo reporting"

	if err := checkRenderedObservationCoverage(body, renderer, "2", caption); err != nil {
		t.Fatalf("valid KV aggregate rejected: %v", err)
	}
	wrongShape := `<dl><div><dt>open observations across the fleet</dt><dd><div><span>2</span><span>1 of 1 repo reporting</span></div></dd></div></dl>`
	if err := checkRenderedObservationCoverage(wrongShape, renderer, "2", caption); err == nil {
		t.Fatal("same-label stat-band decoy unexpectedly passed as the KV aggregate")
	}
	if err := checkRenderedObservationCoverage(wrongShape+body, renderer, "2", caption); err != nil {
		t.Fatalf("same-label wrong-shape decoy hid the valid KV aggregate: %v", err)
	}
	missingCaption := `<div><span>open observations across the fleet</span><span><span>2</span></span></div>`
	if err := checkRenderedObservationCoverage(missingCaption, renderer, "2", caption); err == nil {
		t.Fatal("KV aggregate with a missing caption unexpectedly passed")
	}
	if err := checkRenderedObservationCoverage(body+body, renderer, "2", caption); err == nil {
		t.Fatal("duplicate valid KV aggregates unexpectedly passed as unambiguous")
	}
}

type observationCoverageShape uint8

const (
	observationCoverageStatBand observationCoverageShape = iota
	observationCoverageKV
)

type observationCoverageRenderer struct {
	name   string
	label  string
	shape  observationCoverageShape
	render func(observatoryModel) string
}

func observationCoverageRenderers() []observationCoverageRenderer {
	return []observationCoverageRenderer{
		{name: "fleet", label: "open observations", shape: observationCoverageStatBand, render: renderObsFleet},
		{name: "impact", label: "open observations across the fleet", shape: observationCoverageKV, render: renderObsImpact},
		{name: "week story", label: "open observations (fleet snapshot)", shape: observationCoverageStatBand, render: func(m observatoryModel) string {
			return renderWeekStory(weekStory{From: "2026-09-07", To: "2026-09-13"}, m)
		}},
	}
}

func observationService(name string, open *int) obsServiceRow {
	row := obsServiceRow{}
	row.Service = name
	row.OpenObservations = open
	return row
}

func observationCoverageModel(services []obsServiceRow, degradedNote string) observatoryModel {
	return observatoryModel{
		SchemaVersion: observatorySchemaVersion,
		FathomVersion: "test",
		GeneratedAt:   "2026-09-14T12:00:00Z",
		Arm:           obsArmBanner{Detail: "no measurement arm active"},
		Services:      services,
		DegradedNote:  degradedNote,
	}
}

func assertRenderedObservationCoverage(t *testing.T, body string, renderer observationCoverageRenderer, value, sub string) {
	t.Helper()
	if err := checkRenderedObservationCoverage(body, renderer, value, sub); err != nil {
		t.Error(err)
	}
}

func checkRenderedObservationCoverage(body string, renderer observationCoverageRenderer, value, sub string) error {
	doc, err := nethtml.Parse(strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("parse rendered observation coverage HTML: %w", err)
	}
	candidates := observationCoverageCandidates(doc, renderer)
	if len(candidates) == 0 {
		return fmt.Errorf("rendered view has no structurally valid observation coverage %s labeled %q", renderer.name, renderer.label)
	}
	if len(candidates) != 1 {
		return fmt.Errorf("rendered view has %d structurally valid observation coverage %s rows labeled %q; want exactly 1", len(candidates), renderer.name, renderer.label)
	}
	gotValue := strings.ReplaceAll(nodeText(candidates[0].value), "\u00a0", " ")
	if gotValue != value {
		return fmt.Errorf("observation coverage %q value = %q, want %q", renderer.label, gotValue, value)
	}
	gotSub := nodeText(candidates[0].caption)
	if gotSub != sub {
		return fmt.Errorf("observation coverage %q caption = %q, want %q", renderer.label, gotSub, sub)
	}
	return nil
}

type observationCoverageCandidate struct {
	value   *nethtml.Node
	caption *nethtml.Node
}

func observationCoverageCandidates(root *nethtml.Node, renderer observationCoverageRenderer) []observationCoverageCandidate {
	var candidates []observationCoverageCandidate
	walkHTML(root, func(label *nethtml.Node) {
		if nodeText(label) != renderer.label {
			return
		}
		var valueContainer *nethtml.Node
		switch renderer.shape {
		case observationCoverageStatBand:
			if !isElement(label, "dt") {
				return
			}
			dd := nextElementSibling(label)
			if !isElement(dd, "dd") {
				return
			}
			children := elementChildren(dd)
			if len(children) != 1 || !isElement(children[0], "div") {
				return
			}
			valueContainer = children[0]
		case observationCoverageKV:
			if !isElement(label, "span") || !isElement(label.Parent, "div") {
				return
			}
			valueContainer = nextElementSibling(label)
			if !isElement(valueContainer, "span") {
				return
			}
		default:
			return
		}
		spans := elementChildren(valueContainer)
		if len(spans) != 2 || !isElement(spans[0], "span") || !isElement(spans[1], "span") {
			return
		}
		candidates = append(candidates, observationCoverageCandidate{value: spans[0], caption: spans[1]})
	})
	return candidates
}

func walkHTML(node *nethtml.Node, visit func(*nethtml.Node)) {
	if node == nil {
		return
	}
	visit(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkHTML(child, visit)
	}
}

func isElement(node *nethtml.Node, name string) bool {
	return node != nil && node.Type == nethtml.ElementNode && node.Data == name
}

func nextElementSibling(node *nethtml.Node) *nethtml.Node {
	if node == nil {
		return nil
	}
	for sibling := node.NextSibling; sibling != nil; sibling = sibling.NextSibling {
		if sibling.Type == nethtml.ElementNode {
			return sibling
		}
	}
	return nil
}

func elementChildren(node *nethtml.Node) []*nethtml.Node {
	if node == nil {
		return nil
	}
	var children []*nethtml.Node
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == nethtml.ElementNode {
			children = append(children, child)
		}
	}
	return children
}

func nodeText(node *nethtml.Node) string {
	if node == nil {
		return ""
	}
	var b strings.Builder
	var appendText func(*nethtml.Node)
	appendText = func(current *nethtml.Node) {
		if current.Type == nethtml.TextNode {
			b.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			appendText(child)
		}
	}
	appendText(node)
	return b.String()
}

func checkEscapedDiagnostic(body, diagnostic string) error {
	if strings.Contains(body, diagnostic) {
		return fmt.Errorf("observation coverage diagnostic was rendered raw: %s", diagnostic)
	}
	if !strings.Contains(body, html.EscapeString(diagnostic)) {
		return fmt.Errorf("observation coverage diagnostic was not rendered escaped: %s", diagnostic)
	}
	return nil
}

func assertLandingQuestion(t *testing.T, body, number, question, answer, sub string) {
	t.Helper()
	widget := landingQuestionWidget(t, body, number, question)
	gotAnswer, ok := renderedDivText(widget, `<div class="text-xl font-semibold text-ink">`)
	if !ok {
		t.Fatalf("%s landing widget has no answer: %s", number, widget)
	}
	gotAnswer = strings.ReplaceAll(gotAnswer, "\u00a0", " ")
	if gotAnswer != answer {
		t.Errorf("%s landing answer = %q, want %q", number, gotAnswer, answer)
	}
	gotSub, ok := renderedDivText(widget, `<div class="mt-1 text-ink-muted">`)
	if !ok {
		t.Fatalf("%s landing widget has no context: %s", number, widget)
	}
	if gotSub != sub {
		t.Errorf("%s landing context = %q, want %q", number, gotSub, sub)
	}
}

func landingQuestionWidget(t *testing.T, body, number, question string) string {
	t.Helper()
	marker := `<h2 class="text-sm font-semibold text-ink">` + html.EscapeString(number+" · "+question) + `</h2>`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("landing page has no %s widget", number)
	}
	rest := body[i+len(marker):]
	j := strings.Index(rest, "</section>")
	if j < 0 {
		t.Fatalf("landing %s widget is unterminated", number)
	}
	return rest[:j]
}

func renderedDivText(row, prefix string) (string, bool) {
	i := strings.Index(row, prefix)
	if i < 0 {
		return "", false
	}
	rest := row[i+len(prefix):]
	j := strings.Index(rest, "</div>")
	if j < 0 {
		return "", false
	}
	return html.UnescapeString(rest[:j]), true
}
