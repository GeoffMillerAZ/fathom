package main

// verbs.go — the one table of dispatched subcommands (HF-6).
//
// `fathom help` renders from this table and nothing else, and
// TestHelpListsEveryDispatchedVerb parses main.go's dispatch switch and
// fails when a `case "x":` has no row here. That is the structural fix for
// the 2026-08-25 defect where six dispatched verbs (prime, review, fleet,
// the session hooks) worked but were invisible in help — an agent could
// only find them by reading the source.
//
// It is also the resolver the `## Maintained` doc contract needs (HF-7):
// a name in a use-case doc's Maintained line is legitimate iff it is a
// registered MCP tool or a verb in this table.

// verb is one dispatched subcommand and its one-line help summary.
type verb struct {
	Name    string
	Summary string
	// Hidden freezes a verb out of `fathom help` (SR-16/SR-18, surface
	// reckoning F-53/F-70) while it stays fully dispatched and callable—
	// for a verb an ADR keeps alive on a closed allowlist (research,
	// ADR-046 decision 6) or behind an off-by-default capability (assess,
	// ADR-046/ADR-053), not one anyone should discover by browsing help.
	// TestHelpListsEveryDispatchedVerb still requires it dispatched AND in
	// this table; only printUsage's rendering skips it.
	Hidden bool
}

// verbTable is every subcommand `main` dispatches, in help order.
var verbTable = []verb{
	{Name: "ingest", Summary: "Ingest a service into the knowledge store"},
	{Name: "ingest-diff", Summary: "Ingest only files changed since a git ref"},
	{Name: "watch", Summary: "Poll for file changes and re-ingest automatically"},
	{Name: "gen-claude-md", Summary: "Generate a CLAUDE.md from stored symbols"},
	{Name: "gen-status-cue", Summary: "Generate a CUE status file from a plan + evidence", Hidden: true}, // SR-30: internal dev/round-trip tooling (ADR-018 decision 3), not agent-facing
	{Name: "gen-platform-manifest", Summary: "Generate a platform manifest from *.sqlite/*.db store files"},
	{Name: "inspect", Summary: "List stored symbols for a service"},
	{Name: "query", Summary: "Query symbols for a concern"},
	{Name: "serve-mcp", Summary: "Serve the MCP JSON-RPC protocol over stdio"},
	{Name: "serve-http", Summary: "Serve the MCP JSON-RPC protocol over HTTP (POST /rpc)"},
	{Name: "theme-guide", Summary: "Opt-in loopback theme review using synthetic Observatory renderers; no live data or daemon"},
	{Name: "platform-ingest", Summary: "Ingest all services under a root directory"},
	{Name: "platform-callers", Summary: "Print cross-repo callers of a symbol"},
	{Name: "verify", Summary: "Verify plan files against the index (done-means-proven gate)"},
	{Name: "plans", Summary: "Computed SDD progress: status | next [--eligible-only] | history --plan P | context --row ID | lease | leases | archive | restore"},
	{Name: "remap", Summary: "Point a service's recorded corpus root at a new location"},
	{Name: "compile", Summary: "Compile a semantic layer for a service (dry-run by default)"},
	{Name: "claim", Summary: "Triage a compiled claim: refute or corroborate, with a note | show <id> [--json]: statement, citation excerpt, scope, tier, status and verdict history"},
	{Name: "layers", Summary: "List/show/install semantic-layer packs (embedded + project .fathom/layers/) | strata: the four strata with claims, lifecycle, runs and cost | coverage: claims per project and layer, missing as not compiled | show <name> --json: compile runs, rejections and uncompiled scopes"},
	{Name: "obligations", Summary: "What governs a file, as served to an agent: --service S [--file F] [--layer rules|claims|all] | --explain: every candidate by scope tier, the ranking, the cap, and why each displaced rule lost"},
	{Name: "report", Summary: "Write an analyzer's output as a markdown finding document (deterministic bridge) | report evidence [--service S] [--store DB] [--json]: per-service typed edge share, certificate density, inventory item counts by kind, and whether receipts re-derive (Go only) — ranks no languages (ADR-044 §5) | report packs [--json]: dialect packs as a language × capability matrix (measured precision only where a reading exists) and layer packs with where each is active"},
	{Name: "review", Summary: "Store-direct review of files changed since a git ref (arch conformance, spec coverage); --mode report|persist (--no-persist = report only), --manifest FILE, --idempotency-key, --expect-index-watermark, --budget-seconds = advisory bound"},
	{Name: "review-key", Summary: "One sha256 review key over manifest + typed dependency closure + rules + plans + tooling + index identity (--since REF | --manifest FILE) [--prior FILE]; complete:false + unknown[] name any input it could not verify"},
	{Name: "conserve", Summary: "conserve --since <ref> [--json] [--strict] [--service S] [--store DB]: ADR-044's conservation report — every registered plane's verdict (conserved/losses/unproven) over the capability inventory, certificates and governance, plus the edge tier per language touched; --strict exits 1 on any plane with verdict losses"},
	{Name: "refactor", Summary: "refactor plan <symbol|package|import> [--out FILE] [--service S] [--store DB]: writes a gate-ready grammar-v2 plan (Baseline header, one ratcheted gone: row per referencing package, certified_by rows, the final absence row, a conserve instruction) | refactor status --plan FILE [--json]: each ratchet's live count vs its max and vs the baseline, plus drift landed on the branch since baseline (ADR-044 §4)"},
	{Name: "tests", Summary: "tests coverage --profile <file>: per-symbol Go statement coverage by test layer (--mutation <report>: adds a mutation score column); --diff <base> narrows to changed symbols with no covering test | tests select --since <ref>: ranks Test* functions for a diff by reachability/coverage/layer with a test that already certifies a changed-file target promoted first, emits go test invocations grouped per package plus the test gap | tests ingest --manifest <dir> --results <file|dir>: loads declared tests/*.cue manifests + attested #TestResult envelopes into additive test evidence tables, joining cases to Test* symbols and scanning `// certifies:` markers (vision 33 / ADR-043 stage 0) | tests status: per suite, class/tier/last-run-verdict/totals and the case-to-symbol join rate"},
	{Name: "certify", Summary: "certify status: declared/live/gap counts and the join rate for the compiled certifications layer | certify gaps [--kind invariant|incident|spec|port|rule|behaviour] [--json]: every declared target, invariant row and incident record with no live certificate, each with a remedy line (plus any decision-port suggestion on the table path); --json emits the versioned fathom.certify.gaps/v1 shape the Review Kit adapter parses | certify manifest [--out FILE] [--check]: regenerates docs/certifications.md from the layer, --check fails when the committed file differs | certify orphans [--json]: Test* symbols that certify nothing, reach no symbol changed in 180 days, and never failed/flaked — a list, never an action | certify suggest [--limit N] [--out FILE]: one batched decision-port request per chunk of unregistered Test* symbols (strength choice + asserts noul); proceed-band answers become suggestions in workspace/artifacts/certify-suggestions.jsonl, never a claim; provider none prints one line and exits 0 (vision 33 / ADR-043)"},
	{Name: "session-hook", Summary: "SessionStart hook: resolve this repo's service and emit a one-line reminder (or an honest not-indexed line); always exits 0"},
	{Name: "prime", Summary: "Print the SessionStart context as plain text — the pull tier for a harness with no hook (Codex); always exits 0 | --routes [package|concern]: long-lived discovery paths (pointers established in 2+ sessions over 30 days), grouped by package (SR-29)"},
	{Name: "discoveries", Summary: "Receipts on the hand-off: this repo's recent call-log pointers, resolved fresh against the live index (docs/vision/28-receipts.md §4)"},
	{Name: "edit-hook", Summary: "PostToolUse hook: inject obligations into session context"},
	{Name: "prompt-hook", Summary: "UserPromptSubmit hook: resolve the paths/packages/symbols the prompt names through the index and inject the rules that govern them, before the first edit (O57 items 4/5)"},
	{Name: "stop-hook", Summary: "Stop hook: flag tasks claimed [x] this session that fail verification"},
	{Name: "session-end-hook", Summary: "SessionEnd hook: deposit one continuity item only when the session leaves uncommitted work, stashes, or claimed-unproven rows"},
	{Name: "pre-compact-hook", Summary: "PreCompact hook: file one pointer to the pre-compaction span (the transcript itself is never read)"},
	{Name: "post-commit-hook", Summary: "post-commit hook: refresh this repo's index THROUGH THE DAEMON (never a shell ingest against the shared store — INC-3); always exits 0"},
	{Name: "intent", Summary: "The intent corpus (ADR-026): audit — conditions, invariant rungs, unproven claims | doctrine — the vision promotion pipeline and goals as pre-registered bets | export --guard"},
	{Name: "export", Summary: "export scip --service <s> [--store PATH] [-o index.scip]: write the symbol and typed-edge tables as a SCIP index (VB-32)"},
	{Name: "daemon", Summary: "The serve-http launchd agent: install (writes the plist with YOUR paths and starts it; --reconcile migrates an installed one) | status | stop (the kill switch) | start | pause | resume | uninstall"},
	{Name: "resources", Summary: "What the daemon takes from this machine, from files (works when it is down): budget, footprint history, lifecycle, quarantine, host, in-flight, hooks, scratch daemons, disk, last capture; exit 0 within budget, 1 over/crash-looping/stopped/leaked, 2 unreadable [--json] [--since 24h] [--id ID]"},
	{Name: "gc", Summary: "Disk retention by class (call log rotated and archived — never deleted; old session stamps, dead daemon records, stale test temp dirs, old scenario-bench worktrees and build binaries, diag captures past 10/200 MiB); dry run by default [--apply] [--json] [--workspace DIR] [--call-log PATH]"},
	{Name: "gen-plan-cue", Summary: "Emit the generated CUE shadow of a Markdown plan (round-trip verified)", Hidden: true},                            // SR-30: internal dev/round-trip tooling (ADR-018 decision 3), not agent-facing
	{Name: "instrument", Summary: "Channel-health report: injection coverage, rollup, liveness alarms (--adoption: the flywheel gauge)"},
	{Name: "observe", Summary: "File a structured friction observation into .fathom/observations.md"},
	{Name: "init", Summary: "One-command repo wiring: .fathomignore, ingest, Claude Code hooks, CLAUDE.md/AGENTS.md stanza"},
	{Name: "config", Summary: "Print the effective configuration (--explain for per-key provenance) | audit (custodian concerns) | propose (a diff for each mechanically mendable violation; writes nothing)"},
	{Name: "providers", Summary: "Auth-health probe across configured LLM profiles (zero/near-zero tokens)"},
	{Name: "smoke", Summary: "One tiny real completion per profile: model, latency, tokens, cost"},
	{Name: "doctor", Summary: "Deterministic wiring diagnosis: registration, daemon, hooks, stanza, skills, config, obligations, traffic"},
	{Name: "capabilities", Summary: "What is switched on here and WHY anything is off: list | explain <id> | set <id> on|off"},
	{Name: "flywheel", Summary: "Are the feedback loops closed? Each loop CLOSED/OPEN/IDLE with its evidence and remedy"},
	{Name: "rate", Summary: "Rate this session 1-5: the OUTCOME half of the A/B criterion (fewer tokens is not a win if you got less done)"},
	{Name: "arm", Summary: "A/B counterfactual arm (law 12): off --for D --sessions N [--note ...] | on"},
	{Name: "assess", Summary: "Priced, explicit, off by default: a model judges a unit's on-disk work (typed questions, never the transcript): [--session S] [--interventions] [--dry-run] [--json] | spend", Hidden: true}, // SR-18: frozen (ADR-046/ADR-053)
	{Name: "retro", Summary: "Did the firings help? Join a session's interventions to outcomes: [--session S | latest stamped] [--write] [--json]"},
	{Name: "thread", Summary: "Continuity threads: brief | note | point | list | close | promote | handoff — the work, not the session (vision 21)"},
	{Name: "fleet", Summary: "Fleet operations: report | reconcile [--apply] | realization --service S | receipts | undo <receipt> (every write receipted, every receipt reversible) | remedy: time-to-remedy per repository from the report snapshots"},
	{Name: "research", Summary: "The research flywheel: report (aggregate local evidence) | bundle (sanitized, shareable summary)", Hidden: true}, // SR-16: frozen (ADR-046 decision 6), not deleted
	{Name: "module", Summary: "Expert-module lifecycle: init | sources | map | compile | eval | doctor | pack | runs"},
	{Name: "rules", Summary: "Compiled governance (VB-13/VB-14): gen-test [--root DIR] [--out DIR] emits a Go test from .fathom/guard.cue's layer_rules | replay --since <ref|N> [--rules FILE] [--json]: evaluate today's rules against past commits (VB-16)"},
	{Name: "receipt", Summary: "A stranger's re-derivation (VB-20): verify [--repo DIR] <receipt.json> checks a saved answer against a bare git checkout with go/types and git alone — no daemon, no store"},
	{Name: "metrics", Summary: "Observability surfaces: spend | traffic | channels | interventions | series | rollup | rotate | impact (the four questions, arms as ranges) | catches (the catches timeline) | sessions (the session table, or one session's windowed replay) | load (calls per day per surface, tools by caller class)"},
	{Name: "findings", Summary: "Findings as evidence: near <symbol|file> (call-graph precedent) | status (entry-condition progress)"},
	{Name: "memory", Summary: "Anchored memory: add | list | review | show — a memory with no anchor is refused, and drift expires it"},
	{Name: "scope", Summary: "Reversible suppression: list | suppress <glob> --reason R | unsuppress <id> | preview <glob>"},
	{Name: "purge", Summary: "IRREVERSIBLE removal: purge <glob> --service S --reason R [--dry-run|--yes] | history (exit 2 = SQL zero but bytes unchecked)"},
	{Name: "plugin", Summary: "plugin render [--out DIR] [--mcp-url U] [--marketplace]: render one plugin tree carrying both the Claude Code and the Agent Plugins 1.0 manifests over one skills/ dir"},
	{Name: "decide", Summary: "The decision port (ADR-042): log [--site S] [--json] — the provider, its sites, the ruling log beside the call log and the latest calibration reading's table"},
	{Name: "harness", Summary: "The harness registry (ADR-029): list [--json] | capabilities [--harness ID] [--json] | matrix [--url U] [--json] — every vehicle per capability, unverified rows say so"},
	{Name: "support", Summary: "The support matrix (SM): [--kind language|tool|client|spec|rules|coverage|contract|ecosystem] [--json] — computed support level per kind, never a hand-kept catalog"},
	{Name: "version", Summary: "Print version information"},
	{Name: "help", Summary: "Show this help message"},
}

// verbNames returns every dispatched subcommand name, in help order. The
// `## Maintained` parser and the help renderer share it, so a verb cannot
// be dispatched-but-undocumented or documented-but-undispatched.
func verbNames() []string {
	names := make([]string, 0, len(verbTable))
	for _, v := range verbTable {
		names = append(names, v.Name)
	}
	return names
}

// isVerb reports whether name is a dispatched subcommand.
func isVerb(name string) bool {
	for _, v := range verbTable {
		if v.Name == name {
			return true
		}
	}
	return false
}
