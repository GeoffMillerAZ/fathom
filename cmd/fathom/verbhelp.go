package main

// Verb help (K11 S1, O671): every registered verb answers a SOLE -h,
// --help, -help or help argument, and `fathom help <verb>`, on stdout with
// exit 0 WITHOUT running the verb; `fathom <verb> <sub> <help>` and `fathom
// help <verb> <sub>` do the same for a subverb. The interception is central
// (runWithStdin); runVerbHelp is the one function that answers.
//
// The flags come from the verb's own FlagSet: help hands the registered
// handler ["-h"] (or [sub, "-h"]) with its stderr bound to stdout, so
// flag.Parse prints the defaults there and returns flag.ErrHelp, and help
// ignores the handler's exit. That is safe only for a handler that parses
// first: TestVerbHelpParseFirstProof reads every handler's source and
// proves that, for exactly that input, nothing but a FlagSet, flag
// definitions and literals runs before Parse. A handler that does work
// first (a kill switch, a config read that seeds defaults, a required
// leading positional) is answered from verbHelpStatic, pinned to its
// source by TestVerbHelpStaticListsPinned; one with no FlagSet on the help
// path prints the header alone (verbHelpHeaderOnly).

import (
	"fmt"
	"io"
	"strings"
)

// isHelpToken reports whether a is one of the four help spellings. Only a
// SOLE help token after the verb, or after one of its subverbs, is
// intercepted (helpRequest); a single-dash long flag such as -dry-run or
// -plan is never one.
func isHelpToken(a string) bool {
	switch a {
	case "-h", "--help", "-help", "help":
		return true
	}
	return false
}

// helpRequest reports whether args (the arguments after the verb) ask for
// help: one help token (sub ""), or a subverb of the verb then one help
// token.
func helpRequest(verb string, args []string) (sub string, ok bool) {
	switch {
	case len(args) == 1 && isHelpToken(args[0]):
		return "", true
	case len(args) == 2 && isHelpToken(args[1]) && isSubverb(verb, args[0]):
		return args[0], true
	}
	return "", false
}

// isSubverb reports whether sub is one of verb's subverbs.
func isSubverb(verb, sub string) bool {
	for _, s := range verbSubverbs()[verb] {
		if s == sub {
			return true
		}
	}
	return false
}

// verbSubverbs returns, per verb, the subverbs it dispatches: data kept
// beside the help renderer, a fresh map per call and never a package-level
// one (the tenancy lint's pkg-map rule, design §4.7).
// TestVerbSubverbsMatchDispatch pins each list to the literals the verb's
// handler routes on.
func verbSubverbs() map[string][]string {
	return map[string][]string{
		"arm":          {"off", "on"},
		"assess":       {"spend"},
		"capabilities": {"explain", "set"},
		"certify":      {"status", "gaps", "manifest", "orphans", "suggest"},
		"claim":        {"add", "refute", "corroborate", "show", "list", "effectiveness", "learn"},
		"config":       {"audit", "propose"},
		"coverage":     {"ingest"},
		"daemon":       {"install", "status", "uninstall", "stop", "start", "pause", "resume", "ps", "reap", "deploy-intent"},
		"decide":       {"log"},
		"export":       {"scip"},
		"findings":     {"near", "status", "import", "candidates"},
		"fleet":        {"report", "reconcile", "realization", "receipts", "undo", "remedy"},
		"intent":       {"audit", "adr-rules", "doctrine", "export", "render"},
		"harness":      {"list", "capabilities", "hooks", "ladder", "matrix"},
		"layers":       {"list", "show", "install", "strata", "coverage"},
		"map":          {"new", "append", "name", "include", "compose", "show", "list"},
		"metrics":      {"spend", "traffic", "channels", "interventions", "series", "rollup", "rotate", "impact", "catches", "sessions", "load", "instrument", "flywheel", "retro", "rate", "arm"},
		"obligations":  {"promote"},
		"plans":        {"status", "next", "history", "context", "outcome", "lease", "leases", "archive", "restore", "lines"},
		"plugin":       {"render"},
		"purge":        {"history"},
		"receipt":      {"verify"},
		"refactor":     {"plan", "status"},
		"report":       {"evidence", "packs", "used"},
		"module":       {"init", "sources", "map", "compile", "eval", "doctor", "pack", "runs"},
		"research":     {"report", "bundle"},
		"rules":        {"gen-test", "replay"},
		"tests":        {"coverage", "select", "ingest", "status"},
		"thread":       {"brief", "note", "point", "list", "close", "promote", "handoff"},
		"trace":        {"export", "verify"},
		"scope":        {"list", "suppress", "unsuppress", "preview"},
	}
}

// verbHelpHeaderOnly names the verbs and subverbs ("verb" or "verb sub")
// with no FlagSet on the help path, each with why: help prints the header
// alone. TestVerbHelpParseFirstProof pins the set to the source.
func verbHelpHeaderOnly() map[string]string {
	const dispatch = "dispatches its subverbs; it defines no flags of its own"
	return map[string]string{
		"arm":           "no arguments prints the arm state; its flags belong to `arm off`",
		"arm on":        "takes no flags",
		"certify":       dispatch,
		"claim":         dispatch,
		"coverage":      dispatch,
		"daemon":        dispatch,
		"decide":        dispatch,
		"export":        dispatch,
		"findings":      dispatch,
		"fleet":         dispatch,
		"gen-claude-md": "a deprecated alias: it prints its notice, then runs gen-agents-md (see `fathom gen-agents-md --help`)",
		"harness":       dispatch,
		"help":          "prints the verb list",
		"intent":        dispatch,
		"layers":        dispatch,
		"map":           dispatch,
		"metrics":       dispatch,
		"metrics arm":   "no arguments prints the arm state; its flags belong to `arm off`",
		"module":        dispatch,
		"plans":         dispatch,
		"plugin":        dispatch,
		"receipt":       dispatch,
		"refactor":      dispatch,
		"research":      dispatch,
		"rules":         dispatch,
		"scope":         dispatch + " (no subverb lists)",
		"tests":         dispatch,
		"thread":        dispatch,
		"trace":         dispatch,
		"version":       "reads --json by hand; it has no FlagSet",
	}
}

// helpFlag is one entry of a static help list.
type helpFlag struct {
	Name    string
	Default string // as help prints it; "" for none
	Usage   string
}

// runVerbHelp answers help for `fathom <verb> [<sub>]` on stdout and
// returns 0: the header, then the flags — the handler's own FlagSet's for a
// proven parse-first handler, else its static list, else none.
func runVerbHelp(stdout io.Writer, v verb, sub string) int {
	printVerbHelp(stdout, v, sub)
	key := v.Name
	if sub != "" {
		key += " " + sub
	}
	if _, ok := verbHelpHeaderOnly()[key]; ok {
		return 0
	}
	if flags, ok := verbHelpStatic()[key]; ok {
		printStaticFlags(stdout, key, flags)
		return 0
	}
	in := []string{"-h"}
	if sub != "" {
		in = []string{sub, "-h"}
	}
	// The proven ErrHelp path: Parse prints the defaults to the handler's
	// stderr, bound here to stdout, and the handler returns its parse-error
	// exit, which help ignores. No stdin is handed over.
	_ = v.Run.execVerb(in, strings.NewReader(""), stdout, stdout)
	return 0
}

// printVerbHelp writes the header for `fathom <verb> [<sub>]`: the
// invocation line, the registry summary and, for a verb with subverbs, the
// subverbs.
func printVerbHelp(w io.Writer, v verb, sub string) {
	subs := verbSubverbs()[v.Name]
	// name is the invocation help describes, as its usage line spells it.
	name := v.Name
	switch {
	case sub != "":
		name += " " + sub
	case len(subs) == 0:
	default:
		name += " <subcommand>"
		if _, ok := verbHelpHeaderOnly()[v.Name]; !ok {
			name = v.Name + " [<subcommand>]" // it also runs with flags alone
		}
	}
	fmt.Fprintf(w, "Usage: fathom %s [flags]\n", name)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s: %s\n", v.Name, v.Summary)
	if len(subs) > 0 && sub == "" {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Subcommands: %s\n", strings.Join(subs, ", "))
		fmt.Fprintf(w, "Run `fathom %s <subcommand> --help` for a subcommand's flags.\n", v.Name)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Run `fathom %s` with its flags to execute it; -h, --help, -help and help only print this text.\n", name)
	fmt.Fprintln(w)
}

// printStaticFlags prints a static list in flag.PrintDefaults' layout.
func printStaticFlags(w io.Writer, key string, flags []helpFlag) {
	fmt.Fprintf(w, "Usage of %s:\n", key)
	for _, f := range flags {
		fmt.Fprintf(w, "  -%s\n    \t%s", f.Name, strings.ReplaceAll(f.Usage, "\n", "\n    \t"))
		if f.Default != "" {
			fmt.Fprintf(w, " (default %s)", f.Default)
		}
		fmt.Fprintln(w)
	}
}
