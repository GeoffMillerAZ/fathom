package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestDoctorNativeGateUnadopted(t *testing.T) {
	for _, body := range []string{"echo push\n", "# task validate\n# fathom verify --strict\n"} {
		root := t.TempDir()
		doctorWriteFixture(t, root, ".git/hooks/pre-push", body)
		if row, ok := checkComposedPushGate(root); ok {
			t.Errorf("unadopted hook produced row: %+v", row)
		}
		if row := checkLefthookGates(root); row.Status != doctorSkipped {
			t.Errorf("unadopted hook: %+v", row)
		}
	}
}

func TestDoctorNativeGatePartial(t *testing.T) {
	for _, body := range []string{"task validate\n", "fathom verify --strict\n", "fathom verify\n"} {
		root := t.TempDir()
		doctorWriteFixture(t, root, ".git/hooks/pre-push", body)
		if row := checkLefthookGates(root); row.Status != doctorFail {
			t.Errorf("partial adoption: %+v", row)
		}
	}
}

func TestDoctorGateProofTaskfile(t *testing.T) {
	root := t.TempDir()
	doctorWriteFixture(t, root, ".git/hooks/pre-push", "task push-gate\n")
	doctorWriteFixture(t, root, "Taskfile.yml", "version: '3'\ntasks:\n  push-gate:\n    cmds:\n      - task validate\n      - fathom verify --strict\n")
	if row := checkLefthookGates(root); row.Status != doctorPass || !strings.Contains(row.Reason, "Taskfile.yml") {
		t.Fatalf("proof missing: %+v", row)
	}
}

func TestDoctorGateHookLocations(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(map[bool]string{false: "husky", true: "hooksPath"}[custom], func(t *testing.T) {
			root := t.TempDir()
			if b, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
				t.Fatalf("git init: %v %s", err, b)
			}
			path := ".husky/pre-push"
			if custom {
				path = "hooks/pre-push"
				if b, err := exec.Command("git", "-C", root, "config", "core.hooksPath", "hooks").CombinedOutput(); err != nil {
					t.Fatalf("git config: %v %s", err, b)
				}
			}
			doctorWriteFixture(t, root, path, "task validate\nfathom verify --strict\n")
			if row := checkLefthookGates(root); row.Status != doctorPass || !strings.Contains(row.Reason, path) {
				t.Fatalf("hook missed: %+v", row)
			}
		})
	}
}

func TestDoctorGateHuskyDispatcher(t *testing.T) {
	root := t.TempDir()
	if b, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, b)
	}
	if b, err := exec.Command("git", "-C", root, "config", "core.hooksPath", ".husky/_").CombinedOutput(); err != nil {
		t.Fatalf("git config: %v %s", err, b)
	}
	doctorWriteFixture(t, root, ".husky/_/pre-push", "#!/usr/bin/env sh\n. \"$(dirname \"$0\")/h\"\n")
	doctorWriteFixture(t, root, ".husky/pre-push", "task validate\nfathom verify --strict\n")
	if row := checkLefthookGates(root); row.Status != doctorPass || !strings.Contains(row.Reason, ".husky/pre-push") {
		t.Fatalf("husky dispatcher: %+v", row)
	}
}
