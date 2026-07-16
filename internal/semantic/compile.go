package semantic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

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
					"cite_end": {"type": "integer"}
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
func (c *Compiler) Compile(ctx context.Context, in CompileInput) (core.CompileRun, error) {
	if c.St == nil {
		return core.CompileRun{}, fmt.Errorf("semantic: Compiler.St is required")
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	started := now()

	promptVersion := obligationsPromptVersion()

	run := core.CompileRun{
		ServiceID:       in.ServiceID,
		Layer:           LayerObligations,
		CorpusWatermark: in.Watermark,
		Model:           c.Model,
		PromptVersion:   promptVersion,
		DryRun:          in.DryRun,
		RanAt:           started.UTC(),
	}
	idSum := sha256.Sum256([]byte(in.ServiceID + "\x00" + LayerObligations + "\x00" + in.Watermark + "\x00" + started.UTC().Format(time.RFC3339Nano)))
	run.RunID = hex.EncodeToString(idSum[:])

	// Existing claims, grouped by subject, for the digest gate + supersede.
	// Superseded and refuted rows are EXCLUDED: they keep their old
	// InputDigest/PromptVersion forever, so one such row would make
	// allDigestsMatch false for its subject on every future run — the
	// section re-LLMs for the rest of time (F2). They need no supersede
	// handling either: superseded is already terminal for the sweep, and a
	// refuted claim keeps its stronger human verdict (neither is served).
	existing, err := c.St.ClaimsForService(ctx, in.ServiceID, LayerObligations)
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

	// Budget gate, pre-flight (ADR-016 §4.1, M1-7): a real (non-dry-run)
	// compile refuses to start when its own digest-gate-aware cost estimate
	// exceeds the layer's budget — before any LLM call is made.
	if !in.DryRun && c.Layer.Name != "" {
		estTokensIn := 0
		for _, ds := range all {
			for _, sec := range ds.sections {
				if prior := bySubject[sec.Subject]; len(prior) > 0 && allDigestsMatch(prior, sec.Digest, promptVersion) {
					continue
				}
				estTokensIn += cost.Estimate(sec.Text) + promptOverheadTokens
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

sectionLoop:
	for _, ds := range all {
		doc := ds.doc
		for _, sec := range ds.sections {
			// Digest gate: an unchanged section (same content AND prompt
			// identity) keeps its claims untouched.
			if prior := bySubject[sec.Subject]; len(prior) > 0 && allDigestsMatch(prior, sec.Digest, promptVersion) {
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

			drafts, usage, err := c.compileSection(ctx, sec)
			run.TokensIn += usage.InputTokens
			run.TokensOut += usage.OutputTokens
			if err != nil {
				// A malformed/truncated model response poisons only its own
				// section: count it, keep the section's prior claims, and
				// move on — one bad response must never abort an 88-section
				// corpus. Anything else (auth, network, context) aborts.
				if errors.Is(err, structured.ErrTruncated) || errors.Is(err, structured.ErrMalformed) {
					run.SectionsFailed++
					continue
				}
				return run, fmt.Errorf("semantic: compile %s: %w", sec.Subject, err)
			}
			run.SectionsCompiled++
			recompiledSubjects = append(recompiledSubjects, sec.Subject)

			for _, d := range drafts {
				claim, links, verr := c.verifyDraft(in.ServiceID, d, sec, doc, in.IndexedFiles, in.Watermark, started, promptVersion)
				if verr != nil {
					run.ClaimsRejected++
					continue
				}
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
				break sectionLoop
			}
		}
	}
	run.ClaimsEmitted = len(newClaims)
	run.CostUSD = cost.PriceUSD(run.TokensIn, c.InRatePerKTok) + cost.PriceUSD(run.TokensOut, c.OutRatePerKTok)
	run.DurationMS = now().Sub(started).Milliseconds()

	if in.DryRun {
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
				if err := c.St.SetClaimStatus(ctx, in.ServiceID, old.ClaimID, core.ClaimSuperseded); err != nil {
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
// sections kept v1 claims). promptVersion is the content-addressed identity
// from obligationsPromptVersion (M1-4/M1-5) — no more package-level const.
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
	if estCost := cost.PriceUSD(tokensIn, inRatePerKTok); estCost > layer.Budgets.MaxUSDPerRun {
		return true, fmt.Sprintf("estimated cost $%.4f exceeds layer %q budget max_usd_per_run=$%.4f", estCost, layer.Name, layer.Budgets.MaxUSDPerRun)
	}
	return false, ""
}

// compileSection runs one section through DLP + LLM via structured.Complete
// (M1-6), which sets Request.OutputSchema to claimDraftSchema and classifies
// the two silent failure modes as errors.Is-matchable sentinels
// (structured.ErrTruncated, structured.ErrMalformed) instead of this package
// hand-classifying them via the retired errBadResponse wrapper.
func (c *Compiler) compileSection(ctx context.Context, sec Section) ([]claimDraft, llmclient.Usage, error) {
	prompt := buildSectionPrompt(sec)
	if c.Redactor != nil {
		prompt, _ = c.Redactor.Redact(prompt)
	}
	var envelope claimDraftEnvelope
	resp, err := structured.Complete(ctx, c.LLM, llmclient.Request{
		Model:     c.Model,
		System:    obligationsSystemPromptTemplate.Text,
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
// its anchors.
func (c *Compiler) verifyDraft(serviceID string, d claimDraft, sec Section, doc core.SourceFile, indexedFiles []string, watermark string, at time.Time, promptVersion string) (core.SemanticClaim, []core.ClaimLink, error) {
	rule := strings.TrimSpace(d.Rule)
	if rule == "" {
		return core.SemanticClaim{}, nil, fmt.Errorf("empty rule")
	}
	if n, max := len(strings.Fields(rule)), c.maxRuleWords(); n > max {
		return core.SemanticClaim{}, nil, fmt.Errorf("rule exceeds %d-word budget (%d)", max, n)
	}
	// Citation must lie within this section's document range and hash.
	if d.CiteStart < sec.StartLine || d.CiteEnd > sec.EndLine || d.CiteEnd < d.CiteStart {
		return core.SemanticClaim{}, nil, fmt.Errorf("citation outside section range")
	}
	hash, ok := core.HashLines(doc.Content, d.CiteStart, d.CiteEnd)
	if !ok {
		return core.SemanticClaim{}, nil, fmt.Errorf("citation range does not resolve")
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
	// would structurally contradict each other).
	scope := append([]string(nil), d.Scope...)
	sort.Strings(scope)
	if len(scope) == 0 {
		return core.SemanticClaim{}, nil, fmt.Errorf("empty scope")
	}
	for _, g := range scope {
		m := filter.New([]string{g})
		matched := 0
		for _, f := range indexedFiles {
			if m.Match(f) {
				matched++
			}
		}
		if matched == 0 {
			return core.SemanticClaim{}, nil, fmt.Errorf("scope %q matches no indexed file", g)
		}
		if matched*2 > len(indexedFiles) && !isFileClassGlob(g) {
			return core.SemanticClaim{}, nil, fmt.Errorf("scope %q is over-broad: matches %d/%d indexed files (>50%%) and is not a single-file-class extension glob, rejected as a catch-all scope", g, matched, len(indexedFiles))
		}
	}

	reproSum := sha256.Sum256([]byte(sec.Digest + "\x00" + rule + "\x00" + strings.Join(scope, ",")))
	reproKey := hex.EncodeToString(reproSum[:])
	claim := core.SemanticClaim{
		ClaimID:             core.ClaimID(LayerObligations, KindObligation, sec.Subject, reproKey),
		ServiceID:           serviceID,
		Layer:               LayerObligations,
		LayerVersion:        "v1",
		Kind:                KindObligation,
		Subject:             sec.Subject,
		Rule:                rule,
		Scope:               scope,
		Tier:                core.TierDocClaimed,
		TierProfile:         "engineering",
		Status:              core.ClaimFresh,
		Model:               c.Model,
		PromptVersion:       promptVersion,
		ReproKey:            reproKey,
		InputDigest:         sec.Digest,
		CompiledAtWatermark: watermark,
		CompiledAt:          at.UTC(),
	}
	links := []core.ClaimLink{
		{ClaimID: claim.ClaimID, Rel: core.RelCites, TargetRef: core.CiteRef(doc.Path, d.CiteStart, d.CiteEnd), TargetHash: hash},
	}
	for _, g := range scope {
		links = append(links, core.ClaimLink{ClaimID: claim.ClaimID, Rel: core.RelScopes, TargetRef: g})
	}
	return claim, links, nil
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
