package main

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func node(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var d yaml.Node
	if err := yaml.Unmarshal([]byte(src), &d); err != nil {
		t.Fatal(err)
	}
	return d.Content[0]
}

func TestInstances(t *testing.T) {
	wf := node(t, `jobs:
  - plain
  - named:
      name: alias
  - templated:
      name: tpl-<< matrix.v >>
      matrix: {parameters: {v: [a, b]}}
  - matrixed:
      matrix: {parameters: {v: [a, b]}}
  - unseen
`)
	cw := []cwjob{{name: "plain"}, {name: "alias"}, {name: "tpl-a"}, {name: "tpl-b"}, {name: "matrixed-a"}, {name: "matrixed-b"}, {name: "gate"}}
	got := instances(wf, cw)
	want := map[string]string{
		"plain": "plain", "alias": "named", "tpl-a": "templated", "tpl-b": "templated",
		"matrixed-a": "matrixed", "matrixed-b": "matrixed",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("instances\n got: %v\nwant: %v", got, want)
	}
}

func TestStepNamesRecurseThroughConditionals(t *testing.T) {
	var got []string
	stepNames(node(t, `steps:
  - checkout
  - setup: {p: 1}
  - when:
      condition: true
      steps:
        - inner
`), &got)
	want := []string{"checkout", "setup", "when", "inner"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stepNames = %v, want %v", got, want)
	}
}

func TestExecutorName(t *testing.T) {
	for src, want := range map[string]string{
		"executor: base":           "base",
		"executor: {name: runner}": "runner",
		"docker: [{image: x}]":     "",
	} {
		if got := executorName(node(t, src)); got != want {
			t.Errorf("%q: %q, want %q", src, got, want)
		}
	}
}
