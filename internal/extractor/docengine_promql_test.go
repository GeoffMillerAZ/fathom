package extractor

// docengine_promql_test.go — the doc engine's promql reference mode
// (LB-F317, design §4 "PromQL references"): a reference's or a cross-file
// reference's optional `promql` reads each value as a PromQL expression
// through internal/promql's selector scanner. An intra-file reference
// takes the selectors' metric names (a recording rule's `record:` used in
// another rule's expr, L2); a route-host cross-file reference takes each
// selector's equality matcher on an identifying label as one route value,
// scoped by the selector's own namespace matcher, else the reference's
// scope. Regex and negative matchers are never guessed: they are counted
// in DocResult.Unresolved. It is opt-in: every other dialect reads as
// before (their goldens run beside these, unchanged).

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

const promqlSpec = `{
  "decode": "yaml",
  "detect": {"require_keys": ["groups"]},
  "entities": [
    {"select": "$.groups[*]", "where": {"has_key": ["name"]}, "kind": "section", "source_kind": "PGroup", "name": ".name"},
    {"select": "$.groups[*].rules[*]", "where": {"has_key": ["record"]}, "kind": "rule", "source_kind": "PRecord", "name": ".record"},
    {"select": "$.groups[*].rules[*]", "where": {"has_key": ["alert"]}, "kind": "rule", "source_kind": "PAlert", "name": ".alert"}
  ],
  "references": [
    {"from": "PRecord|PAlert", "select": "$.expr", "value": "$", "promql": {}, "to_kind": "PRecord", "edge": "uses_record", "declared_only": true}
  ],
  "cross_file": [
    {"from": "rule", "select": "$.expr", "value": "$", "promql": {}, "resolve": "route-host", "edge": "route_ref", "scope": "$.namespace"}
  ]
}`

const promqlDoc = `namespace: shop
groups:
- name: api
  rules:
  - record: job:http_requests:rate5m
    expr: sum by (job) (rate(http_requests_total{job="checkout"}[5m]))
  - record: job:errors:rate5m
    expr: |
      sum by (job) (rate(http_requests_total{job="checkout", code=~"5.."}[5m]))
  - alert: CheckoutErrors
    expr: job:errors:rate5m / job:http_requests:rate5m > 0.05
  - alert: CartDown
    expr: up{job="cart", namespace="store"} == 0 or absent(up{service="payments.billing.svc.cluster.local"})
  - alert: AnyFrontend
    expr: up{job=~"frontend.*"} == 0 and up{app!="admin"} == 0
  - alert: Broken
    expr: up{job="a"
  - alert: ByName
    expr: '{__name__="up", job="", app="web"} == 0'
`

func promqlExtract(t *testing.T, spec, doc string) DocResult {
	t.Helper()
	s, err := ParseDocSpec([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	x, err := NewDocExtractor(s)
	if err != nil {
		t.Fatal(err)
	}
	res, err := x.ExtractDoc("svc", "monitoring/rules.yml", []byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// TestDocPromQLMetricRefs: an intra-file promql reference records the
// metric names an expression reads that name a declared entity of its
// to_kind: a recording rule used by another rule (L2), never a raw metric.
func TestDocPromQLMetricRefs(t *testing.T) {
	res := promqlExtract(t, promqlSpec, promqlDoc)
	calls := map[string][]string{}
	for _, n := range res.Nodes {
		calls[n.Name] = n.Calls
	}
	want := map[string][]string{
		"api":                      nil,
		"job:http_requests:rate5m": nil,
		"job:errors:rate5m":        nil,
		"CheckoutErrors":           {"job:errors:rate5m", "job:http_requests:rate5m"},
		"CartDown":                 nil,
		"AnyFrontend":              nil,
		"Broken":                   nil,
		"ByName":                   nil,
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v\nwant %v", calls, want)
	}
}

// TestDocPromQLRouteRefs: each selector's equality matcher on an
// identifying label is one route-host value, joined with the namespace it
// is scoped by (the selector's own matcher, else the reference's scope) in
// the DNS form the route resolver reads (`name.ns`); a dotted value is kept
// as written; regex, negative and unparsed are counted, never guessed.
func TestDocPromQLRouteRefs(t *testing.T) {
	res := promqlExtract(t, promqlSpec, promqlDoc)
	var got []string
	for _, c := range res.CrossFile {
		if c.Strategy != "route-host" || c.Edge != "route_ref" || c.Target != c.Raw {
			t.Fatalf("route contract broken: %+v", c)
		}
		got = append(got, c.From+" "+c.Raw+" scope="+c.Scope+" "+labelString(c.Labels))
	}
	want := []string{
		"job:http_requests:rate5m checkout.shop scope=shop job=checkout",
		"job:errors:rate5m checkout.shop scope=shop job=checkout",
		"CartDown cart.store scope=store job=cart,namespace=store",
		"CartDown payments.billing.svc.cluster.local scope=shop service=payments.billing.svc.cluster.local",
		"ByName web.shop scope=shop app=web",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("route refs:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	var un []string
	for _, u := range res.Unresolved {
		un = append(un, u.From+" "+u.Raw+" "+u.Reason)
	}
	wantUn := []string{
		"AnyFrontend job=~\"frontend.*\" regex-matcher",
		"AnyFrontend app!=\"admin\" negative-matcher",
		"Broken up{job=\"a\" unparsed",
	}
	if !reflect.DeepEqual(un, wantUn) {
		t.Fatalf("unresolved:\n%s\nwant:\n%s", strings.Join(un, "\n"), strings.Join(wantUn, "\n"))
	}
}

// TestDocPromQLLabels: a spec's own label set replaces the default
// (K15: the convention is data), and without a scope a bare value is kept
// as written.
func TestDocPromQLLabels(t *testing.T) {
	spec := strings.Replace(promqlSpec, `"promql": {}, "resolve"`, `"promql": {"labels": ["app"]}, "resolve"`, 1)
	spec = strings.Replace(spec, `, "scope": "$.namespace"`, "", 1)
	res := promqlExtract(t, spec, promqlDoc)
	var got []string
	for _, c := range res.CrossFile {
		got = append(got, c.From+" "+c.Raw)
	}
	if want := []string{"ByName web"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("route refs = %v, want %v", got, want)
	}
	if len(res.Unresolved) != 2 { // app!="admin" and the unparsed expr
		t.Fatalf("unresolved = %+v", res.Unresolved)
	}
}

// TestDocPromQLSpecErrors: the mode's misuses are refused by name.
func TestDocPromQLSpecErrors(t *testing.T) {
	for name, c := range map[string]struct{ from, to string }{
		"labels on an intra-file reference": {`"promql": {}, "to_kind"`, `"promql": {"labels": ["job"]}, "to_kind"`},
		"promql with a pattern":             {`"promql": {}, "to_kind"`, `"promql": {}, "pattern": "(x)", "to_kind"`},
		"promql off route-host":             {`"resolve": "route-host"`, `"resolve": "service-name"`},
		"promql cross with a pattern":       {`"resolve": "route-host"`, `"resolve": "route-host", "pattern": "(x)"`},
		"an empty label":                    {`"promql": {}, "resolve"`, `"promql": {"labels": [""]}, "resolve"`},
	} {
		spec := strings.Replace(promqlSpec, c.from, c.to, 1)
		if spec == promqlSpec {
			t.Fatalf("%s: the case edits nothing", name)
		}
		s, err := ParseDocSpec([]byte(spec))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := NewDocExtractor(s); err == nil || !strings.Contains(err.Error(), "promql") {
			t.Errorf("%s: NewDocExtractor = %v, want a promql error", name, err)
		}
	}
}

func labelString(m map[string]string) string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for i, k := range ks {
		ks[i] = k + "=" + m[k]
	}
	return strings.Join(ks, ",")
}
