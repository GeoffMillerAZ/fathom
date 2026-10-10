package extractor

// docengine_grafana.go — a promql reference's Grafana mode (wave c3,
// LB-F364; docs/design/language-breadth.md §4 "Observability documents"). A
// Grafana panel target or alert query holds a query its datasource reads,
// written with Grafana's own interpolation. With `"promql": {"grafana":
// true}` the engine reads such a value as PromQL only when:
//
//   - its datasource is Prometheus-typed: the target's own (an alert query's
//     model datasource), else its panel's (a mixed panel's targets carry
//     their own), else the dashboard's default (its datasource template
//     variables and `__inputs`, when every one of them is Prometheus); a
//     datasource object's type, a `$var`/`${var}` naming a datasource
//     variable or a `${DS_*}` input reads through the dashboard's own
//     declarations. Any other type (Loki, Tempo, an expression) is counted
//     datasource-not-prometheus and an undecidable one datasource-unknown,
//     never scanned;
//   - Grafana's macros are substituted first ($__interval, $__rate_interval,
//     $__range, $__auto and kin: a fixed duration; their _ms and _s forms and
//     $__from/$__to: a number), so the scanner reads the expression Grafana
//     would send;
//   - a template variable outside a string ($var, ${var}, ${var:fmt},
//     [[var]]) becomes a name no metric can have, so a metric name it writes
//     is refused (template-variable, K20's refuse_templates semantics) and
//     never guessed; one inside a string stays a label value.
//
// Nothing here reads Grafana's source (AGPL-3.0): the behaviour is
// Grafana's documented interpolation and datasource model.

import (
	"strings"
)

// The reasons a Grafana query is not scanned.
const (
	GrafanaNotPrometheus     = "datasource-not-prometheus"
	GrafanaDatasourceUnknown = "datasource-unknown"
)

// grafanaVarPrefix marks a template variable the substitution replaced: no
// metric name begins so.
const grafanaVarPrefix = "__grafana_var__"

// grafanaExpr is the expression a Grafana target or alert query sends,
// ready for the selector scanner, or why it is not read (target is the
// node the query sits in, origin its panel or rule, root the document).
func grafanaExpr(expr string, target, origin, root *docNode) (string, string) {
	switch grafanaDatasource(target, origin, root) {
	case "prometheus":
		return grafanaSubstitute(expr), ""
	case "":
		return "", GrafanaDatasourceUnknown
	default:
		return "", GrafanaNotPrometheus
	}
}

// grafanaDatasource is the plugin id of the datasource a query reads
// ("prometheus", "loki", "__expr__"), "" when it cannot be decided.
func grafanaDatasource(target, origin, root *docNode) string {
	dash := grafanaDashboard(root)
	if target.get("spec") != nil {
		// a v2 dashboard's query: its group (v2beta1, kind DataQuery) or
		// its kind (v2alpha1) is the datasource plugin
		switch k := grafanaScalar(target.get("kind")); {
		case k == "DataQuery" && grafanaScalar(target.get("group")) != "":
			return strings.ToLower(grafanaScalar(target.get("group")))
		case k != "" && k != "DataQuery":
			return strings.ToLower(k)
		}
	}
	for _, ds := range []*docNode{target.get("datasource"), target.get("model").get("datasource")} {
		if id, set := grafanaDatasourceOf(ds, dash, root); set {
			return id
		}
	}
	if uid := target.get("datasourceUid"); uid != nil && uid.kind == docScalar {
		if uid.scalar == "__expr__" {
			return "__expr__" // a server-side expression, no datasource query
		}
		return "" // a literal uid, whose type another file declares
	}
	if id, set := grafanaDatasourceOf(origin.get("datasource"), dash, root); set && id != "mixed" {
		return id
	}
	return grafanaDefault(dash, root)
}

// grafanaDashboard is the dashboard object: the document, or its
// `dashboard` wrapper.
func grafanaDashboard(root *docNode) *docNode {
	if d := root.get("dashboard"); d != nil && d.kind == docMap {
		return d
	}
	return root
}

// grafanaDatasourceOf reads one datasource reference; set is false when
// none is written (absent or null), so the next level decides.
func grafanaDatasourceOf(ds, dash, root *docNode) (id string, set bool) {
	if ds == nil || ds.kind == docNull {
		return "", false
	}
	var ref string
	switch ds.kind {
	case docMap:
		if t := ds.get("type"); t != nil && t.kind == docScalar && t.scalar != "" && !grafanaTemplated(t.scalar) {
			if t.scalar == "datasource" {
				return grafanaUIDKind(ds.get("uid")), true
			}
			return strings.ToLower(t.scalar), true
		}
		u := ds.get("uid")
		if u == nil || u.kind != docScalar {
			return "", true
		}
		ref = u.scalar
	case docScalar:
		ref = ds.scalar
	default:
		return "", true
	}
	switch ref {
	case "-- Mixed --":
		return "mixed", true
	case "-- Grafana --", "grafana":
		return "grafana", true
	case "-- Dashboard --":
		return "dashboard", true
	}
	if name, ok := grafanaVarName(ref); ok {
		return grafanaVarDatasource(name, dash, root), true
	}
	return "", true // a datasource name or uid, whose type is declared elsewhere
}

// grafanaUIDKind reads a `{type: datasource, uid: ...}` special datasource.
func grafanaUIDKind(u *docNode) string {
	if u != nil && u.kind == docScalar {
		switch u.scalar {
		case "-- Mixed --":
			return "mixed"
		case "-- Dashboard --":
			return "dashboard"
		case "grafana", "-- Grafana --":
			return "grafana"
		}
	}
	return ""
}

// grafanaVarName is the variable a whole value names ($x, ${x}, ${x:fmt},
// [[x]]).
func grafanaVarName(v string) (string, bool) {
	v = strings.TrimSpace(v)
	name, n := grafanaVarAt(v)
	if n == 0 || n != len(v) {
		return "", false
	}
	return name, true
}

// grafanaVarDatasource is the plugin id a datasource template variable or
// a `__inputs` datasource input names.
func grafanaVarDatasource(name string, dash, root *docNode) string {
	for _, v := range grafanaList(dash.get("templating").get("list")) {
		if grafanaScalar(v.get("name")) == name && grafanaScalar(v.get("type")) == "datasource" {
			return strings.ToLower(grafanaQueryPlugin(v.get("query")))
		}
	}
	for _, in := range grafanaList(root.get("__inputs")) {
		if grafanaScalar(in.get("name")) == name && grafanaScalar(in.get("type")) == "datasource" {
			return strings.ToLower(grafanaScalar(in.get("pluginId")))
		}
	}
	return ""
}

// grafanaQueryPlugin reads a datasource variable's query: the plugin id, as
// a string or a {query: id} object.
func grafanaQueryPlugin(q *docNode) string {
	if q == nil {
		return ""
	}
	if q.kind == docMap {
		return grafanaScalar(q.get("query"))
	}
	return grafanaScalar(q)
}

// grafanaDefault is the dashboard's default datasource kind: Prometheus when
// every datasource variable and input it declares is Prometheus, else
// undecided.
func grafanaDefault(dash, root *docNode) string {
	var ids []string
	for _, v := range grafanaList(dash.get("templating").get("list")) {
		if grafanaScalar(v.get("type")) == "datasource" {
			ids = append(ids, strings.ToLower(grafanaQueryPlugin(v.get("query"))))
		}
	}
	for _, in := range grafanaList(root.get("__inputs")) {
		if grafanaScalar(in.get("type")) == "datasource" {
			ids = append(ids, strings.ToLower(grafanaScalar(in.get("pluginId"))))
		}
	}
	if len(ids) == 0 {
		return ""
	}
	for _, id := range ids {
		if id != "prometheus" {
			return ""
		}
	}
	return "prometheus"
}

func grafanaList(n *docNode) []*docNode {
	if n == nil || n.kind != docSeq {
		return nil
	}
	return n.items
}

func grafanaScalar(n *docNode) string {
	if n == nil || n.kind != docScalar {
		return ""
	}
	return n.scalar
}

// grafanaTemplated reports whether a value carries a template variable.
func grafanaTemplated(v string) bool {
	return strings.Contains(v, "$") || strings.Contains(v, "[[")
}

// grafanaVarAt reads a variable reference at the start of s: `$name`,
// `${name}`, `${name:fmt}`, `${name.field}` or `[[name]]`, `[[name:fmt]]`;
// n is the bytes it spans, 0 when s does not begin with one.
func grafanaVarAt(s string) (name string, n int) {
	switch {
	case strings.HasPrefix(s, "${"):
		end := strings.IndexByte(s, '}')
		if end < 0 {
			return "", 0
		}
		inner := s[2:end]
		if i := strings.IndexAny(inner, ":."); i >= 0 {
			inner = inner[:i]
		}
		if !grafanaVarIdent(inner) {
			return "", 0
		}
		return inner, end + 1
	case strings.HasPrefix(s, "[["):
		end := strings.Index(s, "]]")
		if end < 0 {
			return "", 0
		}
		inner := s[2:end]
		if i := strings.IndexByte(inner, ':'); i >= 0 {
			inner = inner[:i]
		}
		if !grafanaVarIdent(inner) {
			return "", 0
		}
		return inner, end + 2
	case strings.HasPrefix(s, "$"):
		i := 1
		for i < len(s) && (isAlnumByte(s[i]) || s[i] == '_') {
			i++
		}
		if i == 1 {
			return "", 0
		}
		return s[1:i], i
	}
	return "", 0
}

func grafanaVarIdent(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isAlnumByte(s[i]) && s[i] != '_' {
			return false
		}
	}
	return true
}

func isAlnumByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// grafanaSubstitute substitutes Grafana's macros and marks its template
// variables, outside strings only.
func grafanaSubstitute(expr string) string {
	var b strings.Builder
	var q byte
	for i := 0; i < len(expr); {
		c := expr[i]
		if q != 0 {
			b.WriteByte(c)
			i++
			if c == '\\' && q != '`' && i < len(expr) {
				b.WriteByte(expr[i])
				i++
				continue
			}
			if c == q {
				q = 0
			}
			continue
		}
		switch {
		case c == '"' || c == '\'' || c == '`':
			q = c
			b.WriteByte(c)
			i++
		case c == '$' || (c == '[' && strings.HasPrefix(expr[i:], "[[")):
			name, n := grafanaVarAt(expr[i:])
			if n == 0 {
				b.WriteByte(c)
				i++
				continue
			}
			b.WriteString(grafanaValue(name))
			i += n
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// grafanaValue is what a variable reads as: a macro's fixed duration or
// number, else the marked name.
func grafanaValue(name string) string {
	if strings.HasPrefix(name, "__") {
		switch {
		case strings.HasSuffix(name, "_ms"):
			return "300000"
		case strings.HasSuffix(name, "_s"):
			return "300"
		case name == "__from" || name == "__to":
			return "0"
		}
		return "5m"
	}
	return grafanaVarPrefix + name
}

// isTemplatedMetric reports whether a metric name a selector read was
// written by a template variable (marked, or inside a quoted name).
func isTemplatedMetric(m string) bool {
	return strings.Contains(m, grafanaVarPrefix) || grafanaTemplated(m)
}

// untemplatedName writes a marked name back as the variable it was.
func untemplatedName(m string) string {
	return strings.ReplaceAll(m, grafanaVarPrefix, "$")
}
