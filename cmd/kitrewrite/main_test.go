package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRunCheckRewriteReverse drives the command the way the tasks do: -check
// refuses an upstream-form mirror, a forward rewrite fixes it, -kit -reverse
// restores one copy's upstream form.
func TestRunCheckRewriteReverse(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/owner/app\n\ngo 1.26\n")
	write("main.go", "package main\n\nimport _ \"github.com/geoffmilleraz/app-kit/appkit\"\n\nfunc main() {}\n")
	write("third_party/app-kit/go.mod", "module github.com/geoffmilleraz/app-kit\n")
	write("third_party/app-kit/appkit/a.go", "package appkit\n")

	if rc := run([]string{"-check", root}); rc != 1 {
		t.Fatalf("-check on the upstream form: exit %d, want 1", rc)
	}
	if rc := run([]string{root}); rc != 0 {
		t.Fatalf("forward: exit %d", rc)
	}
	if rc := run([]string{"-check", root}); rc != 0 {
		t.Fatalf("-check after forward: exit %d, want 0", rc)
	}
	if rc := run([]string{"-reverse", "-kit", filepath.Join(root, "third_party", "app-kit"), root}); rc != 0 {
		t.Fatalf("-kit -reverse: exit %d", rc)
	}
	if _, err := os.Stat(filepath.Join(root, "third_party", "app-kit", "go.mod")); err != nil {
		t.Fatalf("-kit -reverse did not restore go.mod: %v", err)
	}
	if rc := run([]string{"a", "b"}); rc != 2 {
		t.Fatalf("two roots: exit %d, want 2", rc)
	}
}
