package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeRepoFile writes rel under root, creating parents.
func writeRepoFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestMatchDeclaredTestCommand pins TR-1 (docs/plans/test-run-hook.md,
// design section 3): the test-run hook acts only on a Bash command that
// invokes one of the repository's DECLARED test commands — a Taskfile task,
// a Makefile target, a package.json script, a `[tests] commands` entry, or
// the runner one of them invokes — and on nothing else.
func TestMatchDeclaredTestCommand(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, "Taskfile.yml", `version: '3'
vars:
  GO: bash scripts/go-wrapper.sh
tasks:
  test:
    desc: all tests
    cmds: ["{{.GO}} test -race -p 4 ./..."]
  validate:
    deps: [vet]
    cmds: [{ task: test }]
  vet:
    cmds: ["{{.GO}} vet ./..."]
  lint:
    cmds:
      - golangci-lint run
`)
	writeRepoFile(t, root, "Makefile", "GO ?= go\n\n.PHONY: check build\ncheck:\n\t$(GO) test ./...\n\nbuild:\n\t$(GO) build ./...\n")
	writeRepoFile(t, root, "web/package.json", `{"scripts": {"build": "tsc"}}`)
	writeRepoFile(t, root, "package.json", `{"name": "x", "scripts": {"test": "vitest run", "test:e2e": "playwright test", "build": "tsc -p ."}}`)

	cases := []struct {
		name, cmd, invocation string
		piped, match          bool
	}{
		{"a declared task", "task test", "task test", false, true},
		{"a task that runs the test task", "task validate", "task validate", false, true},
		{"a task that runs no tests", "task lint", "", false, false},
		{"a task whose runner is not a test runner", "task vet", "", false, false},
		{"the runner a declared task invokes, narrowed and piped", "cd /repo && GOWORK=off go test -race -p 2 ./internal/testrun/ 2>&1 | tail -40", "go test", true, true},
		{"the runner behind env and time", "env CGO_ENABLED=1 time go test -run TestX ./cmd/fathom", "go test", false, true},
		{"the runner by path", "/usr/local/go/bin/go test ./...", "go test", false, true},
		{"another go subcommand", "go vet ./...", "", false, false},
		{"the runner named inside an argument", "echo 'go test ./...'", "", false, false},
		{"the runner inside a commit message", `git commit -m "go test fix"`, "", false, false},
		{"a Makefile target that runs the runner", "make check", "make check", false, true},
		{"a Makefile target that does not", "make build", "", false, false},
		{"the package.json test script", "npm test", "npm test", false, true},
		{"the same script through run", "npm run test -- --reporter dot", "npm run test", false, true},
		{"a test-named script", "pnpm test:e2e", "pnpm test:e2e", false, true},
		{"a non-test script", "npm run build", "", false, false},
		{"the script's runner through npx", "npx vitest run src/", "vitest", false, true},
		{"a later segment after a failing build", "npm run build; npm test | tee out.txt", "npm test", true, true},
		{"a subshell", "(cd sub && task test)", "task test", false, true},
		{"empty", "", "", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, ok := matchDeclaredTestCommand(root, tc.cmd, nil)
			if ok != tc.match {
				t.Fatalf("matchDeclaredTestCommand(%q) ok=%v (%+v), want %v", tc.cmd, ok, m, tc.match)
			}
			if !ok {
				return
			}
			if m.Invocation != tc.invocation {
				t.Errorf("invocation = %q, want %q", m.Invocation, tc.invocation)
			}
			if m.Piped != tc.piped {
				t.Errorf("piped = %v, want %v", m.Piped, tc.piped)
			}
			if m.Source == "" {
				t.Error("a match must name where the command was declared")
			}
		})
	}

	t.Run("[tests] commands", func(t *testing.T) {
		bare := t.TempDir()
		if _, ok := matchDeclaredTestCommand(bare, "go test ./...", nil); ok {
			t.Fatal("a repository that declares nothing matched go test")
		}
		m, ok := matchDeclaredTestCommand(bare, "./bin/itest --fast", []string{"bin/itest", "go test"})
		if !ok || m.Invocation != "bin/itest" || m.Source != ".fathom/config.toml [tests] commands" {
			t.Errorf("configured command: %+v ok=%v", m, ok)
		}
		if m, ok := matchDeclaredTestCommand(bare, "go test -count=1 ./x", []string{"go test"}); !ok || m.Invocation != "go test" {
			t.Errorf("configured runner: %+v ok=%v", m, ok)
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		a, _ := matchDeclaredTestCommand(root, "go test ./...", nil)
		for i := 0; i < 5; i++ {
			if b, _ := matchDeclaredTestCommand(root, "go test ./...", nil); b != a {
				t.Fatalf("run %d: %+v != %+v", i, b, a)
			}
		}
	})
}
