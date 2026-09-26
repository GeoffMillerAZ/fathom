package console

// attention.go — OC-17 (vision 35 §3 Now, design §3): the attention queue.
// An item is a problem or a pending decision with its area, the evidence
// that shows it and the exact command that fixes it. The queue is ordered
// by what blocks proof first:
//
//  1. gate failures — claimed plan rows the verify gate proves false;
//  2. unproven claims — claimed rows the gate cannot prove (verify --strict);
//  3. stale or failed indexes — answers that no longer match HEAD;
//  4. certificate gaps — targets with no live certificate;
//  5. uncompiled governing guides — rules served raw, not compiled;
//  6. open observations — known defects waiting on a decision.
//
// Nothing here runs a verify, a compile or a doctor: every item reads a
// recorded instrument and names when it was true. A source that could not
// be read makes the queue a named lower bound, never a shorter list (I38).

import (
	"fmt"
	"path"
	"sort"
	"strings"
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
	Subject  string `json:"subject,omitempty"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Count    int    `json:"count"`
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
	for _, r := range p.Rows {
		items = append(items, gateItems(r)...)
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
	var obsFailed []string
	for _, r := range p.Rows {
		if r.ObservationsErr != "" {
			obsFailed = append(obsFailed, r.Service)
		}
	}
	obs := SourceRead{Name: SourceObservations, State: StatePresent}
	if len(obsFailed) > 0 {
		obs.State, obs.Reason = StateError, "unreadable in "+shortList(obsFailed, 3)
	}
	return append(out, obs)
}

// gateItems are a project's gate failures, one item per plan whose newest
// recorded run carries them, and its unproven claims: one item per plan when
// a single plan has them, else one item for the project (a dozen plans of
// rows without contracts is one piece of work, not a dozen alarms).
func gateItems(r ProjectFacts) []AttentionItem {
	var out []AttentionItem
	var unproven []VerifyRun
	for _, run := range r.Runs {
		if run.Failing > 0 {
			out = append(out, AttentionItem{
				Kind: AttentionGate, Service: r.Service, Subject: run.Plan, Count: run.Failing, weight: run.Failing,
				Title:    fmt.Sprintf("%s in %s %s the gate", countNoun(run.Failing, "claimed row"), path.Base(run.Plan), pluralVerb(run.Failing, "fails", "fail")),
				Detail:   fmt.Sprintf("%s · the newest verify run (%s): %d claimed, %d verified, %d failing — `fathom verify` exits 1", r.Service, runWhen(run), run.Claimed, run.Verified, run.Failing),
				Evidence: Link{Label: "Plans · " + r.Service, Href: ProjectHref(r.Service, "plans")},
				Remedy:   inRoot(r.Path, "fathom verify --service "+r.Service+" --plan "+run.Plan),
				AsOf:     run.At,
			})
		}
		if run.Unproven() > 0 {
			unproven = append(unproven, run)
		}
	}
	evidence := Link{Label: "Plans · " + r.Service, Href: ProjectHref(r.Service, "plans")}
	switch len(unproven) {
	case 0:
	case 1:
		run := unproven[0]
		n := run.Unproven()
		out = append(out, AttentionItem{
			Kind: AttentionUnproven, Service: r.Service, Subject: run.Plan, Count: n, weight: n,
			Title:    fmt.Sprintf("%s in %s %s checked done, but the gate cannot prove %s", countNoun(n, "row"), path.Base(run.Plan), pluralVerb(n, "is", "are"), pluralVerb(n, "it", "them")),
			Detail:   fmt.Sprintf("%s · the newest verify run (%s): %d claimed, %d verified — `fathom verify --strict` exits 1 on a claimed row with no contract the gate can test", r.Service, runWhen(run), run.Claimed, run.Verified),
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
			Detail:   fmt.Sprintf("%s · the newest run of each plan (%s): %s — `fathom verify --strict` exits 1 on each", r.Service, when, shortList(named, 4)),
			Evidence: evidence,
			Remedy:   inRoot(r.Path, "fathom plans status --service "+r.Service),
			AsOf:     newest,
		})
	}
	return out
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

// certificateItems is one item per project with certification gaps.
func certificateItems(r ProjectFacts) []AttentionItem {
	if r.Certs.State == StateError {
		return []AttentionItem{{
			Kind: AttentionCertificates, Service: r.Service, Subject: "certificates", weight: 1 << 30,
			Title:    r.Service + "'s certificates cannot be read",
			Detail:   r.Service + " · " + r.Certs.Reason + " — gaps here are unknown, not absent",
			Evidence: Link{Label: "Proof · " + r.Service, Href: ProjectHref(r.Service, "proof")},
			Remedy:   r.CertRemedy,
		}}
	}
	c := r.Certs.V
	if r.Certs.State != StatePresent || c.Gaps == 0 {
		return nil
	}
	return []AttentionItem{{
		Kind: AttentionCertificates, Service: r.Service, Subject: "certificates", Count: c.Gaps, weight: c.Gaps,
		Title:    fmt.Sprintf("%d of %s in %s %s no live certificate", c.Gaps, thousandsNoun(c.Declared, "certification target"), r.Service, pluralVerb(c.Gaps, "has", "have")),
		Detail:   fmt.Sprintf("%s · certify status: %d live, %d gaps — each gap names the marker or manifest field that would certify it", r.Service, c.Live, c.Gaps),
		Evidence: Link{Label: "Proof · " + r.Service, Href: ProjectHref(r.Service, "proof")},
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
