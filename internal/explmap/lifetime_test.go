package explmap

import (
	"reflect"
	"testing"
	"time"
)

func at(h int) time.Time { return t0.Add(time.Duration(h) * time.Hour) }

func used(m Map, h int) Map {
	m.CreatedAt, m.LastUsed = at(h), at(h)
	return m
}

func composite(id string, h int, includes ...string) Map {
	return used(Map{ID: id, Kind: KindComposite, Includes: includes}, h)
}

// TestMapLifetimeFollowsReferences pins XM-6: an ephemeral map is pinned while
// a map that is alive includes it (a live map, a named map, or one whose own
// condition has not ended it), and its idle clock restarts when the last such
// reference drops, whether the referrer dropped the include or ended.
func TestMapLifetimeFollowsReferences(t *testing.T) {
	e := used(eph("e"), 0)

	// Referenced by an alive composite: pinned, the clock stopped.
	all := Maps{"e": e, "c": composite("c", 1, "e")}
	lt := Lifetimes(all)
	if got := lt["e"]; got.State != Pinned || !reflect.DeepEqual(got.PinnedBy, []string{"c"}) || !got.IdleSince.IsZero() {
		t.Fatalf("e = %+v, want pinned by [c] with no idle clock", got)
	}
	if got := lt["c"]; got.State != Open || !got.IdleSince.Equal(at(1)) {
		t.Errorf("c = %+v, want open, idle since its last use", got)
	}

	// The referrer ends at hour 3: the last reference dropped, e's clock
	// restarts then (not at its own last use, hour 0).
	c := all["c"]
	c.EndedAt = at(3)
	all["c"] = c
	lt = Lifetimes(all)
	if got := lt["e"]; got.State != Open || !got.IdleSince.Equal(at(3)) {
		t.Errorf("after the referrer ended: e = %+v, want open, idle since hour 3", got)
	}
	if got := lt["c"]; got.State != Ended {
		t.Errorf("c = %+v, want ended", got)
	}

	// Two referrers: one drops the include at hour 2, the other still holds
	// it; e stays pinned. When the second drops it at hour 5, the clock
	// restarts at hour 5, the LAST drop.
	c1 := used(eph("c1"), 1)
	c1.Drops = []IncludeDrop{{ID: "e", At: at(2)}}
	c2 := used(eph("c2", "e"), 1)
	all = Maps{"e": e, "c1": c1, "c2": c2}
	if got := Lifetimes(all)["e"]; got.State != Pinned || !reflect.DeepEqual(got.PinnedBy, []string{"c2"}) {
		t.Errorf("one referrer left: e = %+v, want pinned by [c2]", got)
	}
	c2.Includes = nil
	c2.Drops = []IncludeDrop{{ID: "e", At: at(5)}}
	all["c2"] = c2
	if got := Lifetimes(all)["e"]; got.State != Open || !got.IdleSince.Equal(at(5)) {
		t.Errorf("both dropped: e = %+v, want open, idle since hour 5", got)
	}

	// Pinning is transitive through alive maps: a live map includes an
	// ended composite, which includes e. The live map keeps the composite
	// alive, which keeps e pinned.
	ended := composite("xm-k", 1, "e")
	ended.EndedAt = at(2)
	live := used(Map{ID: "l", Kind: KindLive, Includes: []string{"xm-k"}}, 1)
	all = Maps{"e": e, "xm-k": ended, "l": live}
	lt = Lifetimes(all)
	if lt["xm-k"].State != Pinned || lt["e"].State != Pinned || !reflect.DeepEqual(lt["e"].PinnedBy, []string{"xm-k"}) {
		t.Errorf("transitive pin: xm-k %+v, e %+v; want both pinned", lt["xm-k"], lt["e"])
	}

	// A named map is permanent and pins what it includes.
	named := used(eph("n", "e"), 4)
	named.Name, named.Purpose = "auth", "Which handlers skip auth?"
	all = Maps{"e": e, "n": named}
	lt = Lifetimes(all)
	if lt["n"].State != Permanent || lt["e"].State != Pinned {
		t.Errorf("named: n %+v, e %+v; want permanent and pinned", lt["n"], lt["e"])
	}

	// An unreferenced ephemeral map whose condition held is ended.
	gone := used(eph("g"), 0)
	gone.EndedAt = at(1)
	if got := Lifetimes(Maps{"g": gone})["g"]; got.State != Ended {
		t.Errorf("g = %+v, want ended", got)
	}

	// mapLifetime is the per-map rule Lifetimes applies.
	if got := mapLifetime("e", Maps{"e": e}, map[string]bool{}); got.State != Open || !got.IdleSince.Equal(at(0)) {
		t.Errorf("alone: e = %+v, want open since hour 0", got)
	}
}
