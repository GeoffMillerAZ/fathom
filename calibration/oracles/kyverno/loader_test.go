package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func writeSubject(t *testing.T, files map[string]string) string {
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
	return dir
}

const clusterPolicy = `apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-labels
spec:
  validationFailureAction: Audit
  rules:
    - name: check-team
      match:
        any:
          - resources:
              kinds: [Pod]
      validate:
        message: "team label required"
        pattern:
          metadata:
            labels:
              team: "?*"
    - name: check-app
      match:
        any:
          - resources:
              kinds: [Deployment]
      validate:
        message: "app label required"
        pattern:
          metadata:
            labels:
              app: "?*"
`

// TestLoaderAttestsEntities: definitions and references come from Kyverno's
// own loaders (the CLI's policy.Load and test.LoadTest): a policy the
// loader refuses (a field its CRD schema does not know) yields nothing,
// while the walk alone would have read it.
func TestLoaderAttestsEntities(t *testing.T) {
	dir := writeSubject(t, map[string]string{
		"pol/policy.yaml": clusterPolicy,
		"pol/bad.yaml": `apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: bad
spec:
  notAField: true
  rules:
    - name: r
      match:
        any:
          - resources:
              kinds: [Pod]
      validate:
        pattern:
          metadata:
            name: "?*"
`,
		"pol/exception.yaml": `apiVersion: kyverno.io/v2
kind: PolicyException
metadata:
  name: allow-system
  namespace: kyverno
spec:
  exceptions:
    - policyName: require-labels
      ruleNames: [check-team]
  match:
    any:
      - resources:
          kinds: [Pod]
          namespaces: [kube-system]
`,
		"pol/.kyverno-test/kyverno-test.yaml": `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: require-labels
policies:
  - ../policy.yaml
resources:
  - resource.yaml
results:
  - policy: require-labels
    rule: check-team
    resources: [good-pod]
    kind: Pod
    result: pass
`,
		"pol/.kyverno-test/resource.yaml": `apiVersion: v1
kind: Pod
metadata:
  name: good-pod
  labels: {team: a}
spec:
  containers: [{name: c, image: example.org/app:1}]
`,
	})
	files, err := walkYAML(dir)
	if err != nil {
		t.Fatal(err)
	}
	lr := loadEntities(dir, files)
	var defs []string
	for _, d := range lr.defs {
		defs = append(defs, d.Path+"|"+d.Name+"|"+d.Kind)
	}
	sort.Strings(defs)
	want := []string{
		"pol/.kyverno-test/kyverno-test.yaml|Test/require-labels|test",
		"pol/exception.yaml|PolicyException/kyverno/allow-system|policy",
		"pol/policy.yaml|ClusterPolicy/require-labels|policy",
		"pol/policy.yaml|check-app|rule",
		"pol/policy.yaml|check-team|rule",
	}
	if strings.Join(defs, "\n") != strings.Join(want, "\n") {
		t.Fatalf("defs:\n%s\nwant:\n%s", strings.Join(defs, "\n"), strings.Join(want, "\n"))
	}
	for _, d := range lr.defs {
		if d.Line == 0 {
			t.Fatalf("no line: %+v", d)
		}
	}
	if lr.refused["pol/bad.yaml"] == "" {
		t.Fatalf("the loader did not refuse bad.yaml: %v", lr.refused)
	}
	var refs []string
	for _, r := range lr.refs {
		refs = append(refs, r.Path+"|"+r.From+"->"+r.To)
	}
	sort.Strings(refs)
	wantRefs := []string{
		"pol/.kyverno-test/kyverno-test.yaml|Test/require-labels->check-team",
		"pol/.kyverno-test/kyverno-test.yaml|Test/require-labels->require-labels",
		"pol/exception.yaml|PolicyException/kyverno/allow-system->check-team",
		"pol/exception.yaml|PolicyException/kyverno/allow-system->require-labels",
	}
	if strings.Join(refs, "\n") != strings.Join(wantRefs, "\n") {
		t.Fatalf("refs:\n%s\nwant:\n%s", strings.Join(refs, "\n"), strings.Join(wantRefs, "\n"))
	}
	if got := lr.tests["pol/.kyverno-test/kyverno-test.yaml"]; len(got.policies) != 1 || got.policies[0] != "../policy.yaml" {
		t.Fatalf("test policies = %+v", got)
	}
}

// TestOnlyGlobs: -only keeps the files lang-policies -only measures (a
// path.Match per segment, "**" any number of segments), so the oracle's
// object set and fathom's agree on scope.
func TestOnlyGlobs(t *testing.T) {
	globs := splitGlobs("[^.]*/[^.]*,[^.]*/.kyverno-test/[^.]*,a/**/z.yaml")
	for p, want := range map[string]bool{
		"pol/policy.yaml":                      true,
		"pol/.kyverno-test/kyverno-test.yaml":  true,
		"pol/.chainsaw-test/policy-ready.yaml": false,
		"a/b/c/z.yaml":                         true,
		"a/z.yaml":                             true,
		"top.yaml":                             false,
	} {
		if got := onlyMatch(globs, p); got != want {
			t.Errorf("onlyMatch(%q) = %v, want %v", p, got, want)
		}
	}
	if !onlyMatch(nil, "anything.yaml") {
		t.Error("no globs must keep every file")
	}
}
