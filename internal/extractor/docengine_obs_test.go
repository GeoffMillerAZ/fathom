package extractor

// docengine_obs_test.go — the doc engine's wave c3 additions (LB-F361,
// design §4 "Observability documents"): `$ipath` names a node by its keys and
// sequence indexes; signature_fields and signature_keys sign an entity with
// the fields it writes (keys by name only); matchers read an entity's label
// matchers through internal/promql; facts keep an entity's scalars as data;
// the entity-name and repo-tree cross-file strategies, and a promql
// cross-file reference that reads metric names for entity-name.

import (
	"reflect"
	"strings"
	"testing"
)

func obsSpec(t *testing.T, js string) *DocExtractor {
	t.Helper()
	spec, err := ParseDocSpec([]byte(js))
	if err != nil {
		t.Fatalf("ParseDocSpec: %v", err)
	}
	x, err := NewDocExtractor(spec)
	if err != nil {
		t.Fatalf("NewDocExtractor: %v", err)
	}
	return x
}

const obsRouteDoc = `route:
  receiver: default
  group_by: [alertname, job]
  routes:
    - matchers: ['severity = critical', 'team=~"a|b"']
      receiver: pager
      continue: true
      routes:
        - match: {service: db}
          match_re: {env: prod|staging}
          receiver: dba
    - matchers: ['alertname = Watchdog']
      receiver: "null"
receivers:
  - name: default
    email_configs: [{to: a@example.org}]
    slack_configs: [{api_url: https://hooks.example.org/x}]
  - name: pager
  - name: dba
  - name: "null"
inhibit_rules:
  - source_matchers: ['severity = critical']
    target_matchers: [severity = warning]
    equal: [alertname]
`

// TestDocIPathNamesRouteTree: `$ipath` is the dotted keys and sequence
// indexes from the selection's start, so a recursive route tree names every
// node uniquely (`route.routes[0].routes[0]`).
func TestDocIPathNamesRouteTree(t *testing.T) {
	x := obsSpec(t, `{"decode": "yaml", "detect": {}, "entities": [
		{"select": "$.route", "where": {"is_map": true}, "kind": "route", "source_kind": "R", "name": "$ipath"},
		{"select": "$.route..routes[*]", "where": {"is_map": true}, "kind": "route", "source_kind": "R", "name": "$ipath"},
		{"select": "$.inhibit_rules[*]", "kind": "rule", "source_kind": "I", "name": "$ipath"}]}`)
	res, err := x.ExtractDoc("s", "am/alertmanager.yml", []byte(obsRouteDoc))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range res.Nodes {
		got = append(got, n.Name)
	}
	want := []string{"route", "route.routes[0]", "route.routes[0].routes[0]", "route.routes[1]", "inhibit_rules[0]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %q, want %q", got, want)
	}
}

// TestDocSignatureFieldsAndKeys: fields sign with their values (a scalar,
// a sequence, a map), keys by name only, so a receiver's integrations are
// named and their settings never read.
func TestDocSignatureFieldsAndKeys(t *testing.T) {
	x := obsSpec(t, `{"decode": "yaml", "detect": {}, "entities": [
		{"select": "$.route..routes[*]", "where": {"is_map": true}, "kind": "route", "source_kind": "R", "name": "$ipath",
		 "signature_fields": ["receiver", "matchers", "match", "match_re", "continue", "group_by"]},
		{"select": "$.receivers[*]", "kind": "target", "source_kind": "Recv", "name": ".name",
		 "signature_keys": ["email_configs", "pagerduty_configs", "slack_configs"]}]}`)
	res, err := x.ExtractDoc("s", "alertmanager.yml", []byte(obsRouteDoc))
	if err != nil {
		t.Fatal(err)
	}
	sig := map[string]string{}
	for _, n := range res.Nodes {
		sig[n.Name] = n.Signature
		if strings.Contains(n.Signature, "hooks.example.org") || strings.Contains(n.Signature, "a@example.org") {
			t.Errorf("%s signs a setting value: %q", n.Name, n.Signature)
		}
	}
	for name, want := range map[string]string{
		"route.routes[0]":           `receiver=pager; matchers=[severity = critical, team=~"a|b"]; continue=true`,
		"route.routes[0].routes[0]": `receiver=dba; match={service: db}; match_re={env: prod|staging}`,
		"default":                   "email_configs, slack_configs",
		"pager":                     "",
	} {
		if sig[name] != want {
			t.Errorf("%s signature = %q, want %q", name, sig[name], want)
		}
	}
}

// TestDocMatchersAndFacts: an entity's matchers, both syntaxes, read
// through internal/promql and grouped by edge; a malformed one is
// unresolved, never guessed; a fact is a scalar the entity writes, a map
// fact one per entry.
func TestDocMatchersAndFacts(t *testing.T) {
	x := obsSpec(t, `{"decode": "yaml", "detect": {}, "entities": [
		{"select": "$.route", "where": {"is_map": true}, "kind": "route", "source_kind": "R", "name": "$ipath"},
		{"select": "$.route..routes[*]", "where": {"is_map": true}, "kind": "route", "source_kind": "R", "name": "$ipath"},
		{"select": "$.inhibit_rules[*]", "kind": "rule", "source_kind": "I", "name": "$ipath"}],
		"matchers": [
		{"from": "R", "select": "$.matchers[*]", "syntax": "alertmanager", "edge": "match"},
		{"from": "R", "select": "$.match", "op": "=", "edge": "match"},
		{"from": "R", "select": "$.match_re", "op": "=~", "edge": "match"},
		{"from": "I", "select": "$.source_matchers[*]", "syntax": "alertmanager", "edge": "source"},
		{"from": "I", "select": "$.target_matchers[*]", "syntax": "alertmanager", "edge": "target"}],
		"facts": [
		{"from": "R", "select": "$.receiver", "key": "receiver"},
		{"from": "R", "select": "$.continue", "key": "continue"},
		{"from": "R", "select": "$.match", "key": "match", "map": true}]}`)
	doc := obsRouteDoc + "  - source_matchers: ['severity = \"crit']\n"
	res, err := x.ExtractDoc("s", "alertmanager.yml", []byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	var sets []string
	for _, m := range res.Matchers {
		var ms []string
		for _, mm := range m.Matchers {
			ms = append(ms, mm.Name+mm.Op+mm.Value)
		}
		sets = append(sets, m.Entity+" "+m.Edge+" "+strings.Join(ms, ","))
	}
	wantSets := []string{
		"route.routes[0] match severity=critical,team=~a|b",
		"route.routes[0].routes[0] match service=db,env=~prod|staging",
		"route.routes[1] match alertname=Watchdog",
		"inhibit_rules[0] source severity=critical",
		"inhibit_rules[0] target severity=warning",
		"inhibit_rules[1] source ",
	}
	if !reflect.DeepEqual(sets, wantSets) {
		t.Errorf("matcher sets\n got %q\nwant %q", sets, wantSets)
	}
	if len(res.Unresolved) != 1 || res.Unresolved[0].From != "inhibit_rules[1]" || res.Unresolved[0].Reason != PromQLUnparsed {
		t.Errorf("unresolved = %+v, want inhibit_rules[1]'s malformed matcher unparsed", res.Unresolved)
	}
	var facts []string
	for _, f := range res.Facts {
		facts = append(facts, f.Entity+" "+f.Key+"="+f.Value)
	}
	wantFacts := []string{
		"route receiver=default",
		"route.routes[0] receiver=pager",
		"route.routes[0] continue=true",
		"route.routes[0].routes[0] match.service=db",
		"route.routes[0].routes[0] receiver=dba",
		"route.routes[1] receiver=null",
	}
	if !reflect.DeepEqual(facts, wantFacts) {
		t.Errorf("facts\n got %q\nwant %q", facts, wantFacts)
	}
}

// TestDocEntityNameCrossFile: an entity-name reference keeps its value and
// target kind for fusion's resolver; with promql it reads every metric name
// an expression's selectors write.
func TestDocEntityNameCrossFile(t *testing.T) {
	x := obsSpec(t, `{"decode": "yaml", "detect": {}, "entities": [
		{"select": "$.panels[*]", "kind": "element", "source_kind": "P", "name": ".title"}],
		"cross_file": [
		{"from": "P", "select": "$.targets[*]", "value": ".expr", "promql": {}, "resolve": "entity-name", "to_kind": "PrometheusRecordingRule", "edge": "uses_record"},
		{"from": "P", "select": "$.links[*].url", "value": "$", "pattern": "/d/([A-Za-z0-9_-]+)", "resolve": "entity-name", "to_kind": "GrafanaDashboard", "edge": "links_to"},
		{"from": "P", "select": "$.dir", "value": "$", "resolve": "repo-tree", "to_kind": "GrafanaDashboard", "edge": "provides"}]}`)
	src := `panels:
  - title: cpu
    targets:
      - expr: sum(node:cpu:rate5m{job="node"}) / count(up)
      - expr: rate(x[
    links:
      - url: /d/abc-1/overview?orgId=1
    dir: ../dashboards
`
	res, err := x.ExtractDoc("s", "grafana/provisioning/p.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range res.CrossFile {
		got = append(got, c.From+" "+c.Edge+" "+c.Strategy+" "+c.ToKind+" "+c.Raw+" -> "+c.Target)
	}
	want := []string{
		"cpu uses_record entity-name PrometheusRecordingRule node:cpu:rate5m -> node:cpu:rate5m",
		"cpu uses_record entity-name PrometheusRecordingRule up -> up",
		"cpu links_to entity-name GrafanaDashboard abc-1 -> abc-1",
		"cpu provides repo-tree GrafanaDashboard ../dashboards -> grafana/dashboards",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cross-file\n got %q\nwant %q", got, want)
	}
	if len(res.Unresolved) != 1 || res.Unresolved[0].Reason != PromQLUnparsed {
		t.Errorf("unresolved = %+v, want the unterminated expression unparsed", res.Unresolved)
	}
}

// TestDocObsRefusedAtLoad: each new field is checked when the spec loads.
func TestDocObsRefusedAtLoad(t *testing.T) {
	ent := `"entities": [{"select": "$", "kind": "package", "source_kind": "X", "name": "$file"}]`
	for name, js := range map[string]string{
		"entity-name without to_kind":   `{"decode": "yaml", "detect": {}, ` + ent + `, "cross_file": [{"select": "$.a", "value": "$", "resolve": "entity-name", "edge": "e"}]}`,
		"to_kind on repo-path":          `{"decode": "yaml", "detect": {}, ` + ent + `, "cross_file": [{"select": "$.a", "value": "$", "resolve": "repo-path", "to_kind": "X", "edge": "e"}]}`,
		"promql labels on entity-name":  `{"decode": "yaml", "detect": {}, ` + ent + `, "cross_file": [{"select": "$.a", "value": "$", "resolve": "entity-name", "to_kind": "X", "edge": "e", "promql": {"labels": ["job"]}}]}`,
		"matchers without syntax or op": `{"decode": "yaml", "detect": {}, ` + ent + `, "matchers": [{"from": "X", "select": "$.m", "edge": "e"}]}`,
		"matchers with both":            `{"decode": "yaml", "detect": {}, ` + ent + `, "matchers": [{"from": "X", "select": "$.m", "syntax": "alertmanager", "op": "=", "edge": "e"}]}`,
		"matchers op off the list":      `{"decode": "yaml", "detect": {}, ` + ent + `, "matchers": [{"from": "X", "select": "$.m", "op": "==", "edge": "e"}]}`,
		"matchers from no kind":         `{"decode": "yaml", "detect": {}, ` + ent + `, "matchers": [{"from": "Y", "select": "$.m", "op": "=", "edge": "e"}]}`,
		"fact without key":              `{"decode": "yaml", "detect": {}, ` + ent + `, "facts": [{"from": "X", "select": "$.m"}]}`,
		"empty signature field":         `{"decode": "yaml", "detect": {}, "entities": [{"select": "$", "kind": "package", "source_kind": "X", "name": "$file", "signature_fields": [""]}]}`,
	} {
		spec, err := ParseDocSpec([]byte(js))
		if err != nil {
			t.Fatalf("%s: ParseDocSpec: %v", name, err)
		}
		if _, err := NewDocExtractor(spec); err == nil {
			t.Errorf("%s: NewDocExtractor accepted it", name)
		}
	}
}
