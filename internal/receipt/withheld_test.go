package receipt

// withheld_test.go — EVX-1's withheld state reuses ADR-065's classifier
// (moved here from internal/mcp/secret_withhold.go so the anchor core and
// the MCP read tools share one rule, never two lists).

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/scope"
)

func TestSecretFiles_KubernetesSecretAndPlainFiles(t *testing.T) {
	nodes := []core.SymbolNode{
		{Name: "Deployment/web", Kind: core.KindK8sWorkload, FilePath: "deploy/web.yaml", Signature: "apps/v1"},
		{Name: "Secret/prod/db", Kind: core.KindK8sWorkload, FilePath: "deploy/secret.yaml", Signature: "v1"},
		{Name: "Secret/other", Kind: core.KindK8sWorkload, FilePath: "deploy/secret.yaml", Signature: "v1"},
		{Name: "Secret/not-core", Kind: core.KindK8sWorkload, FilePath: "deploy/crd.yaml", Signature: "example.io/v1"},
		{Name: "main", Kind: core.KindGoFunc, FilePath: "main.go"},
	}
	got := SecretFiles(nodes)
	if len(got) != 1 || got["deploy/secret.yaml"] != "Kubernetes Secret" {
		t.Fatalf("SecretFiles = %v, want only deploy/secret.yaml (a core v1 Secret)", got)
	}
	if k := SecretValuedKind(nodes[1]); k != "Kubernetes Secret" {
		t.Errorf("SecretValuedKind(core Secret) = %q", k)
	}
	if k := SecretValuedKind(nodes[0]); k != "" {
		t.Errorf("SecretValuedKind(Deployment) = %q, want none", k)
	}
}

func TestWithheldReason_SecretOrSuppressed(t *testing.T) {
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	set, err := scope.NewSet([]scope.Suppression{{
		ID: "s1", Service: "svc", Pattern: "vendor-notes/", Reason: "third-party notes", CreatedAt: now.Add(-time.Hour), CreatedBy: "test",
	}})
	if err != nil {
		t.Fatal(err)
	}
	secret := map[string]string{"deploy/secret.yaml": "Kubernetes Secret"}
	if r := withheldReason("deploy/secret.yaml", secret, set, "svc", now); r == "" {
		t.Error("a secret-bearing file is not withheld")
	}
	if r := withheldReason("vendor-notes/a.md", secret, set, "svc", now); r == "" {
		t.Error("a suppressed file is not withheld")
	}
	if r := withheldReason("vendor-notes/a.md", secret, set, "other", now); r != "" {
		t.Errorf("another service's suppression withholds: %q", r)
	}
	if r := withheldReason("main.go", secret, set, "svc", now); r != "" {
		t.Errorf("a plain file is withheld: %q", r)
	}
}
