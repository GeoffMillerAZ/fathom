package extractor

import (
	"reflect"
	"testing"
)

// TestGrafanaSubstitute: macros become a fixed duration or number, a
// template variable outside a string a marked name, one inside a string
// stays.
func TestGrafanaSubstitute(t *testing.T) {
	for in, want := range map[string]string{
		`rate(x{job=~"$job"}[$__rate_interval])`:    `rate(x{job=~"$job"}[5m])`,
		`rate(x[${__interval}]) * $__range_s`:       `rate(x[5m]) * 300`,
		`increase(x[[[__range]]]) / $__interval_ms`: `increase(x[5m]) / 300000`,
		`sum by ($group) (y) > ${threshold:raw}`:    `sum by (__grafana_var__group) (y) > __grafana_var__threshold`,
		`$metric{instance="[[node]]"}`:              `__grafana_var__metric{instance="[[node]]"}`,
		`a{x="\"$q"} or b`:                          `a{x="\"$q"} or b`,
		`up{job="$job"} offset $__auto`:             `up{job="$job"} offset 5m`,
		`$`:                                         `$`,
		`${}`:                                       `${}`,
	} {
		if got := grafanaSubstitute(in); got != want {
			t.Errorf("grafanaSubstitute(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestGrafanaPanelQueries (LB-F364): a panel target is scanned only when
// its datasource is Prometheus-typed (the target's, else the panel's, else
// the dashboard's default), a metric name a template variable writes is
// refused, Loki and expression queries are counted, never scanned.
func TestGrafanaPanelQueries(t *testing.T) {
	x := obsSpec(t, `{"decode": "yaml", "detect": {}, "entities": [
		{"select": "$..panels[*]", "where": {"is_map": true}, "kind": "element", "source_kind": "P", "name": "fmt:panel/{.id}"},
		{"select": "$.rules[*]", "where": {"is_map": true}, "kind": "rule", "source_kind": "A", "name": ".title"}],
		"cross_file": [
		{"from": "P", "select": "$.targets[*]", "value": ".expr", "promql": {"grafana": true}, "resolve": "entity-name", "to_kind": "PrometheusRecordingRule", "edge": "uses_record"},
		{"from": "A", "select": "$.data[*]", "value": ".model.expr", "promql": {"grafana": true}, "resolve": "entity-name", "to_kind": "PrometheusRecordingRule", "edge": "uses_record"}]}`)
	src := `{
  "__inputs": [{"name": "DS_PROM", "type": "datasource", "pluginId": "prometheus"}],
  "templating": {"list": [{"name": "datasource", "type": "datasource", "query": "prometheus"},
                          {"name": "logs", "type": "datasource", "query": "loki"}]},
  "panels": [
    {"id": 1, "datasource": {"type": "prometheus", "uid": "${datasource}"},
     "targets": [{"expr": "sum(rate(job:http:rate5m{job=~\"$job\"}[$__rate_interval]))"}]},
    {"id": 2, "datasource": "$datasource", "targets": [{"expr": "$metric{a=\"b\"} > 0"}]},
    {"id": 3, "datasource": "$logs", "targets": [{"expr": "{app=\"x\"} |= \"err\""}]},
    {"id": 4, "datasource": {"uid": "-- Mixed --"},
     "targets": [{"datasource": {"type": "loki"}, "expr": "{a=\"b\"}"},
                 {"datasource": "${DS_PROM}", "expr": "up"}]},
    {"id": 5, "datasource": {"uid": "webstore-metrics"}, "targets": [{"expr": "up"}]},
    {"id": 6, "type": "row", "panels": [{"id": 7, "targets": [{"expr": "node:load1:avg"}]}]}
  ],
  "rules": [{"title": "R", "data": [{"datasourceUid": "__expr__", "model": {"expr": "A > 1"}},
                                     {"datasourceUid": "p1", "model": {"datasource": {"type": "prometheus"}, "expr": "x:y:z"}}]}]
}`
	res, err := x.ExtractDoc("s", "d.json", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var refs, un []string
	for _, c := range res.CrossFile {
		refs = append(refs, c.From+" "+c.Raw)
	}
	for _, u := range res.Unresolved {
		un = append(un, u.From+" "+u.Reason+" "+u.Raw)
	}
	wantRefs := []string{"panel/1 job:http:rate5m", "panel/4 up", "R x:y:z"}
	wantUn := []string{
		"panel/2 template-variable $metric",
		"panel/3 datasource-not-prometheus {app=\"x\"} |= \"err\"",
		"panel/4 datasource-not-prometheus {a=\"b\"}",
		"panel/5 datasource-unknown up",
		"panel/7 datasource-unknown node:load1:avg",
		"R datasource-not-prometheus A > 1",
	}
	if !reflect.DeepEqual(refs, wantRefs) {
		t.Errorf("refs\n got %q\nwant %q", refs, wantRefs)
	}
	if !reflect.DeepEqual(un, wantUn) {
		t.Errorf("unresolved\n got %q\nwant %q", un, wantUn)
	}
}
