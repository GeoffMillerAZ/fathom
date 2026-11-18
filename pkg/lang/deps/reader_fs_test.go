package deps

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

type openOnlyFS struct{ fs.FS }

type noDirEntriesFS struct{ fs.FS }

func (f noDirEntriesFS) Open(name string) (fs.File, error) {
	file, err := f.FS.Open(name)
	if err != nil {
		return nil, err
	}
	return struct{ fs.File }{file}, nil
}

func TestDependencyRepositoryComponents(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	write := func(root, name, content string) {
		t.Helper()
		file := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(root, "go.mod", "module example.com/control\n")
	write(root, "safe/app.go", "package app\n")
	write(outside, "sub/app.go", "package app\n")
	write(outside, "sub/asset.txt", "fixture")
	write(outside, "go.mod", "module example.org/control\n")
	for name, dest := range map[string]string{"linked": outside, "safe/go.work": filepath.Join(outside, "go.mod")} {
		if err := os.Symlink(dest, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}

	sub, err := fs.Sub(openOnlyFS{os.DirFS(filepath.Dir(root))}, filepath.Base(root))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		view fs.FS
	}{
		{"lstat", os.DirFS(root)},
		{"parent entries", openOnlyFS{os.DirFS(root)}},
		{"sub of open-only FS", sub},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver := resolverFunc(func(view fs.FS, from, raw string) ([]Edge, *Unknown) {
				if _, err := fs.ReadFile(view, "linked/sub/asset.txt"); err == nil {
					t.Error("resolver can read through a symlink ancestor")
				}
				if _, err := fs.ReadDir(view, "linked/sub"); err == nil {
					t.Error("resolver can list through a symlink ancestor")
				}
				if _, err := fs.Stat(view, "linked/sub"); err == nil {
					t.Error("resolver can stat through a symlink ancestor")
				}
				if _, err := fs.ReadFile(view, "safe/go.work"); err == nil {
					t.Error("resolver can read a symlinked config")
				}
				if _, err := fs.ReadFile(view, "safe/app.go"); err != nil {
					t.Errorf("resolver cannot read a regular repository file: %v", err)
				}
				return []Edge{
					{Kind: Import, Resolution: File, To: "linked/sub/asset.txt"},
					{Kind: Import, Resolution: Directory, To: "linked/sub"},
				}, nil
			})
			r, err := New(TrustUnmeasured("go"), WithResolver("go", resolver))
			if err != nil {
				t.Fatal(err)
			}
			batch, err := r.ReadFiles(context.Background(), tc.view, []Input{{Path: "linked/sub/app.go"}, {Path: "safe/app.go"}})
			if err == nil || len(batch) != 2 || batch[0].Complete || !hasUnknown(batch[0], MissingConfig) || len(batch[0].Edges) != 0 {
				t.Errorf("unsafe batch source: %+v, %v", batch, err)
			}
			if len(batch) != 2 || hasEdge(batch[1], ConfigRef, File, "safe/go.work") || !hasUnknown(batch[1], MissingConfig) ||
				!hasEdge(batch[1], ConfigRef, File, "go.mod") {
				t.Errorf("unsafe configuration must be disclosed and safe configuration retained: %+v", batch)
			}

			got, err := r.ReadFile(context.Background(), tc.view, "main.go", "go", []byte("package main\nimport _ \"example.org/dependency\"\n"))
			if err != nil || got.Complete || !hasUnknown(got, UnresolvedAlias) ||
				hasEdge(got, Import, File, "linked/sub/asset.txt") || hasEdge(got, Import, Directory, "linked/sub") {
				t.Errorf("unsafe extension targets: %+v, %v", got, err)
			}
		})
	}

	r, err := New(TrustUnmeasured("go"))
	if err != nil {
		t.Fatal(err)
	}
	batch, err := r.ReadFiles(context.Background(), noDirEntriesFS{os.DirFS(root)}, []Input{{Path: "safe/app.go"}})
	if err == nil || len(batch) != 1 || batch[0].Complete || !hasUnknown(batch[0], MissingConfig) || len(batch[0].Edges) != 0 {
		t.Errorf("batch accepted components it could not inspect: %+v, %v", batch, err)
	}
}
