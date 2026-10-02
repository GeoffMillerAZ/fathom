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

// TestCmdLangPolicies (LB-F137): lang-policies owns a content-claimed policy
// dialect's files by ingest's claim (fusion.PolicyDialect: a file mixing a
// policy with a Deployment stays Kubernetes and is not measured), measures
// them as lang-oracle does, and scores the oracle's policy_references
// against fusion.ResolvePolicyRefs over the files ingest's policy pass reads
// into the xref family (the union with the oracle's cross_references) at the
// oracle's tier, with policy_* counts and the unresolved references by
// reason; the oracle's policy fields never reach lang-oracle's strict
// decode.
func TestCmdLangPolicies(t *testing.T) {
	subject := t.TempDir()
	for p, c := range map[string]string{
		"best/require-labels/require-labels.yaml": `apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-labels
spec:
  rules:
  - name: check-pods
    match:
      any:
      - resources:
          kinds:
          - Pod
  - name: check-widgets
    match:
      any:
      - resources:
          kinds:
          - Widget
`,
		"best/require-labels/.kyverno-test/kyverno-test.yaml": `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: require-labels
policies:
- ../require-labels.yaml
resources:
- resource.yaml
results:
- kind: Pod
  policy: require-labels
  rule: check-pods
  resources:
  - badpod
  result: fail
`,
		"best/require-labels/.kyverno-test/resource.yaml": `apiVersion: v1
kind: Pod
metadata:
  name: badpod
---
apiVersion: v1
kind: Pod
metadata:
  name: goodpod
`,
		"mixed/policy-and-pod.yaml": `apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: mixed
spec:
  rules: []
---
apiVersion: v1
kind: Pod
metadata:
  name: beside
`,
		"far/deploy.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: far
`,
	} {
		full := filepath.Join(subject, filepath.FromSlash(p))
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pol, test, res := "best/require-labels/require-labels.yaml", "best/require-labels/.kyverno-test/kyverno-test.yaml", "best/require-labels/.kyverno-test/resource.yaml"
	oracle := filepath.Join(t.TempDir(), "oracle.json")
	_ = os.WriteFile(oracle, []byte(`{"tool":"test","tier":"C","definitions":[
 {"path":"`+pol+`","name":"ClusterPolicy/require-labels","line":1,"kind":"policy"},
 {"path":"`+pol+`","name":"check-pods","line":6,"kind":"rule"},
 {"path":"`+pol+`","name":"check-widgets","line":12,"kind":"rule"},
 {"path":"`+test+`","name":"Test/require-labels","line":1,"kind":"test"}],
 "references":[{"path":"`+test+`","from":"Test/require-labels","to":"require-labels"},{"path":"`+test+`","from":"Test/require-labels","to":"check-pods"}],
 "cross_references":[{"path":"`+test+`","from":"Test/require-labels","to_path":"`+pol+`","to":"ClusterPolicy/require-labels"}],
 "policy_references":[
 {"path":"`+pol+`","from":"check-pods","to_path":"`+res+`","to":"Pod/badpod","raw":"Pod","strategy":"resource-kind"},
 {"path":"`+pol+`","from":"check-pods","to_path":"`+res+`","to":"Pod/goodpod","raw":"Pod","strategy":"resource-kind"},
 {"path":"vendor/elsewhere.yaml","from":"x","to_path":"`+res+`","to":"Pod/badpod","raw":"Pod"}],
 "policy_unresolved":[{"path":"`+pol+`","from":"check-widgets","raw":"Widget","reason":"undeclared"}],
 "policy_crosscheck":{"tool":"kyverno test 1.0","tier":"A","checked":1,"agree":1,"disagree":[]}}`), 0o644)
	out := filepath.Join(t.TempDir(), "kyverno.json")
	var stdout, stderr bytes.Buffer
	args := []string{"-dialect", "kyverno", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "kyverno", "dialect.json"),
		"-subject", subject, "-subject-name", "fixture", "-tool-oracle", oracle, "-ctags", filepath.Join(t.TempDir(), "none"),
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	if code := cmdLangPolicies(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	raw, _ := os.ReadFile(out)
	var r langbench.Reading
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	s := r.Subjects[0]
	if s.Files != 2 {
		t.Errorf("measured %d files, want 2 (the policy and the test; the mixed file stays Kubernetes)\n%s", s.Files, raw)
	}
	c := s.Counts
	for k, want := range map[string]int{
		langbench.CntPolicyTruth: 2, langbench.CntPolicyFathom: 2, langbench.CntPolicyTruthFound: 2, langbench.CntPolicyFathomInTrue: 2,
		// the union: one test-to-policy cross-reference plus two policy pairs
		langbench.CntXrefTruth: 3, langbench.CntXrefFathom: 3, langbench.CntXrefTruthFound: 3,
		langbench.CntPolicyUnresolvedPrefix + "undeclared":       1,
		langbench.CntPolicyOracleUnresolvedPrefix + "undeclared": 1,
		langbench.CntMatched: 4,
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
	for _, note := range []string{
		"1 oracle policy references on files the policy pass does not read",
		"policy cross-check by kyverno test 1.0 (tier A, not scored): 1 of 1",
		"the 2 measured files are the ones ingest's claim",
	} {
		if !bytes.Contains(raw, []byte(note)) {
			t.Errorf("reading lacks note %q\n%s", note, raw)
		}
	}
	if !strings.Contains(stdout.String(), "policies truth 2 fathom 2 agree 2/2") {
		t.Errorf("stdout %q", stdout.String())
	}
}

// TestCmdLangPoliciesNeedsPolicyTruth: an oracle with no policy_references
// is refused, never read as an empty policy truth.
func TestCmdLangPoliciesNeedsPolicyTruth(t *testing.T) {
	oracle := filepath.Join(t.TempDir(), "oracle.json")
	_ = os.WriteFile(oracle, []byte(`{"tool":"test","tier":"C","definitions":[]}`), 0o644)
	var stdout, stderr bytes.Buffer
	code := cmdLangPolicies([]string{"-dialect", "kyverno", "-subject", t.TempDir(), "-tool-oracle", oracle, "-out", filepath.Join(t.TempDir(), "r.json")}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "no -tool-oracle carries policy_references") {
		t.Errorf("exit %d stderr %q", code, stderr.String())
	}
}

func TestLiteralGlob(t *testing.T) {
	if got := literalGlob("a/[x]*?.yaml"); got != `a/\[x]\*\?.yaml` {
		t.Errorf("literalGlob = %q", got)
	}
}
