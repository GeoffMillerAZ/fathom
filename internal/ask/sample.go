package ask

import (
	"hash/fnv"
	"sort"
)

// SampleSeed is E1's pre-registered sampling seed (EXP-1).
const SampleSeed uint64 = 20261016

// Quota is how many spans a template draws from one repository.
type Quota struct {
	Repo string `json:"repo"`
	N    int    `json:"n"`
}

// E1Quotas are the pre-registered per-template quotas, in the order a
// shortfall is refilled: about 200 spans per template.
var E1Quotas = []Quota{{Repo: "fathom", N: 100}, {Repo: "websocket", N: 50}, {Repo: "bbolt", N: 50}}

// splitmix64 is the sampler's generator, written out so the sample never
// depends on a library's shuffle staying the same.
type splitmix64 uint64

func (s *splitmix64) next() uint64 {
	*s += 0x9e3779b97f4a7c15
	z := uint64(*s)
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func streamSeed(seed uint64, template, repo string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(template + "/" + repo))
	return seed ^ h.Sum64()
}

func spanLess(a, b Span) bool {
	if a.Repo != b.Repo {
		return a.Repo < b.Repo
	}
	if a.File != b.File {
		return a.File < b.File
	}
	if a.Start != b.Start {
		return a.Start < b.Start
	}
	return a.Name < b.Name
}

// Sample draws t's spans from pool (Pool's output): per quota, the
// repository's spans sorted by (repo, file, start, name), shuffled
// Fisher-Yates from the last index with j = next() mod (i+1) on a
// splitmix64 seeded with seed XOR FNV-1a-64(template "/" repo), and the
// first N taken; a repository short of its quota leaves the shortfall to
// the next unused spans of the repositories in quota order. The result is
// in draw order: deterministic for a given pool and seed.
func Sample(t Template, pool []Computed, quotas []Quota, seed uint64) []Computed {
	byRepo := map[string][]Computed{}
	for _, c := range pool {
		byRepo[c.Span.Repo] = append(byRepo[c.Span.Repo], c)
	}
	shuffled := map[string][]Computed{}
	for _, q := range quotas {
		cs := append([]Computed(nil), byRepo[q.Repo]...)
		sort.Slice(cs, func(i, j int) bool { return spanLess(cs[i].Span, cs[j].Span) })
		rng := splitmix64(streamSeed(seed, t.ID, q.Repo))
		for i := len(cs) - 1; i > 0; i-- {
			j := int(rng.next() % uint64(i+1))
			cs[i], cs[j] = cs[j], cs[i]
		}
		shuffled[q.Repo] = cs
	}
	var out []Computed
	used := map[string]int{}
	short := 0
	for _, q := range quotas {
		n := min(q.N, len(shuffled[q.Repo]))
		out = append(out, shuffled[q.Repo][:n]...)
		used[q.Repo] = n
		short += q.N - n
	}
	for _, q := range quotas {
		if short == 0 {
			break
		}
		rest := shuffled[q.Repo][used[q.Repo]:]
		n := min(short, len(rest))
		out = append(out, rest[:n]...)
		used[q.Repo] += n
		short -= n
	}
	return out
}
