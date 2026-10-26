package main

import (
	"sort"
)

type selectedLayer struct {
	ref   layerRef
	value layer
}

func compose(p project, catalog map[string]control, sources []selectedLayer) ([]effectiveControl, error) {
	result := make([]effectiveControl, 0, len(p.ControlIDs))
	for _, id := range p.ControlIDs {
		root := catalog[id]
		e := effectiveControl{ID: id, Event: root.Event, Class: root.Class, MinSamplePPM: root.MinSamplePPM, MaxSamplePPM: root.MaxSamplePPM, SamplePPM: root.DefaultSamplePPM, MaxWorkMillis: root.MaxWorkMillis, OnFault: root.OnFault, AllowMaintenance: root.AllowMaintenance, TunableBy: append([]string{}, root.TunableBy...), ConstrainedBy: []string{"registration"}}
		for _, src := range sources {
			for _, c := range src.value.Controls {
				if c.ID != id {
					continue
				}
				e.ConstrainedBy = append(e.ConstrainedBy, src.ref.ID)
				if c.MinSamplePPM > e.MinSamplePPM {
					e.MinSamplePPM = c.MinSamplePPM
				}
				if c.MaxSamplePPM < e.MaxSamplePPM {
					e.MaxSamplePPM = c.MaxSamplePPM
				}
				if c.MaxWorkMillis < e.MaxWorkMillis {
					e.MaxWorkMillis = c.MaxWorkMillis
				}
				if c.OnFault == "hold" {
					e.OnFault = "hold"
				}
				e.AllowMaintenance = e.AllowMaintenance && c.AllowMaintenance
				e.TunableBy = intersect(e.TunableBy, c.TunableBy)
				if root.Class == "mandatory" && !mandatory(c.MinSamplePPM, c.MaxSamplePPM, 1000000, c.OnFault, c.AllowMaintenance, c.TunableBy) {
					return nil, errPolicy
				}
			}
		}
		if e.MinSamplePPM > e.MaxSamplePPM {
			return nil, errPolicy
		}
		best := -1
		hasPreference := false
		for _, src := range sources {
			for _, pref := range src.value.Preferences {
				if pref.ControlID != id {
					continue
				}
				if root.Class == "mandatory" || !contains(e.TunableBy, src.ref.ID) || pref.SamplePPM < e.MinSamplePPM || pref.SamplePPM > e.MaxSamplePPM {
					return nil, errPreference
				}
				if src.ref.Priority > best {
					best = src.ref.Priority
					e.SamplePPM = pref.SamplePPM
					e.TunedBy = src.ref.ID
					hasPreference = true
				}
			}
		}
		if !hasPreference && (e.SamplePPM < e.MinSamplePPM || e.SamplePPM > e.MaxSamplePPM) {
			return nil, errPolicy
		}
		if root.Class == "mandatory" && !mandatory(e.MinSamplePPM, e.MaxSamplePPM, e.SamplePPM, e.OnFault, e.AllowMaintenance, e.TunableBy) {
			return nil, errPolicy
		}
		sort.Strings(e.TunableBy)
		sort.Strings(e.ConstrainedBy)
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}
func intersect(a, b []string) []string {
	out := []string{}
	for _, x := range a {
		if contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}
