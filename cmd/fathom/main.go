// Command fathom is the CLI entry point for the fathom knowledge-index tool.
//
// Subcommands: ingest, inspect, query, serve-mcp, serve-http, platform-ingest,
// version, help.
//
// The run(args, stdout, stderr) function is the testable core; main() is a
// one-liner that calls os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/geoffmilleraz/fathom/internal/scope"
)

func main() {
	// Arm the suppression read filter for the REAL binary: the store's
	// per-call gate reads state exclusively through $FATHOM_SUPPRESSIONS
	// (scope.EnvVar), and defaulting it HERE — never in a package init —
	// is what keeps `go test ./...` hermetic on every machine while the
	// CLI and the daemon always filter (the V6 rework: no init()-armed
	// globals, no gate result that depends on the developer's ~/.fathom).
	if os.Getenv(scope.EnvVar) == "" {
		if p := scope.DefaultUserPath(); p != "" {
			_ = os.Setenv(scope.EnvVar, p)
		}
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches to the appropriate subcommand handler.
// It returns 0 on success, 1 on any error.
func run(args []string, stdout, stderr io.Writer) int {
	return runWithStdin(args, os.Stdin, stdout, stderr)
}

// runWithStdin is the testable core of run; it accepts an explicit stdin
// reader so that subcommands that consume stdin (e.g. serve-mcp) can be
// driven by in-memory buffers in tests.
func runWithStdin(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: no subcommand given")
		printUsage(stderr)
		return 1
	}

	switch args[0] {
	case "version":
		return cmdVersion(args[1:], stdout, stderr)
	case "help":
		return cmdHelp(args[1:], stdout, stderr)
	case "ingest":
		return cmdIngest(args[1:], stdout, stderr)
	case "ingest-diff":
		return cmdIngestDiff(args[1:], stdout, stderr)
	case "watch":
		return cmdWatch(args[1:], stdout, stderr)
	case "gen-claude-md":
		return cmdGenClaudeMd(args[1:], stdout, stderr)
	case "gen-status-cue":
		return cmdGenStatusCue(args[1:], stdout, stderr)
	case "gen-platform-manifest":
		return cmdGenPlatformManifest(args[1:], stdout, stderr)
	case "session-hook":
		return cmdSessionHook(args[1:], stdin, stdout, stderr)
	case "prime":
		return cmdPrime(args[1:], stdout, stderr)
	case "query":
		return cmdQuery(args[1:], stdout, stderr)
	case "inspect":
		return cmdInspect(args[1:], stdout, stderr)
	case "serve-mcp":
		return runServeMCP(args[1:], stdin, stdout, stderr)
	case "serve-http":
		return runServeHTTP(args[1:], stdout, stderr)
	case "platform-ingest":
		return cmdPlatformIngest(args[1:], stdout, stderr)
	case "platform-callers":
		return cmdPlatformCallers(args[1:], stdout, stderr)
	case "verify":
		return cmdVerify(args[1:], stdout, stderr)
	case "plans":
		return cmdPlans(args[1:], stdout, stderr)
	case "remap":
		return cmdRemap(args[1:], stdout, stderr)
	case "compile":
		return cmdCompile(args[1:], stdout, stderr)
	case "claim":
		return cmdClaim(args[1:], stdout, stderr)
	case "layers":
		return cmdLayers(args[1:], stdout, stderr)
	case "report":
		return cmdReport(args[1:], stdout, stderr)
	case "review":
		return cmdReview(args[1:], stdout, stderr)
	case "edit-hook":
		return cmdEditHook(args[1:], stdout, stderr)
	case "instrument":
		return cmdInstrument(args[1:], stdout, stderr)
	case "observe":
		return cmdObserve(args[1:], stdout, stderr)
	case "arm":
		return cmdArm(args[1:], stdout, stderr)
	case "narrate":
		return cmdNarrate(args[1:], stdout, stderr)
	case "plugin":
		if len(args) > 1 && args[1] == "render" {
			return cmdPluginRender(args[2:], stdout, stderr)
		}
		fmt.Fprintln(stderr, "usage: fathom plugin render [--out DIR] [--mcp-url U] [--marketplace]")
		return 1
	case "brief":
		return cmdBrief(args[1:], stdout, stderr)
	case "capabilities":
		return cmdCapabilities(args[1:], stdout, stderr)
	case "flywheel":
		return cmdFlywheel(args[1:], stdout, stderr)
	case "rate":
		return cmdRate(args[1:], stdout, stderr)
	case "retro":
		return cmdRetro(args[1:], stdout, stderr)
	case "thread":
		return cmdThread(args[1:], stdout, stderr)
	case "session-end-hook":
		return cmdSessionEndHook(args[1:], stdout, stderr)
	case "pre-compact-hook":
		return cmdPreCompactHook(args[1:], stdout, stderr)
	case "fleet":
		if len(args) > 1 && args[1] == "report" {
			return cmdFleetReport(args[2:], stdout, stderr)
		}
		if len(args) > 1 && args[1] == "reconcile" {
			return cmdFleetReconcile(args[2:], stdout, stderr)
		}
		// Phase U (docs/design/self-healing-fleet.md §5): every write fathom
		// makes inside a repository is receipted, and every receipt is
		// reversible from the same terminal.
		if len(args) > 1 && args[1] == "receipts" {
			return cmdFleetReceipts(args[2:], stdout, stderr)
		}
		if len(args) > 1 && args[1] == "undo" {
			return cmdFleetUndo(args[2:], stdout, stderr)
		}
		fmt.Fprintln(stderr, "usage: fathom fleet report|reconcile|receipts|undo [flags]")
		return 1
	case "stop-hook":
		return cmdStopHook(args[1:], stdout, stderr)
	case "gen-plan-cue":
		return cmdGenPlanCue(args[1:], stdout, stderr)
	case "init":
		return cmdInit(args[1:], stdout, stderr)
	case "config":
		return cmdConfig(args[1:], stdout, stderr)
	case "providers":
		return cmdProviders(args[1:], stdout, stderr)
	case "smoke":
		return cmdSmoke(args[1:], stdout, stderr)
	case "doctor":
		return cmdDoctor(args[1:], stdout, stderr)
	case "research":
		return cmdResearch(args[1:], stdout, stderr)
	case "module":
		return cmdModule(args[1:], stdout, stderr)
	case "metrics":
		return cmdMetrics(args[1:], stdout, stderr)
	case "findings":
		return cmdFindings(args[1:], stdout, stderr)
	case "scope":
		return cmdScope(args[1:], stdout, stderr)
	case "purge":
		return cmdPurge(args[1:], stdout, stderr)
	case "memory":
		return cmdMemory(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown subcommand %q\n", args[0])
		printUsage(stderr)
		return 1
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: fathom <subcommand> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintln(w, "  ingest                Ingest a service into the knowledge store")
	fmt.Fprintln(w, "  ingest-diff           Ingest only files changed since a git ref")
	fmt.Fprintln(w, "  watch                 Poll for file changes and re-ingest automatically")
	fmt.Fprintln(w, "  gen-claude-md         Generate a CLAUDE.md from stored symbols")
	fmt.Fprintln(w, "  gen-status-cue        Generate a CUE status file from a plan + evidence")
	fmt.Fprintln(w, "  gen-platform-manifest Generate a platform manifest from *.sqlite/*.db store files")
	fmt.Fprintln(w, "  inspect               List stored symbols for a service")
	fmt.Fprintln(w, "  query                 Query symbols for a concern")
	fmt.Fprintln(w, "  serve-mcp             Serve the MCP JSON-RPC protocol over stdio")
	fmt.Fprintln(w, "  serve-http            Serve the MCP JSON-RPC protocol over HTTP (POST /rpc)")
	fmt.Fprintln(w, "  platform-ingest       Ingest all services under a root directory")
	fmt.Fprintln(w, "  platform-callers      Print cross-repo callers of a symbol")
	fmt.Fprintln(w, "  verify                Verify plan files against the index (done-means-proven gate)")
	fmt.Fprintln(w, "  plans                 Computed SDD progress: status | next [--eligible-only] | context --row ID | lease | leases | archive | restore")
	fmt.Fprintln(w, "  remap                 Point a service's recorded corpus root at a new location")
	fmt.Fprintln(w, "  compile               Compile a semantic layer for a service (dry-run by default)")
	fmt.Fprintln(w, "  claim                 Triage a compiled claim: refute or corroborate, with a note")
	fmt.Fprintln(w, "  layers                List/show/install semantic-layer packs (embedded + project .fathom/layers/)")
	fmt.Fprintln(w, "  report                Write an analyzer's output as a markdown finding document (deterministic bridge)")
	fmt.Fprintln(w, "  review                Store-direct review of files changed since a git ref (arch conformance, spec coverage); --no-persist = report only, --budget-seconds = advisory bound")
	fmt.Fprintln(w, "  session-hook          SessionStart hook: resolve this repo's service and emit a one-line reminder (or an honest not-indexed line); always exits 0")
	fmt.Fprintln(w, "  prime                 Print the SessionStart context as plain text — the pull tier for a harness with no hook (Codex); always exits 0")
	fmt.Fprintln(w, "  edit-hook             PostToolUse hook: inject obligations into session context")
	fmt.Fprintln(w, "  stop-hook             Stop hook: flag tasks claimed [x] this session that fail verification")
	fmt.Fprintln(w, "  session-end-hook      SessionEnd hook: deposit one continuity item only when the session leaves uncommitted work, stashes, or claimed-unproven rows")
	fmt.Fprintln(w, "  pre-compact-hook      PreCompact hook: file one pointer to the pre-compaction span (the transcript itself is never read)")
	fmt.Fprintln(w, "  gen-plan-cue          Emit the generated CUE shadow of a Markdown plan (round-trip verified)")
	fmt.Fprintln(w, "  instrument            Channel-health report: injection coverage, rollup, liveness alarms (--adoption: the flywheel gauge)")
	fmt.Fprintln(w, "  observe               File a structured friction observation into .fathom/observations.md")
	fmt.Fprintln(w, "  init                  One-command repo wiring: .fathomignore, ingest, Claude Code hooks, CLAUDE.md/AGENTS.md stanza")
	fmt.Fprintln(w, "  config                Print the effective configuration (--explain for per-key provenance) | audit (custodian concerns)")
	fmt.Fprintln(w, "  providers             Auth-health probe across configured LLM profiles (zero/near-zero tokens)")
	fmt.Fprintln(w, "  smoke                 One tiny real completion per profile: model, latency, tokens, cost")
	fmt.Fprintln(w, "  doctor                Deterministic wiring diagnosis: registration, daemon, hooks, stanza, skills, config, obligations, traffic")
	fmt.Fprintln(w, "  capabilities          What is switched on here and WHY anything is off: list | explain <id> | set <id> on|off")
	fmt.Fprintln(w, "  flywheel              Are the feedback loops closed? Each loop CLOSED/OPEN/IDLE with its evidence and remedy")
	fmt.Fprintln(w, "  rate                  Rate this session 1-5: the OUTCOME half of the A/B criterion (fewer tokens is not a win if you got less done)")
	fmt.Fprintln(w, "  arm                   A/B counterfactual arm (law 12): off --for D --sessions N [--note ...] | on")
	fmt.Fprintln(w, "  narrate               Self-expiring story mode: on --for D --sessions N [--note ...] | off")
	fmt.Fprintln(w, "  retro                 Did the firings help? Join a session's interventions to outcomes: [--session S | latest stamped] [--write] [--json]")
	fmt.Fprintln(w, "  brief                 Write the shareable markdown value brief (honesty covenant included) to ~/.fathom/briefs, or --print")
	fmt.Fprintln(w, "  thread                Continuity threads: brief | note | point | list | close | promote | handoff — the work, not the session (vision 21)")
	fmt.Fprintln(w, "  fleet                 Fleet operations: report | reconcile [--apply] | receipts | undo <receipt> (every write receipted, every receipt reversible)")
	fmt.Fprintln(w, "  research              The research flywheel: report (aggregate local evidence) | bundle (sanitized, shareable summary)")
	fmt.Fprintln(w, "  module                Expert-module lifecycle: init | sources | map | compile | eval | doctor | pack | runs")
	fmt.Fprintln(w, "  metrics               Observability surfaces: spend | traffic | channels | interventions | series | rollup | rotate")
	fmt.Fprintln(w, "  findings              Findings as evidence: near <symbol|file> (call-graph precedent) | status (entry-condition progress)")
	fmt.Fprintln(w, "  memory                Anchored memory: add | list | review | show — a memory with no anchor is refused, and drift expires it")
	fmt.Fprintln(w, "  scope                 Reversible suppression: list | suppress <glob> --reason R | unsuppress <id> | preview <glob>")
	fmt.Fprintln(w, "  purge                 IRREVERSIBLE removal: purge <glob> --service S --reason R [--dry-run|--yes] | history (exit 2 = SQL zero but bytes unchecked)")
	fmt.Fprintln(w, "  plugin                plugin render [--out DIR] [--mcp-url U] [--marketplace]: render the Claude Code plugin tree")
	fmt.Fprintln(w, "  version               Print version information")
	fmt.Fprintln(w, "  help                  Show this help message")
}

// newFlagSet returns a flag.FlagSet that writes usage to stderr and suppresses
// the default error printing (we print our own).
func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}
