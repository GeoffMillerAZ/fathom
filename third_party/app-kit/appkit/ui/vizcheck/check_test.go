package vizcheck_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/vizcheck"
)

// Expected labels and cells are independent constants, not parsed from output or
// assembled by the rendering loop. These tests assert markup, not business truth.
func fixture(showValues bool) (string, vizcheck.Spec) {
	bars := viz.Bars([]viz.Bar{
		{Label: "Current & complete", Value: 12, Role: "accent"},
		{Label: "Previous", Value: 7, Role: "ink-muted"},
	}, viz.BarsOptions{ShowValue: showValues, AriaLabel: "Failure counts"})
	trend := viz.Sparkline([]float64{3, 5, 8}, viz.SparklineOptions{AriaLabel: "Recent failures"})
	body := `<main id="analysis"><div id="bars">` + bars + `</div>` +
		`<div id="trend">` + trend + `<p id="latest">Latest: 8 failures</p></div>` +
		`<p id="legend">Accent: current; muted: previous</p>` +
		`<table id="values"><caption>Failures by window</caption>` +
		`<thead><tr><th scope="col">Window</th><th scope="col">Failures</th></tr></thead>` +
		`<tbody><tr><th scope="row">Current &amp; complete</th><td>12</td></tr>` +
		`<tr><th scope="row">Previous</th><td>7</td></tr></tbody></table>` +
		`<p id="source">Source: synthetic fixed fixture.</p></main>`
	return body, vizcheck.Spec{
		RootID: "analysis",
		Charts: []vizcheck.Chart{
			{ID: "bars", Name: "Failure counts", Labels: []string{"Current & complete", "12", "Previous", "7"}},
			{ID: "trend", Name: "Recent failures", Annotations: []vizcheck.Text{{ID: "latest", Value: "Latest: 8 failures"}}},
		},
		Table: vizcheck.Table{ID: "values", Caption: "Failures by window", Rows: [][]string{
			{"Window", "Failures"}, {"Current & complete", "12"}, {"Previous", "7"},
		}},
		Source: vizcheck.Text{ID: "source", Value: "Source: synthetic fixed fixture."},
		Legend: []vizcheck.Text{{ID: "legend", Value: "Accent: current; muted: previous"}},
	}
}

func expectPass(t *testing.T, body string, spec vizcheck.Spec) vizcheck.Report {
	t.Helper()
	report, err := vizcheck.CheckHTML([]byte(body), spec)
	if err != nil || len(report.Findings) != 0 {
		t.Fatalf("CheckHTML = %+v, %v; want static expectations satisfied", report, err)
	}
	return report
}

func expectFinding(t *testing.T, body string, spec vizcheck.Spec, rule string) {
	t.Helper()
	report, err := vizcheck.CheckHTML([]byte(body), spec)
	if err != nil {
		t.Fatalf("unexpected operational/spec error: %v", err)
	}
	for _, f := range report.Findings {
		if rule == "" || f.Rule == rule {
			return
		}
	}
	t.Fatalf("missing finding %q: %+v", rule, report)
}

func TestRealBarsAndSparklineComposition(t *testing.T) {
	body, spec := fixture(true)
	input := []byte(body)
	before := bytes.Clone(input)
	first := expectPass(t, body, spec)
	second, err := vizcheck.CheckHTML(input, spec)
	if err != nil || !reflect.DeepEqual(first, second) || !bytes.Equal(input, before) {
		t.Fatalf("check must be deterministic and leave HTML unchanged: %v", err)
	}
	if first.Charts != 2 || first.Tables != 1 || first.Decorations != 0 {
		t.Fatalf("inventory = %+v", first)
	}
	// Multiple color roles do not impose a legend policy on direct labels.
	spec.Legend = nil
	expectPass(t, strings.Replace(body, `<p id="legend">Accent: current; muted: previous</p>`, "", 1), spec)
}

func TestLabelAndValueFailureCanaries(t *testing.T) {
	for _, tc := range []struct {
		name, old, replacement string
	}{
		{"wrong value", `>12</text>`, `>13</text>`},
		{"wrong label", `>Previous</text>`, `>Different</text>`},
		{"unprinted title", `>12</text>`, `></text><title>12</title>`},
		{"hidden label", `>12</text>`, ` hidden>12</text>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, spec := fixture(true)
			if !strings.Contains(body, tc.old) {
				t.Fatal("mutation did not match its renderer target")
			}
			expectFinding(t, strings.Replace(body, tc.old, tc.replacement, 1), spec, "chart-labels")
		})
	}
	body, spec := fixture(false)
	expectFinding(t, body, spec, "chart-labels")
	body, spec = fixture(true)
	spec.Charts[0].Labels = []string{"Previous", "7", "Current & complete", "12"}
	expectFinding(t, body, spec, "chart-labels")
}

func TestScopedStatementsCannotBorrowMatchingText(t *testing.T) {
	for _, id := range []string{"legend", "source", "latest"} {
		t.Run(id, func(t *testing.T) {
			body, spec := fixture(true)
			start := strings.Index(body, `<p id="`+id+`">`)
			end := start + strings.Index(body[start:], "</p>") + len("</p>")
			statement := body[start:end]
			without := body[:start] + body[end:]
			expectFinding(t, without, spec, "element-scope")
			outside := without + statement
			if id == "latest" {
				// Still in the selected composition, but not its chart wrapper.
				outside = strings.Replace(without, "</main>", statement+"</main>", 1)
			}
			expectFinding(t, outside, spec, "element-scope")
			expectFinding(t, strings.Replace(body, statement, strings.Replace(statement, ">", ">Wrong ", 1), 1), spec, "")
			expectFinding(t, strings.Replace(body, `<p id="`+id+`">`, `<p hidden id="`+id+`">`, 1), spec, "element-hidden")
		})
	}
}

func TestTableFailureCanaries(t *testing.T) {
	for _, tc := range []struct{ name, old, replacement, rule string }{
		{"wrong cell", `<td>12</td>`, `<td>21</td>`, "table-cells"},
		{"wrong order", `<th scope="row">Previous</th><td>7</td>`, `<td>7</td><th scope="row">Previous</th>`, "table-cells"},
		{"missing row", `<tr><th scope="row">Previous</th><td>7</td></tr>`, "", "table-rows"},
		{"caption", `<caption>Failures by window</caption>`, `<caption>Other</caption>`, "table-caption"},
		{"missing caption", `<caption>Failures by window</caption>`, "", "table-caption"},
		{"colspan", `<td>12</td>`, `<td colspan="1">12</td>`, "table-span"},
		{"rowspan", `<td>12</td>`, `<td rowspan="1">12</td>`, "table-span"},
		{"empty span", `<td>12</td>`, `<td colspan>12</td>`, "table-span"},
		{"header scope", `<th scope="col">Window</th>`, `<th>Window</th>`, "table-header"},
		{"nested table", `<td>12</td>`, `<td>12<table><tr><td>nested</td></tr></table></td>`, "table-structure"},
		{"hidden row", `<tr><th scope="row">Previous`, `<tr hidden><th scope="row">Previous`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, spec := fixture(true)
			if !strings.Contains(body, tc.old) {
				t.Fatal("mutation missed its target")
			}
			expectFinding(t, strings.Replace(body, tc.old, tc.replacement, 1), spec, tc.rule)
		})
	}
}

func TestNameDeclarationsAreExplicitAndScoped(t *testing.T) {
	body, spec := fixture(true)
	labelled := strings.Replace(body, `aria-label="Failure counts"`, `aria-labelledby="chart-title"`, 1)
	labelled = strings.Replace(labelled, `<title>Failure counts</title>`, `<title id="chart-title">Failure counts</title>`, 1)
	expectPass(t, labelled, spec)
	// aria-labelledby uses HTML ASCII whitespace, not arbitrary Unicode spaces.
	expectFinding(t, strings.Replace(labelled, `aria-labelledby="chart-title"`, `aria-labelledby="chart-title&nbsp;"`, 1), spec, "chart-name")
	for _, replacement := range []string{"", `aria-label="Wrong"`, `aria-labelledby="absent"`} {
		expectFinding(t, strings.Replace(body, `aria-label="Failure counts"`, replacement, 1), spec, "chart-name")
	}
	outOfScope := strings.Replace(body, `aria-label="Failure counts"`, `aria-labelledby="outside"`, 1) + `<span id="outside">Failure counts</span>`
	expectFinding(t, outOfScope, spec, "chart-name")
	// Explicitly hidden SVG is not a displayed chart, even when its nearby value exists.
	hiddenTrend := strings.Replace(body, `<div id="trend"><svg`, `<div id="trend"><svg hidden`, 1)
	expectFinding(t, hiddenTrend, spec, "")
}

func TestInventoryCannotLoseCoverageSilently(t *testing.T) {
	body, spec := fixture(true)
	spec.RootID = "absent"
	expectFinding(t, body, spec, "element-scope")
	body, spec = fixture(true)
	spec.Charts = spec.Charts[:1]
	expectFinding(t, body, spec, "svg-coverage")
	body, spec = fixture(true)
	extra := strings.Replace(body, "</main>", `<svg id="icon" aria-hidden="true"><path d="M0 0"/></svg></main>`, 1)
	expectFinding(t, extra, spec, "svg-coverage")
	spec.Decorations = []vizcheck.Decoration{{ID: "icon", Reason: "Synthetic ornamental separator, not data"}}
	if got := expectPass(t, extra, spec); got.Decorations != 1 {
		t.Fatalf("declared decoration not counted: %+v", got)
	}
	spec.Decorations[0].ID = "source"
	if _, err := vizcheck.CheckHTML([]byte(extra), spec); err == nil {
		t.Fatal("spec ID aliases must be refused")
	}
	body, spec = fixture(true)
	doubled := strings.Replace(body, `<div id="bars">`, `<div id="bars"><svg></svg>`, 1)
	expectFinding(t, doubled, spec, "chart-svg-count")
	body = strings.Replace(body, `<div id="bars">`, `<div id="outer"><div id="bars">`, 1)
	body = strings.Replace(body, `<div id="trend">`, `</div><div id="trend">`, 1)
	spec.Charts = append(spec.Charts, vizcheck.Chart{ID: "outer", Name: "Failure counts", Labels: []string{"Current & complete", "12", "Previous", "7"}})
	expectFinding(t, body, spec, "svg-ownership")
}

func TestChartIDMaySelectTheSVGItself(t *testing.T) {
	body, spec := fixture(true)
	body = strings.Replace(body, `<div id="bars"><svg`, `<div><svg id="bars"`, 1)
	expectPass(t, body, spec)
}

func TestExplicitTableOnlyState(t *testing.T) {
	body, spec := fixture(true)
	start := strings.Index(body, `<div id="bars">`)
	end := strings.Index(body, `<p id="legend">`)
	body = body[:start] + body[end:]
	spec.Charts = []vizcheck.Chart{}
	got := expectPass(t, body, spec)
	if got.Charts != 0 || got.Tables != 1 {
		t.Fatalf("table-only inventory = %+v", got)
	}
	expectFinding(t, strings.Replace(body, "</main>", "<svg></svg></main>", 1), spec, "svg-coverage")
}

func TestTextExtractionIsNotSubstringMatching(t *testing.T) {
	body, spec := fixture(true)
	formatted := strings.Replace(body, `>Current &amp; complete</text>`, `> Current <tspan>&amp;</tspan>\n complete </text>`, 1)
	formatted = strings.ReplaceAll(formatted, `\n`, "\n")
	expectPass(t, formatted, spec)
	for _, prefix := range []string{
		`<defs><text>not printed</text></defs>`, `<desc><text>not printed</text></desc>`,
		`<g hidden><text>not printed</text></g>`, `<title>not printed</title>`,
	} {
		expectPass(t, strings.Replace(body, `<title>Failure counts</title>`, prefix+`<title>Failure counts</title>`, 1), spec)
	}
	expectFinding(t, strings.Replace(body, `>Previous</text>`, `>Previous&nbsp;</text>`, 1), spec, "chart-labels")
	expectFinding(t, strings.Replace(body, `<td>7</td>`, `<td>−7</td>`, 1), spec, "table-cells")
	// Script/style/template text cannot satisfy a required statement.
	for _, tag := range []string{"script", "style", "template"} {
		changed := strings.Replace(body, `<p id="source">Source: synthetic fixed fixture.</p>`, `<`+tag+` id="source">Source: synthetic fixed fixture.</`+tag+`>`, 1)
		expectFinding(t, changed, spec, "element-hidden")
	}
}

func TestStaticSuccessDoesNotCertifyCSSGeometryOrSource(t *testing.T) {
	body, spec := fixture(true)
	// These deliberate defects are outside this helper's stated oracle. A browser
	// and an app-owned source/derivation test must decide them independently.
	body = strings.Replace(body, `<div id="bars">`, `<div id="bars" style="display:none">`, 1)
	body = strings.Replace(body, `<div id="trend">`, `<div id="trend" style="opacity:0">`, 1)
	body = strings.Replace(body, `x="0"`, `x="999999"`, 1)
	spec.Source.Value = "Source: caller assertion, not independently verified."
	body = strings.Replace(body, "Source: synthetic fixed fixture.", spec.Source.Value, 1)
	expectPass(t, body, spec)
}

func TestAmbiguousHTMLIsAnError(t *testing.T) {
	for _, extra := range []string{
		`<p id="source">duplicate</p>`, `<p id="">empty</p>`, `<p class="a" class="b">duplicate attribute</p>`,
		`<p id="outside" ID="other">case-insensitive duplicate</p>`,
	} {
		body, spec := fixture(true)
		if _, err := vizcheck.CheckHTML([]byte(body+extra), spec); err == nil {
			t.Fatalf("ambiguous HTML accepted: %q", extra)
		}
	}
}

func TestInvalidSpecsRefuseEmptySuccess(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*vizcheck.Spec)
	}{
		{"zero", func(s *vizcheck.Spec) { *s = vizcheck.Spec{} }},
		{"empty root", func(s *vizcheck.Spec) { s.RootID = "" }},
		{"spaced id", func(s *vizcheck.Spec) { s.RootID = "not one id" }},
		{"duplicate id", func(s *vizcheck.Spec) { s.Table.ID = s.RootID }},
		{"missing name", func(s *vizcheck.Spec) { s.Charts[0].Name = "" }},
		{"unasserted chart", func(s *vizcheck.Spec) { s.Charts[0].Labels = nil }},
		{"empty label", func(s *vizcheck.Spec) { s.Charts[0].Labels[0] = " " }},
		{"empty caption", func(s *vizcheck.Spec) { s.Table.Caption = "" }},
		{"no rows", func(s *vizcheck.Spec) { s.Table.Rows = nil }},
		{"empty row", func(s *vizcheck.Spec) { s.Table.Rows = [][]string{{}} }},
		{"ragged rows", func(s *vizcheck.Spec) { s.Table.Rows[1] = []string{"one"} }},
		{"source required", func(s *vizcheck.Spec) { s.Source.Value = " " }},
		{"nul", func(s *vizcheck.Spec) { s.Source.Value = "not\x00text" }},
		{"invalid utf8", func(s *vizcheck.Spec) { s.Source.Value = string([]byte{0xff}) }},
		{"decoration reason", func(s *vizcheck.Spec) { s.Decorations = []vizcheck.Decoration{{ID: "icon"}} }},
		{"too many charts", func(s *vizcheck.Spec) { s.Charts = make([]vizcheck.Chart, vizcheck.MaxCharts+1) }},
		{"too many cells", func(s *vizcheck.Spec) { s.Table.Rows = [][]string{make([]string, vizcheck.MaxCells+1)} }},
		{"spec bytes", func(s *vizcheck.Spec) { s.Source.Value = strings.Repeat("x", 256<<10) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, spec := fixture(true)
			tc.change(&spec)
			if got, err := vizcheck.CheckHTML([]byte(body), spec); err == nil {
				t.Fatalf("invalid spec returned successful check: %+v", got)
			}
		})
	}
}

func TestHTMLResourceBounds(t *testing.T) {
	body, spec := fixture(true)
	for _, tc := range []struct {
		name   string
		markup []byte
	}{
		{"empty", nil},
		{"invalid utf8", []byte{0xff}},
		{"byte limit", []byte(strings.Repeat("x", vizcheck.MaxHTMLBytes+1))},
		{"depth limit", []byte(strings.Repeat("<div>", vizcheck.MaxDepth+1) + body + strings.Repeat("</div>", vizcheck.MaxDepth+1))},
		{"node limit", []byte(body + strings.Repeat("<i></i>", vizcheck.MaxNodes+1))},
		{"actual cell limit", []byte(strings.Replace(body, "<tbody>", "<tbody><tr>"+strings.Repeat("<td>x</td>", vizcheck.MaxCells+1)+"</tr>", 1))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := vizcheck.CheckHTML(tc.markup, spec); err == nil {
				t.Fatalf("resource/encoding violation returned success: %+v", got)
			}
		})
	}
}

func TestFindingsAreBoundedWithoutBecomingSuccess(t *testing.T) {
	body, spec := fixture(true)
	body = strings.Replace(body, "</main>", strings.Repeat("<svg></svg>", 300)+"</main>", 1)
	first, err := vizcheck.CheckHTML([]byte(body), spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Findings) == 0 || len(first.Findings) > 129 {
		t.Fatalf("unbounded or silently dropped findings: %d", len(first.Findings))
	}
	if first.Findings[len(first.Findings)-1].Rule != "finding-limit" {
		t.Fatal("omitted failures need an explicit terminal finding")
	}
	second, err := vizcheck.CheckHTML([]byte(body), spec)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("bounded failures must be deterministic: %v", err)
	}
}

func TestNameReferencesDoNotResolveIntoInertContainers(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		pass         bool
	}{
		{"live span", `<span id="resolved-name">Failure counts</span>`, true},
		{"hidden live span", `<span hidden id="resolved-name">Failure counts</span>`, true},
		{"hidden live ancestor", `<div hidden><span id="resolved-name">Failure counts</span></div>`, true},
		{"template ancestor", `<template><span id="resolved-name">Failure counts</span></template>`, false},
		{"script target", `<script id="resolved-name">Failure counts</script>`, false},
		{"style target", `<style id="resolved-name">Failure counts</style>`, false},
		{"noscript target", `<noscript id="resolved-name">Failure counts</noscript>`, false},
		{"script ancestor", `<script><span id="resolved-name">Failure counts</span></script>`, false},
		{"style ancestor", `<style><span id="resolved-name">Failure counts</span></style>`, false},
		{"noscript ancestor", `<noscript><span id="resolved-name">Failure counts</span></noscript>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, spec := fixture(true)
			body = strings.Replace(body, `aria-label="Failure counts"`, `aria-labelledby="resolved-name"`, 1)
			body = strings.Replace(body, "</main>", tc.target+"</main>", 1)
			if tc.pass {
				expectPass(t, body, spec)
			} else {
				expectFinding(t, body, spec, "chart-name")
			}
		})
	}
	// A real SVG title remains a legitimate declaration, not a printed value.
	body, spec := fixture(true)
	body = strings.Replace(body, `aria-label="Failure counts"`, `aria-labelledby="resolved-name"`, 1)
	body = strings.Replace(body, `<title>Failure counts</title>`, `<title id="resolved-name">Failure counts</title>`, 1)
	expectPass(t, body, spec)
}

func TestSVGChecksRespectElementNamespaces(t *testing.T) {
	t.Run("MathML svg is not an SVG chart", func(t *testing.T) {
		body, spec := fixture(true)
		body = strings.Replace(body, `<div id="bars"><svg`, `<div id="bars"><math><svg`, 1)
		body = strings.Replace(body, `</svg></div>`, `</svg></math></div>`, 1)
		expectFinding(t, body, spec, "chart-svg-count")
	})
	t.Run("foreignObject HTML text is not SVG text", func(t *testing.T) {
		body, spec := fixture(true)
		start := strings.Index(body, "<text ")
		end := start + strings.Index(body[start:], "</text>") + len("</text>")
		body = body[:start] + `<foreignObject><text>Current &amp; complete</text></foreignObject>` + body[end:]
		expectFinding(t, body, spec, "chart-labels")
	})
}

func TestSVGDefinitionsCannotSupplyPrintedLabels(t *testing.T) {
	for _, tag := range []string{"filter", "linearGradient", "radialGradient"} {
		t.Run(tag, func(t *testing.T) {
			body, spec := fixture(true)
			start := strings.Index(body, "<text ")
			end := start + strings.Index(body[start:], "</text>") + len("</text>")
			body = body[:start] + "<" + tag + ">" + body[start:end] + "</" + tag + ">" + body[end:]
			expectFinding(t, body, spec, "chart-labels")
		})
	}
}
