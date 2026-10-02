package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTree lays out files under a temp dir and returns the walked YAML.
func writeTree(t *testing.T, files map[string]string) []yamlFile {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fs, err := walkYAML(dir)
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func pairsOf(refs []policyRef) []string {
	var out []string
	for _, r := range refs {
		out = append(out, r.Path+"|"+r.From+"->"+r.ToPath+"|"+r.To)
	}
	return out
}

func reasonsOf(un []policyUnresolved) []string {
	var out []string
	for _, u := range un {
		out = append(out, u.From+":"+u.Raw+":"+u.Reason)
	}
	return out
}

func engineFor(t *testing.T, files map[string]string) engineResult {
	t.Helper()
	fs := writeTree(t, files)
	var cs []constraintDoc
	for _, f := range fs {
		if !claimed(f) {
			continue
		}
		for _, d := range f.docs {
			if ent, kind := entityOf(d); kind == "resource" {
				cs = append(cs, constraintDoc{rel: f.rel, ent: ent, d: d})
			}
		}
	}
	res, err := engineRefs(cs, fs)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

const pods = `apiVersion: v1
kind: Pod
metadata:
  name: in-prod
  namespace: prod
  labels: {app: web, tier: front}
---
apiVersion: v1
kind: Pod
metadata:
  name: in-system
  namespace: kube-system
  labels: {app: web}
---
apiVersion: v1
kind: Pod
metadata:
  name: no-ns
  labels: {app: db}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy
  namespace: prod
  labels: {app: web}
`

// TestEngineKindsGroupsAndNamespaces: Gatekeeper's match decides the pair —
// apiGroups [""] is the core group only, excludedNamespaces disqualifies
// (the walk never read it), and an object that declares no namespace is not
// disqualified by a namespaces list (match.namespacesMatch: gator's view of
// an object as written).
func TestEngineKindsGroupsAndNamespaces(t *testing.T) {
	res := engineFor(t, map[string]string{
		"a/objects.yaml": pods,
		"a/constraint.yaml": `apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sThing
metadata:
  name: c1
spec:
  match:
    kinds:
      - apiGroups: [""]
        kinds: ["Pod", "Deployment"]
    excludedNamespaces: ["kube-system"]
`,
		"b/constraint.yaml": `apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sThing
metadata:
  name: c2
spec:
  match:
    kinds:
      - apiGroups: ["apps"]
        kinds: ["Deployment"]
    namespaces: ["prod"]
`,
	})
	got := strings.Join(pairsOf(res.refs), "\n")
	want := strings.Join([]string{
		"a/constraint.yaml|K8sThing/c1->a/objects.yaml|Pod/no-ns",
		"a/constraint.yaml|K8sThing/c1->a/objects.yaml|Pod/prod/in-prod",
		"b/constraint.yaml|K8sThing/c2->a/objects.yaml|Deployment/prod/deploy",
	}, "\n")
	if got != want {
		t.Fatalf("pairs:\n%s\nwant:\n%s", got, want)
	}
	// The core-group Deployment cell matches nothing: unresolved undeclared.
	if r := strings.Join(reasonsOf(res.unresolved), ","); r != "K8sThing/c1:Deployment:undeclared" {
		t.Fatalf("unresolved = %s", r)
	}
}

// TestEngineSelectorsAndWildcards: a labelSelector's matchExpressions narrow
// the match (Gatekeeper evaluates them; the walk read matchLabels only); a
// selector with no kinds is a label-selector target; no kinds and no selector
// is the dialect's refused wildcard.
func TestEngineSelectorsAndWildcards(t *testing.T) {
	res := engineFor(t, map[string]string{
		"a/objects.yaml": pods,
		"a/sel.yaml": `apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sThing
metadata:
  name: expr
spec:
  match:
    kinds:
      - apiGroups: [""]
        kinds: ["Pod"]
    labelSelector:
      matchExpressions:
        - {key: tier, operator: Exists}
`,
		"a/labels.yaml": `apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sThing
metadata:
  name: lbl
spec:
  match:
    labelSelector:
      matchLabels: {app: db}
`,
		"a/wild.yaml": `apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sThing
metadata:
  name: wild
spec:
  match:
    namespaces: ["prod"]
`,
	})
	got := strings.Join(pairsOf(res.refs), "\n")
	want := strings.Join([]string{
		"a/labels.yaml|K8sThing/lbl->a/objects.yaml|Pod/no-ns",
		"a/sel.yaml|K8sThing/expr->a/objects.yaml|Pod/prod/in-prod",
	}, "\n")
	if got != want {
		t.Fatalf("pairs:\n%s\nwant:\n%s", got, want)
	}
	if r := strings.Join(reasonsOf(res.unresolved), ","); r != "K8sThing/wild:*:wildcard" {
		t.Fatalf("unresolved = %s", r)
	}
}

// TestEngineNearestAndNamespaceSelector: the dialect's join keeps the nearest
// matched objects (the constraint's own file, else its directory's subtree,
// else the repository); a namespaceSelector reads the subject's Namespace
// objects through the target's own cache.
func TestEngineNearestAndNamespaceSelector(t *testing.T) {
	res := engineFor(t, map[string]string{
		"x/near/pod.yaml": `apiVersion: v1
kind: Pod
metadata: {name: near, namespace: team}
`,
		"x/far.yaml": `apiVersion: v1
kind: Pod
metadata: {name: far, namespace: other}
`,
		"ns.yaml": `apiVersion: v1
kind: Namespace
metadata:
  name: team
  labels: {owner: example.org}
---
apiVersion: v1
kind: Namespace
metadata:
  name: other
`,
		"x/near/c.yaml": `apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sThing
metadata:
  name: near
spec:
  match:
    kinds: [{apiGroups: [""], kinds: ["Pod"]}]
`,
		"y/c.yaml": `apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sThing
metadata:
  name: sel
spec:
  match:
    kinds: [{apiGroups: [""], kinds: ["Pod"]}]
    namespaceSelector:
      matchLabels: {owner: example.org}
`,
	})
	got := strings.Join(pairsOf(res.refs), "\n")
	want := strings.Join([]string{
		"x/near/c.yaml|K8sThing/near->x/near/pod.yaml|Pod/team/near",
		"y/c.yaml|K8sThing/sel->x/near/pod.yaml|Pod/team/near",
	}, "\n")
	if got != want {
		t.Fatalf("pairs:\n%s\nwant:\n%s", got, want)
	}
	if res.engineErrors != 0 {
		t.Fatalf("engine errors = %d", res.engineErrors)
	}
	// Every pair the engine matched, before the nearest rule.
	if !res.matched[[2]string{"x/near/c.yaml", "x/far.yaml"}] {
		t.Fatalf("matched before nearest misses the far pod: %v", res.matched)
	}
}
