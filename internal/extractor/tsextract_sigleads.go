package extractor

import "github.com/geoffmilleraz/fathom/internal/extractor/tsengine"

// tsSignatureLeads applies a dialect query's @signature.lead and
// @signature.of pairs (TQ-27): a grammar that makes an item's attributes its
// siblings (Rust's #[test] beside the function_item) leaves the item's own
// node without them, so a pattern captures each attribute as the lead of the
// sibling right after it. A definition's signature then starts at the first
// lead of the chain ending at its node: `#[test] #[should_panic] fn f()`
// reads as one signature. Spans, lines and calls are unchanged, and a
// dialect whose query has no pair is unaffected.
func tsSignatureLeads(matches []tsengine.Match, defs []*tsDef) {
	var leadOf map[uint32]uint32
	for _, m := range matches {
		var lead, of *tsengine.Node
		for i := range m.Captures {
			c := &m.Captures[i]
			switch c.Name {
			case "signature.lead":
				n := c.Node
				lead = &n
			case "signature.of":
				n := c.Node
				of = &n
			}
		}
		if lead == nil || of == nil || lead.StartByte >= of.StartByte {
			continue
		}
		if leadOf == nil {
			leadOf = map[uint32]uint32{}
		}
		leadOf[of.StartByte] = lead.StartByte
	}
	if leadOf == nil {
		return
	}
	for _, d := range defs {
		s := d.start
		for {
			l, ok := leadOf[s]
			if !ok || l >= s {
				break
			}
			s = l
		}
		if s < d.start {
			d.sigStart, d.hasSigStart = s, true
		}
	}
}
