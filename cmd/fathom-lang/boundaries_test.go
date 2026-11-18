package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const boundaryTestRules = `{"schema_version":1,"rules":[{"id":"no-network","from":["app/**/*.go"],"deny":["go:net/http/**"],"reason":"Keep the application offline."}]}`

func TestBoundariesCommand(t *testing.T) {
	t.Setenv("PATH", "")
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/example\n")
	write(t, root, "app/app.go", "package app\nimport _ \"net/http\"\n")
	write(t, root, "app/okay/okay.go", "package okay\n")
	write(t, root, "outside.go", "package outside\nimport _ \"net/http\"\n")
	write(t, root, "rules.json", boundaryTestRules)
	write(t, root, "invalid.json", `{"schema_version":1,"rules":[]}`)
	if err := os.Symlink(filepath.Join(root, "outside.go"), filepath.Join(root, "app", "link")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		args   []string
		exit   int
		stdout string
		stderr string
	}{
		{"help", []string{"--help"}, 0, "fathom-lang boundaries", ""},
		{"missing flag", nil, 2, "", "rules"},
		{"invalid flag", []string{"-bad=" + root}, 2, "", "flags"},
		{"bad rules", []string{"--rules", filepath.Join(root, "invalid.json")}, 2, "", "invalid"},
		{"missing rule file", []string{"--rules", filepath.Join(root, "missing.json")}, 1, "", "cannot open"},
		{"missing root", []string{"--root", filepath.Join(root, "not-a-root"), "--rules", filepath.Join(root, "rules.json")}, 1, "", "cannot open"},
		{"escape", []string{"--rules", filepath.Join(root, "rules.json"), "../outside.go"}, 2, "", "path"},
		{"absolute", []string{"--rules", filepath.Join(root, "rules.json"), filepath.Join(root, "outside.go")}, 2, "", "path"},
		{"duplicate", []string{"--rules", filepath.Join(root, "rules.json"), "app/app.go", "app/app.go"}, 2, "", "duplicate"},
		{"unreadable", []string{"--rules", filepath.Join(root, "rules.json"), "app/absent.go"}, 1, "unverified", "could not be read"},
		{"symlink", []string{"--rules", filepath.Join(root, "rules.json"), "app/link/escaped.go"}, 1, "unverified", "could not be read"},
		{"clean", []string{"--rules", filepath.Join(root, "rules.json"), "--trust-unmeasured", "go", "app/okay/okay.go", "outside.go"}, 0, "clean", ""},
		{"unverified", []string{"--rules", filepath.Join(root, "rules.json"), "app/okay/okay.go"}, 0, "unverified", "unverified"},
		{"strict", []string{"--rules", filepath.Join(root, "rules.json"), "--strict", "app/okay/okay.go"}, 1, "unverified", "unverified"},
		{"empty scope", []string{"--rules", filepath.Join(root, "rules.json"), "--strict", "outside.go"}, 1, "unverified", "no governed files"},
		{"default scope", []string{"--rules", filepath.Join(root, "rules.json")}, 1, "violated", "go:net/http"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"boundaries", "--root", root}, tc.args...)
			out, errw, code := runBoundariesTest(t, args)
			if code != tc.exit || !strings.Contains(out, tc.stdout) || !strings.Contains(errw, tc.stderr) ||
				(tc.stdout == "" && out != "") || (tc.stderr == "" && errw != "") || strings.Contains(out+errw, root) {
				t.Fatalf("exit %d; stdout=%q stderr=%q", code, out, errw)
			}
			if strings.HasPrefix(out, "{") && !json.Valid([]byte(out)) {
				t.Fatalf("stdout is not one JSON document: %q", out)
			}
		})
	}
}

func TestBoundariesJSONGolden(t *testing.T) {
	var priorOut, priorErr string
	for i := range 2 {
		root := t.TempDir()
		write(t, root, "go.mod", "module example.com/example\n")
		write(t, root, "app/app.go", "package app\nimport _ \"net/http\"\n")
		write(t, root, "app/okay/okay.go", "package okay\n")
		write(t, root, "rules.json", boundaryTestRules)
		paths := []string{"app/okay/okay.go", "app/app.go"}
		if i == 1 {
			paths[0], paths[1] = paths[1], paths[0]
		}
		args := append([]string{"boundaries", "--root", root, "--rules", filepath.Join(root, "rules.json")}, paths...)
		out, errw, code := runBoundariesTest(t, args)
		if code != 1 || strings.Contains(out+errw, root) {
			t.Fatalf("exit %d, stdout=%q stderr=%q", code, out, errw)
		}
		wantFindings := "no-network: violated app/app.go:2 -> go:net/http\nno-network: unverified app/app.go\nno-network: unverified app/okay/okay.go\n"
		if errw != wantFindings {
			t.Fatalf("findings: %q", errw)
		}
		if i == 0 {
			priorOut, priorErr = out, errw
			checkGolden(t, "boundaries.json", []byte(out))
		} else if priorOut != out || priorErr != errw {
			t.Fatal("output changed across roots and input order")
		}
	}
}

func runBoundariesTest(t *testing.T, args []string) (string, string, int) {
	t.Helper()
	var out, errw bytes.Buffer
	code := run(t.Context(), args, strings.NewReader(""), &out, &errw)
	return out.String(), errw.String(), code
}

func TestBoundariesCanceledRead(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/example\n")
	write(t, root, "app/okay.go", "package okay\n")
	write(t, root, "rules.json", boundaryTestRules)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var out, errw bytes.Buffer
	args := []string{"boundaries", "--root", root, "--rules", filepath.Join(root, "rules.json"), "app/okay.go"}
	code := run(ctx, args, strings.NewReader(""), &out, &errw)
	if code != 1 || !strings.Contains(out.String(), `"matched_files": 1`) ||
		!strings.Contains(errw.String(), "unverified app/okay.go\n") {
		t.Fatalf("canceled scope: exit %d; stdout=%q stderr=%q", code, &out, &errw)
	}
}

func TestBoundariesOutputError(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/example\n")
	write(t, root, "app/okay.go", "package okay\n")
	write(t, root, "rules.json", boundaryTestRules)
	var errw bytes.Buffer
	code := run(t.Context(), []string{"boundaries", "--root", root, "--rules", filepath.Join(root, "rules.json")}, strings.NewReader(""),
		boundaryBrokenWriter{}, &errw)
	if code != 1 || !strings.Contains(errw.String(), "cannot write") {
		t.Fatalf("exit %d, stderr=%q", code, errw.String())
	}
}

type boundaryBrokenWriter struct{}

func (boundaryBrokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
