package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"gopkg.in/yaml.v3"
)

func TestGateReceiptCommandRoundTrip(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FATHOM_HOME", t.TempDir())
	t.Chdir(root)
	git := func(args ...string) {
		t.Helper()
		c := exec.CommandContext(context.Background(), "git", args...)
		c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.test", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.test")
		if b, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, b, err)
		}
	}
	git("init", "-q")
	for _, name := range []string{"Taskfile.yml", "lefthook.yml"} {
		if err := os.WriteFile(name, []byte("fixture\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("-c", "core.hooksPath="+os.DevNull, "commit", "-qm", "fixture")
	log := filepath.Join(t.TempDir(), "gate.log")
	if err := os.WriteFile(log, []byte(gateCommandLog(t, root)), 0600); err != nil {
		t.Fatal(err)
	}
	goBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(goBin, "go"), []byte("#!/bin/sh\n[ \"$GOTOOLCHAIN\" = local ] || exit 99\ncase \"$1\" in version) printf 'go version fixture fixture/fixture\\n';; env) printf '{\"GOOS\":\"fixture-os\",\"GOARCH\":\"fixture-arch\",\"GOWORK\":\"off\"}\\n';; *) exit 2;; esac\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", goBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOTOOLCHAIN", "auto")
	invoke := func(args ...string) (int, string, string) {
		var out, err bytes.Buffer
		rc := run(args, &out, &err)
		return rc, out.String(), err.String()
	}
	if rc, out, err := invoke("gate", "receipt", "check", "--if-enabled"); rc != 1 || out != "" || err != "" {
		t.Fatalf("off: %d %s %s", rc, out, err)
	}
	rc, id, err := invoke("gate", "receipt", "write", "--log", log)
	if rc != 0 || len(strings.TrimSpace(id)) != 64 {
		t.Fatalf("write: %d %s %s", rc, id, err)
	}
	if rc, out, err := invoke("gate", "receipt", "check"); rc != 0 || out != id {
		t.Fatalf("check: %d %s %s", rc, out, err)
	}
	if err := os.MkdirAll(".fathom", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".fathom/config.toml", []byte("[gate]\naccept_receipts = true\nreceipts_dir = 'workspace/receipts'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(os.Getenv("FATHOM_HOME"), "config.toml"), []byte("[gate]\nallow_receipt_acceptance = true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if rc, _, err := invoke("gate", "receipt", "check", "--if-enabled"); rc != 1 || !strings.Contains(err, "dirty") {
		t.Fatalf("dirty: %d %s", rc, err)
	}
}

func TestGateReceiptLefthookFallsBackUnlessOptedInAndAccepted(t *testing.T) {
	b, err := os.ReadFile("../../lefthook.yml")
	if err != nil {
		t.Fatal(err)
	}
	var hook struct {
		PrePush struct {
			Commands map[string]struct{ Run string } `yaml:"commands"`
		} `yaml:"pre-push"`
	}
	if err := yaml.Unmarshal(b, &hook); err != nil {
		t.Fatal(err)
	}
	script := hook.PrePush.Commands["validate"].Run
	if !strings.Contains(script, "gate receipt check --if-enabled") {
		t.Fatal("no opt-in branch")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	write := func(name, s string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(s), 0700); err != nil {
			t.Fatal(err)
		}
	}
	write("scripts/prepush-refs.sh", "#!/bin/sh\n[ \"$1\" = receipt-trees ] || exit 1\nprintf '%s\\n' tree-one tree-two\n")
	write("bin/fathom", "#!/bin/sh\n[ \"$1 $2 $3 $4 $5\" = 'gate receipt check --if-enabled --tree' ] || exit 2\nprintf '%s\\n' \"$6\" >> checked\nif [ \"${RECEIPT_SECOND_RC:-}\" != '' ] && [ \"$6\" = tree-two ]; then exit \"$RECEIPT_SECOND_RC\"; fi\nexit \"$RECEIPT_RC\"\n")
	write("bin/task", "#!/bin/sh\nprintf 'gate ran\\n'\n")
	for _, tc := range []struct {
		first, second string
		gate          bool
	}{{"1", "1", true}, {"0", "0", false}, {"2", "2", true}, {"0", "1", true}, {"1", "0", true}} {
		rc := tc.first
		if err := os.WriteFile(filepath.Join(root, "checked"), nil, 0600); err != nil {
			t.Fatal(err)
		}
		c := exec.CommandContext(context.Background(), "sh", "-c", script)
		c.Dir = root
		c.Env = append(os.Environ(), "PATH="+filepath.Join(root, "bin")+":"+os.Getenv("PATH"), "RECEIPT_RC="+rc, "RECEIPT_SECOND_RC="+tc.second)
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(out), "gate ran") != tc.gate {
			t.Fatalf("receipt exits %s/%s: %s", rc, tc.second, out)
		}
		checked, err := os.ReadFile(filepath.Join(root, "checked"))
		if err != nil || string(checked) != "tree-one\ntree-two\n" {
			t.Fatalf("not all pushed SHAs checked: %s %v", checked, err)
		}
	}
}

func TestGateRecordIDsCommand(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ADR.md"), []byte("## ADR-001: first\n## ADR-001: duplicate\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if rc := run([]string{"gate", "record-ids", "--root", root}, &out, &errb); rc != 1 || !strings.Contains(errb.String(), "ADR-001") || !strings.Contains(errb.String(), "ADR.md:1 and ADR.md:2") {
		t.Fatalf("%d %s %s", rc, &out, &errb)
	}
	if err := os.WriteFile(filepath.Join(root, "ADR.md"), []byte("## ADR-001: first\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	if rc := run([]string{"gate", "record-ids", "--root", root}, &out, &errb); rc != 0 || !strings.Contains(out.String(), "unique") {
		t.Fatalf("%d %s %s", rc, &out, &errb)
	}
}

func TestGateLogWrapperOwnsTerminalVerdict(t *testing.T) {
	script, err := filepath.Abs("../../scripts/gate-log.sh")
	if err != nil {
		t.Fatal(err)
	}
	root, git := gateRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("workspace/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("-c", "core.hooksPath="+os.DevNull, "commit", "-qm", "ignore logs")
	bins := t.TempDir()
	if err := os.WriteFile(filepath.Join(bins, "fathom"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWORK", "off")
	for _, tc := range []struct{ code, word string }{{"0", "PASS"}, {"7", "FAIL"}} {
		c := exec.CommandContext(context.Background(), "bash", script, "validate", "sh", "-c", "printf 'gate output'; exit "+tc.code)
		c.Dir = root
		c.Env = append(os.Environ(), "PATH="+bins+":"+os.Getenv("PATH"))
		b, err := c.CombinedOutput()
		if tc.code == "0" && err != nil {
			t.Fatal(err)
		}
		if tc.code != "0" {
			e, ok := err.(*exec.ExitError)
			if !ok || e.ExitCode() != 7 {
				t.Fatalf("exit: %v", err)
			}
		}
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		if !strings.HasPrefix(lines[0], "fathom gate: validate START ") || !strings.HasPrefix(lines[len(lines)-1], "fathom gate: validate "+tc.word+" ") {
			t.Fatalf("framing: %s", b)
		}
	}
}

func TestGateReceiptsDirectoryGuard(t *testing.T) {
	gs, err := guardspec.LoadGuards("../../.fathom/guard.cue")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"workspace/receipts/id.json", "workspace/receipts/id.log"} {
		g, ok := matchingFileGuard(gs, p)
		if !ok || len(guardspec.EvaluateGuardEnforcement(g, p, "edited", "original")) == 0 {
			t.Fatalf("receipt evidence unprotected: %s", p)
		}
	}
}
