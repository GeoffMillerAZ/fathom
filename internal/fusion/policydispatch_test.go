package fusion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// policyGolden is a policy dialect fixture's hand-written golden: the
// entities, and every cross-file reference — the doc spec's and the
// policy_ref targets the dispatched extractor adds (Scope and Labels too).
type policyGolden struct {
	Symbols   []policyGoldenSymbol `json:"symbols"`
	CrossFile []policyGoldenCross  `json:"cross_file"`
}

type policyGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type policyGoldenCross struct {
	From     string            `json:"from"`
	Edge     string            `json:"edge"`
	Raw      string            `json:"raw"`
	Target   string            `json:"target"`
	Strategy string            `json:"strategy"`
	Line     int               `json:"line"`
	Scope    string            `json:"scope,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

func policyFixtures(t *testing.T, id string) []string {
	t.Helper()
	dir := filepath.Join("..", "extractor", "dialects", id, "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() && !strings.HasSuffix(e.Name(), ".golden.json") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	if len(out) < 3 {
		t.Fatalf("%s: %d fixtures, want at least 3", dir, len(out))
	}
	return out
}

// TestPolicyDialectGoldens (LB-F134): every Kyverno and Gatekeeper fixture
// is claimed by policyDispatch for its dialect, and the claimed extractor's
// entities, references, cross-file references and policy_ref targets equal
// the fixture's hand-written golden. A fixture is read as if it sat in
// policies/<name>. Fixtures are looped in the test body (no subtests), so a
// plan row's reachable_from can name this test.
func TestPolicyDialectGoldens(t *testing.T) {
	for _, id := range []string{"kyverno", "gatekeeper"} {
		for _, fx := range policyFixtures(t, id) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			rel := "policies/" + filepath.Base(fx)
			e, tk, ok := policyDispatch(rel, src)
			if !ok || string(tk) != id {
				t.Errorf("%s: policyDispatch = (%q, %v), want %s", fx, tk, ok, id)
				continue
			}
			res, err := e.(extractor.DocResulter).ExtractDoc("svc", rel, src)
			if err != nil {
				t.Fatalf("%s: %v", fx, err)
			}
			got := policyGolden{Symbols: []policyGoldenSymbol{}, CrossFile: []policyGoldenCross{}}
			for _, n := range res.Nodes {
				got.Symbols = append(got.Symbols, policyGoldenSymbol{Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature, Calls: n.Calls, Tier: tierName(n.Tier)})
			}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, policyGoldenCross{From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target,
					Strategy: c.Strategy, Line: c.Line, Scope: c.Scope, Labels: c.Labels})
			}
			raw, err := os.ReadFile(fx + ".golden.json")
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want policyGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s.golden.json: %v", fx, err)
			}
			if !reflect.DeepEqual(got, want) {
				gj, _ := json.MarshalIndent(got, "", "  ")
				t.Errorf("%s: extraction differs from its golden\ngot:\n%s", fx, gj)
			}
		}
	}
}

func tierName(t core.EvidenceTier) string {
	switch t {
	case core.TierVerified:
		return "verified"
	case core.TierPreScan:
		return "pre-scan"
	}
	return string(t)
}

// TestPolicyDispatchClaims: the claim decided by apiVersion group, every
// object document of one dialect, and its negatives — a plain Kubernetes
// manifest, a file mixing a policy with a Deployment, a chainsaw test (a
// kyverno.io subdomain that is another tool), a CLI Values file, an Envoy
// or Kong file, a JSON policy, a manifest that only mentions the group in a
// comment or a nested field.
func TestPolicyDispatchClaims(t *testing.T) {
	cp := "apiVersion: kyverno.io/v1\nkind: ClusterPolicy\nmetadata:\n  name: a\nspec:\n  rules: []\n"
	deploy := "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n"
	tmpl := "apiVersion: templates.gatekeeper.sh/v1\nkind: ConstraintTemplate\nmetadata:\n  name: k8sx\nspec:\n  crd:\n    spec:\n      names:\n        kind: K8sX\n"
	cons := "apiVersion: constraints.gatekeeper.sh/v1beta1\nkind: K8sX\nmetadata:\n  name: x\n"
	cases := []struct {
		path, content, want string
	}{
		{"p/policy.yaml", cp, "kyverno"},
		{"p/policy.yml", "---\n" + cp + "---\n# trailing comment\n", "kyverno"},
		{"p/two.yaml", cp + "---\n" + strings.Replace(cp, "name: a", "name: b", 1), "kyverno"},
		{"p/cel.yaml", "apiVersion: policies.kyverno.io/v1\nkind: ValidatingPolicy\nmetadata:\n  name: v\n", "kyverno"},
		{"p/exc.yaml", "apiVersion: kyverno.io/v2\nkind: PolicyException\nmetadata:\n  name: e\n", "kyverno"},
		{"p/kyverno-test.yaml", "apiVersion: cli.kyverno.io/v1alpha1\nkind: Test\nmetadata:\n  name: t\n", "kyverno"},
		{"p/spaced.yaml", "apiVersion : kyverno.io/v1\nkind : ClusterPolicy\nmetadata:\n  name: s\n", "kyverno"},
		{"p/crlf.yaml", strings.ReplaceAll(cp+"---\n"+cp, "\n", "\r\n"), "kyverno"},
		{"g/template.yaml", tmpl, "gatekeeper"},
		{"g/constraint.yaml", cons, "gatekeeper"},
		{"g/both.yaml", tmpl + "---\n" + cons, "gatekeeper"},
		{"g/suite.yaml", "kind: Suite\napiVersion: test.gatekeeper.sh/v1alpha1\nmetadata:\n  name: s\n", "gatekeeper"},

		{"k8s/deploy.yaml", deploy, ""},
		{"k8s/mixed.yaml", cp + "---\n" + deploy, ""},
		{"k8s/cross.yaml", cp + "---\n" + cons, ""},
		{"t/chainsaw-test.yaml", "apiVersion: chainsaw.kyverno.io/v1alpha1\nkind: Test\nmetadata:\n  name: c\n", ""},
		{"t/values.yaml", "apiVersion: cli.kyverno.io/v1alpha1\nkind: Values\nmetadata:\n  name: v\n", ""},
		{"t/not-object.yaml", cp + "---\n- a list\n", ""},
		{"p/policy.json", `{"apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy"}`, ""},
		{"k8s/comment.yaml", "# apiVersion: kyverno.io/v1\n" + deploy, ""},
		{"k8s/nested.yaml", deploy + "spec:\n  apiVersion: kyverno.io/v1\n", ""},
		{"envoy.yaml", "static_resources:\n  clusters:\n  - name: origin\n", ""},
		{"gw/kong.yml", "_format_version: \"3.0\"\nservices: []\n", ""},
	}
	for _, c := range cases {
		if got := PolicyDialect(c.path, []byte(c.content)); got != c.want {
			t.Errorf("PolicyDialect(%s) = %q, want %q", c.path, got, c.want)
		}
		_, tk, ok := policyDispatch(c.path, []byte(c.content))
		if ok != (c.want != "") || (ok && string(tk) != c.want) {
			t.Errorf("policyDispatch(%s) = (%q, %v), want %q", c.path, tk, ok, c.want)
		}
	}
}

// TestResolveExtractor_PolicyBeforeKubernetes: ingest's dispatch reads a
// Kyverno or Gatekeeper file through its dialect (a DocResulter whose
// result carries the policy_ref targets), a plain manifest stays
// Kubernetes, and an Envoy or Kong file stays its gateway dialect's.
func TestResolveExtractor_PolicyBeforeKubernetes(t *testing.T) {
	p := New(nil, nil, Config{})
	t.Cleanup(p.Close)
	idx := buildSiblingIndex(nil)
	cp := "apiVersion: kyverno.io/v1\nkind: ClusterPolicy\nmetadata:\n  name: a\nspec:\n  rules:\n  - name: r\n    match:\n      any:\n      - resources:\n          kinds: [Deployment]\n"
	for path, want := range map[string]ToolKind{
		"policies/a.yaml":    ToolKyverno,
		"k8s/deploy.yaml":    ToolK8sManifest,
		"envoy.yaml":         ToolEnvoy,
		"gw/kong.yml":        ToolKong,
		"gk/constraint.yaml": ToolGatekeeper,
	} {
		content := map[string]string{
			"policies/a.yaml":    cp,
			"k8s/deploy.yaml":    "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n",
			"envoy.yaml":         "static_resources:\n  clusters:\n  - name: origin\n",
			"gw/kong.yml":        "_format_version: \"3.0\"\nservices:\n- name: a\n  host: a.example.com\n",
			"gk/constraint.yaml": "apiVersion: constraints.gatekeeper.sh/v1beta1\nkind: K8sX\nmetadata:\n  name: x\n",
		}[path]
		_, tk, ok, _ := p.resolveExtractor(path, []byte(content), idx)
		if !ok || tk != want {
			t.Errorf("resolveExtractor(%s) = (%q, %v), want %q", path, tk, ok, want)
		}
	}
	e, _, _, _ := p.resolveExtractor("policies/a.yaml", []byte(cp), idx)
	doc, ok := e.(extractor.DocResulter)
	if !ok {
		t.Fatalf("a Kyverno file's extractor %T is no DocResulter", e)
	}
	res, err := doc.ExtractDoc("s", "policies/a.yaml", []byte(cp))
	if err != nil {
		t.Fatal(err)
	}
	var refs []string
	for _, c := range res.CrossFile {
		refs = append(refs, c.From+" "+c.Edge+" "+c.Strategy+" "+c.Target)
	}
	if want := []string{"r policy_ref resource-kind Deployment"}; !reflect.DeepEqual(refs, want) {
		t.Errorf("cross-file references %q, want %q", refs, want)
	}
}

// policyRepo is a repository with a Kyverno ClusterPolicy and a Gatekeeper
// constraint governing the Deployments and Namespace beside them.
func policyRepo() map[string]string {
	return map[string]string{
		"policies/require-labels.yaml": `apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-labels
spec:
  rules:
  - name: check-deployments
    match:
      any:
      - resources:
          kinds:
          - Deployment
  - name: check-crds
    match:
      any:
      - resources:
          kinds:
          - Widget
`,
		"gatekeeper/template.yaml": `apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8srequiredlabels
spec:
  crd:
    spec:
      names:
        kind: K8sRequiredLabels
  targets:
  - target: admission.k8s.gatekeeper.sh
    rego: |
      package k8srequiredlabels
`,
		"gatekeeper/constraint.yaml": `apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sRequiredLabels
metadata:
  name: ns-must-have-owner
spec:
  match:
    kinds:
    - apiGroups: [""]
      kinds: ["Namespace"]
`,
		"deploy/web.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: shop
  labels:
    app: web
---
apiVersion: v1
kind: Namespace
metadata:
  name: shop
`,
		"deploy/api.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
`,
	}
}

func storedPolicyEdges(t *testing.T, st store.Store, svc string) []string {
	t.Helper()
	all, err := st.TypedEdges(context.Background(), svc)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			got = append(got, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(got)
	return got
}

// TestIngest_PolicyRefs (LB-F136): an ingest stores the policy's targets as
// policy_ref edges under the "policy-ref" typed_edges key — a ClusterPolicy
// rule matching kind Deployment to both Deployments the repository declares
// (k8sextract's nodes), a Gatekeeper constraint to the Namespace — beside
// the doc-xref edge from the constraint to its ConstraintTemplate; the rule
// naming a kind nothing declares is disclosed as undeclared, the policy and
// constraint files read as their dialects; a re-ingest with nothing changed
// skips the pass and keeps the edges; and the edges carry a change to a
// Deployment's file to the rule that governs it in blast radius.
func TestIngest_PolicyRefs(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, policyRepo())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "pol", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"gatekeeper/constraint.yaml|pol::gatekeeper/constraint.K8sRequiredLabels/ns-must-have-owner->pol::deploy/web.Namespace/shop",
		"gatekeeper/constraint.yaml|pol::gatekeeper/constraint.K8sRequiredLabels/ns-must-have-owner->pol::gatekeeper/template.K8sRequiredLabels",
		"policies/require-labels.yaml|pol::policies/require-labels.check-deployments->pol::deploy/api.Deployment/api",
		"policies/require-labels.yaml|pol::policies/require-labels.check-deployments->pol::deploy/web.Deployment/shop/web",
	}
	if got := storedPolicyEdges(t, st, "pol"); !reflect.DeepEqual(got, want) {
		t.Fatalf("stored edges\n got: %q\nwant: %q", got, want)
	}
	if rep.PolicyRefEdges != 3 || rep.PolicyRefSkipped != "" {
		t.Errorf("PolicyRefEdges = %d (skipped %q), want 3", rep.PolicyRefEdges, rep.PolicyRefSkipped)
	}
	if wantBy := map[string]int{PolicyUndeclared: 1}; !reflect.DeepEqual(rep.PolicyRefUnresolved, wantBy) {
		t.Errorf("PolicyRefUnresolved = %v, want %v", rep.PolicyRefUnresolved, wantBy)
	}
	obs, err := st.QueryObservations(ctx, "pol", "")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	var nodes []core.SymbolNode
	for _, o := range obs {
		kinds[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
		nodes = append(nodes, o.ToSymbolNode())
	}
	for key, k := range map[string]string{
		"policies/require-labels.yaml|ClusterPolicy/require-labels":       "KyvernoPolicy",
		"policies/require-labels.yaml|check-deployments":                  "KyvernoRule",
		"gatekeeper/template.yaml|K8sRequiredLabels":                      "GatekeeperConstraintTemplate",
		"gatekeeper/constraint.yaml|K8sRequiredLabels/ns-must-have-owner": "GatekeeperConstraint",
		"deploy/web.yaml|Deployment/shop/web":                             "K8sWorkload",
	} {
		if kinds[key] != k {
			t.Errorf("%s stored as %q, want %q", key, kinds[key], k)
		}
	}

	typed, err := st.TypedEdges(ctx, "pol")
	if err != nil {
		t.Fatal(err)
	}
	reached := impact.BlastRadiusWithTypedEdges(nodes, []string{"deploy/api.yaml"}, typed)
	if !slices.Contains(reached.Files, "policies/require-labels.yaml") {
		t.Errorf("blast radius of deploy/api.yaml %q misses the policy that governs it", reached.Files)
	}
	if without := impact.BlastRadiusWithTypedEdges(nodes, []string{"deploy/api.yaml"}, nil); slices.Contains(without.Files, "policies/require-labels.yaml") {
		t.Errorf("the policy was reached without policy edges: the test proves nothing")
	}

	rep2, err := p.Ingest(ctx, core.ServiceRef{ID: "pol", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if rep2.PolicyRefSkipped != SkippedPassReason {
		t.Errorf("unchanged re-ingest: PolicyRefSkipped = %q, want %q", rep2.PolicyRefSkipped, SkippedPassReason)
	}
	if got := storedPolicyEdges(t, st, "pol"); !reflect.DeepEqual(got, want) {
		t.Errorf("edges after a skipped pass\n got: %q\nwant: %q", got, want)
	}
}

// TestReingestFile_PolicyRepo: a one-file re-ingest reads a policy file
// through its dialect, as ingest did, and invalidates the policy-ref pass.
func TestReingestFile_PolicyRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, policyRepo())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "pol", Path: dir}); err != nil {
		t.Fatal(err)
	}
	rel := "policies/require-labels.yaml"
	body := policyRepo()[rel] + "  - name: check-pods\n    match:\n      any:\n      - resources:\n          kinds: [Pod]\n"
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "pol", dir, rel); err != nil {
		t.Fatal(err)
	}
	obs, _ := st.QueryObservations(ctx, "pol", "")
	got := map[string]string{}
	for _, o := range obs {
		if o.SourceRef == rel {
			got[o.Symbol] = string(o.SourceKind)
		}
	}
	if got["check-pods"] != "KyvernoRule" || got["ClusterPolicy/require-labels"] != "KyvernoPolicy" {
		t.Errorf("after ReingestFile %s reads as %v, want the kyverno dialect's entities", rel, got)
	}
	if p.passStateOK(ctx, "pol", policyRefStateKey) {
		t.Errorf("ReingestFile left the policy-ref pass marked complete")
	}
}

// TestPolicyRefEdgesSurviveTheGoPass (O335): the typed Go pass reconciles
// away every typed_edges key that is not one of its packages; the
// policy-ref key is another pass's.
func TestPolicyRefEdgesSurviveTheGoPass(t *testing.T) {
	if !isNamespacedTypedKey(policyRefPackageKey) || !isNamespacedTypedKey(policyRefStateKey) {
		t.Fatalf("isNamespacedTypedKey does not name the policy-ref keys")
	}
	if !slices.Contains(passKeysFor("deploy/web.yaml"), policyRefStateKey) || !slices.Contains(passKeysFor("iam/policy.json"), policyRefStateKey) {
		t.Errorf("passKeysFor does not invalidate the policy-ref pass for a manifest or a .json")
	}
	if slices.Contains(passKeysFor("main.go"), policyRefStateKey) {
		t.Errorf("a Go file invalidates the policy-ref pass")
	}
}

// TestIngest_ExtractorEpoch_PolicyDialectsForceReExtraction pins epoch 45
// (wave pc; D2: the integrator renumbers it, and this test's prior epoch,
// at merge): a Kyverno or Gatekeeper file fingerprinted at epoch 44 holds
// k8sextract's K8sWorkload reading, so the bump must re-read it through its
// dialect. The seeded fingerprints are salted with "44", the true prior
// epoch.
func TestIngest_ExtractorEpoch_PolicyDialectsForceReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"policies/require-labels.yaml": policyRepo()["policies/require-labels.yaml"],
		"gatekeeper/template.yaml":     policyRepo()["gatekeeper/template.yaml"],
	}
	writeCorpus(t, dir, files)
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "epoch45", Path: dir}
	ctx := context.Background()
	for name, src := range files {
		old := sha256.Sum256(append([]byte(src), []byte("\x00fathom-extractor-epoch:44")...))
		if err := st.SetFingerprint(ctx, svc.ID, name, hex.EncodeToString(old[:])); err != nil {
			t.Fatal(err)
		}
	}
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Skipped != 0 {
		t.Fatalf("%d file(s) skipped: a pre-epoch-45 fingerprint was honored", rep.Skipped)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, o := range obs {
		got[o.SourceRef+"|"+string(o.SourceKind)] = true
	}
	for _, want := range []string{"policies/require-labels.yaml|KyvernoPolicy", "gatekeeper/template.yaml|GatekeeperConstraintTemplate"} {
		if !got[want] {
			t.Errorf("re-extraction did not read %s", want)
		}
	}
}
