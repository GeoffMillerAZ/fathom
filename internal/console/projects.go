package console

// projects.go — the facts the Now and Fleet pages share (vision 35 §3, plan
// section B): one row per registered project, gathered once from the
// daemon's registry, the observatory model's call-log and ledger folds, the
// verify ledger and the daemon's store, then projected by AttentionQueue,
// FleetTable and the shared metrics below. Gathering is cmd/fathom's job
// (it owns the readers); everything here is pure over the facts.

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// SourceRead is one input a projection read and how the read went: present
// (read), absent (nothing there to read) or error (there, and unreadable).
// A projection lists its sources so a reader can see what a figure rests on
// — and a source that could not be read makes the figure a named lower
// bound, never a quietly smaller one (I38).
type SourceRead struct {
	Name   string `json:"name"`
	Path   string `json:"path,omitempty"`
	State  State  `json:"state"`
	Reason string `json:"reason,omitempty"`
}

// The sources the project projections name.
const (
	SourceRegistry     = "service registry"
	SourceVerifyLedger = "verify ledger"
	SourceStore        = "daemon store"
	SourceObservations = "observations files"
	// SourceCertTargets is each project's certification targets: the
	// store's declarations and the repo's intent corpus (certify status).
	SourceCertTargets = "certification targets"
	// SourcePlanFiles is each project's docs/plans directory.
	SourcePlanFiles = "plan files"
)

// ClaimCounts are one project's governed claims by status.
type ClaimCounts struct {
	Total        int `json:"total"`
	Fresh        int `json:"fresh"`
	Stale        int `json:"stale"`
	Superseded   int `json:"superseded"`
	Contradicted int `json:"contradicted"`
	Refuted      int `json:"refuted"`
	Other        int `json:"other"` // contradicted, refuted, and any other status
}

// CertCounts are one project's certification targets (certify status):
// the counts, the case-to-symbol join, and every target source that
// exists and could not be read (the counts are then a lower bound).
type CertCounts struct {
	Declared    int      `json:"declared"`
	Live        int      `json:"live"`
	Gaps        int      `json:"gaps"`
	Cases       int      `json:"cases"`
	JoinedCases int      `json:"joined_cases"`
	Unreadable  []string `json:"unreadable,omitempty"`
	// Err is the report's own failure, when certify could not make one.
	Err string `json:"error,omitempty"`
}

// GuideCoverage is the obligations layer's latest compile as it bears on
// the project's governing guides (CLAUDE.md, AGENTS.md, nested or root).
// It carries no "sections compiled" count: a run's own count is the
// sections that missed the digest gate, not the corpus it covers.
type GuideCoverage struct {
	SectionsTotal int    `json:"sections_total"`
	BudgetStopped bool   `json:"budget_stopped,omitempty"`
	RanAt         string `json:"ran_at,omitempty"`
	// Uncompiled are the governing guides among the run's uncompiled scopes
	// (the tail it never reached, or sections the pre-filter skipped):
	// served by the raw fallback, not as compiled rules.
	Uncompiled []string `json:"uncompiled,omitempty"`
}

// ProjectFacts is one registered project as the console reads it. A fact
// that was not read carries its state and reason (Value), never a zero.
type ProjectFacts struct {
	Service string `json:"service"`
	Path    string `json:"path,omitempty"`
	// Symbols is nil when no store read reported a count; SymbolsNote says why.
	Symbols     *int     `json:"symbols,omitempty"`
	SymbolsNote string   `json:"symbols_note,omitempty"`
	Languages   []string `json:"languages,omitempty"`
	// Freshness is "fresh", "stale: …" or "unknown: …" (fleetFreshness).
	Freshness     string `json:"freshness"`
	IndexedCommit string `json:"indexed_commit,omitempty"`
	RepoHead      string `json:"repo_head,omitempty"`
	// RefreshRemedy re-indexes the project through the daemon that owns the
	// store (never a shell ingest).
	RefreshRemedy string `json:"refresh_remedy,omitempty"`
	// TypedEdgesFailed is the typed call-edge pass's failure reason, "" when
	// it did not fail (or never ran here).
	TypedEdgesFailed string `json:"typed_edges_failed,omitempty"`
	Rungs            []Rung `json:"rungs,omitempty"`
	Grade            string `json:"grade,omitempty"`
	// Realized and Available are the ladder's fold (rungs met of rungs
	// that apply) and Sessions the sessions in the window: the fleet's
	// default order ranks by unrealized value, (Available − Realized) ×
	// Sessions (RG-4, "where should I spend the next ten minutes").
	Realized  int  `json:"realized"`
	Available int  `json:"available"`
	Sessions  int  `json:"sessions_in_window"`
	HasPlans  bool `json:"has_plans"`
	// PlanFiles are the active plans under docs/plans, relative to the
	// root; PlansErr is set when that directory exists and could not be
	// read (the proof state is then unknown, never "no plans").
	PlanFiles []string `json:"plan_files,omitempty"`
	PlansErr  string   `json:"plans_error,omitempty"`
	// Runs is each plan's proof (PlanRuns): the newest run at HEAD from
	// any clone of the project, else the registered root's newest, stale.
	Runs  []VerifyRun          `json:"runs,omitempty"`
	Claim Value[ClaimCounts]   `json:"claims"`
	Certs Value[CertCounts]    `json:"certificates"`
	Guide Value[GuideCoverage] `json:"guides"`
	// HasGuide: the root carries a governing guide the obligations layer
	// compiles (a CLAUDE.md or AGENTS.md).
	HasGuide   bool   `json:"has_guide"`
	CertRemedy string `json:"cert_remedy,omitempty"`
	// Traffic is calls per window day, aligned with Projects.Days; nil
	// when the call log was not read.
	Traffic          []int  `json:"traffic,omitempty"`
	OpenObservations *int   `json:"open_observations,omitempty"`
	ObservationsErr  string `json:"observations_error,omitempty"`
}

// Projects is every registered project plus the reads the set rests on.
type Projects struct {
	Rows []ProjectFacts
	// Days is the traffic window, oldest first; Logged[i] says whether the
	// call log holds any row at all on Days[i] — a day it holds none is
	// absent for every project, never a measured zero.
	Days   []string
	Logged []bool
	// CallLogNote is non-empty when the call log was not read: no traffic
	// figure exists, and every one says so.
	CallLogNote string
	// RegistryNote is non-empty when the registry could not be read: Rows
	// is then empty or a lower bound.
	RegistryNote string
	Ledger       SourceRead // the verify ledger
	Store        SourceRead // the daemon's store (claims, certificates, compile runs)
	Now          time.Time
}

// Shared metrics the Now and Fleet pages show from the project set (OC-9:
// one projection per metric; TestConsoleMetricsAgreeAcrossPages holds every
// page to it). Claims and certificates are the same projections the
// Knowledge and Certifications pages read (OC-H3); the plans headline is
// the Proof ledger's own (PlanPortfolio), computed in cmd/fathom.
const MetricWiredFresh MetricID = "fleet.wired_fresh"

// ProjectMetrics is every shared metric projected from the project set.
func ProjectMetrics(p Projects) Metrics {
	m := Metrics{MetricWiredFresh: WiredAndFresh(p)}
	if v, ok := storeUnread[int](p.Store, storeClaimsSource); ok {
		m[MetricClaims] = v
	} else {
		var facts []ClaimStatusFact
		for _, r := range p.Rows {
			if f, ok := ClaimFactFromCounts(r.Service, r.Claim); ok {
				facts = append(facts, f)
			}
		}
		m[MetricClaims] = ClaimsCurrent(facts)
	}
	if v, ok := storeUnread[int](p.Store, certifySource); ok {
		m[MetricCertLive], m[MetricCertGaps] = v, v
	} else {
		var facts []CertServiceFacts
		for _, r := range p.Rows {
			if f, ok := CertFactsFromCounts(r.Service, r.Certs); ok {
				facts = append(facts, f)
			}
		}
		_, m[MetricCertLive], m[MetricCertGaps] = CertifyTotals(facts)
	}
	return m
}

const registryRungsSource = "the observatory model's realization ladder (wired, fresh) over the daemon's registry"

// WiredAndFresh counts projects whose wired and fresh rungs are both met.
// A project whose freshness cannot be determined might be either, so any
// unknown makes the count a named lower bound.
func WiredAndFresh(p Projects) Value[int] {
	if p.RegistryNote != "" && len(p.Rows) == 0 {
		return Absent[int]("registry unavailable: " + p.RegistryNote).WithSource(registryRungsSource)
	}
	n, stale, unknown, unwired := 0, 0, 0, 0
	for _, r := range p.Rows {
		wired := rungState(r.Rungs, "wired") == RungMet
		if !wired {
			unwired++
		}
		switch freshnessClass(r.Freshness) {
		case "fresh":
			if wired {
				n++
			}
		case "stale":
			stale++
		default:
			unknown++
		}
	}
	denom := fmt.Sprintf("of %s listed · %d stale · %d freshness unknown · %d not wired", countNoun(len(p.Rows), "project"), stale, unknown, unwired)
	switch {
	case p.RegistryNote != "":
		return Partial(n, "registry read incompletely: "+p.RegistryNote).WithSource(registryRungsSource).WithDenominator(denom)
	case unknown > 0:
		return Partial(n, fmt.Sprintf("freshness unknown for %d of %s; lower bound", unknown, countNoun(len(p.Rows), "project"))).WithSource(registryRungsSource).WithDenominator(denom)
	}
	return Measured(n, registryRungsSource).WithDenominator(denom)
}

// verifyLedgerSource names what a project's recorded proof is read from.
const verifyLedgerSource = "the verify ledger (each plan's newest run at HEAD — at the registered root or in any clone — else the root's newest, stale)"

const storeClaimsSource = "the daemon's store (semantic_claims)"

// storeCertsSource names what a project's certificate counts are read from.
const storeCertsSource = "certify status over the daemon's store and each repo's intent corpus"

// storeUnread is the store-wide absence or error every store metric shares.
func storeUnread[T any](s SourceRead, source string) (Value[T], bool) {
	switch s.State {
	case StateError:
		return Errored[T]("the daemon's store could not be read: " + s.Reason).WithSource(source), true
	case StateAbsent:
		return Absent[T](nonEmpty(s.Reason, "no store was resolved")).WithSource(source), true
	}
	return Value[T]{}, false
}

// rungState is the named rung's state (ladder.go's Rung states), unknown
// when the ladder lacks it.
func rungState(rungs []Rung, name string) string {
	for _, r := range rungs {
		if r.Name == name {
			return r.State
		}
	}
	return RungUnknown
}

// FreshnessClass folds a freshness reading ("fresh", "stale: …",
// "unknown: …") to fresh, stale or unknown.
func FreshnessClass(f string) string { return freshnessClass(f) }

// freshnessClass folds a freshness string to fresh, stale or unknown.
func freshnessClass(f string) string {
	switch {
	case f == "fresh":
		return "fresh"
	case f == "stale" || strings.HasPrefix(f, "stale"):
		return "stale"
	}
	return "unknown"
}

// ProjectHref is a project's workspace route, or one of its tabs (design
// §3: each tab is a URL). The workspace's own tabs are routes under it
// (/projects/{svc}/index, console wave E); governance, plans, proof and
// refactor open the Knowledge and Proof pages filtered to the project —
// the one mapping the workspace's tab bar (cmd/fathom projectTabList) and
// every link into a tab share.
func ProjectHref(service, tab string) string {
	q := url.QueryEscape(service)
	switch tab {
	case "":
		return "/projects/" + url.PathEscape(service)
	case "governance":
		return "/layers?svc=" + q
	case "plans":
		return "/plans?svc=" + q
	case "proof":
		return "/certifications?svc=" + q
	case "refactor":
		return "/refactors?svc=" + q
	}
	return "/projects/" + url.PathEscape(service) + "/" + url.PathEscape(tab)
}

// thousandsNoun is countNoun with a thousands separator: "4,025 claims".
func thousandsNoun(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return thousands(n) + " " + word + "s"
}

// NonEmptyReason is s, or fallback when s is empty: a reason is never blank.
func NonEmptyReason(s, fallback string) string { return nonEmpty(s, fallback) }

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func verbFor(n int) string {
	if n == 1 {
		return "has"
	}
	return "have"
}

// shortList names up to max items and how many more.
func shortList(items []string, max int) string {
	if len(items) <= max {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:max], ", ") + fmt.Sprintf(" and %d more", len(items)-max)
}
