package iacfacts

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// The guard and its family and metamorphic tests are internal/hclbound's. These tests hold the wiring: the Terraform
// reader and the binding reader both go through the guard, and a refusal reads as ErrLimit with the bound that fired.

func TestReaderRefusalStatesTheBoundThatFired(t *testing.T) {
	for name, c := range map[string]struct{ src, want string }{
		"nesting":  {"x = " + strings.Repeat("!", 4*MaxDepth) + "true\n", "nesting deeper than 128"},
		"template": {"x = <<A\n" + strings.Repeat("a\n", 30_000) + "A\n", "melding budget"},
		"function": {"x = a" + strings.Repeat("::a", 5000) + "()\n", "`::` segments"},
	} {
		_, err := ReadTerraformFile("a.tf", []byte(c.src))
		if !errors.Is(err, ErrLimit) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want ErrLimit naming %q, got %v", name, c.want, err)
		}
		if body, _ := (&bindingReader{}).parse([]byte(c.src), "z.tf"); body != nil {
			t.Errorf("%s: binding must skip the file", name)
		}
	}
	// A 20,000 line heredoc still reads.
	set, err := ReadTerraformFile("a.tf", []byte("resource \"t\" \"n\" {\n  x = <<A\n"+strings.Repeat("a line\n", 20_000)+"A\n}\n"))
	if err != nil || len(set.Resources) != 1 || set.SyntaxErrors != 0 {
		t.Fatalf("a 20,000 line heredoc must read: %v", err)
	}
}

// IAC34 and IAC35 reach the binding reader inside its 1 MiB read bound.
func TestBindingSkipsAndDisclosesAFileThePastTheGuard(t *testing.T) {
	for name, expr := range map[string]string{
		"spaced splat comment": "a" + strings.Repeat("[/**/*]", 140_000),
		"spaced splat newline": "(a" + strings.Repeat("[\n*]", 200_000) + ")",
		"skipped template":     "<<A\n${a[*<<B\nB\n]}\n" + strings.Repeat("a\n", 200_000),
		"invalid directive":    "<<A\n" + strings.Repeat("%{}a\n", 200_000) + "A",
	} {
		fsys := fstest.MapFS{"main.tf": file(reusableModule), "z.tf": file("x = " + expr + "\n")}
		b := bindBlock(fsys, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_lb_listener"})
		if !strings.Contains(strings.Join(b.Disclosures, "|"), "past the read bound") {
			t.Errorf("%s: binding did not disclose the skipped file: %v", name, b.Disclosures)
		}
	}
}

func isLimit(err error) bool { return errors.Is(err, ErrLimit) }

// Real files are never refused: every .tf of the corpus named by IACFACTS_TF_CORPUS reads without ErrLimit and
// binds without a skipped file. Without the variable the test has nothing to read.
func TestHCLBoundDoesNotRefuseRealFiles(t *testing.T) {
	root := os.Getenv("IACFACTS_TF_CORPUS")
	if root == "" {
		t.Skip("IACFACTS_TF_CORPUS names a directory of real Terraform modules")
	}
	files := 0
	walkTF(t, root, func(path string, src []byte) {
		files++
		if _, err := ReadTerraformFile(path, src); isLimit(err) {
			t.Errorf("%s: refused: %v", path, err)
		}
		if body, _ := (&bindingReader{}).parse(src, path); body == nil {
			t.Errorf("%s: binding skipped the file", path)
		}
	})
	if files == 0 {
		t.Fatalf("no .tf file under the corpus")
	}
	t.Logf("%d files read", files)
}

// walkTF calls fn for every .tf and .tofu file under root that fits the read bound.
func walkTF(t *testing.T, root string, fn func(path string, src []byte)) {
	t.Helper()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if n := d.Name(); n == ".git" || n == ".terraform" {
				return filepath.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(p); ext == ".tf" || ext == ".tofu" {
			if src, err := os.ReadFile(p); err == nil && len(src) <= MaxFileBytes {
				fn(p, src)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
