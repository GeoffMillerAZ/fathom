package extractor

// docengine_promql_dialect_test.go — goldens and claims for the prometheus
// doc-engine dialect (LB-Q-prometheus, LB-F319, LB-F320; design §4 "PromQL
// references"): a rule file's groups and alerting and recording rules, a
// configuration's scrape jobs with their TelemetryIdentity, a promtool test
// file's test cases; a recording rule another rule's expr reads (L2); the
// rule_files a configuration or a test names (L3, repo-path); a scrape job's
// static targets and a rule's identifying-label matchers as route targets
// (route-host), regex and negative matchers counted unresolved. A fixture is
// read as if it sat in monitoring/<name>. Goldens are hand-written from
// reading the fixtures (design §13.2 step 10).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

type promGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
	Scope    string `json:"scope,omitempty"`
	Labels   string `json:"labels,omitempty"`
}

type promGoldenUnresolved struct {
	From   string `json:"from"`
	Edge   string `json:"edge"`
	Raw    string `json:"raw"`
	Reason string `json:"reason"`
	Line   int    `json:"line"`
}

type promGolden struct {
	Symbols    []cfnGoldenSymbol      `json:"symbols"`
	CrossFile  []promGoldenCross      `json:"cross_file"`
	Unresolved []promGoldenUnresolved `json:"unresolved,omitempty"`
}

// TestPrometheusDialectGoldens: every fixture's entities, record references,
// cross-file references, route targets and unresolved matchers against its
// hand-written golden. The fixtures are looped in the test's own body, not
// subtests, so a plan row's reachable_from can name this test.
func TestPrometheusDialectGoldens(t *testing.T) {
	x, err := NewDocDialectExtractor("prometheus")
	if err != nil {
		t.Fatalf("NewDocDialectExtractor(prometheus): %v", err)
	}
	dir := filepath.Join("dialects", "prometheus", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range ents {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".golden.json") {
			continue
		}
		n++
		fx := filepath.Join(dir, e.Name())
		src, err := os.ReadFile(fx)
		if err != nil {
			t.Fatal(err)
		}
		if lines := strings.Count(string(src), "\n"); lines > 200 {
			t.Fatalf("%s has %d lines, the recipe's cap is 200", fx, lines)
		}
		rel := "monitoring/" + e.Name()
		res, err := x.ExtractDoc("svc", rel, src)
		if err != nil {
			t.Fatalf("%s: ExtractDoc: %v", fx, err)
		}
		got := promGolden{Symbols: []cfnGoldenSymbol{}, CrossFile: []promGoldenCross{}}
		for _, nd := range res.Nodes {
			if nd.Service != "svc" || nd.FilePath != rel {
				t.Fatalf("%s: plumbing changed: %+v", fx, nd)
			}
			got.Symbols = append(got.Symbols, cfnGoldenSymbol{Name: nd.Name, Kind: string(nd.Kind), Package: nd.Package,
				Line: nd.Line, EndLine: nd.EndLine, Signature: nd.Signature, Calls: nd.Calls, Tier: tierName(nd.Tier)})
		}
		for _, c := range res.CrossFile {
			got.CrossFile = append(got.CrossFile, promGoldenCross{From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target,
				Strategy: c.Strategy, Line: c.Line, Scope: c.Scope, Labels: labelString(c.Labels)})
		}
		for _, u := range res.Unresolved {
			got.Unresolved = append(got.Unresolved, promGoldenUnresolved{From: u.From, Edge: u.Edge, Raw: u.Raw, Reason: u.Reason, Line: u.Line})
		}
		raw, err := os.ReadFile(fx + ".golden.json")
		if err != nil {
			t.Fatalf("read golden: %v", err)
		}
		var want promGolden
		if err := json.Unmarshal(raw, &want); err != nil {
			t.Fatalf("%s.golden.json: %v", fx, err)
		}
		if want.Symbols == nil {
			want.Symbols = []cfnGoldenSymbol{}
		}
		if want.CrossFile == nil {
			want.CrossFile = []promGoldenCross{}
		}
		if !reflect.DeepEqual(got, want) {
			gj, _ := json.MarshalIndent(got, "", "  ")
			t.Errorf("%s: extraction differs from its golden\ngot:\n%s", e.Name(), gj)
		}
	}
	if n < 3 {
		t.Errorf("%d fixtures, want at least 3", n)
	}
}

// TestPrometheusDetectsByContent: the content sniff claims a rule file (a
// top-level groups whose rules write alert or record), a configuration
// (scrape_configs or rule_files at the top level) and a promtool test file,
// and none of the neighbouring YAML: Grafana's alerting provisioning (groups
// of uid/title rules), an OpenTelemetry Collector configuration (a
// prometheus receiver's indented scrape_configs), an Alertmanager
// configuration, a PrometheusRule custom resource (its groups under spec,
// which the kubernetes side pass reads), a GitLab CI file, a stray tests key.
func TestPrometheusDetectsByContent(t *testing.T) {
	x, err := NewDocDialectExtractor("prometheus")
	if err != nil {
		t.Fatal(err)
	}
	yes := []string{
		"groups:\n  - name: a\n    rules:\n      - alert: X\n        expr: up == 0\n",
		"groups:\n- name: a\n  rules:\n  - record: job:x:rate5m\n    expr: rate(x[5m])\n",
		"# rules\ngroups:\n  - name: a\n    rules:\n      - expr: up == 0\n        alert: X\n",
		"global:\n  scrape_interval: 15s\nscrape_configs:\n  - job_name: p\n",
		"rule_files:\n  - a.yml\n",
		"rule_files: [a.yml]\ntests:\n  - interval: 1m\n",
	}
	no := []string{
		"apiVersion: 1\ngroups:\n  - orgId: 1\n    name: g\n    folder: f\n    rules:\n      - uid: a\n        title: T\n        condition: B\n",
		"receivers:\n  prometheus:\n    config:\n      scrape_configs:\n        - job_name: otel\n",
		"route:\n  receiver: default\nreceivers:\n  - name: default\n",
		"apiVersion: monitoring.coreos.com/v1\nkind: PrometheusRule\nmetadata:\n  name: r\nspec:\n  groups:\n  - name: g\n    rules:\n    - alert: X\n      expr: up == 0\n",
		"stages: [build]\nbuild:\n  script: make\n",
		"tests:\n  - name: a\n",
		"groups:\n  - name: admins\n    members: [a, b]\n",
	}
	for _, s := range yes {
		if !x.DetectsContent([]byte(s)) {
			t.Errorf("sniff missed %q", s)
		}
	}
	for _, s := range no {
		if x.DetectsContent([]byte(s)) {
			t.Errorf("sniff claimed %q", s)
		}
	}
}

// TestPrometheusGlobsLeaveOtherClaims: the manifest's basenames and globs
// claim Prometheus' conventional names, and every path another manifest
// claims by name keeps its verdict (classification is first-match by
// basename, then glob, so a glob of this dialect that matched an earlier
// claim would steal it: the gateway and gitlab-ci verdicts).
func TestPrometheusGlobsLeaveOtherClaims(t *testing.T) {
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	var all, others []connector.DialectRule
	for _, d := range ds {
		r := connector.DialectRule{ID: d.ID, Basenames: d.Match.Basenames, Globs: d.Match.Globs, Shebangs: d.Match.Shebangs}
		r.Extensions = append(r.Extensions, d.Match.Extensions...)
		for _, v := range d.Variants {
			r.Extensions = append(r.Extensions, v.Extensions...)
		}
		if s := d.Match.Sniff; s != nil {
			r.Sniff = &connector.DialectSniffRule{Extensions: s.Extensions, Pattern: s.Pattern, Priority: s.Priority}
		}
		all = append(all, r)
		if d.ID != "prometheus" {
			others = append(others, r)
		}
	}
	for _, p := range []string{"prometheus.yml", "deploy/prometheus.yaml", "monitoring/alert.rules.yml", "rules.d/node.rules.yaml",
		"prometheus/recording_rules.yml", "alerting_rules.yaml"} {
		if got := connector.ClassifyDialect(all, p, nil).Dialect; got != "prometheus" {
			t.Errorf("%s: classified %q, want prometheus", p, got)
		}
	}
	// Paths the other manifests claim by name (their documented shapes), and
	// every fixture of every other dialect read at the same paths.
	paths := []string{
		"rules/a.yml", "conf/rules/b.yaml", "dynamic.yml", "dynamic-conf.yaml", "traefik/x.yml", "traefik.yml", "traefik-dynamic.yml",
		"envoy.yaml", "deploy/front-envoy.yaml", "envoy/cds.yml", "kong.yml", "deck/x.yaml", "kong-config.yaml", "gateway-kong.yml",
		".gitlab-ci.yml", ".gitlab/ci/build.yml", "x.gitlab-ci.yml", "chart/templates/a.yaml", "docker-compose.yml",
		"compose.prod.yaml", ".github/workflows/ci.yml", "Pulumi.dev.yaml", "Taskfile.yml", "serverless.yml",
	}
	for _, d := range ds {
		if d.ID == "prometheus" {
			continue
		}
		ents, _ := os.ReadDir(filepath.Join("dialects", d.ID, "testdata"))
		for _, e := range ents {
			if !e.IsDir() && !strings.HasSuffix(e.Name(), ".golden.json") {
				paths = append(paths, e.Name(), "infra/"+e.Name())
			}
		}
	}
	sort.Strings(paths)
	for _, p := range paths {
		before := connector.ClassifyDialect(others, p, nil).Dialect
		if before == "" {
			continue
		}
		if after := connector.ClassifyDialect(all, p, nil).Dialect; after != before {
			t.Errorf("%s: %s's claim became %q", p, before, after)
		}
	}
}
