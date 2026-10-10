package extractor

// obsdialects_test.go — goldens for the wave c3 observability dialects
// (LB-Q-alertmanager, LB-Q-grafana-dashboard; design §4 "Observability
// documents"): every fixture's entities, intra-file references, cross-file
// references, unresolved references, matcher sets and facts against its
// golden. A fixture is read as if it sat in monitoring/<name>. The goldens
// were written from reading the fixtures and checked line by line against
// them (design §13.2 step 10).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type obsGoldenMatchers struct {
	Entity   string   `json:"entity"`
	Edge     string   `json:"edge"`
	Matchers []string `json:"matchers"`
	Line     int      `json:"line"`
	Unparsed int      `json:"unparsed,omitempty"`
}

type obsGoldenFact struct {
	Entity string `json:"entity"`
	Key    string `json:"key"`
	Value  string `json:"value"`
	Line   int    `json:"line"`
}

type obsGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	ToKind   string `json:"to_kind,omitempty"`
	Line     int    `json:"line"`
}

type obsGolden struct {
	Symbols    []cfnGoldenSymbol      `json:"symbols"`
	CrossFile  []obsGoldenCross       `json:"cross_file"`
	Unresolved []promGoldenUnresolved `json:"unresolved,omitempty"`
	Matchers   []obsGoldenMatchers    `json:"matchers,omitempty"`
	Facts      []obsGoldenFact        `json:"facts,omitempty"`
}

// obsExtraction is one fixture's extraction in the golden's shape.
func obsExtraction(t *testing.T, x *DocExtractor, rel string, src []byte) obsGolden {
	t.Helper()
	res, err := x.ExtractDoc("svc", rel, src)
	if err != nil {
		t.Fatalf("%s: ExtractDoc: %v", rel, err)
	}
	got := obsGolden{Symbols: []cfnGoldenSymbol{}, CrossFile: []obsGoldenCross{}}
	for _, nd := range res.Nodes {
		if nd.Service != "svc" || nd.FilePath != rel {
			t.Fatalf("%s: plumbing changed: %+v", rel, nd)
		}
		got.Symbols = append(got.Symbols, cfnGoldenSymbol{Name: nd.Name, Kind: string(nd.Kind), Package: nd.Package,
			Line: nd.Line, EndLine: nd.EndLine, Signature: nd.Signature, Calls: nd.Calls, Tier: tierName(nd.Tier)})
	}
	for _, c := range res.CrossFile {
		got.CrossFile = append(got.CrossFile, obsGoldenCross{From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target,
			Strategy: c.Strategy, ToKind: c.ToKind, Line: c.Line})
	}
	for _, u := range res.Unresolved {
		got.Unresolved = append(got.Unresolved, promGoldenUnresolved{From: u.From, Edge: u.Edge, Raw: u.Raw, Reason: u.Reason, Line: u.Line})
	}
	for _, m := range res.Matchers {
		g := obsGoldenMatchers{Entity: m.Entity, Edge: m.Edge, Matchers: []string{}, Line: m.Line, Unparsed: m.Unparsed}
		for _, mm := range m.Matchers {
			g.Matchers = append(g.Matchers, mm.Name+mm.Op+mm.Value)
		}
		got.Matchers = append(got.Matchers, g)
	}
	for _, f := range res.Facts {
		got.Facts = append(got.Facts, obsGoldenFact{Entity: f.Entity, Key: f.Key, Value: f.Value, Line: f.Line})
	}
	return got
}

// checkObsGoldens runs every fixture of a dialect against its golden and
// returns how many it read.
func checkObsGoldens(t *testing.T, id string) int {
	t.Helper()
	x, err := NewDocDialectExtractor(id)
	if err != nil {
		t.Fatalf("NewDocDialectExtractor(%s): %v", id, err)
	}
	dir := filepath.Join("dialects", id, "testdata")
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
		got := obsExtraction(t, x, "monitoring/"+e.Name(), src)
		gj, _ := json.MarshalIndent(got, "", "  ")
		raw, err := os.ReadFile(fx + ".golden.json")
		if err != nil {
			t.Fatalf("read golden: %v", err)
		}
		var want obsGolden
		if err := json.Unmarshal(raw, &want); err != nil {
			t.Fatalf("%s.golden.json: %v", fx, err)
		}
		if want.Symbols == nil {
			want.Symbols = []cfnGoldenSymbol{}
		}
		if want.CrossFile == nil {
			want.CrossFile = []obsGoldenCross{}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: extraction differs from its golden\ngot:\n%s", e.Name(), gj)
		}
	}
	return n
}

// TestAlertmanagerDialectGoldens (LB-F362): the configuration, its route
// tree named by path with receiver, matchers and continue as data, the
// receivers by name and integration kind only (no setting, no secret), the
// inhibit rules, time intervals and template globs; a route's receiver and
// time intervals (L2); the matcher sets, both syntaxes, read by
// internal/promql; a malformed matcher unresolved. The fixtures are looped
// in the test's own body, so a plan row's reachable_from can name it.
func TestAlertmanagerDialectGoldens(t *testing.T) {
	if n := checkObsGoldens(t, "alertmanager"); n < 3 {
		t.Errorf("%d fixtures, want at least 3", n)
	}
}

// TestAlertmanagerDetectsByContent: the sniff claims a configuration (a
// top-level route and receivers, either order) and none of the neighbouring
// YAML: a Prometheus rule file or configuration, a Grafana notification
// policy file, an OpenTelemetry Collector configuration.
func TestAlertmanagerDetectsByContent(t *testing.T) {
	x, err := NewDocDialectExtractor("alertmanager")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		"route:\n  receiver: a\nreceivers:\n  - name: a\n",
		"global: {}\nreceivers:\n- name: a\nroute: {receiver: a}\n",
	} {
		if !x.DetectsContent([]byte(s)) {
			t.Errorf("sniff missed %q", s)
		}
	}
	for _, s := range []string{
		"groups:\n  - name: a\n    rules:\n      - alert: X\n        expr: up == 0\n",
		"scrape_configs:\n  - job_name: am\n    static_configs: [{targets: [alertmanager:9093]}]\n",
		"apiVersion: 1\npolicies:\n  - orgId: 1\n    receiver: grafana-default-email\n",
		"receivers:\n  otlp:\n    protocols: {grpc: {}}\nservice:\n  pipelines: {}\n",
	} {
		if x.DetectsContent([]byte(s)) {
			t.Errorf("sniff claimed %q", s)
		}
	}
}

// TestGrafanaDashboardDialectGoldens (LB-F363, LB-F364): a dashboard
// (classic, legacy rows, wrapped, v2), its rows, panels (nested in rows, a
// library panel by uid), template variables and links; a panel target's
// PromQL, read only for a Prometheus datasource, macros substituted, a
// templated metric refused and a Loki query counted; uid links, datasource
// uids and recording rules as entity-name references; provisioning: dashboard
// providers (repo-tree), datasources (no secret field read), alerting rule
// groups, rules, contact points (no setting read), policies, mute timings,
// templates. A JSON document that is no dashboard yields nothing.
func TestGrafanaDashboardDialectGoldens(t *testing.T) {
	if n := checkObsGoldens(t, "grafana-dashboard"); n < 6 {
		t.Errorf("%d fixtures, want at least 6", n)
	}
}
