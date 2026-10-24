package main

import (
	"fmt"
	"os"
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

func TestDoctorPushGateDepthLimit(t *testing.T) {
	for _, tc := range []struct {
		name, tasks string
		want        doctorStatus
	}{
		{"verify beyond limit", "  validate:\n    cmds: [task ci]\n  ci:\n    cmds: [task gate]\n  gate:\n    cmds: [fathom verify --strict]\n", doctorUnknown},
		{"verify within limit", "  validate:\n    cmds: [task ci]\n  ci:\n    cmds: [fathom verify --strict]\n", doctorPass},
		{"verify after a cut", "  validate:\n    cmds: [task ci, fathom verify --strict]\n  ci:\n    cmds: [task gate]\n  gate:\n    cmds: [echo done]\n", doctorPass},
		{"no verify and no cut", "  validate:\n    cmds: [task ci]\n  ci:\n    cmds: [echo done]\n", doctorFail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			gitRepo(t, root)
			doctorWriteFixture(t, root, ".git/hooks/pre-push", "#!/bin/sh\ntask validate\n")
			doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n"+tc.tasks)
			got := checkLefthookGates(root)
			if got.Status != tc.want {
				t.Fatalf("got %+v, want %s", got, tc.want)
			}
			if tc.want == doctorUnknown {
				for _, want := range []string{"depth limit", "Taskfile.yml", "task ci"} {
					if !strings.Contains(got.Reason, want) {
						t.Errorf("reason %q does not name %q", got.Reason, want)
					}
				}
				if doctorExitCode([]doctorCheck{got}) != 0 {
					t.Fatal("depth cut must fail open")
				}
			}
			if tc.want == doctorFail && got.Remedy != "wire both commands in the repository-local pre-push wrapper" {
				t.Fatalf("missing-gate remedy changed: %+v", got)
			}
		})
	}
}

func TestDoctorComposedPushGateDepthCutResolution(t *testing.T) {
	for _, taskfile := range []bool{true, false} {
		name, want := "without Taskfile", doctorFail
		if taskfile {
			name, want = "with Taskfile", doctorUnknown
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			doctorWriteFixture(t, root, "lefthook.yml", "pre-push:\n  commands:\n    gate:\n      run: sh scripts/push.sh\n")
			doctorWriteFixture(t, root, "scripts/push.sh", "task validate\n")
			if taskfile {
				doctorWriteFixture(t, root, "Taskfile.yaml", "version: '3'\ntasks:\n  validate:\n    cmds: [task ci]\n  ci:\n    cmds: [task gate]\n  gate:\n    cmds: [fathom verify --strict]\n")
			}
			got := checkLefthookGates(root)
			if got.Status != want {
				t.Fatalf("got %+v, want %s", got, want)
			}
			if taskfile && (!strings.Contains(got.Reason, "Taskfile.yaml (task validate)") || !strings.Contains(got.Reason, "depth limit")) {
				t.Fatalf("deepest followed task absent: %+v", got)
			}
		})
	}
}

func TestDoctorPushGateDepthCutUnresolvedTargets(t *testing.T) {
	for _, command := range []string{"task absent", "sh scripts/absent.sh", "external-tool"} {
		t.Run(command, func(t *testing.T) {
			root := t.TempDir()
			gitRepo(t, root)
			doctorWriteFixture(t, root, ".git/hooks/pre-push", "task validate\n")
			doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds: [task ci]\n  ci:\n    cmds: ["+command+"]\n")
			if got := checkLefthookGates(root); got.Status != doctorFail {
				t.Fatalf("unresolved cut target hides nothing: %+v", got)
			}
		})
	}
}

func TestDoctorLefthookPushGateDepthLimit(t *testing.T) {
	root := t.TempDir()
	gitRepo(t, root)
	doctorWriteFixture(t, root, ".git/hooks/pre-push", "#!/bin/sh\nlefthook run pre-push\n")
	doctorWriteFixture(t, root, "lefthook.yml", "pre-push:\n  commands:\n    gate:\n      run: task validate\n")
	doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds: [task ci]\n  ci:\n    cmds: [task gate]\n  gate:\n    cmds: [fathom verify --strict]\n")
	got := checkLefthookGates(root)
	if got.Status != doctorUnknown || !strings.Contains(got.Reason, "task ci") || !strings.Contains(got.Reason, "depth limit") {
		t.Fatalf("lefthook depth cut = %+v", got)
	}
}

func TestDoctorPushGateDepthLimitNamesScript(t *testing.T) {
	root := t.TempDir()
	gitRepo(t, root)
	doctorWriteFixture(t, root, ".git/hooks/pre-push", "#!/bin/sh\ntask validate\n")
	doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds: [sh scripts/gate.sh]\n")
	doctorWriteFixture(t, root, "scripts/gate.sh", "sh scripts/verify.sh\n")
	doctorWriteFixture(t, root, "scripts/verify.sh", "fathom verify --strict\n")
	got := checkLefthookGates(root)
	if got.Status != doctorUnknown || !strings.Contains(got.Reason, "scripts/gate.sh") || !strings.Contains(got.Reason, "depth limit") {
		t.Fatalf("script depth cut = %+v", got)
	}
}

func TestDoctorPushGateDepthCutWalkedTargets(t *testing.T) {
	for _, tc := range []struct{ name, tasks, script string }{
		{"task walked after cut", "  validate:\n    cmds: [task test, task build]\n  test:\n    cmds: [task build]\n  build:\n    cmds: [go build ./...]\n", ""},
		{"task walked before cut", "  validate:\n    cmds: [task build, task test]\n  test:\n    cmds: [task build]\n  build:\n    cmds: [go build ./...]\n", ""},
		{"cycle", "  validate:\n    cmds: [task ci]\n  ci:\n    cmds: [task validate]\n", ""},
		{"script walked after cut", "  validate:\n    cmds: [task ci, sh scripts/build.sh]\n  ci:\n    cmds: [sh scripts/build.sh]\n", "go build ./...\n"},
		{"script walked before cut", "  validate:\n    cmds: [sh scripts/build.sh, task ci]\n  ci:\n    cmds: [sh scripts/build.sh]\n", "go build ./...\n"},
		{"dynamic script read before cut", "  validate:\n    cmds: [sh scripts/build.sh, task ci]\n  ci:\n    cmds: [sh scripts/build.sh]\n", "x=$(echo)\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			gitRepo(t, root)
			doctorWriteFixture(t, root, ".git/hooks/pre-push", "task validate\n")
			doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n"+tc.tasks)
			if tc.script != "" {
				doctorWriteFixture(t, root, "scripts/build.sh", tc.script)
			}
			if got := checkLefthookGates(root); got.Status != doctorFail {
				t.Fatalf("walked cut target hides no commands: %+v", got)
			}
		})
	}
}

func TestDoctorPushGateDepthCutSymlinkAndHookCycle(t *testing.T) {
	t.Run("symlink to a walked script", func(t *testing.T) {
		root := t.TempDir()
		gitRepo(t, root)
		doctorWriteFixture(t, root, ".git/hooks/pre-push", "task validate\n")
		doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds: [sh scripts/build.sh, task ci]\n  ci:\n    cmds: [sh scripts/link.sh]\n")
		doctorWriteFixture(t, root, "scripts/build.sh", "go build ./...\n")
		if err := os.Symlink("build.sh", filepath.Join(root, "scripts", "link.sh")); err != nil {
			t.Fatal(err)
		}
		if got := checkLefthookGates(root); got.Status != doctorFail {
			t.Fatalf("symlink to a walked script hides nothing: %+v", got)
		}
	})
	t.Run("cycle back into the hook", func(t *testing.T) {
		root := t.TempDir()
		gitRepo(t, root)
		doctorWriteFixture(t, root, ".git/hooks/pre-push", "task validate\n")
		doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds: [task ci]\n  ci:\n    cmds: [sh .git/hooks/pre-push]\n")
		if got := checkLefthookGates(root); got.Status != doctorFail {
			t.Fatalf("a cycle into the hook hides nothing: %+v", got)
		}
	})
}

func TestDoctorLefthookPushGateDepthCutLiteralVerify(t *testing.T) {
	root := t.TempDir()
	gitRepo(t, root)
	doctorWriteFixture(t, root, ".git/hooks/pre-push", "lefthook run pre-push\n")
	doctorWriteFixture(t, root, "lefthook.yml", "pre-push:\n  commands:\n    validate:\n      run: task validate\n    verify:\n      run: fathom verify\n")
	doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds: [task ci]\n  ci:\n    cmds: [task gate]\n  gate:\n    cmds: [echo done]\n")
	got := checkLefthookGates(root)
	if got.Status != doctorPass || !strings.Contains(got.Reason, "has a fathom verify step") {
		t.Fatalf("literal lefthook verdict changed: %+v", got)
	}
}

func TestDoctorPushGateDepthCutPreservesReadBudget(t *testing.T) {
	root := t.TempDir()
	gitRepo(t, root)
	doctorWriteFixture(t, root, ".git/hooks/pre-push", "task validate\n")
	taskfile := "version: '3'\ntasks:\n  validate:\n    cmds: [task ci, sh scripts/verify.sh]\n  ci:\n    cmds: [sh scripts/big.sh]\n"
	doctorWriteFixture(t, root, "Taskfile.yml", taskfile)
	doctorWriteFixture(t, root, "scripts/big.sh", strings.Repeat("#", doctorWrapperBytes-len(taskfile)-1))
	doctorWriteFixture(t, root, "scripts/verify.sh", "fathom verify --strict\n")
	if got := checkLefthookGates(root); got.Status != doctorPass {
		t.Fatalf("cut target spent the read budget: %+v", got)
	}
}

func TestDoctorPushGateDepthCutRequiresWalkableTask(t *testing.T) {
	for _, entry := range []string{"    dir: elsewhere\n    cmds: [fathom verify --strict]\n", "    cmds: invalid\n", "    cmds: [{unknown: fathom verify --strict}]\n"} {
		for _, beyond := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/beyond=%t", strings.TrimSpace(entry), beyond), func(t *testing.T) {
				root := t.TempDir()
				gitRepo(t, root)
				cmd := "task gate"
				if beyond {
					cmd = "task ci"
				}
				doctorWriteFixture(t, root, ".git/hooks/pre-push", "task validate\n")
				doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  validate:\n    cmds: ["+cmd+"]\n  ci:\n    cmds: [task gate]\n  gate:\n"+entry)
				if got := checkLefthookGates(root); got.Status != doctorFail {
					t.Fatalf("unwalkable task hides no readable commands: %+v", got)
				}
			})
		}
	}
}

// Adopter report (2026-10-22): the pre-push hook git runs decides; an uninstalled lefthook.yml does not.
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
	t.Run("comment mentioning lefthook run is not a hand-off", func(t *testing.T) {
		root := doctorNativeOverLefthookFixture(t, "#!/bin/sh\n# do not call lefthook run pre-push here\ngit lfs pre-push \"$@\"\n")
		doctorWriteFixture(t, root, "lefthook.yml", "pre-push:\n  commands:\n    v:\n      run: fathom verify --strict\n")
		if got := checkLefthookGates(root); got.Status != doctorFail {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("unreadable hook is unknown", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads everything")
		}
		root := doctorNativeOverLefthookFixture(t, doctorNativeTaskHook)
		doctorWriteFixture(t, root, "lefthook.yml", "pre-push:\n  commands:\n    v:\n      run: fathom verify --strict\n")
		hook := filepath.Join(root, ".git/hooks/pre-push")
		if err := os.Chmod(hook, 0); err != nil {
			t.Fatal(err)
		}
		got := checkLefthookGates(root)
		if got.Status != doctorUnknown || !strings.Contains(got.Reason, ".git/hooks/pre-push") {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("hook symlinked outside the repository is unknown", func(t *testing.T) {
		root := doctorNativeOverLefthookFixture(t, doctorNativeTaskHook)
		doctorWriteFixture(t, root, "lefthook.yml", "pre-push:\n  commands:\n    v:\n      run: fathom verify --strict\n")
		outside := filepath.Join(t.TempDir(), "hook")
		if err := os.WriteFile(outside, []byte("#!/bin/sh\nfathom verify --strict\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		hook := filepath.Join(root, ".git/hooks/pre-push")
		_ = os.Remove(hook)
		if err := os.Symlink(outside, hook); err != nil {
			t.Fatal(err)
		}
		if got := checkLefthookGates(root); got.Status != doctorUnknown {
			t.Fatalf("got %+v", got)
		}
	})
	for name, hook := range map[string]string{
		"$LEFTHOOK run":     "#!/bin/sh\nexec \"$LEFTHOOK\" run pre-push\n",
		"npx --no lefthook": "#!/bin/sh\nnpx --no lefthook run pre-push\n",
	} {
		t.Run("hand-off "+name, func(t *testing.T) {
			root := doctorNativeOverLefthookFixture(t, hook)
			if got := checkLefthookGates(root); strings.Contains(got.Reason, "not installed") {
				t.Fatalf("got %+v", got)
			}
		})
	}
	t.Run("trailing comment is not a hand-off", func(t *testing.T) {
		root := doctorNativeOverLefthookFixture(t, "#!/bin/sh\ngit lfs pre-push \"$@\" # lefthook run pre-push\n")
		doctorWriteFixture(t, root, "lefthook.yml", "pre-push:\n  commands:\n    v:\n      run: fathom verify --strict\n")
		if got := checkLefthookGates(root); got.Status != doctorFail {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("unreadable external hook names no machine path", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads everything")
		}
		main := doctorNativeOverLefthookFixture(t, doctorNativeTaskHook)
		for _, args := range [][]string{
			{"-c", "user.email=a@b.c", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x"},
			{"worktree", "add", "-q", filepath.Join(main, "wt"), "-b", "wt"},
		} {
			if b, err := exec.Command("git", append([]string{"-C", main}, args...)...).CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v %s", args, err, b)
			}
		}
		if err := os.Chmod(filepath.Join(main, ".git/hooks/pre-push"), 0); err != nil {
			t.Fatal(err)
		}
		wt := filepath.Join(main, "wt")
		doctorWriteFixture(t, wt, "lefthook.yml", "pre-push:\n  commands:\n    v:\n      run: fathom verify --strict\n")
		got := checkLefthookGates(wt)
		if got.Status != doctorUnknown || strings.Contains(got.Reason, main) || !strings.Contains(got.Reason, "permission denied") {
			t.Fatalf("got %+v", got)
		}
	})
}
