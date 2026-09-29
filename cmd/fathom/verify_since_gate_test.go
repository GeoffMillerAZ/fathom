package main

// verify_since_gate_test.go — push 79 (ADR-018 note 2026-09-29, I45): the
// pre-push verify step reads the change range, `--since $(git merge-base
// HEAD origin/main)`, so a `library: true` waiver added in the same range as
// the claim it excuses is self-declared and refused, even when the waiver
// and the claim sit in different commits (O56's one-commit comparison alone
// lets that through). Two halves: the verb's behaviour on a real ingest and
// a real git history, and the lefthook step itself, run by bash against a
// stub binary so the test pins the arguments the gate passes and its exit
// policy (1 blocks, 2 fails open).

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	sinceGateOpenRow    = "- [ ] only a test calls it {id: W-2, symbol: TestOnly, package: lib}\n"
	sinceGateWaivedOpen = "- [ ] only a test calls it {id: W-2, symbol: TestOnly, package: lib, library: true, reason: \"waived inside the range\"}\n"
	sinceGateWaivedDone = "- [x] only a test calls it {id: W-2, symbol: TestOnly, package: lib, library: true, reason: \"waived inside the range\"}\n"
)

// TestCmdVerify_StrictSince_LibraryWaiverInsideTheRangeFails: the waiver
// lands in one commit and the claim in the next. Without --since the gate
// passes (the waiver predates the claiming commit); with --since the
// range's base it fails, because the waiver entered in the same range as
// its claim.
func TestCmdVerify_StrictSince_LibraryWaiverInsideTheRangeFails(t *testing.T) {
	storePath, service := wiredFixtureStore(t)
	repo := newSelfSkipGitRepo(t)

	pf := repo.writePlan("docs/plans/w.md", sinceGateOpenRow)
	repo.commitAll("base: the row is open and carries no waiver")
	baseSHA := gitRevParse(t, repo.root, "HEAD")

	repo.writePlan("docs/plans/w.md", sinceGateWaivedOpen)
	repo.commitAll("waive the row while it is still open")
	repo.writePlan("docs/plans/w.md", sinceGateWaivedDone)
	repo.commitAll("claim the row in a later commit")

	code, out, errBuf := runInDir(t, repo.root, []string{
		"verify", "--strict", "--service", service, "--plan", pf, "--store", storePath,
	})
	if code != 0 {
		t.Fatalf("without --since the one-commit check sees a waiver older than the claim: want exit 0, got %d\nstdout=%s\nstderr=%s", code, out, errBuf)
	}

	code, out, errBuf = runInDir(t, repo.root, []string{
		"verify", "--strict", "--service", service, "--plan", pf, "--store", storePath,
		"--since", baseSHA,
	})
	if code != 1 {
		t.Fatalf("--since the range base: a waiver added in the claim's range must fail --strict, got exit %d\nstdout=%s\nstderr=%s", code, out, errBuf)
	}
	if !strings.Contains(out, "self-declared") || !strings.Contains(out, "W-2") {
		t.Errorf("want W-2 reported self-declared, got:\n%s", out)
	}
}

// TestCmdVerify_StrictSince_LibraryWaiverBeforeTheRangePasses: the same
// waiver, already on the base the range starts from, stands (the change is
// not retroactive to waivers already on main).
func TestCmdVerify_StrictSince_LibraryWaiverBeforeTheRangePasses(t *testing.T) {
	storePath, service := wiredFixtureStore(t)
	repo := newSelfSkipGitRepo(t)

	pf := repo.writePlan("docs/plans/w.md", sinceGateWaivedOpen)
	repo.commitAll("base: the waiver is already there, the row still open")
	baseSHA := gitRevParse(t, repo.root, "HEAD")

	repo.writePlan("docs/plans/w.md", sinceGateWaivedDone)
	repo.commitAll("claim the row; the waiver is unchanged")

	code, out, errBuf := runInDir(t, repo.root, []string{
		"verify", "--strict", "--service", service, "--plan", pf, "--store", storePath,
		"--since", baseSHA,
	})
	if code != 0 {
		t.Fatalf("a waiver predating the range stands: want exit 0, got %d\nstdout=%s\nstderr=%s", code, out, errBuf)
	}
}

// lefthookStepRun returns the `run:` body of one lefthook command in the
// repository's own lefthook.yml.
func lefthookStepRun(t *testing.T, hook, step string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "lefthook.yml"))
	if err != nil {
		t.Fatalf("read lefthook.yml: %v", err)
	}
	var doc map[string]struct {
		Commands map[string]struct {
			Run string `yaml:"run"`
		} `yaml:"commands"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse lefthook.yml: %v", err)
	}
	run := doc[hook].Commands[step].Run
	if run == "" {
		t.Fatalf("lefthook.yml has no %s.%s run body", hook, step)
	}
	return run
}

// gateSandbox is a throwaway git repository with a stub bin/fathom that logs
// its arguments and exits with $FATHOM_STUB_VERIFY_EXIT for verify, and a
// pass-through scripts/gate-lease.sh, so the lefthook step runs for real
// without a store, a lease or the project repository.
type gateSandbox struct {
	root, log string
}

func newGateSandbox(t *testing.T, withOriginMain bool) gateSandbox {
	t.Helper()
	for _, bin := range []string{"git", "bash"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not available", bin)
		}
	}
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("docs/plans/p.md", sinceGateOpenRow)
	repo.commitAll("base")
	if withOriginMain {
		repo.run("update-ref", "refs/remotes/origin/main", "HEAD")
	}
	repo.writePlan("docs/plans/p.md", sinceGateWaivedDone)
	repo.commitAll("the pushed change")

	files := map[string]string{
		"bin/fathom": "#!/bin/sh\necho \"$*\" >> \"$FATHOM_STUB_LOG\"\n" +
			"[ \"$1\" = ingest ] && exit 0\nexit \"${FATHOM_STUB_VERIFY_EXIT:-0}\"\n",
		"scripts/gate-lease.sh": "#!/usr/bin/env bash\nexec \"$@\"\n",
	}
	for rel, body := range files {
		p := filepath.Join(repo.root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return gateSandbox{root: repo.root, log: filepath.Join(t.TempDir(), "stub.log")}
}

func (g gateSandbox) run(t *testing.T, body, verifyExit string) (code int, out, calls string) {
	t.Helper()
	_ = os.Remove(g.log)
	cmd := exec.Command("bash", "-c", body)
	cmd.Dir = g.root
	cmd.Env = append(os.Environ(), "FATHOM_STUB_LOG="+g.log, "FATHOM_STUB_VERIFY_EXIT="+verifyExit)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	code = 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run the verify step: %v\n%s", err, buf.String())
	}
	logged, _ := os.ReadFile(g.log)
	return code, buf.String(), string(logged)
}

// TestLefthook_PrePushVerifyReadsTheChangeRange runs lefthook.yml's own
// pre-push verify body: every verify call carries --strict and --since the
// merge-base with origin/main; verify's exit 1 blocks the push and exit 2
// fails open.
func TestLefthook_PrePushVerifyReadsTheChangeRange(t *testing.T) {
	body := lefthookStepRun(t, "pre-push", "verify")
	g := newGateSandbox(t, true)
	base := gitRevParse(t, g.root, "HEAD~1")

	code, out, calls := g.run(t, body, "0")
	if code != 0 {
		t.Fatalf("verify exit 0: want the step to pass, got %d\n%s", code, out)
	}
	var verifyCalls []string
	for _, l := range strings.Split(strings.TrimSpace(calls), "\n") {
		if strings.HasPrefix(l, "verify ") {
			verifyCalls = append(verifyCalls, l)
		}
	}
	if len(verifyCalls) != 1 {
		t.Fatalf("want one verify call (one plan), got %q", calls)
	}
	if !strings.HasPrefix(verifyCalls[0], "verify --strict --since "+base+" ") || !strings.Contains(verifyCalls[0], "--plan docs/plans/p.md") {
		t.Errorf("verify call = %q, want --strict --since %s over docs/plans/p.md", verifyCalls[0], base)
	}

	if code, out, _ := g.run(t, body, "1"); code != 1 {
		t.Errorf("verify exit 1 (a refused row, a self-declared waiver among them) must block the push, got %d\n%s", code, out)
	}
	code, out, _ = g.run(t, body, "2")
	if code != 0 || !strings.Contains(out, "fail-open") {
		t.Errorf("verify exit 2 must fail open with a named line, got %d\n%s", code, out)
	}
}

// TestLefthook_PrePushVerifyWithoutOriginMainSaysSo: no origin/main ref, no
// merge-base; the step verifies without --since and names what it could
// not check, rather than failing the push or passing in silence.
func TestLefthook_PrePushVerifyWithoutOriginMainSaysSo(t *testing.T) {
	body := lefthookStepRun(t, "pre-push", "verify")
	g := newGateSandbox(t, false)

	code, out, calls := g.run(t, body, "0")
	if code != 0 {
		t.Fatalf("want the step to pass, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "no merge-base with origin/main") {
		t.Errorf("want the no-merge-base line, got:\n%s", out)
	}
	if strings.Contains(calls, "--since") || !strings.Contains(calls, "verify --strict --store ") {
		t.Errorf("want verify --strict without --since, got %q", calls)
	}
}
