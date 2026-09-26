package console

// attention.go — OC-17 (vision 35 §3 Now, design §3): the attention queue.
// An item is a problem or a pending decision with its area, the evidence
// that shows it and the exact command that fixes it. The queue is ordered
// by what blocks proof first:
//
//  1. gate failures — claimed plan rows the verify gate proves false at
//     HEAD;
//  2. unproven claims — claimed rows the gate cannot prove (verify
//     --strict), plans whose newest run proved an older commit (history,
//     stated in the past tense with its commit and date), and plans no
//     recorded run ever checked;
//  3. stale or failed indexes — answers that no longer match HEAD;
//  4. certificate gaps — targets with no live certificate;
//  5. uncompiled governing guides — rules served raw, not compiled;
//  6. open observations — known defects waiting on a decision.
//
// Nothing here runs a verify, a compile or a doctor: every item reads a
// recorded instrument and names when it was true. A source that could not
// be read makes the queue a named lower bound, never a shorter list (I38),
// and an item whose count nobody knows carries its state and no count.

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"
)

// AttentionKind is an item's kind; the order of the kinds is the queue's.
type AttentionKind string

const (
	AttentionGate         AttentionKind = "gate"
	AttentionUnproven     AttentionKind = "unproven"
	AttentionIndex        AttentionKind = "index"
	AttentionCertificates AttentionKind = "certificates"
	AttentionGuides       AttentionKind = "guides"
	AttentionObservations AttentionKind = "observations"
)

// AttentionKinds lists the kinds in queue order: what blocks proof first.
var AttentionKinds = []AttentionKind{AttentionGate, AttentionUnproven, AttentionIndex, AttentionCertificates, AttentionGuides, AttentionObservations}

// Rank is the kind's position in the queue order (0 first).
func (k AttentionKind) Rank() int {
	for i, kk := range AttentionKinds {
		if kk == k {
			return i
		}
	}
	return len(AttentionKinds)
}

// Area is the kind's area label.
func (k AttentionKind) Area() string {
	switch k {
	case AttentionGate, AttentionUnproven:
		return "Gate"
	case AttentionIndex:
		return "Index"
	case AttentionCertificates:
		return "Proof"
	case AttentionGuides:
		return "Knowledge"
	case AttentionObservations:
		return "Observations"
	}
	return string(k)
}

// Link is an evidence link: a label and a console route.
type Link struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

// AttentionItem is one problem with its evidence and remedy.
type AttentionItem struct {
	Rank    int           `json:"rank"`
	Kind    AttentionKind `json:"kind"`
	Area    string        `json:"area"`
	Service string        `json:"service"`
	// Subject names what inside the project: a plan, a guide, a file.
	Subject string `json:"subject,omitempty"`
	Title   string `json:"title"`
	Detail  string `json:"detail"`
	// Count is how many (rows, plans, targets, observations); omitted when
	// the source could not be read and nobody knows — never a 0 standing
	// in for "unknown" (doctrine rule 6).
	Count int `json:"count,omitempty"`
	// State is the item's truth state: present (what the instrument
	// recorded of the code as it is), stale (recorded of an older commit),
	// partial (a lower bound: part of the source was unreadable) or error
	// (the source could not be read; Count is then unknown). Reason says
	// why for any state but present.
	State    State  `json:"state"`
	Reason   string `json:"reason,omitempty"`
	Evidence Link   `json:"evidence"`
	Remedy   string `json:"remedy"`
	// AsOf is when the instrument behind the item recorded it.
	AsOf string `json:"as_of,omitempty"`
	// weight orders items of one kind (most severe first); not rendered.
	weight int
}

// Attention is the queue: its items in order, a count per kind, and the
// reads it rests on.
type Attention struct {
	Items    []AttentionItem       `json:"items"`
	ByKind   map[AttentionKind]int `json:"by_kind"`
	Projects int                   `json:"projects"`
	// State is present when items exist, zero when every source was read
	// and nothing needs attention, partial when a source was not read (the
	// list is then a lower bound) and absent when nothing could be read.
	State   State        `json:"state"`
	Reason  string       `json:"reason,omitempty"`
	Sources []SourceRead `json:"sources"`
}

// AttentionQueue orders every project's problems by what blocks proof
// first, each with its area, evidence link and remedy.
func AttentionQueue(p Projects) Attention {
	var items []AttentionItem
	ledgerRead := p.Ledger.State != StateError
	for _, r := range p.Rows {
		items = append(items, gateItems(r, ledgerRead, p.Now)...)
		items = append(items, indexItems(r)...)
		items = append(items, certificateItems(r)...)
		items = append(items, guideItems(r)...)
		items = append(items, observationItems(r)...)
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if ra, rb := a.Kind.Rank(), b.Kind.Rank(); ra != rb {
			return ra < rb
		}
		// Within a kind, what is known of the code as it is before
		// history and unknowns.
		if sa, sb := itemStateRank(a.State), itemStateRank(b.State); sa != sb {
			return sa < sb
		}
		if a.weight != b.weight {
			return a.weight > b.weight
		}
		if a.Service != b.Service {
			return a.Service < b.Service
		}
		return a.Subject < b.Subject
	})
	by := map[AttentionKind]int{}
	for i := range items {
		items[i].Rank = i + 1
		items[i].Area = items[i].Kind.Area()
		if items[i].State == "" {
			items[i].State = StatePresent
		}
		by[items[i].Kind]++
	}
	a := Attention{Items: items, ByKind: by, Projects: len(p.Rows)}
	if a.Items == nil {
		a.Items = []AttentionItem{}
	}
	a.Sources = attentionSources(p)
	var unread []string
	for _, s := range a.Sources {
		// An absent registry or store is a read that did not happen (the
		// queue cannot see those projects' problems); an absent ledger or
		// observations file is nothing recorded, which hides nothing.
		if s.State == StateError || s.State == StatePartial || (s.State == StateAbsent && (s.Name == SourceRegistry || s.Name == SourceStore)) {
			unread = append(unread, s.Name+" ("+s.Reason+")")
		}
	}
	switch {
	case len(p.Rows) == 0 && p.RegistryNote != "":
		a.State, a.Reason = StateAbsent, "the service registry could not be read: "+p.RegistryNote
	case len(unread) > 0:
		a.State, a.Reason = StatePartial, "not read: "+strings.Join(unread, "; ")+" — the queue is a lower bound"
	case len(items) == 0:
		a.State = StateZero
	default:
		a.State = StatePresent
	}
	return a
}

// itemStateRank orders items of one kind: present, then partial, then
// stale history, then unread.
func itemStateRank(s State) int {
	switch s {
	case StatePresent, "":
		return 0
	case StatePartial:
		return 1
	case StateStale:
		return 2
	}
	return 3
}

// attentionSources names what the queue read.
func attentionSources(p Projects) []SourceRead {
	registry := SourceRead{Name: SourceRegistry, State: StatePresent}
	if p.RegistryNote != "" {
		registry.State, registry.Reason = StateAbsent, p.RegistryNote
		if len(p.Rows) > 0 {
			registry.State = StatePartial
		}
	}
	out := []SourceRead{registry, p.Ledger, p.Store}
	var obsFailed, certsFailed, certsPartial, plansFailed []string
	for _, r := range p.Rows {
		if r.ObservationsErr != "" {
			obsFailed = append(obsFailed, r.Service)
		}
		switch r.Certs.State {
		case StateError:
			certsFailed = append(certsFailed, r.Service)
		case StatePartial:
			certsPartial = append(certsPartial, r.Service)
		}
		if r.PlansErr != "" {
			plansFailed = append(plansFailed, r.Service)
		}
	}
	plans := SourceRead{Name: SourcePlanFiles, State: StatePresent}
	if len(plansFailed) > 0 {
		plans.State, plans.Reason = StateError, "unreadable in "+shortList(plansFailed, 3)
	}
	certs := SourceRead{Name: SourceCertTargets, State: StatePresent}
	var why []string
	if len(certsFailed) > 0 {
		why = append(why, "unreadable in "+shortList(certsFailed, 3))
	}
	if len(certsPartial) > 0 {
		why = append(why, "read in part in "+shortList(certsPartial, 3))
	}
	switch {
	case len(certsFailed) > 0:
		certs.State = StateError
	case len(certsPartial) > 0:
		certs.State = StatePartial
	}
	certs.Reason = strings.Join(why, "; ")
	obs := SourceRead{Name: SourceObservations, State: StatePresent}
	if len(obsFailed) > 0 {
		obs.State, obs.Reason = StateError, "unreadable in "+shortList(obsFailed, 3)
	}
	return append(out, plans, certs, obs)
}

// gateItems are a project's proof problems, each read from the plan's
// recorded proof (PlanRuns):
//
//   - a gate failure per plan whose run at HEAD fails claimed rows;
//   - its unproven claims at HEAD: one item per plan when a single plan has
//     them, else one for the project (a dozen plans of rows without
//     contracts is one piece of work, not a dozen alarms);
//   - plans whose newest run proved an older commit: history, stated in the
//     past tense with the commit and date, never a present-tense failure —
//     one item per plan, or one for the project when several;
//   - plans no recorded run ever checked (and a plans directory that could
//     not be read: unknown, never "no plans").
//
// ledgerRead is false when the verify ledger could not be read: no run is
// then known either way, and the queue's partial state says so instead of
// calling every plan unverified.
func gateItems(r ProjectFacts, ledgerRead bool, now time.Time) []AttentionItem {
	var out []AttentionItem
	var unproven, stale []VerifyRun
	evidence := Link{Label: "Plans · " + r.Service, Href: ProjectHref(r.Service, "plans")}
	for _, run := range r.Runs {
		if !run.Current() {
			stale = append(stale, run)
			continue
		}
		if run.Failing > 0 {
			out = append(out, AttentionItem{
				Kind: AttentionGate, Service: r.Service, Subject: run.Plan, Count: run.Failing, weight: run.Failing,
				Title:    fmt.Sprintf("%s in %s %s the gate", countNoun(run.Failing, "claimed row"), path.Base(run.Plan), pluralVerb(run.Failing, "fails", "fail")),
				Detail:   fmt.Sprintf("%s · the verify run at HEAD (%s): %d claimed, %d verified, %d failing — `fathom verify` exits 1", r.Service, runWhen(run), run.Claimed, run.Verified, run.Failing),
				Evidence: evidence,
				Remedy:   inRoot(r.Path, "fathom verify --service "+r.Service+" --plan "+run.Plan),
				AsOf:     run.At,
			})
		}
		if run.Unproven() > 0 {
			unproven = append(unproven, run)
		}
	}
	switch len(unproven) {
	case 0:
	case 1:
		run := unproven[0]
		n := run.Unproven()
		out = append(out, AttentionItem{
			Kind: AttentionUnproven, Service: r.Service, Subject: run.Plan, Count: n, weight: n,
			Title:    fmt.Sprintf("%s in %s %s checked done, but the gate cannot prove %s", countNoun(n, "row"), path.Base(run.Plan), pluralVerb(n, "is", "are"), pluralVerb(n, "it", "them")),
			Detail:   fmt.Sprintf("%s · the verify run at HEAD (%s): %d claimed, %d verified — `fathom verify --strict` exits 1 on a claimed row with no contract the gate can test", r.Service, runWhen(run), run.Claimed, run.Verified),
			Evidence: evidence,
			Remedy:   inRoot(r.Path, "fathom verify --strict --service "+r.Service+" --plan "+run.Plan),
			AsOf:     run.At,
		})
	default:
		sort.SliceStable(unproven, func(i, j int) bool {
			if a, b := unproven[i].Unproven(), unproven[j].Unproven(); a != b {
				return a > b
			}
			return unproven[i].Plan < unproven[j].Plan
		})
		total, oldest, newest := 0, "", ""
		var named []string
		for _, run := range unproven {
			total += run.Unproven()
			if oldest == "" || run.At < oldest {
				oldest = run.At
			}
			if run.At > newest {
				newest = run.At
			}
			named = append(named, fmt.Sprintf("%s %d", path.Base(run.Plan), run.Unproven()))
		}
		when := dayOf(oldest)
		if d := dayOf(newest); d != when {
			when += " to " + d
		}
		out = append(out, AttentionItem{
			Kind: AttentionUnproven, Service: r.Service, Count: total, weight: total,
			Title:    fmt.Sprintf("%s across %d plans in %s %s checked done, but the gate cannot prove %s", thousandsNoun(total, "row"), len(unproven), r.Service, pluralVerb(total, "is", "are"), pluralVerb(total, "it", "them")),
			Detail:   fmt.Sprintf("%s · the verify runs at HEAD (%s): %s — `fathom verify --strict` exits 1 on each", r.Service, when, shortList(named, 4)),
			Evidence: evidence,
			Remedy:   inRoot(r.Path, "fathom plans status --service "+r.Service),
			AsOf:     newest,
		})
	}
	if item, ok := staleProofItem(r, stale, evidence, now); ok {
		out = append(out, item)
	}
	if !ledgerRead {
		return out
	}
	switch unrun := UnrunPlans(r); {
	case r.PlansErr != "":
		out = append(out, AttentionItem{
			Kind: AttentionUnproven, Service: r.Service, Subject: "docs/plans", weight: 1 << 30,
			State: StateError, Reason: r.PlansErr,
			Title:    r.Service + "'s plans cannot be read",
			Detail:   r.Service + " · " + r.PlansErr + " — its proof state is unknown, never none",
			Evidence: evidence, Remedy: inRoot(r.Path, "fathom doctor"),
		})
	case len(unrun) == 1:
		out = append(out, AttentionItem{
			Kind: AttentionUnproven, Service: r.Service, Subject: unrun[0], Count: 1, weight: 1,
			Title:    path.Base(unrun[0]) + " in " + r.Service + " has no recorded verify run",
			Detail:   r.Service + " · no `fathom verify` run of " + unrun[0] + " is recorded, at HEAD or at the registered root — its claimed rows are unchecked",
			Evidence: evidence, Remedy: inRoot(r.Path, "fathom verify --service "+r.Service+" --plan "+unrun[0]),
		})
	case len(unrun) > 1:
		names := make([]string, len(unrun))
		for i, u := range unrun {
			names[i] = path.Base(u)
		}
		out = append(out, AttentionItem{
			Kind: AttentionUnproven, Service: r.Service, Count: len(unrun), weight: len(unrun),
			Title:    fmt.Sprintf("%d plans in %s have no recorded verify run", len(unrun), r.Service),
			Detail:   r.Service + " · no `fathom verify` run of " + shortList(names, 4) + " is recorded, at HEAD or at the registered root — their claimed rows are unchecked",
			Evidence: evidence, Remedy: inRoot(r.Path, "fathom plans status --service "+r.Service),
		})
	case len(r.Runs) == 0 && r.HasPlans:
		// Plans exist but their files were not listed: one item for the
		// project, the count unknown.
		out = append(out, AttentionItem{
			Kind: AttentionUnproven, Service: r.Service, Subject: "docs/plans", weight: 1,
			Title:    r.Service + "'s plans have no recorded verify run",
			Detail:   r.Service + " · no `fathom verify` run is recorded, at HEAD or at the registered root — their claimed rows are unchecked",
			Evidence: evidence, Remedy: inRoot(r.Path, "fathom plans status --service "+r.Service),
		})
	}
	return out
}

// staleProofItem is a project's plans whose newest recorded run proved an
// older commit: what those runs found, in the past tense, and the command
// that asks the gate about HEAD.
func staleProofItem(r ProjectFacts, stale []VerifyRun, evidence Link, now time.Time) (AttentionItem, bool) {
	if len(stale) == 0 {
		return AttentionItem{}, false
	}
	sort.SliceStable(stale, func(i, j int) bool {
		if a, b := stale[i].Failing+stale[i].Unproven(), stale[j].Failing+stale[j].Unproven(); a != b {
			return a > b
		}
		return stale[i].Plan < stale[j].Plan
	})
	failing, unproven, oldest := 0, 0, ""
	for _, run := range stale {
		failing += run.Failing
		unproven += run.Unproven()
		if oldest == "" || run.At < oldest {
			oldest = run.At
		}
	}
	found := "it proved every claimed row then"
	if failing > 0 || unproven > 0 {
		var parts []string
		if failing > 0 {
			parts = append(parts, fmt.Sprintf("failed %d", failing))
		}
		if unproven > 0 {
			parts = append(parts, fmt.Sprintf("could not prove %d", unproven))
		}
		found = "it " + strings.Join(parts, " and ") + " of its claimed rows then"
	}
	head := "HEAD"
	if r.RepoHead != "" {
		head = "HEAD " + shortCommit(r.RepoHead)
	}
	it := AttentionItem{
		Kind: AttentionUnproven, Service: r.Service, Count: len(stale), weight: failing + unproven,
		State: StateStale, Reason: StaleProofReason(stale, r.RepoHead, now),
		Evidence: evidence, AsOf: oldest,
	}
	if len(stale) == 1 {
		run := stale[0]
		it.Subject = run.Plan
		it.Title = path.Base(run.Plan) + " in " + r.Service + " was last verified before " + head
		it.Detail = fmt.Sprintf("%s · the newest verify run (%s): %d claimed, %d verified — %s; %s has no recorded run", r.Service, runWhen(run), run.Claimed, run.Verified, found, head)
		if run.StaleReason != "" && r.RepoHead == "" {
			it.Detail += " (" + run.StaleReason + ")"
		}
		it.Remedy = inRoot(r.Path, "fathom verify --service "+r.Service+" --plan "+run.Plan)
		return it, true
	}
	named := make([]string, 0, len(stale))
	commits := map[string]bool{}
	var commitList []string
	for _, run := range stale {
		named = append(named, path.Base(run.Plan))
		if c := shortCommit(run.Commit); c != "" && !commits[c] {
			commits[c] = true
			commitList = append(commitList, c)
		}
	}
	sort.Strings(commitList)
	when := dayOf(oldest)
	if d := dayOf(maxAt(stale)); d != when {
		when += " to " + d
	}
	at := ""
	if len(commitList) > 0 {
		at = " at " + shortList(commitList, 3)
	}
	it.Title = fmt.Sprintf("%d plans in %s were last verified before %s", len(stale), r.Service, head)
	it.Detail = fmt.Sprintf("%s · their newest verify runs (%s%s): %s; %s has no recorded run of %s", r.Service, when, at, strings.Replace(found, "it ", "they ", 1), head, shortList(named, 4))
	it.Remedy = inRoot(r.Path, "fathom plans status --service "+r.Service)
	return it, true
}

func maxAt(runs []VerifyRun) string {
	m := ""
	for _, r := range runs {
		if r.At > m {
			m = r.At
		}
	}
	return m
}

// indexItems are a stale index, a failed typed-edge pass and an index whose
// freshness nobody can determine — unknown is never read as fresh.
func indexItems(r ProjectFacts) []AttentionItem {
	var out []AttentionItem
	overview := Link{Label: "Project · " + r.Service, Href: ProjectHref(r.Service, "")}
	switch freshnessClass(r.Freshness) {
	case "stale":
		detail := r.Freshness
		if r.IndexedCommit != "" && r.RepoHead != "" {
			detail = "indexed " + shortCommit(r.IndexedCommit) + ", HEAD " + shortCommit(r.RepoHead) + " — answers describe code that has changed"
		}
		out = append(out, AttentionItem{
			Kind: AttentionIndex, Service: r.Service, Subject: "freshness", Count: 1, weight: 3,
			Title: r.Service + "'s index is behind HEAD", Detail: r.Service + " · " + detail,
			Evidence: overview, Remedy: r.RefreshRemedy,
		})
	case "unknown":
		reason := strings.TrimPrefix(r.Freshness, "unknown: ")
		if reason == "" {
			reason = "no freshness state was reported"
		}
		out = append(out, AttentionItem{
			Kind: AttentionIndex, Service: r.Service, Subject: "freshness", Count: 1, weight: 1,
			Title:    r.Service + "'s index freshness cannot be determined",
			Detail:   r.Service + " · " + reason + " — unknown is never read as fresh",
			Evidence: overview, Remedy: inRoot(r.Path, "fathom doctor"),
		})
	}
	if r.TypedEdgesFailed != "" {
		out = append(out, AttentionItem{
			Kind: AttentionIndex, Service: r.Service, Subject: "typed edges", Count: 1, weight: 2,
			Title:    r.Service + "'s typed call-edge pass failed",
			Detail:   r.Service + " · " + r.TypedEdgesFailed + " — who-calls answers fall back to syntax-only edges",
			Evidence: Link{Label: "Daemon status", Href: "/statusz"}, Remedy: r.RefreshRemedy,
		})
	}
	return out
}

// certificateItems is one item per project with certification gaps. A
// project whose targets could be read only in part is a named lower bound
// (partial); one whose targets could not be read at all is unknown (error,
// no count) — never "declares none".
func certificateItems(r ProjectFacts) []AttentionItem {
	evidence := Link{Label: "Proof · " + r.Service, Href: ProjectHref(r.Service, "proof")}
	if r.Certs.State == StateError {
		return []AttentionItem{{
			Kind: AttentionCertificates, Service: r.Service, Subject: "certificates", weight: 1 << 30,
			State: StateError, Reason: r.Certs.Reason,
			Title:    r.Service + "'s certificates cannot be read",
			Detail:   r.Service + " · " + r.Certs.Reason + " — gaps here are unknown, not absent",
			Evidence: evidence,
			Remedy:   r.CertRemedy,
		}}
	}
	c := r.Certs.V
	if r.Certs.State == StatePartial {
		it := AttentionItem{
			Kind: AttentionCertificates, Service: r.Service, Subject: "certificates", Count: c.Gaps, weight: 1 << 29,
			State: StatePartial, Reason: r.Certs.Reason,
			Title:    fmt.Sprintf("at least %d of %s+ certification targets in %s %s no live certificate", c.Gaps, thousands(c.Declared), r.Service, pluralVerb(c.Gaps, "has", "have")),
			Detail:   fmt.Sprintf("%s · certify status read in part: %d live, %d gaps — %s", r.Service, c.Live, c.Gaps, r.Certs.Reason),
			Evidence: evidence,
			Remedy:   r.CertRemedy,
		}
		if c.Gaps == 0 {
			it.Title = r.Service + "'s certification targets could be read only in part"
		}
		return []AttentionItem{it}
	}
	if r.Certs.State != StatePresent || c.Gaps == 0 {
		return nil
	}
	return []AttentionItem{{
		Kind: AttentionCertificates, Service: r.Service, Subject: "certificates", Count: c.Gaps, weight: c.Gaps,
		Title:    fmt.Sprintf("%d of %s in %s %s no live certificate", c.Gaps, thousandsNoun(c.Declared, "certification target"), r.Service, pluralVerb(c.Gaps, "has", "have")),
		Detail:   fmt.Sprintf("%s · certify status: %d live, %d gaps — each gap names the marker or manifest field that would certify it", r.Service, c.Live, c.Gaps),
		Evidence: evidence,
		Remedy:   r.CertRemedy,
	}}
}

// guideItems names governing guides the obligations layer serves raw: its
// latest compile left them uncompiled, or no compile ever ran.
func guideItems(r ProjectFacts) []AttentionItem {
	evidence := Link{Label: "Governance · " + r.Service, Href: ProjectHref(r.Service, "governance")}
	remedy := "fathom compile --service " + r.Service + " --layer obligations"
	g := r.Guide
	switch g.State {
	case StateError:
		return []AttentionItem{{
			Kind: AttentionGuides, Service: r.Service, Subject: "obligations", weight: 1 << 30,
			State: StateError, Reason: g.Reason,
			Title:    r.Service + "'s compile record cannot be read",
			Detail:   r.Service + " · " + g.Reason + " — uncompiled guides here are unknown, not absent",
			Evidence: evidence, Remedy: inRoot(r.Path, "fathom doctor"),
		}}
	case StateAbsent:
		if !r.HasGuide {
			return nil
		}
		return []AttentionItem{{
			Kind: AttentionGuides, Service: r.Service, Subject: "obligations", Count: 1, weight: 1 << 20,
			Title:    r.Service + "'s governing guides were never compiled",
			Detail:   r.Service + " · " + g.Reason + " — the raw fallback serves every section; `fathom compile` prints the cost first (a dry run)",
			Evidence: evidence, Remedy: remedy,
		}}
	}
	cov := g.V
	n := len(cov.Uncompiled)
	if n == 0 {
		return nil
	}
	title := fmt.Sprintf("%s governs %s files, but its sections are not compiled", cov.Uncompiled[0], r.Service)
	if n > 1 {
		title = fmt.Sprintf("%d governing guides in %s are not compiled", n, r.Service)
	}
	when := fmt.Sprintf("%d sections", cov.SectionsTotal)
	if cov.RanAt != "" {
		when = dayOf(cov.RanAt) + ", " + when
	}
	detail := r.Service + " · the last obligations compile (" + when + ") did not compile them"
	if cov.BudgetStopped {
		detail += " — it stopped at its budget"
	}
	detail += "; the raw fallback serves " + shortList(cov.Uncompiled, 3) + " — `fathom compile` prints the cost first (a dry run)"
	return []AttentionItem{{
		Kind: AttentionGuides, Service: r.Service, Subject: strings.Join(cov.Uncompiled, ","), Count: n, weight: n,
		Title: title, Detail: detail, Evidence: evidence, Remedy: remedy, AsOf: cov.RanAt,
	}}
}

// observationItems are a project's open observations, or its unreadable
// observations file (which hides an unknown number of them).
func observationItems(r ProjectFacts) []AttentionItem {
	file := ".fathom/observations.md"
	if r.Path != "" {
		file = strings.TrimSuffix(r.Path, "/") + "/" + file
	}
	overview := Link{Label: "Project · " + r.Service, Href: ProjectHref(r.Service, "")}
	if r.ObservationsErr != "" {
		return []AttentionItem{{
			Kind: AttentionObservations, Service: r.Service, Subject: file, weight: 1 << 30,
			State: StateError, Reason: r.ObservationsErr,
			Title:    r.Service + "'s observations file cannot be read",
			Detail:   r.Service + " · " + r.ObservationsErr + " — its open defects are unknown, never none",
			Evidence: overview, Remedy: inRoot(r.Path, "fathom doctor"),
		}}
	}
	if r.OpenObservations == nil || *r.OpenObservations == 0 {
		return nil
	}
	n := *r.OpenObservations
	return []AttentionItem{{
		Kind: AttentionObservations, Service: r.Service, Subject: file, Count: n, weight: n,
		Title:    fmt.Sprintf("%s in %s", countNoun(n, "open observation"), r.Service),
		Detail:   r.Service + " · " + file + ": known defects waiting on a fix or a decision",
		Evidence: overview, Remedy: "review " + file + " — close what is fixed, file what is not",
	}}
}

// inRoot prefixes a command with the project root it must run in.
func inRoot(root, cmd string) string {
	if root == "" {
		return cmd
	}
	return "cd " + root + " && " + cmd
}

// runWhen names a run's date and commit.
func runWhen(r VerifyRun) string {
	s := dayOf(r.At)
	if r.Commit != "" {
		s += " at " + shortCommit(r.Commit)
	}
	return s
}

func dayOf(ts string) string {
	if i := strings.IndexByte(ts, 'T'); i > 0 {
		return ts[:i]
	}
	return ts
}

func pluralVerb(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
