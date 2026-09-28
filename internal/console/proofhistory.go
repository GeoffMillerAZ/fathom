package console

// proofhistory.go — PE-2 (docs/design/progress-and-economics.md §3, §7):
// the proof history is a projection of the verify ledger, never a second
// ledger (PE-X3). Schema-3 readings carry a plan's per-row verdicts when
// its row digest changed; this folds them into each row's state over time
// (changes only), keeps every reading's plan-level counts (v1 and v2 rows
// contribute those and nothing else — nothing is backfilled), and is read
// through a bounded tail cached in a memo group keyed by the ledger's size
// and modification time (I51). Every Progress view projects from it.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/memo"
)

// The schema-3 row states, and HistAbsent for a row a later reading of its
// plan no longer carries (the row was deleted or renamed).
const (
	HistOpen          = "open"
	HistProven        = "claimed-proven"
	HistUnproven      = "claimed-unproven"
	HistUnknown       = "unknown"
	HistDispositioned = "dispositioned"
	HistAbsent        = "absent"
)

// RowReading is one row at a reading where its state, criterion or weight
// changed.
type RowReading struct {
	At        string `json:"at"`
	Commit    string `json:"commit,omitempty"`
	Origin    string `json:"origin,omitempty"`
	State     string `json:"state"`
	Criterion string `json:"criterion,omitempty"`
	Weight    int    `json:"weight"`
	// First marks the first reading of the row's plan in the history: a
	// row seen there existed before the history began.
	First bool `json:"first_of_plan,omitempty"`
}

// RowHistory is one plan row's state over time, oldest first.
type RowHistory struct {
	Service  string       `json:"service"`
	Plan     string       `json:"plan"`
	ID       string       `json:"id"`
	Readings []RowReading `json:"readings"`
}

// Key is the row's identity across the fleet.
func (h RowHistory) Key() string { return RowKey(h.Service, h.Plan, h.ID) }

// RowKey keys a row by service, plan and id.
func RowKey(service, plan, id string) string {
	return strings.ToLower(service) + "|" + plan + "|" + id
}

// StateAt is the row's reading in force at instant at (RFC 3339): the
// newest reading no later than it.
func (h RowHistory) StateAt(at string) (RowReading, bool) {
	var out RowReading
	ok := false
	for _, r := range h.Readings {
		if r.At > at {
			break
		}
		out, ok = r, true
	}
	return out, ok
}

// Latest is the row's newest reading.
func (h RowHistory) Latest() RowReading {
	if len(h.Readings) == 0 {
		return RowReading{}
	}
	return h.Readings[len(h.Readings)-1]
}

// PlanReading is one ledger reading's plan-level counts (any schema).
type PlanReading struct {
	At       string `json:"at"`
	Root     string `json:"repo_root,omitempty"`
	Service  string `json:"service,omitempty"`
	Plan     string `json:"plan"`
	Commit   string `json:"commit,omitempty"`
	Origin   string `json:"origin,omitempty"`
	Claimed  int    `json:"claimed"`
	Verified int    `json:"verified"`
}

// Unproven is the reading's claimed rows the gate did not prove.
func (r PlanReading) Unproven() int {
	if n := r.Claimed - r.Verified; n > 0 {
		return n
	}
	return 0
}

// History is the projection.
type History struct {
	Rows     []RowHistory  `json:"rows"`
	Readings []PlanReading `json:"-"`
	// RowReadings carried a rows array; Unchanged were schema-3 readings
	// whose digest had not changed (no array); CountOnly were v1/v2.
	RowReadings int `json:"row_readings"`
	Unchanged   int `json:"unchanged_readings"`
	CountOnly   int `json:"count_only_readings"`
	Malformed   int `json:"malformed_lines"`
	// Truncated says the bounded tail did not reach the ledger's start:
	// the history begins at the tail's first whole line.
	Truncated bool   `json:"truncated,omitempty"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
}

type historyLine struct {
	SchemaVersion int    `json:"schema_version"`
	At            string `json:"at"`
	RepoRoot      string `json:"repo_root"`
	Service       string `json:"service"`
	Plan          string `json:"plan"`
	Commit        string `json:"commit"`
	Origin        string `json:"origin"`
	Claimed       int    `json:"claimed"`
	Verified      int    `json:"verified"`
	Digest        string `json:"digest"`
	Rows          []struct {
		ID        string `json:"id"`
		State     string `json:"state"`
		Criterion string `json:"criterion"`
		Weight    int    `json:"weight"`
	} `json:"rows"`
}

// ParseProofHistory folds ledger bytes into the history. truncated is the
// caller's word that data is a tail, not the whole ledger.
func ParseProofHistory(data []byte, truncated bool) History {
	h := History{Truncated: truncated}
	var lines []historyLine
	for _, raw := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		var l historyLine
		if err := json.Unmarshal(raw, &l); err != nil || l.Plan == "" || l.At == "" {
			h.Malformed++
			continue
		}
		lines = append(lines, l)
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].At < lines[j].At })

	type planState struct {
		seen bool
		rows map[string]RowReading
	}
	plans := map[string]*planState{}
	rows := map[string]*RowHistory{}
	for _, l := range lines {
		plan := RelPlan(l.RepoRoot, l.Plan)
		h.Readings = append(h.Readings, PlanReading{At: l.At, Root: l.RepoRoot, Service: l.Service, Plan: plan,
			Commit: l.Commit, Origin: l.Origin, Claimed: l.Claimed, Verified: l.Verified})
		switch {
		case l.SchemaVersion < 3 || l.Digest == "":
			h.CountOnly++
			continue
		case l.Rows == nil:
			h.Unchanged++
			continue
		}
		h.RowReadings++
		pk := strings.ToLower(l.Service) + "|" + plan
		ps := plans[pk]
		if ps == nil {
			ps = &planState{rows: map[string]RowReading{}}
			plans[pk] = ps
		}
		first := !ps.seen
		ps.seen = true
		present := map[string]bool{}
		for _, r := range l.Rows {
			w := r.Weight
			if w < 1 {
				w = 1
			}
			rd := RowReading{At: l.At, Commit: l.Commit, Origin: l.Origin, State: r.State, Criterion: r.Criterion, Weight: w, First: first}
			present[r.ID] = true
			if prev, ok := ps.rows[r.ID]; ok && prev.State == rd.State && prev.Criterion == rd.Criterion && prev.Weight == rd.Weight {
				continue
			}
			ps.rows[r.ID] = rd
			rh := rows[RowKey(l.Service, plan, r.ID)]
			if rh == nil {
				rh = &RowHistory{Service: l.Service, Plan: plan, ID: r.ID}
				rows[RowKey(l.Service, plan, r.ID)] = rh
			}
			rh.Readings = append(rh.Readings, rd)
		}
		for id, prev := range ps.rows {
			if present[id] || prev.State == HistAbsent {
				continue
			}
			gone := RowReading{At: l.At, Commit: l.Commit, Origin: l.Origin, State: HistAbsent, Weight: prev.Weight}
			ps.rows[id] = gone
			rh := rows[RowKey(l.Service, plan, id)]
			rh.Readings = append(rh.Readings, gone)
		}
	}
	for _, rh := range rows {
		h.Rows = append(h.Rows, *rh)
	}
	sort.Slice(h.Rows, func(i, j int) bool {
		a, b := h.Rows[i], h.Rows[j]
		if a.Service != b.Service {
			return a.Service < b.Service
		}
		if a.Plan != b.Plan {
			return a.Plan < b.Plan
		}
		return a.ID < b.ID
	})
	if len(lines) > 0 {
		h.From, h.To = lines[0].At, lines[len(lines)-1].At
	}
	return h
}

// DefaultHistoryTail is the bytes of the ledger a view reads by default:
// the newest readings, about 30,000 of them.
const DefaultHistoryTail = 16 << 20

type historyKey struct {
	path  string
	size  int64
	mtime int64
	tail  int64
}

// historyGroup caches projections keyed by the ledger's size and mtime: an
// appended reading is a new key, and the old entry ages out by LRU.
var historyGroup = memo.NewGroup[historyKey, History]("console.proof-history", memo.Options{MaxEntries: 4})

// ProofHistory reads the newest tail bytes of the verify ledger at path
// and projects them, naming how the read went. An absent ledger is absent
// (no gate has run here), an unreadable one an error — never an empty
// history.
func ProofHistory(ctx context.Context, path string, tail int64) (History, SourceRead) {
	read := SourceRead{Name: SourceVerifyLedger, Path: path}
	if path == "" {
		read.State, read.Reason = StateAbsent, "no home directory to find the verify ledger in"
		return History{}, read
	}
	fi, err := os.Stat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		read.State, read.Reason = StateAbsent, "no verify ledger at "+path+" — no `fathom verify` run is recorded"
		return History{}, read
	case err != nil:
		read.State, read.Reason = StateError, err.Error()
		return History{}, read
	}
	if tail <= 0 {
		tail = DefaultHistoryTail
	}
	key := historyKey{path: filepath.Clean(path), size: fi.Size(), mtime: fi.ModTime().UnixNano(), tail: tail}
	h, err := historyGroup.Get(ctx, key, func(context.Context) (History, int64, error) {
		data, truncated, err := readTail(path, fi.Size(), tail)
		if err != nil {
			return History{}, 0, err
		}
		return ParseProofHistory(data, truncated), int64(len(data)), nil
	})
	if err != nil {
		read.State, read.Reason = StateError, err.Error()
		return History{}, read
	}
	read.State = StatePresent
	var notes []string
	if h.Truncated {
		notes = append(notes, "the newest "+fmt.Sprint(tail>>20)+" MiB read; the history begins "+h.From)
	}
	if h.Malformed > 0 {
		notes = append(notes, fmt.Sprintf("%d malformed lines skipped", h.Malformed))
	}
	read.Reason = strings.Join(notes, "; ")
	return h, read
}

// readTail reads the last tail bytes of a file of the given size, dropping
// a partial first line.
func readTail(path string, size, tail int64) ([]byte, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	start := size - tail
	truncated := start > 0
	if start < 0 {
		start = 0
	}
	buf := make([]byte, size-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return nil, false, err
	}
	if truncated {
		if i := bytes.IndexByte(buf, '\n'); i >= 0 {
			buf = buf[i+1:]
		}
	}
	return buf, truncated, nil
}

// HistoryFilter narrows every Progress view (design §4): service, plan,
// wave (a plan section label, resolved by the caller into WaveRows), goal
// (resolved into GoalPlans by GoalFilter), agent identity (harness or
// harness/model; claim honesty only) and a date range (YYYY-MM-DD,
// inclusive; the views read the state as of To's end).
type HistoryFilter struct {
	Service   string          `json:"service,omitempty"`
	Plan      string          `json:"plan,omitempty"`
	Wave      string          `json:"wave,omitempty"`
	Goal      string          `json:"goal,omitempty"`
	Agent     string          `json:"agent,omitempty"`
	From      string          `json:"from,omitempty"`
	To        string          `json:"to,omitempty"`
	WaveRows  map[string]bool `json:"-"`
	GoalPlans []string        `json:"-"`
}

func (f HistoryFilter) keepPlan(service, plan string) bool {
	if f.Service != "" && !strings.EqualFold(f.Service, service) {
		return false
	}
	if f.Plan != "" && f.Plan != plan && path.Base(f.Plan) != path.Base(plan) {
		return false
	}
	if len(f.GoalPlans) > 0 {
		ok := false
		for _, p := range f.GoalPlans {
			if p == plan || path.Base(p) == path.Base(plan) {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func (f HistoryFilter) keepRow(h RowHistory) bool {
	if !f.keepPlan(h.Service, h.Plan) {
		return false
	}
	if f.Wave != "" && f.WaveRows != nil && !f.WaveRows[h.Key()] {
		return false
	}
	return true
}

// inRange reports whether instant at falls inside the filter's dates.
func (f HistoryFilter) inRange(at string) bool {
	d := dayOf(at)
	return (f.From == "" || d >= f.From) && (f.To == "" || d <= f.To)
}

// asOf is the instant the views read state at: the end of To, else the
// end of time.
func (f HistoryFilter) asOf() string {
	if f.To == "" {
		return "9999"
	}
	return f.To + "T23:59:59Z"
}

// rows returns the filtered row histories, each cut at the filter's To.
func (f HistoryFilter) rows(h History) []RowHistory {
	out := make([]RowHistory, 0, len(h.Rows))
	cut := f.asOf()
	for _, r := range h.Rows {
		if !f.keepRow(r) {
			continue
		}
		n := sort.Search(len(r.Readings), func(i int) bool { return r.Readings[i].At > cut })
		if n == 0 {
			continue
		}
		r.Readings = r.Readings[:n]
		out = append(out, r)
	}
	return out
}
