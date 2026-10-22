package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestTestDeltaHookSkipsUnknownAndPartialRows(t *testing.T) {
	for _, middle := range []string{"unknown", "piped", "truncated", "other scope"} {
		t.Run(middle, func(t *testing.T) {
			readoutHookEnv(t, "[readouts]\nmode = \"rich\"\n")
			t.Setenv("FATHOM_HTTP", "http://localhost")
			serveHookRequests(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
			repo := t.TempDir()
			gitInitTempRepo(t, repo)
			writeRepoFile(t, repo, "Taskfile.yml", "version: '3'\ntasks:\n  test:\n    cmds: [\"go test ./...\"]\n")
			writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntest_run = true\n")
			writeRepoFile(t, repo, "money/money.go", "package money\n")
			head := commitAll(t, repo)
			output := goFailOutput(deltaMoney, "TestA")
			_, first, _, _ := runReadoutHook(t, []string{"--event", "tool-failure"}, testRunFailurePayload(repo, "go test ./...", output))
			if strings.Contains(first, "delta vs") || lastTestRunRow(t)["delta"] != "not-compared" {
				t.Fatalf("first run: output=%q row=%v", first, lastTestRunRow(t))
			}
			row := map[string]string{"session": "tr-session", "invocation": "go test", "result": "fail", "scope": "./...", "head": head, "tree": "t1"}
			switch middle {
			case "unknown":
				row["result"] = "unknown"
			case "piped", "truncated":
				row[middle] = "true"
			case "other scope":
				row["scope"] = "./cart/"
			}
			emitBeat(testRunChannel, row["result"], row)
			_, last, _, _ := runReadoutHook(t, []string{"--event", "tool-failure"}, testRunFailurePayload(repo, "go test ./...", output))
			if !strings.Contains(last, "still failing money.TestA (since "+head[:7]+")") || strings.Contains(last, "new money.TestA") || lastTestRunRow(t)["delta"] != "compared" {
				t.Fatalf("lost decided baseline: output=%q row=%v", last, lastTestRunRow(t))
			}
		})
	}
}
