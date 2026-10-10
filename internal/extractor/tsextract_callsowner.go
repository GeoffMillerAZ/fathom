package extractor

import "strings"

// tsextract_callsowner.go — two call-site rules a manifest's calls block turns
// on (wave lf5, item html; design §1.8, W1 and its K19 pair key):
//
//   - calls.owner "module": every call site is the file's module symbol's,
//     whatever definition encloses it (extractLocked skips attachTSCalls and
//     mintTSModule counts no span as owned). HTML's ID references are
//     resolved against the document, never an ancestor element.
//   - calls.split "whitespace": a call site's text is split on ASCII
//     whitespace, one callee per token (splitTSCallSites): an attribute that
//     holds a set of IDs names each of them.
//
// A dialect whose manifest names neither is unaffected.

// splitTSCallSites is calls with each site's text split on ASCII whitespace
// (tab, line feed, form feed, carriage return, space: HTML's "split a string
// on ASCII whitespace"), one site per token at the site's range; a site whose
// text is only whitespace names nothing.
func splitTSCallSites(calls []tsCallSite) []tsCallSite {
	out := make([]tsCallSite, 0, len(calls))
	for _, c := range calls {
		for _, tok := range strings.FieldsFunc(c.text, isASCIIWhitespace) {
			out = append(out, tsCallSite{text: tok, start: c.start, end: c.end})
		}
	}
	return out
}

func isASCIIWhitespace(r rune) bool {
	return r == '\t' || r == '\n' || r == '\f' || r == '\r' || r == ' '
}
