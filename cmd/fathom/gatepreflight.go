package main

// gatepreflight.go — RO-40 (docs/plans/readouts.md; design
// docs/design/readouts.md section 20, H6): the gate's static half before a
// Bash `git commit` or `git push` runs. From what fathom computes without
// running a command, one advisory line, never a deny:
//
//   - verify: a plan row claimed [x] in the staged diff (commit) or in the
//     pushed range (push: upstream to HEAD, read through the read-only git
//     wrapper) that the verify engine refuses, through the claim moment's
//     own path (claimMomentRefusals, PC-22) inside the hook's deadline; a
//     miss reads `verify: not checked (deadline)`;
//   - gate (push only): no recorded passing run of a declared test command
//     at this head and tree in the session (RO-35's keys, the same reader
//     RO-39 uses), read as `gate: no passing run at this tree in this
//     session`.
//
// Silent when both hold. The command is lexed by shellcmd.ParseCommand,
// never matched as a pattern over the raw string, and an unreadable command
// is silent. The command guards (ADR-038) keep their deny: this line runs
// only after them. Off by default behind [hooks] gate_preflight.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// gatePreflightChannel is the hook-log channel of the preflight's one row
// and of its verify's error beats.
const gatePreflightChannel = "gate-preflight"

// verifyOriginGatePreflight is the verify ledger origin of a preflight's
// verify, kept apart from the claim moment's (PC-26 counts those).
const verifyOriginGatePreflight = "gate-preflight"

// gatePreflightVerifyCap bounds the verify call when no hook guard runs (a
// var so a test can shrink it).
var gatePreflightVerifyCap = 3 * time.Second

// gatePreflightVerbs reads which gated git verbs the command runs: a
// literal `git commit` or `git push` unit anywhere in it, nested bodies
// included. at is the first gated unit's literal `-C <dir>` path (each -C
// joined onto the last, as git reads them), "" when it names none. A -C
// the shell would compute, --git-dir or --work-tree is not a root the
// preflight can place: that unit is no gated verb.
func gatePreflightVerbs(c shellcmd.Command) (commit, push bool, at string) {
	for _, u := range c.Units() {
		if u.Kind != shellcmd.KindSimple || u.Name() != "git" || len(u.Words) < 2 || !u.Words[0].Literal() {
			continue
		}
		dir := ""
	scan:
		for i := 1; i < len(u.Words); i++ {
			w := u.Words[i]
			if !w.Literal() {
				break
			}
			switch {
			case w.Text == "-c":
				i++ // its value
			case w.Text == "-C":
				if i+1 >= len(u.Words) || !u.Words[i+1].Literal() {
					break scan
				}
				i++
				dir = joinBashPath(dir, u.Words[i].Text)
			case strings.HasPrefix(w.Text, "--git-dir") || strings.HasPrefix(w.Text, "--work-tree"):
				break scan
			case strings.HasPrefix(w.Text, "-"):
			case w.Text == "commit" || w.Text == "push":
				if !commit && !push {
					at = dir
				}
				commit = commit || w.Text == "commit"
				push = push || w.Text == "push"
				break scan
			default:
				break scan
			}
		}
	}
	return commit, push, at
}

// gatePreflightContext is the preflight's local budget: the hook guard's
// deadline, or the verify cap outside a hook.
func gatePreflightContext() (context.Context, context.CancelFunc) {
	if g := activeHookGuard(); g != nil && !g.Deadline.IsZero() {
		return context.WithDeadline(context.Background(), g.Deadline.At())
	}
	return context.WithTimeout(context.Background(), gatePreflightVerifyCap)
}

// gatePreflightLine is the advisory line for a Bash command, "" for
// silence (not a gated verb, the switch off, both halves holding, or any
// error). It logs one row on channel gate-preflight whenever it ran. cwd is
// the directory the command starts in ("" is projDir): a relative
// `git -C` resolves from the directory its leading cd reaches (hookGitDir).
func gatePreflightLine(projDir, cwd, command, sessionID, baseURL string) string {
	lexed := shellcmd.ParseCommand(command)
	if lexed.Unreadable {
		return ""
	}
	commit, push, at := gatePreflightVerbs(lexed)
	if !commit && !push {
		return ""
	}
	if at != "" {
		// `git -C <dir>` commits or pushes from dir's work tree (O1230); a
		// dir that is not one is no root to preflight.
		if cwd == "" {
			cwd = projDir
		}
		d := hookGitDir(command, at, cwd)
		if st, err := os.Stat(d); err != nil || !st.IsDir() {
			return ""
		}
		if projDir = gitToplevelByStat(d); projDir == "" {
			return ""
		}
	}
	cfg, ok := hooksConfigAt(projDir, hookOptInGatePreflight)
	if !ok || !cfg.Hooks.GatePreflight {
		return ""
	}
	ctx, cancel := gatePreflightContext()
	defer cancel()
	top, err := gitro.Output(ctx, projDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	root := strings.TrimSpace(string(top))

	meta := map[string]string{"verb": "commit"}
	if push {
		meta["verb"] = "push"
	}
	var parts []string
	refused, verifyNote := gatePreflightVerify(ctx, projDir, root, push, sessionID, baseURL)
	meta["verify"] = verifyNote
	meta["refused"] = strconv.Itoa(len(refused))
	switch {
	case len(refused) > 0:
		parts = append(parts, "verify: refused — "+strings.Join(refused, "; "))
	case verifyNote == "not checked (deadline)":
		parts = append(parts, "verify: not checked (deadline)")
	}
	if push {
		gate := gatePreflightPassingRun(ctx, root, sessionID, capabilityOnAt(root, "test-run-hook", cfg.Hooks.TestRun))
		meta["gate"] = gate
		if gate == "missing" {
			parts = append(parts, "gate: no passing run at this tree in this session")
		}
	}
	kind := "silent"
	line := ""
	if len(parts) > 0 {
		kind = "flagged"
		line = strings.ReplaceAll("🚦 fathom gate preflight (advisory): "+strings.Join(parts, "; "), "\n", " ")
	}
	emitBeat(gatePreflightChannel, kind, firingMeta(meta, sessionID, ""))
	return line
}

// gatePreflightClaim is one plan file a commit or push claims rows in.
type gatePreflightClaim struct {
	rel        string
	old, newer []byte
}

// gatePreflightClaims lists the depth-1 plans whose claimed rows the staged
// diff (commit) or the pushed range (push) adds, with both versions. ok is
// false when git cannot say (no upstream, a failed read).
func gatePreflightClaims(ctx context.Context, root string, push bool) ([]gatePreflightClaim, bool) {
	oldRev, newRev := "HEAD", "" // commit: HEAD against the index
	diffArgs := []string{"diff", "--cached", "--name-only", "--", "docs/plans"}
	if push {
		up, err := gitro.Output(ctx, root, "rev-parse", "--verify", "--quiet", "@{upstream}")
		if err != nil {
			return nil, false // nothing pushed before: no range to read
		}
		oldRev, newRev = strings.TrimSpace(string(up)), "HEAD"
		diffArgs = []string{"diff", "--name-only", oldRev, newRev, "--", "docs/plans"}
	}
	names, err := gitro.Output(ctx, root, diffArgs...)
	if err != nil {
		return nil, false
	}
	var out []gatePreflightClaim
	for _, rel := range strings.Split(strings.TrimSpace(string(names)), "\n") {
		if _, ok := claimMomentPlanRel(root, filepath.Join(root, filepath.FromSlash(rel))); !ok || rel == "" {
			continue
		}
		newer, err := gitro.Output(ctx, root, "show", newRev+":"+rel)
		if err != nil {
			continue // deleted in the range: nothing claimed
		}
		old, _ := gitro.Output(ctx, root, "show", oldRev+":"+rel) // nil: a new plan
		if len(newlyClaimedTasks(old, newer, "")) == 0 {
			continue
		}
		out = append(out, gatePreflightClaim{rel: rel, old: old, newer: newer})
	}
	return out, true
}

// gatePreflightVerify runs the claim moment's verify for every plan the
// commit or push claims rows in, inside ctx. The note is what the row
// logs: none (nothing claimed), checked, skipped (git or the daemon could
// not answer) or not checked (deadline).
func gatePreflightVerify(ctx context.Context, projDir, root string, push bool, sessionID, baseURL string) ([]string, string) {
	claims, ok := gatePreflightClaims(ctx, root, push)
	if deadlinePassed(ctx) {
		return nil, "not checked (deadline)"
	}
	if !ok {
		return nil, "skipped"
	}
	if len(claims) == 0 {
		return nil, "none"
	}
	var refused []string
	note := "checked"
	for _, c := range claims {
		remaining := time.Until(deadlineOf(ctx))
		if remaining <= 0 {
			return refused, "not checked (deadline)"
		}
		hc := &http.Client{Timeout: remaining}
		r, service, err := claimMomentRefusals(hc, baseURL, projDir, root, c.rel, c.old, c.newer, verifyOriginGatePreflight, gatePreflightChannel, sessionID, "")
		switch {
		case err != nil && (isTimeoutErr(err) || deadlinePassed(ctx) || hookDeadlineSpent()):
			return refused, "not checked (deadline)"
		case err != nil || service == "":
			note = "skipped" // daemon down or unindexed: operational, never evidence
		default:
			refused = append(refused, r...)
		}
	}
	return refused, note
}

// deadlineOf is ctx's deadline, or a cap from now when it carries none.
func deadlineOf(ctx context.Context) time.Time {
	if d, ok := ctx.Deadline(); ok {
		return d
	}
	return time.Now().Add(gatePreflightVerifyCap)
}

// gatePreflightPassingRun reads whether the session recorded a passing run
// of a declared test command at the current head and tree: passing,
// missing, or not checked (why). Without the test-run rows ([hooks]
// test_run off) nothing can be read; a tail that does not reach the
// session's last edit or start is not checked (window), never missing (I38).
func gatePreflightPassingRun(ctx context.Context, root, sessionID string, testRunOn bool) string {
	if !testRunOn {
		return "not checked (test_run off)"
	}
	runs := readSessionRuns(beatLogPath(), sessionID)
	if !runs.Read {
		return "not checked (log)"
	}
	var cur testRunRecord
	fillTestRunRowKeys(ctx, &cur, root, sessionID, "", "")
	if cur.Head == "" || cur.Tree == "" {
		return "not checked (deadline)"
	}
	if runs.passingAt(cur.Head, cur.Tree) {
		return "passing"
	}
	var start time.Time
	if runs.LastEdit.IsZero() {
		start, _ = sessionStartFor(root, sessionID)
	}
	if !runs.covers(start) {
		return "not checked (window)"
	}
	return "missing"
}

// emitGatePreflight prints the preflight line alone, as the PreToolUse
// hook's advisory context, when the guard path has nothing of its own.
func emitGatePreflight(codec harness.Codec, line string, stdout io.Writer) int {
	if line == "" {
		return 0
	}
	out, err := codec.EncodeOutput(harness.EventPreTool, harness.HookOutput{Context: line})
	if err != nil || len(out) == 0 {
		return 0
	}
	fmt.Fprintln(stdout, string(out))
	return 0
}
