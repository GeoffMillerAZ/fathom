package main

// Verb help (K11 S1): every registered verb answers a SOLE -h, --help, -help
// or help argument, and `fathom help <verb>`, with usage on stdout and exit 0
// WITHOUT running the verb. The interception is central (runWithStdin), so no
// verb reads stdin, touches disk or calls a daemon to answer help.

import (
	"fmt"
	"io"
	"strings"
)

// isHelpToken reports whether a is one of the four help spellings. Only a
// SOLE help token after the verb is intercepted (helpRequested); a
// single-dash long flag such as -dry-run or -plan is never one.
func isHelpToken(a string) bool {
	switch a {
	case "-h", "--help", "-help", "help":
		return true
	}
	return false
}

// helpRequested reports whether the arguments after a verb are exactly one
// help token.
func helpRequested(args []string) bool {
	return len(args) == 1 && isHelpToken(args[0])
}

// verbSubverbs returns, per verb, the subverbs it dispatches: data kept
// beside the help renderer, a fresh map per call and never a package-level
// one (the tenancy lint's pkg-map rule, design §4.7).
// TestEveryVerbHelpExitsZeroWithoutRunning pins that every listed verb is
// registered.
func verbSubverbs() map[string][]string {
	return map[string][]string{
		"claim":    {"add", "refute", "corroborate", "show", "list", "effectiveness", "learn"},
		"daemon":   {"install", "status", "uninstall", "stop", "start", "pause", "resume", "ps", "reap", "deploy-intent"},
		"export":   {"scip"},
		"findings": {"near", "status", "import", "candidates"},
		"intent":   {"audit", "adr-rules", "doctrine", "export", "render"},
		"harness":  {"list", "capabilities", "hooks", "ladder", "matrix"},
		"layers":   {"list", "show", "install", "strata", "coverage"},
		"map":      {"new", "append", "name", "include", "compose", "show", "list"},
		"metrics":  {"spend", "traffic", "channels", "interventions", "series", "rollup", "rotate", "impact", "catches", "sessions", "load", "instrument", "flywheel", "retro", "rate", "arm"},
		"plans":    {"status", "next", "history", "context", "outcome", "lease", "leases", "archive", "restore", "lines"},
		"receipt":  {"verify"},
		"module":   {"init", "sources", "map", "compile", "eval", "doctor", "pack", "runs"},
		"research": {"report", "bundle"},
		"rules":    {"gen-test", "replay"},
		"thread":   {"brief", "note", "point", "list", "close", "promote", "handoff"},
		"scope":    {"list", "suppress", "unsuppress", "preview"},
	}
}

// printVerbHelp writes one verb's usage to w: the invocation line, the
// registry summary and its subverbs when it has any.
func printVerbHelp(w io.Writer, v verb) {
	fmt.Fprintf(w, "Usage: fathom %s", v.Name)
	subs := verbSubverbs()[v.Name]
	if len(subs) > 0 {
		fmt.Fprint(w, " <subcommand>")
	}
	fmt.Fprintln(w, " [flags]")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s: %s\n", v.Name, v.Summary)
	if len(subs) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Subcommands: %s\n", strings.Join(subs, ", "))
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Run `fathom %s` with its flags to execute it; -h, --help, -help and help only print this text.\n", v.Name)
}
