package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type hostFixture struct {
	root              *os.Root
	dir, binary, host string
	policy            hostPolicy
}

func newHostFixture(t *testing.T, host string) hostFixture {
	t.Helper()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	binary := filepath.Join(t.TempDir(), "kit-new ' fixture")
	if err := os.WriteFile(binary, []byte("offline fake executable\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return hostFixture{root: root, dir: dir, binary: binary, host: host, policy: hostPolicy{Version: 1, DenyTools: []string{"forbidden*"}, DenyPaths: []string{"private"}, Required: []string{"tool"}, Instructions: "Respect project instructions."}}
}

func (f hostFixture) write(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := f.root.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := f.root.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
func (f hostFixture) install(t *testing.T) hostPlan {
	t.Helper()
	p, err := planHostInstall(f.root, f.dir, f.host, f.binary, f.policy)
	if err != nil {
		t.Fatal(err)
	}
	if err = applyHostPlan(f.root, p); err != nil {
		t.Fatal(err)
	}
	return p
}
func (f hostFixture) guard(t *testing.T, event hostHookInput) (int, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := hostGuard(f.root, f.dir, f.host, hostHash(hostJSON(f.policy)), bytes.NewReader(hostJSON(event)), &out, &errOut)
	if code != 0 && (!strings.Contains(out.String(), `"permissionDecision":"deny"`) || strings.Contains(out.String(), `"continue"`)) {
		t.Fatalf("unsafe deny %s", out.String())
	}
	return code, out.String()
}
func fakeHostDeps(host string) hostDeps {
	return hostDeps{now: func() time.Time { return time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC) }, command: func(_ context.Context, _, _ string, args ...string) (string, error) {
		if args[0] == "--version" {
			if host == "codex" {
				return "codex-cli 0.154.0\n", nil
			}
			return "2.1.270 (Claude Code)\n", nil
		}
		if args[0] == "features" {
			return "hooks stable true\n", nil
		}
		return "Usage: mocked host\n", nil
	}}
}
func control(receipt hostReceipt, name string) hostControl {
	for _, c := range receipt.Controls {
		if c.Name == name {
			return c
		}
	}
	return hostControl{}
}

func TestHostInstallRoundTrip(t *testing.T) {
	for _, host := range []string{"claude", "codex"} {
		t.Run(host, func(t *testing.T) {
			f := newHostFixture(t, host)
			config, instructions, policy, ownership := hostPaths(host)
			foreign := []byte(`{"permissions":{"allow":["Read"]},"hooks":{"PreToolUse":[{"matcher":"foreign","hooks":[{"type":"command","command":"foreign"}]}]}}`)
			f.write(t, config, foreign)
			f.write(t, instructions, []byte("User-owned instructions\n"))
			if err := f.root.Chmod(config, 0640); err != nil {
				t.Fatal(err)
			}
			plan, err := planHostInstall(f.root, f.dir, host, f.binary, f.policy)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Changes) != 4 || plan.Changes[0].Path != ownership || plan.Changes[3].Path != config || len(plan.Digest) != 64 {
				t.Fatalf("unexpected plan %+v", plan)
			}
			if _, exists, _ := hostRead(f.root, policy); exists {
				t.Fatal("plan wrote policy")
			}
			if err := applyHostPlan(f.root, plan); err != nil {
				t.Fatal(err)
			}
			if stat, err := f.root.Stat(config); err != nil || stat.Mode().Perm() != 0640 {
				t.Fatalf("permissions changed %v %v", stat, err)
			}
			own, _, err := validateInstalledHost(f.root, f.dir, host)
			if err != nil {
				t.Fatal(err)
			}
			for event, raw := range own.Entries {
				var entry struct {
					Hooks []struct {
						Command string `json:"command"`
						Timeout int    `json:"timeout"`
					} `json:"hooks"`
				}
				if json.Unmarshal(raw, &entry) != nil || len(entry.Hooks) != 1 || entry.Hooks[0].Timeout != 5 || !strings.Contains(entry.Hooks[0].Command, hostQuote(f.binary)) {
					t.Fatalf("invalid %s entry %s", event, raw)
				}
			}
			second, err := planHostInstall(f.root, f.dir, host, f.binary, f.policy)
			if err != nil || len(second.Changes) != 0 {
				t.Fatalf("not idempotent: %v %+v", err, second)
			}
			// Foreign changes after install survive uninstall.
			raw, _, _ := hostRead(f.root, config)
			doc, hooks, _ := hostConfig(raw)
			doc["foreign_new"] = json.RawMessage(`true`)
			doc["hooks"] = hostJSON(hooks)
			f.write(t, config, hostJSON(doc))
			text, _, _ := hostRead(f.root, instructions)
			f.write(t, instructions, append(text, []byte("Later user text\n")...))
			remove, err := planHostUninstall(f.root, f.dir, host)
			if err != nil {
				t.Fatal(err)
			}
			if err := applyHostPlan(f.root, remove); err != nil {
				t.Fatal(err)
			}
			raw, _, _ = hostRead(f.root, config)
			doc, hooks, err = hostConfig(raw)
			if err != nil || string(doc["foreign_new"]) != "true" || len(hooks["PreToolUse"]) != 1 || len(hooks) != 1 {
				t.Fatalf("foreign config lost %s", raw)
			}
			text, _, _ = hostRead(f.root, instructions)
			if string(text) != "User-owned instructions\nLater user text\n" {
				t.Fatalf("foreign instructions lost %q", text)
			}
			for _, path := range []string{policy, ownership} {
				if _, exists, _ := hostRead(f.root, path); exists {
					t.Fatalf("owned %s remains", path)
				}
			}
			if _, err := os.Stat(f.binary); err != nil {
				t.Fatal("uninstall removed executable")
			}
		})
	}
}

func TestHostInstallConflictsAndRecovery(t *testing.T) {
	t.Run("stale plan", func(t *testing.T) {
		f := newHostFixture(t, "codex")
		p, err := planHostInstall(f.root, f.dir, f.host, f.binary, f.policy)
		if err != nil {
			t.Fatal(err)
		}
		f.write(t, "AGENTS.md", []byte("concurrent"))
		if applyHostPlan(f.root, p) == nil {
			t.Fatal("stale accepted")
		}
		_, _, _, own := hostPaths(f.host)
		if _, exists, _ := hostRead(f.root, own); exists {
			t.Fatal("partial install")
		}
	})
	t.Run("rollback", func(t *testing.T) {
		f := newHostFixture(t, "codex")
		p, err := planHostInstall(f.root, f.dir, f.host, f.binary, f.policy)
		if err != nil {
			t.Fatal(err)
		}
		f.write(t, "AGENTS.md.agentic-kit-stage", []byte("foreign stage"))
		if applyHostPlan(f.root, p) == nil {
			t.Fatal("stage overwritten")
		}
		for _, c := range p.Changes {
			if _, exists, _ := hostRead(f.root, c.Path); exists {
				t.Fatalf("not rolled back %s", c.Path)
			}
		}
		raw, _, _ := hostRead(f.root, "AGENTS.md.agentic-kit-stage")
		if string(raw) != "foreign stage" {
			t.Fatal("foreign stage changed")
		}
	})
	t.Run("interrupted installation", func(t *testing.T) {
		f := newHostFixture(t, "claude")
		p, err := planHostInstall(f.root, f.dir, f.host, f.binary, f.policy)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range p.Changes[:2] {
			if err := writeHostChange(f.root, c); err != nil {
				t.Fatal(err)
			}
		}
		if _, _, err := validateInstalledHost(f.root, f.dir, f.host); err == nil {
			t.Fatal("partial install valid")
		}
		if _, err := planHostInstall(f.root, f.dir, f.host, f.binary, f.policy); err == nil {
			t.Fatal("partial treated as installed")
		}
		p, err = planHostUninstall(f.root, f.dir, f.host)
		if err != nil {
			t.Fatal(err)
		}
		if err = applyHostPlan(f.root, p); err != nil {
			t.Fatal(err)
		}
	})
	for _, pathKind := range []string{"policy", "instruction", "hook"} {
		t.Run("uninstall drift "+pathKind, func(t *testing.T) {
			f := newHostFixture(t, "claude")
			f.install(t)
			config, instruction, policy, _ := hostPaths(f.host)
			path := map[string]string{"policy": policy, "instruction": instruction, "hook": config}[pathKind]
			raw, _, _ := hostRead(f.root, path)
			if pathKind == "policy" {
				raw = append(raw, ' ')
			} else {
				raw = bytes.ReplaceAll(raw, []byte("agentic-kit"), []byte("agentic-kit-modified"))
				if pathKind == "instruction" {
					raw = bytes.ReplaceAll(raw, []byte("Respect project"), []byte("Changed project"))
					raw = bytes.ReplaceAll(raw, []byte("agentic-kit-modified:host:"), []byte("agentic-kit:host:"))
				}
			}
			f.write(t, path, raw)
			if _, err := planHostUninstall(f.root, f.dir, f.host); err == nil {
				t.Fatal("drift removal accepted")
			}
		})
	}
	t.Run("symlink", func(t *testing.T) {
		f := newHostFixture(t, "claude")
		if err := os.Symlink(t.TempDir(), filepath.Join(f.dir, ".claude")); err != nil {
			t.Fatal(err)
		}
		if _, err := planHostInstall(f.root, f.dir, f.host, f.binary, f.policy); err == nil {
			t.Fatal("symlink accepted")
		}
	})
	t.Run("unowned policy", func(t *testing.T) {
		f := newHostFixture(t, "codex")
		_, _, path, _ := hostPaths(f.host)
		f.write(t, path, []byte("foreign"))
		if _, err := planHostInstall(f.root, f.dir, f.host, f.binary, f.policy); err == nil {
			t.Fatal("foreign policy overwritten")
		}
	})
}

func TestHostGuardContract(t *testing.T) {
	for _, host := range []string{"claude", "codex"} {
		t.Run(host, func(t *testing.T) {
			f := newHostFixture(t, host)
			f.install(t)
			for _, tc := range []struct {
				name, tool, input string
				deny              bool
			}{
				{"allow edit", "Edit", `{"file_path":"public.txt"}`, false},
				{"deny tool", "forbidden_tool", `{}`, true},
				{"deny path", "Write", `{"file_path":"private/key"}`, true},
				{"deny traversal", "Read", `{"file_path":"../outside"}`, true},
				{"shell conservative", "Bash", `{"command":"echo hi"}`, true},
				{"stdin conservative", "write_stdin", `{"session_id":1}`, true},
				{"unknown tool", "mcp__unknown", `{}`, true},
				{"missing path", "Edit", `{}`, true},
				{"patch public", "apply_patch", `{"command":"*** Begin Patch\n*** Add File: public.txt\n+hello\n*** End Patch"}`, false},
				{"patch move denied", "apply_patch", `{"command":"*** Begin Patch\n*** Update File: public.txt\n*** Move to: private/key\n@@\n-hi\n+bye\n*** End Patch"}`, true},
				{"malformed patch", "apply_patch", `{"command":"not a patch"}`, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					code, _ := f.guard(t, hostHookInput{Event: "PreToolUse", CWD: f.dir, Tool: tc.tool, Input: json.RawMessage(tc.input)})
					if (code != 0) != tc.deny {
						t.Fatalf("code %d", code)
					}
				})
			}
			for _, source := range []string{"startup", "resume", "compact"} {
				code, out := f.guard(t, hostHookInput{Event: "SessionStart", Source: source, CWD: f.dir})
				if code != 0 || !strings.Contains(out, "additionalContext") {
					t.Fatalf("%s: %d %s", source, code, out)
				}
			}
			for _, event := range []string{"PreCompact", "Stop"} {
				if code, out := f.guard(t, hostHookInput{Event: event, CWD: f.dir}); code != 0 || strings.TrimSpace(out) != "{}" {
					t.Fatalf("%s: %d %s", event, code, out)
				}
			}
			if code, _ := f.guard(t, hostHookInput{Event: "SessionStart", Source: "unknown", CWD: f.dir}); code != 2 {
				t.Fatal("unknown session source passed")
			}
			if code, _ := f.guard(t, hostHookInput{Event: "PreToolUse", CWD: filepath.Dir(f.dir), Tool: "Edit"}); code != 2 {
				t.Fatal("outside cwd passed")
			}
			if err := os.Symlink(filepath.Join(f.dir, "private"), filepath.Join(f.dir, "alias")); err != nil {
				t.Fatal(err)
			}
			if code, _ := f.guard(t, hostHookInput{Event: "PreToolUse", CWD: f.dir, Tool: "Edit", Input: json.RawMessage(`{"file_path":"alias/key"}`)}); code != 2 {
				t.Fatal("symlink path passed")
			}
			f.write(t, "nested/.keep", []byte{})
			if code, _ := f.guard(t, hostHookInput{Event: "PreToolUse", CWD: filepath.Join(f.dir, "nested"), Tool: "Edit", Input: json.RawMessage(`{"file_path":"../private/key"}`)}); code != 2 {
				t.Fatal("nested protected path passed")
			}
		})
	}
}

func TestHostJSONAndGuardFailures(t *testing.T) {
	f := newHostFixture(t, "codex")
	f.install(t)
	for _, input := range []string{`{`, `null`, `{} {}`, `{"hook_event_name":"Stop","hook_event_name":"PreToolUse"}`, strings.Repeat("[", 70) + "0" + strings.Repeat("]", 70), strings.Repeat(" ", hostMaxBytes+1)} {
		var out, errOut bytes.Buffer
		if hostGuard(f.root, f.dir, f.host, hostHash(hostJSON(f.policy)), strings.NewReader(input), &out, &errOut) != 2 {
			t.Fatalf("invalid input passed %.80s", input)
		}
	}
	for _, raw := range []string{`{"version":1,"unknown":true}`, `{"version":1,"version":1}`, `{"version":2}`, `{"version":1,"deny_paths":["../escape"]}`, `{"version":1,"required_controls":["invented"]}`, `{"version":1,"deny_tools":["["]}`} {
		var p hostPolicy
		err := hostDecode([]byte(raw), &p)
		if err == nil {
			err = validateHostPolicy(p)
		}
		if err == nil {
			t.Fatalf("invalid policy passed %s", raw)
		}
	}
	_, _, path, _ := hostPaths(f.host)
	f.write(t, path, []byte("{}"))
	if code, _ := f.guard(t, hostHookInput{Event: "Stop", CWD: f.dir}); code != 2 {
		t.Fatal("policy drift passed")
	}
}

func TestHostReceiptEvidenceAndFailures(t *testing.T) {
	for _, host := range []string{"claude", "codex"} {
		t.Run(host, func(t *testing.T) {
			f := newHostFixture(t, host)
			f.install(t)
			deps := fakeHostDeps(host)
			r := inspectHost(f.root, f.dir, host, host, "", deps)
			if r.Ready || control(r, "tool").Status != "unknown" || control(r, "runtime").Status != "supported" || r.GeneratedAt != "2026-09-13T00:00:00Z" {
				t.Fatalf("false proof %+v", r)
			}
			artifact := []byte("synthetic handler evidence; this TEST does not certify a real host\n")
			f.write(t, "evidence/transcript.txt", artifact)
			e := hostEvidence{Version: 1, Host: host, HostVersion: r.HostVersion, ConfigDigest: r.ConfigDigest, Observations: []hostObservation{{Control: "tool", Status: "supported", Method: "live-host", Path: "evidence/transcript.txt", Digest: hostHash(artifact), Detail: "Fixture tests evidence transport only."}}}
			f.write(t, "evidence/observations.json", hostJSON(e))
			r = inspectHost(f.root, f.dir, host, host, "evidence/observations.json", deps)
			if !r.Ready || control(r, "tool").Basis != "supplied-live-observation" {
				t.Fatalf("evidence transport failed %+v", r)
			}
			for _, name := range []string{"startup", "resume", "nested-instructions", "edit", "shell", "compaction", "stop"} {
				if control(r, name).Status != "unknown" {
					t.Fatalf("invented %s coverage", name)
				}
			}
			if host == "codex" && (control(r, "stdin").Status != "unsupported" || control(r, "hosted-tools").Status != "unsupported") {
				t.Fatal("Codex gap hidden")
			}
			if control(r, "hook-failure-closed").Status != "unsupported" {
				t.Fatal("failure closure claimed")
			}
			f.write(t, "evidence/transcript.txt", []byte("changed"))
			if inspectHost(f.root, f.dir, host, host, "evidence/observations.json", deps).Ready {
				t.Fatal("evidence drift passed")
			}
			f.write(t, "evidence/transcript.txt", artifact)
			config, _, _, _ := hostPaths(host)
			raw, _, _ := hostRead(f.root, config)
			f.write(t, config, append(raw, ' '))
			if inspectHost(f.root, f.dir, host, host, "evidence/observations.json", deps).Ready {
				t.Fatal("config digest drift passed")
			}
		})
	}
	for _, failure := range []string{"missing", "disabled", "malformed", "policy-drift", "binary-drift", "binary-permission", "version", "timeout", "help-parse", "features-disabled"} {
		t.Run(failure, func(t *testing.T) {
			f := newHostFixture(t, "codex")
			f.policy.Required = nil
			f.install(t)
			deps := fakeHostDeps(f.host)
			config, _, policy, _ := hostPaths(f.host)
			switch failure {
			case "missing":
				if err := f.root.Remove(config); err != nil {
					t.Fatal(err)
				}
			case "disabled":
				f.write(t, ".codex/config.toml", []byte("[features]\nhooks = false\n"))
			case "malformed":
				f.write(t, config, []byte("{"))
			case "policy-drift":
				f.write(t, policy, []byte("{}"))
			case "binary-drift":
				if err := os.WriteFile(f.binary, []byte("changed"), 0700); err != nil {
					t.Fatal(err)
				}
			case "binary-permission":
				if err := os.Chmod(f.binary, 0600); err != nil {
					t.Fatal(err)
				}
			default:
				deps.command = func(_ context.Context, _, _ string, args ...string) (string, error) {
					if failure == "timeout" {
						return "", context.DeadlineExceeded
					}
					if args[0] == "--version" {
						if failure == "version" {
							return "codex-cli future", nil
						}
						return "codex-cli 0.154.0", nil
					}
					if failure == "help-parse" {
						return "invalid", nil
					}
					if args[0] == "features" {
						return "hooks stable false", nil
					}
					return "Usage: fixture", nil
				}
			}
			if inspectHost(f.root, f.dir, f.host, f.host, "", deps).Ready {
				t.Fatal("failed control passed")
			}
		})
	}
}

func TestHostCLIPlanExpectedDigest(t *testing.T) {
	f := newHostFixture(t, "claude")
	f.write(t, "input.json", hostJSON(f.policy))
	base := []string{"-host", f.host, "-dir", f.dir, "-binary", f.binary, "-policy", filepath.Join(f.dir, "input.json")}
	call := func(op string, extra ...string) (int, string) {
		var out, errOut bytes.Buffer
		args := append([]string{op}, base...)
		args = append(args, extra...)
		code := runHost(args, strings.NewReader(""), &out, &errOut, fakeHostDeps(f.host))
		return code, out.String()
	}
	code, raw := call("plan")
	if code != 0 {
		t.Fatalf("plan %d", code)
	}
	var p hostPlan
	if json.Unmarshal([]byte(raw), &p) != nil {
		t.Fatal(raw)
	}
	if code, _ := call("install", "-expect", "stale"); code == 0 {
		t.Fatal("stale digest accepted")
	}
	if code, _ := call("install", "-expect", p.Digest); code != 0 {
		t.Fatal("reviewed install rejected")
	}
	var out, errOut bytes.Buffer
	if code := runHost([]string{"doctor", "-host", f.host, "-dir", f.dir}, strings.NewReader(""), &out, &errOut, fakeHostDeps(f.host)); code != 1 {
		t.Fatal("unknown required control passed CLI")
	}
	if code, _, _ := runNew(t, "host"); code != 2 {
		t.Fatal("host dispatch missing")
	}
}

func TestHostCommandBounds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := hostCommand(ctx, t.TempDir(), "/bin/sh", "-c", "exit 0"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if _, err := hostCommand(context.Background(), t.TempDir(), "/bin/sh", "-c", "yes x | head -c 70000"); err == nil {
		t.Fatal("oversized host output accepted")
	}
}
