package main

// support.go — `fathom support` (SM-10, design docs/design/support-matrix.md
// §4, plan docs/plans/support-matrix.md §3): the CLI surface over
// internal/support's projection, and the join design §2 leaves to this
// command — turning each kind's registry, target list and readings into
// the per-capability CellInput internal/support.Project needs.
// internal/support itself never reads a queue file, a dialect manifest, or
// the language readings directory (its own doc.go: "never imports the
// extraction engines"); this file does, so a kind's engine (langbench's
// tree-sitter/wasm machinery today, one day a spec-dialect adapter) never
// has to be imported by the projection package.
//
// No MCP tool is added for this (design §4): an agent reads the CLI or the
// generated doc (SM-11). This command never dials the daemon and never
// calls internal/langbench's extraction engines — TrackerInputFromRepo only
// reads already-committed JSON/manifests.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/langbench"
	"github.com/geoffmilleraz/fathom/internal/support"
)

// supportKinds is the fixed --kind vocabulary, in the fixed print order
// design §1's kinds table lists.
var supportKinds = []support.Kind{
	support.KindLanguage, support.KindTool, support.KindClient, support.KindSpec,
	support.KindRules, support.KindCoverage, support.KindContract, support.KindEcosystem,
}

func supportKindNames() []string {
	names := make([]string, len(supportKinds))
	for i, k := range supportKinds {
		names[i] = string(k)
	}
	return names
}

func validSupportKind(k string) bool {
	for _, sk := range supportKinds {
		if string(sk) == k {
			return true
		}
	}
	return false
}

// cmdSupport implements `fathom support [--kind K] [--json]` (SM-10).
func cmdSupport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("support", stderr)
	kindFlag := fs.String("kind", "", "limit to one kind: "+strings.Join(supportKindNames(), "|"))
	jsonOut := fs.Bool("json", false, "emit the fathom.support/v1 matrix as JSON")
	rootFlag := fs.String("root", "", "the fathom checkout whose matrix to project (default: the current directory)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *kindFlag != "" && !validSupportKind(*kindFlag) {
		fmt.Fprintf(stderr, "error: unknown --kind %q (want %s)\n", *kindFlag, strings.Join(supportKindNames(), "|"))
		return 1
	}
	root := *rootFlag
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		root = wd
	}
	//fathom:tenancy-allow bg-ctx a one-shot CLI process: nothing outlives it to cancel
	src, err := loadSupportSources(context.Background(), root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	matrix := buildSupportMatrix(src, time.Now)
	if *kindFlag != "" {
		matrix.Entries = filterEntriesByKind(matrix.Entries, support.Kind(*kindFlag))
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(matrix); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}
	if *kindFlag != "" && len(matrix.Entries) == 0 {
		fmt.Fprintf(stdout, "## %s (0 entries)\n\nSummary: at-target=0 below=0 tolerated=0 declared=0 refused=0 deferred=0\n", *kindFlag)
		return 0
	}
	renderSupportTable(stdout, matrix)
	return 0
}

// supportSources is every already-loaded input buildSupportMatrix needs —
// no hidden file read, no hidden clock inside the join itself
// (internal/AGENTS.md's determinism contract, the same discipline design §4
// holds internal/support.Project to).
type supportSources struct {
	Queue      []langbench.QueueEntry
	Readings   map[string]langbench.Reading
	Registered map[string]langbench.Registration
	Refused    map[string]string // queue items their plan row refused (design §14.2)
	Targets    []support.Target  // the six shared kinds, docs/plans/support.targets.json
	Clients    []support.RegistryEntry
	Ledger     []support.LedgerRow
	Commit     string
}

// loadSupportSources reads every file buildSupportMatrix needs from a
// checkout at root — the only function in this file that touches disk. It
// never dials the daemon (design §4/§6: "no network").
func loadSupportSources(ctx context.Context, root string) (supportSources, error) {
	tin, err := langbench.TrackerInputFromRepo(root)
	if err != nil {
		return supportSources{}, fmt.Errorf("support: %w", err)
	}
	targets, err := support.LoadTargets(filepath.Join(root, "docs", "plans", "support.targets.json"))
	if err != nil {
		return supportSources{}, err
	}
	ledger, err := support.LoadLedgerRows(filepath.Join(root, "docs", "bench", "readings", "support-ledger.jsonl"))
	if err != nil {
		return supportSources{}, err
	}
	commit, _ := gitro.Head(ctx, root) // "" is fine: Matrix.Commit is descriptive, never gating
	return supportSources{
		Queue:      tin.Queue,
		Readings:   tin.Readings,
		Registered: tin.Registered,
		Refused:    tin.Refused,
		Targets:    targets,
		Clients:    support.ClientRegistry(),
		Ledger:     ledger,
		Commit:     commit,
	}, nil
}

// languageLevels is the fixed capability-column order for a kind: L1-L4 for
// a language, L1-L3 for a tool format — L4 is not even a column for a tool
// (design §2/§3).
func languageLevels(kind support.Kind) []string {
	if kind == support.KindTool {
		return []string{"L1", "L2", "L3"}
	}
	return []string{"L1", "L2", "L3", "L4"}
}

// levelIndex is langbench's Levels index of level ("L1".."L4"), or -1 for
// LevelNone ("none"), "" (no target declared) or any other non-level value.
func levelIndex(level string) int {
	for i, l := range langbench.Levels {
		if l == level {
			return i
		}
	}
	return -1
}

// mapLangGrade converts one langbench per-level grade string (excellent |
// good | tolerated | unmet | not-attempted | not-applicable) into a
// support.Grade — only excellent/good ever count as "a reading exists for
// this capability" (I54: tolerated/unmet/not-attempted must never look like
// a committed reading).
func mapLangGrade(g string) support.Grade {
	switch g {
	case langbench.GradeExcellent:
		return support.GradeExcellent
	case langbench.GradeGood:
		return support.GradeGood
	default:
		return support.GradeNone
	}
}

// languageToolEntries builds language and tool EntryInputs (design §2's
// language/tool row): registry from src.Registered (the dialect manifests
// LB-F1 built plus the incumbents/registered-formats table, LB-F17's own
// inputs), target from the queue's own target_level, reading from
// src.Readings. A queue id with no manifest yet is still emitted
// (Registered: false) so an unbuilt language shows `declared` rather than
// being silently omitted, matching LB-F17's tracker.
func languageToolEntries(src supportSources) []support.EntryInput {
	seen := make(map[string]bool, len(src.Queue))
	var entries []support.EntryInput
	for _, q := range src.Queue {
		seen[q.ID] = true
		kind := support.KindLanguage
		if q.Kind == "tooling" {
			kind = support.KindTool
		}
		_, registered := src.Registered[q.ID]
		reading, hasReading := src.Readings[q.ID]
		e := languageToolEntry(kind, q.ID, q.Name, q.Wave, q.Engine, q.PlanRow, q.TargetLevel, registered, reading, hasReading)
		if _, refused := src.Refused[q.ID]; refused {
			// §14.2: a refused item claims no level, whatever its reading measured.
			for i := range e.Cells {
				e.Cells[i].Disposition = "refused"
			}
		}
		entries = append(entries, e)
	}
	// A registered dialect absent from the queue (should not happen once
	// LB-F22 governs the queue) is still surfaced rather than dropped.
	for id, reg := range src.Registered {
		if seen[id] {
			continue
		}
		kind := support.KindLanguage
		reading, hasReading := src.Readings[id]
		if hasReading && reading.Kind == "tooling" {
			kind = support.KindTool
		}
		entries = append(entries, languageToolEntry(kind, id, id, 0, reg.Extractor, "", "", true, reading, hasReading))
	}
	return entries
}

func languageToolEntry(kind support.Kind, id, name string, wave int, engine, planRow, targetLevel string, registered bool, reading langbench.Reading, hasReading bool) support.EntryInput {
	targetIdx := levelIndex(targetLevel)
	cells := make([]support.CellInput, 0, 4)
	for _, lvl := range languageLevels(kind) {
		n := levelIndex(lvl)
		ci := support.CellInput{Capability: lvl, ApplicableToKind: true, Registered: registered, PlanRow: planRow}
		if targetIdx >= 0 && n <= targetIdx {
			ci.TargetDeclared = true
			ci.Target = support.CapabilityTarget{ProvenOnly: true}
		}
		if hasReading {
			if ceil := levelIndex(reading.LevelMet); ceil >= n {
				ci.Grade = mapLangGrade(reading.GradeAt(lvl))
				if ci.Grade != support.GradeNone {
					ci.ReadingPath = "docs/bench/readings/lang/" + id + ".json"
				}
			}
		}
		// A level neither registered, targeted nor read projects nothing
		// (state.go's ComputeState: "the caller never builds a cell for
		// this shape") — e.g. L2-L4 of a queued-but-unbuilt language whose
		// declared target is only L1.
		if !registered && !ci.TargetDeclared && ci.Grade == support.GradeNone {
			continue
		}
		cells = append(cells, ci)
	}
	return support.EntryInput{Kind: kind, ID: id, Name: name, Registered: registered, Wave: wave, Engine: engine, Cells: cells}
}

// sharedKindEntries builds one kind's EntryInputs from the shared target
// file plus (when one exists) that kind's registry — design §2's "the
// other six kinds share support.targets.json". registry is nil until a
// later plan's adapter lands (spec, rules, coverage, contract, ecosystem);
// client is the one shared kind with a registry today (SM-5).
func sharedKindEntries(targets []support.Target, kind support.Kind, registry []support.RegistryEntry) []support.EntryInput {
	regByID := make(map[string]support.RegistryEntry, len(registry))
	for _, r := range registry {
		regByID[r.ID] = r
	}
	targetByID := make(map[string]support.Target)
	ids := make(map[string]bool)
	for _, t := range targets {
		if t.Kind != kind {
			continue
		}
		targetByID[t.ID] = t
		ids[t.ID] = true
	}
	for id := range regByID {
		ids[id] = true
	}

	sortedIDs := make([]string, 0, len(ids))
	for id := range ids {
		sortedIDs = append(sortedIDs, id)
	}
	sort.Strings(sortedIDs)

	entries := make([]support.EntryInput, 0, len(sortedIDs))
	for _, id := range sortedIDs {
		reg, registered := regByID[id]
		target, hasTarget := targetByID[id]

		name, wave, engine := id, 0, ""
		if registered {
			name, engine = reg.Name, reg.Engine
		}
		if hasTarget {
			if !registered {
				name = target.Name
			}
			wave = target.Wave
		}

		capSet := make(map[string]bool)
		if registered {
			for _, c := range reg.Capabilities {
				capSet[c] = true
			}
		}
		if hasTarget {
			for c := range target.Targets {
				capSet[c] = true
			}
		}

		cells := make([]support.CellInput, 0, len(capSet))
		for capName := range capSet {
			ci := support.CellInput{Capability: capName, ApplicableToKind: true, Registered: registered}
			if hasTarget {
				if ct, ok := target.Targets[capName]; ok {
					ci.TargetDeclared = true
					ci.Target = ct
					ci.PlanRow = target.PlanRow
				}
			}
			// No reading loader for this kind exists yet (SM-12 wires the
			// first one, for client); a cell without a reading is
			// tolerated/declared, never a claimed level (I54).
			cells = append(cells, ci)
		}

		entries = append(entries, support.EntryInput{Kind: kind, ID: id, Name: name, Registered: registered, Wave: wave, Engine: engine, Cells: cells})
	}
	return entries
}

// buildSupportMatrix is the pure join (design §2/§4): registry + targets +
// readings -> internal/support.Project's EntryInput/CellInput, per kind,
// then one Project call for the shared sort/regression/schema logic
// (design §4: "all surfaces render one projection"). now is the injected
// clock Project itself requires.
func buildSupportMatrix(src supportSources, now func() time.Time) support.Matrix {
	var entries []support.EntryInput
	entries = append(entries, languageToolEntries(src)...)
	entries = append(entries, sharedKindEntries(src.Targets, support.KindClient, src.Clients)...)
	for _, k := range []support.Kind{support.KindSpec, support.KindRules, support.KindCoverage, support.KindContract, support.KindEcosystem} {
		entries = append(entries, sharedKindEntries(src.Targets, k, nil)...)
	}
	return support.Project(support.ProjectInput{Entries: entries, Ledger: src.Ledger, Commit: src.Commit}, now)
}

func filterEntriesByKind(entries []support.Entry, kind support.Kind) []support.Entry {
	var out []support.Entry
	for _, e := range entries {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

// entrySummaryBucket classifies one entry into the one-line summary design
// §4 asks for (entries at target, below, tolerated, declared) by its
// worst-ranked cell — regressed/below-target outranks everything else, a
// refused or deferred disposition is its own bucket, and an entry with
// nothing but proven/n-a cells is "at target".
func entrySummaryBucket(e support.Entry) string {
	rank := map[support.State]int{
		support.StateRegressed: 0, support.StateBelowTarget: 1,
		support.StateRefused: 2, support.StateDeferred: 3,
		support.StateDeclared: 4, support.StateTolerated: 5,
		support.StateProvenGood: 6, support.StateProvenExcellent: 6,
		support.StateNA: 7,
	}
	best, bestRank := support.State(""), 99
	for _, c := range e.Cells {
		if r, ok := rank[c.State]; ok && r < bestRank {
			best, bestRank = c.State, r
		}
	}
	switch best {
	case support.StateBelowTarget, support.StateRegressed:
		return "below"
	case support.StateRefused:
		return "refused"
	case support.StateDeferred:
		return "deferred"
	case support.StateDeclared:
		return "declared"
	case support.StateTolerated:
		return "tolerated"
	default:
		return "at-target"
	}
}

// renderSupportTable renders the text form design §4 asks for: a table per
// kind, then a one-line summary of entries by bucket (at target, below,
// tolerated, declared).
func renderSupportTable(w io.Writer, m support.Matrix) {
	byKind := make(map[support.Kind][]support.Entry)
	var order []support.Kind
	for _, e := range m.Entries {
		if _, ok := byKind[e.Kind]; !ok {
			order = append(order, e.Kind)
		}
		byKind[e.Kind] = append(byKind[e.Kind], e)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	for i, kind := range order {
		if i > 0 {
			fmt.Fprintln(w)
		}
		entries := byKind[kind]
		fmt.Fprintf(w, "## %s (%d entries)\n\n", kind, len(entries))
		fmt.Fprintf(w, "%-24s %-28s %-5s %-10s %s\n", "ID", "NAME", "WAVE", "ENGINE", "CELLS")
		counts := map[string]int{}
		for _, e := range entries {
			var cellStrs []string
			for _, c := range e.Cells {
				cellStrs = append(cellStrs, c.Capability+"="+string(c.State))
			}
			fmt.Fprintf(w, "%-24s %-28s %-5d %-10s %s\n", e.ID, e.Name, e.Wave, orDash(e.Engine), strings.Join(cellStrs, " "))
			counts[entrySummaryBucket(e)]++
		}
		fmt.Fprintf(w, "Summary: at-target=%d below=%d tolerated=%d declared=%d refused=%d deferred=%d\n",
			counts["at-target"], counts["below"], counts["tolerated"], counts["declared"], counts["refused"], counts["deferred"])
	}
}
