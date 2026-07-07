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
)

func main() {
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
		return cmdSessionHook(args[1:], stdout, stderr)
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
	fmt.Fprintln(w, "  gen-platform-manifest Generate a platform manifest from *.duckdb store files")
	fmt.Fprintln(w, "  inspect               List stored symbols for a service")
	fmt.Fprintln(w, "  query                 Query symbols for a concern")
	fmt.Fprintln(w, "  serve-mcp             Serve the MCP JSON-RPC protocol over stdio")
	fmt.Fprintln(w, "  serve-http            Serve the MCP JSON-RPC protocol over HTTP (POST /rpc)")
	fmt.Fprintln(w, "  platform-ingest       Ingest all services under a root directory")
	fmt.Fprintln(w, "  platform-callers      Print cross-repo callers of a symbol")
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
