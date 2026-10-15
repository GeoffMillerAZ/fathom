package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestFactsTypedDecode (LB-F451): the truth is k8s.io/api's typed decode of
// LB-F451's own fixture (internal/extractor/testdata/k8sfacts): a quantity
// canonical, an int32 timing only where written, a probe's handler and
// port, the lists apart; the CronJob's templated quantity makes the typed
// decode refuse that document, so it is no truth; no env value, arg value
// or exec command is read.
func TestFactsTypedDecode(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "internal", "extractor", "testdata", "k8sfacts", "workloads.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	w := &factsWalk{}
	w.file("deploy/workloads.yaml", src)
	var got []string
	for _, f := range w.facts {
		if strings.Contains(f.Value, "SENTINEL") {
			t.Errorf("a fact carries a withheld value: %+v", f)
		}
		got = append(got, f.Class+" "+f.Entity+"#"+f.Key+" "+f.Value)
	}
	want := []string{
		"spec Pod/shop/probe-pod#containers web",
		"spec Pod/shop/probe-pod#containers.web.resources.requests.cpu 100m",
		"spec Pod/shop/probe-pod#containers.web.resources.requests.memory 64Mi",
		"spec Pod/shop/probe-pod#containers.web.resources.requests.ephemeral-storage 1Gi",
		"spec Pod/shop/probe-pod#containers.web.resources.limits.cpu 1",
		"spec Pod/shop/probe-pod#containers.web.resources.limits.memory 128Mi",
		"spec Pod/shop/probe-pod#containers.web.resources.limits.ephemeral-storage 2Gi",
		"spec Pod/shop/probe-pod#containers.web.livenessProbe.path /healthz",
		"spec Pod/shop/probe-pod#containers.web.livenessProbe.port 8080",
		"spec Pod/shop/probe-pod#containers.web.livenessProbe httpGet",
		"spec Pod/shop/probe-pod#containers.web.livenessProbe.initialDelaySeconds 5",
		"spec Pod/shop/probe-pod#containers.web.livenessProbe.periodSeconds 10",
		"spec Pod/shop/probe-pod#containers.web.readinessProbe.port http",
		"spec Pod/shop/probe-pod#containers.web.readinessProbe tcpSocket",
		"spec Pod/shop/probe-pod#containers.web.readinessProbe.timeoutSeconds 2",
		"spec Pod/shop/probe-pod#containers.web.readinessProbe.failureThreshold 3",
		"spec Pod/shop/probe-pod#containers.web.startupProbe exec",
		"spec Pod/shop/probe-pod#containers.web.startupProbe.successThreshold 1",
		"template Deployment/shop/api#replicas 3",
		"template Deployment/shop/api#initContainers migrate",
		"template Deployment/shop/api#initContainers.migrate.resources.limits.memory 32Mi",
		"template Deployment/shop/api#containers api,bare",
		"template Deployment/shop/api#containers.api.readinessProbe.port 9090",
		"template Deployment/shop/api#containers.api.readinessProbe grpc",
		"template StatefulSet/db#replicas 1",
		"template StatefulSet/db#containers db",
		"template StatefulSet/db#containers.db.resources.requests.memory 1Gi",
		"template Job/seed#containers seed",
		"template Job/seed#containers.seed.resources.requests.cpu 250m",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("facts\n got %q\nwant %q", got, want)
	}
	if w.st.Typed != 4 || w.st.Refused != 1 || w.st.TypedByPath["spec"] != 1 || w.st.TypedByPath["template"] != 3 {
		t.Errorf("stats %+v", w.st)
	}
}

// TestFactsCompare: fathom's rows match the truth by (path, entity, key,
// value), a quantity compared canonically; a row on an entity the typed
// decode did not read, and a truth fact on a file fathom does not read, are
// counted, never scored.
func TestFactsCompare(t *testing.T) {
	w := &factsWalk{}
	w.file("a.yaml", []byte(`apiVersion: v1
kind: Pod
metadata: {name: p}
spec:
  containers:
    - name: c
      resources: {requests: {cpu: "0.5"}}
`))
	rows := filepath.Join(t.TempDir(), "rows.json")
	if err := os.WriteFile(rows, []byte(`[
 {"path": "a.yaml", "symbol": "Pod/p#containers", "claim": "c"},
 {"path": "a.yaml", "symbol": "Pod/p#containers.c.resources.requests.cpu", "claim": "0.5"},
 {"path": "a.yaml", "symbol": "Pod/p#replicas", "claim": "2"},
 {"path": "a.yaml", "symbol": "Rollout/r#containers", "claim": "x"}
]`), 0o644); err != nil {
		t.Fatal(err)
	}
	files := filepath.Join(t.TempDir(), "files.txt")
	if err := os.WriteFile(files, []byte("a.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.file("testdata/b.yaml", []byte("apiVersion: v1\nkind: Pod\nmetadata: {name: q}\nspec: {containers: [{name: c}]}\n"))
	p, err := w.compare(rows, files)
	if err != nil {
		t.Fatal(err)
	}
	if p.Truth != 2 || p.Fathom != 3 || p.Matched != 2 || p.FathomOnlyCnt != 1 || p.FathomUntyped != 1 || p.ByClass["spec"].Matched != 2 || p.TruthOutOfScope != 1 {
		t.Errorf("parity %+v", p)
	}
}
