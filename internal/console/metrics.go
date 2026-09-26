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
	// MetricAgentTools is how many tools the agent surface serves — the
	// brief's "standing cost" and the home page's context tax read it, and
	// the brief once printed a literal "16-tool" beside it (F0 proof).
	MetricAgentTools MetricID = "surface.agent_tools"
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
	// OpenObservations is nil when the repo reports no count: no
	// observations file, or one that could not be read (ObservationsErr).
	OpenObservations *int
	// ObservationsErr is non-empty when the repo's observations file exists
	// and could not be read — an error, never "not reporting".
	ObservationsErr string
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
// a named lower bound, and only complete coverage permits an exact total. A
// file that exists and could not be read makes the value an ERROR naming
// the repos and the failure: a total without it would read as fewer
// defects, which is the one direction an instrument may never degrade in
// (I38, ADR-027) — the survey watched an EPERM wedge turn "≥ 100" into
// "≥ 8" with nothing named.
func OpenObservations(f FleetFacts) Value[int] {
	total, known := 0, 0
	var unreadable []string
	for _, s := range f.Services {
		switch {
		case s.ObservationsErr != "":
			unreadable = append(unreadable, s.Name+" ("+s.ObservationsErr+")")
		case s.OpenObservations != nil:
			total += *s.OpenObservations
			known++
		}
	}
	const source = "each repo's .fathom/observations.md"
	n := len(f.Services)
	if len(unreadable) > 0 {
		shown := unreadable
		more := ""
		if len(shown) > 3 {
			shown, more = shown[:3], fmt.Sprintf(" and %d more", len(unreadable)-3)
		}
		return Errored[int](fmt.Sprintf("observations file unreadable in %d of %s: %s%s — no total is shown, since one without them would read as fewer defects", len(unreadable), countNoun(n, "repo"), strings.Join(shown, "; "), more)).WithSource(source)
	}
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
	// ConsoleReads is how many rows the console stamped with its own origin
	// (the Observatory reading the registry to render a page): logged,
	// never counted as usage, named in the denominator.
	ConsoleReads int
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
	denom := "every row in the log; " + thousands(c.Rows-c.Unattributed) + " attributed to a service, " + thousands(c.Unattributed) + " unattributed"
	if c.ConsoleReads > 0 {
		denom += "; " + thousands(c.ConsoleReads) + " console page reads excluded (not usage)"
	}
	return Measured(c.Rows, source).WithDenominator(denom)
}

// AgentSurfaceTools is how many tools the agent surface serves, from one
// tools/list on /rpc/agent: tools and schemaBytes are what it returned. A
// surface that did not answer (both zero) is absent — never a 0-tool
// surface.
func AgentSurfaceTools(tools, schemaBytes int) Value[int] {
	const source = "tools/list on the daemon's /rpc/agent surface"
	if tools == 0 && schemaBytes == 0 {
		return Absent[int]("the agent surface did not answer tools/list").WithSource(source)
	}
	return Measured(tools, source).WithDenominator(thousands(schemaBytes) + " bytes of tool schema every connected session carries")
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
	eachLedgerSession(data, func(key, _ string) {
		rows++
		seen[key] = true
	})
	return len(seen), rows
}

// DistinctSessionsByRepo counts distinct sessions per repo label (lowered)
// with the same dedupe rule: the fleet report's and the fleet page's
// per-repo session column, which counted ledger rows (ADR-045's named
// contradiction) until it shared this rule. Rows naming no repo are skipped.
func DistinctSessionsByRepo(data []byte) map[string]int {
	return DistinctSessionsBy(data, strings.ToLower)
}

// DistinctSessionsBy is DistinctSessionsByRepo with the row's repo label
// mapped to a key first (a label to the service whose root it names: the
// Stop hook labels this repository "sextant", its service is "fathom"), so a
// session counts once under its key. A label mapped to "" is skipped.
func DistinctSessionsBy(data []byte, keyOf func(repoLabel string) string) map[string]int {
	seen := map[string]map[string]bool{}
	eachLedgerSession(data, func(key, repo string) {
		if repo == "" {
			return
		}
		repo = keyOf(repo)
		if repo == "" {
			return
		}
		if seen[repo] == nil {
			seen[repo] = map[string]bool{}
		}
		seen[repo][key] = true
	})
	out := make(map[string]int, len(seen))
	for repo, keys := range seen {
		out[repo] = len(keys)
	}
	return out
}

// DistinctSessionsIn is DistinctSessionsBy over one window: a session
// counts under its key when any of its rows was written in [from, to). A
// row with no parseable timestamp cannot be placed in a window and is not
// counted. The realization ladder's idle rule reads it ("no sessions in the
// window", docs/design/realization-grade.md §3) — an all-time count would
// grade a repository busy in July and silent since as active.
func DistinctSessionsIn(data []byte, keyOf func(repoLabel string) string, from, to time.Time) map[string]int {
	seen := map[string]map[string]bool{}
	eachLedgerSessionAt(data, func(key, repo, at string) {
		if repo == "" {
			return
		}
		ts, err := time.Parse(time.RFC3339, at)
		if err != nil || ts.Before(from) || !ts.Before(to) {
			return
		}
		repo = keyOf(repo)
		if repo == "" {
			return
		}
		if seen[repo] == nil {
			seen[repo] = map[string]bool{}
		}
		seen[repo][key] = true
	})
	out := make(map[string]int, len(seen))
	for repo, keys := range seen {
		out[repo] = len(keys)
	}
	return out
}

// VerifyRunsIn counts the verify gate's runs in [from, to) per service key
// from the verify ledger (SD-1). One `fathom verify` invocation writes one
// row per plan, every row stamped with the same instant, so a run is one
// (service, instant) — never a plan row. A row with no timestamp is not
// counted. keyOf maps the ledger's service name to the caller's key ("" is
// skipped).
func VerifyRunsIn(runs []LedgerRun, keyOf func(service string) string, from, to time.Time) map[string]int {
	seen := map[string]map[int64]bool{}
	for _, r := range runs {
		if r.At.IsZero() || r.At.Before(from) || !r.At.Before(to) {
			continue
		}
		k := keyOf(r.Service)
		if k == "" {
			continue
		}
		if seen[k] == nil {
			seen[k] = map[int64]bool{}
		}
		seen[k][r.At.Unix()] = true
	}
	out := make(map[string]int, len(seen))
	for k, at := range seen {
		out[k] = len(at)
	}
	return out
}

// eachLedgerSession calls fn with every parseable ledger row's session key
// (its session id; a row with none counts once on its own) and repo label.
func eachLedgerSession(data []byte, fn func(key, repo string)) {
	eachLedgerSessionAt(data, func(key, repo, _ string) { fn(key, repo) })
}

// eachLedgerSessionAt is eachLedgerSession with the row's timestamp.
func eachLedgerSessionAt(data []byte, fn func(key, repo, at string)) {
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
		key := row.SessionID
		if key == "" {
			key = "\x00anon:" + row.At + ":" + row.Repo
		}
		fn(key, row.Repo, row.At)
	}
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
