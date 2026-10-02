package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// argo-workflows' own v1alpha1 types, strict: the entities are the decoded
// objects' (resource, task, step, job, data_source), and a document the
// types refuse (an unknown field) fails the file.
func TestTypedEntities(t *testing.T) {
	src := []byte(`apiVersion: argoproj.io/v1alpha1
kind: WorkflowTemplate
metadata:
  name: wt
  namespace: ns
spec:
  entrypoint: main
  templates:
    - name: main
      steps:
        - - name: a
            template: echo
          - name: b
            inline:
              container:
                image: example.com/inline:1
    - name: d
      dag:
        tasks:
          - name: t1
            template: echo
    - name: echo
      container:
        image: example.com/echo:1
      sidecars:
        - name: s
          image: example.com/side:1
---
apiVersion: argoproj.io/v1alpha1
kind: CronWorkflow
metadata:
  generateName: cw-
spec:
  schedules: ["* * * * *"]
  workflowSpec:
    entrypoint: s
    templates:
      - name: s
        script:
          image: example.com/py:3
          source: print(1)
---
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  name: plug
spec:
  executorPlugins:
    - metadata:
        name: p
      spec:
        sidecar:
          container:
            name: p
            image: example.com/plugin:1
`)
	got, err := typedEntities(src)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(got, func(i, j int) bool { return got[i].kind+got[i].name < got[j].kind+got[j].name })
	want := []tent{
		{"data_source", "example.com/echo:1"}, {"data_source", "example.com/inline:1"}, {"data_source", "example.com/plugin:1"}, {"data_source", "example.com/py:3"}, {"data_source", "example.com/side:1"},
		{"job", "t1"},
		{"resource", "CronWorkflow/cw-"}, {"resource", "Workflow/plug"}, {"resource", "WorkflowTemplate/ns/wt"},
		{"step", "a"}, {"step", "b"},
		{"task", "d"}, {"task", "echo"}, {"task", "main"}, {"task", "s"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("typedEntities:\n got %v\nwant %v", got, want)
	}
	if _, err := typedEntities([]byte("apiVersion: argoproj.io/v1alpha1\nkind: Workflow\nmetadata:\n  name: w\nspec:\n  entrypoint: m\n  bogus: 1\n")); err == nil {
		t.Fatal("an unknown field must fail the strict decode")
	}
}

// Ablation rewrites exactly one scalar, located by line and column.
func TestReplaceScalar(t *testing.T) {
	src := "a:\n  template: echo\n  other: \"echo\"\n"
	got, ok := replaceScalar(src, 3, 10, "ox-absent-1")
	if !ok || got != "a:\n  template: echo\n  other: \"ox-absent-1\"\n" {
		t.Fatalf("replaceScalar quoted: %q %v", got, ok)
	}
	got, ok = replaceScalar(src, 2, 13, "ox-absent-2")
	if !ok || got != "a:\n  template: ox-absent-2\n  other: \"echo\"\n" {
		t.Fatalf("replaceScalar plain: %q %v", got, ok)
	}
}

// The tool decides: argo lint --offline accepts a file with the subject's
// WorkflowTemplates beside it; each reference is attested when renaming its
// value makes lint fail, each templateRef when lint fails without the
// target's file; a file lint rejects (a templateRef to nothing in the
// subject) contributes nothing.
func TestTierAWorkflows(t *testing.T) {
	bin, err := exec.LookPath("argo")
	if err != nil {
		t.Skip("argo not on PATH")
	}
	dir := t.TempDir()
	write := func(rel, s string) {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("tmpl.yaml", "apiVersion: argoproj.io/v1alpha1\nkind: WorkflowTemplate\nmetadata:\n  name: lib\nspec:\n  templates:\n    - name: echo\n      container:\n        image: example.com/echo:1\n")
	write("wf.yaml", "apiVersion: argoproj.io/v1alpha1\nkind: Workflow\nmetadata:\n  generateName: wf-\nspec:\n  entrypoint: main\n  templates:\n    - name: main\n      steps:\n        - - name: local\n            template: hello\n          - name: remote\n            templateRef:\n              name: lib\n              template: echo\n    - name: hello\n      container:\n        image: example.com/hello:1\n")
	write("dangling.yaml", "apiVersion: argoproj.io/v1alpha1\nkind: Workflow\nmetadata:\n  generateName: d-\nspec:\n  entrypoint: main\n  templates:\n    - name: main\n      steps:\n        - - name: r\n            templateRef:\n              name: absent\n              template: x\n")
	o, files := walkSubject(dir)
	res, err := tierA(dir, bin, t.TempDir(), o, files)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.files, []string{"tmpl.yaml", "wf.yaml"}) {
		t.Fatalf("files: %v (stats %+v)", res.files, res.st)
	}
	if res.st.rejected != 1 || res.st.refsAttested != 2 || res.st.xrefsAttested != 1 {
		t.Fatalf("stats: %+v", res.st)
	}
	if len(res.defs) != 9 {
		t.Fatalf("defs: %+v", res.defs)
	}
}
