package console

// claims.go — one governed claim, whole (design §3 "Claim page", plan
// OC-30): the statement, the lines it cites, its scope, evidence tier and
// status, its provenance, and every verdict recorded against it
// (re-anchored, contradicted, corroborated, refuted, superseded).
// `fathom claim show <id> --json` prints exactly this; /claims/{id}
// renders it, the citation through the kit's code excerpt.
//
// The citation excerpt follows the J2 rule serving follows: a stale or
// contradicted claim's cited range in the CURRENT file may now hold
// unrelated lines, so when the compile-time anchor text was kept, that is
// what is shown, labeled as such — never the current lines passed off as
// the rule's source. A superseded or refuted claim is the same case, only
// more so: those verdicts are terminal, so the sweep never re-anchors its
// citation (internal/semantic/sweep.go) and the range freezes while the
// file moves on. Only a fresh claim's range is kept current, so only a
// fresh claim's current lines are shown as what it cites.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// ClaimPage is the claim page.
type ClaimPage struct {
	ClaimID string `json:"claim_id"`
	// Found is false when no service holds the claim; Matches lists every
	// service holding it when more than one does and none was named.
	Found   bool       `json:"found"`
	Matches []ClaimRef `json:"matches,omitempty"`

	Service   string   `json:"service,omitempty"`
	Layer     string   `json:"layer,omitempty"`
	Kind      string   `json:"kind,omitempty"`
	Subject   string   `json:"subject,omitempty"`
	Statement string   `json:"statement,omitempty"`
	Scope     []string `json:"scope,omitempty"`
	Tier      string   `json:"tier,omitempty"`
	Status    string   `json:"status,omitempty"`
	// Served is whether serving can still hand the claim to an agent
	// (fresh, stale or contradicted).
	Served bool `json:"served"`

	Model               string `json:"model,omitempty"`
	PromptVersion       string `json:"prompt_version,omitempty"`
	CompiledAtWatermark string `json:"compiled_at_watermark,omitempty"`
	CompiledAt          string `json:"compiled_at,omitempty"`
	ValidFrom           string `json:"valid_from,omitempty"`
	ValidTo             string `json:"valid_to,omitempty"`
	AssertedBy          string `json:"asserted_by,omitempty"`

	Citations []Citation     `json:"citations,omitempty"`
	Excerpt   Value[Excerpt] `json:"excerpt"`
	Links     []ClaimLinkRef `json:"links,omitempty"`
	// Events is the claim's verdict history, oldest first; EventCount
	// carries its truth state (an unreadable history is an error, never an
	// empty one).
	Events     []ClaimEventRow `json:"events"`
	EventCount Value[int]      `json:"event_count"`
}

// ClaimRef names one service's copy of a claim id.
type ClaimRef struct {
	Service string `json:"service"`
	Layer   string `json:"layer"`
	Status  string `json:"status"`
}

// Citation is one cites link.
type Citation struct {
	Ref   string `json:"ref"` // path#Lstart-Lend
	Path  string `json:"path"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	// AnchorKept is true when the compile-time text of the cited lines was
	// stored beside the hash.
	AnchorKept bool `json:"anchor_kept"`
}

// ClaimLinkRef is a non-citation link (scopes, anchors, supersedes, …).
type ClaimLinkRef struct {
	Rel    string `json:"rel"`
	Target string `json:"target"`
	Note   string `json:"note,omitempty"`
}

// ClaimEventRow is one recorded verdict.
type ClaimEventRow struct {
	Verdict    string `json:"verdict"`
	At         string `json:"at"`
	Author     string `json:"author,omitempty"`
	Note       string `json:"note,omitempty"`
	Resolution string `json:"resolution,omitempty"`
	Commit     string `json:"commit,omitempty"`
}

// Excerpt is the cited lines with context.
type Excerpt struct {
	Path  string `json:"path"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	// From is "current file" or "compile-time anchor text".
	From  string        `json:"from"`
	Note  string        `json:"note,omitempty"`
	Lines []ExcerptLine `json:"lines"`
}

// ExcerptLine is one numbered line; Cited marks the cited range.
type ExcerptLine struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
	Cited  bool   `json:"cited,omitempty"`
}

// Excerpt bounds: two context lines each side, at most 60 lines shown.
const (
	excerptContext  = 2
	excerptMaxLines = 60
)

// ClaimDetail finds claimID (in service, when named) and reads its
// citation, links and events.
func ClaimDetail(ctx context.Context, src KnowledgeSource, claimID, service string) (ClaimPage, error) {
	st := src.Store
	out := ClaimPage{ClaimID: claimID, Events: []ClaimEventRow{}}
	svcs, err := st.ClaimServiceIDs(ctx)
	if err != nil {
		return out, err
	}
	type hit struct {
		svc string
		cl  core.SemanticClaim
	}
	var hits []hit
	for _, svc := range svcs {
		if service != "" && svc != service {
			continue
		}
		claims, cerr := st.ClaimsForService(ctx, svc, "")
		if cerr != nil {
			return out, fmt.Errorf("claims for %s: %w", svc, cerr)
		}
		for _, cl := range claims {
			if cl.ClaimID == claimID {
				hits = append(hits, hit{svc, cl})
			}
		}
	}
	switch len(hits) {
	case 0:
		out.Excerpt = Absent[Excerpt]("no claim " + claimID + " in the store")
		out.EventCount = Absent[int]("no claim " + claimID + " in the store")
		return out, nil
	case 1:
	default:
		for _, h := range hits {
			out.Matches = append(out.Matches, ClaimRef{Service: h.svc, Layer: h.cl.Layer, Status: string(h.cl.Status)})
		}
		sort.Slice(out.Matches, func(i, j int) bool { return out.Matches[i].Service < out.Matches[j].Service })
		out.Excerpt = Absent[Excerpt](fmt.Sprintf("%d services hold this claim id; name one", len(hits)))
		out.EventCount = Absent[int]("ambiguous claim id")
		return out, nil
	}
	cl := hits[0].cl
	out.Found = true
	out.Service, out.Layer, out.Kind, out.Subject, out.Statement = hits[0].svc, cl.Layer, cl.Kind, cl.Subject, cl.Rule
	out.Scope = append([]string{}, cl.Scope...)
	out.Tier, out.Status, out.Served = string(cl.Tier), string(cl.Status), cl.Status.Served()
	out.Model, out.PromptVersion, out.CompiledAtWatermark = cl.Model, cl.PromptVersion, cl.CompiledAtWatermark
	out.CompiledAt, out.ValidFrom, out.ValidTo = rfc3339(cl.CompiledAt), rfc3339(cl.ValidFrom), rfc3339(cl.ValidTo)
	out.AssertedBy = cl.AssertedBy

	links, lerr := st.ClaimLinks(ctx, out.Service, []string{claimID})
	var anchor string
	if lerr == nil {
		for _, l := range links {
			if l.Rel == core.RelCites {
				path, start, end, ok := parseCiteRef(l.TargetRef)
				if !ok {
					out.Links = append(out.Links, ClaimLinkRef{Rel: l.Rel, Target: l.TargetRef, Note: "not a line citation"})
					continue
				}
				out.Citations = append(out.Citations, Citation{Ref: l.TargetRef, Path: path, Start: start, End: end, AnchorKept: l.AnchorText != ""})
				if anchor == "" && len(out.Citations) == 1 {
					anchor = l.AnchorText
				}
				continue
			}
			out.Links = append(out.Links, ClaimLinkRef{Rel: l.Rel, Target: l.TargetRef, Note: l.AnchorText})
		}
	}
	root := ""
	if meta, found, merr := st.GetServiceMeta(ctx, out.Service); merr == nil && found {
		root = meta.Path
	}
	switch {
	case lerr != nil:
		out.Excerpt = Errored[Excerpt]("claim links unreadable: " + lerr.Error())
	case len(out.Citations) == 0:
		out.Excerpt = Absent[Excerpt]("the claim carries no line citation")
	default:
		out.Excerpt = citationExcerpt(root, out.Citations[0], cl.Status, anchor)
	}

	evs, eerr := st.ClaimEvents(ctx, claimID)
	if eerr != nil {
		out.EventCount = Errored[int]("claim events unreadable: " + eerr.Error())
		return out, nil
	}
	for _, e := range evs {
		out.Events = append(out.Events, ClaimEventRow{
			Verdict: e.Verdict, At: rfc3339(e.At), Author: e.Author, Note: e.Note,
			Resolution: e.ResolutionKind, Commit: e.ResolutionCommit,
		})
	}
	out.EventCount = Measured(len(evs), "claim_events").WithDenominator("verdicts recorded against this claim")
	return out, nil
}

// citationMayHaveMoved reports whether a claim's cited range may no longer
// hold what it restates, and why: stale and contradicted claims drifted
// from their text; superseded and refuted ones are never re-anchored.
func citationMayHaveMoved(status core.ClaimStatus) (bool, string) {
	switch status {
	case core.ClaimStale, core.ClaimContradicted:
		return true, "the claim is " + string(status)
	case core.ClaimSuperseded, core.ClaimRefuted:
		return true, "the claim is " + string(status) + ", a terminal verdict whose citation is never re-anchored as the file changes"
	}
	return false, ""
}

// citationExcerpt reads the cited lines with context. A claim whose range
// may have moved (stale, contradicted, superseded, refuted) and whose
// anchor text was kept shows that text instead (J2); without it, the
// current lines are shown with a note saying they may not be the source.
func citationExcerpt(root string, c Citation, status core.ClaimStatus, anchor string) Value[Excerpt] {
	const src = "the cited file under the service's recorded root"
	drifted, why := citationMayHaveMoved(status)
	if drifted && anchor != "" {
		ex := Excerpt{Path: c.Path, Start: c.Start, End: c.End, From: "compile-time anchor text",
			Note: "cited text at compile — " + why + ", so the current file's lines here may no longer be what it restates"}
		for i, line := range strings.Split(strings.TrimRight(anchor, "\n"), "\n") {
			if i >= excerptMaxLines {
				ex.Note += fmt.Sprintf("; first %d lines shown", excerptMaxLines)
				break
			}
			ex.Lines = append(ex.Lines, ExcerptLine{Number: c.Start + i, Text: line, Cited: true})
		}
		return Present(ex, "claim_links.anchor_text")
	}
	if root == "" {
		return Absent[Excerpt]("no corpus root is recorded for this service, so the cited file cannot be read here")
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.Path)))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Absent[Excerpt]("the cited file " + c.Path + " no longer exists under " + root)
	case err != nil:
		return Errored[Excerpt]("the cited file could not be read: " + err.Error())
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if c.Start < 1 || c.Start > len(lines) {
		return Absent[Excerpt](fmt.Sprintf("the cited range starts at line %d; the file now has %d lines", c.Start, len(lines)))
	}
	end := c.End
	if end < c.Start {
		end = c.Start
	}
	from := max(1, c.Start-excerptContext)
	to := min(len(lines), end+excerptContext)
	ex := Excerpt{Path: c.Path, Start: c.Start, End: end, From: "current file"}
	if drifted {
		ex.Note = why + ", and its compile-time text was not kept: these are the current lines at the cited range, which may have moved"
	}
	for n := from; n <= to; n++ {
		if len(ex.Lines) >= excerptMaxLines {
			ex.Note = strings.TrimPrefix(ex.Note+fmt.Sprintf("; first %d lines shown", excerptMaxLines), "; ")
			break
		}
		ex.Lines = append(ex.Lines, ExcerptLine{Number: n, Text: lines[n-1], Cited: n >= c.Start && n <= end})
	}
	return Present(ex, src)
}

// parseCiteRef splits "path#Lstart-Lend" (or "path#Lstart").
func parseCiteRef(ref string) (path string, start, end int, ok bool) {
	i := strings.LastIndex(ref, "#L")
	if i <= 0 {
		return "", 0, 0, false
	}
	path, rng := ref[:i], ref[i+2:]
	a, b, found := strings.Cut(rng, "-")
	s, err := strconv.Atoi(a)
	if err != nil || s < 1 {
		return "", 0, 0, false
	}
	e := s
	if found {
		e, err = strconv.Atoi(strings.TrimPrefix(b, "L"))
		if err != nil || e < s {
			return "", 0, 0, false
		}
	}
	return path, s, e, true
}
