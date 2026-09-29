package semantic

// carry.go — AG-26 (docs/plans/agents-md.md, design docs/design/agents-md.md
// §3.4): the digest gate keys on a section's subject ("<path>#<slug>"), so
// moving a guide (CLAUDE.md to AGENTS.md, ADR-059) changed every section's
// subject and paid a model call for text already compiled. A section whose
// content digest and prompt identity match a section compiled at another
// path now keeps that section's claims, re-cited to the new path and lines,
// with no model call.
//
// The carry is a one-to-one move, never a guess. A section carries only when
// all of these hold:
//
//  1. its own subject has no claims compiled from its digest under this
//     prompt (the ordinary digest gate would compile it);
//  2. exactly one prior subject that has LEFT the current corpus holds live
//     claims, every one compiled from this digest under this prompt (two
//     departed subjects with the digest: their claim sets need not agree);
//  3. exactly one section of the current corpus has this digest (two
//     current copies: which one inherits is a guess);
//  4. both paths share one scope floor (scopeFloorDir) and neither is a
//     rules file that declares its own scope (the claims' scopes were
//     verified for the old path's floor and the model's own globs);
//  5. every cited line range of the departed claims re-anchors, by one
//     single line shift, onto lines of the new section that hash-match.
//
// Anything else compiles as before. The departed subject's claims retire
// through Compile's ordinary supersede sweep (their subject left the
// corpus), and each carried claim records a supersedes link to the claim it
// replaces.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// carriedSection is one section AG-26 carried: the departed subject it came
// from and the claims and links minted for the new subject.
type carriedSection struct {
	from   string
	claims []core.SemanticClaim
	links  []core.ClaimLink
}

// needsCompile is the digest gate (lifted from Compile's closure for AG-26):
// a section needs the model unless its own subject already holds claims all
// compiled from its digest under promptVersion, or AG-26 carried claims to
// it from a section compiled at another path.
func needsCompile(bySubject map[string][]core.SemanticClaim, carried map[string]carriedSection, sec Section, promptVersion string) bool {
	if _, ok := carried[sec.Subject]; ok {
		return false
	}
	prior := bySubject[sec.Subject]
	return len(prior) == 0 || !allDigestsMatch(prior, sec.Digest, promptVersion)
}

// carryParams is what planCarries stamps on every carried claim: this
// run's identity, not the departed claim's.
type carryParams struct {
	serviceID     string
	promptVersion string
	watermark     string
	runID         string
	at            time.Time
	anchorSymbol  func(claimID, citeRef string, fetch func(string) ([]byte, bool)) (core.ClaimLink, bool)
}

// planCarries returns, by new subject, every section of the current corpus
// the one-to-one rule (this file's header) lets keep a departed section's
// claims. Pure over its inputs except for reading the departed claims'
// links from st. Deterministic: sections are walked in corpus order and the
// maps it reads are only indexed, never ranged into the output unsorted.
func planCarries(ctx context.Context, st store.Store, all []docSections, bySubject map[string][]core.SemanticClaim, currentSubjects map[string]bool, p carryParams) (map[string]carriedSection, error) {
	// Rule 2's index: digest -> departed subjects whose live claims were all
	// compiled from it under this prompt.
	departed := map[string][]string{}
	subjects := make([]string, 0, len(bySubject))
	for subj := range bySubject {
		subjects = append(subjects, subj)
	}
	sort.Strings(subjects)
	for _, subj := range subjects {
		claims := bySubject[subj]
		if currentSubjects[subj] || len(claims) == 0 {
			continue
		}
		digest := claims[0].InputDigest
		if digest == "" || !allDigestsMatch(claims, digest, p.promptVersion) {
			continue
		}
		departed[digest] = append(departed[digest], subj)
	}
	if len(departed) == 0 {
		return nil, nil
	}

	// Rule 3's count: how many current sections carry each digest.
	current := map[string]int{}
	for _, ds := range all {
		for _, sec := range ds.sections {
			current[sec.Digest]++
		}
	}

	var out map[string]carriedSection
	for _, ds := range all {
		for _, sec := range ds.sections {
			if !needsCompile(bySubject, nil, sec, p.promptVersion) {
				continue // rule 1
			}
			from := departed[sec.Digest]
			if len(from) != 1 || current[sec.Digest] != 1 {
				continue // rules 2 and 3
			}
			cs, ok, err := carrySection(ctx, st, ds.doc, sec, from[0], bySubject[from[0]], p)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			if out == nil {
				out = map[string]carriedSection{}
			}
			out[sec.Subject] = cs
		}
	}
	return out, nil
}

// carrySection applies rules 4 and 5 to one candidate and, when both hold,
// mints the carried claims and their links. ok=false means the section
// compiles as before.
func carrySection(ctx context.Context, st store.Store, doc core.SourceFile, sec Section, from string, prior []core.SemanticClaim, p carryParams) (carriedSection, bool, error) {
	fromPath := subjectPath(from)
	if fromPath == "" || scopeFloorDir(fromPath) != scopeFloorDir(sec.Path) {
		return carriedSection{}, false, nil
	}
	if _, owned := declaredDocScope(fromPath, nil); owned {
		return carriedSection{}, false, nil
	}
	if _, owned := declaredDocScope(sec.Path, doc.Content); owned {
		return carriedSection{}, false, nil
	}

	ids := make([]string, 0, len(prior))
	for _, cl := range prior {
		ids = append(ids, cl.ClaimID)
	}
	links, err := st.ClaimLinks(ctx, p.serviceID, ids)
	if err != nil {
		return carriedSection{}, false, fmt.Errorf("semantic: carry %s: load links: %w", from, err)
	}
	shift, ok := citationShift(doc.Content, sec, ids, links)
	if !ok {
		return carriedSection{}, false, nil
	}

	fetch := func(path string) ([]byte, bool) {
		if path == doc.Path {
			return doc.Content, true
		}
		return nil, false
	}
	byClaim := map[string][]core.ClaimLink{}
	for _, l := range links {
		byClaim[l.ClaimID] = append(byClaim[l.ClaimID], l)
	}
	cs := carriedSection{from: from}
	for _, old := range prior {
		cl := old
		cl.Subject = sec.Subject
		cl.ClaimID = core.ClaimID(old.Layer, old.Kind, sec.Subject, old.ReproKey)
		cl.Status = core.ClaimFresh
		cl.CompiledAtWatermark = p.watermark
		cl.ValidFrom = p.at.UTC()
		cl.ValidTo = time.Time{}
		cl.AssertedBy = core.AssertedBy("compile", p.runID)
		cs.claims = append(cs.claims, cl)

		var oldCite, newCite string
		hadAnchor := false
		for _, l := range byClaim[old.ClaimID] {
			switch l.Rel {
			case core.RelCites:
				_, s, e, _ := core.ParseCiteRef(l.TargetRef)
				ref := core.CiteRef(sec.Path, s+shift, e+shift)
				text, _ := core.LinesText(doc.Content, s+shift, e+shift)
				cs.links = append(cs.links, core.ClaimLink{ClaimID: cl.ClaimID, Rel: core.RelCites, TargetRef: ref, TargetHash: l.TargetHash, AnchorText: text})
				if oldCite == "" {
					oldCite, newCite = l.TargetRef, ref
				}
			case core.RelAnchors:
				hadAnchor = true // re-resolved against the new cite below
			case core.RelSupersedes:
				// the departed claim's own history; the carried claim
				// supersedes the departed claim instead (below)
			default:
				l.ClaimID = cl.ClaimID
				cs.links = append(cs.links, l)
			}
		}
		if hadAnchor && p.anchorSymbol != nil && newCite != "" {
			if link, ok := p.anchorSymbol(cl.ClaimID, newCite, fetch); ok {
				cs.links = append(cs.links, link)
			}
		}
		cs.links = append(cs.links, core.ClaimLink{
			ClaimID:    cl.ClaimID,
			Rel:        core.RelSupersedes,
			TargetRef:  old.ClaimID,
			AnchorText: fmt.Sprintf("AG-26 carried: %s -> %s (content digest and prompt identity unchanged, no model call)", oldCite, newCite),
		})
	}
	return cs, true, nil
}

// citationShift finds the one line shift that re-anchors every departed
// citation inside sec: for each cite, the shifts that land it on lines of
// sec whose hash matches the cite's own, intersected across all cites. It
// reports ok=false when a claim in claimIDs has no citation, a citation does
// not parse, or the intersection is empty or holds more than one shift (a
// repeated line the cites cannot tell apart).
func citationShift(content []byte, sec Section, claimIDs []string, links []core.ClaimLink) (int, bool) {
	cited := map[string]bool{}
	var candidates map[int]bool
	for _, l := range links {
		if l.Rel != core.RelCites {
			continue
		}
		cited[l.ClaimID] = true
		_, s, e, ok := core.ParseCiteRef(l.TargetRef)
		if !ok || l.TargetHash == "" {
			return 0, false
		}
		span := e - s
		here := map[int]bool{}
		for start := sec.StartLine; start+span <= sec.EndLine; start++ {
			if h, ok := core.HashLines(content, start, start+span); ok && h == l.TargetHash {
				if candidates == nil || candidates[start-s] {
					here[start-s] = true
				}
			}
		}
		candidates = here
		if len(candidates) == 0 {
			return 0, false
		}
	}
	if len(candidates) != 1 {
		return 0, false
	}
	for _, id := range claimIDs {
		if !cited[id] {
			return 0, false // a claim with no citation cannot be re-cited
		}
	}
	for shift := range candidates {
		return shift, true
	}
	return 0, false
}

// subjectPath is the document path of a section subject ("<path>#<slug>").
func subjectPath(subject string) string {
	i := strings.LastIndex(subject, "#")
	if i <= 0 {
		return ""
	}
	return subject[:i]
}
