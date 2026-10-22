package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

func TestWrapperUnknownCwdSkipsGateAndLastCall(t *testing.T) {
	original := gatePreflightToplevel
	t.Cleanup(func() { gatePreflightToplevel = original })
	gatePreflightToplevel = func(context.Context, string) ([]byte, error) {
		t.Fatal("unknown wrapper cwd reached gate preflight")
		return nil, nil
	}
	repo := testRunRepo(t, true)
	writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ngate_preflight = true\n"+wrapperConfig)
	for _, verb := range []string{"commit", "push"} {
		command := "command-wrapper label 'git " + verb + "'"
		if line := gatePreflightLine(repo, repo, command, "wrapper-session", "http://127.0.0.1:0"); line != "" && !strings.Contains(line, "not checked (wrapper cwd)") {
			t.Errorf("unknown cwd preflight: %q", line)
		}
		raw, err := json.Marshal(map[string]any{"type": "assistant", "cwd": repo, "message": map[string]any{"content": []any{map[string]any{"type": "tool_use", "name": "Bash", "input": map[string]any{"command": command}}}}})
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(t.TempDir(), "transcript.jsonl")
		if err := os.WriteFile(file, raw, 0600); err != nil {
			t.Fatal(err)
		}
		if dir, ok := lastToolCallDir(file, repo); ok || dir != "" {
			t.Errorf("last call directory=%q,%v", dir, ok)
		}
	}
}

func TestWrapperConfigErrorKeepsPlainCommandsReadable(t *testing.T) {
	repo := t.TempDir()
	writeRepoFile(t, repo, ".fathom/config.toml", "[shell]\nwrappers = [invalid\n")
	for _, command := range []string{"go test ./...", "rg needle .", "printf x > file.go", "git push"} {
		got := parseShellCommand(command, repo)
		if want := shellcmd.ParseCommand(command); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %+v; want %+v", command, got, want)
		}
	}
	if paths, shaped := bashWriteTargets("printf x > file.go", repo); !shaped || !reflect.DeepEqual(paths, []string{"file.go"}) {
		t.Errorf("writes=%v shaped=%v", paths, shaped)
	}
}

func TestWrapperDeclarationsLoadOnceAcrossLexes(t *testing.T) {
	repo := t.TempDir()
	writeRepoFile(t, repo, ".fathom/config.toml", wrapperConfig)
	original := shellConfigLoad
	t.Cleanup(func() { shellConfigLoad = original })
	loads := 0
	shellConfigLoad = func(user, project string) (config.Loaded, error) { loads++; return original(user, project) }
	const commands = 16
	for i := 0; i < commands; i++ {
		c := parseShellCommand("uv run go test ./...", repo)
		if c.Unreadable || c.Units()[0].Name() != "go" {
			t.Fatalf("declaration lost: %+v", c)
		}
	}
	if loads != 1 {
		t.Fatalf("config loads=%d for %d lexes; want 1", loads, commands)
	}
}

func TestWrapperDeclarationsRequireExplicitRoot(t *testing.T) {
	repo := t.TempDir()
	writeRepoFile(t, repo, ".fathom/config.toml", wrapperConfig)
	t.Chdir(repo)
	for _, roots := range [][]string{nil, {""}} {
		got := parseShellCommand("uv run go test ./...", roots...)
		if want := shellcmd.ParseCommand("uv run go test ./..."); !reflect.DeepEqual(got, want) {
			t.Errorf("rootless parse borrowed process cwd: %+v", got)
		}
	}
}

func TestWrapperCwdFlowsToTranscriptAndPinReaders(t *testing.T) {
	repo := t.TempDir()
	writeRepoFile(t, repo, ".fathom/config.toml", wrapperConfig)
	b := &replayBlock{Name: "Bash", ID: "read-call", Input: json.RawMessage(`{"command":"uv run cat file.go"}`)}
	fl, _ := roFollowerOf(b, repo)
	if !reflect.DeepEqual(fl.reads, []string{"file.go"}) {
		t.Errorf("report follower lost cwd: %v", fl.reads)
	}
	if fl := blFollowerOf(b, repo); !reflect.DeepEqual(fl.files, []string{"file.go"}) {
		t.Errorf("baseline follower lost cwd: %v", fl.files)
	}
	if !blRunsInvocation("uv run go test ./...", [][]string{{"go", "test"}}, repo) {
		t.Error("baseline matcher lost cwd")
	}
	if _, ok := lastGitCommitDir("uv run git commit -m x", repo); !ok {
		t.Error("pin reader lost cwd")
	}
	raw, err := json.Marshal(map[string]any{"cwd": repo, "message": map[string]any{"content": []any{map[string]any{"type": "tool_use", "name": "Bash", "id": "read-call", "input": map[string]string{"command": "uv run cat file.go"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	open := map[string]*roCall{}
	readoutReportLine(raw, 0, map[string]bool{"read-call": true}, map[string]*roCall{}, open, map[string]*roFollower{})
	if call := open["read-call"]; call == nil || call.cwd != repo {
		t.Fatalf("report dropped transcript cwd: %+v", call)
	}
}

func TestWrapperConfigFailureIsLoadedOnce(t *testing.T) {
	repo := t.TempDir()
	writeRepoFile(t, repo, ".fathom/config.toml", "[shell]\nwrappers = [invalid\n")
	original := shellConfigLoad
	t.Cleanup(func() { shellConfigLoad = original })
	loads := 0
	shellConfigLoad = func(user, project string) (config.Loaded, error) { loads++; return original(user, project) }
	for i := 0; i < 16; i++ {
		if got := parseShellCommand("go test ./...", repo); got.Unreadable {
			t.Fatalf("fallback unreadable: %+v", got)
		}
	}
	if loads != 1 {
		t.Errorf("error retried %d times; want 1 load", loads)
	}
}
