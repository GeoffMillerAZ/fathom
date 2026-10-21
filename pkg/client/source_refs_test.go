package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/scope"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestSourceRefProjectionRefusesPolicyDrift(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("[FR-1]"), 0600); err != nil {
		t.Fatal(err)
	}
	conn := &connector.LocalConnector{}
	policy, err := conn.DiscoveryPolicyDigest(ctx, core.ServiceRef{ID: "svc", Path: root})
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256([]byte("[FR-1]"))
	st := store.NewMemoryStore()
	meta := store.ServiceMeta{ServiceID: "svc", Path: root, IndexedCommit: "head", IndexedAt: 1}
	p := &store.SourceRefProjection{Policy: policy, ExtractorVersion: store.SourceRefExtractorVersion, Complete: true, Rows: []store.SourceRefFact{{Path: "a.go", SourceHash: hex.EncodeToString(h[:]), ExtractorVersion: store.SourceRefExtractorVersion, Refs: []string{"[FR-1]"}}}}
	if err := st.WriteServiceSummary(ctx, meta, store.ServiceSummary{RefProjection: p}, []store.FileStat{{Path: "a.go"}}); err != nil {
		t.Fatal(err)
	}
	c := New(conn, st)
	if _, found, err := c.SourceRefProjection(ctx, "svc"); err != nil || !found {
		t.Fatalf("valid projection missing: found=%v err=%v", found, err)
	}
	if err := os.WriteFile(filepath.Join(root, "new.go"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, found, err := c.SourceRefProjection(ctx, "svc"); err != nil || found {
		t.Fatalf("membership drift accepted: found=%v err=%v", found, err)
	}
	if err := os.Remove(filepath.Join(root, "new.go")); err != nil {
		t.Fatal(err)
	}
	conn.RespectGitignore = true
	if _, found, err := c.SourceRefProjection(ctx, "svc"); err != nil || found {
		t.Fatalf("policy drift accepted: found=%v err=%v", found, err)
	}
}

func TestSourceRefProjectionRefusesSuppressedMember(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suppressions.jsonl")
	t.Setenv(scope.EnvVar, path)
	c, _, _ := windowFixture(t)
	if _, found, err := c.SourceRefProjection(context.Background(), "svc"); err != nil || !found {
		t.Fatalf("unsuppressed projection: found=%v err=%v", found, err)
	}
	if _, err := scope.AppendSuppression(path, scope.Suppression{Service: "svc", Pattern: "a.go", Reason: "private fixture", CreatedAt: time.Now().UTC(), CreatedBy: scope.OriginCLI}); err != nil {
		t.Fatal(err)
	}
	if _, found, err := c.SourceRefProjection(context.Background(), "svc"); err != nil || found {
		t.Fatalf("suppressed source facts served: found=%v err=%v", found, err)
	}
}

func TestSourceRefProjectionFreshGitClientKeepsGitMode(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cmd := exec.CommandContext(ctx, "git", "init", "-q", root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("[FR-1]"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := core.ServiceRef{ID: "svc", Path: root}
	producer := &connector.LocalConnector{RespectGitignore: true}
	if _, err := producer.Discover(ctx, svc); err != nil {
		t.Fatal(err)
	}
	policy, err := producer.DiscoveryPolicyDigest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256([]byte("[FR-1]"))
	st := store.NewMemoryStore()
	p := &store.SourceRefProjection{Policy: policy, ExtractorVersion: store.SourceRefExtractorVersion, Complete: true, Rows: []store.SourceRefFact{{Path: "a.go", SourceHash: hex.EncodeToString(h[:]), ExtractorVersion: store.SourceRefExtractorVersion, Refs: []string{"[FR-1]"}}}}
	if err := st.WriteServiceSummary(ctx, store.ServiceMeta{ServiceID: "svc", Path: root, IndexedCommit: "head", IndexedAt: 1}, store.ServiceSummary{RefProjection: p}, []store.FileStat{{Path: "a.go"}}); err != nil {
		t.Fatal(err)
	}
	reader := New(&connector.LocalConnector{RespectGitignore: true}, st)
	if _, found, err := reader.SourceRefProjection(ctx, "svc"); err != nil || !found {
		t.Fatalf("fresh Git-mode reader rejected sealed projection: found=%v err=%v", found, err)
	}
}
