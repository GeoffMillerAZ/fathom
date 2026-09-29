package console

// workcost.go — WI-2..WI-5 (docs/design/work-intelligence.md §3 U1, U2):
// effort per proven change, cost per zone with CI minutes, the split
// between first-attempt proofs and proofs after claimed-unproven retries,
// and the technical-debt interest rate per zone. Every function here is a
// pure projection over what a caller already read (the join, PE-15; the
// proof history, PE-2; the session ledger and transcript fold, PE-13; one
// `git show`/`git log` per commit) — no new capture (design §4), no
// transcript prose (ADR-041 decision 8), and rows are agent identities,
// never people (I58, agentIdentityKey).

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SessionEffort is one session's measured effort: tokens and turns from
// the transcript fold, wall time from its window, reverted edits observed
// by the edit hook, and its cost when the caller's price table prices the
// model (nil means unpriced — never 0).
type SessionEffort struct {
	SessionID   string
	Harness     string
	Model       string
	Effort      string // the harness's effort setting, "" when unrecorded
	TokensIn    int
	TokensOut   int
	Turns       int
	WallSeconds int
	Reverts     int
	USD         *float64
}

// CommitFacts is one commit's touched files, its rework (lines changed
// again within ReworkWindow of it) and the later commits that revert it.
// ReworkKnown is false when the caller could not read that (a commit git
// cannot resolve, a window still open), and ReworkReason then says which;
// RevertsKnown is false when the read did not cover the time after the
// commit — never a zero standing in for an unread figure.
type CommitFacts struct {
	Commit       string
	Files        []string
	ReworkLines  int
	ReworkKnown  bool
	ReworkReason string
	Reverts      []CommitRevert
	RevertsKnown bool
	RevertReason string
}

// CommitRevert is a later commit that reverts a proving commit (git
// revert's "This reverts commit <sha>" line), with its commit time.
type CommitRevert struct {
	Commit string `json:"commit"`
	At     string `json:"at"`
}

// ReworkWindow is U2's rework horizon: lines changed again within 7 days.
const ReworkWindow = 7 * 24 * time.Hour

// ProvenChange is one proven change — a commit that moved at least one
// row to claimed-proven — with its share of its session's effort.
type ProvenChange struct {
	Commit  string   `json:"commit"`
	At      string   `json:"at"`
	Rows    []string `json:"rows"`
	Agent   string   `json:"agent"`
	Harness string   `json:"harness"`
	Model   string   `json:"model"`
	Effort  string   `json:"effort,omitempty"`
	// Known is false when the session that proved it has no effort row:
	// the change is kept (I38) and every effort figure below is zero and
	// unmeasured.
	Known       bool           `json:"effort_known"`
	Tokens      float64        `json:"tokens"`
	Turns       float64        `json:"turns"`
	WallSeconds float64        `json:"wall_seconds"`
	Reverts     float64        `json:"reverts"`
	USD         Value[float64] `json:"usd"`
	Rework      Value[int]     `json:"rework_lines"`
	// Reverted counts the later commits that revert this change's commit.
	Reverted Value[int] `json:"reverted_by_commits"`
	// Retries counts the claimed-unproven readings of its rows before
	// they were proven (the history records a state change, so repeated
	// unproven readings with no change in between count once).
	Retries int `json:"claimed_unproven_retries"`
	// Weight is the proven rows' summed plan weight at their proof.
	Weight int         `json:"weight"`
	Zones  []ZoneShare `json:"zones"`
}

// FirstAttempt reports whether the change was proven with no
// claimed-unproven reading before it.
func (c ProvenChange) FirstAttempt() bool { return c.Retries == 0 }

// EffortPerProvenChange (WI-4) turns the join's session outcomes into
// proven changes: a session's rows grouped by the commit that proved them
// (a row with no commit is its own change), the session's effort split
// evenly across its changes. Sorted by time, then commit.
func EffortPerProvenChange(join JoinResult, h History, sessions []SessionEffort, commits map[string]CommitFacts, z Zones) []ProvenChange {
	rows := make(map[string]RowHistory, len(h.Rows))
	for _, r := range h.Rows {
		rows[r.Key()] = r
	}
	effort := make(map[string]SessionEffort, len(sessions))
	for _, s := range sessions {
		effort[s.SessionID] = s
	}
	var out []ProvenChange
	for _, so := range join.Outcomes {
		byCommit := map[string]*ProvenChange{}
		var order []string
		for _, pr := range so.Rows {
			key := pr.Commit
			if key == "" {
				key = "at:" + pr.At + "|" + RowKey(pr.Service, pr.Plan, pr.ID)
			}
			c := byCommit[key]
			if c == nil {
				c = &ProvenChange{Commit: pr.Commit, At: pr.At, Harness: agentIdentityPart(so.Harness, "unknown"), Model: agentIdentityPart(so.Model, "unknown")}
				byCommit[key] = c
				order = append(order, key)
			}
			c.Rows = append(c.Rows, RowKey(pr.Service, pr.Plan, pr.ID))
			rh := rows[RowKey(pr.Service, pr.Plan, pr.ID)]
			c.Retries += retriesBefore(rh, pr.At)
			if rr, ok := rh.StateAt(pr.At); ok {
				c.Weight += max(rr.Weight, 1)
			} else {
				c.Weight++
			}
		}
		s, known := effort[so.SessionID]
		k := float64(len(order))
		for _, key := range order {
			c := byCommit[key]
			c.Known = known
			if known {
				c.Effort = agentIdentityPart(s.Effort, "")
				c.Tokens = float64(s.TokensIn+s.TokensOut) / k
				c.Turns = float64(s.Turns) / k
				c.WallSeconds = float64(s.WallSeconds) / k
				c.Reverts = float64(s.Reverts) / k
			}
			c.Agent = agentIdentityKey(c.Harness, c.Model, c.Effort)
			switch {
			case !known:
				c.USD = Absent[float64]("the proving session has no effort row in the session ledger")
			case s.USD == nil:
				c.USD = Absent[float64](fmt.Sprintf("model %q is not in the price table", s.Model))
			default:
				c.USD = Measured(*s.USD/k, "the price table over the transcript fold")
			}
			if cf, ok := commits[c.Commit]; ok && c.Commit != "" {
				c.Zones = ChangeZones(cf.Files, z)
				if cf.ReworkKnown {
					c.Rework = Measured(cf.ReworkLines, "git log over the 7 days after the commit")
				} else {
					c.Rework = Absent[int](orReason(cf.ReworkReason, "rework within 7 days not read for this commit"))
				}
				if cf.RevertsKnown {
					c.Reverted = Measured(len(cf.Reverts), "git log: later commits reverting it")
				} else {
					c.Reverted = Absent[int](orReason(cf.RevertReason, "reverts not read for this commit"))
				}
			} else {
				c.Rework = Absent[int]("the commit's files were not read")
				c.Reverted = Absent[int]("the commit was not read")
			}
			out = append(out, *c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].At != out[j].At {
			return out[i].At < out[j].At
		}
		return out[i].Commit < out[j].Commit
	})
	return out
}

// orReason is reason, else fallback.
func orReason(reason, fallback string) string {
	if reason != "" {
		return reason
	}
	return fallback
}

// retriesBefore counts the row's claimed-unproven readings before at.
func retriesBefore(rh RowHistory, at string) int {
	n := 0
	for _, r := range rh.Readings {
		if r.At >= at {
			break
		}
		if r.State == HistUnproven {
			n++
		}
	}
	return n
}

// ─── CI minutes (WI-2) ──────────────────────────────────────────────────────

// CIRun is one CI run's minutes, bound to the commit it built.
type CIRun struct {
	Commit  string  `json:"commit"`
	Minutes float64 `json:"minutes"`
	Source  string  `json:"source"`
}

type junitSuites struct {
	XMLName xml.Name
	Time    string        `xml:"time,attr"`
	Suites  []junitSuites `xml:"testsuite"`
}

type workflowRun struct {
	HeadSHA      string `json:"head_sha"`
	RunStartedAt string `json:"run_started_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ParseCISummary reads a CI summary file the user names: JUnit XML (its
// `time` totals, in seconds) or a workflow-run JSON export (one run, or
// `{"workflow_runs": [...]}`, each run's updated_at minus run_started_at).
// A run is bound to its head_sha, else to commit; a run bound to no
// commit is refused, never guessed.
func ParseCISummary(name string, data []byte, commit string) ([]CIRun, error) {
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "<") {
		var s junitSuites
		if err := xml.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("%s: JUnit XML: %w", name, err)
		}
		if commit == "" {
			return nil, fmt.Errorf("%s: a JUnit summary names no commit; name the commit it built", name)
		}
		secs, ok := junitSeconds(s)
		if !ok {
			return nil, fmt.Errorf("%s: no time attribute on the suites", name)
		}
		return []CIRun{{Commit: commit, Minutes: secs / 60, Source: name}}, nil
	}
	var export struct {
		Runs []workflowRun `json:"workflow_runs"`
	}
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("%s: workflow-run JSON: %w", name, err)
	}
	if export.Runs == nil {
		var one workflowRun
		if err := json.Unmarshal(data, &one); err == nil && one.RunStartedAt != "" {
			export.Runs = []workflowRun{one}
		}
	}
	if len(export.Runs) == 0 {
		return nil, fmt.Errorf("%s: neither JUnit XML nor a workflow-run export", name)
	}
	var out []CIRun
	for _, r := range export.Runs {
		st, err1 := time.Parse(time.RFC3339, r.RunStartedAt)
		en, err2 := time.Parse(time.RFC3339, r.UpdatedAt)
		if err1 != nil || err2 != nil || en.Before(st) {
			return nil, fmt.Errorf("%s: a run without a readable run_started_at/updated_at", name)
		}
		c := r.HeadSHA
		if c == "" {
			c = commit
		}
		if c == "" {
			return nil, fmt.Errorf("%s: a run with no head_sha and no commit named", name)
		}
		out = append(out, CIRun{Commit: c, Minutes: en.Sub(st).Minutes(), Source: name})
	}
	return out, nil
}

// junitSeconds is the root's time, else the sum of its suites' times.
func junitSeconds(s junitSuites) (float64, bool) {
	if v, err := strconv.ParseFloat(s.Time, 64); err == nil {
		return v, true
	}
	total, any := 0.0, false
	for _, sub := range s.Suites {
		if v, ok := junitSeconds(sub); ok {
			total += v
			any = true
		}
	}
	return total, any
}

// ─── cost per zone (WI-2) ───────────────────────────────────────────────────

// ZoneCost is U1 for one zone: every figure is the share-weighted sum of
// the proven changes touching it.
type ZoneCost struct {
	Zone      string         `json:"zone"`
	N         int            `json:"n"`
	Changes   float64        `json:"changes"`
	Tokens    float64        `json:"tokens"`
	USD       Value[float64] `json:"usd"`
	CIMinutes Value[float64] `json:"ci_minutes"`
}

// costPerZone rolls proven changes up by zone, with the CI minutes of
// the runs bound to their commits.
func costPerZone(changes []ProvenChange, ci []CIRun) []ZoneCost {
	minutes := map[string]float64{}
	for _, r := range ci {
		minutes[r.Commit] += r.Minutes
	}
	type acc struct {
		ZoneCost
		usd, ci                      float64
		priced, unpriced, run, noRun int
	}
	zones := map[string]*acc{}
	for _, c := range changes {
		for _, zs := range c.Zones {
			a := zones[zs.Zone]
			if a == nil {
				a = &acc{ZoneCost: ZoneCost{Zone: zs.Zone}}
				zones[zs.Zone] = a
			}
			a.N++
			a.Changes += zs.Share
			a.Tokens += c.Tokens * zs.Share
			if c.USD.Known() {
				a.usd += c.USD.V * zs.Share
				a.priced++
			} else {
				a.unpriced++
			}
			if m, ok := minutes[c.Commit]; ok {
				a.ci += m * zs.Share
				a.run++
			} else {
				a.noRun++
			}
		}
	}
	out := make([]ZoneCost, 0, len(zones))
	for _, a := range zones {
		zc := a.ZoneCost
		zc.USD = shareValue(a.usd, a.priced, a.unpriced, "the price table over the transcript fold",
			"no proving session's model is priced", "%d of %d changes unpriced")
		switch {
		case len(ci) == 0:
			zc.CIMinutes = Absent[float64]("no CI summary file named")
		default:
			zc.CIMinutes = shareValue(a.ci, a.run, a.noRun, "the named CI summary file",
				"no CI run is bound to these changes' commits", "%d of %d changes have no CI run bound to their commit")
		}
		out = append(out, zc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Zone < out[j].Zone })
	return out
}

// shareValue is a sum over have measured items with miss unmeasured:
// absent when none were measured, partial (a named lower bound) when some
// were not.
func shareValue(sum float64, have, miss int, source, noneReason, partialFmt string) Value[float64] {
	switch {
	case have == 0:
		return Absent[float64](noneReason)
	case miss > 0:
		return Partial(round2(sum), fmt.Sprintf(partialFmt, miss, have+miss)).WithSource(source)
	}
	return Measured(round2(sum), source)
}

// ─── first-attempt split (WI-3) ─────────────────────────────────────────────

// AttemptSide is one side of the split.
type AttemptSide struct {
	N            int            `json:"n"`
	Tokens       float64        `json:"tokens"`
	MedianTokens Value[float64] `json:"median_tokens"`
}

// AttemptSplit is one zone's (or the fleet's) first-attempt and retried
// proofs.
type AttemptSplit struct {
	Zone    string      `json:"zone"`
	First   AttemptSide `json:"first_attempt"`
	Retried AttemptSide `json:"after_retries"`
}

// FirstAttemptModel is the split for the fleet and per zone.
type FirstAttemptModel struct {
	Fleet AttemptSplit   `json:"fleet"`
	Zones []AttemptSplit `json:"zones"`
}

// firstAttemptSplit splits proven changes into those proven with no
// claimed-unproven reading before and those proven after retries.
func firstAttemptSplit(changes []ProvenChange) FirstAttemptModel {
	type sides struct{ first, retried []float64 }
	fleet := sides{}
	zones := map[string]*sides{}
	for _, c := range changes {
		if !c.Known {
			continue
		}
		add := func(s *sides, v float64) {
			if c.FirstAttempt() {
				s.first = append(s.first, v)
			} else {
				s.retried = append(s.retried, v)
			}
		}
		add(&fleet, c.Tokens)
		for _, zs := range c.Zones {
			s := zones[zs.Zone]
			if s == nil {
				s = &sides{}
				zones[zs.Zone] = s
			}
			add(s, c.Tokens*zs.Share)
		}
	}
	side := func(vs []float64) AttemptSide {
		s := AttemptSide{N: len(vs)}
		for _, v := range vs {
			s.Tokens += v
		}
		s.Tokens = round2(s.Tokens)
		if len(vs) == 0 {
			s.MedianTokens = Absent[float64]("no proven change on this side")
		} else {
			s.MedianTokens = Measured(round2(quantile(vs, 0.5)), "the transcript fold joined to proof")
		}
		return s
	}
	m := FirstAttemptModel{Fleet: AttemptSplit{Zone: "all", First: side(fleet.first), Retried: side(fleet.retried)}}
	names := make([]string, 0, len(zones))
	for z := range zones {
		names = append(names, z)
	}
	sort.Strings(names)
	for _, z := range names {
		m.Zones = append(m.Zones, AttemptSplit{Zone: z, First: side(zones[z].first), Retried: side(zones[z].retried)})
	}
	return m
}

// quantile is the linear-interpolated q-quantile of vs (not modified).
func quantile(vs []float64, q float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	s := append([]float64(nil), vs...)
	sort.Float64s(s)
	pos := q * float64(len(s)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	return s[lo] + (s[hi]-s[lo])*(pos-float64(lo))
}

// ─── the interest rate (WI-5) ───────────────────────────────────────────────

// InterestMeasure names the effort figure U2's rate is computed over.
const InterestMeasure = "tokens per proven change, split evenly across the change's zones"

// AgentCount is how many of a zone's changes one agent identity proved.
type AgentCount struct {
	Agent string `json:"agent"`
	N     int    `json:"n"`
}

// ZoneInterest is one zone's interest rate.
type ZoneInterest struct {
	Zone     string         `json:"zone"`
	N        int            `json:"n"`
	Median   float64        `json:"median"`
	Q1       float64        `json:"q1"`
	Q3       float64        `json:"q3"`
	Rate     Value[float64] `json:"rate"`
	ModelMix []AgentCount   `json:"model_mix"`
}

// InterestModel is U2.
type InterestModel struct {
	Measure     string         `json:"measure"`
	FleetMedian Value[float64] `json:"fleet_median"`
	Zones       []ZoneInterest `json:"zones"`
	Unmeasured  int            `json:"unmeasured_changes"`
}

// InterestRate is each zone's median effort per proven change over the
// fleet median (the median over every zone attribution), with n, the
// interquartile range and the agent identities behind it — a zone worked
// mostly by a small model is named as such, never blamed. Highest rate
// first.
func InterestRate(changes []ProvenChange) InterestModel {
	m := InterestModel{Measure: InterestMeasure, Zones: []ZoneInterest{}}
	var all []float64
	per := map[string][]float64{}
	mix := map[string]map[string]int{}
	for _, c := range changes {
		if !c.Known {
			m.Unmeasured++
			continue
		}
		for _, zs := range c.Zones {
			v := c.Tokens * zs.Share
			all = append(all, v)
			per[zs.Zone] = append(per[zs.Zone], v)
			if mix[zs.Zone] == nil {
				mix[zs.Zone] = map[string]int{}
			}
			mix[zs.Zone][c.Agent]++
		}
	}
	if len(all) == 0 {
		m.FleetMedian = Absent[float64]("no measured proven change in view")
		return m
	}
	fleet := quantile(all, 0.5)
	m.FleetMedian = Measured(round2(fleet), "the transcript fold joined to proof").
		WithDenominator(fmt.Sprintf("%d zone attributions", len(all)))
	for z, vs := range per {
		zi := ZoneInterest{Zone: z, N: len(vs), Median: round2(quantile(vs, 0.5)), Q1: round2(quantile(vs, 0.25)), Q3: round2(quantile(vs, 0.75))}
		if fleet == 0 {
			zi.Rate = Absent[float64]("the fleet median is zero tokens")
		} else {
			zi.Rate = Measured(round2(zi.Median/fleet), "zone median over fleet median").WithDenominator(fmt.Sprintf("n = %d", zi.N))
		}
		for a, n := range mix[z] {
			zi.ModelMix = append(zi.ModelMix, AgentCount{Agent: a, N: n})
		}
		sort.Slice(zi.ModelMix, func(i, j int) bool {
			if zi.ModelMix[i].N != zi.ModelMix[j].N {
				return zi.ModelMix[i].N > zi.ModelMix[j].N
			}
			return zi.ModelMix[i].Agent < zi.ModelMix[j].Agent
		})
		m.Zones = append(m.Zones, zi)
	}
	sort.Slice(m.Zones, func(i, j int) bool {
		if m.Zones[i].Rate.V != m.Zones[j].Rate.V {
			return m.Zones[i].Rate.V > m.Zones[j].Rate.V
		}
		return m.Zones[i].Zone < m.Zones[j].Zone
	})
	return m
}

// ─── agent identity (WI-14) ─────────────────────────────────────────────────

// agentIdentityKey is the only row key a work-intelligence or progress
// view uses: harness, model and effort setting — the agent identity,
// never a person (I58).
func agentIdentityKey(harness, model, effort string) string {
	return agentIdentityPart(harness, "unknown") + "|" + agentIdentityPart(model, "unknown") + "|" + agentIdentityPart(effort, "unknown")
}

// agentIdentityPart is one part of an agent identity: a part that is
// empty is fallback, and a part carrying an email address or a git
// "Name <email>" author form is never keyed — it becomes
// IdentityWithheld (I58: a human name or email is never a row key).
func agentIdentityPart(s, fallback string) string {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return fallback
	case strings.ContainsAny(s, "@<>"):
		return IdentityWithheld
	}
	return strings.ReplaceAll(s, "|", "/")
}

// IdentityWithheld replaces an identity part that looked like a person.
const IdentityWithheld = "withheld (not an agent identity)"
