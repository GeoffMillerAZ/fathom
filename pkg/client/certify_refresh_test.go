package client

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func captureCertifyLog(t *testing.T) (*dlog.Logger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	return dlog.New(&buf, dlog.LevelInfo, func() time.Time { return now }), &buf
}

func TestDaemonCertifySkipsOversizeGoFile(t *testing.T) {
	logger, buf := captureCertifyLog(t)
	ctx := context.Background()
	root := t.TempDir()
	for rel, src := range map[string]string{
		"a_generated.go": "// Code generated. DO NOT EDIT.\npackage cart\n" + strings.Repeat(" ", 3<<20),
		"cart_test.go":   "package cart\nimport \"testing\"\n// certifies: invariant I1\nfunc TestCart(t *testing.T) {}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })
	if err := st.PutCertifyDeclarations(ctx, "svc", []store.CertifyDeclaration{{TestSymbolKey: "svc::cart.TestCart", Kind: "invariant", TargetID: "I1"}}); err != nil {
		t.Fatal(err)
	}
	c := New(connector.NewLocalConnector(), st).WithLogger(logger)
	if err := c.IngestServiceAt(ctx, "svc", root); err != nil {
		t.Fatal(err)
	}
	decls, err := st.CertifyDeclarationsForService(ctx, "svc")
	if err != nil || len(decls) != 1 {
		t.Fatalf("declarations=%+v err=%v", decls, err)
	}
	claims, err := st.ClaimsForService(ctx, "svc", certify.Layer)
	if err != nil || len(claims) != 1 {
		t.Errorf("marker claim lost beside oversized file: claims=%+v err=%v", claims, err)
	}
	if !strings.Contains(buf.String(), "oversize_skipped=1") {
		t.Errorf("oversize count missing from daemon log: %s", buf.String())
	}
}

type certifyRefreshErrorStore struct {
	store.Store
	phase string
}

var errCertifyRefresh = errors.New("certify read failed")

func (s *certifyRefreshErrorStore) CertifyDeclarationsForService(ctx context.Context, svc string) ([]store.CertifyDeclaration, error) {
	if s.phase == "declarations" {
		return nil, errCertifyRefresh
	}
	return s.Store.CertifyDeclarationsForService(ctx, svc)
}

func (s *certifyRefreshErrorStore) GetServiceMeta(ctx context.Context, svc string) (store.ServiceMeta, bool, error) {
	if s.phase == "watermark" {
		return store.ServiceMeta{}, false, errCertifyRefresh
	}
	return s.Store.GetServiceMeta(ctx, svc)
}

func (s *certifyRefreshErrorStore) PutCertifyDeclarations(ctx context.Context, svc string, decls []store.CertifyDeclaration) error {
	if s.phase == "scan" {
		return errCertifyRefresh
	}
	return s.Store.PutCertifyDeclarations(ctx, svc, decls)
}

func (s *certifyRefreshErrorStore) ClaimsForService(ctx context.Context, svc, layer string) ([]core.SemanticClaim, error) {
	if s.phase == "compile" {
		return nil, errCertifyRefresh
	}
	return s.Store.ClaimsForService(ctx, svc, layer)
}

func TestDaemonCertifyLogsRefreshErrors(t *testing.T) {
	for _, phase := range []string{"declarations", "watermark", "scan", "compile"} {
		t.Run(phase, func(t *testing.T) {
			logger, buf := captureCertifyLog(t)
			ctx := context.Background()
			st := &certifyRefreshErrorStore{Store: store.NewMemoryStore(), phase: phase}
			t.Cleanup(func() { _ = st.Close() })
			if err := st.Store.PutCertifyDeclarations(ctx, "svc", []store.CertifyDeclaration{{TestSymbolKey: "svc::cart.TestCart", Kind: "invariant", TargetID: "I1"}}); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "cart_test.go"), []byte("package cart\n// certifies: invariant I1\nfunc TestCart() {}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			New(connector.NewLocalConnector(), st).WithLogger(logger).refreshCertifications(ctx, "svc", root, Report{})
			if !strings.Contains(buf.String(), errCertifyRefresh.Error()) {
				t.Errorf("%s failure silently swallowed: %q", phase, buf.String())
			}
		})
	}
}
