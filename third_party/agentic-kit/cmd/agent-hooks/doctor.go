package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Doctor verdicts.
const (
	pass = "PASS"
	warn = "WARN"
	fail = "FAIL"
)

// ledgerStaleAfter is how old the newest ledger record may be before doctor
// warns that the measurement channel looks dead while hooks are registered.
const ledgerStaleAfter = 7 * 24 * time.Hour

// doctor runs every check and prints one line per finding. It never writes
// host configuration. Exit 1 when any check fails.
func (a *app) doctor(onlyHost string) int {
	failed := false
	report := func(verdict, check, detail string) {
		if verdict == fail {
			failed = true
		}
		fmt.Fprintf(a.stdout, "%-4s %-22s %s\n", verdict, check, detail)
	}

	report(pass, "version", buildVersion())

	lp := a.loadPolicy()
	switch {
	case lp.Fallback != nil:
		report(fail, "policy", fmt.Sprintf("rejected, built-in policy in force: %v", lp.Fallback))
	default:
		report(pass, "policy", fmt.Sprintf("%s (%d tiers)", lp.Source, len(lp.Tiers)))
	}
	for _, h := range []string{hostClaude, hostCodex} {
		if !lp.hostMapped(h) {
			report(warn, "policy "+h, "no models listed: model and effort are required, tiers are not enforced")
		}
	}
	for _, r := range selfTest(lp.Policy) {
		report(r.verdict, "self-test", r.detail)
	}

	a.checkStateDir(report)
	a.checkLedger(onlyHost, report)

	for _, h := range []string{hostClaude, hostCodex} {
		if onlyHost != "" && onlyHost != h {
			continue
		}
		a.checkWiring(h, report)
		a.checkAgents(h, report)
	}
	if failed {
		return 1
	}
	return 0
}

type selfTestResult struct{ verdict, detail string }

// selfTest runs the effective policy through the real decision function with
// one case per rule, proving the policy is usable, not just parseable.
func selfTest(p *Policy) []selfTestResult {
	type tc struct {
		name     string
		req      spawnRequest
		wantDeny bool
		skip     bool
	}
	cheapest := func(host string) string {
		for _, t := range p.Tiers {
			if m := p.hostModels(t, host); len(m) > 0 {
				return m[0]
			}
		}
		return ""
	}
	claudeModel, codexModel := cheapest(hostClaude), cheapest(hostCodex)
	cases := []tc{
		{name: expectLabel("claude spawn without a model", p.Spawn.RequireModel), req: spawnRequest{Host: hostClaude, Brief: "find the config loader"}, wantDeny: p.Spawn.RequireModel},
		{name: "claude spawn on the cheapest tier is allowed", req: spawnRequest{Host: hostClaude, Model: claudeModel, Brief: "find the config loader"}, skip: claudeModel == ""},
		{name: expectLabel("claude fork without the marker", p.Spawn.Fork != forkAllow), req: spawnRequest{Host: hostClaude, Fork: true, Brief: "continue"}, wantDeny: p.Spawn.Fork != forkAllow},
		{name: expectLabel("codex spawn without model and effort", p.Spawn.RequireModel || p.requiresEffort(hostCodex)), req: spawnRequest{Host: hostCodex, Brief: "find the config loader"}, wantDeny: p.Spawn.RequireModel || p.requiresEffort(hostCodex)},
		{name: "codex spawn on the cheapest tier is allowed", req: spawnRequest{Host: hostCodex, Model: codexModel, Effort: "low", Brief: "find the config loader"}, skip: codexModel == ""},
	}
	var out []selfTestResult
	for _, c := range cases {
		if c.skip {
			continue
		}
		v := decideSpawn(p, c.req)
		if v.Deny == c.wantDeny {
			out = append(out, selfTestResult{pass, c.name})
		} else {
			out = append(out, selfTestResult{fail, fmt.Sprintf("%s: got deny=%v (%s)", c.name, v.Deny, strings.Join(v.Codes, ","))})
		}
	}
	return out
}

// expectLabel words a self-test case by what the policy expects.
func expectLabel(what string, denied bool) string {
	if denied {
		return what + " is denied"
	}
	return what + " is allowed (the policy does not require it)"
}

func (a *app) checkStateDir(report func(string, string, string)) {
	dir := a.stateDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		report(fail, "state dir", fmt.Sprintf("%s: %v", dir, err))
		return
	}
	probe, err := os.CreateTemp(dir, ".doctor-*")
	if err != nil {
		report(fail, "state dir", fmt.Sprintf("%s is not writable: %v", dir, err))
		return
	}
	_ = probe.Close()
	_ = os.Remove(probe.Name())
	report(pass, "state dir", dir)
}

// checkLedger confirms the measurement channel exists and is alive, and
// surfaces recent faults the hook failed open on.
func (a *app) checkLedger(onlyHost string, report func(string, string, string)) {
	path := a.ledgerPath()
	info, err := os.Stat(path)
	if err != nil {
		report(warn, "ledger", fmt.Sprintf("%s does not exist yet (no hook has fired)", path))
		return
	}
	var last time.Time
	faults, lastFault := 0, ""
	cutoff := a.now().Add(-ledgerStaleAfter)
	scanLedger(path, func(r ledgerRecord, ts time.Time) {
		if onlyHost != "" && r.Host != onlyHost {
			return // liveness is per host: one host's records prove nothing about the other
		}
		if ts.After(last) {
			last = ts
		}
		if r.Kind == kindFault && ts.After(cutoff) {
			faults++
			lastFault = r.Fault
		}
	})
	switch {
	case last.IsZero():
		report(warn, "ledger", fmt.Sprintf("%s has no readable %srecords", path, map[bool]string{true: onlyHost + " ", false: ""}[onlyHost != ""]))
	case last.Before(cutoff):
		report(warn, "ledger", fmt.Sprintf("newest record is %s old: are the hooks still firing?", a.now().Sub(last).Round(time.Hour)))
	default:
		report(pass, "ledger", fmt.Sprintf("%s (%d KiB, newest record %s ago)", path, info.Size()/1024, a.now().Sub(last).Round(time.Second)))
	}
	if faults > 0 {
		report(warn, "faults (7d)", fmt.Sprintf("%d, latest: %s", faults, lastFault))
	}
}

// hookEntry is one handler found in a host hook configuration.
type hookEntry struct {
	Event, Matcher, Command string
	Group, Handler          int
}

// hostHookConfig reads the hook entries a host would load from its
// user-level configuration.
func (a *app) hostHookConfig(host string) (path string, entries []hookEntry, err error) {
	if host == hostCodex {
		path = filepath.Join(a.codexHome(), "hooks.json")
	} else {
		path = filepath.Join(a.claudeHome(), "settings.json")
	}
	raw, err := readCapped(path, 4<<20)
	if err != nil {
		return path, nil, err
	}
	var doc struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return path, nil, err
	}
	for event, groups := range doc.Hooks {
		for gi, g := range groups {
			for hi, h := range g.Hooks {
				entries = append(entries, hookEntry{Event: event, Matcher: g.Matcher, Command: h.Command, Group: gi, Handler: hi})
			}
		}
	}
	return path, entries, nil
}

// checkWiring confirms the host loads this pack's hooks, flags legacy
// spawn gates that would decide the same calls twice, and (Codex) whether
// each handler has been trusted.
func (a *app) checkWiring(host string, report func(string, string, string)) {
	path, entries, err := a.hostHookConfig(host)
	check := "wiring " + host
	if err != nil {
		if isNotExist(err) {
			report(warn, check, fmt.Sprintf("%s not found (host not installed or not configured)", path))
		} else {
			report(fail, check, fmt.Sprintf("%s: %v", path, err))
		}
		return
	}
	spawnTool := "Agent"
	if host == hostCodex {
		spawnTool = codexSpawnTools[len(codexSpawnTools)-1] // the name Codex actually sends
	}
	ours := func(e hookEntry) bool { return runsAgentHooks(e.Command, host) }
	required := []struct{ event, tool string }{
		{eventPreToolUse, spawnTool}, {eventPostToolUse, spawnTool}, {eventPostToolUse, "Bash"}, {eventSubagentStop, ""},
	}
	var found []hookEntry
	for _, r := range required {
		var hit *hookEntry
		for i, e := range entries {
			if e.Event == r.event && ours(e) && matcherCovers(e.Matcher, r.tool) {
				hit = &entries[i]
				break
			}
		}
		label := r.event
		if r.tool != "" {
			label += " " + r.tool
		}
		if hit == nil {
			report(fail, check, fmt.Sprintf("%s: no agent-hooks handler in %s (agent-hooks print-hooks --host %s)", label, path, host))
			continue
		}
		found = append(found, *hit)
		report(pass, check, label)
	}

	for _, e := range entries {
		if e.Event == eventPreToolUse && !ours(e) && e.Matcher != "" && e.Matcher != "*" &&
			matcherCovers(e.Matcher, spawnTool) {
			report(warn, check, fmt.Sprintf("another PreToolUse %s gate is registered (%s): two gates decide each spawn", spawnTool, e.Command))
		}
	}

	if host == hostClaude && a.claudeHooksDisabled() {
		report(fail, check, "settings.json sets disableAllHooks: no hook runs, this pack included")
	}
	seen := map[string]bool{}
	for _, e := range found {
		if bin := firstField(e.Command); !seen[bin] {
			seen[bin] = true
			a.checkBinary(e.Command, check, report)
		}
	}
	if host == hostCodex {
		a.checkCodexTrust(path, found, report)
	}
}

// runsAgentHooks reports whether a hook command runs this pack for host, in
// either flag form.
func runsAgentHooks(command, host string) bool {
	return strings.Contains(command, "agent-hooks") &&
		(strings.Contains(command, "--host "+host) || strings.Contains(command, "--host="+host))
}

// claudeHooksDisabled reports whether Claude Code's user settings switch every
// hook off.
func (a *app) claudeHooksDisabled() bool {
	raw, err := readCapped(filepath.Join(a.claudeHome(), "settings.json"), 4<<20)
	if err != nil {
		return false
	}
	var s struct {
		DisableAllHooks bool `json:"disableAllHooks"`
	}
	return json.Unmarshal(raw, &s) == nil && s.DisableAllHooks
}

// checkBinary confirms the registered command points at an executable, and
// warns when it is not the binary running doctor (a stale copy).
func (a *app) checkBinary(command, check string, report func(string, string, string)) {
	bin := a.expandHome(firstField(command))
	info, err := os.Stat(bin)
	switch {
	case err != nil:
		report(fail, check, fmt.Sprintf("registered binary %s: %v", bin, err))
	case info.Mode()&0o111 == 0:
		report(fail, check, fmt.Sprintf("registered binary %s is not executable", bin))
	default:
		if self := selfPath(); self != "" {
			if resolved, err := filepath.EvalSymlinks(bin); err == nil && resolved != self {
				report(warn, check, fmt.Sprintf("hooks run %s but doctor is %s: is one of them stale?", resolved, self))
				return
			}
		}
		report(pass, check, "binary "+bin)
	}
}

// checkCodexTrust looks for each handler's trust entry in config.toml. Codex
// skips untrusted hooks until they are approved in /hooks; the hash itself
// is Codex's to compute, so doctor checks presence only.
func (a *app) checkCodexTrust(hooksPath string, found []hookEntry, report func(string, string, string)) {
	raw, err := readCapped(filepath.Join(a.codexHome(), "config.toml"), 4<<20)
	if err != nil {
		report(warn, "trust codex", fmt.Sprintf("config.toml unreadable: %v", err))
		return
	}
	type handlerID struct {
		event          string
		group, handler int
	}
	reported := map[handlerID]bool{}
	for _, e := range found {
		// One handler can satisfy several requirements; approve it once.
		id := handlerID{e.Event, e.Group, e.Handler}
		if reported[id] {
			continue
		}
		reported[id] = true
		key := fmt.Sprintf("%s:%s:%d:%d", hooksPath, snakeEvent(e.Event), e.Group, e.Handler)
		if bytes.Contains(raw, []byte(`[hooks.state."`+key+`"]`)) {
			report(pass, "trust codex", e.Event+" has a trust entry (Codex checks its hash; doctor cannot)")
		} else {
			report(warn, "trust codex", fmt.Sprintf("%s handler not trusted yet: approve it in Codex with /hooks", e.Event))
		}
	}
}

// checkAgents compares installed agent definitions with the shipped ones.
func (a *app) checkAgents(host string, report func(string, string, string)) {
	for _, ag := range agentAssets() {
		if ag.Host != host {
			continue
		}
		path := a.agentInstallPath(ag)
		installed, err := os.ReadFile(path)
		switch {
		case err != nil:
			report(warn, "agent "+host, fmt.Sprintf("%s not installed (%s)", ag.Name, path))
		case !bytes.Equal(installed, ag.Content):
			report(warn, "agent "+host, fmt.Sprintf("%s differs from the shipped definition", path))
		default:
			report(pass, "agent "+host, ag.Name)
		}
	}
}

// matcherCovers reports whether a hook matcher selects the tool. An empty
// matcher or "*" selects everything; otherwise the matcher is an anchored
// regular expression. An empty tool means the event has no tool.
func matcherCovers(matcher, tool string) bool {
	if tool == "" || matcher == "" || matcher == "*" {
		return true
	}
	re, err := regexp.Compile("^(?:" + matcher + ")$")
	return err == nil && re.MatchString(tool)
}

// snakeEvent converts PreToolUse to pre_tool_use, Codex's trust-key form.
func snakeEvent(event string) string {
	var b strings.Builder
	for i, r := range event {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// expandHome resolves a leading ~/, $HOME/ or ${HOME}/ the way the host's
// shell will when it runs the hook command, so a portable registration
// ("$HOME/.local/bin/agent-hooks") is checked at the right path.
func (a *app) expandHome(path string) string {
	for _, prefix := range []string{"~/", "$HOME/", "${HOME}/"} {
		if strings.HasPrefix(path, prefix) {
			return filepath.Join(a.home, path[len(prefix):])
		}
	}
	return path
}

// firstField returns the command's program: the first POSIX shell word,
// with quotes and backslash escapes resolved, so it round-trips commandWord
// for any path.
func firstField(command string) string {
	if w := shellWords(command, 1); len(w) > 0 {
		return w[0]
	}
	return ""
}

// scanLedger streams one ledger file, calling fn for each well-formed record.
// Malformed and oversized lines are skipped whole, so a torn or corrupt
// record never hides the records after it.
func scanLedger(path string, fn func(ledgerRecord, time.Time)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	_ = eachLine(f, 1<<20, func(b []byte) {
		var r ledgerRecord
		if json.Unmarshal(b, &r) != nil {
			return
		}
		ts, err := time.Parse(time.RFC3339Nano, r.TS)
		if err != nil {
			return
		}
		fn(r, ts)
	})
}
