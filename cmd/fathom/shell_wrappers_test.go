package main

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/readout"
)

const wrapperConfig = `[shell]
wrappers = [{ prefix = "uv run" }, { command = "command-wrapper", script_arg = 2 }, { command = "directory-wrapper", script_arg = 2, cwd_arg = 1 }]
`

func TestDeclaredWrappersReachBashSeams(t *testing.T) {
	repo := testRunRepo(t, true)
	t.Chdir(repo)
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntest_run = true\n"+wrapperConfig)
	for _, command := range []string{`uv run go test ./...`, `command-wrapper label 'go test ./...'`, `command-wrapper label "go test ./... && go test ./money"`} {
		if m, ok := matchDeclaredTestCommand(repo, command, nil); !ok || m.Invocation != "go test" {
			t.Errorf("test-run misses %q: %+v, %v", command, m, ok)
		}
	}
	if got := shellDiscoveryKind(`command-wrapper label 'rg needle .'`); got != "shell-search" {
		t.Errorf("discovery kind = %q", got)
	}
	if target, tool := bashObservedReadPath(`command-wrapper label 'cat file.go'`); target != "file.go" || tool != "read" {
		t.Errorf("observed read = %q, %q", target, tool)
	}
	if paths, shaped := bashWriteTargets(`command-wrapper label 'printf x > file.go'`); !shaped || !reflect.DeepEqual(paths, []string{"file.go"}) {
		t.Errorf("write targets = %v, %v", paths, shaped)
	}
	if paths, _ := bashWriteTargets(`command-wrapper label 'printf x > a.go' | command-wrapper label 'tee b.go'`); !reflect.DeepEqual(paths, []string{"a.go", "b.go"}) {
		t.Errorf("piped wrapper writes = %v", paths)
	}
	if paths, _ := bashWriteTargets(`directory-wrapper child 'printf x > file.go'; printf x > other.go`); !reflect.DeepEqual(paths, []string{"child/file.go", "other.go"}) {
		t.Errorf("wrapper cwd leaked = %v", paths)
	}
	if m, ok := matchDeclaredTestCommand(repo, `command-wrapper label 'go test ./...' | tail -20`, nil); !ok || !m.Piped {
		t.Errorf("wrapped pipeline = %+v, %v", m, ok)
	}
	c := parseShellCommand(`command-wrapper label 'git commit -m test && git push'`, repo)
	if commit, push, _ := gatePreflightVerbs(c); !commit || !push {
		t.Errorf("gate preflight misses wrapper: commit=%v push=%v", commit, push)
	}
	rs, err := readout.LoadRules("")
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{`go test ./...`, `uv run go test ./...`, `command-wrapper label 'go test ./...'`} {
		c := parseShellCommand(command, repo).Transparent()
		ro := readout.Render(rs, readout.Input{Command: &c, Stdout: testRunPassingOutput, DeclaredTest: true, Mode: readout.ModeObserve})
		if ro.Rule != "go-test@1" {
			t.Errorf("readout classifier misses %q: rule=%q reason=%q", command, ro.Rule, ro.Reason)
		}
	}
}

func TestDeclaredWrapperTestRunFailureRow(t *testing.T) {
	repo := testRunRepo(t, true)
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntest_run = true\n"+wrapperConfig)
	commitAll(t, repo)
	var baseline map[string]string
	for _, command := range []string{`go test ./...`, `command-wrapper label 'go test ./...'`, `directory-wrapper '` + repo + `' 'go test ./...'`} {
		var out bytes.Buffer
		payload := testRunFailurePayload(repo, command, testRunFailingOutput)
		before := len(testRunBeats(t))
		code := cmdTestRunHook(harness.ClaudeCodec(), harness.EventToolFailure, "failed-test-run", "http://127.0.0.1:0", "", strings.NewReader(payload), &out)
		if code != 0 || out.Len() == 0 || len(testRunBeats(t)) != before+1 {
			t.Fatalf("%q: exit=%d output=%q rows=%v", command, code, out.String(), testRunBeats(t))
		}
		row := lastTestRunRow(t)
		if baseline == nil {
			baseline = row
			continue
		}
		for _, key := range []string{"invocation", "result", "failing", "piped", "head", "tree", "scope", "emitted"} {
			if row[key] != baseline[key] {
				t.Errorf("%q: %s = %q, unwrapped = %q", command, key, row[key], baseline[key])
			}
		}
	}
}

func TestDeclaredWrapperCwdKeysTheActualRepository(t *testing.T) {
	repo := testRunRepo(t, true)
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntest_run = true\n"+wrapperConfig)
	ownerHead := commitAll(t, repo)
	child := filepath.Join(repo, "child")
	writeRepoFile(t, child, "marker.txt", "different repository\n")
	gitInitTempRepo(t, child)
	writeRepoFile(t, child, "Taskfile.yml", "version: '3'\ntasks:\n  test:\n    cmds: [\"go test ./...\"]\n")
	writeRepoFile(t, child, ".fathom/config.toml", "[hooks]\ntest_run = true\n")
	childHead := commitAll(t, child)
	for _, tc := range []struct{ command, head string }{
		{`command-wrapper label 'go test ./...'`, ownerHead},
		{`command-wrapper label 'cd child && go test ./...'`, ownerHead},
		{`directory-wrapper child 'go test ./...'`, childHead},
	} {
		var out bytes.Buffer
		payload := testRunFailurePayload(repo, tc.command, testRunFailingOutput)
		before := len(testRunBeats(t))
		cmdTestRunHook(harness.ClaudeCodec(), harness.EventToolFailure, "failed-test-run", "http://127.0.0.1:0", "", strings.NewReader(payload), &out)
		if len(testRunBeats(t)) != before+1 {
			t.Fatalf("%s: no row", tc.command)
		}
		if row := lastTestRunRow(t); row["head"] != tc.head || row["scope"] != "./..." {
			t.Errorf("%s: head=%q scope=%q; want head=%q scope=./...", tc.command, row["head"], row["scope"], tc.head)
		}
	}
}

func TestDeclaredWrappersInTaskDeclarations(t *testing.T) {
	for _, tc := range []struct{ file, body, invocation string }{
		{"Taskfile.yml", "version: '3'\ntasks:\n  check:\n    cmds: [\"uv run go test ./...\"]\n", "task check"},
		{"Makefile", "check:\n\tuv run go test ./...\n", "make check"},
		{"package.json", `{"scripts":{"check":"command-wrapper label 'go test ./...'"}}`, "npm run check"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			root := t.TempDir()
			writeRepoFile(t, root, ".fathom/config.toml", wrapperConfig)
			writeRepoFile(t, root, tc.file, tc.body)
			if _, ok := matchDeclaredTestCommand(root, tc.invocation, nil); !ok {
				t.Errorf("wrapped test runner did not declare %s", tc.invocation)
			}
		})
	}
}

func TestDeclaredWrappersKeepGuardMatchingUnchanged(t *testing.T) {
	repo := testRunRepo(t, true)
	writeRepoFile(t, repo, ".fathom/config.toml", wrapperConfig)
	writeRepoFile(t, repo, ".fathom/guard.cue", readoutGuardFixture)
	for _, tc := range []struct {
		command string
		warn    bool
	}{
		{`go test ./...`, true},
		{`uv run go test ./...`, false},
		{`command-wrapper label 'go test ./...'`, false},
	} {
		c := parseShellCommand(tc.command, repo).Transparent()
		r := readoutRichLines(context.Background(), harness.HookInput{Command: tc.command}, repo, &c, time.Time{})
		if got := strings.Contains(strings.Join(r.lines, "\n"), "lacks -race"); got != tc.warn {
			t.Errorf("%q: guard warning=%v; want %v", tc.command, got, tc.warn)
		}
	}
}

func TestDeclaredWrapperWithoutEnvelopeCwdUsesOverride(t *testing.T) {
	repo := testRunRepo(t, true)
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntest_run = true\n"+wrapperConfig)
	var out bytes.Buffer
	payload := testRunFailurePayload("", `command-wrapper label 'go test ./...'`, testRunFailingOutput)
	cmdTestRunHook(harness.ClaudeCodec(), harness.EventToolFailure, "failed-test-run", "http://127.0.0.1:0", repo, strings.NewReader(payload), &out)
	if out.Len() == 0 {
		t.Fatal("--dir fallback did not load the wrapper declaration")
	}
	if row := lastTestRunRow(t); row["scope"] != "./..." {
		t.Fatalf("scope=%q; want ./...", row["scope"])
	}
}
