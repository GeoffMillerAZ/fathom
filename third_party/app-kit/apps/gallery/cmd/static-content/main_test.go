package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func staticExportCanonicalTempDir(t *testing.T) string {
	t.Helper()
	// Resolve the host's temporary-root alias for valid export fixtures. This
	// does not change the command's refusal of explicitly supplied symlinks.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestStaticExportCommandRefusesBeforeCreatingOutput(t *testing.T) {
	root := staticExportCanonicalTempDir(t)
	css := filepath.Join(root, "input.css")
	if err := os.WriteFile(css, []byte("body { color: var(--ak-color-ink); }"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.css")
	if err := os.Symlink(css, link); err != nil {
		t.Fatal(err)
	}
	large := filepath.Join(root, "large.css")
	if err := os.WriteFile(large, bytes.Repeat([]byte("x"), maxCSSBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		args []string
		code int
	}{
		{"missing", nil, 2},
		{"unknown", []string{"-unknown"}, 2},
		{"relative-css", []string{"-css", "relative.css"}, 2},
		{"missing-css", []string{"-css", filepath.Join(root, "missing.css")}, 1},
		{"directory-css", []string{"-css", root}, 1},
		{"symlink-css", []string{"-css", link}, 1},
		{"oversize-css", []string{"-css", large}, 1},
		{"invalid-base", []string{"-css", css, "-base-path", "//outside/"}, 2},
		{"dot-base", []string{"-css", css, "-base-path", "/a/../"}, 2},
		{"positional", []string{"-css", css, "extra"}, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			out := filepath.Join(root, "out-"+test.name)
			var stdout, stderr bytes.Buffer
			args := append([]string{"-out", out}, test.args...)
			if code := run(args, &stdout, &stderr); code != test.code || stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if _, err := os.Lstat(out); !os.IsNotExist(err) {
				t.Fatalf("refusal created output: %v", err)
			}
		})
	}
}

func TestStaticExportCommandHelpAndDelivery(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-h"}, &stdout, &stderr); code != 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "-out") {
		t.Fatalf("help code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	root := staticExportCanonicalTempDir(t)
	css := filepath.Join(root, "input.css")
	if err := os.WriteFile(css, []byte("body { color: var(--ak-color-ink); }"), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "site")
	args := []string{"-out", out, "-css", css}
	stdout.Reset()
	stderr.Reset()
	if code := run(args, &stdout, &stderr); code != 0 || stderr.Len() != 0 || !strings.Contains(stdout.String(), out) {
		t.Fatalf("export code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	before, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(args, &stdout, &stderr); code != 1 || stderr.Len() == 0 || stdout.Len() != 0 {
		t.Fatalf("repeat code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	after, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("repeat modified existing export", err)
	}
	args[1] = filepath.Join(root, "stdout-failure")
	stderr.Reset()
	if code := run(args, refusedWriter{}, &stderr); code != 1 || !strings.Contains(stderr.String(), "files were exported") {
		t.Fatalf("output failure code=%d stderr=%q", code, stderr.String())
	}
}

type refusedWriter struct{}

func (refusedWriter) Write([]byte) (int, error) { return 0, errors.New("synthetic output failure") }
