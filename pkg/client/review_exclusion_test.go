package client_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestReadSourceFilesPreservesOversizeExclusion(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "generated.lock"), bytes.Repeat([]byte("x"), 3<<20), 0600); err != nil {
		t.Fatal(err)
	}
	st := store.NewMemoryStore()
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: root}); err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), st)
	files, err := c.ReadSourceFiles(ctx, "svc", []string{"generated.lock"})
	var over *connector.OversizeError
	if len(files) != 0 || !errors.As(err, &over) || over.Size != 3<<20 {
		t.Fatalf("files=%v err=%v, want named oversize error", files, err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package sample\n"), 0600); err != nil {
		t.Fatal(err)
	}
	files, err = c.ReadSourceFiles(ctx, "svc", []string{"generated.lock", "a.go"})
	if len(files) != 1 || files[0].Path != "a.go" || !errors.As(err, &over) {
		t.Fatalf("batch files=%v err=%v, want readable files beside the exclusion", files, err)
	}
}
