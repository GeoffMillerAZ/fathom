// Package mcp — review_diff, the program's ONE new MCP tool (C1-1, docs/
// plans/review-verb.md): composes diff impact + obligations/findings +
// architecture conformance + spec coverage into ONE cited verdict over an
// explicit changed-files list, replacing the 8-step manual sequence
// docs/use-cases/change-review.md previously documented tool-by-tool.
package mcp

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/coverage"
	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/projectrules"
)

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
}

func registerReviewDiff(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "review_diff",
		Description: "Compose diff impact (blast radius, typed-edge aware) + obligations/findings scoped to the changed files + architecture conformance (project rule pack) + spec coverage + test coverage into ONE cited verdict for a set of changed files, plus test_impact: affected_tests (the Test* functions that statically reach a changed symbol, with file:line) and test_gap (changed symbols no test reaches). Each dimension reports checked:false with a reason instead of a clean-shaped pass when it cannot run (e.g. no architecture rule pack configured, or no coverage_profile(s) supplied). Pass coverage_profiles (an array of {layer, path} — one Go cover profile per test layer, e.g. [{\"layer\":\"unit\",\"path\":\"workspace/cov/unit.out\"},{\"layer\":\"integration\",\"path\":\"workspace/cov/integration.out\"}], each path repo-relative under the service's corpus root or its workspace/) to get the \"coverage\" dimension: per-changed-symbol {symbol, layers (which supplied layers actually cover it), uncovered_at (layers that ran over it with 0%), percent (union across every layer), uncovered}. `coverage_profile` (a single bare path) is kept as the single-profile alias — it is treated as one profile labeled layer \"unit\" (there is no way to recover which layer produced an unlabeled profile from its own contents, so \"unit\" is the honest default, not an inference); pass coverage_profiles instead when you have more than one layer. Layer attribution is ALWAYS from the caller-declared label, never guessed from per-test data — a standard cover profile carries none. fathom asserts the coverage FACT only, never whether it is adequate. WRITES TO THE CLAIMS STORE when the backend supports persistence (findings_persisted reports it): arch_conformance and spec_coverage findings — the deterministic-oracle dimensions only, never obligations (which re-serves existing claims), impact context, or coverage — are persisted as content-addressed claims on the \"findings\" layer with hashed citation + scope-fingerprint anchors, and review-minted claims whose finding this run no longer emits are superseded (absent = fixed). A finding refuted via `fathom claim refute` or the obligations tool's feedback action stays refuted (status:\"refuted\", excluded from count, still listed) across every later re-review that re-emits the identical finding. For a report-only run without the store write, use the CLI mirror: fathom review --since <ref> --no-persist.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":          map[string]any{"type": "string", "description": "Service / repo identifier"},
				"changed_files":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Repo-relative changed file paths (e.g. from git diff --name-only)"},
				"concern":          map[string]any{"type": "string", "description": "Optional concern token (currently descriptive only; carried for forward compatibility)"},
				"coverage_profile": map[string]any{"type": "string", "description": "Optional single-profile alias for coverage_profiles: a repo-relative path to one Go cover profile, labeled layer \"unit\" (no per-profile layer metadata exists to say otherwise). Ignored when coverage_profiles is also given."},
				"coverage_profiles": map[string]any{
					"type":        "array",
					"description": "Optional: one entry per test layer — {layer, path}, each path repo-relative under the service's corpus root (e.g. workspace/cov/unit.out). Enables the \"coverage\" dimension with real per-layer attribution; absent (and no coverage_profile) = checked:false, reason \"no coverage profile supplied\".",
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

		in, sources, rpcErr := buildReviewInput(ctx, backend, p.Service, p.ChangedFiles, p.Concern)
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
		v := review.Run(in)
		persisted, resolutions := applyAndPersistFindings(ctx, backend, &v, sources)

		payload := v.Payload(persisted)
		if vo := reviewVerdictOutcome(v, resolutions); vo != nil {
			payload["verdict_outcome"] = vo
		}
		payload["test_impact"] = reviewTestGap(in).Payload()
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
func reviewTestGap(in review.Input) impact.TestGapResult {
	return impact.TestGap(in.Nodes, in.ChangedFiles, in.TypedEdges)
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

// buildReviewInput does every I/O review.Run itself never performs:
// ListAllSymbols/TypedEdges for impact, per-changed-file Obligations for the
// obligations dimension, ServiceMeta + internal/projectrules.ResolveRules
// for arch_conformance (review_diff never accepts inline arch rules — it
// always resolves through the service's own project rule pack, or reports
// checked:false), and the changed files' own source content for
// spec_coverage. The full source list is also returned: the persist step
// needs it to hash cited lines (semantic.Sweep's contract), including arch
// citations on files OUTSIDE the changed set.
func buildReviewInput(ctx context.Context, backend QueryInterface, service string, changedFiles []string, concern string) (review.Input, []core.SourceFile, *rpcError) {
	nodes, err := backend.ListAllSymbols(ctx, service)
	if err != nil {
		return review.Input{}, nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
	}
	typed, _ := backend.TypedEdges(ctx, service)

	in := review.Input{
		Service:      service,
		ChangedFiles: changedFiles,
		Concern:      concern,
		Nodes:        nodes,
		TypedEdges:   typed,
	}

	if ob, ok := backend.(obligationsBackend); ok {
		in.ObligationsSupported = true
		in.ObligationsByFile = make(map[string][]review.ObligationRule, len(changedFiles))
		for _, f := range changedFiles {
			obl, oerr := ob.Obligations(ctx, service, f, 0)
			if oerr != nil {
				continue // best-effort per file: a lookup error degrades that file's rules, not the whole dimension
			}
			for _, r := range obl.Rules {
				in.ObligationsByFile[f] = append(in.ObligationsByFile[f], review.ObligationRule{
					File: f, Rule: r.Rule, Src: r.Src, Tier: r.Tier, Kind: r.Kind, Status: r.Status,
				})
			}
		}
	}

	corpusRoot := ""
	if path, _, found, metaErr := backend.ServiceMeta(ctx, service); metaErr == nil && found {
		corpusRoot = path
	}
	rules, source, deny, checked, reason, resolveErr := projectrules.ResolveRules(corpusRoot, nil, false)
	if resolveErr != nil {
		return review.Input{}, nil, &rpcError{Code: ErrCodeInternal, Message: "review_diff: project rules: " + resolveErr.Error()}
	}
	in.ArchRules = rules
	in.ArchRuleSource = source
	in.ArchDenyByDefault = deny
	in.ArchChecked = checked
	in.ArchReason = reason

	sources, serr := backend.Sources(ctx, service)
	if serr == nil {
		changedSet := make(map[string]bool, len(changedFiles))
		for _, f := range changedFiles {
			changedSet[f] = true
		}
		for _, f := range sources {
			if changedSet[f.Path] {
				in.ChangedSources = append(in.ChangedSources, f)
			}
		}
	}

	return in, sources, nil
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
func applyAndPersistFindings(ctx context.Context, backend QueryInterface, v *review.Verdict, sources []core.SourceFile) (bool, map[string]string) {
	rcb, ok := backend.(reviewClaimsBackend)
	if !ok {
		return false, nil
	}
	existing, _ := rcb.ClaimsForService(ctx, v.Service, review.ClaimsLayer)
	existingByID := make(map[string]core.SemanticClaim, len(existing))
	for _, cl := range existing {
		existingByID[cl.ClaimID] = cl
	}

	watermark := ""
	if _, indexedCommit, found, err := backend.ServiceMeta(ctx, v.Service); err == nil && found {
		watermark = indexedCommit
	}
	content := make(map[string][]byte, len(sources))
	for _, f := range sources {
		content[f.Path] = f.Content
	}
	claims, links, supersede := review.BuildPersist(v, existing, review.PersistInput{
		Now:       time.Now,
		Watermark: watermark,
		FileContent: func(path string) ([]byte, bool) {
			data, ok := content[path]
			return data, ok
		},
		ScopeFingerprint: func(path string) (string, bool) {
			hash, found, ferr := rcb.SourceFingerprint(ctx, v.Service, path)
			return hash, ferr == nil && found
		},
	})

	okAll := true
	if len(claims) > 0 {
		if err := rcb.PutClaims(ctx, claims, links); err != nil {
			okAll = false
		}
	}
	fileExists := func(path string) bool { _, ok := content[path]; return ok }
	resolutions := make(map[string]string, len(supersede))
	for _, id := range supersede {
		if cl, ok := existingByID[id]; ok {
			resolutions[id] = review.ClassifyResolution(cl.Scope, fileExists)
		}
		if err := rcb.SetClaimStatus(ctx, v.Service, id, core.ClaimSuperseded); err != nil {
			okAll = false
		}
	}
	return okAll, resolutions
}
