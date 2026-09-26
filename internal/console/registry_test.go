package console

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestRegistry_ExcludesOnlyNamedUnlistedServices pins the rule's edges: a
// row naming no service is unattributed, never excluded; nothing is
// excluded when the registry could not be read, and the note says so.
func TestRegistry_ExcludesOnlyNamedUnlistedServices(t *testing.T) {
	r := RegistryOf([]string{"fathom", "App-Kit"})
	for svc, want := range map[string]bool{"fathom": false, "app-kit": false, "APP-KIT": false, "": false, "svc": true, "Ghost": true} {
		if got := r.Excludes(svc); got != want {
			t.Errorf("Excludes(%q) = %v, want %v", svc, got, want)
		}
	}
	u := UnregisteredOf([]string{"fathom", "svc", "", "SVC", "ghost", "svc"}, r)
	if u.Rows != 4 || !reflect.DeepEqual(u.Services, []string{"ghost", "svc"}) || u.RegistryNote != "" {
		t.Errorf("UnregisteredOf = %+v, want 4 rows naming ghost and svc", u)
	}
	if got := u.Phrase("review row"); got != "4 review rows naming 2 services the registry does not list (ghost, svc) excluded" {
		t.Errorf("Phrase = %q", got)
	}
	unread := UnregisteredOf([]string{"svc"}, RegistryUnread("service registry unreachable"))
	if unread.Rows != 0 || unread.RegistryNote != "service registry unreachable" || unread.Phrase("row") != "" {
		t.Errorf("an unread registry excludes nothing and says so: %+v", unread)
	}
}

// TestCatchTimeline_ExcludesServicesTheRegistryDoesNotList pins OC-H2 on
// the catches timeline: the fixture service "svc"'s review rows (the 55
// "conserve losses" that were test pollution) are not the fleet's catches.
// Every lane leaves out rows naming a service the registry does not list,
// and names them — count and ids — in the lane and on the model.
func TestCatchTimeline_ExcludesServicesTheRegistryDoesNotList(t *testing.T) {
	at := func(s string) time.Time { v, _ := time.Parse(time.RFC3339, s); return v }
	in := CatchInputs{
		VerifyRuns: []VerifyRun{
			{At: "2026-09-20T10:00:00Z", Service: "fathom", Plan: "docs/plans/a.md", Failing: 1, Claimed: 4},
			{At: "2026-09-20T11:00:00Z", Service: "ghost", Plan: "docs/plans/g.md", Failing: 3, Claimed: 3},
		},
		Reviews: []ReviewRun{
			{At: "2026-09-23T10:00:00Z", Service: "fathom", Outcome: map[string]int{"conservation": 1, "integrity": 0}},
			{At: "2026-09-26T05:15:42Z", Service: "svc", Outcome: map[string]int{"conservation": 1, "integrity": 0}},
			{At: "2026-09-26T05:15:46Z", Service: "svc", Outcome: map[string]int{"conservation": 1, "integrity": 1}},
		},
		GuardBeats: []GuardBeat{
			{At: at("2026-09-24T11:00:01Z"), Kind: "denied", Target: "file", Service: "fathom"},
			{At: at("2026-09-24T11:00:02Z"), Kind: "denied", Target: "file", Service: "001"},
			{At: at("2026-09-24T11:00:03Z"), Kind: "denied", Target: "command"}, // unattributed: kept
		},
		Registry: RegistryOf([]string{"fathom"}),
	}
	m := CatchTimeline(in, CatchFilter{}, 1, 50)
	lane := func(k CatchKind) Value[int] {
		for _, s := range m.Kinds {
			if s.Kind == k {
				return s.Count
			}
		}
		t.Fatalf("no lane %s", k)
		return Value[int]{}
	}
	if v := lane(CatchConserveLoss); v.V != 1 || !strings.Contains(v.Denominator, "2 review rows naming a service the registry does not list (svc) excluded") {
		t.Errorf("conserve lane = %+v, want 1 (fathom's), naming svc's 2 rows", v)
	}
	if v := lane(CatchIntegrity); v.V != 0 || !strings.Contains(v.Denominator, "(svc) excluded") {
		t.Errorf("integrity lane = %+v, want a measured zero naming svc", v)
	}
	if v := lane(CatchVerifyRefusal); v.V != 1 || !strings.Contains(v.Denominator, "of 1 recorded verify run") || !strings.Contains(v.Denominator, "1 verify run naming a service the registry does not list (ghost) excluded") {
		t.Errorf("verify lane = %+v", v)
	}
	if v := lane(CatchGuardRefusal); v.V != 2 || !strings.Contains(v.Denominator, "(001) excluded") {
		t.Errorf("guard lane = %+v, want fathom's and the unattributed denial", v)
	}
	if m.Total != 4 {
		t.Errorf("total = %d, want 4 catches (1 verify, 1 conserve, 2 guard)", m.Total)
	}
	for _, c := range m.Catches {
		if c.Service == "svc" || c.Service == "ghost" || c.Service == "001" {
			t.Errorf("an excluded row reached the table: %+v", c)
		}
	}
	want := Unregistered{Rows: 4, Services: []string{"001", "ghost", "svc"}}
	if !reflect.DeepEqual(m.Unregistered, want) {
		t.Errorf("model unregistered = %+v, want %+v", m.Unregistered, want)
	}
	if in.NamesService("svc") || !in.ExcludesNamedService("svc") || in.ExcludesNamedService("fathom") || in.ExcludesNamedService("nobody") {
		t.Error("an excluded row names no service the timeline could show, and ExcludesNamedService says so")
	}

	// Only excluded rows in a lane: the lane is not instrumented, and says
	// its rows were excluded rather than reading as silence.
	onlySvc := in
	onlySvc.Reviews = in.Reviews[1:]
	m2 := CatchTimeline(onlySvc, CatchFilter{}, 1, 50)
	for _, s := range m2.Kinds {
		if s.Kind == CatchConserveLoss && (s.Count.State != StateAbsent || !strings.Contains(s.Count.Reason, "(svc) excluded")) {
			t.Errorf("a lane whose every row was excluded = %+v", s.Count)
		}
	}

	// An unread registry excludes nothing and the model says why.
	unread := in
	unread.Registry = RegistryUnread("service registry unreachable")
	m3 := CatchTimeline(unread, CatchFilter{}, 1, 50)
	if m3.Total != 9 || m3.Unregistered.Rows != 0 || m3.Unregistered.RegistryNote != "service registry unreachable" {
		t.Errorf("unread registry: total %d, unregistered %+v", m3.Total, m3.Unregistered)
	}
}

// TestCallTotal_NamesUnregisteredRows: calls.total is every row in the log
// — it never drops one — and its denominator splits the attributed rows
// into those naming a registered service and those naming none it lists.
func TestCallTotal_NamesUnregisteredRows(t *testing.T) {
	f := CallLogFacts{Log: ResolvedCallLog{Path: "/l", Rule: RuleDaemon, Note: "n"}, Rows: 100, Unattributed: 10,
		Unregistered: Unregistered{Rows: 5, Services: []string{"svc"}}}
	v := CallTotal(f)
	if v.V != 100 || !strings.Contains(v.Denominator, "85 attributed to a registered service") || !strings.Contains(v.Denominator, "5 rows naming a service the registry does not list (svc)") || !strings.Contains(v.Denominator, "10 unattributed") {
		t.Errorf("CallTotal = %+v", v)
	}
	f.Unregistered = Unregistered{}
	if v := CallTotal(f); !strings.Contains(v.Denominator, "90 attributed to a service, 10 unattributed") {
		t.Errorf("no exclusion keeps the old denominator: %q", v.Denominator)
	}
}
