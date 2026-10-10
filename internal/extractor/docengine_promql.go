package extractor

// docengine_promql.go — the doc engine's promql reference mode (LB-F317,
// docs/design/language-breadth.md §4 "PromQL references"). A Prometheus
// rule, a Grafana panel target or a PrometheusRule spec carries its
// references inside a PromQL string, which no selector or pattern reads
// soundly. With `promql` set, each value a reference's select and value
// reach is read as an expression by internal/promql's selector scanner
// (never evaluated):
//
//   - an intra-file reference (DocRef) takes every selector's metric name:
//     a recording rule's `record:` name another rule's expr reads (L2);
//   - a route-host cross-file reference (DocCrossRef) takes each selector's
//     equality matcher on an identifying label (promql.IdentityLabels(), or
//     the spec's own `labels`) as one route value, scoped by the selector's
//     own `namespace` equality matcher, else the reference's scope (a
//     PrometheusRule's metadata.namespace). The value is written in the DNS
//     form the route resolver reads (`name.ns`, fusion.routeTarget), a value
//     already dotted kept as written, and the matchers it was read from are
//     kept in Labels.
//
// A regex or negative matcher on an identifying label (or on the namespace
// that would scope one) and an expression the scanner cannot read are never
// guessed: each is a DocUnresolved, counted by reason.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/promql"
)

// DocPromQL is a reference's promql mode.
type DocPromQL struct {
	// Labels are the identifying labels a route-host reference reads; unset,
	// promql.IdentityLabels(). The namespace label scopes the others and is
	// never a value itself.
	Labels []string `json:"labels,omitempty"`
	// Grafana (wave c3) reads the value as a Grafana panel or alert query:
	// scanned only when its datasource is Prometheus-typed, its macros
	// substituted and its template variables refused as names first
	// (docengine_grafana.go).
	Grafana bool `json:"grafana,omitempty"`
}

// DocUnresolved is a reference the engine read and refuses to guess.
type DocUnresolved struct {
	From, Edge string
	// Raw is the matcher as written (`job=~"api|web"`), or the expression
	// when it did not scan.
	Raw    string
	Reason string
	Line   int
}

// The reasons a promql reference is unresolved.
const (
	PromQLRegex    = "regex-matcher"
	PromQLNegative = "negative-matcher"
	PromQLUnparsed = "unparsed"
	// PromQLTemplated: a metric name a template variable writes (Grafana's
	// `$metric`), decided at render time, never a name (K20).
	PromQLTemplated = "template-variable"
)

// compilePromQLRef checks an intra-file reference's promql mode.
func compilePromQLRef(r DocRef, where string) error {
	if r.PromQL == nil {
		return nil
	}
	switch {
	case len(r.PromQL.Labels) > 0:
		return fmt.Errorf("%s: promql labels belong to a route-host cross-file reference; an intra-file one reads metric names", where)
	case r.Pattern != "" || r.Scan != "" || len(r.Pointer) > 0:
		return fmt.Errorf("%s: promql reads the value as an expression; it takes no pattern, scan or pointer", where)
	}
	return nil
}

// compilePromQLCross checks a cross-file reference's promql mode and
// returns the identifying label set it reads (nil without the mode).
func compilePromQLCross(c DocCrossRef, where string) (map[string]bool, error) {
	if c.PromQL == nil {
		return nil, nil
	}
	switch {
	case c.Resolve != "route-host" && c.Resolve != "entity-name":
		return nil, fmt.Errorf("%s: promql reads route values (route-host) or metric names (entity-name), not %q", where, c.Resolve)
	case c.Pattern != "" || c.Via != nil:
		return nil, fmt.Errorf("%s: promql reads the value as an expression; it takes no pattern or via", where)
	case c.Resolve == "entity-name" && len(c.PromQL.Labels) > 0:
		return nil, fmt.Errorf("%s: promql labels belong to a route-host reference; an entity-name one reads metric names", where)
	case c.Resolve == "entity-name":
		return nil, nil
	}
	labels := c.PromQL.Labels
	if len(labels) == 0 {
		labels = promql.IdentityLabels()
	}
	set := map[string]bool{}
	for _, l := range labels {
		if l == "" {
			return nil, fmt.Errorf("%s: promql labels: an empty label", where)
		}
		set[l] = true
	}
	return set, nil
}

// promqlMetrics is every metric name the expression's selectors read, in
// written order (none when it does not scan).
func promqlMetrics(expr string) []string {
	sels, err := promql.Selectors(expr)
	if err != nil {
		return nil
	}
	var out []string
	for _, s := range sels {
		if s.Metric != "" {
			out = append(out, s.Metric)
		}
	}
	return out
}

// promqlNameRefs reads one expression's metric names for entity from, each
// an entity-name reference to toKind; a name a template variable writes and
// an expression the scanner cannot read are refused, never guessed.
func promqlNameRefs(from, edge, toKind, expr string, line int) ([]DocCrossFileRef, []DocUnresolved) {
	sels, err := promql.Selectors(expr)
	if err != nil {
		return nil, []DocUnresolved{{From: from, Edge: edge, Raw: strings.TrimSpace(expr), Reason: PromQLUnparsed, Line: line}}
	}
	var refs []DocCrossFileRef
	var un []DocUnresolved
	seen := map[string]bool{}
	for _, s := range sels {
		switch {
		case s.Metric == "" || seen[s.Metric]:
			continue
		case isTemplatedMetric(s.Metric):
			un = append(un, DocUnresolved{From: from, Edge: edge, Raw: untemplatedName(s.Metric), Reason: PromQLTemplated, Line: line})
		default:
			refs = append(refs, DocCrossFileRef{From: from, Edge: edge, Raw: s.Metric, Target: s.Metric, Strategy: "entity-name", Line: line, ToKind: toKind})
		}
		seen[s.Metric] = true
	}
	return refs, un
}

// promqlRouteRefs reads one expression's route values for entity from:
// the references, and what it refused to guess.
func promqlRouteRefs(from, edge, expr string, line int, labels map[string]bool, scope string) ([]DocCrossFileRef, []DocUnresolved) {
	sels, err := promql.Selectors(expr)
	if err != nil {
		return nil, []DocUnresolved{{From: from, Edge: edge, Raw: strings.TrimSpace(expr), Reason: PromQLUnparsed, Line: line}}
	}
	var refs []DocCrossFileRef
	var un []DocUnresolved
	miss := func(m promql.Matcher) {
		reason := PromQLRegex
		if m.Op == promql.OpNotEqual {
			reason = PromQLNegative
		}
		un = append(un, DocUnresolved{From: from, Edge: edge, Raw: m.Name + m.Op + fmt.Sprintf("%q", m.Value), Reason: reason, Line: line})
	}
	for _, s := range sels {
		ns, nsFixed := scope, true
		for _, m := range s.Matchers {
			if m.Name != promql.NamespaceLabel {
				continue
			}
			if m.Op == promql.OpEqual {
				ns = m.Value
			} else {
				nsFixed = false
			}
		}
		for _, m := range s.Matchers {
			if !labels[m.Name] || m.Name == promql.NamespaceLabel {
				continue
			}
			if m.Op != promql.OpEqual {
				miss(m)
				continue
			}
			if m.Value == "" {
				continue // matches a series without the label: names nothing
			}
			if !nsFixed {
				for _, n := range s.Matchers {
					if n.Name == promql.NamespaceLabel && n.Op != promql.OpEqual {
						miss(n)
					}
				}
				continue
			}
			raw := m.Value
			got := map[string]string{m.Name: m.Value}
			if ns != "" && !strings.Contains(raw, ".") {
				raw += "." + ns
			}
			if v, ok := s.Equal(promql.NamespaceLabel); ok {
				got[promql.NamespaceLabel] = v
			}
			refs = append(refs, DocCrossFileRef{From: from, Edge: edge, Raw: raw, Target: raw, Strategy: "route-host", Line: line, Scope: ns, Labels: got})
		}
	}
	return refs, un
}

// sortUnresolved orders a result's unresolved references (line, raw,
// reason), deterministically.
func sortUnresolved(u []DocUnresolved) {
	sort.SliceStable(u, func(i, j int) bool {
		a, b := u[i], u[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return false
	})
}
