package console

// project.go — the project workspace's shared metrics (vision 35 §3, design
// §1, OC-9). A repository's symbol count, its calls and its sessions appear
// on more than one of its tabs, so each has exactly one projection here and
// every tab that shows it calls it: the overview and the index tab cannot
// disagree about symbols, nor the overview and the activity tab about calls.

import (
	"fmt"
	"time"
)

// Per-project shared metrics. Every page under /projects/{svc} that shows
// one of these carries it in its JSON "metrics" section, and
// TestConsoleMetricsAgreeAcrossPages holds those pages to one value.
const (
	// MetricProjectSymbols is the symbols the store indexes for the project.
	MetricProjectSymbols MetricID = "project.symbols"
	// MetricProjectCalls is every row of the resolved call log attributed to
	// the project (console page reads excluded).
	MetricProjectCalls MetricID = "project.calls"
	// MetricProjectSessions is the distinct sessions the session ledger
	// records in the project's repository.
	MetricProjectSessions MetricID = "project.sessions"
)

// WorkspaceFacts is one project's row as one read of the registry, the call
// log and the session ledger saw it (the observatory model's row).
type WorkspaceFacts struct {
	Name string
	// Symbols is nil when no store read reported a count; SymbolsNote says why.
	Symbols     *int
	SymbolsNote string
	// Calls is the calls fact for the whole log (CallTotal's input); the
	// project's own count is CallsForProject.
	Calls           CallLogFacts
	CallsForProject int
	// Sessions is nil when the ledger was not read; SessionsState then
	// carries the ledger's own truth state (absent or error) with its reason.
	Sessions      *int
	SessionsState Value[int]
	AsOf          time.Time
}

// ProjectSymbols is the project's symbol count: measured when the store
// reported one, absent with the store's reason otherwise — never a 0.
func ProjectSymbols(f WorkspaceFacts) Value[int] {
	if f.Symbols == nil {
		reason := f.SymbolsNote
		if reason == "" {
			reason = "no store read reported a symbol count for " + f.Name
		}
		return Absent[int](reason).WithSource(registrySource)
	}
	return Measured(*f.Symbols, registrySource).WithDenominator("symbols the store indexes for " + f.Name)
}

// ProjectCalls is the rows of the resolved call log attributed to the
// project. The log's own truth state wins: a missing log is absent and an
// unreadable one is an error, exactly as the fleet total says — a project
// never reads "0 calls" from a log nobody could read.
func ProjectCalls(f WorkspaceFacts) Value[int] {
	whole := CallTotal(f.Calls)
	if !whole.Known() {
		return whole
	}
	denom := fmt.Sprintf("rows in the call log attributed to %s, of %s in the log", f.Name, thousands(f.Calls.Rows))
	if f.Calls.ConsoleReads > 0 {
		denom += "; console page reads excluded (not usage)"
	}
	v := Measured(f.CallsForProject, whole.Source).WithDenominator(denom)
	if !f.AsOf.IsZero() {
		v = v.WithAsOf(f.AsOf)
	}
	return v
}

// ProjectSessions is the distinct sessions the ledger records in the
// project's repository (a row's repo label mapped to the service whose root
// it names). An unread ledger keeps the ledger's own state and reason.
func ProjectSessions(f WorkspaceFacts) Value[int] {
	if f.Sessions == nil {
		if f.SessionsState.State == StateError || f.SessionsState.State == StateAbsent {
			return Value[int]{State: f.SessionsState.State, Reason: f.SessionsState.Reason, Source: f.SessionsState.Source}
		}
		return Absent[int]("the session ledger was not read for this snapshot")
	}
	v := Measured(*f.Sessions, f.SessionsState.Source).WithDenominator("distinct sessions whose repo label names " + f.Name)
	if !f.AsOf.IsZero() {
		v = v.WithAsOf(f.AsOf)
	}
	return v
}

// WorkspaceMetrics is every per-project shared metric for one project.
func WorkspaceMetrics(f WorkspaceFacts) Metrics {
	return Metrics{
		MetricProjectSymbols:  ProjectSymbols(f),
		MetricProjectCalls:    ProjectCalls(f),
		MetricProjectSessions: ProjectSessions(f),
	}
}
