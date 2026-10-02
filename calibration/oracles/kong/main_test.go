package main

import (
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

func TestUniqueElsewhere(t *testing.T) {
	d := map[string]map[string]bool{"a/x.yaml": {"s": true, "only": true}, "a/y.yaml": {"s": true, "t": true}, "b/z.yaml": {"t": true}}
	if f, ok := uniqueElsewhere(d, "a/x.yaml", "t"); !ok || f != "a/y.yaml" {
		t.Errorf("uniqueElsewhere t = %q, %v", f, ok)
	}
	if _, ok := uniqueElsewhere(d, "a/x.yaml", "only"); ok {
		t.Error("a name declared only in the referring file resolves nowhere else")
	}
}
