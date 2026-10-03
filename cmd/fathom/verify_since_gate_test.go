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
//
// EW-16: the range's base is the remote sha lefthook hands the step on stdin
// (`use_stdin: true`; git feeds pre-push "<local ref> <local sha> <remote ref>
// <remote sha>" per pushed ref), the ref this push replaces, so a stale local
// origin/main neither blocks a legitimate push nor widens the range. A new
// branch (an all-zero remote sha), a remote sha this clone does not hold, a
// delete-only push or no stdin at all fall back to the merge-base with
// origin/main and say so. The step runs under every shell the gate meets:
// lefthook's own `sh` (dash on Debian and Ubuntu, CS-8) and every bash,
// macOS /bin/bash 3.2 included.

import (
	"bytes"
	"fmt"
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

// gateShells lists the shells the step runs under, each once: lefthook's
// own `sh` — it runs every `run:` body with `sh -c`, which is dash on
// Debian and Ubuntu (the cloud session VMs, CS-8) and bash 3.2 in POSIX
// mode on a stock Mac — then macOS /bin/bash (3.2, the oldest bash the
// gate meets) and the first bash on PATH. `sh` is always its own entry: a
// bash run as sh is a different dialect, and dash has no --version to
// compare.
func gateShells(t *testing.T) []string {
	t.Helper()
	var shells, versions []string
	if path, err := exec.LookPath("sh"); err == nil {
		shells = append(shells, path)
	}
	for _, cand := range []string{"/bin/bash", "bash"} {
		path, err := exec.LookPath(cand)
		if err != nil {
			continue
		}
		out, err := exec.Command(path, "--version").Output()
		if err != nil {
			continue
		}
		first, _, _ := strings.Cut(string(out), "\n")
		dup := false
		for _, v := range versions {
			dup = dup || v == first
		}
		if !dup {
			shells, versions = append(shells, path), append(versions, first)
		}
	}
	if len(shells) == 0 {
		t.Skip("no sh or bash available")
	}
	return shells
}

// eachGateShell runs fn once per shell as a named subtest.
func eachGateShell(t *testing.T, fn func(t *testing.T, shell string)) {
	t.Helper()
	for i, shell := range gateShells(t) {
		t.Run(fmt.Sprintf("shell%d_%s", i, filepath.Base(shell)), func(t *testing.T) { fn(t, shell) })
	}
}

// gateSandbox is a throwaway git repository with a stub bin/fathom that logs
// its arguments and exits with $FATHOM_STUB_VERIFY_EXIT for verify, and a
// pass-through scripts/gate-lease.sh, so the lefthook step runs for real
// without a store, a lease or the project repository.
type gateSandbox struct {
	root, log string
	repo      *selfSkipGitRepo
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
	return gateSandbox{root: repo.root, log: filepath.Join(t.TempDir(), "stub.log"), repo: repo}
}

// run executes the step body under shell with stdin as the pre-push ref
// lines (empty: no stdin, as when the step is run by hand).
func (g gateSandbox) run(t *testing.T, shell, body, verifyExit, stdin string) (code int, out, calls string) {
	t.Helper()
	_ = os.Remove(g.log)
	cmd := exec.Command(shell, "-c", body)
	cmd.Dir = g.root
	cmd.Env = append(os.Environ(), "FATHOM_STUB_LOG="+g.log, "FATHOM_STUB_VERIFY_EXIT="+verifyExit)
	cmd.Stdin = strings.NewReader(stdin)
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

// verifyCall returns the sandbox's one logged `verify` call.
func verifyCall(t *testing.T, calls string) string {
	t.Helper()
	var found []string
	for _, l := range strings.Split(strings.TrimSpace(calls), "\n") {
		if strings.HasPrefix(l, "verify ") {
			found = append(found, l)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want one verify call (one plan), got %q", calls)
	}
	return found[0]
}

// TestLefthook_PrePushVerifyReadsTheChangeRange runs lefthook.yml's own
// pre-push verify body with no stdin (the step run by hand): every verify
// call carries --strict and --since the merge-base with origin/main;
// verify's exit 1 blocks the push and exit 2 fails open.
func TestLefthook_PrePushVerifyReadsTheChangeRange(t *testing.T) {
	body := lefthookStepRun(t, "pre-push", "verify")
	eachGateShell(t, func(t *testing.T, shell string) {
		g := newGateSandbox(t, true)
		base := gitRevParse(t, g.root, "HEAD~1")

		code, out, calls := g.run(t, shell, body, "0", "")
		if code != 0 {
			t.Fatalf("verify exit 0: want the step to pass, got %d\n%s", code, out)
		}
		vc := verifyCall(t, calls)
		if !strings.HasPrefix(vc, "verify --strict --since "+base+" ") || !strings.Contains(vc, "--plan docs/plans/p.md") {
			t.Errorf("verify call = %q, want --strict --since %s over docs/plans/p.md", vc, base)
		}

		if code, out, _ := g.run(t, shell, body, "1", ""); code != 1 {
			t.Errorf("verify exit 1 (a refused row, a self-declared waiver among them) must block the push, got %d\n%s", code, out)
		} else if !strings.Contains(out, "the range was read from "+base) {
			t.Errorf("a block under --since must name the base it read, so a stale origin/main is diagnosable, got:\n%s", out)
		}
		code, out, _ = g.run(t, shell, body, "2", "")
		if code != 0 || !strings.Contains(out, "fail-open") {
			t.Errorf("verify exit 2 must fail open with a named line, got %d\n%s", code, out)
		}
	})
}

// TestLefthook_PrePushVerifyWithoutOriginMainSaysSo: no origin/main ref, no
// merge-base; the step verifies without --since and names what it could
// not check, rather than failing the push or passing in silence.
func TestLefthook_PrePushVerifyWithoutOriginMainSaysSo(t *testing.T) {
	body := lefthookStepRun(t, "pre-push", "verify")
	eachGateShell(t, func(t *testing.T, shell string) {
		g := newGateSandbox(t, false)

		code, out, calls := g.run(t, shell, body, "0", "")
		if code != 0 {
			t.Fatalf("want the step to pass, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "no merge-base with origin/main") {
			t.Errorf("want the no-merge-base line, got:\n%s", out)
		}
		if strings.Contains(calls, "--since") || !strings.Contains(calls, "verify --strict --store ") {
			t.Errorf("want verify --strict without --since, got %q", calls)
		}
		if code, out, _ := g.run(t, shell, body, "1", ""); code != 1 || strings.Contains(out, "the range was read from") {
			t.Errorf("without a merge-base a block still blocks and names no range base, got %d\n%s", code, out)
		}
	})
}

// refLine is one line git feeds a pre-push hook.
func refLine(lref, lsha, rref, rsha string) string {
	return lref + " " + lsha + " " + rref + " " + rsha + "\n"
}

var zeroSHA = strings.Repeat("0", 40)

// TestLefthook_PrePushVerifyReadsTheRemoteShaFromStdin (EW-16): the range
// base is the remote sha on stdin, not the local origin/main. The sandbox's
// origin/main is stale on purpose: two commits behind the remote's tip.
func TestLefthook_PrePushVerifyReadsTheRemoteShaFromStdin(t *testing.T) {
	body := lefthookStepRun(t, "pre-push", "verify")
	eachGateShell(t, func(t *testing.T, shell string) {
		g := newGateSandbox(t, true) // origin/main = base; HEAD = "the pushed change"
		g.repo.writePlan("docs/plans/p.md", sinceGateWaivedDone+"<!-- this push -->\n")
		g.repo.commitAll("this push")
		head := gitRevParse(t, g.root, "HEAD")
		remoteTip := gitRevParse(t, g.root, "HEAD~1") // what the remote holds
		staleBase := gitRevParse(t, g.root, "HEAD~2") // origin/main, stale
		since := func(t *testing.T, calls string) string {
			t.Helper()
			f := strings.Fields(verifyCall(t, calls))
			for i, w := range f {
				if w == "--since" && i+1 < len(f) {
					return f[i+1]
				}
			}
			return ""
		}

		t.Run("the remote sha replaces the stale origin/main as the base", func(t *testing.T) {
			stdin := refLine("refs/heads/main", head, "refs/heads/main", remoteTip)
			code, out, calls := g.run(t, shell, body, "0", stdin)
			if code != 0 {
				t.Fatalf("want the step to pass, got %d\n%s", code, out)
			}
			if got := since(t, calls); got != remoteTip {
				t.Errorf("--since %s, want the remote sha %s (a stale origin/main would give %s)", got, remoteTip, staleBase)
			}
			code, out, _ = g.run(t, shell, body, "1", stdin)
			if code != 1 || !strings.Contains(out, "the range was read from "+remoteTip+" (the remote sha this push replaces)") {
				t.Errorf("a block must name the remote sha it read, got %d\n%s", code, out)
			}
			if strings.Contains(out, "fetch main") {
				t.Errorf("a block on a remote-sha range has no stale-origin remedy to offer, got:\n%s", out)
			}
		})

		t.Run("a new branch has no remote sha and falls back to the merge-base, saying so", func(t *testing.T) {
			stdin := refLine("refs/heads/feat", head, "refs/heads/feat", zeroSHA)
			code, out, calls := g.run(t, shell, body, "0", stdin)
			if code != 0 {
				t.Fatalf("want the step to pass, got %d\n%s", code, out)
			}
			if got := since(t, calls); got != staleBase {
				t.Errorf("--since %s, want the merge-base with origin/main %s", got, staleBase)
			}
			if !strings.Contains(out, "new branch") || !strings.Contains(out, "merge-base with origin/main") {
				t.Errorf("the fallback must say why, got:\n%s", out)
			}
			if code, out, _ := g.run(t, shell, body, "1", stdin); code != 1 || !strings.Contains(out, "the range was read from "+staleBase+" (merge-base with origin/main)") {
				t.Errorf("a block on the merge-base fallback names it, got %d\n%s", code, out)
			}
		})

		t.Run("a remote sha this clone does not hold falls back, saying so", func(t *testing.T) {
			unknown := strings.Repeat("ab", 20)
			stdin := refLine("refs/heads/main", head, "refs/heads/main", unknown)
			code, out, calls := g.run(t, shell, body, "0", stdin)
			if code != 0 {
				t.Fatalf("want the step to pass, got %d\n%s", code, out)
			}
			if got := since(t, calls); got != staleBase {
				t.Errorf("--since %s, want the merge-base fallback %s", got, staleBase)
			}
			if !strings.Contains(out, unknown) || !strings.Contains(out, "not a commit in this clone") {
				t.Errorf("the fallback must name the sha it could not use, got:\n%s", out)
			}
		})

		t.Run("a force push reads the fork point, not a remote sha outside HEAD's history", func(t *testing.T) {
			branch := strings.TrimSpace(func() string {
				out, err := exec.Command("git", "-C", g.root, "rev-parse", "--abbrev-ref", "HEAD").Output()
				if err != nil {
					t.Fatal(err)
				}
				return string(out)
			}())
			g.repo.run("checkout", "-q", "-b", "rewritten", staleBase)
			g.repo.writePlan("docs/plans/p.md", sinceGateOpenRow+"<!-- the remote's own history -->\n")
			g.repo.commitAll("a commit only the remote has")
			diverged := gitRevParse(t, g.root, "HEAD")
			g.repo.run("checkout", "-q", branch)
			stdin := refLine("refs/heads/main", head, "refs/heads/main", diverged)
			_, out, calls := g.run(t, shell, body, "0", stdin)
			if got := since(t, calls); got != staleBase {
				t.Errorf("--since %s, want the fork point %s\n%s", got, staleBase, out)
			}
		})

		t.Run("no stdin, a delete-only push or a blank line fall back to the merge-base", func(t *testing.T) {
			for name, stdin := range map[string]string{
				"empty":       "",
				"delete only": refLine("(delete)", zeroSHA, "refs/heads/old", remoteTip),
				"blank":       "\n",
			} {
				code, out, calls := g.run(t, shell, body, "0", stdin)
				if code != 0 {
					t.Fatalf("%s: want the step to pass, got %d\n%s", name, code, out)
				}
				if got := since(t, calls); got != staleBase {
					t.Errorf("%s: --since %s, want the merge-base %s", name, got, staleBase)
				}
			}
		})

		t.Run("with several refs the one pushing HEAD names the base", func(t *testing.T) {
			stdin := refLine("refs/tags/v1", gitRevParse(t, g.root, "HEAD~2"), "refs/tags/v1", zeroSHA) +
				refLine("refs/heads/main", head, "refs/heads/main", remoteTip)
			_, out, calls := g.run(t, shell, body, "0", stdin)
			if got := since(t, calls); got != remoteTip {
				t.Errorf("--since %s, want the remote sha of the ref pushing HEAD %s\n%s", got, remoteTip, out)
			}
		})
	})
}

// TestLefthook_PrePushVerifyReceivesStdinFromLefthook (EW-16): the whole
// path through the real lefthook. The repository's own verify command (its
// `use_stdin` setting and its body, nothing else of lefthook.yml) runs on a
// real `git push` from a clone whose origin/main is stale, and the stub
// bin/fathom sees --since the remote's tip. Without `use_stdin: true` in
// lefthook.yml lefthook hands the step no stdin, and the step falls back to
// the stale origin/main: this test is red then. Skipped without lefthook.
func TestLefthook_PrePushVerifyReceivesStdinFromLefthook(t *testing.T) {
	if _, err := exec.LookPath("lefthook"); err != nil {
		t.Skip("lefthook not available")
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "lefthook.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		PrePush struct {
			Commands map[string]struct {
				Run      string `yaml:"run"`
				UseStdin bool   `yaml:"use_stdin"`
			} `yaml:"commands"`
		} `yaml:"pre-push"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	verify, ok := doc.PrePush.Commands["verify"]
	if !ok {
		t.Fatal("lefthook.yml has no pre-push verify command")
	}
	only, err := yaml.Marshal(map[string]any{"pre-push": map[string]any{"commands": map[string]any{
		"verify": map[string]any{"run": verify.Run, "use_stdin": verify.UseStdin},
	}}})
	if err != nil {
		t.Fatal(err)
	}

	g := newGateSandbox(t, true) // origin/main = base (soon stale); HEAD = pushed change
	remote := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	g.repo.run("remote", "add", "origin", remote)
	g.repo.run("push", "-q", "--no-verify", "origin", "HEAD:refs/heads/main") // the remote's tip
	remoteTip := gitRevParse(t, g.root, "HEAD")
	staleBase := gitRevParse(t, g.root, "HEAD~1")
	g.repo.run("update-ref", "refs/remotes/origin/main", staleBase) // this clone never fetched
	g.repo.writePlan("docs/plans/p.md", sinceGateWaivedDone+"<!-- this push -->\n")
	g.repo.commitAll("this push")

	if err := os.WriteFile(filepath.Join(g.root, "lefthook.yml"), only, 0o644); err != nil {
		t.Fatal(err)
	}
	env := []string{"FATHOM_STUB_LOG=" + g.log, "FATHOM_STUB_VERIFY_EXIT=0"}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "LEFTHOOK=") { // LEFTHOOK=0 opts out of the very hook under test
			env = append(env, kv)
		}
	}
	lh := exec.Command("lefthook", "install")
	lh.Dir, lh.Env = g.root, env
	if out, err := lh.CombinedOutput(); err != nil {
		t.Fatalf("lefthook install: %v\n%s", err, out)
	}
	push := exec.Command("git", "push", "origin", "HEAD:refs/heads/main")
	push.Dir, push.Env = g.root, env
	if out, err := push.CombinedOutput(); err != nil {
		t.Fatalf("git push through lefthook: %v\n%s", err, out)
	}
	logged, err := os.ReadFile(g.log)
	if err != nil {
		t.Fatalf("the pre-push hook never ran the verify step: %v", err)
	}
	if vc := verifyCall(t, string(logged)); !strings.HasPrefix(vc, "verify --strict --since "+remoteTip+" ") {
		t.Errorf("verify call = %q, want --since the remote tip %s (the stale origin/main is %s)", vc, remoteTip, staleBase)
	}
}
