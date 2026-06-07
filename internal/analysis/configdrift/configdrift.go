// Package configdrift compares configuration maps across services and reports
// values that deviate from the majority.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package configdrift

import (
	"sort"
)

// DriftItem reports a single service-key pair whose value differs from the
// majority value held by other services.
type DriftItem struct {
	Key      string // config key
	Service  string // service that has the differing value
	Value    string // the service's actual value
	Majority string // the majority value across all services
}

// Drift examines configs (service → key → value) for each key in keys and
// returns DriftItems for every service whose value differs from the majority.
//
// Majority is determined by the most common value for a key. When there is a
// tie (two values appear equally often), the lexicographically smaller value
// is treated as the majority. Services that do not define a key are excluded
// from both counting and reporting for that key.
//
// The returned slice is sorted by (Key, Service) for determinism.
func Drift(configs map[string]map[string]string, keys []string) []DriftItem {
	// Sort keys for determinism.
	sortedKeys := make([]string, len(keys))
	copy(sortedKeys, keys)
	sort.Strings(sortedKeys)

	var items []DriftItem

	for _, key := range sortedKeys {
		// Gather all (service, value) pairs that define this key.
		type pair struct {
			service string
			value   string
		}
		var pairs []pair
		for svc, kvs := range configs {
			if v, ok := kvs[key]; ok {
				pairs = append(pairs, pair{svc, v})
			}
		}
		if len(pairs) < 2 {
			// Not enough data to determine drift.
			continue
		}

		// Count occurrences of each value.
		freq := make(map[string]int, len(pairs))
		for _, p := range pairs {
			freq[p.value]++
		}

		majority := majorityValue(freq)

		// Collect drifting services, sorted for determinism.
		sort.Slice(pairs, func(i, j int) bool {
			return pairs[i].service < pairs[j].service
		})
		for _, p := range pairs {
			if p.value != majority {
				items = append(items, DriftItem{
					Key:      key,
					Service:  p.service,
					Value:    p.value,
					Majority: majority,
				})
			}
		}
	}

	// Final sort for determinism (keys were already processed in order, but
	// service ordering inside each key is stable from the inner sort).
	sort.Slice(items, func(i, j int) bool {
		if items[i].Key != items[j].Key {
			return items[i].Key < items[j].Key
		}
		return items[i].Service < items[j].Service
	})
	return items
}

// majorityValue returns the value with the highest frequency. Ties are broken
// by choosing the lexicographically smaller value.
func majorityValue(freq map[string]int) string {
	// Collect unique values, sorted, for deterministic tie-breaking.
	vals := make([]string, 0, len(freq))
	for v := range freq {
		vals = append(vals, v)
	}
	sort.Strings(vals)

	best := vals[0]
	for _, v := range vals[1:] {
		if freq[v] > freq[best] {
			best = v
		}
		// On equal frequency, keep the lexicographically smaller one (vals is
		// sorted, so best is already smaller than v when frequencies are equal).
	}
	return best
}
