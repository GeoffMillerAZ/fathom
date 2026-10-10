package fusion

// alertroute_ingest_test.go — the alertmanager dialect through ingest and
// the observability joins (LB-F361, LB-F362, LB-F363; wave c3 item obs): the
// late-YAML hub claims a configuration by its name or its content; ingest
// stores the route tree, receivers, inhibit rules, time intervals and
// template globs; the doc-xref pass joins a route's and an inhibit rule's
// alertname to the alerting rule, and routes every alerting rule to the
// receivers its static labels reach (routes_to) or may reach (may_route_to);
// ReingestFile reads each file as ingest does.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/promql"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func promqlMatcher(name, op, v string) promql.Matcher {
	return promql.Matcher{Name: name, Op: op, Value: v}
}

func alertmanagerRepo() map[string]string {
	return map[string]string{
		"monitoring/alertmanager.yml": `global:
  resolve_timeout: 5m
templates:
  - templates/*.tmpl
route:
  receiver: default
  routes:
    - matchers: ['alertname = Watchdog']
      receiver: deadman
    - matchers: [severity="critical"]
      receiver: pager
      continue: true
    - matchers: ['team = storage']
      receiver: storage
    - match_re:
        severity: warning|info
      receiver: tickets
receivers:
  - name: default
  - name: deadman
    webhook_configs: [{url: 'https://deadman.example.org/ping'}]
  - name: pager
  - name: storage
  - name: tickets
inhibit_rules:
  - source_matchers: ['alertname = NodeDown']
    target_matchers: ['alertname = DiskFull']
    equal: [instance]
`,
		// claimed by its content alone
		"ops/am/routing.yaml": `route:
  receiver: ops
receivers:
  - name: ops
`,
		"monitoring/alerts.rules.yml": `groups:
  - name: node
    labels:
      team: storage
    rules:
      - alert: NodeDown
        expr: up{job="node"} == 0
        labels:
          severity: critical
      - alert: DiskFull
        expr: node_filesystem_avail_bytes == 0
        labels:
          severity: warning
  - name: meta
    rules:
      - alert: Watchdog
        expr: vector(1)
      - alert: Latency
        expr: histogram_quantile(0.9, rate(http_request_duration_seconds_bucket[5m])) > 1
        labels:
          severity: '{{ if gt $value 2.0 }}critical{{ else }}warning{{ end }}'
`,
		"monitoring/templates/default.tmpl": `{{ define "slack.title" }}{{ .CommonLabels.alertname }}{{ end }}
`,
	}
}

func obsIngest(t *testing.T, repo map[string]string, svc string) (*Pipeline, store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	writeCorpus(t, dir, repo)
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: svc, Path: dir}); err != nil {
		t.Fatal(err)
	}
	return p, st, dir
}

// obsResolution is the resolver's labelled answer over a repository, read
// the way ingest reads it (ObsXrefsForBench), as "from-file|from -> to-file|to edge".
func obsResolution(t *testing.T, p *Pipeline, dir, svc string) ([]string, ObsResolution) {
	t.Helper()
	refs, err := connector.NewLocalConnector().Discover(context.Background(), core.ServiceRef{ID: svc, Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	_, res := p.ObsXrefsForBench(svc, refs, func(rel string) ([]byte, error) {
		return os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	})
	var out []string
	for _, x := range res.Refs {
		out = append(out, x.FromFile+"|"+x.From+" -> "+x.ToFile+"|"+x.To+" "+x.Edge)
	}
	sort.Strings(out)
	return out, res
}

// TestIngest_AlertmanagerRepo (LB-F361, LB-F362, LB-F363): every
// Alertmanager entity is stored; an alertname matcher reaches the alerting
// rule; each alerting rule reaches the receivers its name and static labels
// route to, may when a matcher reads a label the rule leaves to runtime
// (Watchdog has no team, Latency's severity is templated); the template
// glob is kept, joined to nothing (no dialect reads a .tmpl file).
func TestIngest_AlertmanagerRepo(t *testing.T) {
	p, st, dir := obsIngest(t, alertmanagerRepo(), "am")
	kinds := crSourceKinds(t, st, "am")
	for rel, want := range map[string][]string{
		"monitoring/alertmanager.yml": {"AlertmanagerConfig", "AlertmanagerRoute", "AlertmanagerReceiver", "AlertmanagerInhibitRule", "AlertmanagerTemplate"},
		"ops/am/routing.yaml":         {"AlertmanagerConfig", "AlertmanagerRoute", "AlertmanagerReceiver"},
		"monitoring/alerts.rules.yml": {"PrometheusAlertingRule"},
	} {
		for _, k := range want {
			if !crHasKind(kinds[rel], k) {
				t.Errorf("%s read as %q, want %s among them", rel, kinds[rel], k)
			}
		}
	}
	obs, err := st.QueryObservations(context.Background(), "am", "")
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string]string{}
	for _, o := range obs {
		n := o.ToSymbolNode()
		if n.Kind == "AlertmanagerRoute" {
			calls[n.Name] = strings.Join(n.Calls, ",")
		}
	}
	if calls["route.routes[1]"] != "pager" {
		t.Errorf("route.routes[1] calls %q, want its receiver pager", calls["route.routes[1]"])
	}

	got, res := obsResolution(t, p, dir, "am")
	want := []string{
		"monitoring/alertmanager.yml|inhibit_rules[0] -> monitoring/alerts.rules.yml|DiskFull inhibit_target",
		"monitoring/alertmanager.yml|inhibit_rules[0] -> monitoring/alerts.rules.yml|NodeDown inhibit_source",
		"monitoring/alertmanager.yml|route.routes[0] -> monitoring/alerts.rules.yml|Watchdog alertname",
		// DiskFull: team storage (its group's), severity warning: not critical,
		// so storage, which ends the search before tickets
		"monitoring/alerts.rules.yml|DiskFull -> monitoring/alertmanager.yml|storage routes_to",
		"monitoring/alerts.rules.yml|DiskFull -> ops/am/routing.yaml|ops routes_to",
		// Latency: a templated severity, no team: every route below Watchdog may
		"monitoring/alerts.rules.yml|Latency -> monitoring/alertmanager.yml|default may_route_to",
		"monitoring/alerts.rules.yml|Latency -> monitoring/alertmanager.yml|pager may_route_to",
		"monitoring/alerts.rules.yml|Latency -> monitoring/alertmanager.yml|storage may_route_to",
		"monitoring/alerts.rules.yml|Latency -> monitoring/alertmanager.yml|tickets may_route_to",
		"monitoring/alerts.rules.yml|Latency -> ops/am/routing.yaml|ops routes_to",
		// NodeDown: critical, continue, then team storage
		"monitoring/alerts.rules.yml|NodeDown -> monitoring/alertmanager.yml|pager routes_to",
		"monitoring/alerts.rules.yml|NodeDown -> monitoring/alertmanager.yml|storage routes_to",
		"monitoring/alerts.rules.yml|NodeDown -> ops/am/routing.yaml|ops routes_to",
		// Watchdog: its own route, first, without continue
		"monitoring/alerts.rules.yml|Watchdog -> monitoring/alertmanager.yml|deadman routes_to",
		"monitoring/alerts.rules.yml|Watchdog -> ops/am/routing.yaml|ops routes_to",
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("observability joins\n got %q\nwant %q", got, want)
	}
	if res.MayRoutes != 4 || res.Rules != 4 || res.Configs != 2 {
		t.Errorf("disclosure = may %d rules %d configs %d, want 4, 4, 2", res.MayRoutes, res.Rules, res.Configs)
	}

	// the stored doc-xref edges carry every join
	edges := promEdges(t, st, "am")
	for _, e := range []string{
		"monitoring/alertmanager.yml|am::monitoring/alertmanager.route.routes[0]->am::monitoring/alerts.rules.Watchdog",
		"monitoring/alerts.rules.yml|am::monitoring/alerts.rules.NodeDown->am::monitoring/alertmanager.pager",
		"monitoring/alerts.rules.yml|am::monitoring/alerts.rules.Latency->am::monitoring/alertmanager.tickets",
	} {
		if !containsEdge(edges, e) {
			t.Errorf("missing stored edge %s\nstored: %q", e, edges)
		}
	}
	for _, e := range edges {
		if strings.Contains(e, "templates/*.tmpl->") {
			t.Errorf("the template glob joined something: %s", e)
		}
	}
}

// TestRouteAlertSemantics: Alertmanager's own matching, three-valued: the
// first matching child ends the search unless it continues; a route none of
// whose children matched is itself the match; the receiver is inherited; a
// regular expression is anchored.
func TestRouteAlertSemantics(t *testing.T) {
	m := func(name, op, v string) amMatcher {
		return newAMMatcher(promqlMatcher(name, op, v))
	}
	leaf := func(recv string, cont bool, ms ...amMatcher) *amRoute {
		return &amRoute{receiver: recv, cont: cont, matchers: ms}
	}
	root := &amRoute{receiver: "root", children: []*amRoute{
		leaf("a", false, m("severity", "=~", "crit")),    // anchored: never "critical"
		leaf("", false, m("team", "=", "x")),             // inherits root
		leaf("b", true, m("severity", "!=", "info")),     // continues
		leaf("c", false, m("service", "=", "db")),        // service: runtime
		leaf("d", false, m("severity", "=", "critical")), // after c's may
	}}
	c := amConfig{root: root}
	got := map[string]matchState{}
	for _, h := range routeAlert(c, amRule{labels: map[string]string{"alertname": "X", "severity": "critical", "team": "y"}}) {
		got[h.receiver] = h.state
	}
	want := map[string]matchState{"b": matchYes, "c": matchMay, "d": matchMay}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("routes = %v, want %v", got, want)
	}
	got = map[string]matchState{}
	for _, h := range routeAlert(c, amRule{labels: map[string]string{"alertname": "X", "severity": "info", "team": "x"}}) {
		got[h.receiver] = h.state
	}
	if want := map[string]matchState{"root": matchYes}; !reflect.DeepEqual(got, want) {
		t.Errorf("an inherited receiver: routes = %v, want %v", got, want)
	}
}

// TestReingestFile_AlertmanagerRepo: the one-file refresh reads each file as
// ingest does.
func TestReingestFile_AlertmanagerRepo(t *testing.T) {
	p, st, dir := obsIngest(t, alertmanagerRepo(), "am")
	reingestParity(t, p, st, dir, "am", alertmanagerRepo())
}

// reingestParity edits every file of repo, refreshes it alone, and checks
// what it reads as is unchanged.
func reingestParity(t *testing.T, p *Pipeline, st store.Store, dir, svc string, repo map[string]string) {
	t.Helper()
	before := crSourceKinds(t, st, svc)
	var rels []string
	for rel := range repo {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		edit := "\n# edited\n"
		if strings.HasSuffix(rel, ".json") {
			edit = "\n"
		}
		if err := os.WriteFile(path, append(b, []byte(edit)...), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(context.Background(), st, svc, dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	if after := crSourceKinds(t, st, svc); !reflect.DeepEqual(before, after) {
		t.Errorf("ReingestFile changed what a file reads as\nbefore: %v\n after: %v", before, after)
	}
}

// TestAMRulesStaticLabels: an alerting rule's static labels are its own and
// its group's, wherever the group writes them, a templated value left to
// runtime, and its name as alertname.
func TestAMRulesStaticLabels(t *testing.T) {
	x := lateYAMLLookup("prometheus")
	if x == nil {
		t.Fatal("no prometheus dialect")
	}
	src := `groups:
  - name: g
    rules:
      - alert: A
        expr: up == 0
        labels:
          severity: critical
          owner: '{{ $labels.team }}'
      - record: r:x
        expr: sum(x)
        labels:
          team: recording
    labels:
      team: storage
`
	res, err := x.ExtractDoc("s", "rules.yml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	files := []DocFile{{Path: "rules.yml", Result: res}}
	rules := amRules(newObsIndex(files), files)
	if len(rules) != 1 {
		t.Fatalf("rules = %+v, want A", rules)
	}
	want := map[string]string{"alertname": "A", "severity": "critical", "team": "storage"}
	if !reflect.DeepEqual(rules[0].labels, want) {
		t.Errorf("static labels = %v, want %v", rules[0].labels, want)
	}
}
