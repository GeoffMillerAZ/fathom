// Package mcp — review_diff, the program's ONE new MCP tool (C1-1, docs/
// plans/review-verb.md): composes diff impact + obligations/findings +
// architecture conformance + spec coverage into ONE cited verdict over an
// explicit changed-files list, replacing the 8-step manual sequence
// docs/use-cases/change-review.md previously documented tool-by-tool.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/coverage"
	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/conserve"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/projectrules"
	"github.com/geoffmilleraz/fathom/internal/receipt"
	riskreview "github.com/geoffmilleraz/fathom/internal/review"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// conserveBackend is the optional backend capability DimConservation's
// review_diff wiring (ADR-044 §1.6/§2, wave 2) uses to build a real
// internal/conserve.Input — the same optional-capability convention
// certificationsBackend/obligationsBackend already use. *pkg/client.
// Client implements it via its own Store() accessor; a minimal test fake
// that omits it simply leaves the conservation dimension unsupported,
// exactly like an Evidence without CertificationEvidence leaves
// certifications unsupported.
type conserveBackend interface {
	Store() store.Store
}

// storeOf returns backend's store and whether it has one: a backend
// without conserveBackend, or a wrapper whose wrapped backend holds none
// (autoRefreshBackend.Store answers nil), has none.
func storeOf(backend QueryInterface) (store.Store, bool) {
	sb, ok := backend.(conserveBackend)
	if !ok {
		return nil, false
	}
	st := sb.Store()
	return st, st != nil
}

// reviewClaimsBackend is the optional backend capability review_diff's
// findings bridge (C2-1, FT-7's pre-armed trigger) uses to persist review
// findings as content-addressed claims, read back prior verdict status,
// reconcile fixed findings (SetClaimStatus → superseded), and anchor each
// claim's scope-fp drift link (SourceFingerprint). pkg/client.Client
// implements all four as thin store passthroughs. Test fakes may omit it:
// review_diff still returns a full verdict, with findings_persisted:false
// and no claim IDs attached — the composed analysis never depends on this
// capability.
type reviewClaimsBackend interface {
	PutClaims(ctx context.Context, claims []core.SemanticClaim, links []core.ClaimLink) error
	ClaimsForService(ctx context.Context, serviceID, layer string) ([]core.SemanticClaim, error)
	SetClaimStatus(ctx context.Context, serviceID, claimID string, status core.ClaimStatus) error
	SourceFingerprint(ctx context.Context, serviceID, path string) (hash string, found bool, err error)
	// SetClaimValidTo (SU-19, ADR-041 decision 7) closes a superseded
	// claim's bi-temporal validity window at the same moment SetClaimStatus
	// flips it.
	SetClaimValidTo(ctx context.Context, serviceID, claimID string, t time.Time) error
	// WriteSupersedes (SU-20, ADR-041 decision 7) records the explicit
	// successor->predecessor link when this run's reconcile found a
	// genuine successor claim (review.SupersedeSuccessor) — see
	// semantic.WriteSupersedes' own doc comment for the two cases that
	// never call it.
	WriteSupersedes(ctx context.Context, serviceID, successorID, predecessorID, why string) error
}

func registerReviewDiff(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "review_diff",
		Description: "Composed review verdict: diff impact, obligations, findings, architecture, spec/test coverage, certifications, plus integrity/conservation when since is given. Reach for it when reviewing a change. Required: service, changed_files.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":          map[string]any{"type": "string", "description": "Service / repo identifier"},
				"changed_files":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Repo-relative changed file paths"},
				"concern":          map[string]any{"type": "string", "description": "Optional concern token (descriptive only)"},
				"since":            map[string]any{"type": "string", "description": "Base commit ref (ADR-044); feeds integrity + conservation."},
				"as_of":            map[string]any{"type": "string"},
				"coverage_profile": map[string]any{"type": "string", "description": "Alias for one coverage_profiles entry, layer \"unit\"; ignored when coverage_profiles is set."},
				"coverage_profiles": map[string]any{
					"type":        "array",
					"description": "One entry per test layer: {layer, path}. Enables per-layer coverage; omit for no coverage dimension.",
					"items": map[string]any{
						"type":       "object",
						"properties": map[string]any{"layer": map[string]any{"type": "string"}, "path": map[string]any{"type": "string"}},
						"required":   []string{"layer", "path"},
					},
				},
			},
			"required": []string{"service", "changed_files"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service      string   `json:"service"`
			ChangedFiles []string `json:"changed_files"`
			Concern      string   `json:"concern"`
			Since        string   `json:"since"`
			AsOf         string   `json:"as_of"`
			// CoverageProfile is the single-profile alias; CoverageProfiles
			// is the real multi-layer form. CoverageProfiles wins when both
			// are given.
			CoverageProfile  string `json:"coverage_profile"`
			CoverageProfiles []struct {
				Layer string `json:"layer"`
				Path  string `json:"path"`
			} `json:"coverage_profiles"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "review_diff: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "review_diff: service required"}
		}
		if len(p.ChangedFiles) == 0 {
			return nil, &rpcError{Code: ErrCodeParams, Message: "review_diff: changed_files required"}
		}

		asOf, aerr := asOfFilter(ctx, backend, p.Service, p.AsOf, "review_diff")
		if aerr != nil {
			return nil, aerr
		}
		if asOf.active() {
			if _, ok := backend.(obligationsAsOfBackend); !ok {
				return nil, &rpcError{Code: ErrCodeInvalid, Message: "review_diff: backend does not support as_of"}
			}
		}
		in, sources, rpcErr := buildReviewInput(ctx, backend, p.Service, p.ChangedFiles, p.Concern, p.Since, asOf, s.reviewNow)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var labeledPaths []labeledProfilePath
		switch {
		case len(p.CoverageProfiles) > 0:
			for _, cp := range p.CoverageProfiles {
				layer := cp.Layer
				if layer == "" {
					layer = defaultCoverageProfileLayer
				}
				labeledPaths = append(labeledPaths, labeledProfilePath{Layer: layer, Path: cp.Path})
			}
		case p.CoverageProfile != "":
			labeledPaths = []labeledProfilePath{{Layer: defaultCoverageProfileLayer, Path: p.CoverageProfile}}
		}
		if len(labeledPaths) > 0 {
			if rpcErr := applyCoverageProfiles(ctx, backend, p.Service, labeledPaths, &in); rpcErr != nil {
				return nil, rpcErr
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, navLoadError("review_diff", p.Service, err)
		}
		v := review.Run(in)
		if len(sources.gaps) > 0 {
			for i := range v.Dimensions {
				if v.Dimensions[i].Name == review.DimSpecCoverage {
					v.Dimensions[i].Checked = false
					v.Dimensions[i].Reason = "changed source unreadable or over review budget: " + strings.Join(sources.gaps, ", ")
				}
			}
		}
		if len(sources.gaps) > 0 {
			v.Summary = summarizeReviewVerdict(v)
		}
		if err := ctx.Err(); err != nil {
			return nil, navLoadError("review_diff", p.Service, err)
		}
		// GL-17: a dated review is a read of the past — it never writes the
		// findings claims or supersedes the predecessors a live run
		// reconciles, so findings_persisted is honestly false.
		persisted, resolutions := false, map[string]string(nil)
		if !asOf.active() {
			persisted, resolutions = applyAndPersistFindings(ctx, backend, &v, &sources)
		}
		if err := ctx.Err(); err != nil {
			return nil, navLoadError("review_diff", p.Service, err)
		}
		if sources.windowUnverified {
			for i := range v.Dimensions {
				if v.Dimensions[i].Name == review.DimSpecCoverage {
					v.Dimensions[i].Checked = false
					v.Dimensions[i].Reason = strings.TrimPrefix(sources.partial, "partial: ")
				}
			}
			v.Summary = summarizeReviewVerdict(v)
		}

		payload := v.Payload(persisted)
		if len(sources.excluded) > 0 {
			excluded := make([]reviewSourceExclusion, 0, len(sources.excluded))
			for path, reason := range sources.excluded {
				excluded = append(excluded, reviewSourceExclusion{Path: path, Reason: reason})
			}
			sort.Slice(excluded, func(i, j int) bool { return excluded[i].Path < excluded[j].Path })
			payload["excluded"] = excluded
		}
		if sources.typedNote != "" {
			payload["typed_edges_note"] = sources.typedNote
		}
		if asOf.active() {
			payload["as_of"] = asOf.label()
			payload["as_of_scope"] = "obligations and certifications are answered as of " + asOf.label() +
				"; impact, architecture, spec, test and integrity dimensions read the code and index as they stand now; nothing was persisted"
		}
		if note := sources.partialNote(); note != "" {
			payload["corpus_partial"] = note
		}
		if vo := reviewVerdictOutcome(v, resolutions); vo != nil {
			payload["verdict_outcome"] = vo
		}
		if err := ctx.Err(); err != nil {
			return nil, navLoadError("review_diff", p.Service, err)
		}
		gap := reviewTestGap(ctx, backend, in)
		payload["test_impact"] = gap.Payload()
		markReviewLocations(ctx, backend, p.Service, &v, payload, &gap)

		// resolution/index_commit (VB-19, docs/vision/29-verbs-not-lookups.md
		// §3 wave E): the typed/heuristic mix behind the impact dimension's
		// blast radius (the edge-derived facts review_diff composes) — the
		// same by_depth tally blast_radius itself reports, over the SAME
		// nodes/changed_files/typed edges the impact dimension already
		// resolved — plus the daemon's watermark for the service.
		// Governance dimensions (obligations, arch_conformance, spec
		// coverage, test coverage) are not edge-derived and are outside
		// this tally.
		if err := ctx.Err(); err != nil {
			return nil, navLoadError("review_diff", p.Service, err)
		}
		ng, gerr := loadNavGraphsErr(ctx, backend, p.Service)
		if gerr == nil {
			payload["resolution"] = resolutionFromBlastBuckets(blastRadiusByDepth(ng.Both, ng.Nodes, in.ChangedFiles))
		} else {
			payload["partial"] = navLoadError("review_diff", p.Service, gerr).Message
		}
		if err := ctx.Err(); err != nil {
			return nil, navLoadError("review_diff", p.Service, err)
		}
		if ic := indexCommitFor(ctx, backend, p.Service); ic != "" {
			payload["index_commit"] = ic
		}
		// VB-30: other services whose outbound HTTP client call joins a
		// route one of changed_files registers (serviceRefHopsInto,
		// shared with blast_radius). Additive:
		// every existing payload field is unchanged (law 4, same as
		// test_impact above).
		if err := ctx.Err(); err != nil {
			return nil, navLoadError("review_diff", p.Service, err)
		}
		hops, unextracted := serviceRefHopsInto(ctx, backend, p.Service, p.ChangedFiles)
		if err := ctx.Err(); err != nil {
			return nil, navLoadError("review_diff", p.Service, err)
		}
		withServiceRefHops(payload, hops, unextracted)
		// GL-22: the changed files layered by blast radius (leaf, shared
		// callee, cross-service provider) from the graph and hops above.
		if gerr == nil {
			payload["layers"] = layerDiffByBlastRadius(ng.Both, ng.Nodes, in.ChangedFiles, hops)
			// CX-5 (docs/plans/change-and-compliance.md §2): the change risk
			// receipt, additive, over the SAME graph/hops/verdict this call
			// already resolved — review_receipt.go's own doc comment names
			// exactly what is and is not wired yet. The risk table is the
			// repo's own `.fathom/risk.cue` when present (LoadRiskTable's
			// dual-mode load), the binary default otherwise — never the
			// daemon process's cwd (only read under in.Root, the service's
			// own resolved corpus root; "" skips the read, the same
			// corpusRoot-gated discipline every other read in this file
			// already keeps).
			table := riskreview.RiskTableFor(in.Root)
			risk := reviewDiffReceipt(ng.Both, ng.Nodes, in, v, hops, table)
			riskPayload := riskReceiptPayload(risk)
			for _, hint := range riskPayload["link_hints"].([]*reviewLinkLocation) {
				if file := citationPath(hint.Citation); file != "" {
					bindCitation(ctx, backend, p.Service, file, func(status *core.CitationStatus) { hint.CitationStatus = status })
				}
			}
			payload["receipt"] = riskPayload
			// SR-49 (docs/plans/surface-reckoning.md): the SAME receipt,
			// additionally rendered as SARIF 2.1.0 and a GitHub check-run
			// payload through review.ChangeRiskReceipt — embedded as raw
			// JSON (json.RawMessage) rather than an escaped string, so the
			// tool's overall JSON payload carries them as nested objects.
			// EVX-3: the SARIF run carries its provenance, anchored at the
			// indexed commit under the service root against the store's
			// fingerprints (receipt_sarif's content grows; its key does not).
			if err := ctx.Err(); err != nil {
				return nil, navLoadError("review_diff", p.Service, err)
			}
			anchorRun := receipt.Run{Dir: in.Root, Rev: indexCommitFor(ctx, backend, p.Service), Service: p.Service, Nodes: ng.Nodes,
				RepositoryURIAt: receipt.ConfiguredRepositoryURI}
			if rcb, ok := backend.(reviewClaimsBackend); ok {
				anchorRun.Stored = func(path string) (string, bool) {
					if ctx.Err() != nil {
						return "", false
					}
					hash, found, ferr := rcb.SourceFingerprint(ctx, p.Service, path)
					return hash, ferr == nil && found
				}
			}
			receiptSARIF, receiptCheck := review.ChangeRiskReceipt(ctx, "fathom review_diff", anchorRun, risk)
			payload["receipt_sarif"] = json.RawMessage(receiptSARIF)
			payload["receipt_check"] = json.RawMessage(receiptCheck)
		}
		if err := ctx.Err(); err != nil {
			return nil, navLoadError("review_diff", p.Service, err)
		}
		return payload, nil
	})
}

// reviewTestGap composes review_diff's `test_impact` block (BD-4, docs/
// plans/blast-depth.md) from the same inputs the impact dimension already
// resolved — nodes, typed edges, changed files — via impact.TestGap: the
// Test* functions that statically reach a changed symbol (affected_tests,
// with file:line citations and which symbols each reaches) and the changed
// symbols no test reaches (test_gap). It is the test_impact tool's own
// graph walk (callgraph.TestImpactDetail) run once per changed symbol, so
// review_diff names the tests to run and the unprotected code in the same
// verdict instead of sending the agent back for N test_impact calls.
// Additive: every existing payload field is unchanged (law 4).
//
// The walk runs on the shared syntax+typed graph (RT-74) — the same
// graph BuildBoth(in.Nodes, in.TypedEdges) would build, once per (service,
// watermark) instead of once per review.
func reviewTestGap(ctx context.Context, backend QueryInterface, in review.Input) impact.TestGapResult {
	snap, err := serviceSnapshot(ctx, backend, in.Service)
	if err != nil {
		return impact.TestGapOn(nil, nil, in.ChangedFiles)
	}
	// A typed-edge read failure degrades this review to the syntax graph
	// and is never cached (GraphForTyped).
	g, _, _, err := memo.GraphForTyped(ctx, snap, memo.GraphBoth, typedLoader(backend, in.Service))
	if err != nil {
		return impact.TestGapOn(nil, nil, in.ChangedFiles)
	}
	return impact.TestGapOn(g, snap.Nodes, in.ChangedFiles)
}

// reviewVerdictOutcome computes review_diff's gate-shaped result summary
// (IT-7, docs/plans/instrument-truth.md), mirroring verify_plan's own
// verdict_outcome pattern (verify_plan_tool.go's verdictOutcome) so the
// call logger's existing extraction (calllog.go's verdictOutcomeFrom) picks
// review_diff calls up with zero logger changes:
//
//   - active: the sum of Count (active, non-refuted findings) across every
//     CHECKED dimension — exactly Verdict.Summary's own "active finding(s)"
//     figure.
//   - fixed / superseded: this run's reconciled claims (resolutions, from
//     applyAndPersistFindings — a supersededClaimID→ResolutionKind map)
//     split by review.ClassifyResolution's two outcomes: fixed is a
//     code-change resolution (the scoped file still exists — the issue was
//     actually fixed in place); superseded is a scope-disappearance
//     resolution (the scoped file is gone — reconciled, but not provably
//     fixed).
//
// Returns nil when no dimension ran at all: nothing gate-relevant happened
// in this call, so there is nothing to log (mirrors verdictOutcome's own
// >0-claimed opt-in gate for verify_plan).
func reviewVerdictOutcome(v review.Verdict, resolutions map[string]string) map[string]any {
	checked := false
	active := 0
	for _, d := range v.Dimensions {
		if d.Checked {
			checked = true
			active += d.Count
		}
	}
	if !checked {
		return nil
	}
	fixed, superseded := 0, 0
	for _, kind := range resolutions {
		switch kind {
		case core.ResolutionCodeChange:
			fixed++
		case core.ResolutionScopeDisappearance:
			superseded++
		}
	}
	return map[string]any{"active": active, "fixed": fixed, "superseded": superseded}
}

// reviewDiffWithBase fills in.Root unconditionally, and in.Base by
// resolving since (a git ref) against root via `git -C root rev-parse
// <since>` — CV-13 (ADR-044 §1.6), the review_diff/cmd/fathom-review
// counterpart to cmd/fathom verify --since's sinceRefPlan (verify never
// shells the daemon's own cwd: root is always the SERVICE's own indexed
// corpus root, resolved by the caller via ServiceMeta).
//
// since == "" or root == "" is a complete no-op for Base (Root is still
// set from root, empty or not) — nothing to resolve. A ref that does not
// resolve (bad ref, root not a git work tree, git unavailable) fails OPEN:
// Base stays "" exactly as if since had not been given, and no error is
// returned — every other review_diff dimension must still run regardless
// of whether the base-aware ones can.
func reviewDiffWithBase(ctx context.Context, in *review.Input, root, since string) {
	in.Root = root
	if since == "" || root == "" {
		return
	}
	out, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", since).Output()
	if err != nil {
		return
	}
	in.Base = strings.TrimSpace(string(out))
}

// applyConservationDimension fills in.ConservationSupported/
// ConservationPlanes when in.Base is set AND backend carries the optional
// conserveBackend capability — a no-op otherwise (DimConservation's own
// "needs a base ref"/"conservation input could not be built" gates cover
// both cases). Builds a real internal/conserve.Input via conserve.NewInput
// against corpusRoot (the service's own indexed root, resolved by the
// caller via ServiceMeta — never the daemon process's cwd) and runs it
// through conserve.Run(ctx, in, conserve.Planes()), the exact composition
// `fathom conserve`/`fathom review --since` already use (cmd/fathom/
// reviewconservation.go's own twin of this function). Fails open: an
// error building the conserve.Input (an unreadable corpus, since Base is
// by construction already resolvable) leaves ConservationSupported false
// rather than failing the whole review_diff call.
func applyConservationDimension(ctx context.Context, in *review.Input, backend QueryInterface, corpusRoot, service string) {
	if in == nil || in.Base == "" || corpusRoot == "" {
		return
	}
	st, ok := storeOf(backend)
	if !ok {
		return
	}
	cin, err := conserve.NewInput(ctx, corpusRoot, in.Base, st, service, discoverConservePlanPaths(corpusRoot))
	if err != nil || ctx.Err() != nil {
		return
	}
	// CV-65/design §3.4: the conservation dimension carries the SAME
	// reading set `fathom conserve` would report for this range, execution
	// evidence included — previously always static reach here, since this
	// caller never loaded a coverage profile of its own.
	conserve.LoadConfiguredCoverageContext(ctx, &cin, corpusRoot, "")
	if ctx.Err() != nil {
		return
	}
	in.ConservationSupported = true
	rep := conserve.Run(ctx, cin, conserve.Planes())
	in.ConservationPlanes = conservationPlaneSummaries(rep.Planes)
	if ctx.Err() != nil {
		return
	}
	in.ConservationResidue = conservationResidueSummary(conserve.Residue(ctx, cin, conserve.DefaultResidueMax))
}

// conservationPlaneSummaries flattens internal/conserve.PlaneReport into
// review.ConservationPlaneSummary 1:1 — cmd/fathom/reviewconservation.go
// carries its own identical copy (review.Input's own doc comment on
// ConservationPlanes names this exact conversion; review never imports
// internal/conserve, so both callers do it themselves, the same
// intentional duplication CertificationRecord's two builders already
// have).
func conservationPlaneSummaries(planes []conserve.PlaneReport) []review.ConservationPlaneSummary {
	out := make([]review.ConservationPlaneSummary, 0, len(planes))
	for _, p := range planes {
		findings := make([]review.ConservationFinding, 0, len(p.Findings))
		for _, f := range p.Findings {
			findings = append(findings, review.ConservationFinding{
				Plane: f.Plane, Kind: f.Kind, Key: f.Key, Detail: f.Detail,
				Base: f.Base, Head: f.Head, Declared: f.Declared, Remedy: f.Remedy,
			})
		}
		out = append(out, review.ConservationPlaneSummary{
			Plane: p.Plane, Verdict: p.Verdict, Missing: p.Missing, Findings: findings,
		})
	}
	return out
}

// conservationResidueSummary flattens internal/conserve.ResidueReport into
// review.ConservationResidueSummary 1:1 (CV-65, design §3.4) —
// cmd/fathom/reviewconservation.go carries its own identical copy
// (conservationPlaneSummaries' own precedent just above).
func conservationResidueSummary(rep conserve.ResidueReport) review.ConservationResidueSummary {
	items := make([]review.ConservationResidueItem, 0, len(rep.Items))
	for _, it := range rep.Items {
		items = append(items, review.ConservationResidueItem{
			Symbol: it.Symbol, File: it.File, Line: it.Line, Change: it.Change,
			Class: it.Class, Tests: it.Tests, Callers: it.Callers,
			Evidence: it.Evidence, UnexecutedLines: it.UnexecutedLines,
		})
	}
	return review.ConservationResidueSummary{
		Changed: rep.Changed, Attested: rep.Attested, Unattested: rep.Unattested, Unreached: rep.Unreached,
		Partial: rep.Partial, Unexecuted: rep.Unexecuted, UnanalyzedFunctions: rep.UnanalyzedFunctions,
		Items: items, Truncated: rep.Truncated, Unanalyzed: rep.Unanalyzed, Missing: rep.Missing,
		CoverageUnusable: rep.CoverageUnusable,
		CoverageUnmapped: rep.CoverageUnmapped, CoverageUnmappedSample: rep.CoverageUnmappedSample,
	}
}

// discoverConservePlanPaths returns every docs/plans/*.md at root,
// non-recursive — cmd/fathom/conserve.go's discoverPlanPaths' own twin
// (docs/plans/README.md: "both the pre-push verify command and CI walk
// only the top level of docs/plans/"). A glob error or an absent
// directory yields an empty slice, never a failure: --since still runs,
// just with no prior declarations to excuse anything.
func discoverConservePlanPaths(root string) []string {
	matches, err := filepath.Glob(filepath.Join(root, "docs", "plans", "*.md"))
	if err != nil {
		return nil
	}
	sort.Strings(matches)
	return matches
}

// buildReviewInput does every I/O review.Run itself never performs:
// ListAllSymbols/TypedEdges for impact, per-changed-file Obligations for the
// obligations dimension, ServiceMeta + internal/projectrules.ResolveRules
// for arch_conformance (review_diff never accepts inline arch rules — it
// always resolves through the service's own project rule pack, or reports
// checked:false), and the changed files' own source content for
// spec_coverage. A complete path inventory supports reconciliation, while
// bounded named reads supply changed text and any persisted citation text.
// reviewCorpus separates complete path membership from bounded source text.
// Paths settle deletion; only changed and cited files need content.
type reviewSourceExclusion struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type reviewCorpus struct {
	files                  map[string][]byte
	fileMeta               map[string]core.SourceFile
	paths                  map[string]bool
	pathsComplete          bool
	gaps                   []string
	excluded               map[string]string
	partial                string
	used                   int64
	deadline               time.Time
	windowUnverified       bool
	now                    func() time.Time
	legacy                 bool
	indexScope             memo.Scope
	indexCommit            string
	indexFound, indexKnown bool
	typedNote              string
}

func (rc *reviewCorpus) indexCurrent(ctx context.Context, backend QueryInterface, service string) bool {
	if err := ctx.Err(); err != nil {
		return rc.refuseWindow("partial: index identity recheck: " + err.Error())
	}
	if rc.indexScope.Watermark != "" && navScopeNoEnsure(ctx, backend, service) != rc.indexScope {
		return rc.refuseWindow("partial: index identity changed during review")
	}
	if rc.indexKnown {
		_, commit, found, err := backend.ServiceMeta(ctx, service)
		if err == nil && (commit != rc.indexCommit || found != rc.indexFound) {
			return rc.refuseWindow("partial: index identity changed during review")
		}
	}
	return ctx.Err() == nil
}

func (rc reviewCorpus) partialNote() string {
	if len(rc.gaps) > 0 {
		gap := "required source unreadable or over review budget: " + strings.Join(rc.gaps, ", ")
		if rc.partial != "" {
			return rc.partial + "; " + gap
		}
		return "partial: " + gap
	}
	return rc.partial
}

func summarizeReviewVerdict(v review.Verdict) string {
	checked, active := 0, 0
	var unchecked []string
	for _, d := range v.Dimensions {
		if d.Checked {
			checked++
			active += d.Count
		} else {
			unchecked = append(unchecked, d.Name)
		}
	}
	s := fmt.Sprintf("service %s: %d changed file(s); %d/%d dimensions checked; %d active finding(s)", v.Service, len(v.ChangedFiles), checked, len(v.Dimensions), active)
	if len(unchecked) > 0 {
		s += "; unchecked: " + strings.Join(unchecked, ", ")
	}
	return s
}

func buildReviewInput(ctx context.Context, backend QueryInterface, service string, changedFiles []string, concern, since string, asOf *asOfWindow, clock ...func() time.Time) (review.Input, reviewCorpus, *rpcError) {
	now := time.Now
	if len(clock) > 0 && clock[0] != nil {
		now = clock[0]
	}
	if err := ctx.Err(); err != nil {
		return review.Input{}, reviewCorpus{}, navLoadError("review_diff", service, err)
	}
	snap, err := serviceSnapshot(ctx, backend, service)
	if err != nil {
		return review.Input{}, reviewCorpus{}, navLoadError("review_diff", service, err)
	}
	typed, typedErr := backend.TypedEdges(ctx, service)

	in := review.Input{
		Service:      service,
		ChangedFiles: changedFiles,
		Concern:      concern,
		Nodes:        snap.Nodes,
		TypedEdges:   typed,
	}

	if err := ctx.Err(); err != nil {
		return review.Input{}, reviewCorpus{}, navLoadError("review_diff", service, err)
	}
	if ob, ok := backend.(obligationsBackend); ok {
		in.ObligationsSupported = true
		in.ObligationsByFile = make(map[string][]review.ObligationRule, len(changedFiles))
		// GL-5: a review verb is served the review-audience rules
		// (BUGBOT.md) too, when the backend can tell them apart.
		serve := ob.Obligations
		if rb, ok := backend.(reviewObligationsBackend); ok {
			serve = rb.ReviewObligations
		}
		if err := ctx.Err(); err != nil {
			return review.Input{}, reviewCorpus{}, navLoadError("review_diff", service, err)
		}
		// GL-17: a dated review asks what governed each file then. The
		// as-of answer is the general obligations read (ObligationsAsOf
		// names no audience split), and review_diff has already refused a
		// backend without it.
		if asOf.active() {
			if aob, ok := backend.(obligationsAsOfBackend); ok {
				serve = func(ctx context.Context, service, file string, max int) (semantic.Obligations, error) {
					return aob.ObligationsAsOf(ctx, service, file, max, asOf.instant())
				}
			}
		}
		for _, f := range changedFiles {
			if err := ctx.Err(); err != nil {
				return review.Input{}, reviewCorpus{}, navLoadError("review_diff", service, err)
			}
			obl, oerr := serve(ctx, service, f, 0)
			if oerr != nil {
				continue
			}
			for _, r := range obl.Rules {
				in.ObligationsByFile[f] = append(in.ObligationsByFile[f], review.ObligationRule{
					File: f, Rule: r.Rule, Src: r.Src, Tier: r.Tier, Kind: r.Kind, Status: r.Status,
				})
			}
		}
	}

	// certifications (vision 33 / ADR-043 stage 1, TC-10) — mirrors the
	// obligations block above exactly, gated on certificationsBackend (the
	// SAME optional-capability interface verify_plan_tool.go's
	// CertificationsByTest already asserts backend against) rather than
	// unconditionally, since QueryInterface implementations here (unlike
	// cmd/fathom's concrete *client.Client) may not serve semantic layers
	// at all.
	if err := ctx.Err(); err != nil {
		return review.Input{}, reviewCorpus{}, navLoadError("review_diff", service, err)
	}
	if cb, ok := backend.(certificationsBackend); ok {
		in.CertificationsSupported = true
		if certClaims, cerr := cb.ClaimsForService(ctx, service, certify.Layer); cerr == nil {
			records := certify.Records(asOf.claims(certClaims))
			in.CertificationsDeclared = len(records) > 0
			if in.CertificationsDeclared {
				changedSet := make(map[string]bool, len(changedFiles))
				for _, f := range changedFiles {
					changedSet[f] = true
				}
				in.CertificationsByFile = make(map[string][]review.CertificationRecord, len(changedFiles))
				for _, r := range records {
					for _, f := range r.Scope {
						if !changedSet[f] {
							continue
						}
						in.CertificationsByFile[f] = append(in.CertificationsByFile[f], review.CertificationRecord{
							Subject: r.Subject, Test: r.Test, Strength: r.Strength, State: r.State,
							Tier: r.Tier, Rule: r.Rule, Status: r.Status, Scope: r.Scope,
						})
					}
				}
			}
		}
	}

	if err := ctx.Err(); err != nil {
		return review.Input{}, reviewCorpus{}, navLoadError("review_diff", service, err)
	}
	corpusRoot, indexCommit, indexFound, metaErr := backend.ServiceMeta(ctx, service)
	if metaErr != nil || !indexFound {
		corpusRoot = ""
	}
	// CV-13 (ADR-044 §1.6): resolve since in the service's OWN indexed
	// root — never the daemon process's cwd — and fill Input.Base/Root.
	if err := ctx.Err(); err != nil {
		return review.Input{}, reviewCorpus{}, navLoadError("review_diff", service, err)
	}
	reviewDiffWithBase(ctx, &in, corpusRoot, since)
	// DimIntegrity (ADR-044 §1.6, CV-12, gap-closing wave 3 item 1): the
	// "integrity" conserve plane over exactly the base just resolved — a
	// no-op (leaves IntegrityFindings empty) when Base is still "" or
	// backend carries no conserveBackend capability, so DimIntegrity's own
	// "needs a base ref" gate is the only thing a caller with no since ever
	// sees. Must run BEFORE DimConservation is even reached (Run's own
	// dimension order), but the ordering here is incidental — the two are
	// independent computations over the identical base..head range.
	if err := ctx.Err(); err != nil {
		return review.Input{}, reviewCorpus{}, navLoadError("review_diff", service, err)
	}
	applyIntegrityDimension(ctx, &in, backend, corpusRoot, service)
	// DimConservation (ADR-044 §1.6/§2, wave 2): the same base, run
	// through every registered conserve.Plane — a no-op (leaves
	// ConservationSupported false) when Base is still "" or backend
	// carries no conserveBackend capability.
	if err := ctx.Err(); err != nil {
		return review.Input{}, reviewCorpus{}, navLoadError("review_diff", service, err)
	}
	applyConservationDimension(ctx, &in, backend, corpusRoot, service)
	if err := ctx.Err(); err != nil {
		return review.Input{}, reviewCorpus{}, navLoadError("review_diff", service, err)
	}
	rules, source, deny, checked, reason, resolveErr := projectrules.ResolveRules(corpusRoot, nil, false)
	if resolveErr != nil {
		return review.Input{}, reviewCorpus{}, &rpcError{Code: ErrCodeInternal, Message: "review_diff: project rules: " + resolveErr.Error()}
	}
	in.ArchRules = rules
	in.ArchRuleSource = source
	in.ArchDenyByDefault = deny
	in.ArchChecked = checked
	in.ArchReason = reason

	if err := ctx.Err(); err != nil {
		return review.Input{}, reviewCorpus{}, navLoadError("review_diff", service, err)
	}
	rc := readReviewCorpus(ctx, backend, service, changedFiles, now)
	rc.indexScope = snap.Scope
	rc.indexCommit, rc.indexFound, rc.indexKnown = indexCommit, indexFound, metaErr == nil
	if typedErr != nil {
		in.TypedEdges = nil
		rc.typedNote = "typed edge read failed; using syntax graph: " + typedErr.Error()
	}
	for _, f := range changedFiles {
		if data, ok := rc.files[f]; ok {
			in.ChangedSources = append(in.ChangedSources, core.SourceFile{Path: f, Content: data})
		}
	}
	sort.Slice(in.ChangedSources, func(i, j int) bool { return in.ChangedSources[i].Path < in.ChangedSources[j].Path })
	if err := ctx.Err(); err != nil {
		return review.Input{}, reviewCorpus{}, navLoadError("review_diff", service, err)
	}
	return in, rc, nil
}

// defaultCoverageProfileLayer is the layer an unlabeled `coverage_profile`
// alias entry is treated as — the same "unit" default the CLI's bare
// `--profile <path>` (no `layer=` prefix) uses (internal/testlayer.
// Classify's own default for an unlabeled test file); no per-profile
// metadata exists to say otherwise.
const defaultCoverageProfileLayer = "unit"

// labeledProfilePath is one resolved (layer, repo-relative path) pair from
// either coverage_profiles or the coverage_profile alias, before the file
// is opened.
type labeledProfilePath struct {
	Layer string
	Path  string
}

// applyCoverageProfiles resolves review_diff's coverage_profiles (or the
// single-profile coverage_profile alias, both already normalized to
// labeledProfilePath by the caller) and sets in's coverage fields: read +
// parse each Go cover profile, label it by its caller-declared layer, join
// the set against in.Nodes (review.ResolveCoverage — the SAME composition
// the CLI mirror's --profile flags use, so both surfaces resolve
// identically), and enable the "coverage" dimension.
//
// Each path is resolved SAFELY under the service's own corpus root — the
// lexical + physical containment checks read_source already established
// (validateRelPath, resolveRealPath: no absolute path, no traversal, no
// symlink escape) — AND read_source's notIndexedReason checks, EXCEPT its
// plain project-.fathomignore corpus-membership refusal: a coverage
// profile is a build artifact, typically under workspace/, which a repo's
// own .fathomignore routinely excludes from the indexed corpus, so that ONE
// check alone must not refuse the exact files this parameter exists to
// read.
//
// CRITICAL fix (review_tool.go finding 3): before this fix the WHOLE of
// notIndexedReason was skipped, so a coverage_profiles path could name a
// .env-shaped file, a purged/scoped-ignored file, or a suppressed file —
// none of which this carve-out's rationale ever covered — and
// coverage.ParseProfile's own error could echo that file's content back
// through the MCP error channel. notIndexedReasonOpts (readsource.go) now
// applies every refusal EXCEPT the narrow one the comment above justifies:
// .env, always-skipped directories, a scoped-ignore/purge exclusion, and
// suppression all still refuse.
func applyCoverageProfiles(ctx context.Context, backend QueryInterface, service string, paths []labeledProfilePath, in *review.Input) *rpcError {
	corpusRoot, err := resolveCorpusRoot(ctx, backend, service)
	if err != nil {
		return &rpcError{Code: ErrCodeParams, Message: "review_diff: coverage_profile(s): " + err.Error()}
	}

	labeled := make([]coverage.LabeledProfile, 0, len(paths))
	for _, lp := range paths {
		rel, full, err := validateRelPath(corpusRoot, lp.Path)
		if err != nil {
			return &rpcError{Code: ErrCodeParams, Message: "review_diff: coverage_profiles: " + err.Error()}
		}
		if reason := notIndexedReasonOpts(ctx, corpusRoot, service, rel, notIndexedOpts{SkipProjectIgnore: true}); reason != "" {
			return &rpcError{Code: ErrCodeParams, Message: "review_diff: coverage_profiles: " + reason}
		}
		full, err = resolveRealPath(corpusRoot, full, lp.Path)
		if err != nil {
			return &rpcError{Code: ErrCodeParams, Message: "review_diff: coverage_profiles: " + err.Error()}
		}
		f, err := os.Open(full) //nolint:gosec — full is the EvalSymlinks-resolved, corpus-root-contained path from resolveRealPath
		if err != nil {
			return &rpcError{Code: ErrCodeParams, Message: "review_diff: coverage_profiles: open: " + err.Error()}
		}
		profile, perr := coverage.ParseProfile(f)
		f.Close()
		if perr != nil {
			return &rpcError{Code: ErrCodeParams, Message: "review_diff: coverage_profiles: " + perr.Error()}
		}
		labeled = append(labeled, coverage.LabeledProfile{Layer: lp.Layer, Profile: profile})
	}

	in.CoverageChecked = true
	in.CoverageLayered = review.ResolveCoverage(labeled, in.Nodes)
	return nil
}

// applyAndPersistFindings runs the C2-1 findings bridge when the backend
// supports it, via review.BuildPersist (the one composition shared with the
// CLI): read the service's existing findings-layer claims, attach
// content-addressed claim IDs (hashed cites + scope-fp anchors, so the
// daemon's post-refresh sweep and drift passes verify rather than falsify
// them), label already-refuted findings, persist this run's claims (the
// store's refutation-durability upsert, never re-derived here), and
// supersede review-minted claims this run no longer emits (absent = fixed).
// Returns whether persistence fully happened (false when the backend does
// not implement reviewClaimsBackend, or any write failed), plus a
// supersededClaimID→review.ClassifyResolution-kind map for every claim this
// run reconciled as fixed (IT-7's verdict_outcome fixed/superseded split,
// reviewVerdictOutcome) — classified against the PRE-run existing claims'
// own Scope and whether each scoped file is still present in sources (the
// full corpus source list, not just the changed files), mirroring
// cmd/fathom/review.go's runReview resolve closure exactly.
func applyAndPersistFindings(ctx context.Context, backend QueryInterface, v *review.Verdict, rc *reviewCorpus) (bool, map[string]string) {
	rcb, ok := backend.(reviewClaimsBackend)
	if !ok || ctx.Err() != nil {
		return false, nil
	}
	existing, _ := rcb.ClaimsForService(ctx, v.Service, review.ClaimsLayer)
	if ctx.Err() != nil {
		return false, nil
	}
	existingByID := make(map[string]core.SemanticClaim, len(existing))
	for _, cl := range existing {
		existingByID[cl.ClaimID] = cl
	}

	watermark := ""
	if _, indexedCommit, found, err := backend.ServiceMeta(ctx, v.Service); err == nil && found {
		watermark = indexedCommit
	}
	// A citation can be outside the diff. Read every persistable finding's
	// cited path before BuildPersist, so an unreadable citation cannot cause
	// PutClaims to replace a previously valid link set with an empty one.
	// Citation reads have their own bounded window, starting after analysis.
	rc.deadline = rc.now().Add(treeBudget.Wall)
	for _, d := range v.Dimensions {
		if d.Name != review.DimArch && d.Name != review.DimSpecCoverage {
			continue
		}
		for _, f := range d.Findings {
			if path, _, _, ok := core.ParseCiteRef(f.Citation); ok {
				rc.readRequired(ctx, backend, v.Service, path)
			}
		}
	}
	if ctx.Err() != nil || len(rc.gaps) > 0 || (!rc.legacy && !rc.pathsComplete) {
		return false, nil
	}
	claims, links, supersede := review.BuildPersist(v, existing, review.PersistInput{
		Now:       rc.now,
		Watermark: watermark,
		FileContent: func(path string) ([]byte, bool) {
			data, ok := rc.files[path]
			return data, ok
		},
		ScopeFingerprint: func(path string) (string, bool) {
			if ctx.Err() != nil {
				return "", false
			}
			hash, found, ferr := rcb.SourceFingerprint(ctx, v.Service, path)
			return hash, ferr == nil && found
		},
	})
	// SU-22 (ADR-041 decision 7): every claim this run mints is asserted by
	// its corpus watermark as the run identity — review_diff carries no
	// idempotency key of its own (unlike the CLI's --idempotency-key), so
	// every claim from the SAME watermark shares one AssertedBy.
	for i := range claims {
		claims[i].AssertedBy = core.AssertedBy("review", watermark)
	}
	// The path inventory and source bytes used above are two live reads.
	// Refuse all writes if either changed before reconciliation starts.
	if !rc.revalidate(ctx, backend, v.Service) || !rc.indexCurrent(ctx, backend, v.Service) {
		return false, nil
	}

	okAll := true
	if len(claims) > 0 {
		if err := rcb.PutClaims(ctx, claims, links); err != nil {
			okAll = false
		}
	}
	fileExists := func(path string) bool { return rc.paths[path] || (rc.legacy && !rc.pathsComplete) }
	resolutions := make(map[string]string, len(supersede))
	supersedeAt := rc.now().UTC()
	for _, id := range supersede {
		if ctx.Err() != nil {
			return false, resolutions
		}
		cl, hasExisting := existingByID[id]
		if hasExisting {
			resolutions[id] = review.ClassifyResolution(cl.Scope, fileExists)
		}
		if err := rcb.SetClaimStatus(ctx, v.Service, id, core.ClaimSuperseded); err != nil {
			okAll = false
			continue
		}
		// SU-19 (ADR-041 decision 7): close the predecessor's bi-temporal
		// validity window at the same instant its status flips —
		// best-effort, mirroring SetClaimStatus's own error handling.
		if ctx.Err() != nil {
			return false, resolutions
		}
		_ = rcb.SetClaimValidTo(ctx, v.Service, id, supersedeAt)
		// SU-20: an explicit supersedes link when this run's reconcile
		// found a genuine successor for the SAME subject — see
		// review.SupersedeSuccessor's own doc comment.
		if ctx.Err() != nil {
			return false, resolutions
		}
		if hasExisting {
			if successorID, sok := review.SupersedeSuccessor(*v, cl.Subject); sok {
				_ = rcb.WriteSupersedes(ctx, v.Service, successorID, id, "review reconciliation: fixed/changed finding")
			}
		}
	}
	return okAll, resolutions
}
