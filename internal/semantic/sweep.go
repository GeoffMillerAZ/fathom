package semantic

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// SweepReport summarises one citation sweep.
type SweepReport struct {
	ClaimsChecked int
	Fresh         int
	Stale         int
	Contradicted  int
	Flipped       int // status changes written this sweep

	// DriftDetected counts served claims where at least one citation's hash
	// no longer matched its stored TargetHash this sweep — T5-1/O13's
	// "wrong citations found", regardless of whether the drift was healed by
	// re-anchoring or left stale. A superset of ReAnchored: every re-anchored
	// claim also had drift detected; a claim can also drift and NOT heal
	// (ambiguous/absent anchor match, or no stored anchor text at all).
	DriftDetected int
	// ReAnchored counts served claims healed this sweep by re-anchoring: a
	// drifted citation's stored AnchorText resolved to exactly one new
	// location in the same file, so the sweep rewrote the citation
	// (store.UpdateClaimLink) and appended a ClaimEvent{Verdict:
	// "re-anchored"} instead of flipping the claim stale. Claims with no
	// stored AnchorText (legacy hash-only links, pre-T5-1) are never counted
	// here — they can only ever go stale on drift, never heal.
	ReAnchored int
}

// Sweep is the token-free post-ingest freshness pass (ADR-016 §4.3): for
// every served claim of the service, re-hash its cited line ranges against
// the current corpus and flip status on drift. fetch returns a corpus file's
// current content by repo-relative path (ok=false when the file is gone).
//
// Status semantics: all citations hash-match → fresh (a stale claim heals if
// the doc is restored, or if a drifted citation RE-ANCHORS — see below); any
// remaining mismatch → stale; any citation that no longer resolves (file
// gone, range out of bounds) → contradicted (dominant). Superseded/refuted
// claims are never touched — those are terminal verdicts, not freshness
// states.
//
// T5-1/O13 (docs/dogfood-observations.md, docs/plans/trust-and-tokenomics.md):
// a hash-only citation can only ever detect drift, never distinguish "the
// rule's text moved" from "the rule's text was rewritten or deleted" — an
// upstream insertion above the cited lines shifts the rule to a new line
// range while different (or no) text lands at the OLD cited range, and a
// pure hash check has no way back to the truth. When a citation's hash no
// longer matches AND the link carries AnchorText (the literal cited text,
// stored at compile time — empty for links written before this field
// existed, "legacy hash-only" claims), the sweep searches the CITED FILE for
// that exact text via findAnchorText:
//   - exactly one match → RE-ANCHOR: store.UpdateClaimLink rewrites the
//     citation to the new line range and hash, and a
//     ClaimEvent{Verdict:"re-anchored"} records the old→new move (mirrors
//     ADR-017's source-ref remap) — the claim reads FRESH again, never
//     silently wrong.
//   - zero or more-than-one match (ambiguous, indistinguishable from a real
//     rewrite) → falls through to the stale path exactly as before T5-1.
//
// A link with no AnchorText skips the re-anchor attempt entirely and goes
// straight to stale on drift — it can heal only the old way (the doc
// reverting to its original text at the original range).
//
// P1-9 vocabulary (docs/plans/pulled-tools-excellence.md; see also
// internal/semantic/serve.go and docs/design/semantic-layers.md): "contradicted"
// and "refuted" name two DIFFERENT things and must never share a verdict
// string. Contradicted is this function's own machine-detected, anchor-dead
// state — a claim whose citation no longer resolves; it is served WITH the
// status label (Served() includes ClaimContradicted) so a caller sees the
// flagged answer, never silence, pending a recompile that either heals it or
// confirms it. Refuted (core.ClaimRefuted) is a deliberate human — or
// human-equivalent — judgment via `fathom claim refute` / the obligations
// tool's feedback action (D2-1): a terminal verdict that is NEVER served
// again. Sweep only ever produces the former.
//
// D2-2 (docs/plans/expert-depth.md): a transition INTO contradicted — an
// anchor-dead claim, the strongest drift signal short of a human refutation —
// also appends a ClaimEvent{Verdict:"contradicted", Author:"sweep"} beside
// the status flip, so the automated judgment leaves the same append-only
// audit trail a human `fathom claim refute` / the obligations tool's feedback
// action (D2-1) would leave for its own "refuted" verdict — same mechanism,
// distinct vocabulary, because they are distinct judgments. Only fired on the
// transition (next != cl.Status), not on every sweep of an already-
// contradicted claim — durability needs one event per verdict, not unbounded
// re-affirmation noise. now is the injected clock (repo convention, mirrors
// Compiler.Now); nil uses time.Now.
func Sweep(ctx context.Context, st store.Store, serviceID string, fetch func(path string) ([]byte, bool), now func() time.Time) (SweepReport, error) {
	clock := time.Now
	if now != nil {
		clock = now
	}
	var rep SweepReport
	claims, err := st.ClaimsForService(ctx, serviceID, "")
	if err != nil {
		return rep, fmt.Errorf("semantic: sweep load claims: %w", err)
	}
	served := claims[:0]
	ids := make([]string, 0, len(claims))
	for _, cl := range claims {
		if cl.Status.Served() {
			served = append(served, cl)
			ids = append(ids, cl.ClaimID)
		}
	}
	if len(served) == 0 {
		return rep, nil
	}
	links, err := st.ClaimLinks(ctx, serviceID, ids)
	if err != nil {
		return rep, fmt.Errorf("semantic: sweep load links: %w", err)
	}
	citesByClaim := map[string][]core.ClaimLink{}
	// anchorsByClaim (SU-21, ADR-041 decision 7) is the "anchors" link each
	// claim carries, keyed by ClaimID — at most one per claim (AnchorSymbol
	// resolves at most one enclosing symbol per cites span, and every
	// compiled claim has at most one cites link today). Used only as a
	// FALLBACK when the AnchorText-based re-anchor below is unavailable or
	// fails.
	anchorsByClaim := map[string]core.ClaimLink{}
	for _, l := range links {
		switch l.Rel {
		case core.RelCites:
			citesByClaim[l.ClaimID] = append(citesByClaim[l.ClaimID], l)
		case core.RelAnchors:
			anchorsByClaim[l.ClaimID] = l
		}
	}

	// symbolIdx (SU-21) is the service's symbol index, loaded at most ONCE
	// for this whole sweep and only on first actual use — most claims carry
	// no "anchors" link at all (a governing-doc citation resolves to zero
	// symbols), so a sweep with nothing to symbol-re-anchor never pays for
	// the load.
	var (
		symbolIdxLoaded bool
		symbolIdx       SymbolIndex
	)
	loadSymbolIdx := func() SymbolIndex {
		if !symbolIdxLoaded {
			symbolIdxLoaded = true
			symbolIdx, _ = LoadSymbolIndex(ctx, st, serviceID)
		}
		return symbolIdx
	}

	// Deterministic order: by claim ID.
	sort.Slice(served, func(i, j int) bool { return served[i].ClaimID < served[j].ClaimID })

	content := map[string][]byte{} // per-file cache
	missing := map[string]bool{}
	for _, cl := range served {
		rep.ClaimsChecked++
		next := core.ClaimFresh
		deadCite := "" // the citation that killed the anchor, for the contradicted-event note
		claimDrifted := false
		claimReanchored := false
		for _, cite := range citesByClaim[cl.ClaimID] {
			path, start, end, ok := core.ParseCiteRef(cite.TargetRef)
			if !ok {
				next = core.ClaimContradicted
				deadCite = cite.TargetRef
				break
			}
			data, have := content[path]
			if !have && !missing[path] {
				if d, found := fetch(path); found {
					content[path] = d
					data = d
				} else {
					missing[path] = true
				}
			}
			if missing[path] {
				next = core.ClaimContradicted
				deadCite = cite.TargetRef
				break
			}
			hash, ok := core.HashLines(data, start, end)
			if !ok {
				next = core.ClaimContradicted
				deadCite = cite.TargetRef
				break
			}
			if hash == cite.TargetHash {
				continue // this citation still resolves correctly
			}
			claimDrifted = true
			// T5-1/O13: try to re-anchor before giving up to stale — only
			// possible when this link carries the compile-time anchor text
			// (legacy hash-only links cannot heal, by construction).
			if cite.AnchorText != "" {
				if newStart, newEnd, unambiguous := findAnchorText(data, cite.AnchorText); unambiguous {
					newHash, hok := core.HashLines(data, newStart, newEnd)
					if hok {
						newRef := core.CiteRef(path, newStart, newEnd)
						if err := st.UpdateClaimLink(ctx, serviceID, cl.ClaimID, core.RelCites, cite.TargetRef, newRef, newHash); err != nil {
							return rep, fmt.Errorf("semantic: sweep reanchor %s: %w", cl.ClaimID, err)
						}
						ev := core.ClaimEvent{
							ClaimID: cl.ClaimID,
							Verdict: "re-anchored",
							Author:  "sweep",
							Note:    fmt.Sprintf("%s -> %s (citation text unchanged, line range shifted)", cite.TargetRef, newRef),
							At:      clock(),
						}
						if err := st.AppendClaimEvent(ctx, ev); err != nil {
							return rep, fmt.Errorf("semantic: sweep reanchor event %s: %w", cl.ClaimID, err)
						}
						claimReanchored = true
						continue // healed: this citation no longer counts against next
					}
				}
			}
			// SU-21 (ADR-041 decision 7): the AnchorText path above either
			// had nothing to search with (legacy hash-only link) or found
			// zero/multiple matches (ambiguous — ADR-017's pre-T5-1
			// fallthrough). When this claim's cites span resolved to
			// exactly one symbol at mint (an "anchors" link), check
			// whether that SAME symbol (by SymbolKey — identity, not
			// position) still exists in the current index: a rename or an
			// edit that changes the symbol's own body would drop it, but a
			// move that only shifts its line range does not. Re-anchors
			// the cites link to the symbol's CURRENT range instead of
			// flipping stale — same-file only (a cross-file move is not
			// attempted here: this file's already-loaded `data` would not
			// match a different file's content, and re-reading a second
			// file per candidate is out of this row's scope).
			if link, ok := anchorsByClaim[cl.ClaimID]; ok {
				if sym, symOK := loadSymbolIdx().ByKey(link.TargetRef); symOK && sym.FilePath == path {
					// The hash comparison against the STORED anchors
					// TargetHash (never just re-hashing current content) is
					// load-bearing: it is what proves the symbol's own body
					// is byte-identical to mint time and only its line
					// range moved — without it this would blindly
					// "heal" a genuine content edit inside the symbol's
					// current range instead of correctly flagging it stale.
					if newHash, hok := core.HashLines(data, sym.Line, sym.EndLine); hok && link.TargetHash != "" && newHash == link.TargetHash {
						newRef := core.CiteRef(path, sym.Line, sym.EndLine)
						if err := st.UpdateClaimLink(ctx, serviceID, cl.ClaimID, core.RelCites, cite.TargetRef, newRef, newHash); err != nil {
							return rep, fmt.Errorf("semantic: sweep reanchor-symbol %s: %w", cl.ClaimID, err)
						}
						ev := core.ClaimEvent{
							ClaimID: cl.ClaimID,
							Verdict: "re-anchored",
							Author:  "sweep",
							Note:    fmt.Sprintf("%s -> %s (anchored symbol %s still resolves)", cite.TargetRef, newRef, sym.Key()),
							At:      clock(),
						}
						if err := st.AppendClaimEvent(ctx, ev); err != nil {
							return rep, fmt.Errorf("semantic: sweep reanchor-symbol event %s: %w", cl.ClaimID, err)
						}
						claimReanchored = true
						continue // healed: this citation no longer counts against next
					}
				}
			}
			next = core.ClaimStale // keep scanning: a later broken cite still dominates
		}
		if claimDrifted {
			rep.DriftDetected++
		}
		if claimReanchored {
			rep.ReAnchored++
		}
		switch next {
		case core.ClaimFresh:
			rep.Fresh++
		case core.ClaimStale:
			rep.Stale++
		case core.ClaimContradicted:
			rep.Contradicted++
		}
		if next != cl.Status {
			if err := st.SetClaimStatus(ctx, serviceID, cl.ClaimID, next); err != nil {
				return rep, fmt.Errorf("semantic: sweep flip %s: %w", cl.ClaimID, err)
			}
			rep.Flipped++
			if next == core.ClaimContradicted {
				note := "citation anchor no longer resolves"
				if deadCite != "" {
					note += ": " + deadCite
				}
				ev := core.ClaimEvent{
					ClaimID: cl.ClaimID,
					Verdict: "contradicted",
					Author:  "sweep",
					Note:    note,
					At:      clock(),
				}
				if err := st.AppendClaimEvent(ctx, ev); err != nil {
					return rep, fmt.Errorf("semantic: sweep claim event %s: %w", cl.ClaimID, err)
				}
			}
		}
	}
	return rep, nil
}

// findAnchorText searches content — normalized the same way HashLines/
// LinesText split lines (CRLF→LF, joined by "\n") — for a single unambiguous
// occurrence of anchor, the exact (possibly multi-line) snippet
// ClaimLink.AnchorText stored at compile time. Returns the 1-based inclusive
// line range of the match. unambiguous is false when anchor resolves to
// zero or more than one location in content: T5-1/O13 re-anchors ONLY on a
// clean, unique match — an ambiguous or absent anchor is indistinguishable
// from a genuine rewrite/deletion and must fall through to the sweep's
// existing stale path rather than guess which occurrence is "the" rule.
func findAnchorText(content []byte, anchor string) (start, end int, unambiguous bool) {
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	anchorLines := strings.Split(anchor, "\n")
	n := len(anchorLines)
	if n == 0 || n > len(lines) {
		return 0, 0, false
	}
	found := -1
	for i := 0; i+n <= len(lines); i++ {
		match := true
		for j := 0; j < n; j++ {
			if lines[i+j] != anchorLines[j] {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		if found >= 0 {
			return 0, 0, false // a second match: ambiguous
		}
		found = i
	}
	if found < 0 {
		return 0, 0, false
	}
	return found + 1, found + n, true
}

// DriftedFindings computes remediation drift for drift-tracked-layer claims
// (FT-6, docs/vision/03-findings-and-experts.md §2): "has the SCOPED CODE —
// not the cited finding document — changed since compile?" This is a second,
// independent pass beside Sweep's citation check, over a different anchor
// (scope-fp links against code fingerprints, not cites links against doc
// content) — and, deliberately, a PURE READ: it never calls SetClaimStatus.
// code_drifted is a SERVING annotation, not a freshness verdict — scoped code
// changing can mean the finding was FIXED, not that the claim is wrong
// (docs/vision/03 §2 is explicit: never a status flip). Recomputing this
// live at serve time (ObligationsFor calls it directly) rather than
// persisting a drifted flag on the claim keeps both Sweep and ObligationsFor
// pure functions of current store state, with no third claim-status axis to
// keep consistent — the simplest design that satisfies the "never a status
// flip" constraint.
//
// Compile time persists each drift-tracked claim's scope-matched files'
// content fingerprints as ClaimLink{Rel: RelScopeFP} (verifyDraft,
// compile.go) — no disk read needed here, only the store's already-
// maintained fingerprint table (the same one the fusion pipeline's hash-gate
// keeps current on every ingest). A scope-fp path with no CURRENT
// fingerprint at all (re-ingest purged it, the file moved, or the service
// was never re-indexed after a corpus move) is treated as drifted: the
// scoped code's current state cannot be verified unchanged, so silence would
// be dishonest.
//
// LC-2 (docs/plans/layer-catalog.md): which claims are in scope is now the
// manifest's serving.drift_tracked field, not a kind=="finding" Go hardcode
// — resolved per distinct claim.Layer via the union loader (LoadLayerIn,
// LC-3: embedded packs ∪ the service's own project-local .fathom/layers/, so
// a project-installed drift-tracked layer works exactly like an embedded
// one), memoized so an N-claim service costs at most one manifest lookup per
// distinct layer, not per claim. A claim whose layer no longer resolves
// (e.g. a project-local pack later uninstalled) is treated as not
// drift-tracked rather than erroring this read-hot serving path.
//
// serviceRoot resolves a service's on-disk root the same way SweepClaims and
// pkg/client.CompileLayer do (service_meta.Path) — the ONE place a
// service's project directory is knowable from just (ctx, st, serviceID).
// Every serve-time drift check funnels through this so LoadLayerIn always
// sees the union its compile-time counterpart saw.
func serviceRoot(ctx context.Context, st store.Store, serviceID string) string {
	if meta, found, err := st.GetServiceMeta(ctx, serviceID); err == nil && found {
		return meta.Path
	}
	return ""
}

// layerIsDriftTracked reports whether the named layer's
// serving.drift_tracked is true, consulting the union of embedded packs and
// root's project-local .fathom/layers/ (LC-2/LC-3). Both DriftedFindings'
// per-claim filter and ObligationsFor's serve.go short-circuit share this
// resolution. An unresolvable layer name (e.g. claims left behind by a
// project-local pack later uninstalled) is treated as not drift-tracked
// rather than erroring this serve-time hot path — DriftedFindings' own doc
// comment explains the same leniency for the per-claim case.
func layerIsDriftTracked(root, name string) bool {
	def, err := LoadLayerIn(root, name)
	return err == nil && def.Serving.DriftTracked
}

func DriftedFindings(ctx context.Context, st store.Store, serviceID string) (map[string]bool, error) {
	claims, err := st.ClaimsForService(ctx, serviceID, "")
	if err != nil {
		return nil, fmt.Errorf("semantic: drift load claims: %w", err)
	}
	root := serviceRoot(ctx, st, serviceID)
	driftTrackedLayer := map[string]bool{}
	var findingIDs []string
	for _, cl := range claims {
		if !cl.Status.Served() {
			continue
		}
		tracked, ok := driftTrackedLayer[cl.Layer]
		if !ok {
			tracked = layerIsDriftTracked(root, cl.Layer)
			driftTrackedLayer[cl.Layer] = tracked
		}
		if tracked {
			findingIDs = append(findingIDs, cl.ClaimID)
		}
	}
	if len(findingIDs) == 0 {
		return nil, nil
	}
	sort.Strings(findingIDs) // deterministic iteration order below

	links, err := st.ClaimLinks(ctx, serviceID, findingIDs)
	if err != nil {
		return nil, fmt.Errorf("semantic: drift load links: %w", err)
	}
	scopeFPByClaim := map[string][]core.ClaimLink{}
	for _, l := range links {
		if l.Rel == core.RelScopeFP {
			scopeFPByClaim[l.ClaimID] = append(scopeFPByClaim[l.ClaimID], l)
		}
	}

	// Per-call fingerprint cache: this runs on EVERY ObligationsFor serve
	// (the edit-hook hot path), and a broad-but-legal scope (e.g. **/*.go)
	// can anchor hundreds of files shared across N claims — each distinct
	// file is queried at most once per call, regardless of how many claims
	// link it. A map, not memoization across calls: DriftedFindings stays a
	// pure read of current store state.
	type fpState struct {
		hash  string
		found bool
	}
	cache := map[string]fpState{}

	drifted := map[string]bool{}
	for _, id := range findingIDs {
		// A claim with ZERO scope-fp links falls through this loop as
		// never-drifted. That is safe by the FT-6 compile invariant: the
		// compiler writes ≥1 scope-fp link for every finding-kind claim —
		// scopes matching no indexed file are rejected at compile time, and
		// a fingerprint read failure aborts the whole compile loudly
		// (errDriftAnchor, compile.go) instead of silently omitting links.
		// The review-verb findings bridge (internal/analysis/review
		// .AttachClaims, C2-1) is the SECOND legitimate writer of
		// drift-tracked-layer claims and holds the same invariant its own
		// way: a finding whose scoped file has no stored fingerprint is not
		// persisted at all, so every bridge-written claim also carries
		// exactly one scope-fp link. Zero links therefore only occurs for
		// claims that went through neither writer (e.g. hand-seeded test
		// data), never as a degraded state of a real compile or review.
		for _, l := range scopeFPByClaim[id] {
			cur, ok := cache[l.TargetRef]
			if !ok {
				hash, found, ferr := st.Fingerprint(ctx, serviceID, l.TargetRef)
				if ferr != nil {
					return nil, fmt.Errorf("semantic: drift fingerprint %s: %w", l.TargetRef, ferr)
				}
				cur = fpState{hash: hash, found: found}
				cache[l.TargetRef] = cur
			}
			if !cur.found || cur.hash != l.TargetHash {
				drifted[id] = true
				break
			}
		}
	}
	return drifted, nil
}

// Layer serving roles (schema.cue #Layer.role, ADR-032 §4 D1/D2).
//
// RoleGoverns names a layer whose claims restate BINDING rules a human
// wrote about the code; RoleClaims names a layer whose claims restate what
// some document CLAIMS about the code. The distinction is not decoration:
// it is what ObligationsFor splits the serving budget on, and O23 is why it
// exists — six plan-row spec-bindings filled the whole budget for one Go
// file, displaced 65 more, and the CLAUDE.md rule the file was actually
// governed by never reached the agent.
const (
	RoleGoverns = "governs"
	RoleClaims  = "claims"
)

// LayerRole reports the named layer's serving role, resolved through the
// SAME embedded ∪ project-local union layerIsDriftTracked uses (LC-3), so a
// project-installed pack's role is honored exactly like an embedded one.
//
// An unresolvable layer name — or a pack whose role field predates this
// schema field — reads RoleGoverns, the schema default: a rule can never
// lose serving priority because its pack was uninstalled or written before
// the field existed. Same fail-open leniency as layerIsDriftTracked, and
// for the same reason: this is a serve-time hot path.
func LayerRole(root, name string) string {
	def, err := LoadLayerIn(root, name)
	if err != nil || def.Role != RoleClaims {
		return RoleGoverns
	}
	return RoleClaims
}
