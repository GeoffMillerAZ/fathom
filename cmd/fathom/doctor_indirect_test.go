package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func doctorHookFixture(t *testing.T, command string) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	hooks := map[string]any{}
	for _, spec := range fathomHookSpecsAt(root) {
		cmd := spec.marker
		if spec.sub == "prompt-hook" {
			cmd = command
		}
		group := map[string]any{"matcher": spec.matcher, "hooks": []any{map[string]any{"type": "command", "command": cmd}}}
		entries, _ := hooks[spec.event].([]any)
		hooks[spec.event] = append(entries, group)
	}
	data, err := json.Marshal(map[string]any{"hooks": hooks})
	if err != nil {
		t.Fatal(err)
	}
	doctorWriteFixture(t, root, ".claude/settings.json", string(data))
	gitRepo(t, root)
	return root
}

func doctorWriteFixture(t *testing.T, root, path, content string) {
	t.Helper()
	p := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorRecognizesWrapperHook(t *testing.T) {
	for _, mode := range []string{"script", "task", "lefthook"} {
		t.Run(mode, func(t *testing.T) {
			command := "./scripts/hooks/prompt.sh"
			if mode == "task" {
				command = "task hooks:prompt"
			}
			if mode == "lefthook" {
				command = "lefthook run prompt"
			}
			root := doctorHookFixture(t, command)
			doctorWriteFixture(t, root, "scripts/hooks/prompt.sh", "#!/bin/sh\ntouch executed\nexec fathom prompt-hook\n")
			doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  hooks:prompt:\n    cmds:\n      - ./scripts/hooks/prompt.sh\n  unrelated:\n    cmds: ['fathom stop-hook']\n")
			doctorWriteFixture(t, root, "lefthook.yml", "prompt:\n  commands:\n    wiring:\n      run: ./scripts/hooks/prompt.sh\n")
			got := checkHooksInstalled(root, "claude")
			if got.Status != doctorPass || !strings.Contains(got.Reason, "configured (indirect)") {
				t.Fatalf("wrapper wiring = %+v", got)
			}
			if _, err := os.Stat(filepath.Join(root, "executed")); !os.IsNotExist(err) {
				t.Fatal("wrapper was executed")
			}
		})
	}
}

func TestDoctorWrapperBeyondLimitIsUnknown(t *testing.T) {
	root := doctorHookFixture(t, "./one.sh")
	doctorWriteFixture(t, root, "one.sh", "exec ./two.sh\n")
	doctorWriteFixture(t, root, "two.sh", "exec ./three.sh\n")
	doctorWriteFixture(t, root, "three.sh", "exec fathom prompt-hook\n")
	got := checkHooksInstalled(root, "claude")
	if got.Status != doctorStatus("unknown") {
		t.Fatalf("beyond limit = %+v", got)
	}
	if doctorExitCode([]doctorCheck{got}) != 0 {
		t.Fatal("unknown must retain fail-open exit contract")
	}
}

type doctorSlowTransport struct{}

func (doctorSlowTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithDeadline(req.Context(), time.Unix(1, 0))
	defer cancel()
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestDoctorProbeTimeoutReportsDaemonFailure(t *testing.T) {
	_, got := checkDaemonReachable(&http.Client{Transport: doctorSlowTransport{}}, "http://daemon.invalid")
	if got.Status != doctorFail {
		t.Fatalf("expired probe = %+v", got)
	}
	if doctorExitCode([]doctorCheck{got}) != 1 {
		t.Fatal("daemon timeout must fail the diagnosis")
	}
}

func TestDoctorConfiguredIsNotFiring(t *testing.T) {
	root := doctorHookFixture(t, "fathom prompt-hook")
	got := checkHooksInstalled(root, "claude")
	if got.Status != doctorPass || !strings.Contains(got.Reason, "configured") || strings.Contains(got.Reason, "firing") {
		t.Fatalf("configuration = %+v", got)
	}
	t.Setenv("FATHOM_CALL_LOG", filepath.Join(root, "absent.jsonl"))
	firing := checkHooksFiring(&http.Client{}, "http://daemon.invalid", false, root, "claude")
	if firing.Status == doctorPass {
		t.Fatalf("configuration is not execution evidence: %+v", firing)
	}
}

func TestDoctorProbeCancelsInjectedWork(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer cancel()
	observed := false
	row := runDoctorProbe(ctx, func(ctx context.Context) doctorCheck {
		<-ctx.Done()
		observed = true
		return doctorCheck{Name: "slow", Status: doctorFail, Reason: "not established", probeErr: ctx.Err()}
	})
	if !observed || row.Status != doctorTimeout {
		t.Fatalf("cancelled work = %+v, observed %v", row, observed)
	}
	pass, _, fail, skipped := tallyDoctorChecks([]doctorCheck{row, {Status: doctorUnknown}})
	if pass != 0 || fail != 0 || skipped != 2 {
		t.Fatalf("uncertain tally = %d/%d/%d", pass, fail, skipped)
	}
}

func TestDoctorWrapperUncertainAndNegativeControls(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     doctorStatus
	}{
		{"oversize", strings.Repeat("#", doctorWrapperBytes) + "\nfathom prompt-hook", doctorUnknown},
		{"missing", "", doctorUnknown},
		{"comment", "# fathom prompt-hook\necho fathom prompt-hook\n", doctorFail},
		{"unrelated-task", "task unrelated\n", doctorFail},
		{"dynamic", "eval \"$HOOK\"\n", doctorUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := doctorHookFixture(t, "./wrapper.sh")
			if tc.body != "" {
				doctorWriteFixture(t, root, "wrapper.sh", tc.body)
			}
			doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  unrelated:\n    cmds: ['echo done']\n  hooks:prompt:\n    cmds: ['fathom prompt-hook']\n")
			got := checkHooksInstalled(root, "claude")
			if got.Status != tc.status {
				t.Fatalf("%s = %+v", tc.name, got)
			}
		})
	}
}

func TestDoctorRecognizesProjectMCPWrapper(t *testing.T) {
	root := doctorHookFixture(t, "fathom prompt-hook")
	doctorWriteFixture(t, root, ".mcp.json", `{"mcpServers":{"fathom":{"command":"./scripts/mcp.sh"}}}`)
	doctorWriteFixture(t, root, "scripts/mcp.sh", "#!/bin/sh\nexec fathom serve-mcp\n")
	got := checkMCPRegistration(root, "claude", "http://daemon.invalid")
	if got.Status != doctorPass || !strings.Contains(got.Reason, "configured (indirect)") {
		t.Fatalf("MCP wrapper = %+v", got)
	}
}

func TestDoctorDeadlineReachesProfileAndGitProbes(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer cancel()
	_, err := buildLLMClientForProfileContext(ctx, config.Profile{Provider: config.ProviderCodexCLI, Bin: filepath.Join(t.TempDir(), "must-not-execute")})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("profile construction lost deadline: %v", err)
	}
	root := t.TempDir()
	gitRepo(t, root)
	_, err = doctorGitHead(root, ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("git lost deadline: %v", err)
	}
}

func TestDoctorHTTPProbeCancellationReachesServer(t *testing.T) {
	started, stopped := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(stopped)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		resp, err := doctorHTTPClient(ctx).Get(server.URL)
		if resp != nil {
			resp.Body.Close()
		}
		result <- err
	}()
	<-started
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("request did not cancel: %v", err)
	}
	<-stopped
}

func TestDoctorExpiredDeadlineStopsFileWalks(t *testing.T) {
	root := t.TempDir()
	doctorWriteFixture(t, root, "code/main.go", "package main\n")
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer cancel()
	visited, err := walkForUserHiddenFiles(root, nil, filter.New([]string{"**"}), ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("walk lost cancellation error: %v", err)
	}
	if len(visited) != 0 {
		t.Fatalf("walk continued after deadline: %v", visited)
	}
	got := doctorCommandEvidence(ctx, root, "./wrapper.sh", "prompt-hook")
	if got.found || !got.unknown {
		t.Fatalf("expired wrapper read = %+v", got)
	}
}
