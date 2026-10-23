package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
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

func TestDoctorNativeHookFixRound(t *testing.T) {
	t.Run("lint-only python script fails", func(t *testing.T) {
		root := doctorNativeOverLefthookFixture(t, doctorNativeTaskHook)
		doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds:\n      - python3 scripts/lint.py\n")
		doctorWriteFixture(t, root, "scripts/lint.py", "print('lint')\n")
		if got := checkLefthookGates(root); got.Status != doctorFail {
			t.Fatalf("got %+v", got)
		}
	})
	// A large Taskfile read once per `task X` must be charged once.
	bigTaskfile := func(last string) string {
		var b strings.Builder
		b.WriteString("version: '3'\ntasks:\n  validate:\n    cmds:\n")
		for i := 1; i <= 20; i++ {
			fmt.Fprintf(&b, "      - task t%02d\n", i)
		}
		b.WriteString("      - task last\n")
		for i := 1; i <= 20; i++ {
			fmt.Fprintf(&b, "  t%02d:\n    desc: %s\n    cmds: [echo step]\n", i, strings.Repeat("x", 900))
		}
		b.WriteString("  last:\n    cmds: [" + last + "]\n")
		return b.String()
	}
	t.Run("big Taskfile literal verify last passes", func(t *testing.T) {
		root := doctorNativeOverLefthookFixture(t, doctorNativeTaskHook)
		doctorWriteFixture(t, root, "Taskfile.yml", bigTaskfile("fathom verify --strict"))
		if got := checkLefthookGates(root); got.Status != doctorPass {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("big Taskfile python verify last is unknown", func(t *testing.T) {
		root := doctorNativeOverLefthookFixture(t, doctorNativeTaskHook)
		doctorWriteFixture(t, root, "Taskfile.yml", bigTaskfile("python3 scripts/project.py fathom-verify"))
		doctorWriteFixture(t, root, "scripts/project.py", "print('v')\n")
		if got := checkLefthookGates(root); got.Status != doctorUnknown || !strings.Contains(got.Reason, "scripts/project.py") {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("budget exhaustion is named", func(t *testing.T) {
		root := doctorNativeOverLefthookFixture(t, doctorNativeTaskHook)
		doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\n# "+strings.Repeat("x", doctorWrapperBytes)+"\ntasks:\n  validate:\n    cmds: [echo]\n")
		got := checkLefthookGates(root)
		if got.Status != doctorUnknown || !strings.Contains(got.Reason, "read budget") {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("linked worktree", func(t *testing.T) {
		main := doctorNativeOverLefthookFixture(t, doctorNativeTaskHook)
		for _, args := range [][]string{
			{"-c", "user.email=a@b.c", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x"},
			{"worktree", "add", "-q", filepath.Join(main, "wt"), "-b", "wt"},
		} {
			if b, err := exec.Command("git", append([]string{"-C", main}, args...)...).CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v %s", args, err, b)
			}
		}
		wt := filepath.Join(main, "wt")
		doctorWriteFixture(t, wt, "lefthook.yml", "pre-push:\n  commands:\n    validate:\n      run: task validate\n")
		doctorWriteFixture(t, wt, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds:\n      - fathom verify --strict\n")
		got := checkLefthookGates(wt)
		if got.Status != doctorPass || !strings.Contains(got.Reason, "lefthook.yml is not installed") {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("lfs-only hook fails despite lefthook.yml verify", func(t *testing.T) {
		root := doctorNativeOverLefthookFixture(t, "#!/bin/sh\ngit lfs pre-push \"$@\"\n")
		doctorWriteFixture(t, root, "lefthook.yml", "pre-push:\n  commands:\n    v:\n      run: fathom verify --strict\n")
		got := checkLefthookGates(root)
		if got.Status != doctorFail || !strings.Contains(got.Remedy, "lefthook install") {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("lefthook run hand-off keeps lefthook.yml", func(t *testing.T) {
		root := doctorNativeOverLefthookFixture(t, "#!/bin/sh\nexec lefthook run pre-push \"$@\"\n")
		got := checkLefthookGates(root)
		if strings.Contains(got.Reason, "not installed") {
			t.Fatalf("got %+v", got)
		}
	})
}
