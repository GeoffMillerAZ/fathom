package certify

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/review"
	"github.com/geoffmilleraz/fathom/internal/syslink"
)

// linkRow builds one contract_keys row for the link join.
func linkRow(t *testing.T, service, key string, role syslink.Role, node string, tier syslink.Tier, file string, line int) syslink.KeyRow {
	t.Helper()
	k, err := syslink.ParseContractKey(key)
	if err != nil {
		t.Fatalf("ParseContractKey(%q): %v", key, err)
	}
	return syslink.KeyRow{Service: service, Key: k, Role: role, NodeID: node, TierHint: tier, File: file, Line: line, Extractor: "t", ExtractorEpoch: 1}
}

// linkFixture is one fleet's joined links plus the certificates found at
// the link ends, covering every shape CertifyLinks must tell apart:
//
//	A spec    checkout/Place   -> orders/getOrder   both ends live          certified
//	B typed   checkout/Refund  -> orders/Create     consumer only flaky     uncertified (consumer)
//	C spec    checkout/Pay     -> billing/charge    provider unproven only  uncertified (both)
//	D spec    reports/Load     -> orders/getOrder   provider live, no cert  uncertified (consumer)
//	E literal web/ping         -> orders/health     neither end             hint, never gating
//
// Provider orders/getOrder carries a live certificate and is shared by A
// and D, so D is missing the consumer end only.
func linkFixture(t *testing.T) ([]syslink.Link, EndCertificates) {
	t.Helper()
	rows := []syslink.KeyRow{
		linkRow(t, "orders", "route:GET /orders/{}", syslink.RoleProvides, "getOrder", syslink.TierSpec, "openapi.yaml", 10),
		linkRow(t, "orders", "rpc:shop.v1.Orders/Create", syslink.RoleProvides, "Create", syslink.TierSpec, "orders.proto", 4),
		linkRow(t, "orders", "route:GET /health", syslink.RoleProvides, "health", syslink.TierLiteral, "main.go", 5),
		linkRow(t, "billing", "route:POST /charge", syslink.RoleProvides, "charge", syslink.TierSpec, "billing.yaml", 3),

		linkRow(t, "checkout", "route:GET /orders/{}", syslink.RoleConsumes, "Place", syslink.TierSpec, "place.go", 7),
		linkRow(t, "checkout", "rpc:shop.v1.Orders/Create", syslink.RoleConsumes, "Refund", syslink.TierTyped, "refund.go", 9),
		linkRow(t, "checkout", "route:POST /charge", syslink.RoleConsumes, "Pay", syslink.TierSpec, "pay.go", 12),
		linkRow(t, "reports", "route:GET /orders/{}", syslink.RoleConsumes, "Load", syslink.TierSpec, "load.go", 2),
		linkRow(t, "web", "route:GET /health", syslink.RoleConsumes, "ping", syslink.TierLiteral, "api.ts", 4),
	}
	links := syslink.MatchLinks(rows)
	if len(links) != 5 {
		t.Fatalf("fixture joined %d links, want 5: %+v", len(links), links)
	}

	certs := EndCertificates{
		{Service: "orders", Node: "getOrder"}: {
			{Test: "orders.TestGetOrderContract", Target: "spec:ORD-1", Strength: "conformance", State: StateLive},
		},
		{Service: "checkout", Node: "Place"}: {
			{Test: "checkout.TestPlaceAgainstSpec", Target: "spec:ORD-1", Strength: "conformance", State: StateLive},
		},
		{Service: "orders", Node: "Create"}: {
			{Test: "orders.TestCreateStub", Target: "port:Orders", Strength: "conformance", State: StateLive},
		},
		{Service: "checkout", Node: "Refund"}: {
			{Test: "checkout.TestRefundContract", Target: "spec:ORD-2", Strength: "conformance", State: StateFlaky},
		},
		{Service: "billing", Node: "charge"}: {
			{Test: "billing.TestChargeContract", Target: "spec:BIL-1", Strength: "conformance", State: StateUnproven},
		},
	}
	return links, certs
}

func linkByConsumer(t *testing.T, cs []CertifiedLink, consumerService, consumerNode string) CertifiedLink {
	t.Helper()
	for _, c := range cs {
		if c.Link.ConsumerService == consumerService && c.Link.ConsumerNode == consumerNode {
			return c
		}
	}
	t.Fatalf("no link from %s/%s in %+v", consumerService, consumerNode, cs)
	return CertifiedLink{}
}

// TestCertifyLinksBothEnds is TQ-17's contract: a link is certified only
// when BOTH ends carry a live certificate (I43: a flaky, broken, unproven
// or unreachable certificate certifies nothing); the spec and typed links
// that are not certified are listed for the risk receipt naming which end
// is missing; a literal or template link is listed as a hint and never
// gates (I57); and the answer is independent of input order.
func TestCertifyLinksBothEnds(t *testing.T) {
	links, certs := linkFixture(t)
	got := CertifyLinks(links, certs)

	if len(got.Links) != 5 {
		t.Fatalf("Links = %d, want all 5 joined links", len(got.Links))
	}
	if got.Certified != 1 {
		t.Errorf("Certified = %d, want 1 (checkout/Place -> orders/getOrder)", got.Certified)
	}

	a := linkByConsumer(t, got.Links, "checkout", "Place")
	if !a.Certified || !a.ProviderCertified || !a.ConsumerCertified {
		t.Errorf("both-ends-live link = %+v, want certified", a)
	}
	if len(a.Provider) != 1 || len(a.Consumer) != 1 {
		t.Errorf("certified link should carry each end's certificate: provider %v consumer %v", a.Provider, a.Consumer)
	}

	// B: the consumer's only certificate is flaky. The provider end is live,
	// the link is not certified, and the flaky certificate is still shown
	// (it is the reason).
	b := linkByConsumer(t, got.Links, "checkout", "Refund")
	if b.Certified || !b.ProviderCertified || b.ConsumerCertified {
		t.Errorf("flaky consumer link = %+v, want provider certified, consumer not", b)
	}
	if len(b.Consumer) != 1 || b.Consumer[0].State != StateFlaky {
		t.Errorf("flaky consumer certificate not shown: %+v", b.Consumer)
	}

	// C: an unproven provider certificate and no consumer certificate.
	c := linkByConsumer(t, got.Links, "checkout", "Pay")
	if c.Certified || c.ProviderCertified || c.ConsumerCertified {
		t.Errorf("unproven provider link = %+v, want neither end certified", c)
	}

	// D: provider shared with A is live, the consumer has no entry at all.
	d := linkByConsumer(t, got.Links, "reports", "Load")
	if d.Certified || !d.ProviderCertified || d.ConsumerCertified || len(d.Consumer) != 0 {
		t.Errorf("no-consumer-certificate link = %+v", d)
	}

	// Uncertified: the gating-tier (spec, typed) links that are not
	// certified, in the link join's own order; E (literal) is a hint.
	var gotUncertified []string
	for _, u := range got.Uncertified {
		gotUncertified = append(gotUncertified, u.Link.ConsumerService+"/"+u.Link.ConsumerNode+"->"+u.Link.ProviderService+"/"+u.Link.ProviderNode)
	}
	wantUncertified := []string{
		"checkout/Pay->billing/charge",
		"checkout/Refund->orders/Create",
		"reports/Load->orders/getOrder",
	}
	if !reflect.DeepEqual(gotUncertified, wantUncertified) {
		t.Errorf("Uncertified = %v, want %v", gotUncertified, wantUncertified)
	}
	for _, u := range got.Uncertified {
		if !u.Gating {
			t.Errorf("uncertified receipt entry %+v is not gating tier", u.Link)
		}
	}

	if len(got.Hints) != 1 || got.Hints[0].Link.ConsumerNode != "ping" || got.Hints[0].Gating {
		t.Errorf("Hints = %+v, want exactly the literal-tier web/ping link, not gating", got.Hints)
	}
	for _, u := range got.Uncertified {
		if u.Link.Tier == syslink.TierLiteral || u.Link.Tier == syslink.TierTemplate {
			t.Errorf("a %s link reached the receipt list (I57): %+v", u.Link.Tier, u.Link)
		}
	}

	// A literal-tier link IS certified when both ends are live: the tier
	// only decides whether an uncertified link may gate, never whether a
	// live pair counts.
	literal := certs
	literal[EndKey{Service: "orders", Node: "health"}] = []EndCertificate{{Test: "orders.TestHealth", Target: "spec:H", Strength: "conformance", State: StateLive}}
	literal[EndKey{Service: "web", Node: "ping"}] = []EndCertificate{{Test: "web.TestPing", Target: "spec:H", Strength: "conformance", State: StateLive}}
	again := CertifyLinks(links, literal)
	if again.Certified != 2 || len(again.Hints) != 0 {
		t.Errorf("after certifying the literal link: Certified = %d Hints = %d, want 2 and 0", again.Certified, len(again.Hints))
	}
}

// TestCertifyLinksOnlyLiveCertifies pins I43 at the link end: every state
// but live leaves the end uncertified.
func TestCertifyLinksOnlyLiveCertifies(t *testing.T) {
	links, _ := linkFixture(t)
	only := []syslink.Link{linkByConsumerLink(t, links, "checkout", "Place")}
	for _, s := range []State{StateFlaky, StateBroken, StateUnproven, StateUnreachable, State("stale-ish")} {
		certs := EndCertificates{
			{Service: "orders", Node: "getOrder"}: {{Test: "t.P", Target: "spec:X", State: StateLive}},
			{Service: "checkout", Node: "Place"}:  {{Test: "t.C", Target: "spec:X", State: s}},
		}
		got := CertifyLinks(only, certs)
		if got.Certified != 0 || got.Links[0].ConsumerCertified {
			t.Errorf("consumer certificate in state %q certified the link: %+v", s, got.Links[0])
		}
	}
	// One live certificate among non-live ones is enough for its end.
	certs := EndCertificates{
		{Service: "orders", Node: "getOrder"}: {{Test: "t.P", Target: "spec:X", State: StateLive}},
		{Service: "checkout", Node: "Place"}: {
			{Test: "t.Broken", Target: "spec:X", State: StateBroken},
			{Test: "t.Live", Target: "spec:X", State: StateLive},
		},
	}
	if got := CertifyLinks(only, certs); got.Certified != 1 {
		t.Errorf("a live certificate beside a broken one should certify the end: %+v", got.Links[0])
	}
}

func linkByConsumerLink(t *testing.T, links []syslink.Link, service, node string) syslink.Link {
	t.Helper()
	for _, l := range links {
		if l.ConsumerService == service && l.ConsumerNode == node {
			return l
		}
	}
	t.Fatalf("no link from %s/%s", service, node)
	return syslink.Link{}
}

// TestCertifyLinksDeterministic proves the order contract
// (internal/CLAUDE.md): any permutation of the links and of each end's
// certificates yields the identical report, and the caller's inputs are
// not reordered or mutated.
func TestCertifyLinksDeterministic(t *testing.T) {
	links, certs := linkFixture(t)
	want := CertifyLinks(links, certs)

	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 20; i++ {
		shuffled := append([]syslink.Link(nil), links...)
		rng.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
		perm := EndCertificates{}
		for k, v := range certs {
			vv := append([]EndCertificate(nil), v...)
			rng.Shuffle(len(vv), func(a, b int) { vv[a], vv[b] = vv[b], vv[a] })
			perm[k] = vv
		}
		if got := CertifyLinks(shuffled, perm); !reflect.DeepEqual(got, want) {
			t.Fatalf("permutation %d changed the report:\n got %+v\nwant %+v", i, got, want)
		}
	}

	in := append([]syslink.Link(nil), links...)
	CertifyLinks(in, certs)
	if !reflect.DeepEqual(in, links) {
		t.Error("CertifyLinks reordered its input links")
	}

	// Multiple certificates on one end are reported sorted by test then target.
	multi := EndCertificates{
		{Service: "orders", Node: "getOrder"}: {
			{Test: "b.T", Target: "spec:2", State: StateLive},
			{Test: "a.T", Target: "spec:9", State: StateLive},
			{Test: "a.T", Target: "spec:1", State: StateBroken},
		},
	}
	rep := CertifyLinks([]syslink.Link{linkByConsumerLink(t, links, "checkout", "Place")}, multi)
	var order []string
	for _, c := range rep.Links[0].Provider {
		order = append(order, c.Test+"|"+c.Target)
	}
	if !reflect.DeepEqual(order, []string{"a.T|spec:1", "a.T|spec:9", "b.T|spec:2"}) {
		t.Errorf("provider certificates unsorted: %v", order)
	}
}

// TestCertifyLinksEmpty: nothing to certify is an empty report, not a
// panic and not a fabricated clean bill (Certified stays 0).
func TestCertifyLinksEmpty(t *testing.T) {
	got := CertifyLinks(nil, nil)
	if len(got.Links) != 0 || got.Certified != 0 || len(got.Uncertified) != 0 || len(got.Hints) != 0 {
		t.Errorf("CertifyLinks(nil, nil) = %+v, want the zero report", got)
	}
	links, _ := linkFixture(t)
	none := CertifyLinks(links, nil)
	if none.Certified != 0 || len(none.Links) != len(links) {
		t.Errorf("no certificates at all: Certified = %d Links = %d", none.Certified, len(none.Links))
	}
	if len(none.Uncertified) != 4 || len(none.Hints) != 1 {
		t.Errorf("no certificates: Uncertified = %d Hints = %d, want the 4 spec/typed links and 1 hint", len(none.Uncertified), len(none.Hints))
	}
}

// TestUncertifiedLinksInReceipt is TQ-18's contract: the uncertified
// spec/typed links map onto the change risk receipt's
// `uncertified_contracts` input, so the factor fires citing each one; a
// literal or template link never reaches it (I57); with every gating link
// certified the factor reads none.
func TestUncertifiedLinksInReceipt(t *testing.T) {
	links, certs := linkFixture(t)
	report := CertifyLinks(links, certs)
	refs := uncertifiedLinksForReceipt(report)

	if len(refs) != 3 {
		t.Fatalf("refs = %+v, want the 3 uncertified spec/typed links", refs)
	}
	want := []review.LinkRef{
		{Provider: "billing/charge", Consumer: "Pay", ConsumerService: "checkout", Tier: "spec", Citation: "pay.go#L12 -> billing.yaml#L3"},
		{Provider: "orders/Create", Consumer: "Refund", ConsumerService: "checkout", Tier: "typed", Citation: "refund.go#L9 -> orders.proto#L4"},
		{Provider: "orders/getOrder", Consumer: "Load", ConsumerService: "reports", Tier: "spec", Citation: "load.go#L2 -> openapi.yaml#L10"},
	}
	if !reflect.DeepEqual(refs, want) {
		t.Errorf("refs =\n%+v\nwant\n%+v", refs, want)
	}
	for _, r := range refs {
		if r.Tier != review.LinkTierSpec && r.Tier != review.LinkTierTyped {
			t.Errorf("a %q-tier link reached the receipt input (I57): %+v", r.Tier, r)
		}
	}

	// The refs feed the receipt's own factor.
	receipt := review.BuildRiskReceipt(review.Input{
		ChangedFiles:         []string{"orders/handler.go"},
		UncertifiedContracts: refs,
	}, review.DefaultRiskTable())
	f := receiptFactor(t, receipt, review.FactorUncertified)
	if f.Weight != review.WeightMedium {
		t.Errorf("uncertified_contracts weight = %v, want medium", f.Weight)
	}
	wantEvidence := []string{
		"billing/charge -> Pay (checkout)",
		"orders/Create -> Refund (checkout)",
		"orders/getOrder -> Load (reports)",
	}
	if !reflect.DeepEqual(f.Evidence, wantEvidence) {
		t.Errorf("uncertified_contracts evidence = %v, want %v", f.Evidence, wantEvidence)
	}

	// Certify every gating end: the factor reads none, the hint stays a hint.
	for _, k := range []EndKey{{"checkout", "Refund"}, {"checkout", "Pay"}, {"billing", "charge"}, {"reports", "Load"}} {
		certs[k] = []EndCertificate{{Test: "t.Live", Target: "spec:X", State: StateLive}}
	}
	all := CertifyLinks(links, certs)
	if refs := uncertifiedLinksForReceipt(all); len(refs) != 0 {
		t.Fatalf("every gating link certified, refs = %+v", refs)
	}
	clean := review.BuildRiskReceipt(review.Input{UncertifiedContracts: uncertifiedLinksForReceipt(all)}, review.DefaultRiskTable())
	if f := receiptFactor(t, clean, review.FactorUncertified); f.Weight != review.WeightNone {
		t.Errorf("all certified: weight = %v, want none", f.Weight)
	}

	// A hand-built report cannot smuggle a hint into the receipt: the
	// mapper re-applies I57's choke point rather than trusting the field.
	forged := LinkCertification{Uncertified: report.Hints}
	if refs := uncertifiedLinksForReceipt(forged); len(refs) != 0 {
		t.Errorf("a literal-tier link in Uncertified reached the receipt: %+v", refs)
	}

	// A same-service link leaves ConsumerService empty (LinkRef's contract).
	same := syslink.MatchLinks([]syslink.KeyRow{
		linkRow(t, "orders", "route:GET /orders/{}", syslink.RoleProvides, "getOrder", syslink.TierSpec, "openapi.yaml", 10),
		linkRow(t, "orders", "route:GET /orders/{}", syslink.RoleConsumes, "Warm", syslink.TierSpec, "warm.go", 3),
	})
	got := uncertifiedLinksForReceipt(CertifyLinks(same, nil))
	if len(got) != 1 || got[0].ConsumerService != "" {
		t.Errorf("same-service link refs = %+v, want one with an empty ConsumerService", got)
	}
}

func receiptFactor(t *testing.T, r review.RiskReceipt, name string) review.RiskFactor {
	t.Helper()
	for _, f := range r.Factors {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("receipt has no %q factor", name)
	return review.RiskFactor{}
}
