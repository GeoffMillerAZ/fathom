package expertchat

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	scoring "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/review/scoring"
)

// ErrNoCitations is returned by NewCitedAnswer when citations is empty. It is
// the type-level enforcement of "an Answer with zero citations must be
// unrepresentable": CitedAnswer's fields are unexported, so the only public
// way to build a non-zero CitedAnswer is NewCitedAnswer, and NewCitedAnswer
// refuses to return one without at least one citation.
var ErrNoCitations = errors.New("expertchat: CitedAnswer requires at least one citation")

// Citation is one claim's resolved provenance, attached to a composed
// CitedAnswer. Every field mirrors the "why trusted / when stopped being
// sure / what compiled from" triad (docs/spec/expert-module-v0.md §2): Module
// + ModuleVersion + Watermark answer "what was it compiled from", Tier
// answers "why should this be trusted", Status answers "when did it stop
// being sure", and SourceRef + Anchor let a reader verify the claim
// independently of expertchat's own say-so.
type Citation struct {
	// Module is the module the cited claim came from.
	Module string
	// ModuleVersion is that module's manifest version.
	ModuleVersion string
	// Watermark is that module's corpus watermark at compile time.
	Watermark string
	// ClaimID is the cited claim's identifier within Module.
	ClaimID string
	// Tier is the cited claim's evidence tier (foundations/review/scoring).
	Tier scoring.Tier
	// Status is the cited claim's staleness verdict at retrieval time: one of
	// StatusFresh or StatusStale (StatusContradicted/StatusRefuted claims are
	// never citable — see EvidenceItem.Citable).
	Status string
	// SourceRef locates the claim's citation within its module's corpus.
	SourceRef string
	// Anchor is the quoted anchor text the claim is grounded in.
	Anchor string
}

// citationFromEvidence builds a Citation from the EvidenceItem it resolves,
// dropping the Text field (Citation carries provenance only, not the
// evidence content shown to the model — requirement 2 of the founding plan).
func citationFromEvidence(e EvidenceItem) Citation {
	return Citation{
		Module:        e.Module,
		ModuleVersion: e.ModuleVersion,
		Watermark:     e.Watermark,
		ClaimID:       e.ClaimID,
		Tier:          e.Tier,
		Status:        e.Status,
		SourceRef:     e.SourceRef,
		Anchor:        e.Anchor,
	}
}

// CitedAnswer is the citation-mandatory response type: a governed answer to
// a chat query, backed by at least one Citation. The zero value
// (CitedAnswer{}) is not a usable answer — Text() returns "" and Citations()
// returns nil — but more importantly it can never be turned into one except
// through NewCitedAnswer, because both fields are unexported. There is no
// setter, no exported struct literal path, and no way to append a citation
// after construction: an "answer" with zero citations simply cannot be
// built through this package's public API.
type CitedAnswer struct {
	text      string
	citations []Citation
}

// NewCitedAnswer constructs a CitedAnswer. It fails when citations is empty
// (ErrNoCitations) or when any citation is missing a required identifying
// field (ClaimID, Module, or Tier) — a citation that cannot be traced back to
// a claim is not a citation. Citations are defensively copied and sorted by
// (Module, ClaimID) so CitedAnswer.Citations() is always deterministic
// regardless of the order the caller assembled them in (the kit's
// determinism contract: sort outputs).
func NewCitedAnswer(text string, citations []Citation) (CitedAnswer, error) {
	if len(citations) == 0 {
		return CitedAnswer{}, ErrNoCitations
	}
	cp := make([]Citation, len(citations))
	copy(cp, citations)
	for i, c := range cp {
		if strings.TrimSpace(c.ClaimID) == "" {
			return CitedAnswer{}, fmt.Errorf("expertchat: citation %d has an empty ClaimID", i)
		}
		if strings.TrimSpace(c.Module) == "" {
			return CitedAnswer{}, fmt.Errorf("expertchat: citation %d (claim %q) has an empty Module", i, c.ClaimID)
		}
		if strings.TrimSpace(string(c.Tier)) == "" {
			return CitedAnswer{}, fmt.Errorf("expertchat: citation %d (claim %q) has an empty Tier", i, c.ClaimID)
		}
	}
	sort.Slice(cp, func(i, j int) bool {
		if cp[i].Module != cp[j].Module {
			return cp[i].Module < cp[j].Module
		}
		return cp[i].ClaimID < cp[j].ClaimID
	})
	return CitedAnswer{text: text, citations: cp}, nil
}

// Text returns the composed answer text.
func (a CitedAnswer) Text() string { return a.text }

// Citations returns a copy of the answer's citations, sorted deterministically
// by (Module, ClaimID). Always non-empty for a value constructed via
// NewCitedAnswer; empty for the zero value.
func (a CitedAnswer) Citations() []Citation {
	out := make([]Citation, len(a.citations))
	copy(out, a.citations)
	return out
}

// Render produces a deterministic, byte-stable textual rendering of the
// answer: the answer text, followed by one "- " line per citation in
// Citations() order (already sorted). Two CitedAnswer values built from
// equal inputs render identically — the property GKE-EC-5's replay-backend
// test coverage exercises ("a fixed module-host response cassette produces a
// byte-identical composed answer").
func (a CitedAnswer) Render() string {
	var b strings.Builder
	b.WriteString(a.text)
	if len(a.citations) > 0 {
		b.WriteString("\n\nCitations:\n")
		for _, c := range a.citations {
			fmt.Fprintf(&b, "- [%s] %s/%s tier=%s status=%s source=%s anchor=%q\n",
				c.ClaimID, c.Module, c.ModuleVersion, c.Tier, c.Status, c.SourceRef, c.Anchor)
		}
	}
	return b.String()
}
