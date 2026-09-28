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
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func main() {
	prepareProcess()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// prepareProcess is the real binary's process-wide setup, done here and
// never in a package init so `go test` stays hermetic.
func prepareProcess() {
	// RT-164 (§10.4): every request to a daemon carries X-Fathom-Caller,
	// including the ones not built by originHeader (the session primer,
	// /refresh-file, /route, /metrics, /statusz, the custodian's reads).
	client.InstallCallerTransport(func() string { return rpcOrigin })
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
	case "discoveries":
		return cmdDiscoveries(args[1:], stdout, stderr)
	// "routes" (SR-29, surface reckoning F-47) is no longer its own
	// top-level verb — cmdRoutes is now reached only through cmdPrime's
	// --routes flag ("prime" case above).
	case "query":
		return cmdQuery(args[1:], stdout, stderr)
	case "inspect":
		return cmdInspect(args[1:], stdout, stderr)
	case "serve-mcp":
		return runServeMCP(args[1:], stdin, stdout, stderr)
	case "serve-http":
		return runServeHTTP(args[1:], stdout, stderr)
	case "theme-guide":
		return cmdThemeGuide(args[1:], stdout, stderr)
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
	case "obligations":
		return cmdObligations(args[1:], stdout, stderr)
	case "report":
		if len(args) > 1 && args[1] == "evidence" {
			return cmdReportEvidence(args[2:], stdout, stderr)
		}
		if len(args) > 1 && args[1] == "packs" {
			return cmdReportPacks(args[2:], stdout, stderr)
		}
		return cmdReport(args[1:], stdout, stderr)
	case "review":
		return cmdReview(args[1:], stdout, stderr)
	case "conserve":
		return cmdConserve(args[1:], stdout, stderr)
	case "refactor":
		if len(args) > 1 && args[1] == "plan" {
			return cmdRefactorPlan(args[2:], stdout, stderr)
		}
		if len(args) > 1 && args[1] == "status" {
			return cmdRefactorStatus(args[2:], stdout, stderr)
		}
		fmt.Fprintln(stderr, "usage: fathom refactor plan <symbol|package|import> [--out FILE] [--service S] [--store DB]\n       fathom refactor status --plan FILE [--json]\n       fathom refactor status --migration \"<kind> <from> -> <to>\" [--store DB] [--services a,b,c] [--json]")
		return 2
	case "review-key":
		return cmdReviewKey(args[1:], stdout, stderr)
	case "coverage":
		if len(args) > 1 && args[1] == "ingest" {
			return cmdCoverageIngest(args[2:], stdout, stderr)
		}
		fmt.Fprintln(stderr, "usage: fathom coverage ingest <file> --format lcov|cobertura|jacoco|coverage-py|istanbul|llvm-cov [--commit SHA] [--json]")
		return 1
	case "tests":
		if len(args) > 1 && args[1] == "coverage" {
			return cmdTestsCoverage(args[2:], stdout, stderr)
		}
		if len(args) > 1 && args[1] == "select" {
			return cmdTestsSelect(args[2:], stdout, stderr)
		}
		if len(args) > 1 && args[1] == "ingest" {
			return cmdTestsIngest(args[2:], stdout, stderr)
		}
		if len(args) > 1 && args[1] == "status" {
			return cmdTestsStatus(args[2:], stdout, stderr)
		}
		fmt.Fprintln(stderr, "usage: fathom tests coverage --profile <file> [--service S] [--diff <base>] [--json]\n       fathom tests select --since <ref> [--profile <file>]... [--service S] [--json]\n       fathom tests ingest --manifest <dir> --results <file|dir> [--service S] [--store DB]\n       fathom tests status [--service S] [--store DB] [--json]")
		return 1
	case "certify":
		if len(args) > 1 && args[1] == "status" {
			return cmdCertifyStatus(args[2:], stdout, stderr)
		}
		if len(args) > 1 && args[1] == "gaps" {
			return cmdCertifyGaps(args[2:], stdout, stderr)
		}
		if len(args) > 1 && args[1] == "manifest" {
			return cmdCertifyManifest(args[2:], stdout, stderr)
		}
		if len(args) > 1 && args[1] == "orphans" {
			return cmdCertifyOrphans(args[2:], stdout, stderr)
		}
		if len(args) > 1 && args[1] == "suggest" {
			return cmdCertifySuggest(args[2:], stdout, stderr)
		}
		fmt.Fprintln(stderr, "usage: fathom certify status [--service S] [--store DB] [--json]\n       fathom certify gaps [--kind invariant|incident|spec|port|rule|behaviour] [--service S] [--store DB] [--json]\n       fathom certify manifest [--out FILE] [--check] [--service S] [--store DB]\n       fathom certify orphans [--service S] [--store DB] [--json]\n       fathom certify suggest [--service S] [--store DB] [--limit N]")
		return 1
	case "edit-hook":
		return cmdEditHook(args[1:], stdout, stderr)
	case "prompt-hook":
		return cmdPromptHook(args[1:], stdout, stderr)
	case "instrument":
		return cmdInstrument(args[1:], stdout, stderr)
	case "observe":
		return cmdObserve(args[1:], stdout, stderr)
	case "arm":
		return cmdArm(args[1:], stdout, stderr)
	case "plugin":
		if len(args) > 1 && args[1] == "render" {
			return cmdPluginRender(args[2:], stdout, stderr)
		}
		fmt.Fprintln(stderr, "usage: fathom plugin render [--out DIR] [--mcp-url U] [--marketplace]")
		return 1
	case "harness":
		return cmdHarness(args[1:], stdout, stderr)
	case "support":
		return cmdSupport(args[1:], stdout, stderr)
	case "capabilities":
		return cmdCapabilities(args[1:], stdout, stderr)
	case "flywheel":
		return cmdFlywheel(args[1:], stdout, stderr)
	case "rate":
		return cmdRate(args[1:], stdout, stderr)
	case "assess":
		return cmdAssess(args[1:], stdout, stderr)
	case "retro":
		return cmdRetro(args[1:], stdout, stderr)
	case "thread":
		return cmdThread(args[1:], stdout, stderr)
	case "session-end-hook":
		return cmdSessionEndHook(args[1:], stdout, stderr)
	case "pre-compact-hook":
		return cmdPreCompactHook(args[1:], stdout, stderr)
	case "post-commit-hook":
		return cmdPostCommitHook(args[1:], stdout, stderr)
	case "intent":
		return cmdIntent(args[1:], stdout, stderr)
	case "export":
		return cmdExport(args[1:], stdout, stderr)
	case "daemon":
		return cmdDaemon(args[1:], stdout, stderr)
	case "fleet":
		if len(args) > 1 && args[1] == "report" {
			return cmdFleetReport(args[2:], stdout, stderr)
		}
		if len(args) > 1 && args[1] == "reconcile" {
			return cmdFleetReconcile(args[2:], stdout, stderr)
		}
		if len(args) > 1 && args[1] == "realization" {
			return cmdFleetRealization(args[2:], stdout, stderr)
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
		// SH-9: time-to-remedy over the report snapshots.
		if len(args) > 1 && args[1] == "remedy" {
			return cmdFleetRemedy(args[2:], stdout, stderr)
		}
		fmt.Fprintln(stderr, "usage: fathom fleet report|reconcile|realization|receipts|undo|remedy [flags]")
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
	case "resources":
		return cmdResources(args[1:], stdout, stderr)
	case "gc":
		return cmdGC(args[1:], stdout, stderr)
	case "research":
		return cmdResearch(args[1:], stdout, stderr)
	case "module":
		return cmdModule(args[1:], stdout, stderr)
	case "rules":
		return cmdRules(args[1:], stdout, stderr)
	case "receipt":
		return cmdReceipt(args[1:], stdout, stderr)
	case "metrics":
		return cmdMetrics(args[1:], stdout, stderr)
	case "findings":
		return cmdFindings(args[1:], stdout, stderr)
	case "scope":
		return cmdScope(args[1:], stdout, stderr)
	case "purge":
		return cmdPurge(args[1:], stdout, stderr)
	case "decide":
		return cmdDecide(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown subcommand %q\n", args[0])
		printUsage(stderr)
		return 1
	}
}

// printUsage renders the dispatched verb table, skipping any verb frozen
// out of help (verb.Hidden, SR-16/SR-18): still fully dispatched and
// callable (TestHelpListsEveryDispatchedVerb pins that), just not
// something an agent discovers by browsing `fathom help`.
func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: fathom <subcommand> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Subcommands:")
	width := 0
	for _, v := range verbTable {
		if v.Hidden {
			continue
		}
		if n := len(v.Name); n > width {
			width = n
		}
	}
	for _, v := range verbTable {
		if v.Hidden {
			continue
		}
		fmt.Fprintf(w, "  %-*s  %s\n", width, v.Name, v.Summary)
	}
}

// newFlagSet returns a flag.FlagSet that writes usage to stderr and suppresses
// the default error printing (we print our own).
func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}
