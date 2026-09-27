package console

// ledgerfold.go — the session ledger folded one line at a time (RT-113,
// docs/design/responsible-tenant.md §14: "incremental tail readers keep
// windowed aggregates"). The ledger is append-only and grows a row per
// turn; the console's readers (the shared sessions count, the per-repo and
// windowed session counts, the activity tab's session rows, the sessions
// table) each re-parsed every row on every model rebuild and page view.
// A LedgerFold is what those readers need, kept as each line arrives:
// a compact (session, repo, time) per row and each session's newest
// state — never the rows' bytes. The byte-slice readers (SessionCount,
// DistinctSessionsBy, DistinctSessionsIn, SessionTable, the activity
// rows) fold their bytes through the same Add, so a tail and a whole-file
// read can never count differently.

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// LedgerFold is a session ledger folded by every rule the console's
// readers share. The zero value is not usable; use NewLedgerFold.
type LedgerFold struct {
	// The one dedupe rule (LedgerFold.addRow: one per session id; a row with none counts once on its own): every parseable row's
	// session key, repo label and time, in file order.
	keyIdx  map[string]int32
	keys    []string
	repoIdx map[string]int32
	repos   []string
	rows    []ledgerFoldRow
	// best is the activity tab's per (session, repo) newest row.
	best map[ledgerPair]ledgerBest
	seq  int

	// SessionTable's rule (a ledgerSessionRow decode): each session's
	// last row, in first-seen order, and the table's counts.
	tblLatest map[string]SessionRow
	tblOrder  []string
	tblRows   int
	tblAnon   int
	tblRepos  map[string]bool
	tblArms   map[string]bool
}

type ledgerFoldRow struct {
	key, repo int32
	at        time.Time
	atOK      bool
}

type ledgerPair struct{ key, repo int32 }

type ledgerBest struct {
	rows int
	seq  int
	s    ActivitySession
}

// NewLedgerFold returns an empty fold.
func NewLedgerFold() *LedgerFold {
	return &LedgerFold{
		keyIdx: map[string]int32{}, repoIdx: map[string]int32{}, best: map[ledgerPair]ledgerBest{},
		tblLatest: map[string]SessionRow{}, tblRepos: map[string]bool{}, tblArms: map[string]bool{},
	}
}

// FoldLedger folds ledger bytes (the whole-file readers' entry).
func FoldLedger(data []byte) *LedgerFold {
	f := NewLedgerFold()
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		f.Add(line)
	}
	return f
}

// Add folds one ledger line (without its newline).
func (f *LedgerFold) Add(line []byte) {
	if len(bytes.TrimSpace(line)) == 0 {
		return
	}
	f.addRow(line)
	f.addTableRow(line)
}

func (f *LedgerFold) addRow(line []byte) {
	var row struct {
		SessionID string `json:"session_id"`
		At        string `json:"at"`
		Repo      string `json:"repo"`
	}
	if json.Unmarshal(line, &row) != nil {
		return
	}
	key := row.SessionID
	if key == "" {
		key = "\x00anon:" + row.At + ":" + row.Repo
	}
	k, r := f.intern(f.keyIdx, &f.keys, key), f.intern(f.repoIdx, &f.repos, row.Repo)
	fr := ledgerFoldRow{key: k, repo: r}
	if ts, err := time.Parse(time.RFC3339, row.At); err == nil {
		fr.at, fr.atOK = ts, true
	}
	f.rows = append(f.rows, fr)

	// The activity tab's row: its optional fields decoded leniently (a
	// field that does not decode is not recorded; the row still names its
	// session).
	var a struct {
		SessionID     string          `json:"session_id"`
		Arm           json.RawMessage `json:"arm"`
		At            string          `json:"at"`
		TurnCount     json.RawMessage `json:"turn_count"`
		FirstEditTurn json.RawMessage `json:"first_edit_turn"`
		FathomCalls   json.RawMessage `json:"fathom_calls"`
	}
	_ = json.Unmarshal(line, &a)
	f.seq++
	p := ledgerPair{k, r}
	b := f.best[p]
	if b.rows == 0 {
		b.s.ID = a.SessionID
	}
	b.rows++
	if a.At >= b.s.At {
		var arm string
		_ = json.Unmarshal(a.Arm, &arm)
		b.seq = f.seq
		b.s.At, b.s.Arm, b.s.Turns, b.s.FirstEditTurn, b.s.FathomCalls = a.At, arm, optionalCount(a.TurnCount), optionalCount(a.FirstEditTurn), optionalCount(a.FathomCalls)
	}
	f.best[p] = b
}

func (f *LedgerFold) addTableRow(line []byte) {
	var r ledgerSessionRow
	if json.Unmarshal(line, &r) != nil {
		return
	}
	f.tblRows++
	key := r.SessionID
	if key == "" {
		key = "\x00anon:" + r.At + ":" + r.Repo
		f.tblAnon++
	}
	row, seen := f.tblLatest[key]
	if !seen {
		f.tblOrder = append(f.tblOrder, key)
	}
	row = SessionRow{ID: r.SessionID, Repo: r.Repo, Arm: r.Arm, At: r.At, Rows: row.Rows + 1,
		Turns: r.TurnCount, FirstEditTurn: r.FirstEditTurn, FathomCalls: fathomCalls(r), VerifyCatches: r.VerifyCatches}
	if r.TokensIn != nil && r.TokensOut != nil {
		t := *r.TokensIn + *r.TokensOut
		row.Tokens = &t
	}
	f.tblLatest[key] = row
	if r.Repo != "" {
		f.tblRepos[r.Repo] = true
	}
	if r.Arm != "" {
		f.tblArms[r.Arm] = true
	}
}

func (f *LedgerFold) intern(idx map[string]int32, names *[]string, s string) int32 {
	if i, ok := idx[s]; ok {
		return i
	}
	i := int32(len(*names))
	idx[s] = i
	*names = append(*names, s)
	return i
}

// Clone is an independent copy: folding more into either never changes
// the other (a tail hands a reader a clone and keeps folding).
func (f *LedgerFold) Clone() *LedgerFold {
	c := &LedgerFold{
		keyIdx: make(map[string]int32, len(f.keyIdx)), repoIdx: make(map[string]int32, len(f.repoIdx)),
		best: make(map[ledgerPair]ledgerBest, len(f.best)), tblLatest: make(map[string]SessionRow, len(f.tblLatest)),
		tblRepos: make(map[string]bool, len(f.tblRepos)), tblArms: make(map[string]bool, len(f.tblArms)),
		// Clipped: an append to the clone copies, never writing under f.
		keys: f.keys[:len(f.keys):len(f.keys)], repos: f.repos[:len(f.repos):len(f.repos)],
		rows: f.rows[:len(f.rows):len(f.rows)], tblOrder: f.tblOrder[:len(f.tblOrder):len(f.tblOrder)],
		seq: f.seq, tblRows: f.tblRows, tblAnon: f.tblAnon,
	}
	for k, v := range f.keyIdx {
		c.keyIdx[k] = v
	}
	for k, v := range f.repoIdx {
		c.repoIdx[k] = v
	}
	for k, v := range f.best {
		c.best[k] = v
	}
	for k, v := range f.tblLatest {
		c.tblLatest[k] = v
	}
	for k := range f.tblRepos {
		c.tblRepos[k] = true
	}
	for k := range f.tblArms {
		c.tblArms[k] = true
	}
	return c
}

// Sessions is the distinct sessions and parseable rows (DistinctSessions).
func (f *LedgerFold) Sessions() (sessions, rows int) {
	return len(f.keys), len(f.rows)
}

// DistinctBy is DistinctSessionsBy over the fold.
func (f *LedgerFold) DistinctBy(keyOf func(repoLabel string) string) map[string]int {
	return f.distinct(keyOf, func(ledgerFoldRow) bool { return true })
}

// DistinctIn is DistinctSessionsIn over the fold.
func (f *LedgerFold) DistinctIn(keyOf func(repoLabel string) string, from, to time.Time) map[string]int {
	return f.distinct(keyOf, func(r ledgerFoldRow) bool {
		return r.atOK && !r.at.Before(from) && r.at.Before(to)
	})
}

func (f *LedgerFold) distinct(keyOf func(string) string, keep func(ledgerFoldRow) bool) map[string]int {
	mapped := make([]string, len(f.repos))
	for i, repo := range f.repos {
		if repo != "" {
			mapped[i] = keyOf(repo)
		}
	}
	seen := map[string]map[int32]bool{}
	for _, r := range f.rows {
		repo := mapped[r.repo]
		if repo == "" || !keep(r) {
			continue
		}
		if seen[repo] == nil {
			seen[repo] = map[int32]bool{}
		}
		seen[repo][r.key] = true
	}
	out := make(map[string]int, len(seen))
	for repo, keys := range seen {
		out[repo] = len(keys)
	}
	return out
}

// activitySessions is the activity tab's sessions of service (sessionRows'
// fold): per session, the rows whose repo label maps to service, their
// count and the newest one's fields (a tie: the later row).
func (f *LedgerFold) activitySessions(service string, labelKey func(string) string, limit int) []ActivitySession {
	type acc struct {
		s   ActivitySession
		seq int
	}
	byKey := map[int32]*acc{}
	match := map[int32]bool{}
	for i, repo := range f.repos {
		if repo != "" && labelKey(repo) == service {
			match[int32(i)] = true
		}
	}
	for p, b := range f.best {
		if !match[p.repo] {
			continue
		}
		a := byKey[p.key]
		if a == nil {
			a = &acc{s: b.s, seq: b.seq}
			a.s.Rows = 0
			byKey[p.key] = a
		} else if b.s.At > a.s.At || (b.s.At == a.s.At && b.seq > a.seq) {
			rows, id := a.s.Rows, a.s.ID
			a.s, a.seq = b.s, b.seq
			a.s.Rows, a.s.ID = rows, id
		}
		a.s.Rows += b.rows
	}
	out := make([]ActivitySession, 0, len(byKey))
	for _, a := range byKey {
		out = append(out, a.s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].At != out[j].At {
			return out[i].At > out[j].At
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// table is SessionTable over the fold.
func (f *LedgerFold) table(source string, sf SessionFilter, page, perPage int, m SessionTableModel) SessionTableModel {
	m.Sessions = sessionCountValue(source, len(f.tblOrder), f.tblRows)
	m.LedgerRows = Measured(f.tblRows, source).WithDenominator("parseable ledger rows (the Stop hook appends one per turn)")
	m.Anonymous = Measured(f.tblAnon, source).WithDenominator("rows with no session id, each counted as its own session")
	var rows []SessionRow
	q := strings.ToLower(strings.TrimSpace(sf.Query))
	for _, key := range f.tblOrder {
		r := f.tblLatest[key]
		if sf.Arm != "" && r.Arm != sf.Arm {
			continue
		}
		if sf.Repo != "" && !strings.EqualFold(r.Repo, sf.Repo) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(r.ID), q) && !strings.Contains(strings.ToLower(r.Repo), q) {
			continue
		}
		rows = append(rows, r)
	}
	sortSessions(rows, sf.Sort, sf.Asc)
	matching := len(rows)
	m.Matching = Measured(matching, source).WithDenominator("of " + countNoun(len(f.tblOrder), "session") + " after the filter")
	m.Pages = max(1, (matching+perPage-1)/perPage)
	page = min(max(page, 1), m.Pages)
	m.Page = page
	start := (page - 1) * perPage
	m.Rows = append([]SessionRow{}, rows[start:min(start+perPage, matching)]...)
	m.Repos = sortedKeys(f.tblRepos)
	m.Arms = sortedKeys(f.tblArms)
	return m
}
