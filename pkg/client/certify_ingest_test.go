package client_test

// certify_ingest_test.go — O1271: the daemon's ingest (IngestService,
// IngestServiceAt: the ingest MCP tool and auto-refresh) runs the same
// post-ingest certifications pass the CLI's plain `fathom ingest` runs,
// so a certificate in the daemon's store moves when the daemon
// re-ingests, and a service that declares nothing pays one count read.
import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/testevidence"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func certifyIngestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out.String())
	}
}

func certifyIngestWrite(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func certifyIngestMarkedTest(name, target string) string {
	return "package cart\n\nimport \"testing\"\n\n// certifies: invariant " + target + " (asserts)\nfunc " + name + "(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal(\"sum\")\n\t}\n}\n"
}

// certifyIngestCountingStore counts the reads and writes the post-ingest
// certifications pass makes, so a test can tell how far it went.
type certifyIngestCountingStore struct {
	store.Store
	declReads, declWrites, certClaimReads int
}

func (s *certifyIngestCountingStore) CertifyDeclarationsForService(ctx context.Context, service string) ([]store.CertifyDeclaration, error) {
	s.declReads++
	return s.Store.CertifyDeclarationsForService(ctx, service)
}

func (s *certifyIngestCountingStore) PutCertifyDeclarations(ctx context.Context, service string, decls []store.CertifyDeclaration) error {
	s.declWrites++
	return s.Store.PutCertifyDeclarations(ctx, service, decls)
}

func (s *certifyIngestCountingStore) ClaimsForService(ctx context.Context, service, layer string) ([]core.SemanticClaim, error) {
	if layer == certify.Layer {
		s.certClaimReads++
	}
	return s.Store.ClaimsForService(ctx, service, layer)
}

func certifyIngestStatusByTest(t *testing.T, st store.Store, service string) map[string]core.ClaimStatus {
	t.Helper()
	claims, err := st.ClaimsForService(context.Background(), service, certify.Layer)
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]core.ClaimStatus, len(claims))
	for _, c := range claims {
		var m certify.Meta
		if err := json.Unmarshal(c.Meta, &m); err != nil {
			t.Fatalf("claim %s meta: %v", c.ClaimID, err)
		}
		out[m.Test] = c.Status
	}
	return out
}

// TestDaemonIngestRefreshesCertificates: a certificate compiled into the
// daemon's store (as a CLI verb would) goes stale when a pkg/client ingest
// runs after its test moved to another file.
func TestDaemonIngestRefreshesCertificates(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	root := t.TempDir()
	certifyIngestGit(t, root, "init", "-q")
	certifyIngestGit(t, root, "config", "commit.gpgsign", "false")
	certifyIngestGit(t, root, "config", "user.name", "Alice")
	certifyIngestGit(t, root, "config", "user.email", "alice@example.com")
	certifyIngestWrite(t, root, "go.mod", "module example.com/cart\n\ngo 1.22\n")
	certifyIngestWrite(t, root, "cart.go", "package cart\n\n// Add sums two ints.\nfunc Add(a, b int) int { return a + b }\n")
	certifyIngestWrite(t, root, "moved_test.go", certifyIngestMarkedTest("TestMoved", "I1"))
	certifyIngestWrite(t, root, "docs/intent/README.md", "# Router\n\ndocs/intent/invariants.md is the living doc.\n")
	certifyIngestWrite(t, root, "docs/intent/invariants.md", "# Invariants\n\n| # | Statement | Enforced by |\n|---|---|---|\n| I1 | Sums never overflow | test (`TestMoved`) |\n")
	certifyIngestGit(t, root, "add", "-A")
	certifyIngestGit(t, root, "commit", "-q", "-m", "c0")

	st, err := store.Open(filepath.Join(t.TempDir(), "daemon.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	if err := c.IngestServiceAt(ctx, "cart-svc", root); err != nil {
		t.Fatalf("IngestServiceAt: %v", err)
	}

	// The certificate a CLI verb would compile into this store.
	meta, found, err := st.GetServiceMeta(ctx, "cart-svc")
	if err != nil || !found || meta.IndexedCommit == "" {
		t.Fatalf("service meta after ingest: found=%v commit=%q err=%v", found, meta.IndexedCommit, err)
	}
	if _, err := testevidence.IngestCertifyMarkers(ctx, st, "cart-svc", meta.IndexedCommit, root); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	if _, err := certify.CompileCertifications(ctx, st, "cart-svc", meta.IndexedCommit, now, root, certify.ExecGit); err != nil {
		t.Fatal(err)
	}
	if got := certifyIngestStatusByTest(t, st, "cart-svc")["TestMoved"]; got == core.ClaimStale || got == "" {
		t.Fatalf("TestMoved before the move = %q, want a compiled, non-stale certificate", got)
	}

	if err := os.Remove(filepath.Join(root, "moved_test.go")); err != nil {
		t.Fatal(err)
	}
	certifyIngestWrite(t, root, "elsewhere_test.go", certifyIngestMarkedTest("TestMoved", "I1"))
	if err := c.IngestService(ctx, "cart-svc"); err != nil {
		t.Fatalf("IngestService: %v", err)
	}
	if got := certifyIngestStatusByTest(t, st, "cart-svc")["TestMoved"]; got != core.ClaimStale {
		t.Errorf("TestMoved after a daemon ingest of the move = %q, want %q", got, core.ClaimStale)
	}
}

// TestDaemonIngestWithoutDeclarationsSkipsCertify: a service with no
// certification declared pays one declaration read per daemon ingest and
// stops: no marker scan writes declarations, no certificate is read.
func TestDaemonIngestWithoutDeclarationsSkipsCertify(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	certifyIngestWrite(t, root, "x.go", "package x\n\nfunc Foo() {}\n")
	certifyIngestWrite(t, root, "x_test.go", "package x\n\nimport \"testing\"\n\nfunc TestFoo(t *testing.T) { Foo() }\n")

	cs := &certifyIngestCountingStore{Store: store.NewMemoryStore()}
	c := client.New(connector.NewLocalConnector(), cs)
	if err := c.IngestServiceAt(ctx, "bare-svc", root); err != nil {
		t.Fatalf("IngestServiceAt: %v", err)
	}
	if err := c.IngestService(ctx, "bare-svc"); err != nil {
		t.Fatalf("IngestService: %v", err)
	}
	if cs.declReads != 2 || cs.declWrites != 0 || cs.certClaimReads != 0 {
		t.Errorf("declaration reads = %d, declaration writes = %d, certificate reads = %d; want 2, 0, 0: one count read per ingest and nothing more",
			cs.declReads, cs.declWrites, cs.certClaimReads)
	}
}
