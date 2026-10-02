package main

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const sample = `_format_version: "3.0"
services:
- name: s
  host: up
  routes:
  - name: nested
  plugins:
  - name: cors
routes:
- name: flat
  service:
    name: s
upstreams:
- name: up
  targets:
  - target: a:80
plugins:
- name: key-auth
  route: flat
- name: global
`

func TestWalkWrittenAndRendered(t *testing.T) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(sample), &doc); err != nil {
		t.Fatal(err)
	}
	ents, refs := walkWritten(doc.Content[0])
	got := map[string]int{}
	for _, e := range ents {
		got[key(e)] = e.line
	}
	for k, line := range map[string]int{
		"service\x00s\x00":                 3,
		"route\x00nested\x00service:s":     6,
		"plugin\x00cors\x00service:s":      8,
		"route\x00flat\x00service:s":       10,
		"upstream\x00up\x00":               14,
		"target\x00a:80\x00upstream:up":    16,
		"plugin\x00key-auth\x00route:flat": 18,
		"plugin\x00global\x00":             20,
	} {
		if got[k] != line {
			t.Errorf("entity %q at line %d, want %d (all: %v)", k, got[k], line, got)
		}
	}
	// only the foreign keys written: the nested route and plugin write none
	want := map[string]bool{"route flat -> s": true, "service s -> up": true, "plugin key-auth -> flat": true}
	for _, r := range refs {
		k := r.fromKind + " " + r.from + " -> " + r.to
		if !want[k] {
			t.Errorf("unexpected reference %q", k)
		}
		delete(want, k)
	}
	for k := range want {
		t.Errorf("missing reference %q", k)
	}
	rend := walkRendered(map[string]any{
		"services": []any{map[string]any{"name": "s", "routes": []any{map[string]any{"name": "nested"}, map[string]any{"name": "flat", "plugins": []any{map[string]any{"name": "key-auth"}}}},
			"plugins": []any{map[string]any{"name": "cors"}}}},
		"upstreams": []any{map[string]any{"name": "up", "targets": []any{map[string]any{"target": "a:80"}}}},
		"plugins":   []any{map[string]any{"name": "global"}},
	})
	rk := map[string]bool{}
	for _, e := range rend {
		rk[key(e)] = true
	}
	for k := range got {
		if !rk[k] {
			t.Errorf("decK's rendering lacks the written entity %q", k)
		}
	}
}

// TestRoutesFileTier (LB-F168): a routes file is tier A when every pair is
// attested (decK's rendered state holds the written host, Docker Compose read the
// compose side) and tier C when one is written text decK does not attest (a
// DB-less file it refuses, a host its rendering lacks).
func TestRoutesFileTier(t *testing.T) {
	for _, c := range []struct {
		refs []routeRef
		want string
	}{
		{nil, `"tier":"A"`},
		{[]routeRef{{Tier: ""}}, `"tier":"A"`},
		{[]routeRef{{Tier: ""}, {Tier: "C"}}, `"tier":"C"`},
	} {
		b, err := json.Marshal(newRoutesFile("deck", "v1", c.refs, nil))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), c.want) {
			t.Errorf("routes file %s lacks %s", b, c.want)
		}
	}
}

// TestRenderedHosts: decK's rendered services and upstream targets give the
// hosts a written route target is confirmed against.
func TestRenderedHosts(t *testing.T) {
	var rendered map[string]any
	if err := json.Unmarshal([]byte(`{"services":[{"name":"auth","host":"auth.internal","port":8000},{"name":"rest","host":"rest"}],
 "upstreams":[{"name":"pool","targets":[{"target":"web1:8000"},{"target":"web2"}]}]}`), &rendered); err != nil {
		t.Fatal(err)
	}
	got := renderedHosts(rendered)
	for _, c := range []struct {
		kind, name, host string
		want             bool
	}{
		{"service", "auth", "auth.internal", true},
		{"service", "auth", "rest", false},
		{"service", "rest", "rest", true},
		{"upstream", "pool", "web1", true},
		{"upstream", "pool", "web2", true},
		{"upstream", "pool", "web3", false},
		{"service", "nope", "auth.internal", false},
	} {
		if has := got[c.kind+"\x00"+c.name][c.host]; has != c.want {
			t.Errorf("rendered %s %s holds %q = %v, want %v", c.kind, c.name, c.host, has, c.want)
		}
	}
}

func TestUniqueElsewhere(t *testing.T) {
	d := map[string]map[string]bool{"a/x.yaml": {"s": true, "only": true}, "a/y.yaml": {"s": true, "t": true}, "b/z.yaml": {"t": true}}
	if f, ok := uniqueElsewhere(d, "a/x.yaml", "t"); !ok || f != "a/y.yaml" {
		t.Errorf("uniqueElsewhere t = %q, %v", f, ok)
	}
	if _, ok := uniqueElsewhere(d, "a/x.yaml", "only"); ok {
		t.Error("a name declared only in the referring file resolves nowhere else")
	}
}
