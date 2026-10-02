package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// kubectl's -o json prints one object per document, concatenated: the
// decode reads the stream and keeps kind, namespace, name and the images of
// the dialect's four container lists, in order.
func TestKubectlObjects(t *testing.T) {
	stream := []byte(`{
 "apiVersion": "apps/v1", "kind": "Deployment",
 "metadata": {"name": "web", "annotations": {"ox.example.com/m": "1"}},
 "spec": {"template": {"spec": {
   "initContainers": [{"name": "init", "image": "example.com/init:1"}],
   "containers": [{"name": "app", "image": "example.com/app:2"}, {"name": "noimage"}]}}}
}
{
 "apiVersion": "v1", "kind": "Service",
 "metadata": {"name": "web", "namespace": "shop"},
 "spec": "not a map"
}
{"apiVersion": "v1", "kind": "ConfigMap", "metadata": {"generateName": "x-"}}
`)
	got, err := kubectlObjects(stream)
	if err != nil {
		t.Fatal(err)
	}
	want := []kobj{
		{Kind: "Deployment", Name: "web", Images: []kimage{
			{List: "spec.template.spec.containers", Index: 0, Image: "example.com/app:2"},
			{List: "spec.template.spec.initContainers", Index: 0, Image: "example.com/init:1"},
		}},
		{Kind: "Service", Namespace: "shop", Name: "web"},
		{Kind: "ConfigMap"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("kubectlObjects:\n got %+v\nwant %+v", got, want)
	}
}

// Lines are not kubectl's (it prints none): an independent yaml.v3 reader
// places each object at its document's first key and each image at its
// container item, matching by kind, namespace and name in document order.
// An object no document root carries (a List item) is unlocated, a nameless
// object is counted, and neither is guessed into the truth.
func TestLocateKubectlObjects(t *testing.T) {
	src := []byte(`# a comment-only document
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  template:
    spec:
      containers:
        - name: app
          image: example.com/app:2
---
apiVersion: v1
kind: Service
metadata:
  name: web
  namespace: shop
`)
	objs := []kobj{
		{Kind: "Deployment", Name: "web", Images: []kimage{{List: "spec.template.spec.containers", Index: 0, Image: "example.com/app:2"}}},
		{Kind: "Service", Namespace: "shop", Name: "web"},
		{Kind: "Secret", Name: "from-a-list"},
		{Kind: "ConfigMap"},
	}
	defs, st := locate("m/web.yaml", src, objs)
	want := []def{
		{"m/web.yaml", "Deployment/web", 3, "resource"},
		{"m/web.yaml", "example.com/app:2", 11, "data_source"},
		{"m/web.yaml", "Service/shop/web", 14, "resource"},
	}
	if !reflect.DeepEqual(defs, want) {
		t.Fatalf("locate:\n got %+v\nwant %+v", defs, want)
	}
	if st.unlocated != 1 || st.nameless != 1 {
		t.Fatalf("stats: %+v, want 1 unlocated, 1 nameless", st)
	}
}

// The tool decides acceptance: a file with one document kubectl cannot
// decode (no kind) contributes nothing, the whole file.
func TestKubectlAcceptance(t *testing.T) {
	bin, err := exec.LookPath("kubectl")
	if err != nil {
		t.Skip("kubectl not on PATH")
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "good.yaml")
	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(good, []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n---\napiVersion: example.org/v1\nkind: Widget\nmetadata:\n  name: w\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n---\nfoo: bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	objs, err := kubectlDecode(bin, good)
	if err != nil || len(objs) != 2 || objs[1].Kind != "Widget" {
		t.Fatalf("good file: %v %+v", err, objs)
	}
	if _, err := kubectlDecode(bin, bad); err == nil {
		t.Fatal("a file with a kind-less document must be rejected whole")
	}
}
