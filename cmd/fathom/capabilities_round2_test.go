package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
)

func TestCapabilitiesReadoutsOnIsRich(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir())
	path, err := setHookSeam(t.TempDir(), "user", "readouts", true)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(path, "")
	if err != nil || loaded.Config.Readouts.Mode != config.ReadoutsRich {
		t.Fatalf("on must select rich: %+v %v", loaded.Config.Readouts, err)
	}
}

func TestCapabilitiesSetPrintsHookRegistration(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir())
	var out, stderr bytes.Buffer
	if code := cmdCapabilities([]string{"set", "--repo", t.TempDir(), "read_outline_hint", "on"}, &out, &stderr); code != 0 {
		t.Fatalf("code %d: %s", code, &stderr)
	}
	if !strings.Contains(out.String(), "run `fathom init --hooks` to register it") {
		t.Fatalf("registration remedy missing: %s", &out)
	}
}

func TestDoctorDuplicateHooksCompareCommandsAndIgnoreSilent(t *testing.T) {
	for _, tc := range []struct {
		name, event, matcher, user, project string
		want                                doctorStatus
	}{
		{"different", "PostToolUse", "Edit|Write|MultiEdit", "fathom edit-hook", "fathom edit-hook --harness claude-code", doctorPass},
		{"same", "PostToolUse", "Edit|Write|MultiEdit", "fathom edit-hook", "fathom edit-hook", doctorWarn},
		{"silent-compact", "PreCompact", "manual|auto", "fathom pre-compact-hook", "fathom pre-compact-hook", doctorPass},
		{"silent-end", "SessionEnd", "", "fathom session-end-hook", "fathom session-end-hook", doctorPass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := doctorHookFixture(t, "fathom prompt-hook")
			body := func(cmd string) string {
				return `{"hooks":{"` + tc.event + `":[{"matcher":"` + tc.matcher + `","hooks":[{"type":"command","command":"` + cmd + `"}]}]}}`
			}
			doctorWriteFixture(t, root, ".claude/settings.json", body(tc.project))
			doctorWriteFixture(t, os.Getenv("CLAUDE_CONFIG_DIR"), "settings.json", body(tc.user))
			if row := checkDuplicateHookRegistration(root, "claude"); row.Status != tc.want {
				t.Fatalf("duplicate check: %+v", row)
			}
		})
	}
}
