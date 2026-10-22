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
// ride the decision-moment payload whose whole budget is ~320 tokens (ADR-016, amended).

import (
	"context"
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
func obligationSnippet(ctx context.Context, backend QueryInterface, service, cite string, _ bool) (string, bool) {
	file, start, end, ok := parseCitation(cite)
	if !ok {
		return "", false
	}
	var rng sourceRange
	var err error
	rng, err = readSourceRange(ctx, backend, service, file, start, end, end-start+1, obligationSnippetMaxBytes*2)
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

// staleSnippetLabel (J2, app-kit O6/O16/O24/O26/O29/O30) is the exact label
// every renderer attaches to a stale/contradicted rule's snippet — it is
// NEVER a live read of the current file at the rule's stored citation
// range (that range may now hold unrelated content; sweep.go already
// proved the hash no longer matches), it is the literal text the rule was
// compiled against.
const staleSnippetLabel = "cited text at compile; current file differs"

// withSnippets renders served rules as JSON objects with an inlined
// "snippet" (BP-20). A rule whose citation cannot be read keeps exactly the
// shape it had before this field existed — the absence is the honest
// answer. Its typed wire row retains the former map's alphabetical key order.
//
// A stale/contradicted rule (r.Status != "") never reaches obligationSnippet
// at all: that function reads the CURRENT file at the rule's OLD cited line
// range, which is exactly the unsafe read J2 exists to stop — a stale
// citation's line range can hold content that moved there for unrelated
// reasons. Instead its stored AnchorText (semantic.Obligation's J2 field,
// set only when the claim's cites link recorded compile-time text) renders
// as "snippet", labeled with staleSnippetLabel via "snippet_label" so a
// caller can never mistake it for a live read. A stale rule with no stored
// AnchorText (a legacy hash-only citation) gets neither field — an honest
// absence, never a fabricated or unsafe one.
type obligationWithSnippet struct {
	AnchorCurrent  *bool                    `json:"anchor_current,omitempty"`
	AnchorText     string                   `json:"anchor_text,omitempty"`
	AssertedBy     string                   `json:"asserted_by,omitempty"`
	CitationStatus *semantic.CitationStatus `json:"citation_status,omitempty"`
	ClaimID        string                   `json:"claim_id,omitempty"`
	Class          string                   `json:"class,omitempty"`
	CodeDrifted    bool                     `json:"code_drifted,omitempty"`
	InForce        string                   `json:"in_force,omitempty"`
	Kind           string                   `json:"kind,omitempty"`
	Layer          string                   `json:"layer"`
	Module         string                   `json:"module,omitempty"`
	ObservedAt     string                   `json:"observed_at,omitempty"`
	Producer       string                   `json:"producer,omitempty"`
	Relevance      *float64                 `json:"relevance,omitempty"`
	Remedy         string                   `json:"remedy,omitempty"`
	Rule           string                   `json:"rule"`
	RuleID         string                   `json:"rule_id,omitempty"`
	Snippet        string                   `json:"snippet,omitempty"`
	SnippetLabel   string                   `json:"snippet_label,omitempty"`
	SourceID       string                   `json:"source_id,omitempty"`
	SourceRevision string                   `json:"source_revision,omitempty"`
	Src            string                   `json:"src"`
	Status         string                   `json:"status,omitempty"`
	Tier           string                   `json:"tier"`
	Unanchored     bool                     `json:"unanchored,omitempty"`
	Why            string                   `json:"why,omitempty"`
}

func obligationSnippetRow(r semantic.Obligation) *obligationWithSnippet {
	return &obligationWithSnippet{AnchorCurrent: r.AnchorCurrent, AnchorText: r.AnchorText, AssertedBy: r.AssertedBy, CitationStatus: r.CitationStatus, ClaimID: r.ClaimID, Class: r.Class, CodeDrifted: r.CodeDrifted, InForce: r.InForce, Kind: r.Kind, Layer: r.Layer, Module: r.Module, ObservedAt: r.ObservedAt, Producer: r.Producer, Relevance: r.Relevance, Remedy: r.Remedy, Rule: r.Rule, RuleID: r.RuleID, SourceID: r.SourceID, SourceRevision: r.SourceRevision, Src: r.Src, Status: r.Status, Tier: r.Tier, Unanchored: r.Unanchored, Why: r.Why}
}
func withSnippets(ctx context.Context, backend QueryInterface, service string, rules []semantic.Obligation) []any {
	prepareObligationWindows(ctx, service, rules)
	out := make([]any, 0, len(rules))
	for i := range rules {
		r := &rules[i]
		row := obligationSnippetRow(*r)
		if file, _, _, ok := parseCitation(r.Src); ok && r.Tier != "raw" {
			bindCitation(ctx, backend, service, file, func(status *semantic.CitationStatus) { row.CitationStatus = status })
		}
		if r.Status != "" {
			if r.AnchorText != "" {
				row.Snippet = r.AnchorText
				row.SnippetLabel = staleSnippetLabel
			}
		} else if snippet, ok := obligationSnippet(ctx, backend, service, r.Src, false); ok {
			row.Snippet = snippet
		}
		out = append(out, row)
	}
	return out
}
