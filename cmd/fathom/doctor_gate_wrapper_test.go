package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestDoctorComposedPushGate(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       doctorStatus
	}{
		{"both", "task validate\nfathom verify --strict --since origin/main\n", doctorPass},
		{"missing-verify", "task validate\n", doctorFail},
		{"missing-validate", "fathom verify --strict\n", doctorFail},
		{"non-strict", "task validate\nfathom verify\n", doctorFail},
		{"neither", "echo gate\n", doctorFail},
		{"comment-only", "# task validate\n# fathom verify --strict\n", doctorFail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			doctorWriteFixture(t, root, "lefthook.yml", "pre-push:\n  commands:\n    gate:\n      run: sh scripts/push.sh\n")
			doctorWriteFixture(t, root, "scripts/push.sh", "#!/bin/sh\nexec ./scripts/gate.sh\n")
			doctorWriteFixture(t, root, "scripts/gate.sh", tc.body)
			got := checkLefthookGates(root)
			if got.Status != tc.want {
				t.Fatalf("got %+v, want %s", got, tc.want)
			}
			if tc.want == doctorPass && !strings.Contains(got.Reason, "scripts/gate.sh") {
				t.Fatalf("proof file absent: %+v", got)
			}
		})
	}
}

func TestDoctorNativePushGateWrapper(t *testing.T) {
	root := t.TempDir()
	doctorWriteFixture(t, root, ".git/hooks/pre-push", "#!/bin/sh\nexec ./scripts/gate.sh\n")
	doctorWriteFixture(t, root, "scripts/gate.sh", "task validate\nfathom verify --strict\n")
	got := checkLefthookGates(root)
	if got.Status != doctorPass || !strings.Contains(got.Reason, "scripts/gate.sh") {
		t.Fatalf("native wrapper = %+v", got)
	}
}

// O27: the pre-push hook git runs decides; an uninstalled lefthook.yml does not.
func doctorNativeOverLefthookFixture(t *testing.T, hook string) string {
	root := t.TempDir()
	if b, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, b)
	}
	doctorWriteFixture(t, root, "lefthook.yml", "pre-push:\n  commands:\n    validate:\n      run: task validate\n")
	doctorWriteFixture(t, root, ".git/hooks/pre-push", hook)
	return root
}

const doctorNativeTaskHook = "#!/bin/sh\nset -eu\nproject_root=$(git rev-parse --show-toplevel)\ncd \"$project_root\"\nexport PATH=\"$project_root/.tools/bin:$PATH\"\n\"$project_root/.tools/bin/task\" validate </dev/null\ngit lfs pre-push \"$@\"\n"

func TestDoctorNativeHookOverUninstalledLefthook(t *testing.T) {
	t.Run("python verify is unknown", func(t *testing.T) {
		root := doctorNativeOverLefthookFixture(t, doctorNativeTaskHook)
		doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds:\n      - task fathom:audit\n      - task fathom:verify\n  fathom:audit:\n    cmds: [python3 Tools/audit.py]\n  fathom:verify:\n    cmds: [python3 Tools/project.py fathom-verify]\n")
		doctorWriteFixture(t, root, "Tools/audit.py", "print('audit')\n")
		doctorWriteFixture(t, root, "Tools/project.py", "argv = ['fathom', 'verify', '--strict']\n")
		got := checkLefthookGates(root)
		if got.Status != doctorUnknown || !strings.Contains(got.Reason, "Tools/project.py") || !strings.Contains(got.Reason, "lefthook.yml is not installed") || !strings.Contains(got.Reason, ".git/hooks/pre-push") {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("shell verify passes", func(t *testing.T) {
		root := doctorNativeOverLefthookFixture(t, doctorNativeTaskHook)
		doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds:\n      - sh scripts/verify.sh\n")
		doctorWriteFixture(t, root, "scripts/verify.sh", "#!/bin/sh\nfathom verify --strict\n")
		got := checkLefthookGates(root)
		if got.Status != doctorPass || !strings.Contains(got.Reason, "scripts/verify.sh") || !strings.Contains(got.Reason, "lefthook.yml is not installed") {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("task validate without verify fails", func(t *testing.T) {
		root := doctorNativeOverLefthookFixture(t, doctorNativeTaskHook)
		doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds:\n      - echo ok\n")
		if got := checkLefthookGates(root); got.Status != doctorFail || !strings.Contains(got.Reason, "lefthook.yml is not installed") {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("lefthook dispatcher keeps lefthook.yml", func(t *testing.T) {
		root := doctorNativeOverLefthookFixture(t, "#!/bin/sh\ncall_lefthook()\n{\n  lefthook \"$@\"\n}\ncall_lefthook run \"pre-push\" \"$@\"\n")
		got := checkLefthookGates(root)
		if got.Status != doctorFail || !strings.Contains(got.Reason, "lefthook.yml") || strings.Contains(got.Reason, "not installed") {
			t.Fatalf("got %+v", got)
		}
	})
}
