package explmap

import (
	"sort"
	"time"
)

// LifeState is where a map is in its lifetime.
type LifeState string

const (
	// Pinned: a map that is alive includes it; its idle clock is stopped.
	Pinned LifeState = "pinned"
	// Permanent: a named map; it does not decay while unreferenced.
	Permanent LifeState = "permanent"
	// Open: unreferenced and not ended; idle since IdleSince. How long an
	// idle map is kept is deliberately not decided here: the duration is set
	// when live maps are built, from XM-M3's reuse-gap reading (design
	// section 2), so this reports the clock and never expires on it.
	Open LifeState = "open"
	// Ended: its own thread condition held (or it was closed) and nothing
	// alive includes it.
	Ended LifeState = "ended"
)

// Lifetime is one map's state under the reference rule.
type Lifetime struct {
	State LifeState
	// PinnedBy are the alive maps that include it directly, sorted.
	PinnedBy []string
	// IdleSince is when its idle clock (re)started: the later of its own
	// last use and the moment its last reference dropped. Zero while pinned
	// or ended.
	IdleSince time.Time
}

// Lifetimes applies the reference rule to every map in all (XM-6).
func Lifetimes(all Maps) map[string]Lifetime {
	alive := map[string]bool{}
	out := make(map[string]Lifetime, len(all))
	for _, id := range all.IDs() {
		out[id] = mapLifetime(id, all, alive)
	}
	return out
}

// mapLifetime is the lifetime rule for one map (XM-6): lifetimes follow
// references. A map is pinned while a map that is itself alive includes it:
// "alive" is a live map (a stored query, always alive while it exists) or any
// map whose own condition has not ended it (a named one included), and a map pinned
// by an alive map is alive in turn, so pinning follows the DAG. While pinned its
// idle clock is stopped; when the last such reference drops (the referrer drops
// the include, or ends while nothing keeps it alive) the clock restarts at that
// moment. An unreferenced named map is permanent; an unreferenced map whose
// condition held is ended. aliveMemo caches aliveness across calls.
func mapLifetime(id string, all Maps, aliveMemo map[string]bool) Lifetime {
	m := all[id]
	var pinnedBy []string
	lastDrop := time.Time{}
	for _, rid := range all.IDs() {
		if rid == id {
			continue
		}
		r := all[rid]
		if containsString(r.Includes, id) {
			if isAlive(rid, all, aliveMemo, map[string]bool{}) {
				pinnedBy = append(pinnedBy, rid)
				continue
			}
			// It still names the map but has ended: the reference dropped
			// when the referrer ended.
			if r.EndedAt.After(lastDrop) {
				lastDrop = r.EndedAt
			}
		}
		for _, d := range r.Drops {
			if d.ID == id && d.At.After(lastDrop) {
				lastDrop = d.At
			}
		}
	}
	sort.Strings(pinnedBy)
	switch {
	case len(pinnedBy) > 0:
		return Lifetime{State: Pinned, PinnedBy: pinnedBy}
	case m.Ended():
		return Lifetime{State: Ended}
	}
	idle := m.LastUsed
	if m.CreatedAt.After(idle) {
		idle = m.CreatedAt
	}
	if lastDrop.After(idle) {
		idle = lastDrop
	}
	if m.Named() {
		return Lifetime{State: Permanent, IdleSince: idle}
	}
	return Lifetime{State: Open, IdleSince: idle}
}

// isAlive reports whether a map keeps what it includes: a live map, a map not
// ended (a named map is not ended until it is closed), or one pinned by an
// alive map. visiting guards a cycle
// in corrupt data (cycles are refused at write): a map on its own route is not
// alive through itself.
func isAlive(id string, all Maps, memo, visiting map[string]bool) bool {
	if v, ok := memo[id]; ok {
		return v
	}
	m, ok := all[id]
	if !ok || visiting[id] {
		return false
	}
	if m.Kind == KindLive || !m.Ended() {
		memo[id] = true
		return true
	}
	visiting[id] = true
	alive := false
	for _, rid := range all.IDs() {
		if rid != id && containsString(all[rid].Includes, id) && isAlive(rid, all, memo, visiting) {
			alive = true
			break
		}
	}
	delete(visiting, id)
	memo[id] = alive
	return alive
}
