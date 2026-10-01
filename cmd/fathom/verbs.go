package main

// verbs.go — the verb registry (HF-6, EW-45).
//
// Every dispatched subcommand registers itself: a `func init()` at the end
// of the file that defines its handler calls declareVerb with its name,
// help summary, help group and order, whether help hides it, and the
// runner `main` dispatches to. `main` routes through lookupVerb and
// `fathom help` renders verbs(), so there is no dispatch switch and no
// verb table to edit — a wave that adds a verb edits only its own file
// (main.go and this file conflicted in 18 merges while they held both).
// The precedent is the console's page registry (console_registry.go,
// declarePage).
//
// TestHelpListsEveryDispatchedVerb holds the structural fix for the
// 2026-08-25 defect where six dispatched verbs (prime, review, fleet, the
// session hooks) worked but were invisible in help: it reads the registry,
// routes every registered verb, and requires every unhidden verb to be a
// help line and every help line to be a registered verb.
//
// It is also the resolver the `## Maintained` doc contract needs (HF-7):
// a name in a use-case doc's Maintained line is legitimate iff it is a
// registered MCP tool or a registered verb.
//
// Why Run is an interface and not a func field: a call through a func
// field is a call the typed pass cannot resolve (an EdgeUnresolved edge),
// so `run` would reach no handler and every test that drives a verb
// through run(...) would stop statically reaching it. A call through an
// interface method is an EdgeIfaceDispatch edge to every implementer, so
// runWithStdin keeps reaching each verb's runner, and each runner its
// handler, exactly as the switch's direct calls did.

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"sync"
)

// verbRunner runs one verb: args are the arguments after the verb's name;
// the result is the process exit code. Each verb's file declares a small
// named type implementing it (see the init at the end of ingest.go).
type verbRunner interface {
	execVerb(args []string, stdin io.Reader, stdout, stderr io.Writer) int
}

// verb is one dispatched subcommand: its name, one-line help summary, where
// help lists it, and its runner.
type verb struct {
	Name    string
	Summary string
	// Group and Order place the verb in `fathom help`: by the group's rank
	// in verbGroups, then Order, then Name. A (Group, Order) pair names one
	// verb; leave gaps (10, 20, …) so a new verb slots in without moving
	// its neighbours.
	Group string
	Order int
	// Hidden freezes a verb out of `fathom help` (SR-16/SR-18, surface
	// reckoning F-53/F-70) while it stays fully dispatched and callable—
	// for a verb an ADR keeps alive on a closed allowlist (research,
	// ADR-046 decision 6) or behind an off-by-default capability (assess,
	// ADR-046/ADR-053), not one anyone should discover by browsing help.
	// TestHelpListsEveryDispatchedVerb still requires it registered and
	// routed; only printUsage's rendering skips it.
	Hidden bool
	Run    verbRunner
}

// verbGroups is the help order of the verb groups. A verb declaring a
// group not listed here is refused.
var verbGroups = []string{
	"index",   // ingest and read the index
	"serve",   // the MCP surfaces and fleet-wide reads
	"plans",   // plans, verification and semantic layers
	"review",  // review, conservation and test evidence
	"hooks",   // harness hooks and session continuity
	"operate", // wiring, the daemon, diagnostics and the A/B instruments
	"govern",  // continuity, fleet, modules, governance and the registries
	"meta",    // version and help
}

// verbDecls collects init()-time declarations.
var verbDecls []verb

// declareVerb is what each verb's file calls from its init().
func declareVerb(v verb) { verbDecls = append(verbDecls, v) }

var (
	verbRegOnce sync.Once
	verbReg     []verb
)

// verbs is every registered verb in help order. A declaration orderVerbs
// refuses is a programming error and panics at first use (in every test
// that dispatches or renders help), never a silently missing verb.
func verbs() []verb {
	verbRegOnce.Do(func() {
		reg, err := orderVerbs(verbDecls)
		if err != nil {
			panic(err)
		}
		verbReg = reg
	})
	return verbReg
}

// orderVerbs validates decls and returns them in help order: by group rank,
// then Order, then Name. A missing name or runner, an unknown group, a
// duplicate name or a duplicate (group, order) slot is refused.
func orderVerbs(decls []verb) ([]verb, error) {
	rank := func(group string) int {
		for i, g := range verbGroups {
			if g == group {
				return i
			}
		}
		return -1
	}
	names := make(map[string]bool, len(decls))
	slots := make(map[string]string, len(decls))
	for _, v := range decls {
		switch {
		case v.Name == "":
			return nil, fmt.Errorf("fathom: a verb declaration has no Name (summary %q)", v.Summary)
		case v.Run == nil:
			return nil, fmt.Errorf("fathom: verb %q declares no Run", v.Name)
		case rank(v.Group) < 0:
			return nil, fmt.Errorf("fathom: verb %q declares unknown group %q", v.Name, v.Group)
		case names[v.Name]:
			return nil, fmt.Errorf("fathom: verb %q is declared twice", v.Name)
		}
		names[v.Name] = true
		slot := v.Group + "/" + strconv.Itoa(v.Order)
		if prev, ok := slots[slot]; ok {
			return nil, fmt.Errorf("fathom: verbs %q and %q both declare help slot %s", prev, v.Name, slot)
		}
		slots[slot] = v.Name
	}
	out := append([]verb(nil), decls...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if ra, rb := rank(a.Group), rank(b.Group); ra != rb {
			return ra < rb
		}
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		return a.Name < b.Name
	})
	return out, nil
}

// lookupVerb returns the registered verb named name.
func lookupVerb(name string) (verb, bool) {
	for _, v := range verbs() {
		if v.Name == name {
			return v, true
		}
	}
	return verb{}, false
}

// verbNames returns every dispatched subcommand name, in help order. The
// `## Maintained` parser and the help renderer share it, so a verb cannot
// be dispatched-but-undocumented or documented-but-undispatched.
func verbNames() []string {
	all := verbs()
	names := make([]string, 0, len(all))
	for _, v := range all {
		names = append(names, v.Name)
	}
	return names
}

// isVerb reports whether name is a dispatched subcommand.
func isVerb(name string) bool {
	_, ok := lookupVerb(name)
	return ok
}
