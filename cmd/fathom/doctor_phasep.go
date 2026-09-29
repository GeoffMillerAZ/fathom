package main

// doctor_phasep.go — phase-P doctor checks (production tokenomics +
// scoped config, docs/design/production-tokenomics.md /
// docs/design/scoped-config.md). House honesty discipline (doctor.go's own
// doc comment): an uncheckable state is `skipped` with a reason, never a
// guessed pass; a real, observable condition worth attention but not a
// wiring failure is `warn`, never `fail`.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
)

// checkChannelsEnv reports whether FATHOM_CHANNELS=off is set in DOCTOR's
// OWN process environment — the honesty half of the kill switch
// (channels.go): a silent kill switch nobody can detect repeats the O15
// failure mode, so doctor names it loudly whenever IT finds hooks
// disabled by environment. doctor itself is never silenced by
// FATHOM_CHANNELS — it is a diagnostic, not a hook.
//
// Critic finding F10 (phase-P fix round): the original wording ("hooks
// run normally") overclaimed — hooks run in the HARNESS's own environment
// (Claude Code's settings.json `env` block, a launchd plist's environment
// for the daemon), which doctor cannot see and may differ from doctor's
// own shell. The pass case now says exactly what was checked, no more.
func checkChannelsEnv() doctorCheck {
	const name = "channels_env"
	if st, active := loadArmState(time.Now()); active {
		// The measurement arm (IO-14) — deliberately loud, and named as an
		// ARM, not as the env kill switch: the whole point of the stateful
		// form is that doctor can say who set it, its bound, and the undo.
		bound := st.Until
		if st.SessionsRemaining != nil {
			bound += fmt.Sprintf(" / %d session(s) remaining", *st.SessionsRemaining)
		}
		return doctorCheck{
			Name:   name,
			Status: doctorWarn,
			Reason: fmt.Sprintf("measurement OFF arm ACTIVE (set %s by fathom %s%s): every channel is silent by design — this is counterfactual data collection (vision 14 §2). Bound: %s.", st.SetAt, st.FathomVersion, noteSuffix(st.Note), bound),
			Remedy: "let it expire, or end it early with `fathom arm on`",
		}
	}
	if fathomChannelsDisabled() {
		return doctorCheck{
			Name:   name,
			Status: doctorWarn,
			Reason: "FATHOM_CHANNELS=off is set in doctor's own environment — every fathom hook (session-hook, edit-hook pre/post, the discovery observer, stop-hook) exits in total silence WHEN RUN WITH THIS SAME ENVIRONMENT. Hooks run in the harness's own environment (Claude Code's settings.json, a launchd plist), which may differ from doctor's — check there too if hooks seem live despite this warning.",
			Remedy: `unset FATHOM_CHANNELS (or set it to anything other than "off") in doctor's environment; also check the harness's own hook environment (settings.json / plist), which doctor cannot see`,
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: `FATHOM_CHANNELS is not "off" in doctor's own environment (hooks run in the harness's own environment, which doctor cannot see and may differ)`}
}

// checkIdentCacheEnv is checkChannelsEnv's twin for the --no-cache escape
// (identcache.go, docs/design/production-tokenomics.md §7): honest
// disclosure when the service-identity cache is disabled by environment,
// never silent.
func checkIdentCacheEnv() doctorCheck {
	const name = "ident_cache_env"
	if identCacheDisabled() {
		return doctorCheck{
			Name:   name,
			Status: doctorWarn,
			Reason: "FATHOM_IDENT_NO_CACHE is set — every hook re-resolves service identity via the daemon on every call (higher latency/load, never a correctness issue)",
			Remedy: "unset FATHOM_IDENT_NO_CACHE to restore identity caching",
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("service-identity caching is active (TTL %s)", identCacheTTL)}
}

// userScopeIgnoreWalkCap bounds checkUserScopeIgnore's filesystem walk — a
// doctor check must stay fast even on a very large repo; this is a
// diagnostic sample, not an exhaustive audit.
const userScopeIgnoreWalkCap = 20000

// checkUserScopeIgnore is scoped-config's disclosure check
// (docs/design/scoped-config.md §4): names ~/.fathom/ignore when it is in
// effect for root, and — the "why can't fathom see my code" pre-empt —
// warns when a USER-scope pattern, specifically, is what excludes an
// otherwise-indexable path (a file that would be indexed under
// project-scope-only exclusions, but is hidden once the user-scope
// defaults are unioned in). A malformed user-scope file (e.g. a re-include
// violation) is reported as its own warn, naming the file, rather than
// silently degrading the way the read-time LoadIgnore convenience does.
func checkUserScopeIgnore(root string) doctorCheck {
	const name = "user_scope_ignore"
	userPath := connector.UserIgnorePath()
	if userPath == "" {
		return doctorCheck{Name: name, Status: doctorPass, Reason: "no resolvable $HOME — no user-scope ignore file to check"}
	}
	if _, err := os.Stat(userPath); err != nil {
		return doctorCheck{Name: name, Status: doctorPass, Reason: "no ~/.fathom/ignore configured"}
	}

	union, counts, err := connector.LoadIgnoreScoped(root)
	if err != nil {
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("~/.fathom/ignore is invalid: %v", err),
			Remedy: "fix the reported line, or see docs/design/scoped-config.md §2 (user scope may only ADD exclusions, never re-include)",
		}
	}
	projectOnly := connector.ProjectOnlyIgnore(root)

	hidden := walkForUserHiddenFiles(root, projectOnly, union)
	if len(hidden) > 0 {
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("~/.fathom/ignore (%d pattern(s)) is excluding %d otherwise-indexable file(s) from this repo, e.g. %q — this may be why fathom can't see some of your code", counts.User, len(hidden), hidden[0]),
			Remedy: fmt.Sprintf("review %s, or add a project-scope override in .fathomignore (project scope always wins — docs/design/scoped-config.md law 13)", userPath),
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("~/.fathom/ignore in effect (%d pattern(s)); excluding %d pattern(s) total (%d project, %d user-scope)", counts.User, counts.Project+counts.User, counts.Project, counts.User)}
}

// walkForUserHiddenFiles samples root (capped at userScopeIgnoreWalkCap
// entries) for regular files excluded by union but NOT by projectOnly —
// i.e. excluded specifically because of the user-scope union, never
// because of the project's own .fathomignore. Returns up to a handful of
// example relative paths (sorted, first-found order from a deterministic
// walk), nil when none found or the walk cannot proceed at all.
func walkForUserHiddenFiles(root string, projectOnly, union *filter.Matcher) []string {
	if union == nil {
		return nil
	}
	var hidden []string
	visited := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // best-effort diagnostic walk; a single unreadable entry must not abort the check
		}
		if visited >= userScopeIgnoreWalkCap || len(hidden) >= 5 {
			return filepath.SkipAll
		}
		visited++
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if connector.IsSkippedDir(d.Name()) {
				return filepath.SkipDir
			}
			if projectOnly != nil && projectOnly.Match(rel+"/") {
				return filepath.SkipDir // project already excludes this subtree — not the user-scope's doing
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if projectOnly != nil && projectOnly.Match(rel) {
			return nil // project already excludes this file
		}
		if union.Match(rel) {
			hidden = append(hidden, rel)
		}
		return nil
	})
	return hidden
}

// checkUserScopeGuard is checkUserScopeIgnore's twin for
// ~/.fathom/guard.cue (docs/design/scoped-config.md deliverable 2/§4):
// names the file when in effect, loud on a governance-field violation
// (LoadUserGuards refuses it — see guardspec.LoadUserGuards's doc
// comment), pass on absence (the common case).
func checkUserScopeGuard() doctorCheck {
	const name = "user_scope_guard"
	path := fathomhome.Path("guard.cue")
	if path == "" {
		return doctorCheck{Name: name, Status: doctorPass, Reason: "no resolvable $HOME — no user-scope guard file to check"}
	}
	if _, err := os.Stat(path); err != nil {
		return doctorCheck{Name: name, Status: doctorPass, Reason: "no ~/.fathom/guard.cue configured"}
	}
	gs, err := guardspec.LoadUserGuards(path)
	if err != nil {
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("~/.fathom/guard.cue is invalid: %v", err),
			Remedy: "fix the reported field — user scope is restricted to note/command_scopes (docs/design/scoped-config.md law 13)",
		}
	}
	if len(gs.Guards) == 0 {
		// Present but declares nothing (O22, mirrors checkGuardFile's
		// project-scope twin): a hand-copied, all-commented guard.cue
		// loads clean with zero guards — distinct from both "no file"
		// above and "invalid" below.
		return doctorCheck{Name: name, Status: doctorPass, Reason: "~/.fathom/guard.cue present but declares no guards yet — scaffold awaiting opt-in"}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("~/.fathom/guard.cue in effect — %d command guard(s)", len(gs.Guards))}
}

// sessionLedgerConsecutiveUnavailableThreshold is how many consecutive
// recent sessions all reporting tokens:unavailable trips checkSessionLedgerTokens's
// warn — docs/design/production-tokenomics.md §8's amended non-goal, guard
// 3: the transcript-schema-coupled instrument (§6) must degrade LOUDLY,
// not silently, since the A/B protocol (§5) depends on it.
const sessionLedgerConsecutiveUnavailableThreshold = 5

// checkSessionLedgerTokens is docs/design/production-tokenomics.md §8's
// amended non-goal, guard 3 (critic finding F4, phase-P fix round): reads
// the tail of the session ledger and warns when the most recent
// sessionLedgerConsecutiveUnavailableThreshold consecutive sessions ALL
// report tokens:unavailable — a sign Claude Code's transcript schema has
// drifted out from under transcriptSummaryFrom's parsing, which would
// otherwise silently starve the A/B protocol's causal instrument.
// Absence/too-few-entries is honestly `pass` (nothing wrong observed yet),
// never a guessed warn.
func checkSessionLedgerTokens() doctorCheck {
	const name = "session_ledger_tokens"
	path := sessionLedgerPath()
	if path == "" {
		return doctorCheck{Name: name, Status: doctorPass, Reason: "no resolvable $HOME — no session ledger to check"}
	}
	f, err := os.Open(path)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorPass, Reason: "no session ledger yet (no Stop hook has fired)"}
	}
	defer f.Close()

	var entries []sessionLedgerEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e sessionLedgerEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue // a torn/malformed line must not abort the check
		}
		entries = append(entries, e)
	}
	if len(entries) < sessionLedgerConsecutiveUnavailableThreshold {
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("only %d session(s) logged — not enough to judge (threshold %d)", len(entries), sessionLedgerConsecutiveUnavailableThreshold)}
	}

	recent := entries[len(entries)-sessionLedgerConsecutiveUnavailableThreshold:]
	allUnavailable := true
	for _, e := range recent {
		if e.TokensTotal != nil {
			allUnavailable = false
			break
		}
	}
	if allUnavailable {
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("the last %d session(s) all report tokens:unavailable — the transcript-parsing instrument (docs/design/production-tokenomics.md §6/§8) may have silently degraded (a Claude Code transcript schema change is the likely cause)", sessionLedgerConsecutiveUnavailableThreshold),
			Remedy: "inspect a recent transcript_path file directly against transcriptSummaryFrom's expected shape (cmd/fathom/sessionledger.go) — this instrument feeds the A/B protocol (§5) and must not degrade quietly",
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("recent sessions include real token totals (checked last %d)", sessionLedgerConsecutiveUnavailableThreshold)}
}

// noteSuffix renders an arm's note for inline display.
func noteSuffix(note string) string {
	if note == "" {
		return ""
	}
	return ", note: " + note
}

// checkBinaryOnPath (IO-20): `fathom` must resolve from a NON-interactive
// shell — the 2026-07-31 journal case: `fathom verify …` printed
// command-not-found as one token and the pipeline still exited 0, a
// verification failure wearing a pass. Doctor checks resolvability in its
// own environment and says exactly what that does and does not prove.
func checkBinaryOnPath() doctorCheck {
	const name = "binary_on_path"
	p, err := exec.LookPath("fathom")
	if err != nil {
		return doctorCheck{
			Name:   name,
			Status: doctorFail,
			Reason: "`fathom` does not resolve on doctor's own PATH — every stanza command and fail-open gate that spells `fathom` is silently dead in shells with this PATH",
			Remedy: "run `fathom init` (installs the ~/.local/bin/fathom shim → deployed binary), or add the shim by hand: ln -s ~/.local/libexec/fathom/fathom ~/.local/bin/fathom",
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: "`fathom` resolves to " + p + " (doctor's own PATH; non-interactive shells inherit their own — the shim in ~/.local/bin covers standard PATHs)"}
}
