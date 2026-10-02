package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestDryNames(t *testing.T) {
	out := []byte("task: [b] echo b\ntask: [a] echo a\nsomething else\n\x1b[32mtask: [ns:x] go build\n\x1b[0m")
	got := dryNames(out)
	want := map[string]bool{"a": true, "b": true, "ns:x": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dryNames = %v, want %v", got, want)
	}
}

// The ablation needs the real tool and yq; it is skipped without them.
func TestAblateRef(t *testing.T) {
	if _, err := exec.LookPath("task"); err != nil {
		t.Skip("task not on PATH")
	}
	if _, err := exec.LookPath("yq"); err != nil {
		t.Skip("yq not on PATH")
	}
	dir := t.TempDir()
	src := "version: '3'\nvars:\n  DYN:\n    sh: exit 1\ntasks:\n  a:\n    deps: [b, quiet]\n    cmds:\n      - echo a\n      - task: c\n  b:\n    cmds: [echo b]\n  c:\n    cmds: [echo c]\n  quiet: {}\n  e:\n    preconditions:\n      - sh: exit 1\n    deps: [b]\n  r:\n    requires:\n      vars: [REG, {name: MODE, enum: [fast, slow]}]\n    cmds:\n      - task: b\n"
	file := filepath.Join(dir, "Taskfile.yml")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	r := runner{task: "task", yq: "yq", timeout: time.Minute, env: os.Environ()}
	ctx := context.Background()
	// No subject shell runs: a dynamic variable or precondition that fails would stop the tool.
	if err := r.neutralize(ctx, file); err != nil {
		t.Fatal(err)
	}
	ab := newAblator(r)
	for _, c := range []struct {
		from, to string
		want     bool
	}{
		{"a", "b", true},      // b leaves the run
		{"a", "c", true},      // a task: call leaves the run
		{"a", "quiet", false}, // a task that runs nothing is not observable
		{"e", "b", true},      // the precondition no longer stops the run
		{"r", "b", true},      // a required variable is given a placeholder (an enum its first value)
	} {
		got, err := ab.attest(ctx, dir, file, c.from, c.to)
		if err != nil {
			t.Fatalf("%s -> %s: %v", c.from, c.to, err)
		}
		if got != c.want {
			t.Errorf("%s -> %s attested %v, want %v", c.from, c.to, got, c.want)
		}
	}
	if b, _ := os.ReadFile(file); string(b) == src {
		t.Error("the file must hold the neutralized copy, not the subject's bytes")
	}
}

func TestRequiredVars(t *testing.T) {
	got := requiredVars([]byte(`{"a": {"requires": {"vars": ["REG", {"name": "MODE", "enum": ["fast", "slow"]}]}}, "b": ["echo"], "c": {}}`))
	want := []string{"MODE=fast", "REG=example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("requiredVars = %v, want %v", got, want)
	}
}
