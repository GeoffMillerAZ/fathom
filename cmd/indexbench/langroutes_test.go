package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// TestCmdLangRoutes (LB-F98): lang-routes measures a subject as lang-oracle
// does and scores the oracle's route_references against
// fusion.ResolveRouteRefs over the files ingest's route pass reads —
// Kubernetes routes through the kubernetes dialect, a compose mongobetween's
// launch arguments — into the xref family (the union with the oracle's
// cross_references) at the oracle's tier, with route_* counts and the
// unresolved references by reason; the oracle's route fields never reach
// lang-oracle's strict decode.
func TestCmdLangRoutes(t *testing.T) {
	subject := t.TempDir()
	for p, c := range map[string]string{
		"k8s/svc.yaml": `apiVersion: v1
kind: Service
metadata:
  name: orders
  namespace: shop
spec:
  selector:
    app: orders
---
apiVersion: v1
kind: Service
metadata:
  name: web
  namespace: shop
spec:
  selector:
    app: web
`,
		"k8s/routes.yaml": `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: front
  namespace: shop
spec:
  defaultBackend:
    service:
      name: web
---
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: orders
  namespace: shop
spec:
  http:
    - route:
        - destination:
            host: orders
    - route:
        - destination:
            host: api.example.com
`,
		"k8s/web.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: shop
spec:
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
        - name: web
          image: acme/web:1
`,
		"docker-compose.yml": `services:
  mongo:
    image: mongo:7
  pooler:
    image: coinbase/mongobetween:latest
    command: [":27016=mongodb://mongo:27017/app"]
`,
	} {
		full := filepath.Join(subject, p)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	oracle := filepath.Join(t.TempDir(), "oracle.json")
	_ = os.WriteFile(oracle, []byte(`{"tool":"test","tier":"C","definitions":[
 {"path":"k8s/svc.yaml","name":"Service/shop/orders","line":1,"kind":"resource"},
 {"path":"k8s/svc.yaml","name":"Service/shop/web","line":10,"kind":"resource"},
 {"path":"k8s/routes.yaml","name":"Ingress/shop/front","line":1,"kind":"resource"},
 {"path":"k8s/routes.yaml","name":"VirtualService/shop/orders","line":11,"kind":"resource"},
 {"path":"k8s/web.yaml","name":"Deployment/shop/web","line":1,"kind":"resource"},
 {"path":"k8s/web.yaml","name":"acme/web:1","line":13,"kind":"data_source"}],
 "references":[],
 "cross_references":[{"path":"k8s/svc.yaml","from":"Service/shop/web","to_path":"k8s/web.yaml","to":"Deployment/shop/web"}],
 "route_references":[
 {"path":"k8s/routes.yaml","from":"Ingress/shop/front","to_path":"k8s/svc.yaml","to":"Service/shop/web","raw":"web"},
 {"path":"k8s/routes.yaml","from":"VirtualService/shop/orders","to_path":"k8s/svc.yaml","to":"Service/shop/orders","raw":"orders"},
 {"path":"docker-compose.yml","from":"pooler","to_path":"docker-compose.yml","to":"mongo","raw":"mongo:27017"},
 {"path":"vendor/elsewhere.yaml","from":"Ingress/x","to_path":"k8s/svc.yaml","to":"Service/shop/web","raw":"web"}],
 "route_unresolved":[{"path":"k8s/routes.yaml","from":"VirtualService/shop/orders","raw":"api.example.com","reason":"external"}],
 "route_crosscheck":{"tool":"istioctl analyze 1.0","tier":"A","checked":2,"agree":2,"disagree":[]}}`), 0o644)
	out := filepath.Join(t.TempDir(), "kubernetes.json")
	var stdout, stderr bytes.Buffer
	args := []string{"-dialect", "kubernetes", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "kubernetes", "dialect.json"),
		"-subject", subject, "-subject-name", "fixture", "-tool-oracle", oracle, "-ctags", filepath.Join(t.TempDir(), "none"),
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	if code := cmdLangRoutes(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	raw, _ := os.ReadFile(out)
	var r langbench.Reading
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	c := r.Subjects[0].Counts
	for k, want := range map[string]int{
		langbench.CntRouteTruth: 3, langbench.CntRouteFathom: 3, langbench.CntRouteTruthFound: 3, langbench.CntRouteFathomInTrue: 3,
		// the union: one label-selector cross-reference plus three routes
		langbench.CntXrefTruth: 4, langbench.CntXrefFathom: 4, langbench.CntXrefTruthFound: 4,
		langbench.CntRouteUnresolvedPrefix + "external":       1,
		langbench.CntRouteOracleUnresolvedPrefix + "external": 1,
		langbench.CntMatched:                                  6,
	} {
		if c[k] != want {
			t.Errorf("count %s = %d, want %d\n%s", k, c[k], want, raw)
		}
	}
	for _, m := range []string{langbench.MSymRecall, langbench.MSymPrecision, langbench.MXrefPrecision, langbench.MXrefRecall} {
		if v := r.Metrics[m]; v == nil || *v != 1 || r.MetricTiers[m] != "C" {
			t.Errorf("%s = %v tier %q, want 1 at C\n%s", m, v, r.MetricTiers[m], raw)
		}
	}
	// Tier C caps every level at good; level_met also needs the tenancy
	// floor, which four tiny files under the race detector may not reach.
	if r.Grade.L3 != "good" {
		t.Errorf("grade L3 = %q, want good (tier C)\n%s", r.Grade.L3, raw)
	}
	for _, note := range []string{"1 oracle route references on files the route pass does not read", "route cross-check by istioctl analyze 1.0 (tier A, not scored): 2 of 2"} {
		if !bytes.Contains(raw, []byte(note)) {
			t.Errorf("reading lacks note %q\n%s", note, raw)
		}
	}
	if !strings.Contains(stdout.String(), "routes truth 3 fathom 3 agree 3/3") {
		t.Errorf("stdout %q", stdout.String())
	}

	// A wrong route the oracle does not hold lowers xref_precision.
	_ = os.WriteFile(filepath.Join(subject, "k8s/extra.yaml"), []byte("apiVersion: networking.k8s.io/v1\nkind: Ingress\nmetadata:\n  name: stray\n  namespace: shop\nspec:\n  defaultBackend:\n    service:\n      name: orders\n"), 0o644)
	out2 := filepath.Join(t.TempDir(), "kubernetes.json")
	args[len(args)-3] = out2
	stdout.Reset()
	stderr.Reset()
	if code := cmdLangRoutes(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	raw2, _ := os.ReadFile(out2)
	var r2 langbench.Reading
	_ = json.Unmarshal(raw2, &r2)
	if v := r2.Metrics[langbench.MXrefPrecision]; v == nil || *v >= 1 {
		t.Errorf("xref_precision with an unproven route = %v, want < 1", v)
	}
}

// TestCmdLangRoutesNeedsRouteTruth: an oracle with no route_references is
// refused, never read as an empty route truth.
func TestCmdLangRoutesNeedsRouteTruth(t *testing.T) {
	oracle := filepath.Join(t.TempDir(), "oracle.json")
	_ = os.WriteFile(oracle, []byte(`{"tool":"test","tier":"C","definitions":[]}`), 0o644)
	var stdout, stderr bytes.Buffer
	code := cmdLangRoutes([]string{"-dialect", "kubernetes", "-subject", t.TempDir(), "-tool-oracle", oracle, "-out", filepath.Join(t.TempDir(), "r.json")}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "no -tool-oracle carries route_references") {
		t.Errorf("exit %d stderr %q", code, stderr.String())
	}
}
