package console

// metrics.go — one projection per shared metric (design §1, OC-9). The
// survey found the same number computed three ways on three pages ("≥114"
// on one, "114" on another; 50,747 calls on the fleet page and 88,875 in
// the log; ledger rows counted as sessions). Every page that shows one of
// these metrics now calls the function here, and
// TestConsoleMetricsAgreeAcrossPages renders every page's JSON and fails
// on any two pages that disagree about a metric ID.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// MetricID names a metric shared across pages.
type MetricID string

const (
	MetricServices     MetricID = "fleet.services"
	MetricSymbols      MetricID = "fleet.symbols"
	MetricCalls        MetricID = "calls.total"
	MetricSessions     MetricID = "sessions.total"
	MetricObservations MetricID = "observations.open"
)

// Metrics is a page's shared-metric section, keyed by ID (JSON orders the
// keys, so the section is deterministic).
type Metrics map[MetricID]Value[int]

// ServiceFact is one registered service as the metric projections see it.
type ServiceFact struct {
	Name string
	// Symbols is nil when no store read reported a count; SymbolsNote then
	// says why.
	Symbols     *int
	SymbolsNote string
	// OpenObservations is nil when the repo has no readable observations file.
	OpenObservations *int
}

// FleetFacts is the registry's answer. RegistryNote is non-empty when the
// registry could not be read completely; rows present then are a lower bound.
type FleetFacts struct {
	Services     []ServiceFact
	RegistryNote string
}

const registrySource = "the daemon's service registry (list_services)"

// ServiceCount is how many services the registry lists.
func ServiceCount(f FleetFacts) Value[int] {
	n := len(f.Services)
	if f.RegistryNote == "" {
		return Measured(n, registrySource).WithDenominator("services the registry lists")
	}
	if n == 0 {
		return Absent[int]("registry coverage unavailable: " + f.RegistryNote).WithSource(registrySource)
	}
	return Partial(n, fmt.Sprintf("%d listed services; fleet coverage unavailable: %s", n, f.RegistryNote)).WithSource(registrySource)
}

// SymbolTotal sums the services that reported a symbol count.
func SymbolTotal(f FleetFacts) Value[int] {
	if f.RegistryNote != "" {
		return Absent[int](f.RegistryNote).WithSource(registrySource)
	}
	total, known, firstNote := 0, 0, ""
	for _, s := range f.Services {
		if s.Symbols != nil {
			total += *s.Symbols
			known++
		} else if firstNote == "" {
			firstNote = s.SymbolsNote
		}
	}
	n := len(f.Services)
	switch {
	case n > 0 && known == 0:
		if firstNote == "" {
			firstNote = "no service reported a symbol count"
		}
		return Absent[int](firstNote).WithSource(registrySource)
	case known < n:
		return Partial(total, fmt.Sprintf("across %d of %s — the rest report no symbol count", known, countNoun(n, "repo"))).WithSource(registrySource)
	}
	return Measured(total, registrySource).WithDenominator("across " + countNoun(n, "repo"))
}

// OpenObservations sums open observations across repos with a readable
// .fathom/observations.md: no reporting repo is absent, partial coverage is
// a named lower bound, and only complete coverage permits an exact total.
func OpenObservations(f FleetFacts) Value[int] {
	total, known := 0, 0
	for _, s := range f.Services {
		if s.OpenObservations != nil {
			total += *s.OpenObservations
			known++
		}
	}
	const source = "each repo's .fathom/observations.md"
	n := len(f.Services)
	if known == 0 {
		reason := "no observation counts available"
		if f.RegistryNote != "" {
			reason = f.RegistryNote
		}
		return Absent[int](reason).WithSource(source)
	}
	denom := fmt.Sprintf("%d of %s reporting", known, countNoun(n, "repo"))
	switch {
	case f.RegistryNote != "":
		return Partial(total, fmt.Sprintf("%d of %s reporting; lower bound; fleet coverage unavailable: %s", known, countNoun(n, "listed repo"), f.RegistryNote)).WithSource(source).WithDenominator(denom)
	case known < n:
		return Partial(total, denom+"; lower bound").WithSource(source).WithDenominator(denom)
	}
	return Measured(total, source).WithDenominator(denom)
}

// CallLogFacts is one read of the resolved call log.
type CallLogFacts struct {
	Log          ResolvedCallLog
	Rows         int
	Unattributed int // rows naming no service
	// ReadErr is non-empty when the log exists and could not be read;
	// Missing is true when there is no log at the path yet.
	ReadErr string
	Missing bool
	// AbsentReason, when set, is why no read of the log stands behind this
	// snapshot at all (the model's own read-state note); it wins.
	AbsentReason string
}

// CallTotal is every row in the resolved call log — attributed or not. The
// unattributed rows are part of the total and named in the denominator,
// never silently dropped (the survey's 50,747-vs-88,875 contradiction).
func CallTotal(c CallLogFacts) Value[int] {
	source := c.Log.Path + " (" + string(c.Log.Rule) + ": " + c.Log.Note + ")"
	switch {
	case c.AbsentReason != "":
		return Absent[int](c.AbsentReason).WithSource(source)
	case c.Log.Path == "":
		reason := c.Log.Note
		if reason == "" {
			reason = "no call log was resolved"
		}
		return Absent[int](reason).WithSource("no call log")
	case c.Missing:
		return Absent[int]("no call log at " + c.Log.Path + " — nothing recorded yet; traffic figures are not measured, never zero").WithSource(source)
	case c.ReadErr != "":
		return Errored[int]("call log unreadable: " + c.ReadErr).WithSource(source)
	}
	return Measured(c.Rows, source).WithDenominator("every row in the log; " + thousands(c.Rows-c.Unattributed) + " attributed to a service, " + thousands(c.Unattributed) + " unattributed")
}

// SessionCount is the number of distinct sessions in the session ledger —
// one per session id, never one per row (the Stop hook appends a row at
// every turn boundary). A row with no session id counts once on its own.
func SessionCount(ledgerPath string, data []byte, readErr error) Value[int] {
	source := "session ledger " + ledgerPath
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return Absent[int]("no session ledger at " + ledgerPath + " — sessions are not measured, never zero").WithSource(source)
		}
		return Errored[int]("session ledger unreadable: " + readErr.Error()).WithSource(source)
	}
	sessions, rows := DistinctSessions(data)
	return Measured(sessions, source).WithDenominator("distinct sessions from " + thousands(rows) + " ledger rows")
}

// DistinctSessions counts distinct sessions and non-empty rows in ledger
// bytes, with the dedupe rule every page shares.
func DistinctSessions(data []byte) (sessions, rows int) {
	seen := map[string]bool{}
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var row struct {
			SessionID string `json:"session_id"`
			At        string `json:"at"`
			Repo      string `json:"repo"`
		}
		if json.Unmarshal(line, &row) != nil {
			continue
		}
		rows++
		key := row.SessionID
		if key == "" {
			key = "\x00anon:" + row.At + ":" + row.Repo
		}
		seen[key] = true
	}
	return len(seen), rows
}

// Sources names what the projections read: the resolved call log, the
// ledgers, and the injected clock (design §1).
type Sources struct {
	CallLog       ResolvedCallLog
	SessionLedger string
	VerifyLedger  string
	Now           func() time.Time
}

// Clock is the injected now, defaulting to time.Now.
func (s Sources) Clock() time.Time {
	if s.Now == nil {
		return time.Now()
	}
	return s.Now()
}

// countNoun renders "1 repo" / "3 repos" — the pages' own grammar.
func countNoun(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// thousands renders 34993 as "34,993" — denominators are read by people.
func thousands(n int) string {
	s := fmt.Sprintf("%d", n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}
