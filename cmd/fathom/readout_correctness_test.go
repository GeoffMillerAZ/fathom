package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/readout"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

func TestCompileReadoutRequiresCompilerEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, command, output string
		want                  int
	}{
		{"gofmt ignored directory", "go build ./...; command gofmt -l .", "_reference/bad.go:3:1: expected 'package', found 'func'\n", 0},
		{"deadcode after successful vet", "go build ./... && go vet ./...; echo VET-OK; deadcode ./cmd/...", "VET-OK\ncmd/tool/main.go:4:6: unreachable func: unused\n", 0},
		{"module error without header", "go build ./...", "main.go:3:2: no required module provides package example.com/missing; to add it:\n", 0},
		{"missing module sum without header", "go build ./...", "main.go:3:2: missing go.sum entry for module providing package example.com/missing; to add:\n", 0},
		{"missing module package without header", "go build ./...", "main.go:3:2: module example.com/missing@latest found, but does not contain package example.com/missing/sub\n", 0},
		{"embed error without header", "go build ./...", "assets.go:5:12: pattern missing/*: no matching files found\n", 0},
		{"headerless compiler error after tail", "cd . && go build ./... | tail -20", "cart.go:10:20: undefined: Add\n", 1},
		{"headerless vet error after head", "go vet ./... | head -20", "cart.go:10:20: fmt.Printf format %d has arg s of wrong type string\n", 1},
		{"compiler header", "go build ./...", "# example.com/m/cart\ncart.go:10:20: undefined: Add\n", 1},
		{"embed error with header", "go build ./...", "# example.com/m/cart\nassets.go:5:12: pattern missing/*: no matching files found\n", 1},
		{"mixed compiler and deadcode", "go build ./...; deadcode ./cmd/...", "# example.com/m/cart\ncart.go:10:20: undefined: Add\ncmd/tool/main.go:4:6: unreachable func: unused\n", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := shellcmd.ParseCommand(tc.command)
			in := harness.HookInput{Shell: harness.ShellOutput{Stdout: tc.output}}
			got := readoutCompileLine(context.Background(), in, t.TempDir(), &c, time.Time{})
			if got.records != tc.want {
				t.Fatalf("compile records = %d, want %d: %q", got.records, tc.want, got.line)
			}
		})
	}
}

func TestTestDeltaPartialBaseNeverLabelsNew(t *testing.T) {
	for _, partial := range []string{"piped", "truncated"} {
		t.Run(partial, func(t *testing.T) {
			testRunRepo(t, false)
			emitBeat(testRunChannel, "fail", map[string]string{
				"session": "s-1", "invocation": "go test", "result": "fail", partial: "true",
				"head": "h1", "tree": "t1", "scope": "./...",
			})
			prev, ok := sessionTestRunRows(beatLogPath(), "s-1", "")
			if !ok || len(prev) != 1 {
				t.Fatalf("previous runs = %+v, read=%v", prev, ok)
			}
			line, _ := readout.RunDelta(prev, readout.TestRunNow{
				Invocation: "go test", Head: "h2", Tree: "t2", Scope: "./...", Failing: []string{"example.com/m/cart.TestA"},
			})
			if strings.Contains(line, "new ") || !strings.Contains(line, "not compared") {
				t.Fatalf("partial failing baseline: %q", line)
			}
		})
	}
}

func TestTestRunLogsTruncationAndOmitsUnknownReach(t *testing.T) {
	repo := testRunRepo(t, true)
	t.Setenv("FATHOM_HOME", t.TempDir())
	serveHookRequests(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity" {
			_ = json.NewEncoder(w).Encode(identityResult{Schema: 1})
			return
		}
		joinRPCText(w, map[string]any{"services": []any{}})
	}))
	var payload map[string]any
	if err := json.Unmarshal([]byte(testRunPostPayload(repo, "go test ./...", testRunFailingOutput)), &payload); err != nil {
		t.Fatal(err)
	}
	payload["tool_response"].(map[string]any)["persistedOutputPath"] = "workspace/tmp/captured.log"
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	code, out := runTestRunHook(t, "test-run", repo, "http://localhost", string(b))
	if code != 0 || !strings.Contains(out, "TestRound") || strings.Contains(out, "no changed symbol reaches") {
		t.Fatalf("truncated run exit=%d output=%q", code, out)
	}
	if row := lastTestRunRow(t); row["truncated"] != "true" || row["result"] != "fail" {
		t.Fatalf("truncated run row = %+v", row)
	}
}
