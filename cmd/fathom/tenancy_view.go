package main

// tenancy_view.go — the one tenancy projection (design §13.2, §13.3;
// RT-52). /statusz's tenancy block is tenancyStatus over the live
// governor; `fathom resources`, doctor and the primer read the same block
// — from /statusz when the daemon answers, else rebuilt from the resource
// ring by tenancyFromRing (a stopped daemon is still visible, I38). The
// fields both sources can carry are the same fields with the same meaning
// (TestStatuszTenancyBlockMatchesResources); the live-only ones (every
// in-flight call with its origin, the last transition's reason) are
// omitted from a ring projection, never zero-filled.

import (
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// inflightView is one piece of in-flight work in the projection.
type inflightView struct {
	Class   string  `json:"class"`
	Tool    string  `json:"tool"`
	Service string  `json:"service,omitempty"`
	Origin  string  `json:"origin,omitempty"`
	AgeSec  float64 `json:"age_seconds"`
}

// diagView is the newest diagnostic capture (§13.4).
type diagView struct {
	Path    string `json:"path"`
	At      string `json:"at,omitempty"`
	Trigger string `json:"trigger,omitempty"`
}

// refusalsNotCounted is the refusals field until the admission gate
// (RT-53) counts refusals: disclosed, never rendered as zero (I38).
const refusalsNotCounted = "not counted: the admission gate (RT-53) is not built yet"

// modeOf names the daemon's mode.
func modeOf(safe bool) string {
	if safe {
		return "safe"
	}
	return "normal"
}

// tenancyStatus is /statusz's tenancy block: the live governor's
// projection (§13.3 — level and since, footprint vs budget, in-flight by
// class with tool and origin, refusals, mode, last diag).
func tenancyStatus(g *governor) *governorSnapshot {
	if g == nil {
		return nil
	}
	g.mu.Lock() // one lock: every sample-derived field is one sample's
	s := g.snapshotLocked()
	now := g.now()
	s.Source = "live"
	if g.haveSmp {
		s.SampleAt = g.sample.At.UTC().Format(time.RFC3339)
		s.SwapTotal = g.sample.SwapTotal
	}
	s.Mode = modeOf(g.safeMode())
	if g.diagPath != "" {
		s.LastDiag = &diagView{Path: g.diagPath, At: g.diagAt.UTC().Format(time.RFC3339), Trigger: g.diagWhy}
	}
	g.mu.Unlock()
	snap := g.inflight.Snapshot()
	s.InFlightByClass = map[string]int{}
	for _, c := range tenancy.Classes {
		s.InFlightByClass[c.String()] = 0
	}
	for _, e := range snap {
		s.InFlightByClass[e.Class.String()]++
		s.InFlight = append(s.InFlight, inflightView{Class: e.Class.String(), Tool: e.Tool, Service: e.Service, Origin: e.Origin, AgeSec: now.Sub(e.Started).Seconds()})
	}
	if len(snap) > 0 {
		o := s.InFlight[0]
		s.Oldest = &o
	}
	s.Refusals = refusalsNotCounted
	return &s
}

// tenancyFromRing rebuilds the projection from the ring: the newest
// record's sample, the newest transition's level time, the header's
// budget. ok is false when the ring holds no record. quarantined is the
// ledger's quarantine (Quarantine.Attention lines).
func tenancyFromRing(rd tenancy.RingRead, quarantined []string) (*governorSnapshot, bool) {
	last, ok := rd.Last()
	if !ok {
		return nil, false
	}
	s := &governorSnapshot{
		Source: "ring", SampleAt: last.At.UTC().Format(time.RFC3339),
		Level: last.Level.String(), LadderOff: last.LadderOff, Mode: modeOf(last.SafeMode),
		Footprint: last.Footprint, Children: last.Children,
		Pressure: last.Pressure.String(), SwapUsed: last.SwapUsed, SwapTotal: last.SwapTotal,
		DiskWritten: last.DiskWritten, GCCPUFraction: last.GCCPUFraction,
		Abandoned: int64(last.Abandoned), Quarantined: quarantined, Refusals: refusalsNotCounted,
	}
	if rd.HasMeta {
		m := rd.Meta
		s.Budget, s.Shed, s.Refuse, s.Exit, s.ExitEnabled = m.Budget, m.Shed, m.Refuse, m.Exit, m.ExitEnabled
	}
	if tr, ok := rd.LastTransition(); ok {
		if tr.Level == last.Level { // else the step that led here was not recorded: since unknown
			s.Since = tr.At.UTC().Format(time.RFC3339)
		}
		s.LastTransition = fmt.Sprintf("%s → %s", tr.At.UTC().Format(time.RFC3339), tr.Level)
	}
	smp := tenancy.Sample{
		Footprint: last.Footprint, Children: last.Children, DiskWritten: last.DiskWritten,
		Pressure: last.Pressure, SwapUsed: last.SwapUsed, SwapTotal: last.SwapTotal,
		StoreFree: last.StoreFree, TmpFree: last.TmpFree, SwapVolumeFree: last.SwapVolumeFree,
	}
	s.Unknowns = smp.Unknowns()
	s.InFlightByClass = map[string]int{}
	for _, c := range tenancy.Classes {
		s.InFlightByClass[c.String()] = int(last.InFlight[c])
	}
	if last.InFlightTotal() > 0 && last.OldestTool != "" {
		s.Oldest = &inflightView{Class: last.OldestClass.String(), Tool: last.OldestTool, AgeSec: last.OldestAge.Seconds()}
	}
	return s, true
}
