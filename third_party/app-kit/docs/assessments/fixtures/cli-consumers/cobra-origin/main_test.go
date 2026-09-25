package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotHashIsOrderIndependentAndByteSensitive(t *testing.T) {
	spec := moduleSpec{Name: "example.invalid/module", Version: "v1.0.0"}
	a := map[string][]byte{"a.go": []byte("package a\n"), "LICENSE": []byte("fixture\n")}
	b := map[string][]byte{"LICENSE": []byte("fixture\n"), "a.go": []byte("package a\n")}
	x, err := treeSum(spec, a)
	if err != nil {
		t.Fatal(err)
	}
	y, err := treeSum(spec, b)
	if err != nil || x != y {
		t.Fatalf("order changed sum: %q %q %v", x, y, err)
	}
	b["a.go"][0] = 'P'
	z, err := treeSum(spec, b)
	if err != nil || x == z {
		t.Fatalf("byte mutation not detected: %q %q %v", x, z, err)
	}
}

func TestReadTreeRefusesLinksAndLargeFiles(t *testing.T) {
	for _, kind := range []string{"file-link", "directory-link", "large-file"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "unsafe")
			if kind == "large-file" {
				f, err := os.Create(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := f.Truncate((4 << 20) + 1); err != nil {
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				target := t.TempDir()
				if kind == "file-link" {
					target = filepath.Join(target, "target")
					if err := os.WriteFile(target, []byte("fixture"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := readTree(root); err == nil {
				t.Fatal("unsafe source accepted")
			}
		})
	}
}

func TestPrepareRefusesDriftBeforeCreatingOutput(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "source")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "go.mod"), []byte("module spoofed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(parent, "candidate")
	err := prepare(source, source, out)
	if err == nil || !strings.Contains(err.Error(), "source mismatch") {
		t.Fatalf("wanted hash refusal, got %v", err)
	}
	if _, err := os.Lstat(out); !os.IsNotExist(err) {
		t.Fatalf("failed preparation wrote output: %v", err)
	}
}

func TestPrepareRefusesExistingOutputWithoutOverwriting(t *testing.T) {
	out := t.TempDir()
	marker := filepath.Join(out, "keep")
	if err := os.WriteFile(marker, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepare("", "", out); err == nil || !strings.Contains(err.Error(), "must not exist") {
		t.Fatalf("wanted existing-output refusal, got %v", err)
	}
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "unchanged" {
		t.Fatalf("existing output changed: %q %v", data, err)
	}
}

func TestPatchRefusesUnknownConstructorLayout(t *testing.T) {
	spec := moduleSpec{Name: "github.com/spf13/cobra", Addition: "cobra_invocation.go.txt"}
	if err := patch(spec, map[string][]byte{"args.go": []byte("package cobra\n")}); err == nil {
		t.Fatal("unknown patch layout accepted")
	}
}

func TestWithinDoesNotConfuseSiblingPrefixes(t *testing.T) {
	if !within("/fixture/kit", "/fixture/kit/child") || within("/fixture/kit", "/fixture/kit-other") || within("/fixture/kit", "/fixture") {
		t.Fatal("incorrect path containment")
	}
}
