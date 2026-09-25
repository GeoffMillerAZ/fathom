// Package vizcheck checks explicitly selected static chart compositions against
// an application-owned output oracle. It does not render, execute JavaScript,
// evaluate CSS, verify geometry or establish that source data supports a claim.
package vizcheck

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

const (
	MaxHTMLBytes = 1 << 20
	MaxNodes     = 50000
	MaxDepth     = 128
	MaxCharts    = 64
	MaxCells     = 4096
)

// Text binds expected text to one exact element ID, not a document-wide match.
// HTML ASCII whitespace is collapsed; nonbreaking spaces and minus signs remain.
type Text struct{ ID, Value string }

// Chart owns exactly one SVG, either at ID or inside its wrapper. Labels is the
// complete ordered sequence of SVG text elements (each tspan tree is one label).
// Annotations must be inside that wrapper: use them for a trend's last value.
// At least one label or annotation is required. Name checks only an aria-label
// or resolving aria-labelledby declaration, not the accessible-name algorithm.
type Chart struct {
	ID, Name    string
	Labels      []string
	Annotations []Text
}

// Table describes one simple table, including its header row. Spans and nested
// tables are deliberately unsupported. Caption may be visually hidden by CSS;
// this checker establishes text/markup presence, not computed visibility.
type Table struct {
	ID, Caption string
	Rows        [][]string
}

// Decoration explicitly excludes one SVG from chart expectations. The SVG itself
// needs an ID and a nonempty reason; aria-hidden alone never excuses coverage.
type Decoration struct{ ID, Reason string }

// Spec selects one composition, its entire SVG inventory, a table and source.
// An explicitly empty Charts slice is valid for a missing/empty/table-only state.
// Legend is optional: direct labels can communicate series without a legend.
// The caller declares the intended encoding and must test its derivation separately.
type Spec struct {
	RootID      string
	Charts      []Chart
	Decorations []Decoration
	Table       Table
	Source      Text
	Legend      []Text
}

type Finding struct{ Rule, ID, Message string }

// Report counts inspected declared elements, not verified applications. A nil
// error and zero Findings establish only the static expectations in this call.
type Report struct {
	Charts, Decorations, Tables int
	Findings                    []Finding
}

// CheckHTML parses bounded HTML and checks exact, scope-bound expectations.
// Invalid specifications, ambiguous IDs/attributes and resource bounds are errors,
// never empty successful reports. HTML parsing repairs markup; it is not validation.
func CheckHTML(markup []byte, spec Spec) (Report, error) {
	if err := validateSpec(spec); err != nil {
		return Report{}, err
	}
	if len(markup) == 0 || len(markup) > MaxHTMLBytes || !utf8.Valid(markup) {
		return Report{}, fmt.Errorf("vizcheck: HTML must be nonempty UTF-8, at most %d bytes", MaxHTMLBytes)
	}
	doc, err := html.Parse(strings.NewReader(string(markup)))
	if err != nil {
		return Report{}, fmt.Errorf("vizcheck: parse HTML: %w", err)
	}
	ids, err := index(doc)
	if err != nil {
		return Report{}, err
	}
	a := audit{ids: ids}
	root := a.element(spec.RootID, doc)
	if root == nil {
		return a.report, nil
	}
	owned := map[*html.Node]string{}
	for _, c := range spec.Charts {
		wrapper := a.element(c.ID, root)
		if wrapper == nil {
			continue
		}
		svgs := elements(wrapper, "svg")
		if len(svgs) != 1 {
			a.fail("chart-svg-count", c.ID, "chart must own exactly one SVG")
			continue
		}
		svg := svgs[0]
		if prior, ok := owned[svg]; ok {
			a.fail("svg-ownership", c.ID, "SVG already belongs to "+prior)
		}
		owned[svg] = c.ID
		a.report.Charts++
		if unavailable(svg) {
			a.fail("chart-hidden", c.ID, "SVG is hidden or non-content markup")
		}
		name, ok := a.name(svg, root)
		if !ok || normalize(name) != normalize(c.Name) {
			a.fail("chart-name", c.ID, "SVG name declaration differs or does not resolve")
		}
		var labels []string
		walk(svg, func(n *html.Node) bool {
			if excluded(n) {
				return false
			}
			if n.Type == html.ElementNode && n.Namespace == "svg" && n.Data == "text" {
				labels = append(labels, content(n))
				return false
			}
			return true
		})
		want := make([]string, len(c.Labels))
		for i, v := range c.Labels {
			want[i] = normalize(v)
		}
		if !slices.Equal(labels, want) {
			a.fail("chart-labels", c.ID, "ordered SVG text content differs from expected labels/values")
		}
		for _, annotation := range c.Annotations {
			a.text(annotation, wrapper, "chart-annotation")
		}
	}
	for _, d := range spec.Decorations {
		n := a.element(d.ID, root)
		if n == nil {
			continue
		}
		if n.Data != "svg" || n.Namespace != "svg" {
			a.fail("decoration-svg", d.ID, "decoration ID must identify the SVG itself")
			continue
		}
		if _, ok := owned[n]; ok {
			a.fail("svg-ownership", d.ID, "SVG cannot be both chart and decoration")
		}
		owned[n] = d.ID
		a.report.Decorations++
	}
	for _, svg := range elements(root, "svg") {
		if _, ok := owned[svg]; !ok {
			a.fail("svg-coverage", attr(svg, "id"), "SVG has no declared chart or decoration expectation")
		}
	}
	a.table(spec.Table, root)
	a.text(spec.Source, root, "source-text")
	for _, legend := range spec.Legend {
		a.text(legend, root, "legend-text")
	}
	return a.report, nil
}

type audit struct {
	ids    map[string]*html.Node
	report Report
}

func (a *audit) fail(rule, id, message string) {
	const maxFindings = 128
	if len(a.report.Findings) < maxFindings {
		a.report.Findings = append(a.report.Findings, Finding{rule, id, message})
	} else if len(a.report.Findings) == maxFindings {
		a.report.Findings = append(a.report.Findings, Finding{"finding-limit", "", "further failures omitted; this check has failed"})
	}
}

func (a *audit) element(id string, scope *html.Node) *html.Node {
	n := a.ids[id]
	if n == nil || !within(n, scope) {
		a.fail("element-scope", id, "required element missing from its declared scope")
		return nil
	}
	if unavailable(n) {
		a.fail("element-hidden", id, "required element is hidden or non-content markup")
		return nil
	}
	return n
}

func (a *audit) text(t Text, scope *html.Node, rule string) {
	n := a.element(t.ID, scope)
	if n != nil && content(n) != normalize(t.Value) {
		a.fail(rule, t.ID, "text content differs from the expected statement")
	}
}

func (a *audit) name(svg, root *html.Node) (string, bool) {
	if refs := attr(svg, "aria-labelledby"); refs != "" {
		var parts []string
		for _, id := range strings.Split(normalize(refs), " ") {
			n := a.ids[id]
			if n == nil || !within(n, root) || inertNameReference(n) {
				return "", false
			}
			// title is a legitimate name declaration, but never a printed value.
			var b strings.Builder
			walk(n, func(v *html.Node) bool {
				if v.Type == html.ElementNode {
					switch v.Data {
					case "script", "style", "template", "noscript":
						return false
					}
				}
				if v.Type == html.TextNode {
					b.WriteString(v.Data)
				}
				return true
			})
			parts = append(parts, normalize(b.String()))
		}
		return strings.Join(parts, " "), true
	}
	label := attr(svg, "aria-label")
	return label, normalize(label) != ""
}

func (a *audit) table(t Table, root *html.Node) {
	n := a.element(t.ID, root)
	if n == nil {
		return
	}
	if n.Data != "table" || n.Namespace != "" || len(elements(n, "table")) != 1 {
		a.fail("table-structure", t.ID, "expected one table without nested tables")
		return
	}
	a.report.Tables++
	var captions []*html.Node
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Namespace == "" && child.Data == "caption" {
			captions = append(captions, child)
		}
	}
	if len(captions) != 1 || content(captions[0]) != normalize(t.Caption) {
		a.fail("table-caption", t.ID, "one matching table caption is required")
	}
	rows := elements(n, "tr")
	if len(rows) != len(t.Rows) {
		a.fail("table-rows", t.ID, "table row count differs")
	}
	for i, row := range rows {
		var values []string
		for cell := row.FirstChild; cell != nil; cell = cell.NextSibling {
			if cell.Type != html.ElementNode || cell.Namespace != "" || (cell.Data != "td" && cell.Data != "th") {
				continue
			}
			if has(cell, "colspan") || has(cell, "rowspan") {
				a.fail("table-span", t.ID, "spanning cells are unsupported")
			}
			if unavailable(cell) {
				a.fail("table-hidden", t.ID, "table cells must not be hidden or non-content markup")
			}
			if i == 0 && (cell.Data != "th" || attr(cell, "scope") != "col") {
				a.fail("table-header", t.ID, "header cells require th with scope=col")
			}
			values = append(values, content(cell))
		}
		if i < len(t.Rows) {
			want := make([]string, len(t.Rows[i]))
			for j, v := range t.Rows[i] {
				want[j] = normalize(v)
			}
			if !slices.Equal(values, want) {
				a.fail("table-cells", t.ID, fmt.Sprintf("table row %d differs", i))
			}
		}
	}
}

func validateSpec(s Spec) error {
	ids := map[string]bool{}
	bytes, cells := 0, 0
	value := func(v string, nonempty bool) bool {
		bytes += len(v)
		return utf8.ValidString(v) && !strings.ContainsRune(v, 0) && (!nonempty || normalize(v) != "") && bytes <= 256<<10
	}
	id := func(v string) bool {
		if !value(v, true) || strings.ContainsAny(v, " \t\r\n\f") || ids[v] {
			return false
		}
		ids[v] = true
		return true
	}
	text := func(t Text) bool { return id(t.ID) && value(t.Value, true) }
	bad := func() error {
		return fmt.Errorf("vizcheck: invalid or oversized specification; IDs must be distinct and expectations explicit")
	}
	if !id(s.RootID) || len(s.Charts)+len(s.Decorations) > MaxCharts || !id(s.Table.ID) || !value(s.Table.Caption, true) || !text(s.Source) || len(s.Table.Rows) == 0 {
		return bad()
	}
	width := len(s.Table.Rows[0])
	if width == 0 {
		return bad()
	}
	for _, row := range s.Table.Rows {
		cells += len(row)
		if len(row) != width || cells > MaxCells {
			return bad()
		}
		for _, v := range row {
			if !value(v, false) {
				return bad()
			}
		}
	}
	for _, c := range s.Charts {
		if !id(c.ID) || !value(c.Name, true) || len(c.Labels)+len(c.Annotations) == 0 || len(c.Labels)+len(c.Annotations) > MaxCells {
			return bad()
		}
		for _, v := range c.Labels {
			if !value(v, true) {
				return bad()
			}
		}
		for _, t := range c.Annotations {
			if !text(t) {
				return bad()
			}
		}
	}
	for _, d := range s.Decorations {
		if !id(d.ID) || !value(d.Reason, true) {
			return bad()
		}
	}
	if len(s.Legend) > MaxCharts {
		return bad()
	}
	for _, t := range s.Legend {
		if !text(t) {
			return bad()
		}
	}
	return nil
}
