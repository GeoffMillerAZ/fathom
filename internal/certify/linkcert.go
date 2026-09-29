package certify

// linkcert.go — CertifyLinks (TQ-17) and uncertifiedLinksForReceipt (TQ-18),
// docs/design/test-intelligence.md §3 T5: contract certificates on
// system-graph links. A cross-service link (internal/syslink, ADR-050) is
// CERTIFIED when a provider-side test certifies the provider node and a
// consumer-side contract test certifies the consumer's call, both LIVE. The
// uncertified links at the spec or typed tier are what the change risk
// receipt lists (change-and-compliance design §2 row 8, "uncertified
// contracts").
//
// Pure over its own input shapes, like SelectCertifyingFirst (selection.go):
// the links come from syslink.MatchLinks (or, once SG-2 lands, the stored
// system_links rows), and the certificates found at each link end are the
// caller's join (EndCertificates). Nothing here reads a store, a clock or a
// tree, so it is independently testable and never an I/O-per-link cost
// (I48).
//
// Two invariants are honored by construction, not by convention:
//
//   - I43: only a certificate in State == StateLive certifies an end
//     (State.Satisfies). A flaky, broken, unproven or unreachable
//     certificate is reported beside the end, because it is the reason the
//     end is uncertified, and counts for nothing.
//   - I57: only a spec or typed link may gate. The receipt list is built
//     through syslink.GatingLinks, the one choke point every link gate
//     shares; an uncertified literal or template link is served as a hint
//     with its tier named and never reaches the receipt.
//
// fathom asserts a fact here and never judges adequacy (vision 33 §3): a
// certificate's strength is carried through, never thresholded.

import (
	"sort"
	"strconv"

	"github.com/geoffmilleraz/fathom/internal/review"
	"github.com/geoffmilleraz/fathom/internal/syslink"
)

// EndKey identifies one end of a link: a node of a service. It is the same
// identity a syslink.Link carries for its two ends
// (ConsumerService/ConsumerNode, ProviderService/ProviderNode).
type EndKey struct {
	Service string
	Node    string
}

// EndCertificate is one certificate at a link end: test Test certifies
// Target ("<kind>:<id>") at Strength, in State.
type EndCertificate struct {
	Test     string
	Target   string
	Strength string
	State    State
}

// EndCertificates is the caller's join of certificates to link ends. An end
// absent from the map has no certificate at all.
type EndCertificates map[EndKey][]EndCertificate

// CertifiedLink is one link with its certification verdict.
type CertifiedLink struct {
	Link syslink.Link
	// Provider and Consumer are every certificate found at each end,
	// sorted by test then target, whatever its state.
	Provider []EndCertificate
	Consumer []EndCertificate
	// ProviderCertified and ConsumerCertified say whether that end carries
	// at least one LIVE certificate.
	ProviderCertified bool
	ConsumerCertified bool
	// Certified is true only when both ends are certified.
	Certified bool
	// Gating is true when the link's tier is spec or typed: the only tiers
	// that may gate or be counted (I57).
	Gating bool
}

// LinkCertification is CertifyLinks' answer.
type LinkCertification struct {
	// Links is every input link with its verdict, in the link join's own
	// order (syslink.SortLinks).
	Links []CertifiedLink
	// Certified counts the links with both ends live.
	Certified int
	// Uncertified is the spec and typed links that are not certified: the
	// change risk receipt's input.
	Uncertified []CertifiedLink
	// Hints is the uncertified literal and template links, listed with
	// their tier and never gating (I57).
	Hints []CertifiedLink
}

// CertifyLinks joins links to the certificates at their ends. It never
// mutates or reorders its inputs; the report is identical for any input
// order.
func CertifyLinks(links []syslink.Link, certs EndCertificates) LinkCertification {
	var out LinkCertification
	if len(links) == 0 {
		return out
	}
	sorted := append([]syslink.Link(nil), links...)
	syslink.SortLinks(sorted)

	// I57's choke point: a link is gating iff syslink.GatingLinks admits it.
	// syslink.Link holds only comparable fields, so the admitted set is
	// looked up by value.
	gating := map[syslink.Link]bool{}
	for _, l := range syslink.GatingLinks(sorted) {
		gating[l] = true
	}

	out.Links = make([]CertifiedLink, 0, len(sorted))
	for _, l := range sorted {
		prov := endCertificates(certs, EndKey{Service: l.ProviderService, Node: l.ProviderNode})
		cons := endCertificates(certs, EndKey{Service: l.ConsumerService, Node: l.ConsumerNode})
		c := CertifiedLink{
			Link:              l,
			Provider:          prov,
			Consumer:          cons,
			ProviderCertified: anyLive(prov),
			ConsumerCertified: anyLive(cons),
			Gating:            gating[l],
		}
		c.Certified = c.ProviderCertified && c.ConsumerCertified
		out.Links = append(out.Links, c)
		switch {
		case c.Certified:
			out.Certified++
		case c.Gating:
			out.Uncertified = append(out.Uncertified, c)
		default:
			out.Hints = append(out.Hints, c)
		}
	}
	return out
}

// endCertificates returns a sorted copy of the certificates at k (nil when
// there are none), so the caller's slices are never reordered.
func endCertificates(certs EndCertificates, k EndKey) []EndCertificate {
	src := certs[k]
	if len(src) == 0 {
		return nil
	}
	out := append([]EndCertificate(nil), src...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Test != out[j].Test {
			return out[i].Test < out[j].Test
		}
		if out[i].Target != out[j].Target {
			return out[i].Target < out[j].Target
		}
		return out[i].State < out[j].State
	})
	return out
}

// anyLive reports whether any certificate is live (I43: State.Satisfies).
func anyLive(cs []EndCertificate) bool {
	for _, c := range cs {
		if c.State.Satisfies() {
			return true
		}
	}
	return false
}

// uncertifiedLinksForReceipt maps CertifyLinks' uncertified spec and typed
// links onto the change risk receipt's `uncertified_contracts` input
// (review.Input.UncertifiedContracts, TQ-18): Provider names the provider
// node with its service, Consumer the consumer node, ConsumerService the
// consumer's service ("" when it is the provider's own, LinkRef's
// contract), Tier the link's tier and Citation both ends' file:line. The
// list is in the link join's order.
//
// I57 is re-applied here rather than trusted from the field: a link that
// syslink.GatingLinks does not admit never reaches the receipt, even in a
// hand-built LinkCertification.
func uncertifiedLinksForReceipt(c LinkCertification) []review.LinkRef {
	if len(c.Uncertified) == 0 {
		return nil
	}
	links := make([]syslink.Link, 0, len(c.Uncertified))
	for _, u := range c.Uncertified {
		links = append(links, u.Link)
	}
	admitted := syslink.GatingLinks(links)
	out := make([]review.LinkRef, 0, len(admitted))
	for _, l := range admitted {
		ref := review.LinkRef{
			Provider: l.ProviderService + "/" + l.ProviderNode,
			Consumer: l.ConsumerNode,
			Tier:     string(l.Tier),
			Citation: citeEnd(l.Evidence.Consumer) + " -> " + citeEnd(l.Evidence.Provider),
		}
		if l.ConsumerService != l.ProviderService {
			ref.ConsumerService = l.ConsumerService
		}
		out = append(out, ref)
	}
	return out
}

// citeEnd renders one link end's evidence as file#Lline (file alone when
// the line is unknown).
func citeEnd(e syslink.End) string {
	if e.Line > 0 {
		return e.File + "#L" + strconv.Itoa(e.Line)
	}
	return e.File
}
