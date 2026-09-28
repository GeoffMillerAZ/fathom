package syslink

import (
	"sort"
	"strings"
)

// End is one side of a link's evidence: where the key row was seen, by
// which extractor, and at which tier.
type End struct {
	Service   string
	Node      string
	File      string
	Line      int
	Key       ContractKey
	Tier      Tier
	Extractor string
	Epoch     int
}

// Evidence is a link's proof at both ends (§4).
type Evidence struct {
	Consumer End
	Provider End
}

// Link is one system_links row (§6): a consumer node joined to a provider
// node on a contract key, graded by the weaker end's tier.
type Link struct {
	ConsumerService string
	ConsumerNode    string
	ProviderService string
	ProviderNode    string
	Key             ContractKey // the provider's key: the contract the link names
	Kind            Kind
	Tier            Tier
	Evidence        Evidence
}

// MatchLinks joins provides and consumes rows on their key (SG-9, §4). A
// link's tier is the weaker of its two ends. A key that more than one
// provider offers fans out to every one of them (the true blast radius;
// nothing is dropped for being ambiguous). Two route: relaxations apply:
// a provider registered for ANY method serves every method, and a
// template-tier consumer matches providers as a segment pattern ({} matches
// one segment), a match that is never above template. The join is pure;
// the output is sorted and independent of input order.
func MatchLinks(rows []KeyRow) []Link {
	providers := map[ContractKey][]KeyRow{}
	var routeProviders []KeyRow
	for _, r := range rows {
		if r.Role != RoleProvides {
			continue
		}
		providers[r.Key] = append(providers[r.Key], r)
		if r.Key.Kind == KindRoute {
			routeProviders = append(routeProviders, r)
		}
	}
	type dedup struct {
		cs, cn, cf string
		cl         int
		ps, pn     string
		key        ContractKey
	}
	seen := map[dedup]bool{}
	var out []Link
	add := func(c, p KeyRow, capTier Tier) {
		d := dedup{c.Service, c.NodeID, c.File, c.Line, p.Service, p.NodeID, p.Key}
		if seen[d] {
			return
		}
		seen[d] = true
		tier := Weaker(c.TierHint, p.TierHint)
		if capTier != "" {
			tier = Weaker(tier, capTier)
		}
		out = append(out, Link{
			ConsumerService: c.Service, ConsumerNode: c.NodeID,
			ProviderService: p.Service, ProviderNode: p.NodeID,
			Key: p.Key, Kind: p.Key.Kind, Tier: tier,
			Evidence: Evidence{Consumer: endOf(c), Provider: endOf(p)},
		})
	}
	for _, c := range rows {
		if c.Role != RoleConsumes {
			continue
		}
		for _, p := range providers[c.Key] {
			add(c, p, "")
		}
		if c.Key.Kind != KindRoute {
			continue
		}
		method, path, _ := strings.Cut(c.Key.Body, " ")
		if method != "ANY" {
			for _, p := range providers[ContractKey{Kind: KindRoute, Body: "ANY " + path}] {
				add(c, p, "")
			}
		}
		if c.TierHint == TierTemplate && strings.Contains(path, "{}") {
			for _, p := range routeProviders {
				pm, pp, _ := strings.Cut(p.Key.Body, " ")
				if (pm == method || pm == "ANY") && pp != path && pathPatternMatch(path, pp) {
					add(c, p, TierTemplate)
				}
			}
		}
	}
	SortLinks(out)
	return out
}

// pathPatternMatch reports whether a consumer template (holes as "{}")
// matches a provider path segment by segment.
func pathPatternMatch(pattern, path string) bool {
	a, b := strings.Split(pattern, "/"), strings.Split(path, "/")
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != "{}" && a[i] != b[i] {
			return false
		}
	}
	return true
}

func endOf(r KeyRow) End {
	return End{Service: r.Service, Node: r.NodeID, File: r.File, Line: r.Line, Key: r.Key, Tier: r.TierHint, Extractor: r.Extractor, Epoch: r.ExtractorEpoch}
}

// SortLinks orders links by consumer, then provider, then key.
func SortLinks(ls []Link) {
	sort.SliceStable(ls, func(i, j int) bool {
		a, b := ls[i], ls[j]
		for _, p := range [][2]string{
			{a.ConsumerService, b.ConsumerService}, {a.ConsumerNode, b.ConsumerNode},
			{a.Evidence.Consumer.File, b.Evidence.Consumer.File},
		} {
			if p[0] != p[1] {
				return p[0] < p[1]
			}
		}
		if a.Evidence.Consumer.Line != b.Evidence.Consumer.Line {
			return a.Evidence.Consumer.Line < b.Evidence.Consumer.Line
		}
		for _, p := range [][2]string{
			{a.ProviderService, b.ProviderService}, {a.ProviderNode, b.ProviderNode},
			{a.Key.String(), b.Key.String()},
		} {
			if p[0] != p[1] {
				return p[0] < p[1]
			}
		}
		return false
	})
}

// linkGates is I57's single choke point (SG-16): only a spec or typed link
// may pass or fail a verify row, a conservation plane or a review verdict.
// A literal or template link — and any tier this package does not know —
// is served with its tier named, as a hint, and never gates.
func linkGates(l Link) bool {
	return l.Tier == TierSpec || l.Tier == TierTyped
}

// GatingLinks returns the links a gate may read: those linkGates admits.
// Every gate that consumes links goes through it.
func GatingLinks(ls []Link) []Link {
	var out []Link
	for _, l := range ls {
		if linkGates(l) {
			out = append(out, l)
		}
	}
	return out
}
