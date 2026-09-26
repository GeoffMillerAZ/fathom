package console

// catches.go — OC-42 (vision 35 §4 view 8, design §3 "Impact"): what
// fathom caught, on one filterable timeline. Four kinds, each from the
// instrument that records it:
//
//   - verify refusal   — a verify-ledger run whose verdict carried at least
//     one failing claimed row (the one canonical gate figure, verifyFigure:
//     an EVENT, never a sum of standing failures);
//   - conserve loss    — a `fathom review` run whose conservation dimension
//     reported findings (its call-log row's verdict_outcome.conservation);
//   - integrity catch  — the same row's integrity dimension;
//   - guard refusal    — an enforce:true guard denying an edit or command
//     (the edit hook's "denied" beat, ADR-038).
//
// A kind whose instrument recorded nothing it could count from is absent
// with its reason — never an empty lane that reads as "caught nothing".

import (
	"sort"
	"strings"
	"time"
)

// CatchKind is one kind of catch.
type CatchKind string

const (
	CatchVerifyRefusal CatchKind = "verify-refusal"
	CatchConserveLoss  CatchKind = "conserve-loss"
	CatchIntegrity     CatchKind = "integrity-catch"
	CatchGuardRefusal  CatchKind = "guard-refusal"
)

// CatchKinds is the fixed order the timeline lists its lanes in.
var CatchKinds = []CatchKind{CatchVerifyRefusal, CatchConserveLoss, CatchIntegrity, CatchGuardRefusal}

// CatchKindLabel is a kind's human name.
func CatchKindLabel(k CatchKind) string {
	switch k {
	case CatchVerifyRefusal:
		return "verify refusals"
	case CatchConserveLoss:
		return "conserve losses"
	case CatchIntegrity:
		return "integrity catches"
	case CatchGuardRefusal:
		return "guard refusals"
	}
	return string(k)
}

// ParseCatchKind maps a query value to a kind.
func ParseCatchKind(s string) (CatchKind, bool) {
	for _, k := range CatchKinds {
		if string(k) == s {
			return k, true
		}
	}
	return "", false
}

// VerifyRun is one verify-ledger row.
type VerifyRun struct {
	At      string
	Service string
	Plan    string
	Origin  string
	Failing int
	Claimed int
}

// ReviewRun is one `fathom review` call-log row with its per-dimension
// finding counts (only checked dimensions carry a key).
type ReviewRun struct {
	At      string
	Service string
	Outcome map[string]int
}

// GuardBeat is one PreToolUse edit-hook beat: Kind "seen", "answered" or
// "denied"; Target "file" or "command".
type GuardBeat struct {
	At      time.Time
	Kind    string
	Target  string
	Service string
	Session string
}

// CatchInputs are the four instruments' rows, already read. A non-empty
// *Reason means that instrument could not be read at all.
type CatchInputs struct {
	VerifyRuns   []VerifyRun
	VerifyReason string
	VerifySource string
	Reviews      []ReviewRun
	ReviewReason string
	ReviewSource string
	GuardBeats   []GuardBeat
	GuardReason  string
	GuardSource  string
}

// CatchFilter narrows the timeline: kinds (none = all), one service, and a
// window [From, To] (zero = unbounded).
type CatchFilter struct {
	Kinds   []CatchKind `json:"kinds,omitempty"`
	Service string      `json:"service,omitempty"`
	From    time.Time   `json:"-"`
	To      time.Time   `json:"-"`
	// FromText / ToText echo the window in RFC 3339 ("" = unbounded).
	FromText string `json:"from,omitempty"`
	ToText   string `json:"to,omitempty"`
}

func (f CatchFilter) wants(k CatchKind) bool {
	if len(f.Kinds) == 0 {
		return true
	}
	for _, x := range f.Kinds {
		if x == k {
			return true
		}
	}
	return false
}

func (f CatchFilter) inWindow(t time.Time) bool {
	if !f.From.IsZero() && t.Before(f.From) {
		return false
	}
	if !f.To.IsZero() && t.After(f.To) {
		return false
	}
	return true
}

// Catch is one caught event.
type Catch struct {
	At      time.Time `json:"at"`
	Kind    CatchKind `json:"kind"`
	Service string    `json:"service,omitempty"`
	Subject string    `json:"subject"`
	Detail  string    `json:"detail"`
}

// CatchKindSummary is one kind's lane: its count in the filtered window
// (absent with a reason when its instrument is not recording) and where it
// comes from.
type CatchKindSummary struct {
	Kind     CatchKind  `json:"kind"`
	Label    string     `json:"label"`
	Selected bool       `json:"selected"`
	Count    Value[int] `json:"count"`
}

// CatchTimelineModel is the timeline's projection: every kind's lane, the
// filtered catches newest first (one page of them), and the filter echoed.
type CatchTimelineModel struct {
	Filter  CatchFilter        `json:"filter"`
	Kinds   []CatchKindSummary `json:"kinds"`
	Total   int                `json:"total"`
	Page    int                `json:"page"`
	Pages   int                `json:"pages"`
	PerPage int                `json:"per_page"`
	Catches []Catch            `json:"catches"`
	// Window is every filtered catch's time and kind (the timeline draws
	// all of them; Catches is the page of rows the table lists).
	Window []CatchPoint `json:"window"`
}

// CatchPoint is one catch on the timeline (the table's rows carry the rest).
type CatchPoint struct {
	At   time.Time `json:"at"`
	Kind CatchKind `json:"kind"`
}

// MaxCatchPoints bounds the timeline's dots; past it the newest are drawn
// and the model says how many were left off.
const MaxCatchPoints = 2000

func parseAt(s string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), true
	}
	return time.Time{}, false
}

// CatchTimeline projects the four instruments onto one timeline under f,
// paging the rows perPage at a time (1-based page, clamped).
func CatchTimeline(in CatchInputs, f CatchFilter, page, perPage int) CatchTimelineModel {
	if perPage <= 0 {
		perPage = 50
	}
	if !f.From.IsZero() {
		f.FromText = f.From.UTC().Format(time.RFC3339)
	}
	if !f.To.IsZero() {
		f.ToText = f.To.UTC().Format(time.RFC3339)
	}
	m := CatchTimelineModel{Filter: f, PerPage: perPage}
	var all []Catch
	counts := map[CatchKind]int{}
	add := func(c Catch) {
		if f.Service != "" && !strings.EqualFold(c.Service, f.Service) {
			return
		}
		if !f.inWindow(c.At) {
			return
		}
		counts[c.Kind]++
		if f.wants(c.Kind) {
			all = append(all, c)
		}
	}

	// Verify refusals: one ledger row whose verdict carried a failing row.
	for _, r := range in.VerifyRuns {
		if r.Failing <= 0 {
			continue
		}
		t, ok := parseAt(r.At)
		if !ok {
			continue
		}
		origin := r.Origin
		if origin == "" {
			origin = "origin unrecorded"
		}
		add(Catch{At: t, Kind: CatchVerifyRefusal, Service: r.Service, Subject: r.Plan,
			Detail: countNoun(r.Failing, "failing claimed row") + " of " + countNoun(r.Claimed, "claimed row") + " (" + origin + ")"})
	}
	// Conserve losses and integrity catches: a review run's dimensions.
	reviewedConservation, reviewedIntegrity := false, false
	for _, r := range in.Reviews {
		t, ok := parseAt(r.At)
		if !ok {
			continue
		}
		if n, checked := r.Outcome["conservation"]; checked {
			reviewedConservation = true
			if n > 0 {
				add(Catch{At: t, Kind: CatchConserveLoss, Service: r.Service, Subject: "fathom review",
					Detail: countNoun(n, "conservation finding")})
			}
		}
		if n, checked := r.Outcome["integrity"]; checked {
			reviewedIntegrity = true
			if n > 0 {
				add(Catch{At: t, Kind: CatchIntegrity, Service: r.Service, Subject: "fathom review",
					Detail: countNoun(n, "integrity finding")})
			}
		}
	}
	// Guard refusals: the pre-hook's denied beat.
	guardFired := false
	for _, b := range in.GuardBeats {
		guardFired = true
		if b.Kind != "denied" {
			continue
		}
		target := b.Target
		if target == "" {
			target = "edit"
		}
		add(Catch{At: b.At.UTC(), Kind: CatchGuardRefusal, Service: b.Service, Subject: "guard denied a " + target,
			Detail: "enforce:true guard (ADR-038)"})
	}

	for _, k := range CatchKinds {
		s := CatchKindSummary{Kind: k, Label: CatchKindLabel(k), Selected: f.wants(k)}
		n := counts[k]
		switch k {
		case CatchVerifyRefusal:
			switch {
			case in.VerifyReason != "":
				s.Count = Absent[int](in.VerifyReason)
			case len(in.VerifyRuns) == 0:
				s.Count = Absent[int]("not instrumented — the verify ledger records no `fathom verify` run")
			default:
				s.Count = Measured(n, "").WithDenominator("runs refused, of " + countNoun(len(in.VerifyRuns), "recorded verify run"))
			}
			s.Count = s.Count.WithSource(in.VerifySource)
		case CatchConserveLoss, CatchIntegrity:
			checked := reviewedConservation
			dim := "conservation"
			if k == CatchIntegrity {
				checked, dim = reviewedIntegrity, "integrity"
			}
			switch {
			case in.ReviewReason != "":
				s.Count = Absent[int](in.ReviewReason)
			case !checked:
				s.Count = Absent[int]("not instrumented — no `fathom review` run in the call log checked its " + dim + " dimension")
			default:
				s.Count = Measured(n, "").WithDenominator("review runs whose " + dim + " dimension reported findings")
			}
			s.Count = s.Count.WithSource(in.ReviewSource)
		case CatchGuardRefusal:
			switch {
			case in.GuardReason != "":
				s.Count = Absent[int](in.GuardReason)
			case !guardFired:
				s.Count = Absent[int]("not instrumented — no PreToolUse guard-hook beat in the hooks log (the guard hook is not wired, or never fired)")
			default:
				s.Count = Measured(n, "").WithDenominator("edits or commands an enforce:true guard denied")
			}
			s.Count = s.Count.WithSource(in.GuardSource)
		}
		m.Kinds = append(m.Kinds, s)
	}

	sort.SliceStable(all, func(i, j int) bool {
		if !all[i].At.Equal(all[j].At) {
			return all[i].At.After(all[j].At)
		}
		if all[i].Kind != all[j].Kind {
			return all[i].Kind < all[j].Kind
		}
		return all[i].Subject < all[j].Subject
	})
	m.Total = len(all)
	m.Pages = (m.Total + perPage - 1) / perPage
	if m.Pages == 0 {
		m.Pages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > m.Pages {
		page = m.Pages
	}
	m.Page = page
	start := (page - 1) * perPage
	end := min(start+perPage, m.Total)
	m.Catches = append([]Catch{}, all[start:end]...)
	for i, c := range all {
		if i == MaxCatchPoints {
			break
		}
		m.Window = append(m.Window, CatchPoint{At: c.At, Kind: c.Kind})
	}
	return m
}
