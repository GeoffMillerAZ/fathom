package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestRemap_UpdatesServicePath(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	oldRoot, newRoot := t.TempDir(), t.TempDir()

	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ctx := context.Background()
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "mod", Path: oldRoot, IndexedCommit: "manifest:abc"}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var out, errBuf bytes.Buffer
	code := run([]string{"remap", "--store", storePath, "--service", "mod", "--path", newRoot}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("remap exit = %d, stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), newRoot) {
		t.Errorf("output %q does not name the new root", out.String())
	}

	st2, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("re-open store: %v", err)
	}
	defer st2.Close()
	meta, found, err := st2.GetServiceMeta(ctx, "mod")
	if err != nil || !found {
		t.Fatalf("GetServiceMeta: found=%v err=%v", found, err)
	}
	if meta.Path != newRoot {
		t.Errorf("Path = %q, want %q", meta.Path, newRoot)
	}
	if meta.IndexedCommit != "manifest:abc" {
		t.Errorf("IndexedCommit = %q, want preserved watermark", meta.IndexedCommit)
	}
}

func TestRemap_Errors(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	dir := t.TempDir()
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	cases := []struct {
		name string
		args []string
	}{
		{"missing flags", []string{"remap", "--store", storePath}},
		{"path not a dir", []string{"remap", "--store", storePath, "--service", "mod", "--path", filepath.Join(dir, "nope")}},
		{"unknown service", []string{"remap", "--store", storePath, "--service", "ghost", "--path", dir}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errBuf bytes.Buffer
			if code := run(tc.args, &out, &errBuf); code == 0 {
				t.Errorf("exit = 0, want non-zero; stderr: %s", errBuf.String())
			}
		})
	}
}
