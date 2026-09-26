package console

// sessions.go — OC-43 (vision 35 §3 Evidence, design §3 "Sessions", §1
// budgets): a table of sessions and a windowed replay.
//
// The survey found the week page listing one session id eight times
// ("1,053 sessions" were ledger rows) and the replay shipping 9.3 MB of JSON
// for one long session, re-reading the whole beat log on every request.
// SessionTable dedupes with the rule every page shares (the last ledger row
// per session id; a row with no id counts once on its own) and pages the
// rows; SessionWindow never returns more than MaxWindowEvents events in one
// response, whatever the session's length, and pages the rest.

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// SessionRow is one session's last ledger row, as the table shows it. A nil
// figure was not recorded — never a zero.
type SessionRow struct {
	ID            string `json:"session_id"`
	Repo          string `json:"repo,omitempty"`
	Arm           string `json:"arm,omitempty"`
	At            string `json:"at"`
	Rows          int    `json:"ledger_rows"`
	Turns         *int   `json:"turns,omitempty"`
	FirstEditTurn *int   `json:"first_edit_turn,omitempty"`
	Tokens        *int   `json:"tokens_in_out,omitempty"`
	FathomCalls   *int   `json:"fathom_calls,omitempty"`
	VerifyCatches *int   `json:"verify_catches,omitempty"`
}

// SessionFilter narrows and orders the table. Sort is one of the
// SessionSorts keys; the default is newest first.
type SessionFilter struct {
	Arm   string `json:"arm,omitempty"`
	Repo  string `json:"repo,omitempty"`
	Query string `json:"q,omitempty"`
	Sort  string `json:"sort,omitempty"`
	Asc   bool   `json:"asc,omitempty"`
}

// SessionSorts are the columns the table sorts by.
var SessionSorts = []string{"at", "turns", "first_edit", "tokens", "fathom_calls", "catches"}

// SessionTableModel is one page of the session table. Every count carries
// its truth state: with no ledger, or an unreadable one, each is absent or
// an error with its reason — the JSON carries no zero for any of them.
type SessionTableModel struct {
	Filter     SessionFilter `json:"filter"`
	Sessions   Value[int]    `json:"sessions"`       // distinct sessions in the ledger (sessions.total's projection)
	Matching   Value[int]    `json:"matching"`       // after the filter
	LedgerRows Value[int]    `json:"ledger_rows"`    // parseable rows they were deduped from
	Anonymous  Value[int]    `json:"anonymous_rows"` // rows with no session id, each counted once
	Page       int           `json:"page"`
	Pages      int           `json:"pages"`
	PerPage    int           `json:"per_page"`
	Rows       []SessionRow  `json:"rows"`
	Repos      []string      `json:"repos"` // every repo label, for the filter
	Arms       []string      `json:"arms"`
}

type ledgerSessionRow struct {
	SessionID     string `json:"session_id"`
	Repo          string `json:"repo"`
	Arm           string `json:"arm"`
	At            string `json:"at"`
	TurnCount     *int   `json:"turn_count"`
	FirstEditTurn *int   `json:"first_edit_turn"`
	TokensIn      *int   `json:"tokens_in"`
	TokensOut     *int   `json:"tokens_out"`
	FathomCalls   *int   `json:"fathom_calls"`
	VerifyCatches *int   `json:"verify_catches"`
	// The exploration ledger's estimates: when either is present the row
	// was measured, so an omitted fathom_calls (the writer omits a zero)
	// is a measured zero rather than an unrecorded figure.
	RawTokensEst    *int `json:"raw_discovery_tokens_est"`
	FathomTokensEst *int `json:"fathom_tokens_est"`
}

// SessionTable dedupes the ledger bytes into sessions (the last row per id
// wins: the final stop has seen the most of the transcript), filters, sorts
// and returns one page (1-based, clamped; perPage defaults to 50). readErr
// is the ledger read's error: a missing ledger makes every count absent, an
// unreadable one an error — SessionCount's own classification, so the table
// and the shared sessions metric never disagree about the ledger.
func SessionTable(ledgerPath string, ledger []byte, readErr error, f SessionFilter, page, perPage int) SessionTableModel {
	if perPage <= 0 {
		perPage = 50
	}
	m := SessionTableModel{Filter: f, PerPage: perPage, Page: 1, Pages: 1, Rows: []SessionRow{}, Repos: []string{}, Arms: []string{}}
	if readErr != nil {
		v := SessionCount(ledgerPath, nil, readErr)
		m.Sessions, m.Matching, m.LedgerRows, m.Anonymous = v, v, v, v
		return m
	}
	source := "session ledger " + ledgerPath
	var ledgerRows, anonymous int
	latest := map[string]*SessionRow{}
	var order []string
	repos, arms := map[string]bool{}, map[string]bool{}
	for _, line := range bytes.Split(ledger, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var r ledgerSessionRow
		if json.Unmarshal(line, &r) != nil {
			continue
		}
		ledgerRows++
		key := r.SessionID
		if key == "" {
			key = "\x00anon:" + r.At + ":" + r.Repo
			anonymous++
		}
		row, seen := latest[key]
		if !seen {
			row = &SessionRow{}
			latest[key] = row
			order = append(order, key)
		}
		n := row.Rows + 1
		*row = SessionRow{ID: r.SessionID, Repo: r.Repo, Arm: r.Arm, At: r.At, Rows: n,
			Turns: r.TurnCount, FirstEditTurn: r.FirstEditTurn, FathomCalls: fathomCalls(r), VerifyCatches: r.VerifyCatches}
		if r.TokensIn != nil && r.TokensOut != nil {
			t := *r.TokensIn + *r.TokensOut
			row.Tokens = &t
		}
		if r.Repo != "" {
			repos[r.Repo] = true
		}
		if r.Arm != "" {
			arms[r.Arm] = true
		}
	}
	m.Sessions = sessionCountValue(source, len(order), ledgerRows)
	m.LedgerRows = Measured(ledgerRows, source).WithDenominator("parseable ledger rows (the Stop hook appends one per turn)")
	m.Anonymous = Measured(anonymous, source).WithDenominator("rows with no session id, each counted as its own session")
	var rows []SessionRow
	q := strings.ToLower(strings.TrimSpace(f.Query))
	for _, key := range order {
		r := *latest[key]
		if f.Arm != "" && r.Arm != f.Arm {
			continue
		}
		if f.Repo != "" && !strings.EqualFold(r.Repo, f.Repo) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(r.ID), q) && !strings.Contains(strings.ToLower(r.Repo), q) {
			continue
		}
		rows = append(rows, r)
	}
	sortSessions(rows, f.Sort, f.Asc)
	matching := len(rows)
	m.Matching = Measured(matching, source).WithDenominator("of " + countNoun(len(order), "session") + " after the filter")
	m.Pages = max(1, (matching+perPage-1)/perPage)
	page = min(max(page, 1), m.Pages)
	m.Page = page
	start := (page - 1) * perPage
	m.Rows = append([]SessionRow{}, rows[start:min(start+perPage, matching)]...)
	m.Repos = sortedKeys(repos)
	m.Arms = sortedKeys(arms)
	return m
}

// fathomCalls is a row's fathom call count: recorded, a measured zero when
// the exploration ledger measured the row (the writer omits a zero), or
// nil — unrecorded, never zero — for a row that predates it.
func fathomCalls(r ledgerSessionRow) *int {
	if r.FathomCalls != nil {
		return r.FathomCalls
	}
	if r.RawTokensEst != nil || r.FathomTokensEst != nil {
		zero := 0
		return &zero
	}
	return nil
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sortSessions orders rows by key; an unrecorded figure sorts after every
// recorded one in either direction, and ties fall back to newest first,
// then id, so the order is deterministic.
func sortSessions(rows []SessionRow, key string, asc bool) {
	fig := func(r SessionRow) *int {
		switch key {
		case "turns":
			return r.Turns
		case "first_edit":
			return r.FirstEditTurn
		case "tokens":
			return r.Tokens
		case "fathom_calls":
			return r.FathomCalls
		case "catches":
			return r.VerifyCatches
		}
		return nil
	}
	tie := func(a, b SessionRow) bool {
		if a.At != b.At {
			return a.At > b.At
		}
		return a.ID < b.ID
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if key == "" || key == "at" {
			if a.At != b.At {
				if asc {
					return a.At < b.At
				}
				return a.At > b.At
			}
			return a.ID < b.ID
		}
		fa, fb := fig(a), fig(b)
		switch {
		case fa == nil && fb == nil:
			return tie(a, b)
		case fa == nil:
			return false
		case fb == nil:
			return true
		case *fa != *fb:
			if asc {
				return *fa < *fb
			}
			return *fa > *fb
		}
		return tie(a, b)
	})
}

// ReplayEvent is one instrumented event in a session's replay: the beat's
// channel and kind, the service it names, and the rest of its metadata as
// one line (names and counts only; the beat log never carries content).
type ReplayEvent struct {
	At      time.Time `json:"at"`
	Channel string    `json:"channel"`
	Kind    string    `json:"kind,omitempty"`
	Service string    `json:"service,omitempty"`
	Detail  string    `json:"detail,omitempty"`
}

// ChannelCount is how many of a session's events one channel carries.
type ChannelCount struct {
	Channel  string `json:"channel"`
	Events   int    `json:"events"`
	Selected bool   `json:"selected"`
}

// MaxWindowEvents bounds one replay response (design §1: ≤ 2,000 events per
// response, the rest paged); DefaultWindowEvents is a page's default size.
const (
	MaxWindowEvents     = 2000
	DefaultWindowEvents = 500
)

// SessionWindowModel is one window of a session's replay. Events and
// Matching carry their truth state: a hooks log that is missing is absent,
// one that exists and could not be read is an error naming the failure, and
// a session with no beat in a readable log is absent with that reason —
// never a zero standing in for any of them.
type SessionWindowModel struct {
	SessionID string         `json:"session_id"`
	Events    Value[int]     `json:"events"`   // the session's events, all channels
	Matching  Value[int]     `json:"matching"` // after the channel filter
	Channels  []ChannelCount `json:"channels"`
	Page      int            `json:"page"`
	Pages     int            `json:"pages"`
	PerPage   int            `json:"per_page"`
	From      string         `json:"from,omitempty"` // first and last event in this window
	To        string         `json:"to,omitempty"`
	Window    []ReplayEvent  `json:"window"`
}

// ReplaySource is the hooks-log read a replay is built from: the session's
// events, where they were read, and — when the log could not be read — the
// reason (Error: the log exists and failed, rather than being absent).
// Partial names a generation (the rotated hooks.jsonl.1) that could not be
// read while the other was: the counts are then lower bounds.
type ReplaySource struct {
	Events  []ReplayEvent
	Source  string
	Reason  string
	Error   bool
	Partial string
}

// SessionWindow orders a session's events oldest first, keeps the channels
// asked for (none = every channel) and returns one page of at most
// MaxWindowEvents of them (perPage ≤ 0 means DefaultWindowEvents).
func SessionWindow(sessionID string, src ReplaySource, channels []string, page, perPage int) SessionWindowModel {
	switch {
	case perPage <= 0:
		perPage = DefaultWindowEvents
	case perPage > MaxWindowEvents:
		perPage = MaxWindowEvents
	}
	events := src.Events
	m := SessionWindowModel{SessionID: sessionID, PerPage: perPage, Page: 1, Pages: 1, Window: []ReplayEvent{}}
	switch {
	case src.Reason != "" && src.Error:
		m.Events = Errored[int](src.Reason).WithSource(src.Source)
		m.Matching = m.Events
		return m
	case src.Reason != "":
		m.Events = Absent[int](src.Reason).WithSource(src.Source)
		m.Matching = m.Events
		return m
	case len(events) == 0:
		reason := "no beat in the hooks log names this session id — its hooks were not wired, or its beats rotated out of the log's two generations"
		if src.Partial != "" {
			reason = "no beat in the readable part of the hooks log names this session id — " + src.Partial
		}
		m.Events = Absent[int](reason).WithSource(src.Source)
		m.Matching = m.Events
		return m
	}
	const denom = "beats naming this session, every channel"
	if src.Partial != "" {
		m.Events = Partial(len(events), src.Partial).WithSource(src.Source).WithDenominator(denom)
	} else {
		m.Events = Measured(len(events), src.Source).WithDenominator(denom)
	}
	want := map[string]bool{}
	for _, c := range channels {
		if c = strings.TrimSpace(c); c != "" {
			want[c] = true
		}
	}
	counts := map[string]int{}
	sorted := append([]ReplayEvent(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].At.Before(sorted[j].At) })
	var kept []ReplayEvent
	for _, e := range sorted {
		counts[e.Channel]++
		if len(want) == 0 || want[e.Channel] {
			kept = append(kept, e)
		}
	}
	for _, c := range sortedKeys(func() map[string]bool {
		s := map[string]bool{}
		for k := range counts {
			s[k] = true
		}
		return s
	}()) {
		m.Channels = append(m.Channels, ChannelCount{Channel: c, Events: counts[c], Selected: len(want) == 0 || want[c]})
	}
	matching := len(kept)
	if src.Partial != "" {
		m.Matching = Partial(matching, src.Partial).WithSource(src.Source)
	} else {
		m.Matching = Measured(matching, src.Source)
	}
	m.Matching = m.Matching.WithDenominator("after the channel filter, of " + thousands(len(events)) + " events")
	m.Pages = max(1, (matching+perPage-1)/perPage)
	page = min(max(page, 1), m.Pages)
	m.Page = page
	start := (page - 1) * perPage
	m.Window = append([]ReplayEvent{}, kept[start:min(start+perPage, matching)]...)
	if len(m.Window) > 0 {
		m.From = m.Window[0].At.UTC().Format(time.RFC3339)
		m.To = m.Window[len(m.Window)-1].At.UTC().Format(time.RFC3339)
	}
	return m
}
