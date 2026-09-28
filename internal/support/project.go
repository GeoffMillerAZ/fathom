package support

// project.go — Project (SM-9, design §4): assembles the fathom.support/v1
// Matrix under an injected clock, computing each cell via ComputeState
// (SM-6) and then overlaying `regressed` from the readings ledger (SM-8's
// AppendLedger writes the history this reads, design §3's "the U12 idea for
// support"). Joining a kind's registry, targets and readings into the
// per-capability CellInput this takes is the `fathom support` CLI's job
// (SM-10); Project's own contract is state-plus-history-plus-ordering.

import (
	"sort"
	"time"
)

// EntryInput is one entry's already-joined data: identity plus one
// CellInput per capability (design §2's join across registry, targets and
// readings, resolved by the caller).
type EntryInput struct {
	Kind       Kind
	ID         string
	Name       string
	Registered bool
	Wave       int
	Engine     string
	Cells      []CellInput
}

// ProjectInput is Project's whole input: no hidden file read, no hidden
// clock.
type ProjectInput struct {
	Entries []EntryInput
	// Ledger is the full readings-ledger history (docs/bench/readings/
	// support-ledger.jsonl) Project reads `regressed` from.
	Ledger []LedgerRow
	Commit string
}

// Project builds the matrix: each entry's cells via ComputeState, then
// `regressed` overlaid wherever the ledger shows the capability's target
// was met at an earlier date and the cell computed here does not currently
// meet it. Entries sort by (kind, id); each entry's cells sort by the
// capability's Level rank when it has one, else lexically. now is the
// injected clock design §4 requires (internal/CLAUDE.md: never call
// time.Now() in query/analysis logic).
func Project(in ProjectInput, now func() time.Time) Matrix {
	entries := make([]Entry, 0, len(in.Entries))
	for _, e := range in.Entries {
		cells := make([]Cell, 0, len(e.Cells))
		for _, ci := range e.Cells {
			cell := ComputeState(ci)
			if regressedCapability(in.Ledger, e.Kind, e.ID, ci, cell.State) {
				cell.State = StateRegressed
			}
			cells = append(cells, cell)
		}
		sort.Slice(cells, func(i, j int) bool { return lessCapability(cells[i].Capability, cells[j].Capability) })
		entries = append(entries, Entry{
			Kind:       e.Kind,
			ID:         e.ID,
			Name:       e.Name,
			Registered: e.Registered,
			Wave:       e.Wave,
			Engine:     e.Engine,
			Cells:      cells,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind < entries[j].Kind
		}
		return entries[i].ID < entries[j].ID
	})

	return Matrix{
		Schema:      "fathom.support/v1",
		GeneratedAt: now().UTC().Format(time.RFC3339),
		Commit:      in.Commit,
		Entries:     entries,
	}
}

// regressedCapability reports whether the ledger shows ci's (kind, id,
// capability) met its declared target at some earlier date, while current
// (the state ComputeState just derived from ci) does NOT currently meet it
// (tolerated, having lapsed entirely, or below-target). A cell already
// proven, or one with no target to have regressed from
// (n/a/declared/refused/deferred), is never overridden.
func regressedCapability(ledger []LedgerRow, kind Kind, id string, ci CellInput, current State) bool {
	if current != StateTolerated && current != StateBelowTarget {
		return false
	}
	for _, r := range ledger {
		if r.Kind != kind || r.ID != id || r.Capability != ci.Capability {
			continue
		}
		switch {
		case ci.Target.ProvenOnly:
			if r.Grade != GradeNone {
				return true
			}
		case ci.Target.Level != "":
			if levelRank(r.LevelMet) >= levelRank(ci.Target.Level) {
				return true
			}
		}
	}
	return false
}

// lessCapability orders two capability names: by Level rank when both are
// levels, else lexically — the fixed L1..L4 column order for language/tool,
// alphabetical for every other kind's named capabilities.
func lessCapability(a, b string) bool {
	ra, rb := levelRank(Level(a)), levelRank(Level(b))
	if ra > 0 && rb > 0 {
		return ra < rb
	}
	return a < b
}
