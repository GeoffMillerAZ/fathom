package main

import (
	"strconv"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// verbHelpStatic is the flag list help prints for each verb or subverb whose
// handler does work before fs.Parse, so help cannot hand it -h (O671). Each
// entry says why. TestVerbHelpStaticListsPinned holds every list to the
// flags its handler defines (names, and usage and default wherever the
// source spells them as literals); a flag added to one of these handlers
// fails it until the list gains it. A fresh map per call (pkg-map rule).
func verbHelpStatic() map[string][]helpFlag {
	url := defaultFathomURL()
	return map[string][]helpFlag{
		// Kill switch first (FATHOM_CHANNELS=off exits before any flag is
		// read), then the channel identity, then the FlagSet.
		"edit-hook": {
			{"harness", "string", "", harnessFlagUsage},
			{"pre", "bool", "", "run as the PreToolUse guard hook (docs/design/guarded-edits.md) instead of the default PostToolUse obligations hook"},
			{"url", "string", url, "fathom serve-http base URL"},
		},
		"prompt-hook": {
			{"harness", "string", "", harnessFlagUsage},
			{"url", "string", url, "fathom serve-http base URL"},
		},
		"stop-hook": {
			{"block", "bool", "", "block session end on disproven claims (advisory otherwise)"},
			{"harness", "string", "", harnessFlagUsage},
			{"url", "string", url, "fathom serve-http base URL"},
		},
		"session-end-hook": {
			{"harness", "string", "", harnessFlagUsage},
		},
		"pre-compact-hook": {
			{"harness", "string", "", harnessFlagUsage},
		},
		// Kill switch first, then the instruction-event route and its beat.
		"session-hook": {
			{"dir", "string", "", "project dir (default: $CLAUDE_PROJECT_DIR, else cwd)"},
			{"harness", "string", "", harnessFlagUsage},
			{"url", "string", url, "fathom serve-http base URL"},
		},
		// RT-3 kill switch first, then config.toml, whose [daemon] table
		// seeds the defaults (serveRun.configure).
		"serve-http": {
			{"addr", "string", "[daemon].addr in config.toml, else the loopback default", "listen address (host:port); loopback by default — surfaces are not a security boundary (ADR-014 amendment)"},
			{"allow-operator-state", "bool", "", "unmanaged only: allow --store/--call-log to name the managed daemon's store or call log (RT-22)"},
			{"auto-refresh", "bool", "[daemon].auto_refresh, else true; an unmanaged daemon leaves it off unless set", "re-ingest a service when its git HEAD drifts from the indexed commit, before answering queries"},
			{"call-log", "string", "[daemon].call_log, else " + defaultCallLogPath(), "JSONL tool-call log path, resolved against the daemon working directory; empty to disable"},
			{"exit-with-parent", "bool", "true", "unmanaged only: exit within 10 s of the parent process's death, SIGKILL included (RT-20)"},
			{"idle-exit", "duration", tenancy.DefaultUnmanagedBounds().IdleExit.String(), "unmanaged only: exit after this long with no request in flight; 0 disables (RT-21)"},
			{"lifecycle-id", "string", "", "unmanaged only: keep the lifecycle ledger (and the crash-loop breaker) under ~/.fathom/daemon/<id>/ across relaunches — the stress harness's (RT-158); never a managed label. Default: a per-process addr-<port>-<pid> ledger"},
			{"managed", "bool", "", "this is the operator's managed (launchd/systemd-supervised) daemon (§10.1; plist schema 2 passes it): it obeys ~/.fathom/daemon/disabled.json and paused.json ('fathom daemon stop|pause') and is the only daemon that watches its own binary for self-restart. A launchd job under the default label or an installed fathom agent's Label is detected as managed without it. Unmanaged daemons are bounded: --exit-with-parent, --idle-exit, --ttl, no auto-refresh by default, switch files ignored, and the managed daemon's store and call log refused"},
			{"module", "var", "", "path to a conforming expert-module store (.sqlite/.db, docs/spec/expert-module-v0.md), mounted READ-ONLY so obligations unions in the rules of every module a service binds in .fathom/modules.cue; repeatable. A non-conforming or unreadable store aborts startup loudly (docs/plans/gke-module-host.md GKE-MH-1) rather than starting up silently degraded."},
			{"store", "string", "[daemon].store in config.toml", "store path, .sqlite/.db (required; [daemon].store in config.toml, or --store)"},
			{"ttl", "duration", tenancy.DefaultUnmanagedBounds().TTL.String(), "unmanaged only: exit after this long regardless; 0 disables (RT-21)"},
		},
		// config.toml and the decision ports open before the FlagSet
		// (runCertifySuggest parses).
		"certify suggest": {
			{"limit", "int", "", "cap the number of unregistered Test* symbols considered (0 = no cap)"},
			{"mutation", "string", "", "JF-9: a mutation-testing report (go-mutesting output or fathom's JSON shape); with a killed mutant in it, mutation-killed is offered as a strength (falls back to $" + mutationReportEnvVar + ")"},
			{"out", "string", workspacePath("artifacts/certify-suggestions.jsonl"), "suggestions artifact path (regenerated fresh each run, like certify manifest — never appended)"},
			{"service", "string", "", "service ID (default: resolved from the repo directory name)"},
			{"store", "string", defaultStorePath(), "store path (.sqlite/.db)"},
		},
		// A required leading positional (the plan file, the row id) is read
		// before Parse, which sees only what follows it.
		"plans context": {
			{"budget", "int", strconv.Itoa(defaultContextBudget), "byte budget for the rendered pack (elided beyond this)"},
			{"preflight", "bool", "", "also print the row's task preflight: difficulty profile and advisory tier recommendation (WI-16, WI-17; needs --store)"},
			{"row", "string", "", "row id to render (required)"},
			{"score", "bool", "", "with --preflight: also ask the decision port for its Score (WI-18; only with [decide] configured and the preflight site allowed)"},
			{"service", "string", "", "service ID (required)"},
			{"store", "string", "", "store path, .sqlite/.db (direct mode; bypasses daemon)"},
			{"url", "string", url, "fathom serve-http base URL (HTTP mode)"},
		},
		"plans lease": {
			{"holder", "string", "", "lease holder (defaults to $CLAUDE_SESSION_ID; required otherwise)"},
			{"plan", "string", "", "plan file this row belongs to (required)"},
			{"release", "bool", "", "release this row's lease instead of filing one"},
			{"root", "string", ".", "project root — .fathom/leases.jsonl lives under here"},
			{"ttl", "duration", (2 * time.Hour).String(), "lease time-to-live"},
		},
		"plans outcome": {
			{"effort", "string", "", "the effort setting it ran at"},
			{"first-attempt", "string", "", "true or false; default: read from the proof history for a proven row"},
			{"harness", "string", "", "the harness that worked the row (an agent identity, never a person)"},
			{"ledger", "string", preflightLedgerPath(), "outcome ledger path"},
			{"model", "string", "", "the model that worked the row"},
			{"result", "string", "", "proven or unproven (required)"},
			{"row", "string", "", "row id the outcome is for (required)"},
			{"score-provider", "string", "", "the provider that gave that Score"},
			{"score-tier", "string", "", "the tier the decision port's Score gave, when it was asked"},
			{"service", "string", "", "service ID (required)"},
			{"tier", "string", "", "the tier the rule table recommended (light, standard, heavy)"},
		},
		// The single target positional is extracted before Parse.
		"refactor plan": {
			{"out", "string", "", "output file for the generated plan (default: stdout)"},
			{"service", "string", "", "service ID (default: resolved from the repo directory name)"},
			{"store", "string", defaultStorePath(), "store path (.sqlite/.db)"},
		},
	}
}
