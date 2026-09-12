package mcp

// obligationsnippet.go — BP-20 (docs/plans/bench-pyramid.md, ADR-032 §4
// D3): a served rule carries the lines it cites.
//
// The stopped replicated run measured the cost of NOT doing this: in 5 of
// the 6 fathom cells that got a useful rule, the agent immediately spent a
// whole extra turn round-tripping read_source on that rule's citation. The
// rule text is a 28-word restatement; the agent wants the source. Inlining
// a capped snippet is one turn saved per rule the agent trusts, and the
// pattern already exists — symbol_context's T5-6 definition snippet, read
// through the very same bounded readSourceRange path, so a snippet and a
// read_source citation of the same lines are byte-identical.
//
// The cap is deliberately small (240 bytes, versus symbol_context's 40
// lines): a citation is a rule's anchor, not its body, and six of these
// ride the decision-moment payload whose whole budget is ~150 tokens.

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// obligationSnippetMaxBytes caps one inlined citation. A rule's cited lines
// are an anchor, not a body; six of them share the obligations payload.
const obligationSnippetMaxBytes = 240

// obligationSnippetCutNote is the NAMED cut (law 3: a truncation is stated,
// never silent) appended when the cited lines exceed the cap. It is counted
// against the cap, so the returned string is never longer than it.
const obligationSnippetCutNote = " … (cut at 240 bytes)"

// obligationSnippet returns the text of a rule's cited line range, read
// through the same bounded, indexed-corpus-enforcing path read_source and
// symbol_context use.
//
// ok=false is an honest absence, never a fabricated empty string: the
// citation did not parse, the service has no recorded corpus root, the
// cited file is outside the indexed corpus, or the range no longer
// resolves. The caller omits the field rather than emitting "".
func obligationSnippet(ctx context.Context, backend QueryInterface, service, cite string) (string, bool) {
	file, start, end, ok := parseCitation(cite)
	if !ok {
		return "", false
	}
	rng, err := readSourceRange(ctx, backend, service, file, start, end, end-start+1, obligationSnippetMaxBytes*2)
	if err != nil || len(rng.Lines) == 0 {
		return "", false
	}
	parts := make([]string, 0, len(rng.Lines))
	for _, ln := range rng.Lines {
		parts = append(parts, strings.TrimRight(ln.Text, " \t"))
	}
	text := strings.TrimSpace(strings.Join(parts, "\n"))
	if text == "" {
		return "", false
	}
	if len(text) > obligationSnippetMaxBytes {
		keep := obligationSnippetMaxBytes - len(obligationSnippetCutNote)
		// Cut on a rune boundary: a snippet that ends mid-UTF-8 is invalid
		// JSON content dressed up as a citation.
		for keep > 0 && !utf8Start(text[keep]) {
			keep--
		}
		text = strings.TrimRight(text[:keep], " \t\n") + obligationSnippetCutNote
	}
	return text, true
}

// utf8Start reports whether b can begin a UTF-8 rune (i.e. is not a
// continuation byte).
func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

// parseCitation splits a claim citation — "path#Lstart-Lend", or
// "path#Lstart" for a single line (core.CiteRef's shape) — into its parts.
func parseCitation(ref string) (file string, start, end int, ok bool) {
	file, frag, found := strings.Cut(ref, "#L")
	if !found || file == "" {
		return "", 0, 0, false
	}
	lo, hi, ranged := strings.Cut(frag, "-L")
	start, err := strconv.Atoi(lo)
	if err != nil || start < 1 {
		return "", 0, 0, false
	}
	end = start
	if ranged {
		if end, err = strconv.Atoi(hi); err != nil || end < start {
			return "", 0, 0, false
		}
	}
	return file, start, end, true
}

// withSnippets renders served rules as JSON objects with an inlined
// "snippet" (BP-20). A rule whose citation cannot be read keeps exactly the
// shape it had before this field existed — the absence is the honest
// answer, and every other field is carried through untouched by
// round-tripping the rule's own JSON rather than re-listing its fields here
// (semantic.Obligation is extended additively; a hand-copied field list
// would silently drop the next one).
func withSnippets(ctx context.Context, backend QueryInterface, service string, rules []semantic.Obligation) []any {
	out := make([]any, 0, len(rules))
	for _, r := range rules {
		raw, err := json.Marshal(r)
		if err != nil {
			out = append(out, r)
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			out = append(out, r)
			continue
		}
		if snip, ok := obligationSnippet(ctx, backend, service, r.Src); ok {
			m["snippet"] = snip
		}
		out = append(out, m)
	}
	return out
}
