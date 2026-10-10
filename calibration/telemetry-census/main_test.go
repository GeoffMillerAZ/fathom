package main

import (
	"encoding/json"
	"testing"
)

// TestClassify: a source literal names a metric through Prometheus' series
// suffixes and the OpenTelemetry translation (E); a platform prefix or name
// is P; anything else U.
func TestClassify(t *testing.T) {
	var cl classes
	if err := json.Unmarshal(classesJSON, &cl); err != nil {
		t.Fatal(err)
	}
	lit := map[string]bool{}
	for _, l := range []string{"demo.cart.add_item.latency", "demo.recommendation.requests", "app_frontend_requests_total", "thanos_objstore_bucket_operations_total"} {
		lit[normalize(l)] = true
	}
	for name, want := range map[string]string{
		"demo_cart_add_item_latency_seconds_bucket": "E",
		"demo_recommendation_requests_total":        "E",
		"app_frontend_requests_total":               "E",
		"thanos_objstore_bucket_operations_total":   "E",
		"thanos_sidecar_prometheus_up":              "P",
		"node_cpu_seconds_total":                    "P",
		"up":                                        "P",
		"upstream_latency":                          "U",
		"job:http_requests:rate5m":                  "U",
		"checkout_orders_total":                     "U",
	} {
		if got := classify(name, lit, cl); got != want {
			t.Errorf("classify(%s) = %s, want %s", name, got, want)
		}
	}
}

// TestYAMLExprs: a rule file, a PrometheusRule, a Grafana alert rule and a
// dashboard in a ConfigMap each give their expressions.
func TestYAMLExprs(t *testing.T) {
	src := `groups:
- name: a
  rules:
  - record: job:x:rate5m
    expr: rate(x[5m])
---
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
spec:
  groups:
  - name: b
    rules:
    - alert: Down
      expr: up == 0
---
apiVersion: 1
groups:
- name: g
  rules:
  - uid: u
    data:
    - model:
        expr: sum(rate(y_total[1m]))
---
kind: ConfigMapList
items:
- kind: ConfigMap
  data:
    d.json: '{"panels":[{"targets":[{"expr":"z"}]}]}'
`
	rec := map[string]bool{}
	got := yamlExprs("f.yaml", []byte(src), rec)
	var texts []string
	for _, e := range got {
		texts = append(texts, e.Source+":"+e.Text)
	}
	want := []string{"rule:rate(x[5m])", "rule:up == 0", "grafana-alert:sum(rate(y_total[1m]))", "dashboard:z"}
	if len(texts) != len(want) {
		t.Fatalf("exprs = %q, want %q", texts, want)
	}
	for i := range want {
		if texts[i] != want[i] {
			t.Errorf("expr %d = %q, want %q", i, texts[i], want[i])
		}
	}
	if !rec["job:x:rate5m"] {
		t.Error("the record name was not collected")
	}
}

// TestDegrafana: Grafana's variables outside a string are read as PromQL, a
// variable inside one is left alone.
func TestDegrafana(t *testing.T) {
	for in, want := range map[string]string{
		`rate(x{job="$job"}[$__rate_interval])`:        `rate(x{job="$job"}[5m])`,
		`sum(rate(x[${__interval}])) * $__interval_ms`: `sum(rate(x[5m])) * 1`,
		`topk($n, sum by ($label) (y))`:                `topk(grafana_var__, sum by (grafana_var__) (y))`,
		`up{instance=~"${host:regex}"}`:                `up{instance=~"${host:regex}"}`,
	} {
		if got, _ := degrafana(in); got != want {
			t.Errorf("degrafana(%q) = %q, want %q", in, got, want)
		}
	}
}
