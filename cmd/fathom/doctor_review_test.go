package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

func TestDoctorBriefNamesUnfinishedProbes(t *testing.T) {
	checks := []doctorCheck{{Name: "rpc", Status: doctorTimeout, Reason: "deadline expired"}, {Name: "hooks", Status: doctorUnknown, Reason: "unreadable wrapper"}}
	got := doctorBrief(checks)
	for _, want := range []string{"1 check(s) TIMEOUT", "1 check(s) UNKNOWN", "rpc:", "hooks:"} {
		if !strings.Contains(got, want) {
			t.Errorf("brief missing %q: %q", want, got)
		}
	}
}

func TestDoctorProbePreservesDefiniteVerdicts(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer cancel()
	for _, status := range []doctorStatus{doctorPass, doctorWarn, doctorFail} {
		got := runDoctorProbe(ctx, func(context.Context) doctorCheck {
			return doctorCheck{Name: "evidence", Status: status, Reason: "read complete", probeErr: errors.New("unrelated diagnostic error")}
		})
		if got.Status != status {
			t.Errorf("%s changed to %+v", status, got)
		}
	}
}

func TestDoctorStepsDoNotCapLocalWork(t *testing.T) {
	for _, ownBudget := range []bool{false, true} {
		rows := runDoctorSteps([]doctorStep{{ownBudget: ownBudget, probe: func(ctx context.Context) doctorCheck {
			if _, capped := ctx.Deadline(); capped {
				t.Errorf("ownBudget=%v: step cap present=%v", ownBudget, capped)
			}
			return doctorCheck{Name: "local", Status: doctorFail, Reason: "definite evidence"}
		}}})
		if len(rows) != 1 || rows[0].Status != doctorFail {
			t.Fatalf("local verdict = %+v", rows)
		}
	}
}

func TestDoctorCancelledWrapperReadersKeepStepContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, label := range []string{"claude", "codex"} {
		t.Run(label, func(t *testing.T) {
			root := t.TempDir()
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
			doctorWriteFixture(t, root, "wrapper.sh", "fathom serve-mcp\n")
			if label == "claude" {
				doctorWriteFixture(t, home, ".claude.json", `{"mcpServers":{"fathom":{"command":"./wrapper.sh"}}}`)
			} else {
				doctorWriteFixture(t, root, ".codex/config.toml", "[mcp_servers.fathom]\ncommand = './wrapper.sh'\n")
			}
			got := checkMCPRegistration(root, label, "http://daemon.invalid", ctx)
			if got.Status == doctorPass {
				t.Fatalf("reader replaced cancelled step context: %+v", got)
			}
		})
	}
	root := t.TempDir()
	gitRepo(t, root)
	if got := gitToplevelContext(ctx, root); got != "" {
		t.Fatalf("cancelled git lookup ran: %s", got)
	}
}

func TestDoctorDaemonTimeoutIsDefiniteFailure(t *testing.T) {
	_, got := checkDaemonReachable(&http.Client{Transport: doctorSlowTransport{}}, "http://daemon.invalid")
	if got.Status != doctorFail || !strings.Contains(got.Reason, "daemon did not answer within 5 s") || doctorExitCode([]doctorCheck{got}) != 1 {
		t.Fatalf("daemon timeout = %+v", got)
	}
}

func TestDoctorRPCProbeErrorsReachBrief(t *testing.T) {
	root := t.TempDir()
	doctorWriteFixture(t, root, "go.mod", "module fixture\n\ngo 1.26.2\n")
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer cancel()
	hc := &http.Client{Transport: doctorSlowTransport{}}
	for _, probe := range []struct {
		name string
		run  func() doctorCheck
	}{
		{"fresh", func() doctorCheck { return checkDaemonFresh(hc, "http://daemon.invalid", true) }},
		{"writes", func() doctorCheck { return checkDaemonDiskWrites(hc, "http://daemon.invalid", true) }},
		{"admission", func() doctorCheck { return checkAdmissionMode(hc, "http://daemon.invalid", true) }},
		{"compile", func() doctorCheck { return checkCompileLayerEnabled(hc, "http://daemon.invalid", true) }},
		{"typed", func() doctorCheck { return checkTypedEdges(hc, "http://daemon.invalid", root, "fixture", true) }},
		{"languages", func() doctorCheck {
			return checkUnsupportedLanguages(hc, "http://daemon.invalid", root, "fixture", true)
		}},
		{"roots", func() doctorCheck { return checkServiceRoots(hc, "http://daemon.invalid", root, "fixture", true) }},
		{"traffic", func() doctorCheck { return checkSurfaceTraffic(hc, "http://daemon.invalid", true, root) }},
	} {
		t.Run(probe.name, func(t *testing.T) {
			row := runDoctorProbe(ctx, func(context.Context) doctorCheck { return probe.run() })
			if row.Status != doctorTimeout || doctorBrief([]doctorCheck{row}) == "" {
				t.Fatalf("unfinished RPC absent from brief: %+v", row)
			}
		})
	}
}

func TestDoctorWrapperReaderReviewControls(t *testing.T) {
	for _, tc := range []struct {
		name, command, task, lefthook string
		found, unknown                bool
	}{
		{name: "quoted-variable", command: `"$CLAUDE_PROJECT_DIR"/scripts/x.sh`, found: true},
		{name: "braced-variable", command: `"${CLAUDE_PROJECT_DIR}"/scripts/x.sh`, found: true},
		{name: "pipe", command: "echo x | fathom prompt-hook", found: true},
		{name: "or", command: "false || fathom prompt-hook", found: true},
		{name: "background", command: "echo x & fathom prompt-hook", found: true},
		{name: "task-deps", command: "task prompt", task: "version: '3'\ntasks:\n  prompt:\n    deps: [nested]\n    cmds: ['echo done']\n  nested:\n    cmds: ['fathom prompt-hook']\n", unknown: true},
		{name: "lefthook-scripts", command: "lefthook run prompt", lefthook: "prompt:\n  scripts:\n    x.sh: {}\n  commands:\n    other:\n      run: echo done\n", unknown: true},
		{name: "lefthook-jobs", command: "lefthook run prompt", lefthook: "prompt:\n  jobs:\n    - run: fathom prompt-hook\n  commands:\n    other:\n      run: echo done\n", unknown: true},
		{name: "foreign-npx", command: "npx lint-hook"},
		{name: "foreign-node", command: "node tool.js"},
		{name: "foreign-python", command: "python tool.py"},
		{name: "sh-script", command: "sh scripts/x.sh", found: true},
		{name: "escape", command: "../outside.sh", unknown: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			doctorWriteFixture(t, root, "scripts/x.sh", "exec fathom prompt-hook\n")
			if tc.task != "" {
				doctorWriteFixture(t, root, "Taskfile.yml", tc.task)
			}
			if tc.lefthook != "" {
				doctorWriteFixture(t, root, "lefthook.yml", tc.lefthook)
			}
			got := doctorCommandEvidence(context.Background(), root, tc.command, "prompt-hook")
			if got.found != tc.found || got.unknown != tc.unknown {
				t.Fatalf("evidence = %+v", got)
			}
		})
	}
}

func TestDoctorWrapperRefusesExternalFilesAndFIFOs(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	doctorWriteFixture(t, outside, "outside.sh", "fathom prompt-hook\n")
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{filepath.Join(outside, "outside.sh"), "./escape/outside.sh"} {
		got := doctorCommandEvidence(context.Background(), root, command, "prompt-hook")
		if got.found || (got.unknown != strings.HasPrefix(command, "./")) {
			t.Fatalf("external wrapper %q = %+v", command, got)
		}
	}
	if err := syscall.Mkfifo(filepath.Join(root, "fifo.sh"), 0600); err != nil {
		t.Fatal(err)
	}
	got := doctorCommandEvidence(context.Background(), root, "./fifo.sh", "prompt-hook")
	if got.found || !got.unknown {
		t.Fatalf("FIFO = %+v", got)
	}
}

func TestDoctorUncertaintyCannotHideMissingHooks(t *testing.T) {
	for _, mode := range []string{"foreign", "mixed", "parse"} {
		t.Run(mode, func(t *testing.T) {
			root := doctorHookFixture(t, "./missing.sh")
			p := filepath.Join(root, ".claude/settings.json")
			data, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			hooks := doc["hooks"].(map[string]any)
			if mode == "foreign" {
				hooks["UserPromptSubmit"] = []any{map[string]any{"matcher": "", "hooks": []any{map[string]any{"command": "npx other-hook"}}}}
			} else {
				delete(hooks, "Stop")
			}
			data, err = json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, data, 0600); err != nil {
				t.Fatal(err)
			}
			if mode == "parse" {
				doctorWriteFixture(t, root, ".claude/settings.local.json", "{")
			}
			got := checkHooksInstalled(root, "claude")
			if got.Status != doctorFail || !strings.Contains(got.Remedy, "fathom init") {
				t.Fatalf("missing hooks = %+v", got)
			}
			want := "stop-hook"
			if mode == "foreign" {
				want = "prompt-hook"
			}
			if !strings.Contains(got.Reason, want) {
				t.Fatalf("missing hook unnamed: %+v", got)
			}
		})
	}
}

func TestDoctorHarnessUncertaintyIsPerHook(t *testing.T) {
	d, ok := harness.Lookup(harness.GeminiCLI)
	if !ok {
		t.Fatal("descriptor absent")
	}
	for _, mode := range []string{"mixed", "parse", "parse-only", "uncertain-only"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", t.TempDir())
			hooks := map[string]any{}
			specs := hookSpecsFor(d.ID)
			for i, spec := range specs {
				if mode != "uncertain-only" && i == 1 {
					continue
				}
				command := spec.marker
				if i == 0 && mode != "parse-only" {
					command = "./missing.sh"
				}
				groups, _ := hooks[spec.event].([]any)
				hooks[spec.event] = append(groups, map[string]any{"matcher": spec.matcher, "hooks": []any{map[string]any{"command": command}}})
			}
			data, err := json.Marshal(map[string]any{"hooks": hooks})
			if err != nil {
				t.Fatal(err)
			}
			doctorWriteFixture(t, root, ".gemini/settings.json", string(data))
			testDescriptor := d
			if strings.HasPrefix(mode, "parse") {
				// Exercise an addressable unreadable scope alongside a readable one.
				testDescriptor.HookFiles = append(append([]harness.HookFile(nil), d.HookFiles...), harness.HookFile{Path: ".gemini/broken.json", Scope: "project-local", Kind: "json-hook"})
				doctorWriteFixture(t, root, ".gemini/broken.json", "{")
			}
			got := checkHarnessHooksInstalled(root, testDescriptor)
			want := doctorFail
			if strings.HasPrefix(mode, "parse") && !strings.Contains(got.Reason, "did not parse") {
				t.Fatalf("parse note missing: %+v", got)
			}
			if got.Status != want || (mode != "uncertain-only" && !strings.Contains(got.Reason, specLabel(specs[1]))) {
				t.Fatalf("harness wiring = %+v", got)
			}
		})
	}
}

func TestDoctorExpiredWalkReportsTimeout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FATHOM_HOME", filepath.Join(home, ".fathom"))
	writeUserIgnoreTest(t, home, "*.go\n")
	root := t.TempDir()
	doctorWriteFixture(t, root, "code.go", "package fixture\n")
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer cancel()
	got := runDoctorProbe(ctx, func(ctx context.Context) doctorCheck { return checkUserScopeIgnoreContext(ctx, root) })
	if got.Status != doctorTimeout || doctorBrief([]doctorCheck{got}) == "" {
		t.Fatalf("unfinished walk read as complete: %+v", got)
	}
}

func TestDoctorExpiredGuardWalkReportsTimeout(t *testing.T) {
	root := t.TempDir()
	writeGuardCue(t, root, `guards: [{scopes: ["scripts/**"]}]`)
	doctorWriteFixture(t, root, "scripts/run.go", "package fixture\n")
	srv := fakeDoctorDaemon(t, fakeDaemonCfg{sources: []string{"other.go"}})
	defer srv.Close()
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer cancel()
	got := runDoctorProbe(ctx, func(ctx context.Context) doctorCheck {
		return checkGuardDeadScopes(srv.Client(), srv.URL, root, "fixture", true, ctx)
	})
	if got.Status != doctorTimeout || strings.Contains(got.Remedy, "remove the scope") {
		t.Fatalf("unfinished guard walk read as complete: %+v", got)
	}
}
