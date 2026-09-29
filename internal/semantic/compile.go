package semantic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient/structured"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/cost"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/llmsec"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// LayerObligations is the v1 tracer-bullet layer (ADR-016 §4.8 #1).
const LayerObligations = "obligations"

// KindObligation is the v1 claim kind.
const KindObligation = "obligation"

// LayerFindings is the D3 findings tracer-bullet layer (docs/plans/
// findings-tracer.md, docs/vision/03-findings-and-experts.md §3.2).
const LayerFindings = "findings"

// KindFinding is the findings-layer claim kind.
const KindFinding = "finding"

// defaultMaxRuleWords is the per-claim rule-length budget used when no
// manifest layer is configured on the Compiler (mirrors schema.cue's own
// default, #Layer.budgets.max_rule_words | *28). Compiler.Layer, when set
// (as pkg/client.CompileObligations does via LoadLayer), overrides it via
// Compiler.maxRuleWords.
const defaultMaxRuleWords = 28

// claimDraft is the strict-JSON shape the LLM must emit per claim.
type claimDraft struct {
	Rule      string   `json:"rule"`
	Scope     []string `json:"scope"`
	CiteStart int      `json:"cite_start"` // 1-based document line
	CiteEnd   int      `json:"cite_end"`   // inclusive
	// Class (docs/design/findings-as-evidence.md §3.2, §6 item 2) is the
	// finding-kind reviewer's closed-vocabulary failure class
	// (core.FindingClass). Present in the schema for every layer (a single
	// shared claimDraftSchema serves all layers), but ENFORCED only for
	// kind==KindFinding — see verifyDraft. Empty/omitted for every other
	// layer's drafts, which never populate it.
	Class string `json:"class,omitempty"`
}

// claimDraftEnvelope is the object-rooted decode target for a section's
// completion: {"claims": [...]}. Anthropic structured outputs document only
// object roots and require additionalProperties:false on every object, so
// the drafts array is wrapped rather than emitted bare — an array-rooted
// schema would 400 the very first real section and abort the whole compile
// (a 400 is transport-class, not the skippable ErrTruncated/ErrMalformed).
type claimDraftEnvelope struct {
	Claims []claimDraft `json:"claims"`
}

// claimDraftSchema is the JSON Schema constraining structured.Complete's
// generation to claimDraftEnvelope's shape (M1-6). Its properties mirror
// the json tags exactly — a mismatch surfaces as structured.ErrMalformed
// via the strict, DisallowUnknownFields decode. Every object level carries
// additionalProperties:false and the root is an object (see
// TestClaimDraftSchema_AnthropicStructuredOutputCompatible, which pins both
// wire constraints).
var claimDraftSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"claims": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"rule": {"type": "string"},
					"scope": {"type": "array", "items": {"type": "string"}},
					"cite_start": {"type": "integer"},
					"cite_end": {"type": "integer"},
					"class": {"type": "string"}
				},
				"required": ["rule", "scope", "cite_start", "cite_end"],
				"additionalProperties": false
			}
		}
	},
	"required": ["claims"],
	"additionalProperties": false
}`)

// Compiler compiles governing docs into obligations claims. The LLM client
// is injected (llmclient.NewMockClient in tests — network-free and
// reproducible); the Redactor is the mandatory DLP gate on every prompt.
type Compiler struct {
	St       store.Store
	LLM      llmclient.LLMClient
	Redactor *llmsec.Redactor
	Model    string // recorded in provenance; "" uses the client default

	// Layer is the manifest-loaded layer definition (LoadLayer("obligations"))
	// whose corpus/budgets govern this compile (M1-1/M1-7). A zero-value
	// Layer (Name == "") disables budget enforcement and falls back to
	// defaultMaxRuleWords — direct Compiler construction without a loaded
	// layer (as most existing tests, and the expert-module compile path,
	// do) preserves the pre-M1 unbudgeted behavior.
	Layer LayerDef

	// USD per 1k tokens for the cost ledger; zero rates record honest zeros.
	InRatePerKTok  float64
	OutRatePerKTok float64

	// Decider is the decision port (ADR-042, vision 32 §2 item 1) this
	// compile asks PrefilterSections through before anything else considers
	// an ordered section. nil (the default: no wave wires it without
	// operator config) makes PrefilterSections the identity — no call, no
	// section skipped, byte-identical to a Compiler built before this
	// field existed.
	Decider decision.Decider
	// PrefilterThreshold is the noul cutoff PrefilterSections applies: a
	// section scoring below it is skipped (not compiled, but still counted
	// and disclosed — see CompileRun.SectionsPrefiltered). Zero with a
	// non-nil Decider means 0.5 (defaultPrefilterThreshold), documented
	// here since the zero value would otherwise silently skip every
	// section (noul is never negative). Meaningless when Decider is nil.
	PrefilterThreshold float64

	Now func() time.Time // injected clock (repo convention)
}

// maxRuleWords returns the effective per-claim rule-length budget: the
// manifest layer's value when configured, else defaultMaxRuleWords.
func (c *Compiler) maxRuleWords() int {
	if c.Layer.Budgets.MaxRuleWords > 0 {
		return c.Layer.Budgets.MaxRuleWords
	}
	return defaultMaxRuleWords
}

// CompileInput is one compile invocation.
type CompileInput struct {
	ServiceID string
	// Docs are the governing documents (repo-relative paths + content),
	// already selected by the caller (the layer's corpus selector).
	Docs []core.SourceFile
	// IndexedFiles is the live index's file list for scope verification: a
	// claim whose scope globs match none of these is rejected at compile
	// time (ADR-016 §4.4 — the scope-trap defense).
	IndexedFiles []string
	// Watermark is the service corpus watermark at compile time.
	Watermark string
	// DryRun estimates sections/tokens without calling the LLM or writing
	// claims. Mandatory before any spending compile (ADR-016).
	DryRun bool
	// Escape carries forward into the recorded CompileRun's Escape field
	// (core.CompileRun.Escape's doc comment) — the caller's audit note when
	// an explicit override flag let this run proceed past a guard that would
	// otherwise have refused it. Empty for every normal run.
	Escape string

	// Resume is O54 deliverable 3 (`fathom compile --layer obligations
	// --continue`): a prior run of the SAME (service, layer) at the SAME
	// budget can leave most of the corpus uncompiled (BudgetStopped) —
	// section-level progress already survives via the digest gate (an
	// unbudgeted or larger-budget follow-up compiles only what's left), but
	// a follow-up run at the SAME max_tokens_per_run budget refuses at the
	// PRE-FLIGHT gate below whenever the REMAINING corpus alone still
	// estimates over that budget, even though the mid-run gate would
	// happily make further incremental progress and stop cleanly again —
	// an all-or-nothing gate over the remaining work, not a resume. Resume
	// skips that pre-flight refusal so a budget-stopped layer keeps making
	// progress call over call, the mid-run gate capping each call exactly
	// as it always has; it also asks Compile to name the first
	// not-yet-compiled section (in orderSectionsByGoverningScope's own
	// order) as run.ResumedFrom — proof, not a claim, that this call picked
	// up where the last one left off rather than restarting.
	Resume bool

	// NoPriorRun is refactor-conservation gap-closing wave 3, item 3
	// (docs/plans/refactor-conservation.md, CV-41): true when the caller
	// found NO existing compile_runs row for this (service, layer) at all —
	// a genuinely first-ever compile, never before attempted. Before this
	// field existed, a layer whose WHOLE corpus estimate exceeded
	// max_tokens_per_run was refused outright at the pre-flight gate on
	// its very first compile, exactly like a re-compile — so CV-41's new
	// default layer set (adr-rules/spec-bindings activate whenever their
	// corpus matches) could never activate on any real repository whose
	// corpus is bigger than one run's budget, without an operator already
	// knowing to pass --continue for a layer that had never compiled
	// anything yet. NoPriorRun skips the SAME pre-flight refusal Resume
	// does, so a first compile makes as much progress as its budget
	// allows and stops cleanly as BudgetStopped (resumable with
	// --continue from then on, exactly as an ordinary budget-stopped
	// layer already is) — never Refused for a layer nothing has ever
	// tried to compile. Unlike Resume, NoPriorRun never sets
	// run.ResumedFrom (nothing is being resumed): the two flags are
	// mutually exclusive in practice (Resume implies a prior run already
	// exists), but Compile ORs them for the pre-flight gate rather than
	// asserting exclusivity, so a caller mistake here degrades to "skip
	// the refusal" rather than a confusing panic. The SECOND call against
	// the same (service, layer) — even one that stopped on BudgetStopped,
	// not Refused — sees a real compile_runs row and reverts to the
	// ordinary pre-flight refusal, exactly the "a plain re-run refuses at
	// the pre-flight gate — pass --continue" guidance
	// printCompileRunCore's own BudgetStopped line already prints.
	NoPriorRun bool
}

// docSections pairs one governing doc with its already-split sections, so
// the whole corpus can be walked once up front (see Compile).
type docSections struct {
	doc      core.SourceFile
	sections []Section
}

// Compile runs the obligations compile: split → digest gate → budget gate →
// DLP → LLM (strict JSON via structured.Complete) → anchor + scope
// verification → transactional store, then supersedes claims of recompiled
// subjects that were not re-emitted. The returned CompileRun has already
// been recorded to the store (unless DryRun).
// Dry-run output-band ratios (O8): the honest bracket for estimated output
// tokens as a multiple of estimated input. Observed: the obligations layer
// ran ~6.7x. Revisable data, not doctrine.
const (
	dryRunOutRatioLow  = 2
	dryRunOutRatioHigh = 8
)

func (c *Compiler) Compile(ctx context.Context, in CompileInput) (core.CompileRun, error) {
	if c.St == nil {
		return core.CompileRun{}, fmt.Errorf("semantic: Compiler.St is required")
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	started := now()

	// Layer identity: c.Layer (manifest-loaded via semantic.LoadLayer, set by
	// pkg/client.CompileLayer) is authoritative when present. A zero-value
	// Layer (Name == "") falls back to the pre-M1 obligations identity, so
	// direct Compiler construction without a loaded layer — most existing
	// tests, and the expert-module non-git-corpus test — keeps its exact
	// original behavior unchanged (FT-1 generalizes the compiler past a
	// single hardcoded layer without breaking that contract). LC-1: both
	// branches now resolve through the SAME generic promptForLayer(LayerDef)
	// — the fallback is "no layer configured, default to obligations", not a
	// second prompt-resolution mechanism.
	layerDef := c.Layer
	if layerDef.Name == "" {
		var lerr error
		layerDef, lerr = LoadLayer(LayerObligations)
		if lerr != nil {
			return core.CompileRun{}, fmt.Errorf("semantic: load default obligations layer: %w", lerr)
		}
	}
	systemPrompt, promptVersion, perr := promptForLayer(layerDef)
	if perr != nil {
		return core.CompileRun{}, fmt.Errorf("semantic: resolve layer %q prompt: %w", layerDef.Name, perr)
	}

	layerName := LayerObligations
	layerVersion := "v1"
	kind := KindObligation
	tier := core.TierDocClaimed
	tierProfile := "engineering"
	// driftTracked resolves the layer's serving.drift_tracked (LC-2,
	// docs/plans/layer-catalog.md): whether this layer's claims persist
	// scope-fp drift anchors at compile time. Replaces the old
	// `kind == KindFinding` hardcode — a schema field, not a Go type check,
	// gates the behavior, so the next layer that wants drift tracking gets
	// it by setting one CUE field, not by editing this switch.
	driftTracked := false
	if c.Layer.Name != "" {
		layerName = c.Layer.Name
		if c.Layer.Version != "" {
			layerVersion = c.Layer.Version
		}
		kind = c.Layer.Kind
		tier = core.EvidenceTier(c.Layer.Tier)
		if c.Layer.TierProfile != "" {
			tierProfile = c.Layer.TierProfile
		}
		driftTracked = c.Layer.Serving.DriftTracked
	}

	run := core.CompileRun{
		ServiceID:       in.ServiceID,
		Layer:           layerName,
		CorpusWatermark: in.Watermark,
		Model:           c.Model,
		PromptVersion:   promptVersion,
		DryRun:          in.DryRun,
		RanAt:           started.UTC(),
		Escape:          in.Escape,
	}
	idSum := sha256.Sum256([]byte(in.ServiceID + "\x00" + layerName + "\x00" + in.Watermark + "\x00" + started.UTC().Format(time.RFC3339Nano)))
	run.RunID = hex.EncodeToString(idSum[:])

	// Existing claims, grouped by subject, for the digest gate + supersede.
	// Superseded and refuted rows are EXCLUDED: they keep their old
	// InputDigest/PromptVersion forever, so one such row would make
	// allDigestsMatch false for its subject on every future run — the
	// section re-LLMs for the rest of time (F2). They need no supersede
	// handling either: superseded is already terminal for the sweep, and a
	// refuted claim keeps its stronger human verdict (neither is served).
	existing, err := c.St.ClaimsForService(ctx, in.ServiceID, layerName)
	if err != nil {
		return run, fmt.Errorf("semantic: load existing claims: %w", err)
	}
	bySubject := map[string][]core.SemanticClaim{}
	for _, cl := range existing {
		if cl.Status == core.ClaimSuperseded || cl.Status == core.ClaimRefuted {
			continue
		}
		bySubject[cl.Subject] = append(bySubject[cl.Subject], cl)
	}

	// Deterministic doc order.
	docs := append([]core.SourceFile(nil), in.Docs...)
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })

	// Split every doc and record every section's subject UP FRONT, not
	// lazily per-doc inside the compile loop below: a mid-run budget stop
	// (enforceBudgets, M1-7) may break out before reaching later docs, and
	// the supersede sweep at the end must still see EVERY section's subject
	// in currentSubjects, or it would wrongly retire a doc that simply
	// hasn't been reached yet, instead of leaving it untouched for the next
	// (digest-gated) run to resume.
	all := make([]docSections, 0, len(docs))
	currentSubjects := map[string]bool{}
	for _, doc := range docs {
		sections := SplitMarkdownSections(doc.Path, doc.Content)
		run.SectionsTotal += len(sections)
		for _, sec := range sections {
			currentSubjects[sec.Subject] = true
		}
		all = append(all, docSections{doc: doc, sections: sections})
	}

	// O54 deliverable 1: a FLAT, nearest-scope-first compile order —
	// every path-scoped guide before every root-scoped document, smaller
	// sections first within a tier — replacing the old doc-then-section
	// walk whose doc order was merely alphabetical (see
	// orderSectionsByGoverningScope's doc comment for the bug this fixes).
	ordered := orderSectionsByGoverningScope(all)

	// ADR-042 decision 2 / vision 32 §2 item 1: ask the decision port which
	// ordered sections state a binding rule BEFORE anything else considers
	// them — before the digest gate, the pre-flight budget estimate, and
	// the section loop. A skipped section is removed from ordered for the
	// rest of this run (never compiled, never failed) but is never lost:
	// prefilteredOrdered feeds run.UncompiledScopes below exactly like a
	// budget-stopped tail, so ADR-037's raw fallback still serves it. With
	// c.Decider nil (the default), PrefilterSections is the identity and
	// this changes nothing.
	threshold := c.PrefilterThreshold
	if c.Decider != nil && threshold == 0 {
		threshold = defaultPrefilterThreshold
	}
	orderedSecs := make([]Section, len(ordered))
	for i, item := range ordered {
		orderedSecs[i] = item.sec
	}
	_, prefilteredSecs, decisions, decErr, perr := PrefilterSections(ctx, c.Decider, threshold, orderedSecs)
	if perr != nil {
		// DP-22 (O109): decErr is non-nil here (a real Decide error, not
		// ErrNoProvider — PrefilterSections' identity/no-provider paths
		// never set perr at all) — carry it onto the aborted run so the
		// caller can log it instead of it vanishing with the error return.
		run.PrefilterError = toCoreDecisionError(decErr)
		return run, fmt.Errorf("semantic: compile: %w", perr)
	}
	run.Decisions = toCoreDecisionRecords(decisions)
	var prefilteredOrdered []orderedSection
	if len(prefilteredSecs) > 0 {
		skip := make(map[string]bool, len(prefilteredSecs))
		for _, sec := range prefilteredSecs {
			skip[sec.Subject] = true
			run.PrefilteredSections = append(run.PrefilteredSections, sec.Subject)
		}
		run.SectionsPrefiltered = len(prefilteredSecs)
		kept := make([]orderedSection, 0, len(ordered)-len(prefilteredSecs))
		for _, item := range ordered {
			if skip[item.sec.Subject] {
				prefilteredOrdered = append(prefilteredOrdered, item)
				continue
			}
			kept = append(kept, item)
		}
		ordered = kept
	}

	needsCompile := func(sec Section) bool {
		prior := bySubject[sec.Subject]
		return len(prior) == 0 || !allDigestsMatch(prior, sec.Digest, promptVersion)
	}

	// O54 deliverable 3 (`--continue`): name the first not-yet-compiled
	// section in the resume order — proof this call picked up where a
	// prior budget-stopped run left off, never a restart. Computed
	// up front so it is recorded even on a DryRun estimate.
	if in.Resume {
		for _, item := range ordered {
			if needsCompile(item.sec) {
				run.ResumedFrom = item.sec.Subject
				break
			}
		}
	}

	// Budget gate, pre-flight (ADR-016 §4.1, M1-7): a real (non-dry-run)
	// compile refuses to start when its own digest-gate-aware cost estimate
	// exceeds the layer's budget — before any LLM call is made. Skipped
	// entirely when Resume is set: the whole point of resuming a
	// budget-stopped layer is that the REMAINING corpus alone may still
	// estimate over the same per-run budget — refusing here would turn
	// "make more progress" into a permanent refusal instead of a resume.
	// Also skipped when NoPriorRun is set (CV-41, gap-closing wave 3 item
	// 3): a layer with no compile_runs row at all for this service has
	// never been attempted, so refusing its very first compile the same
	// way a re-compile is refused would make a corpus bigger than one
	// run's budget permanently unreachable without an operator already
	// knowing to pass --continue for work that never started — see
	// CompileInput.NoPriorRun's own doc comment. The mid-run gate below
	// still caps this call's real spend exactly as it always has, either
	// way.
	if !in.DryRun && c.Layer.Name != "" && !in.Resume && !in.NoPriorRun {
		estTokensIn := 0
		for _, item := range ordered {
			if needsCompile(item.sec) {
				estTokensIn += cost.Estimate(item.sec.Text) + promptOverheadTokens
			}
		}
		if refuse, reason := enforceBudgets(c.Layer, estTokensIn, 0, c.InRatePerKTok); refuse {
			run.Refused = true
			run.RefusalReason = reason
			run.TokensIn = estTokensIn
			run.CostUSD = cost.PriceUSD(estTokensIn, c.InRatePerKTok)
			run.DurationMS = now().Sub(started).Milliseconds()
			if err := c.St.RecordCompileRun(ctx, run); err != nil {
				return run, fmt.Errorf("semantic: record refused compile run: %w", err)
			}
			return run, fmt.Errorf("semantic: compile refused: %s", reason)
		}
	}

	var (
		newClaims          []core.SemanticClaim
		newLinks           []core.ClaimLink
		recompiledSubjects []string
	)
	// O19's fix (docs/design/observability-surfaces.md §8 item 5):
	// accumulated across the whole run, turned into run.RejectionReasons
	// once via buildRejectionReasons — never populated on a DryRun (the
	// section loop below skips verifyDraft entirely in that branch).
	rejectionCounts := map[rejectionCause]int{}
	rejectionSamples := map[rejectionCause][]string{}

	// stoppedAt is the ordered-index of the section a mid-run budget stop
	// broke on; -1 means the whole ordered corpus was walked (O54
	// deliverable 2's layer_coverage.uncompiled_scopes needs to know
	// exactly which tail, if any, this run never reached).
	stoppedAt := -1

	// SU-21 (ADR-041 decision 7): resolves a cites span to its enclosing
	// symbol, when one exists — a governing PROSE document resolves to
	// zero symbols on every call (anchorSymbolResolver never pays for the
	// index load in that overwhelmingly common case); a layer whose corpus
	// cites real code gets a live "anchors" link on every claim it mints.
	anchorSymbol := anchorSymbolResolver(ctx, c.St, in.ServiceID)

sectionLoop:
	for i, item := range ordered {
		doc, sec := item.doc, item.sec
		// Digest gate: an unchanged section (same content AND prompt
		// identity) keeps its claims untouched.
		if !needsCompile(sec) {
			continue
		}
		if in.DryRun {
			run.SectionsCompiled++
			run.TokensIn += cost.Estimate(sec.Text) + promptOverheadTokens
			continue
		}
		if c.LLM == nil {
			return run, fmt.Errorf("semantic: no LLM client configured (section %s needs compilation; run with dry_run to estimate)", sec.Subject)
		}

		drafts, usage, err := c.compileSection(ctx, sec, systemPrompt)
		run.TokensIn += usage.InputTokens
		run.TokensOut += usage.OutputTokens
		if err != nil && ctx.Err() == nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			// One retry after a short pause: a harness that hit a rate
			// limit or printed a non-envelope line usually answers the
			// second time (the 2026-09-13 scenario dry cells failed one
			// section of ~130 on two different runs, a different section
			// each time).
			time.Sleep(compileRetryPause)
			var usage2 llmclient.Usage
			drafts, usage2, err = c.compileSection(ctx, sec, systemPrompt)
			run.TokensIn += usage2.InputTokens
			run.TokensOut += usage2.OutputTokens
		}
		if err != nil {
			// A section whose answer never arrived poisons only its own
			// section: count it, name it, keep the section's prior
			// claims, and move on — one bad response must never abort
			// an 88-section corpus. Only a cancelled or expired context
			// aborts; a run in which NOTHING compiled fails at the end.
			if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return run, fmt.Errorf("semantic: compile %s: %w", sec.Subject, err)
			}
			run.SectionsFailed++
			if len(run.SectionErrors) < maxSectionErrors {
				run.SectionErrors = append(run.SectionErrors, fmt.Sprintf("%s: %v", sec.Subject, firstLine(err.Error())))
			}
			continue
		}
		run.SectionsCompiled++
		recompiledSubjects = append(recompiledSubjects, sec.Subject)

		for _, d := range drafts {
			claim, links, verr := c.verifyDraft(ctx, in.ServiceID, d, sec, doc, in.IndexedFiles, in.Watermark, started, layerName, layerVersion, kind, tier, tierProfile, promptVersion, driftTracked, anchorSymbol)
			if verr != nil {
				// A drift-anchor failure is infrastructure, not a bad
				// draft: abort the run loudly (FT-6 — see errDriftAnchor).
				if errors.Is(verr, errDriftAnchor) {
					return run, fmt.Errorf("semantic: compile %s: %w", sec.Subject, verr)
				}
				run.ClaimsRejected++
				cause := classifyRejectionCause(verr)
				rejectionCounts[cause]++
				if len(rejectionSamples[cause]) < maxRejectionSamplesPerCause {
					rejectionSamples[cause] = append(rejectionSamples[cause], fmt.Sprintf("%s: %v", sec.Subject, verr))
				}
				continue
			}
			// SU-22 (ADR-041 decision 7): every claim this run mints is
			// asserted by THIS compile run's own id — run.RunID is already
			// computed (before the section loop starts) so every claim in
			// the run shares one identity.
			claim.AssertedBy = core.AssertedBy("compile", run.RunID)
			newClaims = append(newClaims, claim)
			newLinks = append(newLinks, links...)
		}

		// Budget gate, mid-run (M1-7): the REAL running token total may
		// cross max_tokens_per_run (or, opportunistically, the real
		// spend so far may cross max_usd_per_run) after any section.
		// Stop cleanly — sections not yet reached stay untouched and
		// resume via the digest gate on the next invocation.
		if refuse, reason := enforceBudgets(c.Layer, run.TokensIn, run.TokensOut, c.InRatePerKTok); refuse {
			run.BudgetStopped = true
			run.RefusalReason = reason
			stoppedAt = i
			break sectionLoop
		}
	}
	run.ClaimsEmitted = len(newClaims)
	// uncompiledTail starts with whatever the decision-port pre-filter
	// skipped above (DP-6: a skipped section is treated exactly like an
	// uncompiled one here, never as compiled, never as failed) and, on a
	// mid-run budget stop, gains the tail of the deterministic order this
	// run never reached — exactly what layer_coverage.uncompiled_scopes
	// discloses at serve time.
	uncompiledTail := append([]orderedSection(nil), prefilteredOrdered...)
	if stoppedAt >= 0 {
		// O54 deliverable 2, second review round: the tail alone is not
		// enough — a RESUMED run can stop again before reaching a section
		// LATER in order that a PRIOR run already compiled (a small guide
		// inserted between two already-compiled sections, say), and that
		// section's doc must not be reported uncompiled just because this
		// call never revisited it. Filter through needsCompile (the same
		// digest-gate predicate the loop above uses) so only sections that
		// ACTUALLY still need compiling — never reached this run AND no
		// matching prior claim — count.
		for _, item := range ordered[stoppedAt+1:] {
			if needsCompile(item.sec) {
				uncompiledTail = append(uncompiledTail, item)
			}
		}
	}
	run.UncompiledScopes = uncompiledDocScopes(uncompiledTail)
	run.RejectionReasons = buildRejectionReasons(rejectionCounts, rejectionSamples)
	run.CostUSD = cost.PriceUSD(run.TokensIn, c.InRatePerKTok) + cost.PriceUSD(run.TokensOut, c.OutRatePerKTok)
	if run.SectionsCompiled == 0 && run.SectionsFailed > 0 {
		return run, fmt.Errorf("semantic: compile: every section that needed the model failed (%d); first: %s", run.SectionsFailed, run.SectionErrors[0])
	}
	run.DurationMS = now().Sub(started).Milliseconds()

	if in.DryRun {
		// O8: the dry run once reported input-only tokens and cost_usd
		// 0.000000 for a run whose real cost was 25x the token estimate.
		// Output is a RANGE: the observed obligations-layer ratio was
		// ~6.7x input; the band [2x, 8x] brackets the layer classes seen
		// so far and is data to revise, not a promise.
		run.EstTokensOutLow = run.TokensIn * dryRunOutRatioLow
		run.EstTokensOutHigh = run.TokensIn * dryRunOutRatioHigh
		inCost := cost.PriceUSD(run.TokensIn, c.InRatePerKTok)
		run.EstCostLowUSD = inCost + cost.PriceUSD(run.EstTokensOutLow, c.OutRatePerKTok)
		run.EstCostHighUSD = inCost + cost.PriceUSD(run.EstTokensOutHigh, c.OutRatePerKTok)
		return run, nil
	}

	if err := c.St.PutClaims(ctx, newClaims, newLinks); err != nil {
		return run, fmt.Errorf("semantic: store claims: %w", err)
	}

	// Supersede: prior claims of recompiled subjects that were not re-emitted,
	// AND claims whose subject is no longer in the corpus at all — a compile
	// owns its whole layer, so a doc leaving the corpus selector retires its
	// claims (the sweep can't: the doc still exists and its citations still
	// hash-match). currentSubjects was built for the WHOLE corpus up front
	// (see above), so a subject from a doc not yet reached (mid-run budget
	// stop) is correctly treated as still-current, not retired.
	emitted := map[string]bool{}
	for _, cl := range newClaims {
		emitted[cl.ClaimID] = true
	}
	retire := map[string]bool{}
	sort.Strings(recompiledSubjects)
	for _, subj := range recompiledSubjects {
		retire[subj] = true
	}
	subjects := make([]string, 0, len(bySubject))
	for subj := range bySubject {
		if !currentSubjects[subj] {
			retire[subj] = true
		}
		subjects = append(subjects, subj)
	}
	sort.Strings(subjects)
	for _, subj := range subjects {
		if !retire[subj] {
			continue
		}
		for _, old := range bySubject[subj] {
			if !emitted[old.ClaimID] && old.Status != core.ClaimSuperseded {
				// GL-15 (design doc §7): supersession is a claim-ending
				// transition exactly like refute — close the validity
				// window at the same instant the status flips, never
				// leave it open. closeClaimValidity replaces a bare
				// SetClaimStatus call that used to leave ValidTo
				// untouched here (every OTHER claim-ending path already
				// paired the two calls by hand; this one had drifted).
				if err := closeClaimValidity(ctx, c.St, in.ServiceID, old.ClaimID, core.ClaimSuperseded, now()); err != nil {
					return run, fmt.Errorf("semantic: supersede %s: %w", old.ClaimID, err)
				}
			}
		}
	}

	if err := c.St.RecordCompileRun(ctx, run); err != nil {
		return run, fmt.Errorf("semantic: record compile run: %w", err)
	}
	return run, nil
}

// promptOverheadTokens approximates the fixed prompt scaffolding per section
// for dry-run / pre-flight estimates.
const promptOverheadTokens = 260

// allDigestsMatch reports whether every prior claim was compiled from this
// exact section content AND under the given prompt version — a prompt
// version change must recompile every section, or stale-prompt claims
// survive the gate (observed on the v1→v2 hand-bumped-const bug: 27/32
// sections kept v1 claims). promptVersion is the layer's content-addressed
// identity from promptForLayer (M1-4/M1-5, LC-1) — no more package-level const.
func allDigestsMatch(claims []core.SemanticClaim, digest, promptVersion string) bool {
	for _, cl := range claims {
		if cl.InputDigest != digest || cl.PromptVersion != promptVersion {
			return false
		}
	}
	return true
}

// enforceBudgets applies the manifest's run-level budget gate (schema.cue
// #Layer.budgets, ADR-016 §4.1, M1-7). The compile loop calls it at two
// moments with the same signature, reacting differently to a refusal:
//
//   - pre-flight (before any LLM call): tokensIn is the digest-gate-aware
//     estimate over sections that would actually be compiled, tokensOut is
//     0 — a refusal here means the run never starts.
//   - mid-run (after each compiled section): tokensIn/tokensOut are the
//     REAL running totals so far — a refusal here means the run stops
//     cleanly; sections not yet reached stay untouched and resume via the
//     digest gate on the next invocation.
//
// A zero-value LayerDef (Name == "") disables both checks: direct Compiler
// construction without a loaded layer (most existing tests, and the
// expert-module compile path) preserves the pre-M1 unbudgeted behavior.
func enforceBudgets(layer LayerDef, tokensIn, tokensOut int, inRatePerKTok float64) (refuse bool, reason string) {
	if layer.Name == "" {
		return false, ""
	}
	if max := layer.Budgets.MaxTokensPerRun; max > 0 {
		if total := tokensIn + tokensOut; total > max {
			return true, fmt.Sprintf("token total %d exceeds layer %q budget max_tokens_per_run=%d", total, layer.Name, max)
		}
	}
	// wave O4, item 2: max_usd_per_run <= 0 now means unlimited, mirroring
	// max_tokens_per_run's own ">0" convention above — previously this
	// check ran unconditionally, so an override (or a hand-authored layer.cue)
	// that set max_usd_per_run to 0 "for no cost cap" instead refused every
	// non-free run outright (estCost > 0 for any real spend). A layer.cue
	// author who wants a hard $0 cap should simply never compile that layer
	// non-dry-run; nothing in this repo's shipped packs relies on the old
	// always-refuse-at-zero behavior (they all set a positive max_usd_per_run).
	if max := layer.Budgets.MaxUSDPerRun; max > 0 {
		if estCost := cost.PriceUSD(tokensIn, inRatePerKTok); estCost > max {
			return true, fmt.Sprintf("estimated cost $%.4f exceeds layer %q budget max_usd_per_run=$%.4f", estCost, layer.Name, max)
		}
	}
	return false, ""
}

// compileSection runs one section through DLP + LLM via structured.Complete
// (M1-6), which sets Request.OutputSchema to claimDraftSchema and classifies
// the two silent failure modes as errors.Is-matchable sentinels
// (structured.ErrTruncated, structured.ErrMalformed) instead of this package
// hand-classifying them via the retired errBadResponse wrapper. systemPrompt
// is the caller's layer-resolved prompt text (promptForLayer, FT-1) — every
// layer shares the claimDraft envelope/schema; only the instructions differ.
func (c *Compiler) compileSection(ctx context.Context, sec Section, systemPrompt string) ([]claimDraft, llmclient.Usage, error) {
	prompt := buildSectionPrompt(sec)
	if c.Redactor != nil {
		prompt, _ = c.Redactor.Redact(prompt)
	}
	var envelope claimDraftEnvelope
	resp, err := structured.Complete(ctx, c.LLM, llmclient.Request{
		Model:     c.Model,
		System:    systemPrompt,
		Prompt:    prompt,
		MaxTokens: 2400, // rule-dense sections (large tables) overflowed 1200 and truncated mid-JSON
	}, claimDraftSchema, &envelope)
	if err != nil {
		return nil, resp.Usage, err
	}
	return envelope.Claims, resp.Usage, nil
}

// buildSectionPrompt renders the section with absolute document line numbers
// so the model can cite verifiable ranges.
func buildSectionPrompt(sec Section) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Document: %s\nSection: %s (lines %d-%d)\n\n", sec.Path, sec.Subject, sec.StartLine, sec.EndLine)
	for i, line := range strings.Split(sec.Text, "\n") {
		fmt.Fprintf(&b, "%d: %s\n", sec.StartLine+i, line)
	}
	return b.String()
}

// verifyDraft applies compile-time verification (ADR-016 §4.4): the citation
// must resolve inside the document, and every scope glob must match at least
// one indexed file. Failures reject the claim — a claim is only as good as
// its anchors. layerName/layerVersion/kind/tier/tierProfile/driftTracked are
// the caller's resolved layer identity (FT-1: Compile resolves them once from
// c.Layer, defaulting to the obligations identity when no layer is loaded).
func (c *Compiler) verifyDraft(ctx context.Context, serviceID string, d claimDraft, sec Section, doc core.SourceFile, indexedFiles []string, watermark string, at time.Time, layerName, layerVersion, kind string, tier core.EvidenceTier, tierProfile, promptVersion string, driftTracked bool, anchorSymbol func(claimID, citeRef string, fetch func(string) ([]byte, bool)) (core.ClaimLink, bool)) (core.SemanticClaim, []core.ClaimLink, error) {
	rule := strings.TrimSpace(d.Rule)
	if rule == "" {
		return core.SemanticClaim{}, nil, rejectErr(causeMalformed, "empty rule")
	}
	if n, max := len(strings.Fields(rule)), c.maxRuleWords(); n > max {
		return core.SemanticClaim{}, nil, rejectErr(causeMalformed, "rule exceeds %d-word budget (%d)", max, n)
	}
	// Citation must lie within this section's document range and hash.
	if d.CiteStart < sec.StartLine || d.CiteEnd > sec.EndLine || d.CiteEnd < d.CiteStart {
		return core.SemanticClaim{}, nil, rejectErr(causeAnchorMissing, "citation outside section range")
	}
	hash, ok := core.HashLines(doc.Content, d.CiteStart, d.CiteEnd)
	if !ok {
		return core.SemanticClaim{}, nil, rejectErr(causeAnchorMissing, "citation range does not resolve")
	}
	// T5-1/O13: persist the cited lines' literal text alongside the hash —
	// LinesText extracts with the exact same normalization HashLines just
	// hashed, so the two are always consistent. The sweep uses AnchorText to
	// re-anchor a drifted citation (search the file for this snippet) rather
	// than merely detecting drift; a claim compiled without it can only ever
	// stale, never heal.
	anchorText, ok := core.LinesText(doc.Content, d.CiteStart, d.CiteEnd)
	if !ok {
		return core.SemanticClaim{}, nil, rejectErr(causeAnchorMissing, "citation range does not resolve")
	}
	// Scope globs must be non-empty, each match >= 1 indexed file, and none
	// may be over-broad (ADR-016 §4.4 scope-trap defense, extended by O8/
	// OP-2): a glob matching more than half of the indexed corpus is a
	// catch-all in disguise (typically "**" or a repo-wide directory
	// pattern) and is rejected exactly like a failed citation, not silently
	// narrowed or allowed through. Single-file-class extension globs are
	// EXEMPT from the breadth check (never from the zero-match check):
	// prompt v3 mandates "**/*.go"-shaped scopes as the only legitimate form
	// for fleet-wide rules, and in a go-majority corpus that glob necessarily
	// matches >50% of files — without the exemption the guard would reject
	// every prompt-compliant repo-wide code rule (the guard and the prompt
	// would structurally contradict each other). Every indexed file actually
	// matched by ANY glob is collected into scopeFiles (not just "matched at
	// least one") — finding-kind claims need the full matched-file set below
	// to anchor remediation-drift fingerprints (FT-6).
	// GL-2..GL-6 scope injection (rulesscope.go): a rules file that
	// declares its own scope (Cursor globs, Copilot applyTo, Kiro
	// fileMatchPattern) replaces the model's proposed scope here, BEFORE
	// the floor and the match/breadth checks — the declared globs pass the
	// same I27 verification any claim's scope does, never a bypass.
	declared, audience := applyDeclaredScope(doc.Path, doc.Content, d.Scope)
	scope := append([]string(nil), declared...)
	if len(scope) == 0 {
		return core.SemanticClaim{}, nil, rejectErr(causeMalformed, "empty scope")
	}
	// BP-32: a claim compiled from a NESTED instruction file governs only
	// that file's own subtree. Applied before the match/breadth checks so
	// everything downstream (validation, RelScopes links, reproKey) sees the
	// floored globs.
	floored, ferr := scopeFloor(doc.Path, scope)
	if ferr != nil {
		return core.SemanticClaim{}, nil, rejectErr(causeScopeOutOfSubtree, "%s", ferr)
	}
	scope = floored
	sort.Strings(scope)
	scopeFiles := map[string]bool{}
	for _, g := range scope {
		m := filter.New([]string{g})
		matched := 0
		for _, f := range indexedFiles {
			if m.Match(f) {
				matched++
				scopeFiles[f] = true
			}
		}
		if matched == 0 {
			return core.SemanticClaim{}, nil, rejectErr(causeScopeUnverifiable, "scope %q matches no indexed file", g)
		}
		if matched*2 > len(indexedFiles) && !isFileClassGlob(g) {
			return core.SemanticClaim{}, nil, rejectErr(causeOverBroad, "scope %q is over-broad: matches %d/%d indexed files (>50%%) and is not a single-file-class extension glob, rejected as a catch-all scope", g, matched, len(indexedFiles))
		}
	}

	// Findings-as-evidence typed failure class (docs/design/findings-as-
	// evidence.md §3.2, §6 item 2): the reviewer assigns `class` at emit
	// time for kind==KindFinding drafts ONLY — an empty or unrecognized
	// class is a compile-time rejection, mirroring O19's four causes
	// above, never a silent "other" bucket. Every other layer's drafts
	// never carry a class and are unaffected.
	class := core.FindingClass(d.Class)
	if kind == KindFinding {
		if !class.Valid() {
			return core.SemanticClaim{}, nil, rejectErr(causeInvalidClass, "class %q is not a member of the closed finding-class vocabulary", d.Class)
		}
	} else {
		class = ""
	}

	reproSum := sha256.Sum256([]byte(sec.Digest + "\x00" + rule + "\x00" + strings.Join(scope, ",")))
	reproKey := hex.EncodeToString(reproSum[:])
	claim := core.SemanticClaim{
		ClaimID:             core.ClaimID(layerName, kind, sec.Subject, reproKey),
		ServiceID:           serviceID,
		Layer:               layerName,
		LayerVersion:        layerVersion,
		Kind:                kind,
		Subject:             sec.Subject,
		Rule:                rule,
		Scope:               scope,
		Tier:                tier,
		TierProfile:         tierProfile,
		Status:              core.ClaimFresh,
		Model:               c.Model,
		PromptVersion:       promptVersion,
		ReproKey:            reproKey,
		InputDigest:         sec.Digest,
		CompiledAtWatermark: watermark,
		CompiledAt:          at.UTC(),
		Class:               class,
	}
	links := []core.ClaimLink{
		{ClaimID: claim.ClaimID, Rel: core.RelCites, TargetRef: core.CiteRef(doc.Path, d.CiteStart, d.CiteEnd), TargetHash: hash, AnchorText: anchorText},
	}
	// SU-21 (ADR-041 decision 7): when the cites span resolves to exactly
	// one symbol definition, bind the claim to it too — a no-op (ok=false)
	// for the overwhelmingly common case of a claim citing prose, never a
	// hard failure either way.
	if anchorSymbol != nil {
		fetch := func(p string) ([]byte, bool) {
			if p == doc.Path {
				return doc.Content, true
			}
			return nil, false
		}
		if link, ok := anchorSymbol(claim.ClaimID, core.CiteRef(doc.Path, d.CiteStart, d.CiteEnd), fetch); ok {
			links = append(links, link)
		}
	}
	for _, g := range scope {
		links = append(links, core.ClaimLink{ClaimID: claim.ClaimID, Rel: core.RelScopes, TargetRef: g})
	}
	if audience != "" {
		links = append(links, core.ClaimLink{ClaimID: claim.ClaimID, Rel: core.RelAudience, TargetRef: audience})
	}
	// Remediation-drift substrate (FT-6, docs/vision/03-findings-and-experts.md
	// §2): for drift-tracked layers only (LC-2: serving.drift_tracked, not a
	// kind=="finding" Go hardcode — findings sets it true, obligations
	// doesn't set it and defaults to false), persist each scope-matched
	// file's CURRENT store fingerprint as a ClaimLink. No disk access needed
	// here —
	// the fingerprint is already maintained by the fusion pipeline's hash-gate
	// (SetFingerprint runs for every processed file at ingest) — this just
	// reads it at compile time as the honest "as of" anchor a later live
	// drift check re-reads (semantic.DriftedFindings, serve.go).
	//
	// Anchor persistence is LOUD, never best-effort: a read failure here
	// aborts the whole compile (errDriftAnchor, checked in the compile loop)
	// rather than skipping the link — the digest gate would never recompile
	// the unchanged section, so a silently missing anchor would permanently
	// blind remediation drift for that file. This is what upholds the FT-6
	// invariant: every stored claim of a drift-tracked layer carries ≥1
	// scope-fp link (scopes matching zero indexed files were already
	// rejected above).
	if driftTracked {
		files := make([]string, 0, len(scopeFiles))
		for f := range scopeFiles {
			files = append(files, f)
		}
		sort.Strings(files)
		for _, f := range files {
			fp, found, ferr := c.St.Fingerprint(ctx, serviceID, f)
			if ferr != nil {
				return core.SemanticClaim{}, nil, fmt.Errorf("%w: read fingerprint for %s: %v", errDriftAnchor, f, ferr)
			}
			if !found {
				// Violates the pipeline contract (every indexed file has a
				// fingerprint) — the store is in a state drift cannot be
				// honestly anchored against.
				return core.SemanticClaim{}, nil, fmt.Errorf("%w: indexed file %s has no stored fingerprint", errDriftAnchor, f)
			}
			links = append(links, core.ClaimLink{ClaimID: claim.ClaimID, Rel: core.RelScopeFP, TargetRef: f, TargetHash: fp})
		}
	}
	return claim, links, nil
}

// ─── BP-32: the directory scope floor (O23's root cause) ───────────────────

// instructionFileBases are the agent-instruction filenames whose rules bind
// the directory they sit in, not the repo. A nested one is a human writing
// "in THIS subtree, do X"; the compiler must not let an LLM restate it as a
// repo-wide rule.
//
// GEMINI.md (GL-6) is read as AGENTS.md is, and BUGBOT.md (GL-5) is scoped
// to its own directory (design doc §3), so both floor the same way.
var instructionFileBases = map[string]bool{"CLAUDE.md": true, "AGENTS.md": true, "GEMINI.md": true, "BUGBOT.md": true}

// scopeFloorDir returns the subtree a governing document's claims are floored
// to, or "" when the document imposes no floor. Only a NESTED instruction
// file floors: the root CLAUDE.md/AGENTS.md, ADR.md, docs/plans/README.md and
// every other governing doc keep whatever scope the model proposed (verified
// by the match/breadth checks as before).
func scopeFloorDir(docPath string) string {
	dir, base := path.Split(path.Clean(docPath))
	if !instructionFileBases[base] {
		return ""
	}
	dir = strings.Trim(dir, "/")
	// GL-5: Bugbot's own location, <dir>/.cursor/BUGBOT.md, governs <dir>.
	if base == bugbotFile {
		if dir == ".cursor" {
			return ""
		}
		dir = strings.TrimSuffix(dir, "/.cursor")
	}
	if dir == "" || dir == "." {
		return ""
	}
	return dir
}

// scopeFloor applies the directory scope floor to one draft's proposed scope
// globs (BP-32, docs/plans/bench-pyramid.md; O23: internal/CLAUDE.md's
// extractor rule was never compiled, and once it is, nothing stopped it from
// being scoped "**/*.go"). For a claim compiled from <dir>/CLAUDE.md:
//
//   - a glob already inside <dir>/ passes through unchanged;
//   - a glob whose first segment is a wildcard, or that names a bare file
//     ("**/*.go", "*_test.go", "goextract.go"), is PREFIXED with <dir>/ —
//     the model expressed a file class, the document says where it applies;
//   - a glob whose first segment is a literal directory OTHER than <dir>
//     ("cmd/**", "docs/plans/*.md") is a compile error, named per claim.
//     It is not silently widened and not silently re-homed: a rule scoped to
//     a subtree its own document does not govern is a wrong claim, not a
//     narrow one.
//
// Deterministic and LLM-free: same document path plus same globs, same
// result, on every machine and every recompile.
func scopeFloor(docPath string, globs []string) ([]string, error) {
	dir := scopeFloorDir(docPath)
	if dir == "" {
		return globs, nil
	}
	out := make([]string, 0, len(globs))
	for _, g := range globs {
		g = strings.TrimPrefix(strings.TrimSpace(g), "./")
		switch {
		case g == dir || strings.HasPrefix(g, dir+"/"):
			out = append(out, g)
		case namesAnotherDirectory(g):
			return nil, fmt.Errorf("scope %q is outside %s's own subtree %q — a nested instruction file governs only its own directory", g, docPath, dir)
		default:
			out = append(out, dir+"/"+g)
		}
	}
	return out, nil
}

// namesAnotherDirectory reports whether a glob explicitly names a directory
// path with a literal (wildcard-free) first segment — "cmd/**" does,
// "**/*.go" and "goextract.go" do not.
func namesAnotherDirectory(g string) bool {
	first, _, found := strings.Cut(g, "/")
	if !found {
		return false
	}
	return !strings.ContainsAny(first, "*?[{")
}

// errDriftAnchor marks a scope-fp drift-anchor persistence failure inside
// verifyDraft (FT-6). Unlike every other verifyDraft error — which rejects
// only the one draft (ClaimsRejected++) — this one is infrastructure, not a
// bad draft, and the compile loop aborts the whole run on it.
var errDriftAnchor = errors.New("semantic: finding drift anchor")

// ─── O19's fix: compile rejection-reason capture ───────────────────────────
//
// docs/dogfood-observations.md O19 (the first real platform module: 202 of
// 219 claims rejected, only a COUNT survived — the WHY was discarded the
// moment ClaimsRejected was incremented). The four causes below are the
// taxonomy docs/design/observability-surfaces.md §8 item 5 names; every one
// of verifyDraft's seven rejection sites is classified, so the compile loop
// can group counts AND keep a few example messages per cause without
// changing which drafts get rejected or why (behavior-preserving — only the
// bookkeeping is new).

// rejectionCause is one of the four O19 taxonomy buckets.
type rejectionCause string

const (
	// causeMalformed: the draft itself is shape-invalid (empty rule, a
	// rule over the word budget, an empty scope list) — nothing about the
	// corpus was even consulted yet.
	causeMalformed rejectionCause = "malformed"
	// causeAnchorMissing: the citation does not resolve inside the cited
	// document (out of range, or the hash/line-text extraction failed).
	causeAnchorMissing rejectionCause = "anchor-missing"
	// causeScopeUnverifiable: a scope glob matched zero indexed files.
	causeScopeUnverifiable rejectionCause = "scope-unverifiable"
	// causeOverBroad: a scope glob matched more than half the indexed
	// corpus and is not an exempt single-file-class extension glob.
	causeOverBroad rejectionCause = "over-broad"
	// causeInvalidClass (docs/design/findings-as-evidence.md §3.2, §6 item
	// 2): a kind==KindFinding draft's class is empty or not a member of
	// core.ValidFindingClasses — mirrors O19's four causes above, added as
	// a fifth for the findings-layer reviewer's own emit-time contract.
	// Never applies to a non-finding-kind draft (class is not required
	// there at all).
	causeInvalidClass rejectionCause = "invalid-class"
	// causeScopeOutOfSubtree (BP-32, O23): a claim compiled from a nested
	// instruction file (<dir>/CLAUDE.md) proposed a scope glob that names a
	// path outside <dir>/. Never silently widened and never silently
	// re-homed — the claim is rejected by name, because a rule a human
	// wrote about one subtree cannot bind another.
	causeScopeOutOfSubtree rejectionCause = "scope-out-of-subtree"
)

// claimRejectionError wraps a verifyDraft rejection with its classified
// cause; Unwrap makes it transparent to errors.Is/errors.As (in particular
// errors.Is(err, errDriftAnchor) at the compile loop's own call site, which
// must keep working unchanged — errDriftAnchor is never wrapped this way,
// since it is infrastructure, not a rejection).
type claimRejectionError struct {
	cause rejectionCause
	err   error
}

func (e *claimRejectionError) Error() string { return e.err.Error() }
func (e *claimRejectionError) Unwrap() error { return e.err }

// rejectErr builds a classified rejection error — every verifyDraft
// rejection site (never errDriftAnchor) returns through this.
func rejectErr(cause rejectionCause, format string, args ...any) error {
	return &claimRejectionError{cause: cause, err: fmt.Errorf(format, args...)}
}

// classifyRejectionCause recovers the cause a rejectErr call recorded.
// An error that was never built via rejectErr (defensive — every current
// verifyDraft rejection site goes through it, but a future site might not)
// defaults to causeMalformed rather than being silently dropped from the
// breakdown: an uncategorized rejection is still counted, just under the
// most conservative bucket.
func classifyRejectionCause(err error) rejectionCause {
	var rej *claimRejectionError
	if errors.As(err, &rej) {
		return rej.cause
	}
	return causeMalformed
}

// maxRejectionSamplesPerCause caps how many example rejection messages
// buildRejectionReasons keeps per cause — enough for an author to act on,
// never the full corpus (a 202-rejection run must not persist 202 strings).
const maxRejectionSamplesPerCause = 3

// buildRejectionReasons turns one compile run's accumulated cause→count and
// cause→samples maps into a deterministic, sorted breakdown (core.CompileRun.
// RejectionReasons). Pure: no I/O, no clock — callers accumulate the maps
// while looping and call this once, exactly like ClaimsEmitted is set once
// after the loop.
func buildRejectionReasons(counts map[rejectionCause]int, samples map[rejectionCause][]string) []core.RejectionBreakdown {
	if len(counts) == 0 {
		return nil
	}
	causes := make([]string, 0, len(counts))
	for c := range counts {
		causes = append(causes, string(c))
	}
	sort.Strings(causes)
	out := make([]core.RejectionBreakdown, 0, len(causes))
	for _, c := range causes {
		out = append(out, core.RejectionBreakdown{
			Cause:   c,
			Count:   counts[rejectionCause(c)],
			Samples: samples[rejectionCause(c)],
		})
	}
	return out
}

// isFileClassGlob reports whether g is a single-file-class extension glob:
// its final path segment is "*.<ext>" with exactly one literal extension and
// no further glob metacharacters (e.g. "**/*.go", "docs/**/*.md", "*.go";
// "*.pb.go" also qualifies — a compound suffix is a strict subset of one
// class). Such a glob is class-scoped by construction — the very shape
// prompt v3 mandates for legitimately repo-wide rules — so verifyDraft
// exempts it from the >50% over-broad breadth check. Bare "**", directory
// globs ("internal/**"), and multi-extension patterns ("*.{go,md}") are NOT
// class globs and stay subject to the breadth check.
func isFileClassGlob(g string) bool {
	seg := g
	if i := strings.LastIndex(g, "/"); i >= 0 {
		seg = g[i+1:]
	}
	if !strings.HasPrefix(seg, "*.") {
		return false
	}
	ext := seg[2:]
	if ext == "" {
		return false
	}
	return !strings.ContainsAny(ext, "*?[{}")
}

// isDecodeError reports a transport-level JSON decode failure — the CLI
// backend's "malformed response envelope" (a *json.SyntaxError or an
// unexpected EOF when the harness printed something that is not its JSON
// envelope, or was cut off). It is the same class as structured.ErrMalformed
// seen one layer down: one section's answer was unusable, the run goes on.
// The 2026-09-13 scenario dry cell aborted a 7-minute compile on the 88th
// section for exactly this.
func isDecodeError(err error) bool {
	var syn *json.SyntaxError
	var typ *json.UnmarshalTypeError
	return errors.As(err, &syn) || errors.As(err, &typ) || errors.Is(err, io.ErrUnexpectedEOF)
}

// compileRetryPause is the wait before a section's single retry.
const compileRetryPause = 3 * time.Second

// maxSectionErrors bounds how many failed sections a run names verbatim.
const maxSectionErrors = 10

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
