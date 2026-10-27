package main

// egress.go — ADR-083's egress guard at the PreToolUse hook: the shipped
// egress pack (internal/guardspec/egress.cue) over a Bash command on Claude
// Code and Codex, and over Claude Code's own WebFetch|WebSearch. warn (the
// default) injects one advisory line; enforce denies while guard
// enforcement is on; off is silent. The user sets the mode and the
// allowlist; a repository may tighten the mode and never loosen it. String
// matching is a tripwire, not data-loss prevention: the harness's own
// permissions and sandbox are the boundary, and doctor's egress_guard row
// names them.

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/capability"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// egressCapability is the switchable capability id; [guards] egress is its
// config.toml switch (configSwitchKeys).
const egressCapability = "egress-guard"

// egressPolicy is the egress guard's resolved configuration for one root.
type egressPolicy struct {
	mode  string // config.EgressOff, EgressWarn or EgressEnforce
	allow []string
	// source is the config layer whose mode is in effect; a repository's
	// own enforce obeys the repository's enforcement switches, the user's
	// obeys only the user's.
	source config.Source
	// broken is the config file that did not load ("" when both did), and
	// loadErr why; doctor's egress_guard row names it.
	broken  string
	loadErr error
}

// egressPolicyAt resolves the mode for root: [guards] egress (the user's,
// tightened by the repository's), then the egress-guard capability. A
// repository config that does not load is skipped, never read as warn, so
// it cannot drop the user's enforce or allowlist; a user config that does
// not load leaves warn with no allowlist.
func egressPolicyAt(root string) egressPolicy {
	repoPath := projectdir.Config.Path(root)
	loaded, err := config.Load(config.DefaultUserPath(), repoPath)
	var p egressPolicy
	if err != nil {
		p.broken, p.loadErr = repoPath, err
		loaded, err = config.Load(config.DefaultUserPath(), "")
		if err != nil {
			return egressPolicy{mode: config.EgressWarn, broken: config.DefaultUserPath(), loadErr: err}
		}
	}
	g := loaded.Config.Guards
	p.mode, p.allow, p.source = g.EgressMode(), g.EgressAllow, g.EgressSource
	reg := capability.Default()
	c, ok := reg.Get(egressCapability)
	if !ok {
		return p
	}
	res, errs := loadCapabilityConfig(reg, root)
	if len(errs) > 0 {
		return p
	}
	switch set := res.For(c); {
	case set.Scope == capability.ScopeConfig || set.Scope == capability.ScopeBuiltIn:
	case !set.On:
		// Only the user switches it off (ScopeUserGoverns); a repository
		// that tightened the mode in its own config keeps it.
		if p.source != config.SourceProject {
			p.mode = config.EgressOff
		}
	case p.mode == config.EgressOff:
		p.mode = config.EgressWarn
	}
	return p
}

// egressDenies reports whether a match under p denies: enforce, with guard
// enforcement on. The user's enforce reads only the user's switches, so a
// repository's [guards] enforce = false cannot loosen it.
func egressDenies(root string, p egressPolicy) bool {
	if p.mode != config.EgressEnforce {
		return false
	}
	if p.source == config.SourceProject {
		return enforceEnabled(root)
	}
	return userEnforceEnabled()
}

// userEnforceEnabled is enforceEnabled read from the user's layers alone.
// Unlike it, a user config that does not load turns no deny on.
func userEnforceEnabled() bool {
	if os.Getenv("FATHOM_ENFORCE") == "off" {
		return false
	}
	loaded, err := config.Load(config.DefaultUserPath(), "")
	if err != nil {
		return false
	}
	on := loaded.Config.Guards.Enforce == nil || *loaded.Config.Guards.Enforce
	reg := capability.Default()
	if p := capability.DefaultUserPath(); p != "" {
		if s, err := reg.LoadSettings(p, capability.ScopeUser); err == nil {
			if v, set := s["guard-enforce"]; set {
				on = v
			}
		}
	}
	return on
}

// egressPack loads the shipped pack bound to p's allowlist and a git remote
// resolver for command, which runs from cwd.
func egressPack(p egressPolicy, command, cwd string) (guardspec.GuardSet, bool) {
	gs, err := guardspec.EgressPack(guardspec.EgressOptions{
		Allow: p.allow,
		RemoteURLs: func(dir, remote string) ([]string, bool) {
			return egressRemoteURLs(hookGitDir(command, dir, cwd), remote)
		},
	})
	return gs, err == nil
}

// egressRemoteURLs reads a remote's push URLs with git in dir; remote ""
// is the current branch's push remote, else origin. Any failure leaves the
// target unknown: no such remote (exit 2) or no repository (exit 128) now
// may be added by an earlier step of the same command, and a deadline
// says nothing.
func egressRemoteURLs(dir, remote string) ([]string, bool) {
	if dir == "" {
		return nil, false
	}
	ctx, cancel := gatePreflightContext()
	defer cancel()
	if remote == "" {
		remote = egressDefaultPushRemote(ctx, dir)
	}
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "remote", "get-url", "--push", "--all", remote).Output()
	if err != nil {
		return nil, false
	}
	urls := strings.Fields(string(out))
	return urls, len(urls) > 0
}

// egressDefaultPushRemote is the remote a bare `git push` uses: the current
// branch's push remote (branch.<b>.pushRemote, remote.pushDefault,
// branch.<b>.remote), else origin.
func egressDefaultPushRemote(ctx context.Context, dir string) string {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "for-each-ref", "--format=%(HEAD) %(push:remotename)", "refs/heads/").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if name, ok := strings.CutPrefix(line, "* "); ok && strings.TrimSpace(name) != "" {
				return strings.TrimSpace(name)
			}
		}
	}
	return "origin"
}

// egressCommandMatch is the egress pack's match for a Bash command under
// the policy at projDir; ok false when the guard is off or nothing matched.
func egressCommandMatch(projDir, cwd, command string) (guardspec.Guard, egressPolicy, bool) {
	p := egressPolicyAt(projDir)
	if p.mode == config.EgressOff {
		return guardspec.Guard{}, p, false
	}
	if cwd == "" {
		cwd = projDir
	}
	gs, ok := egressPack(p, command, cwd)
	if !ok {
		return guardspec.Guard{}, p, false
	}
	g, matched := guardspec.MatchCommandGuard(gs, command)
	return g, p, matched
}

// cmdEditHookPreWeb is the PreToolUse answer for a harness web tool
// (WebFetch, WebSearch): the egress pack's host check, and nothing else.
func cmdEditHookPreWeb(codec harness.Codec, in harness.HookInput, armOff bool, stdout io.Writer) int {
	meta := map[string]string{"phase": "pre", "target": "web"}
	emitBeat("edit-hook", "seen", firingMetaArm(meta, in.SessionID, "", armOff))
	if armOff {
		emitBeat("edit-hook", "answered", firingMetaArm(silentAnswerMeta(map[string]string{"phase": "pre", "target": "web"}, silenceArmOff), in.SessionID, "", armOff))
		return 0
	}
	projDir := resolveProjDirFor(codec.ID(), in.Cwd)
	p := egressPolicyAt(projDir)
	var g guardspec.Guard
	matched := false
	if p.mode != config.EgressOff {
		if gs, ok := egressPack(p, "", projDir); ok {
			var domains []string
			if in.WebDomains != "" {
				domains = strings.Split(in.WebDomains, "\n")
			}
			g, matched = guardspec.MatchEgressWeb(gs, in.ToolName, in.URL, domains)
		}
	}
	if !matched {
		emitBeat("edit-hook", "answered", firingMeta(silentAnswerMeta(map[string]string{"phase": "pre", "target": "web"}, silenceNoGuard), in.SessionID, ""))
		return 0
	}
	if egressDenies(projDir, p) {
		if v := guardspec.EvaluateCommandEnforcement(g); v != nil {
			d := newEnforceDecision(*v, "")
			if out, err := codec.EncodeOutput(harness.EventPreTool, harness.HookOutput{Deny: true, DenyReason: d.Reason}); err == nil {
				emitBeat("edit-hook", "denied", firingMeta(map[string]string{"phase": "pre", "target": "web"}, in.SessionID, ""))
				recordBlockedOutcome(d.OutcomeID(), in.SessionID, "")
				fmt.Fprintln(stdout, string(out))
				return 0
			}
		}
	}
	text := "⚖️ fathom guard — web: " + g.Note
	emitPayloadCost("pre-edit-guard", text, 0, "pre", in.SessionID, nil)
	out, err := codec.EncodeOutput(harness.EventPreTool, harness.HookOutput{Context: text})
	if err != nil {
		emitBeat("edit-hook", "answered", firingMeta(silentAnswerMeta(map[string]string{"phase": "pre", "target": "web"}, silenceEncodeFailed), in.SessionID, ""))
		return 0
	}
	emitBeat("edit-hook", "answered", firingMeta(map[string]string{"emitted": "true", "phase": "pre", "target": "web"}, in.SessionID, ""))
	fmt.Fprintln(stdout, string(out))
	return 0
}

// egressNativeControls names each harness's own boundary, which this
// tripwire does not replace.
func egressNativeControls(harnessID string) string {
	switch harnessID {
	case harness.Codex:
		return "Codex: fathom's hooks see Bash only, so Codex web search and MCP calls are not intercepted; the boundary is Codex's sandbox network setting ([sandbox_workspace_write] network_access = false in ~/.codex/config.toml)"
	default:
		return "Claude Code: the boundary is its own permissions.deny (for example WebFetch and Bash(curl:*)) and the sandbox network allowlist in settings.json"
	}
}

// checkEgressGuard is doctor's egress_guard row: recommends the guard when
// it is off and names the harness's native control either way.
func checkEgressGuard(root, harnessLabel string) doctorCheck {
	const name = "egress_guard"
	native := egressNativeControls(doctorHarnessID(harnessLabel))
	p := egressPolicyAt(root)
	const limit = "string matching is a tripwire, not data-loss prevention"
	switch {
	case p.loadErr != nil:
		return doctorCheck{Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("%s does not load (%v); the egress guard runs without it: %s, %d allowlisted host(s)", p.broken, p.loadErr, p.mode, len(p.allow)),
			Remedy: fmt.Sprintf("fix or remove %s; %s; %s", p.broken, limit, native)}
	case p.mode == config.EgressOff:
		return doctorCheck{Name: name, Status: doctorWarn,
			Reason: "the egress guard is off: shell uploads, remote copies, git push to an unlisted host and web fetches pass without a line",
			Remedy: fmt.Sprintf(`remove [guards] egress = "off" (or the egress-guard switch) from ~/.fathom/config.toml to warn again; %s; %s`, limit, native)}
	case p.mode == config.EgressEnforce && !egressDenies(root, p):
		return doctorCheck{Name: name, Status: doctorWarn,
			Reason: "[guards] egress = \"enforce\" but guard enforcement is off (FATHOM_ENFORCE=off, [guards] enforce = false or guard-enforce off), so it only warns",
			Remedy: "turn guard enforcement back on to deny; " + native}
	}
	return doctorCheck{Name: name, Status: doctorPass,
		Reason: fmt.Sprintf("the egress guard is on (%s, %d allowlisted host(s)); %s; %s", p.mode, len(p.allow), limit, native)}
}
