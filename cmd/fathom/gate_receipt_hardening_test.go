package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gateRepo(t *testing.T) (string, func(...string) string) {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		c := exec.CommandContext(context.Background(), "git", args...)
		c.Dir = root
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.test", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.test")
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, b)
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-q")
	for _, name := range []string{"Taskfile.yml", "lefthook.yml"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("fixture\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("-c", "core.hooksPath="+os.DevNull, "commit", "-qm", "fixture")
	return root, git
}

func TestGateReceiptRequiresTwoLayerConsent(t *testing.T) {
	root, _ := gateRepo(t)
	home := t.TempDir()
	t.Setenv("FATHOM_HOME", home)
	if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, user, project string
		want                bool
	}{
		{"neither", "", "", false},
		{"project only", "", "accept_receipts = true", false},
		{"user only", "allow_receipt_acceptance = true", "", false},
		{"both", "allow_receipt_acceptance = true", "accept_receipts = true", true},
		{"project grants itself consent", "", "accept_receipts = true\nallow_receipt_acceptance = true", false},
		{"user alone opts repository in", "allow_receipt_acceptance = true\naccept_receipts = true", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte("[gate]\n"+tc.user+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".fathom/config.toml"), []byte("[gate]\n"+tc.project+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			_, got, err := gateReceiptOptions(context.Background(), root, "validate")
			if err != nil || got != tc.want {
				t.Fatalf("accept=%v want=%v err=%v", got, tc.want, err)
			}
		})
	}
}

func TestGateReceiptDisabledIsSilent(t *testing.T) {
	root, _ := gateRepo(t)
	t.Setenv("FATHOM_HOME", t.TempDir())
	var out, errb bytes.Buffer
	rc := cmdGateReceiptCheck([]string{"--root", root, "--if-enabled"}, &out, &errb)
	if rc != 1 || out.Len() != 0 || errb.Len() != 0 {
		t.Fatalf("disabled: %d %q %q", rc, &out, &errb)
	}
}

func TestGateReceiptDoctorNamesHalfConsent(t *testing.T) {
	for _, project := range []bool{false, true} {
		root, _ := gateRepo(t)
		user := filepath.Join(t.TempDir(), "config.toml")
		if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0700); err != nil {
			t.Fatal(err)
		}
		path, body := user, "[gate]\nallow_receipt_acceptance = true\n"
		if project {
			path = filepath.Join(root, ".fathom/config.toml")
			body = "[gate]\naccept_receipts = true\n"
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		c := checkKeyScope(root, user)
		if c.Status != doctorWarn || !strings.Contains(c.Reason, "allow_receipt_acceptance") || !strings.Contains(c.Reason, "accept_receipts") {
			t.Fatalf("half consent: %+v", c)
		}
	}
}

func TestGateReceiptUserDirectoryIsNotRefused(t *testing.T) {
	root, _ := gateRepo(t)
	home := t.TempDir()
	t.Setenv("FATHOM_HOME", home)
	if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0700); err != nil {
		t.Fatal(err)
	}
	user := filepath.Join(home, "config.toml")
	if err := os.WriteFile(user, []byte("[gate]\nreceipts_dir = '/evidence'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".fathom/config.toml"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	c := checkKeyScope(root, user)
	if c.Status != doctorPass {
		t.Fatalf("user directory refused: %+v", c)
	}
	if err := os.WriteFile(filepath.Join(root, ".fathom/config.toml"), []byte("[gate]\nreceipts_dir = '/evidence'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c = checkKeyScope(root, user)
	if c.Status != doctorWarn || !strings.Contains(c.Reason, "gate.receipts_dir") {
		t.Fatalf("project directory accepted: %+v", c)
	}
}

func TestGateReceiptWriteCommandGuard(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_HOME", t.TempDir())
	t.Setenv("FATHOM_ENFORCE", "on")
	for _, command := range []string{"fathom gate receipt write --log gate.log", "bin/fathom gate receipt write --log gate.log"} {
		if d := evaluateEnforcedCommand(root, command); !d.Denied {
			t.Fatalf("writer not guarded: %s %+v", command, d)
		}
	}
	if d := evaluateEnforcedCommand(root, "fathom gate receipt check --tree HEAD"); d.Denied {
		t.Fatalf("check denied: %+v", d)
	}
}

func TestGateLogBindsTreeAndWritesReceipt(t *testing.T) {
	script, err := filepath.Abs("../../scripts/gate-log.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"pass", "dirty-start", "dirty-end", "tree-change"} {
		t.Run(kind, func(t *testing.T) {
			root, git := gateRepo(t)
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("workspace/\n"), 0600); err != nil {
				t.Fatal(err)
			}
			git("add", ".")
			git("-c", "core.hooksPath="+os.DevNull, "commit", "-qm", "ignore logs")
			tree := git("rev-parse", "HEAD^{tree}")
			commit := git("rev-parse", "HEAD")
			bins := t.TempDir()
			receiptLog := filepath.Join(t.TempDir(), "receipt.log")
			goScript := "#!/bin/sh\ncase \"$1\" in\nversion) echo 'go version fixture fixture/fixture';;\nenv) printf 'fixture-os\\nfixture-arch\\noff\\n0\\n\\n';;\n*) exit 2;;\nesac\n"
			writer := "#!/bin/sh\n[ \"$1 $2 $3\" = 'gate receipt write' ] || exit 2\nwhile [ \"$#\" -gt 0 ]; do if [ \"$1\" = --log ]; then cat \"$2\" >\"$RECEIPT_LOG\"; exit; fi; shift; done\nexit 2\n"
			for name, body := range map[string]string{"go": goScript, "fathom": writer} {
				if err := os.WriteFile(filepath.Join(bins, name), []byte(body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			body := "true"
			switch kind {
			case "dirty-start":
				if err := os.WriteFile(filepath.Join(root, "Taskfile.yml"), []byte("dirty\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "dirty-end":
				body = "echo dirty >>Taskfile.yml"
			case "tree-change":
				body = "echo changed >>Taskfile.yml; git -c core.hooksPath=/dev/null commit -qam changed"
			}
			c := exec.CommandContext(context.Background(), "bash", script, "validate", "sh", "-c", body)
			c.Dir = root
			c.Env = append(os.Environ(), "PATH="+bins+":"+os.Getenv("PATH"), "RECEIPT_LOG="+receiptLog, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.test", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.test")
			b, err := c.CombinedOutput()
			s := string(b)
			if kind == "pass" {
				if err != nil {
					t.Fatalf("gate: %v %s", err, b)
				}
				if strings.Count(s, "tree="+tree+" clean=1 commit="+commit) != 2 {
					t.Fatalf("missing tree endpoints: %s", b)
				}
				copied, e := os.ReadFile(receiptLog)
				if e != nil || !strings.Contains(string(copied), "fathom gate: validate PASS") {
					t.Fatalf("no automatic receipt: %s %v", copied, e)
				}
				if !strings.Contains(s, "go version fixture fixture/fixture") || !strings.Contains(s, "fixture-os\nfixture-arch\noff\n0\n\n") {
					t.Fatalf("missing toolchain header: %s", b)
				}
			} else {
				if err == nil || strings.Contains(s, " validate PASS ") || !strings.Contains(s, " validate FAIL ") {
					t.Fatalf("changed/dirty gate accepted: %v %s", err, b)
				}
				if _, e := os.Stat(receiptLog); !os.IsNotExist(e) {
					t.Fatalf("wrote a receipt after failure: %v", e)
				}
			}
		})
	}
}

func gateCommandLog(t *testing.T, root string) string {
	t.Helper()
	read := func(ref string) string {
		c := exec.CommandContext(context.Background(), "git", "rev-parse", ref)
		c.Dir = root
		b, err := c.Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(b))
	}
	stamp := "tree=" + read("HEAD^{tree}") + " clean=1 commit=" + read("HEAD")
	return "fathom gate: validate START 2026-10-22T01:00:00Z " + stamp + "\ngo version fixture fixture/fixture\nfathom gate: go env GOOS GOARCH GOWORK CGO_ENABLED GOFLAGS\nfixture-os\nfixture-arch\noff\n0\n\nchecks passed\nfathom gate: validate PASS 2026-10-22T02:00:00Z " + stamp + "\n"
}
