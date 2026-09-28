package syslink

import "testing"

func row(service, key string, role Role, node string, tier Tier, file string, line int) KeyRow {
	return KeyRow{Service: service, Key: mustKey(key), Role: role, NodeID: node, TierHint: tier, File: file, Line: line, Extractor: "t", ExtractorEpoch: 1}
}

// TestMatchLinksTierIsWeakerEnd is SG-9's contract: provides and consumes
// rows join on their key; a link's tier is the weaker of its two ends; the
// link carries evidence (file:line, extractor, tier) at both ends; a
// template consumer matches providers only as a segment pattern and never
// above template; an ANY-method route provider serves every method.
func TestMatchLinksTierIsWeakerEnd(t *testing.T) {
	rows := []KeyRow{
		// orders provides by spec (OpenAPI) and by code (literal).
		row("orders", "route:GET /orders/{}", RoleProvides, "getOrder", TierSpec, "openapi.yaml", 10),
		row("orders", "route:ANY /health", RoleProvides, "health", TierLiteral, "main.go", 5),
		row("orders", "rpc:shop.v1.Orders/Create", RoleProvides, "shop.v1.Orders/Create", TierSpec, "orders.proto", 4),
		// checkout consumes.
		row("checkout", "route:GET /orders/{}", RoleConsumes, "Place", TierLiteral, "c.go", 7),
		row("checkout", "rpc:shop.v1.Orders/Create", RoleConsumes, "Place", TierTyped, "c.go", 9),
		row("checkout", "route:GET /health", RoleConsumes, "Ping", TierLiteral, "c.go", 12),
		row("web", "route:GET /{}/{}", RoleConsumes, "load", TierTemplate, "api.ts", 3),
		// A consumer with no provider links to nothing.
		row("web", "route:GET /nowhere", RoleConsumes, "lost", TierLiteral, "api.ts", 9),
	}
	links := MatchLinks(rows)
	type want struct {
		consumer, provider, key string
		tier                    Tier
	}
	wants := []want{
		{"checkout/Ping", "orders/health", "route:ANY /health", TierLiteral},
		{"checkout/Place", "orders/getOrder", "route:GET /orders/{}", TierLiteral},
		{"checkout/Place", "orders/shop.v1.Orders/Create", "rpc:shop.v1.Orders/Create", TierTyped},
		{"web/load", "orders/getOrder", "route:GET /orders/{}", TierTemplate},
	}
	if len(links) != len(wants) {
		t.Fatalf("MatchLinks = %d links, want %d:\n%+v", len(links), len(wants), links)
	}
	for i, w := range wants {
		l := links[i]
		if l.ConsumerService+"/"+l.ConsumerNode != w.consumer || l.ProviderService+"/"+l.ProviderNode != w.provider || l.Key.String() != w.key || l.Tier != w.tier {
			t.Errorf("link %d = %s/%s -> %s/%s %s %s, want %+v", i, l.ConsumerService, l.ConsumerNode, l.ProviderService, l.ProviderNode, l.Key, l.Tier, w)
		}
		if l.Kind != l.Key.Kind {
			t.Errorf("link %d kind %q vs key %v", i, l.Kind, l.Key)
		}
		if l.Evidence.Consumer.File == "" || l.Evidence.Consumer.Line == 0 || l.Evidence.Provider.File == "" || l.Evidence.Provider.Line == 0 || l.Evidence.Consumer.Extractor == "" {
			t.Errorf("link %d missing evidence at an end: %+v", i, l.Evidence)
		}
	}
	// The weaker end: a spec provider and a literal consumer make literal.
	if got := Weaker(TierSpec, TierLiteral); got != TierLiteral {
		t.Errorf("Weaker(spec, literal) = %s", got)
	}
	if got := Weaker(TierTemplate, TierTyped); got != TierTemplate {
		t.Errorf("Weaker(template, typed) = %s", got)
	}
	// Deterministic: shuffled input, identical output.
	rev := append([]KeyRow(nil), rows...)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	again := MatchLinks(rev)
	for i := range links {
		if links[i].Key != again[i].Key || links[i].ConsumerNode != again[i].ConsumerNode || links[i].ProviderNode != again[i].ProviderNode {
			t.Fatalf("MatchLinks is order-dependent at %d", i)
		}
	}
}

// TestHeuristicLinksNeverGate is SG-16 (I57): a literal or template link is
// served with its tier named, but never passes or fails a gate; only spec
// and typed links may.
func TestHeuristicLinksNeverGate(t *testing.T) {
	rows := []KeyRow{
		row("a", "route:GET /x", RoleProvides, "x", TierSpec, "a.yaml", 1),
		row("a", "rpc:p.S/M", RoleProvides, "p.S/M", TierSpec, "a.proto", 1),
		row("b", "route:GET /x", RoleConsumes, "lit", TierLiteral, "b.go", 1),
		row("b", "route:GET /{}", RoleConsumes, "tmpl", TierTemplate, "b.go", 2),
		row("b", "rpc:p.S/M", RoleConsumes, "typed", TierTyped, "b.go", 3),
		row("c", "route:GET /x", RoleConsumes, "spec", TierSpec, "c.go", 1),
	}
	links := MatchLinks(rows)
	gating := GatingLinks(links)
	for _, l := range links {
		heuristic := l.Tier == TierLiteral || l.Tier == TierTemplate
		if linkGates(l) == heuristic {
			t.Errorf("linkGates(%s link %s) = %v", l.Tier, l.Key, linkGates(l))
		}
		if !heuristic != (l.Tier == TierSpec || l.Tier == TierTyped) {
			t.Errorf("unexpected tier %q", l.Tier)
		}
	}
	for _, l := range gating {
		if l.Tier != TierSpec && l.Tier != TierTyped {
			t.Errorf("GatingLinks kept a %s link: %v", l.Tier, l.Key)
		}
	}
	if len(gating) != 2 || len(links) != 4 {
		t.Errorf("links %d gating %d, want 4 and 2", len(links), len(gating))
	}
	// An unknown tier never gates either.
	if linkGates(Link{Tier: "heuristic"}) {
		t.Error("an unknown tier gated")
	}
}
