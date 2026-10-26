package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestGitHelpWithoutControls(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"git-inspect", "--help"}, {"git-inspect", "-h"}} {
		var out bytes.Buffer
		if execute(args, &out) != 0 || !strings.Contains(out.String(), "git-inspect") || !strings.Contains(out.String(), "git-history") {
			t.Fatalf("help %v: %q", args, out.String())
		}
	}
}

func gitFixture(t *testing.T) (string, scope, policy) {
	t.Helper()
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	gitdir := filepath.Join(root, ".git")
	if e := os.MkdirAll(filepath.Join(gitdir, "objects"), 0700); e != nil {
		t.Fatal(e)
	}
	cfg := []byte("[core]\n\trepositoryformatversion = 0\n\tbare = false\n")
	if e := os.WriteFile(filepath.Join(gitdir, "config"), cfg, 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(gitdir, "HEAD"), []byte("ref: refs/heads/main\n"), 0600); e != nil {
		t.Fatal(e)
	}
	exe := filepath.Join(t.TempDir(), "fakegit")
	exe, _ = filepath.EvalSymlinks(filepath.Dir(exe))
	exe = filepath.Join(exe, "fakegit")
	if e := os.WriteFile(exe, []byte("fakegit"), 0500); e != nil {
		t.Fatal(e)
	}
	eh := sha256.Sum256([]byte("fakegit"))
	ch := sha256.Sum256(cfg)
	s := scope{Version: "command-guard.scope/1", RequiredLayerIDs: []string{"L"}, RequiredRuleIDs: []string{"R"}, RequiredGitProfiles: []string{"git-read/1"}}
	p := policy{Version: "command-guard.policy/1", Layers: []layer{{ID: "L", AllowedOperations: []string{"git-status", "git-object-diff", "git-history"}, Limits: limits{MaxFileBytes: 4 << 20, MaxTotalBytes: 64 << 20, MaxOutputBytes: 1 << 20, MaxResults: 1000}, DeniedPathPrefixes: []string{}}}, Rules: []rule{{ID: "R", Pattern: "SECRET"}}, Git: &gitPolicy{Profile: "git-read/1", Executable: exe, SHA256: hex.EncodeToString(eh[:]), ExpectedVersion: "git version 2.54.0", TimeoutMillis: 5000, Repository: gitRepository{Kind: "worktree", Root: root, GitDir: gitdir, CommonDir: gitdir, ConfigSHA256: hex.EncodeToString(ch[:]), WorktreeConfigSHA256: "absent", CommonDirFileSHA256: "absent"}}}
	return root, s, p
}
func fakeGit(objects map[string][]byte, status, diff, history string) gitExec {
	return func(ctx context.Context, exe, dir string, env, args []string, cap int) scannerRunResult {
		out := ""
		switch {
		case containsArgs(args, "version"):
			out = "git version 2.54.0\n"
		case containsArgs(args, "config"):
			out = "core.repositoryformatversion\n0\x00core.bare\nfalse\x00"
		case containsArgs(args, "rev-parse", "--show-object-format=storage"):
			out = "sha1\n"
		case containsArgs(args, "rev-parse", "--verify", "--quiet", "--end-of-options"):
			return scannerRunResult{started: true, exit: 1}
		case containsArgs(args, "rev-parse", "--verify"):
			for _, a := range args {
				if a == "HEAD^{commit}" {
					for id := range objects {
						out = id + "\n"
						break
					}
				}
			}
		case containsArgs(args, "status"):
			out = status
		case containsArgs(args, "diff-tree"):
			out = diff
		case containsArgs(args, "rev-list"):
			out = history
		case containsArgs(args, "cat-file", "-t"):
			id := args[len(args)-1]
			b := objects[id]
			if b == nil {
				return scannerRunResult{started: true, exit: 128}
			}
			if strings.Contains(string(b), "tree ") {
				out = "commit\n"
			} else {
				out = "blob\n"
			}
		case containsArgs(args, "cat-file", "-s"):
			id := args[len(args)-1]
			out = strconv.Itoa(len(objects[id])) + "\n"
		case containsArgs(args, "cat-file", "commit"):
			out = string(objects[args[len(args)-1]])
		case containsArgs(args, "cat-file", "blob"):
			out = string(objects[args[len(args)-1]])
		default:
			return scannerRunResult{started: true, exit: 128}
		}
		return scannerRunResult{started: true, out: []byte(out)}
	}
}
func containsArgs(args []string, seq ...string) bool {
	for i := 0; i+len(seq) <= len(args); i++ {
		ok := true
		for j, x := range seq {
			if args[i+j] != x {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}
func TestGitThreeRecipesAndLateRule(t *testing.T) {
	root, s, p := gitFixture(t)
	blob := []byte("public\n")
	bid := oidHash("blob", blob, "sha1")
	commit := []byte("tree " + strings.Repeat("a", 40) + "\nauthor A <a@example.test> 1 +0000\ncommitter A <a@example.test> 1 +0000\n\npublic message\n")
	cid := oidHash("commit", commit, "sha1")
	objects := map[string][]byte{bid: blob, cid: commit}
	status := "1 M. N... 100644 100644 100644 " + bid + " " + bid + " a.txt\x00"
	diff := ":000000 100644 " + strings.Repeat("0", 40) + " " + bid + " A\x00a.txt\x00"
	runner := fakeGit(objects, status, diff, cid+"\n")
	cases := []struct {
		kind string
		q    gitRequest
	}{{"git-status", gitRequest{Version: "command-guard.git-request/1", Kind: "git-status", Paths: []string{"."}}}, {"git-object-diff", gitRequest{Version: "command-guard.git-request/1", Kind: "git-object-diff", Base: cid, Target: cid, Paths: []string{"a.txt"}}}, {"git-history", gitRequest{Version: "command-guard.git-request/1", Kind: "git-history", Target: cid, MaxCommits: 1}}}
	for _, tc := range cases {
		r := runGit(root, s, p, tc.q, "json", runner)
		if r.Status != "no-findings" || len(r.Rows) != 1 || r.AttemptedCalls != r.CompletedCalls {
			t.Fatalf("%s: %+v", tc.kind, r)
		}
	}
	objects[bid] = []byte("SECRET\n")
	objects[oidHash("blob", objects[bid], "sha1")] = objects[bid]
	delete(objects, bid)
	bid = oidHash("blob", []byte("SECRET\n"), "sha1")
	runner = fakeGit(objects, status, ":000000 100644 "+strings.Repeat("0", 40)+" "+bid+" A\x00a.txt\x00", cid+"\n")
	q := gitRequest{Version: "command-guard.git-request/1", Kind: "git-object-diff", Base: cid, Target: cid, Paths: []string{"a.txt"}}
	r := runGit(root, s, p, q, "json", runner)
	if r.Status != "blocked" || len(r.Rows) != 0 {
		t.Fatalf("late rule: %+v", r)
	}
}
func TestGitRequestPresenceAndPolicy(t *testing.T) {
	for _, raw := range []string{`{"version":"command-guard.git-request/1","kind":"git-history","target":"HEAD","maxCommits":0}`, `{"version":"command-guard.git-request/1","kind":"git-status","paths":["."],"maxCommits":0}`, `{"version":"command-guard.git-request/1","kind":"git-object-diff","base":"HEAD","target":"HEAD","paths":["."] ,"maxCommits":0}`} {
		f := filepath.Join(t.TempDir(), "request.json")
		os.WriteFile(f, []byte(raw), 0600)
		var q gitRequest
		if strictFile(f, &q) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
func TestGitConfigIncludeBeforeRepositoryAndDrift(t *testing.T) {
	root, s, p := gitFixture(t)
	q := gitRequest{Version: "command-guard.git-request/1", Kind: "git-status", Paths: []string{"."}}
	calls := 0
	runner := func(ctx context.Context, exe, dir string, env, args []string, cap int) scannerRunResult {
		calls++
		if containsArgs(args, "version") {
			return scannerRunResult{started: true, out: []byte("git version 2.54.0\n")}
		}
		if containsArgs(args, "config") {
			return scannerRunResult{started: true, out: []byte("core.repositoryformatversion\n0\x00core.bare\nfalse\x00include.path\n/private/secret\x00")}
		}
		t.Fatalf("repository Git called before config rejection: %v", args)
		return scannerRunResult{}
	}
	r := runGit(root, s, p, q, "json", runner)
	if r.Code != "config_invalid" || calls != 2 || len(r.Rows) != 0 {
		t.Fatalf("%+v calls=%d", r, calls)
	}
	runner = fakeGit(nil, "? a.txt\x00", "", "")
	wrapped := func(ctx context.Context, exe, dir string, env, args []string, cap int) scannerRunResult {
		res := runner(ctx, exe, dir, env, args, cap)
		if containsArgs(args, "status") {
			os.WriteFile(filepath.Join(p.Git.Repository.GitDir, "HEAD"), []byte("ref: refs/heads/other\n"), 0600)
		}
		return res
	}
	r = runGit(root, s, p, q, "json", wrapped)
	if r.Code != "state_drift" || len(r.Rows) != 0 {
		t.Fatalf("%+v", r)
	}
}
func TestGitRunnerEnvironmentAndMetadataSymlink(t *testing.T) {
	root, s, p := gitFixture(t)
	q := gitRequest{Version: "command-guard.git-request/1", Kind: "git-status", Paths: []string{"."}}
	os.Setenv("GT6_SECRET_WITNESS", "private")
	defer os.Unsetenv("GT6_SECRET_WITNESS")
	runner := fakeGit(nil, "? a.txt\x00", "", "")
	wrapped := func(ctx context.Context, exe, dir string, env, args []string, cap int) scannerRunResult {
		for _, v := range env {
			if strings.Contains(v, "GT6_SECRET_WITNESS") {
				t.Fatal("inherited environment")
			}
		}
		return runner(ctx, exe, dir, env, args, cap)
	}
	r := runGit(root, s, p, q, "json", wrapped)
	if r.Status != "no-findings" {
		t.Fatalf("%+v", r)
	}
	os.Symlink(filepath.Join(root, "elsewhere"), filepath.Join(p.Git.Repository.GitDir, "refs"))
	r = runGit(root, s, p, q, "json", wrapped)
	if r.Code != "metadata_invalid" {
		t.Fatalf("%+v", r)
	}
}
func TestGitBoundedProjectionScansOmittedTail(t *testing.T) {
	root, s, p := gitFixture(t)
	cidRaw := []byte("tree " + strings.Repeat("a", 40) + "\nauthor A <a@example.test> 1 +0000\ncommitter A <a@example.test> 1 +0000\n\npublic\n")
	cid := oidHash("commit", cidRaw, "sha1")
	blob := []byte(strings.Repeat("public\n", 100000))
	bid := oidHash("blob", blob, "sha1")
	objects := map[string][]byte{cid: cidRaw, bid: blob}
	raw := ":000000 100644 " + strings.Repeat("0", 40) + " " + bid + " A\x00a.txt\x00"
	q := gitRequest{Version: "command-guard.git-request/1", Kind: "git-object-diff", Base: cid, Target: cid, Paths: []string{"a.txt"}}
	r := runGit(root, s, p, q, "json", fakeGit(objects, "", raw, ""))
	if r.Status != "no-findings" || !r.ProjectionTruncated || r.OmittedLines != 99992 || len(r.Rows) != 1 || len(r.Rows[0].After) != 8 {
		t.Fatalf("%+v", r)
	}
	blob = append(blob, []byte("SECRET late\n")...)
	bid = oidHash("blob", blob, "sha1")
	objects[bid] = blob
	raw = ":000000 100644 " + strings.Repeat("0", 40) + " " + bid + " A\x00a.txt\x00"
	r = runGit(root, s, p, q, "json", fakeGit(objects, "", raw, ""))
	if r.Status != "blocked" || len(r.Rows) != 0 {
		t.Fatalf("%+v", r)
	}
	view, omitted := lineView([]byte(strings.Repeat("x", 1<<20) + "\nsmall\n"))
	if omitted != 2 || len(view) != 0 {
		t.Fatalf("%d %v", omitted, view)
	}
}
func TestGitScopeRequiresDisclosureScan(t *testing.T) {
	root, s, p := gitFixture(t)
	runner := fakeGit(nil, "", "", "")
	q := gitRequest{Version: "command-guard.git-request/1", Kind: "git-status", Paths: []string{"SECRET.txt"}}
	r := runGit(root, s, p, q, "json", runner)
	if r.Status != "blocked" || r.Code != "rule" || len(r.Scope) != 0 || len(r.Rows) != 0 {
		t.Fatalf("secret selector disclosed: %+v", r)
	}
	q.Paths = []string{"public.txt"}
	p.Git.Repository.ConfigSHA256 = strings.Repeat("0", 64)
	r = runGit(root, s, p, q, "json", runner)
	if r.Status != "incomplete" || len(r.Scope) != 0 {
		t.Fatalf("failed metadata disclosed scope: %+v", r)
	}
	_, s, p = gitFixture(t)
	q.Paths = []string{"public.txt"}
	s.RequiredScannerIDs = []string{"gitleaks"}
	p.Scanner = &scannerPolicy{Kind: "gitleaks", Profile: "gitleaks-stdin-8.30.1", Executable: filepath.Join(t.TempDir(), "missing"), SHA256: strings.Repeat("0", 64), TimeoutMillis: 1000, MaxReportBytes: 1024}
	r = runGit(p.Git.Repository.Root, s, p, q, "json", runner)
	if r.Status != "incomplete" || r.Code != "scanner_incomplete" || len(r.Scope) != 0 {
		t.Fatalf("failed scanner disclosed scope: %+v", r)
	}
}
func TestGitStatusWireAndExecutionHolds(t *testing.T) {
	root, s, p := gitFixture(t)
	q := gitRequest{Version: "command-guard.git-request/1", Kind: "git-status", Paths: []string{"a.txt"}}
	cases := []struct{ name, wire, want string }{{"missing-nul", "? a.txt", "source_invalid"}, {"foreign-path", "? b.txt\x00", "source_invalid"}, {"conflict", "u UU N... 100644 100644 100644 100644 " + strings.Repeat("a", 40) + " " + strings.Repeat("a", 40) + " " + strings.Repeat("a", 40) + " a.txt\x00", "unsupported"}, {"bad-xy", "1 ZZ N... 100644 100644 100644 " + strings.Repeat("a", 40) + " " + strings.Repeat("a", 40) + " a.txt\x00", "unsupported"}, {"bad-oid", "1 M. N... 100644 100644 100644 xyz xyz a.txt\x00", "source_invalid"}, {"bad-mode", "1 M. N... 120000 100644 100644 " + strings.Repeat("a", 40) + " " + strings.Repeat("a", 40) + " a.txt\x00", "unsupported"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := runGit(root, s, p, q, "json", fakeGit(nil, tc.wire, "", ""))
			if r.Code != tc.want || len(r.Rows) != 0 || len(r.Scope) != 0 {
				t.Fatalf("%+v", r)
			}
		})
	}
	t.Run("runner-exit", func(t *testing.T) {
		base := fakeGit(nil, "? a.txt\x00", "", "")
		wrapped := func(ctx context.Context, exe, dir string, env, args []string, cap int) scannerRunResult {
			if containsArgs(args, "status") {
				return scannerRunResult{started: true, exit: 128, errout: []byte("private failure")}
			}
			return base(ctx, exe, dir, env, args, cap)
		}
		r := runGit(root, s, p, q, "json", wrapped)
		if r.Code != "git_incomplete" || r.StartedCalls != r.AttemptedCalls || r.CompletedCalls >= r.AttemptedCalls || len(r.Scope) != 0 {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("deadline", func(t *testing.T) {
		base := fakeGit(nil, "? a.txt\x00", "", "")
		wrapped := func(ctx context.Context, exe, dir string, env, args []string, cap int) scannerRunResult {
			if containsArgs(args, "status") {
				<-ctx.Done()
				return scannerRunResult{started: true, code: "deadline"}
			}
			return base(ctx, exe, dir, env, args, cap)
		}
		p2 := p
		p2.Git = &gitPolicy{}
		*p2.Git = *p.Git
		p2.Git.TimeoutMillis = 10
		r := runGit(root, s, p2, q, "json", wrapped)
		if r.Code != "deadline" || len(r.Scope) != 0 {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("denied-ancestor", func(t *testing.T) {
		q2 := q
		q2.Paths = []string{"."}
		p2 := p
		p2.Layers = append([]layer{}, p.Layers...)
		p2.Layers[0].DeniedPathPrefixes = []string{"secret"}
		r := runGit(root, s, p2, q2, "json", fakeGit(nil, "", "", ""))
		if r.Code != "denied" || r.AttemptedCalls != 0 {
			t.Fatalf("%+v", r)
		}
	})
}
func TestGitDiffObjectHolds(t *testing.T) {
	root, s, p := gitFixture(t)
	commit := []byte("tree " + strings.Repeat("a", 40) + "\nauthor A <a@example.test> 1 +0000\ncommitter A <a@example.test> 1 +0000\n\npublic\n")
	cid := oidHash("commit", commit, "sha1")
	blob := []byte("public\n")
	bid := oidHash("blob", blob, "sha1")
	objects := map[string][]byte{cid: commit, bid: blob}
	q := gitRequest{Version: "command-guard.git-request/1", Kind: "git-object-diff", Base: cid, Target: cid, Paths: []string{"a.txt"}}
	prefix := ":000000 100644 " + strings.Repeat("0", 40) + " "
	suffix := " A\x00a.txt\x00"
	cases := []struct{ name, wire, want string }{{"missing-nul", prefix + bid + " A\x00a.txt", "source_invalid"}, {"symlink", ":000000 120000 " + strings.Repeat("0", 40) + " " + bid + suffix, "unsupported"}, {"bad-oid", prefix + strings.Repeat("b", 40) + suffix, "git_incomplete"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := runGit(root, s, p, q, "json", fakeGit(objects, "", tc.wire, ""))
			if r.Code != tc.want || len(r.Rows) != 0 || r.BaseCommit != "" {
				t.Fatalf("%+v", r)
			}
		})
	}
	bad := map[string][]byte{cid: commit, bid: []byte("different\n")}
	r := runGit(root, s, p, q, "json", fakeGit(bad, "", prefix+bid+suffix, ""))
	if r.Code != "source_invalid" || len(r.Rows) != 0 {
		t.Fatalf("hash mismatch: %+v", r)
	}
}

func TestGitRejectsHiddenMalformedRecords(t *testing.T) {
	root, s, p := gitFixture(t)
	raw := []byte("tree " + strings.Repeat("a", 40) + "\nauthor A <a@example.test> 1 +0000\ncommitter A <a@example.test> 1 +0000\n\npublic\n")
	id := oidHash("commit", raw, "sha1")
	objects := map[string][]byte{id: raw}
	status := gitRequest{Version: "command-guard.git-request/1", Kind: "git-status", Paths: []string{"a.txt"}}
	diff := gitRequest{Version: "command-guard.git-request/1", Kind: "git-object-diff", Base: id, Target: id, Paths: []string{"a.txt"}}
	history := gitRequest{Version: "command-guard.git-request/1", Kind: "git-history", Target: id, MaxCommits: 1}
	cases := []struct {
		name                  string
		request               gitRequest
		status, diff, history string
	}{
		{"empty-status-record", status, "? a.txt\x00\x00", "", ""},
		{"empty-diff-record", diff, "", "\x00", ""},
		{"hidden-diff-tail", diff, "", "\x00unparsed\x00", ""},
		{"invalid-history-sentinel", history, "", "", id + "\ninvalid\n"},
		{"duplicate-history-sentinel", history, "", "", id + "\n" + id + "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := runGit(root, s, p, tc.request, "json", fakeGit(objects, tc.status, tc.diff, tc.history))
			if r.Status != "incomplete" || r.Code != "source_invalid" || len(r.Rows) != 0 || len(r.Scope) != 0 || r.TargetCommit != "" {
				t.Fatalf("malformed record accepted: %+v", r)
			}
		})
	}
}
func TestGitCLIOutputBudgetAndSink(t *testing.T) {
	root, s, p := gitFixture(t)
	q := gitRequest{Version: "command-guard.git-request/1", Kind: "git-status", Paths: []string{"public.txt"}}
	dir := t.TempDir()
	write := func(name string, v any) string {
		t.Helper()
		b, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(dir, name)
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
		return path
	}
	sp := write("scope.json", s)
	qp := write("request.json", q)
	p.Layers[0].Limits.MaxOutputBytes = 2
	pp := write("policy.json", p)
	args := []string{"--root", root, "--scope", sp, "--policy", pp, "--request", qp, "--format", "json"}
	var out bytes.Buffer
	if code := executeGit(args, &out); code != 2 || out.Len() != 0 {
		t.Fatalf("tiny cap code=%d output=%q", code, out.String())
	}
	p.Layers[0].Limits.MaxOutputBytes = 1024
	pp = write("policy.json", p)
	args[5] = pp
	out.Reset()
	if code := executeGit(args, &out); code != 2 || out.Len() == 0 || out.Len() > 1024 {
		t.Fatalf("failure cap code=%d bytes=%d", code, out.Len())
	}
	if code := executeGit(args, failWriter{}); code != 2 {
		t.Fatalf("sink code=%d", code)
	}
}
func TestGitHeadPinnedOnceAndSignedCommit(t *testing.T) {
	root, s, p := gitFixture(t)
	commit := []byte("tree " + strings.Repeat("a", 40) + "\nauthor A <a@example.test> 1 +0000\ncommitter A <a@example.test> 1 +0000\ngpgsig -----BEGIN PGP SIGNATURE-----\n fake-signature\n -----END PGP SIGNATURE-----\n\n")
	cid := oidHash("commit", commit, "sha1")
	if row, ok := parseCommit(cid, commit, "sha1"); !ok || row.Message != "" {
		t.Fatalf("signed commit parse %+v %v", row, ok)
	}
	blob := []byte("public\n")
	bid := oidHash("blob", blob, "sha1")
	objects := map[string][]byte{cid: commit, bid: blob}
	raw := ":000000 100644 " + strings.Repeat("0", 40) + " " + bid + " A\x00a.txt\x00"
	base := fakeGit(objects, "", raw, cid+"\n")
	headCalls := 0
	runner := func(ctx context.Context, exe, dir string, env, args []string, cap int) scannerRunResult {
		if containsArgs(args, "rev-parse", "--verify", "HEAD^{commit}") {
			headCalls++
			return scannerRunResult{started: true, out: []byte(cid + "\n")}
		}
		return base(ctx, exe, dir, env, args, cap)
	}
	q := gitRequest{Version: "command-guard.git-request/1", Kind: "git-object-diff", Base: "HEAD", Target: "HEAD", Paths: []string{"a.txt"}}
	r := runGit(root, s, p, q, "json", runner)
	if r.Status != "no-findings" || r.BaseCommit != cid || r.TargetCommit != cid || headCalls != 1 {
		t.Fatalf("%+v calls=%d", r, headCalls)
	}
	q = gitRequest{Version: "command-guard.git-request/1", Kind: "git-history", Target: "HEAD", MaxCommits: 1}
	r = runGit(root, s, p, q, "json", runner)
	if r.Status != "no-findings" || r.TargetCommit != cid || r.BaseCommit != "" {
		t.Fatalf("%+v", r)
	}
}
