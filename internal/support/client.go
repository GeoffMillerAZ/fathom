package support

// client.go — the client registry adapter (SM-5, design §2): every harness
// descriptor, with the harness Capability list as columns.

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// clientRegistry builds the client registry from every harness descriptor
// (harness.All()). Every entry carries the FULL harness.Capabilities list
// as its columns — not only the capabilities a given descriptor's Vehicles
// map happens to set — so a client with no reading yet for a capability
// still gets a column to project `tolerated`/`declared` against.
func clientRegistry() []RegistryEntry {
	caps := make([]string, 0, len(harness.Capabilities))
	for _, c := range harness.Capabilities {
		caps = append(caps, string(c))
	}

	descriptors := harness.All()
	entries := make([]RegistryEntry, 0, len(descriptors))
	for _, d := range descriptors {
		entries = append(entries, RegistryEntry{
			Kind:         KindClient,
			ID:           d.ID,
			Name:         d.Display,
			Capabilities: caps,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries
}
