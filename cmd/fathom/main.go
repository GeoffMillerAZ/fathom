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
	"strings"
	"syscall"

	"github.com/geoffmilleraz/fathom/internal/scope"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func main() {
	// The launchd agent's program is fathomd (launcher.go): before any other
	// work, flag parsing included, it execs the fathom binary beside it.
	if code, handled := runLauncher(os.Args, os.Environ(), launcherSys{Stat: os.Stat, Exec: syscall.Exec, Executable: os.Executable}, os.Stderr); handled {
		os.Exit(code)
	}
	prepareProcess()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// installCallerTransport wires the process's origin and hook session (the
// single rpcOrigin/rpcSession sources originHeader also reads) into every
// hand-built daemon request (O1151).
func installCallerTransport() {
	client.InstallCallerTransport(func() string { return rpcOrigin }, func() string { return rpcSession })
}

// prepareProcess is the real binary's process-wide setup, done here and
// never in a package init so `go test` stays hermetic.
func prepareProcess() {
	// RT-164 (§10.4): every request to a daemon carries X-Fathom-Caller,
	// including the ones not built by originHeader (the session primer,
	// /refresh-file, /route, /metrics, /statusz, the custodian's reads).
	installCallerTransport()
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

// run dispatches to the registered verb (verbs.go) named by args[0].
// It returns 0 on success, 1 on any error.
func run(args []string, stdout, stderr io.Writer) int {
	return runWithStdin(args, os.Stdin, stdout, stderr)
}

// runWithStdin is the testable core of run; it accepts an explicit stdin
// reader so that subcommands that consume stdin (e.g. serve-mcp) can be
// driven by in-memory buffers in tests. Each verb registers itself from its
// own file (declareVerb, EW-45); an unknown name prints the usage.
func runWithStdin(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: no subcommand given")
		printUsage(stderr)
		return 1
	}
	switch args[0] {
	case "--help", "-h", "-help":
		printUsage(stdout)
		return 0
	case "--version", "-version":
		fmt.Fprintln(stdout, BuildRevision())
		return 0
	}
	v, ok := lookupVerb(args[0])
	if ok && v.Name == "help" && len(args) >= 2 && !isHelpToken(args[1]) {
		// `fathom help <verb> [<sub> ...]` is the same path followed by --help.
		hv, found := lookupVerb(args[1])
		if !found {
			fmt.Fprintf(stderr, "error: unknown subcommand %q\n", args[1])
			printUsage(stderr)
			return 1
		}
		path := args[2:]
		if len(path) > 0 && isHelpToken(path[len(path)-1]) {
			path = path[:len(path)-1]
		}
		sub := strings.Join(path, " ")
		if sub != "" && !isSubverb(hv.Name, sub) {
			fmt.Fprintf(stderr, "error: unknown %s subcommand %q (%s)\n", hv.Name, sub, strings.Join(verbSubverbs()[hv.Name], ", "))
			return 1
		}
		return runVerbHelp(stdout, hv, sub)
	}
	if ok {
		if sub, help := helpRequest(v.Name, args[1:]); help {
			if v.Name == "help" {
				printUsage(stdout)
				return 0
			}
			return runVerbHelp(stdout, v, sub)
		}
	}
	if !ok {
		fmt.Fprintf(stderr, "error: unknown subcommand %q\n", args[0])
		printUsage(stderr)
		return 1
	}
	return v.Run.execVerb(args[1:], stdin, stdout, stderr)
}

// printUsage renders the verb registry in help order, skipping any verb
// frozen out of help (verb.Hidden, SR-16/SR-18): still fully dispatched and
// callable (TestHelpListsEveryDispatchedVerb pins that), just not
// something an agent discovers by browsing `fathom help`.
func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: fathom <subcommand> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Subcommands:")
	all := verbs()
	width := 0
	for _, v := range all {
		if v.Hidden {
			continue
		}
		if n := len(v.Name); n > width {
			width = n
		}
	}
	for _, v := range all {
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
