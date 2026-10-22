package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorForeignHookCommandsAreDefinite(t *testing.T) {
	for _, mode := range []string{"absolute", "home", "absolute-task", "home-fathom"} {
		t.Run(mode, func(t *testing.T) {
			outside := t.TempDir()
			doctorWriteFixture(t, outside, "other.sh", "exec fathom prompt-hook\n")
			command := filepath.Join(outside, "other.sh")
			if mode == "home" {
				command = "~/.claude/hooks/other.sh"
			} else if mode == "absolute-task" {
				command = filepath.Join(outside, "task") + " prompt"
			} else if mode == "home-fathom" {
				command = "~/bin/fathom prompt-hook"
			}
			root := doctorHookFixture(t, command)
			doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  prompt:\n    cmds: ['fathom prompt-hook']\n")
			got := checkHooksInstalled(root, "claude")
			if mode == "home-fathom" {
				if got.Status != doctorPass || doctorExitCode([]doctorCheck{got}) != 0 {
					t.Fatalf("fathom binary = %+v", got)
				}
				return
			}
			if got.Status != doctorFail || doctorExitCode([]doctorCheck{got}) != 1 || !strings.Contains(got.Reason, "prompt-hook") || !strings.Contains(got.Remedy, "fathom init") {
				t.Fatalf("foreign hook = %+v", got)
			}
		})
	}
}

func TestDoctorMissingHookScriptIsDefinite(t *testing.T) {
	for _, command := range []string{"./scripts/absent.sh", "sh scripts/absent.sh", `"$CLAUDE_PROJECT_DIR"/scripts/absent.sh`, "./wrapper.sh"} {
		t.Run(command, func(t *testing.T) {
			root := doctorHookFixture(t, command)
			doctorWriteFixture(t, root, "wrapper.sh", "exec ./scripts/absent.sh\n")
			got := checkHooksInstalled(root, "claude")
			if got.Status != doctorFail || doctorExitCode([]doctorCheck{got}) != 1 || !strings.Contains(got.Reason, "hook script not found: "+filepath.Join(root, "scripts/absent.sh")) || !strings.Contains(got.Remedy, "fathom init") {
				t.Fatalf("missing script = %+v", got)
			}
		})
	}
}

func TestDoctorReadableDynamicHookRemainsUnknown(t *testing.T) {
	root := doctorHookFixture(t, "./wrapper.sh")
	doctorWriteFixture(t, root, "wrapper.sh", "eval \"$HOOK\"\n")
	got := checkHooksInstalled(root, "claude")
	if got.Status != doctorUnknown || doctorExitCode([]doctorCheck{got}) != 0 {
		t.Fatalf("dynamic wrapper = %+v", got)
	}
}
