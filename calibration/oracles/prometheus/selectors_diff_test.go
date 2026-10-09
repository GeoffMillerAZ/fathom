package main

// selectors_diff_test.go — LB-F316's differential: fathom's pure-Go selector
// scanner (internal/promql) against the Prometheus parser's own
// ExtractSelectors (prometheus/promql/parser, Apache-2.0), over every PromQL
// expression of the calibration corpus. It lives in this nested, test-only
// module so the parser never becomes a dependency of fathom's main module
// (ADR-020, ADR-025): the main module stays pure, and this module imports it
// through a local replace.
//
//	FATHOM_PROMQL_CORPUS=<dir>[:<dir>...] go test -run TestSelectorsAgreeWithParser -v .
//
// An expression the parser refuses (a Grafana template variable, `$job`) is
// counted and left out: the parser is the oracle only where it parses. With
// no corpus set, the hand-written cases still run.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/prometheus/prometheus/promql/parser"
	"gopkg.in/yaml.v3"

	"github.com/geoffmilleraz/fathom/internal/promql"
)

// canon is one selector as a sorted matcher list, the metric name as its
// `__name__` equality matcher (the parser's form).
func canon(name, op, value string) string { return name + op + value }

func scannerSets(expr string) ([]string, error) {
	sels, err := promql.Selectors(expr)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, s := range sels {
		var ms []string
		hasName := false
		for _, m := range s.Matchers {
			ms = append(ms, canon(m.Name, m.Op, m.Value))
			hasName = hasName || (m.Name == promql.NameLabel && m.Op == promql.OpEqual)
		}
		if s.Metric != "" && !hasName {
			ms = append(ms, canon(promql.NameLabel, promql.OpEqual, s.Metric))
		}
		sort.Strings(ms)
		out = append(out, strings.Join(ms, ","))
	}
	sort.Strings(out)
	return out, nil
}

// promParser is the reference parser with every optional syntax on, so an
// expression any configuration of Prometheus accepts is compared.
var promParser = parser.NewParser(parser.Options{EnableExperimentalFunctions: true, ExperimentalDurationExpr: true, EnableExtendedRangeSelectors: true, EnableBinopFillModifiers: true})

func parserSets(expr string) ([]string, error) {
	e, err := promParser.ParseExpr(expr)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, set := range parser.ExtractSelectors(e) {
		var ms []string
		for _, m := range set {
			ms = append(ms, canon(m.Name, m.Type.String(), m.Value))
		}
		sort.Strings(ms)
		out = append(out, strings.Join(ms, ","))
	}
	sort.Strings(out)
	return out, nil
}

// exprsOf collects every string under an "expr" key of a decoded YAML or
// JSON tree, decoding a JSON object held in a string (a Grafana dashboard in
// a ConfigMap) on the way.
func exprsOf(v any, out *[]string) {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			if s, ok := x.(string); ok && k == "expr" {
				*out = append(*out, s)
				continue
			}
			exprsOf(x, out)
		}
	case []any:
		for _, x := range t {
			exprsOf(x, out)
		}
	case string:
		if s := strings.TrimSpace(t); strings.HasPrefix(s, "{") && strings.Contains(s, `"expr"`) {
			var inner any
			if json.Unmarshal([]byte(s), &inner) == nil {
				exprsOf(inner, out)
			}
		}
	}
}

// corpusExprs walks dirs for YAML and JSON files and returns every expr.
func corpusExprs(t *testing.T, dirs []string) []string {
	t.Helper()
	var out []string
	for _, d := range dirs {
		err := filepath.WalkDir(d, func(p string, de os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if de.IsDir() {
				if n := de.Name(); n == ".git" || n == "node_modules" || n == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(p))
			if ext != ".yml" && ext != ".yaml" && ext != ".json" {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil || !strings.Contains(string(b), "expr") {
				return nil
			}
			if ext == ".json" {
				var v any
				if json.Unmarshal(b, &v) == nil {
					exprsOf(v, &out)
				}
				return nil
			}
			dec := yaml.NewDecoder(strings.NewReader(string(b)))
			for {
				var v any
				if dec.Decode(&v) != nil {
					break
				}
				exprsOf(v, &out)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", d, err)
		}
	}
	sort.Strings(out)
	return out
}

// grafanaRange is a Grafana dashboard's range variable ($__rate_interval,
// $__interval, $__range, ${__rate_interval}), which stands for a duration.
var grafanaRange = regexp.MustCompile(`\$\{?__(rate_interval|interval|range)\}?`)

// diffCases are hand-written expressions that run without a corpus.
var diffCases = []string{
	`up`,
	`up{job="api"}`,
	`{__name__="up", job!="x"}`,
	`{"my.metric", "service.name"="checkout"}`,
	`sum by (job, instance) (rate(x_total{code=~"5.."}[1m])) / on(job) group_left(version) build_info`,
	`max_over_time(deriv(rate(x[1m])[5m:1m])[1h:]) offset 5m`,
	`x @ start() - y offset -5m`,
	`count_values("version", build_info) and topk(5, a) or quantile(0.9, b) unless c`,
	`label_replace(up, "dst", "$1", "src", "(.*)") # trailing comment`,
	`histogram_quantile(0.99, sum by (le) (rate(h_bucket{job="a\"b", path='c\'d'}[5m])))`,
	`job:http_requests:rate5m{job="api"} > bool 0.5`,
	`a * ignoring(instance) group_right (env) b`,
	`sum(x) by (job) / on (job) group_left sum(y)`,
	`absent(up{job="a"}) == 1 and vector(1) + scalar(z) - time()`,
	`limitk(2, x) + limit_ratio(0.5, y)`,
}

// TestSelectorsAgreeWithParser: on every expression the parser accepts, the
// scanner's selectors equal ExtractSelectors' as multisets of matcher sets.
func TestSelectorsAgreeWithParser(t *testing.T) {
	exprs := append([]string(nil), diffCases...)
	if env := os.Getenv("FATHOM_PROMQL_CORPUS"); env != "" {
		exprs = append(exprs, corpusExprs(t, filepath.SplitList(env))...)
	}
	var agree, refused, scannerOnly, templated int
	for _, e := range exprs {
		if g := grafanaRange.ReplaceAllString(e, "5m"); g != e {
			templated++ // a Grafana range variable, read as the duration it stands for
			e = g
		}
		want, perr := parserSets(e)
		got, serr := scannerSets(e)
		switch {
		case perr != nil:
			refused++
			if serr == nil {
				scannerOnly++
			}
			continue
		case serr != nil:
			t.Errorf("scanner refused an expression the parser reads: %q: %v", e, serr)
			continue
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("selectors differ for %q\nscanner: %q\nparser:  %q", e, got, want)
			continue
		}
		agree++
	}
	t.Logf("expressions %d (%d with a Grafana range variable read as 5m): agree %d, parser-refused %d (the scanner read %d of them), disagree %d", len(exprs), templated, agree, refused, scannerOnly, len(exprs)-agree-refused)
	if os.Getenv("FATHOM_PROMQL_REFUSED") != "" {
		for _, e := range exprs {
			if _, err := parserSets(grafanaRange.ReplaceAllString(e, "5m")); err != nil {
				t.Logf("refused: %q: %v", e, err)
			}
		}
	}
}
