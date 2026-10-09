package promql

import (
	"reflect"
	"strings"
	"testing"
)

// sel is a compact expectation: the metric name and the matchers as
// `name<op>value`, in written order.
type sel struct {
	metric   string
	matchers []string
}

func shape(t *testing.T, expr string) []sel {
	t.Helper()
	got, err := Selectors(expr)
	if err != nil {
		t.Fatalf("Selectors(%q): %v", expr, err)
	}
	var out []sel
	for _, s := range got {
		if s.Start < 0 || s.End > len(expr) || s.Start >= s.End {
			t.Fatalf("Selectors(%q): span [%d,%d) outside the expression", expr, s.Start, s.End)
		}
		var ms []string
		for _, m := range s.Matchers {
			ms = append(ms, m.Name+m.Op+m.Value)
		}
		out = append(out, sel{s.Metric, ms})
	}
	return out
}

// TestSelectors: every vector selector of an expression, in written order,
// and nothing else: functions, aggregations and their by/without lists,
// on/ignoring/group_left/group_right lists, keywords, numbers, durations,
// strings and comments are never selectors (LB-F316).
func TestSelectors(t *testing.T) {
	cases := []struct {
		expr string
		want []sel
	}{
		{`up`, []sel{{"up", nil}}},
		{`up{job="api"}`, []sel{{"up", []string{"job=api"}}}},
		{`{__name__="up", job!="x"}`, []sel{{"up", []string{"__name__=up", "job!=x"}}}},
		{`{__name__=~"node_.*"}`, []sel{{"", []string{"__name__=~node_.*"}}}},
		{`http_requests_total{code=~"5..", method!~"GET|HEAD",}`, []sel{{"http_requests_total", []string{"code=~5..", "method!~GET|HEAD"}}}},
		{`rate(http_requests_total{job="api"}[5m])`, []sel{{"http_requests_total", []string{"job=api"}}}},
		{`sum by (job, instance) (rate(x_total[1m]))`, []sel{{"x_total", nil}}},
		{`sum(rate(x_total[1m])) without (instance)`, []sel{{"x_total", nil}}},
		{`a / on(job) group_left(version) b`, []sel{{"a", nil}, {"b", nil}}},
		{`a * ignoring(instance) group_right (env) b`, []sel{{"a", nil}, {"b", nil}}},
		{`a > bool 1`, []sel{{"a", nil}}},
		{`a and b or c unless d`, []sel{{"a", nil}, {"b", nil}, {"c", nil}, {"d", nil}}},
		{`AND`, nil},
		{`job:http_requests:rate5m{job="api"} > 0.5`, []sel{{"job:http_requests:rate5m", []string{"job=api"}}}},
		{`max_over_time(deriv(rate(x[1m])[5m:1m])[1h:])`, []sel{{"x", nil}}},
		{`x offset 5m`, []sel{{"x", nil}}},
		{`x offset -5m @ 1609746000`, []sel{{"x", nil}}},
		{`x @ start() - y @ end()`, []sel{{"x", nil}, {"y", nil}}},
		{`topk(5, x)`, []sel{{"x", nil}}},
		{`count_values("version", build_info)`, []sel{{"build_info", nil}}},
		{`quantile(0.9, x)`, []sel{{"x", nil}}},
		{`label_replace(up, "dst", "$1", "src", "(.*)")`, []sel{{"up", nil}}},
		{`absent(up{job="a"}) == 1`, []sel{{"up", []string{"job=a"}}}},
		{`vector(1) + scalar(x)`, []sel{{"x", nil}}},
		{`time() - process_start_time_seconds`, []sel{{"process_start_time_seconds", nil}}},
		{`1e3 * 0x1f + Inf - NaN + .5`, nil},
		{`-x`, []sel{{"x", nil}}},
		{"x # a comment with y{job=\"z\"}\n + w", []sel{{"x", nil}, {"w", nil}}},
		{`x{job="a\"b", path='c\'d', raw=` + "`e\\f`" + `}`, []sel{{"x", []string{`job=a"b`, `path=c'd`, `raw=e\f`}}}},
		{`x{job="a\nb\x41é"}`, []sel{{"x", []string{"job=a\nbAé"}}}},
		{`{"my.metric", "service.name"="checkout"}`, []sel{{"my.metric", []string{"service.name=checkout"}}}},
		{`sum by ("service.name") (x)`, []sel{{"x", nil}}},
		{`(a + b) / ignoring (code) c`, []sel{{"a", nil}, {"b", nil}, {"c", nil}}},
		{`sum(x) by (job) / on (job) group_left sum(y)`, []sel{{"x", nil}, {"y", nil}}},
		{`histogram_quantile(0.99, sum by (le) (rate(h_bucket[5m])))`, []sel{{"h_bucket", nil}}},
		{`group(x) by (on, by)`, []sel{{"x", nil}}},
		{`x{on="a", by="b", sum="c"}`, []sel{{"x", []string{"on=a", "by=b", "sum=c"}}}},
		{`limitk(2, x) + limit_ratio(0.5, y)`, []sel{{"x", nil}, {"y", nil}}},
		{`info(x, {k8s_cluster_name=~".+"})`, []sel{{"x", nil}, {"", []string{"k8s_cluster_name=~.+"}}}},
		{`x[5m] offset 1h`, []sel{{"x", nil}}},
		{`rate(x[1h30m])`, []sel{{"x", nil}}},
		{"sum(\n  rate(x{job=\"a\"}[5m])\n)\n", []sel{{"x", []string{"job=a"}}}},
		{`x atan2 y`, []sel{{"x", nil}, {"y", nil}}},
	}
	for _, c := range cases {
		got := shape(t, c.expr)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Selectors(%q)\n got %+v\nwant %+v", c.expr, got, c.want)
		}
	}
}

// TestSelectorsSpan: Start and End bound the written selector, braces
// included, the range and offset not.
func TestSelectorsSpan(t *testing.T) {
	expr := `rate(http_requests_total{job="api"}[5m]) + {__name__="up"} offset 1m`
	got, err := Selectors(expr)
	if err != nil || len(got) != 2 {
		t.Fatalf("Selectors = %v, %v", got, err)
	}
	if s := expr[got[0].Start:got[0].End]; s != `http_requests_total{job="api"}` {
		t.Errorf("span 0 = %q", s)
	}
	if s := expr[got[1].Start:got[1].End]; s != `{__name__="up"}` {
		t.Errorf("span 1 = %q", s)
	}
}

// TestSelectorsErrors: malformed input is an error, never a guess.
func TestSelectorsErrors(t *testing.T) {
	for _, expr := range []string{
		`x{job="a}`,
		`x{job="a"`,
		`x{job}`,
		`x{job=}`,
		`x{job=~a}`,
		`x[5m`,
		`"unterminated`,
		`x{job="a" job="b"}`,
		`sum by (job x)`,
		`x{job="\q"}`,
	} {
		if got, err := Selectors(expr); err == nil {
			t.Errorf("Selectors(%q) = %+v, want an error", expr, got)
		}
	}
}

// TestIdentityLabels: the universal default identifying set is data (K15),
// returned as a fresh copy.
func TestIdentityLabels(t *testing.T) {
	want := []string{"job", "service", "app", "destination_service", "destination_workload", "service_name", "namespace"}
	got := IdentityLabels()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IdentityLabels() = %v, want %v", got, want)
	}
	got[0] = "mutated"
	if IdentityLabels()[0] != "job" {
		t.Fatal("IdentityLabels returned shared storage")
	}
	if !strings.Contains(identityData, "destination_workload") {
		t.Fatal("the identity set is not the embedded data")
	}
}

// TestEqualityMatchers: the convenience view the doc engine reads.
func TestEqualityMatchers(t *testing.T) {
	got, err := Selectors(`x{job="a", namespace="n", code=~"5.."}`)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := got[0].Equal("job"); !ok || v != "a" {
		t.Errorf("Equal(job) = %q, %v", v, ok)
	}
	if _, ok := got[0].Equal("code"); ok {
		t.Error("Equal(code) read a regex matcher")
	}
	if _, ok := got[0].Equal("missing"); ok {
		t.Error("Equal(missing) = ok")
	}
}

// FuzzSelectors: the scanner never panics and every span lies inside the
// expression.
func FuzzSelectors(f *testing.F) {
	for _, s := range []string{`up`, `sum by (a) (rate(x{a="b"}[5m]))`, `{"a.b"}`, `x # c`, `x[5m:1m]`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, expr string) {
		got, err := Selectors(expr)
		if err != nil {
			return
		}
		for _, s := range got {
			if s.Start < 0 || s.End > len(expr) || s.Start >= s.End {
				t.Fatalf("span [%d,%d) outside %q", s.Start, s.End, expr)
			}
		}
	})
}
