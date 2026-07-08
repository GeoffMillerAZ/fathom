package semantic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/cost"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/llmsec"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// PromptVersion identifies the obligations compile prompt. Bump on any prompt
// change — it is part of every claim's provenance.
const PromptVersion = "obligations/v1"

// LayerObligations is the v1 tracer-bullet layer (ADR-016 §4.8 #1).
const LayerObligations = "obligations"

// KindObligation is the v1 claim kind.
const KindObligation = "obligation"

// maxRuleWords budgets each compiled rule so a 6-rule payload stays inside
// the ≤150-token serving budget (ADR-016 §4.5). Enforced at compile time —
// the serving path never truncates meaning.
const maxRuleWords = 28

// claimDraft is the strict-JSON shape the LLM must emit per claim.
type claimDraft struct {
	Rule      string   `json:"rule"`
	Scope     []string `json:"scope"`
	CiteStart int      `json:"cite_start"` // 1-based document line
	CiteEnd   int      `json:"cite_end"`   // inclusive
}

// Compiler compiles governing docs into obligations claims. The LLM client
// is injected (llmclient.NewMockClient in tests — network-free and
// reproducible); the Redactor is the mandatory DLP gate on every prompt.
type Compiler struct {
	St       store.Store
	LLM      llmclient.LLMClient
	Redactor *llmsec.Redactor
	Model    string // recorded in provenance; "" uses the client default

	// USD per 1k tokens for the cost ledger; zero rates record honest zeros.
	InRatePerKTok  float64
	OutRatePerKTok float64

	Now func() time.Time // injected clock (repo convention)
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

// Compile runs the obligations compile: split → digest gate → DLP → LLM
// (strict JSON) → anchor + scope verification → transactional store, then
// supersedes claims of recompiled subjects that were not re-emitted. The
// returned CompileRun has already been recorded to the store (unless DryRun).
func (c *Compiler) Compile(ctx context.Context, in CompileInput) (core.CompileRun, error) {
	if c.St == nil {
		return core.CompileRun{}, fmt.Errorf("semantic: Compiler.St is required")
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	started := now()

	run := core.CompileRun{
		ServiceID:       in.ServiceID,
		Layer:           LayerObligations,
		CorpusWatermark: in.Watermark,
		Model:           c.Model,
		PromptVersion:   PromptVersion,
		DryRun:          in.DryRun,
		RanAt:           started.UTC(),
	}
	idSum := sha256.Sum256([]byte(in.ServiceID + "\x00" + LayerObligations + "\x00" + in.Watermark + "\x00" + started.UTC().Format(time.RFC3339Nano)))
	run.RunID = hex.EncodeToString(idSum[:])

	// Existing claims, grouped by subject, for the digest gate + supersede.
	existing, err := c.St.ClaimsForService(ctx, in.ServiceID, LayerObligations)
	if err != nil {
		return run, fmt.Errorf("semantic: load existing claims: %w", err)
	}
	bySubject := map[string][]core.SemanticClaim{}
	for _, cl := range existing {
		bySubject[cl.Subject] = append(bySubject[cl.Subject], cl)
	}

	// Deterministic doc order.
	docs := append([]core.SourceFile(nil), in.Docs...)
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })

	var (
		newClaims          []core.SemanticClaim
		newLinks           []core.ClaimLink
		recompiledSubjects []string
	)

	for _, doc := range docs {
		sections := SplitMarkdownSections(doc.Path, doc.Content)
		run.SectionsTotal += len(sections)
		for _, sec := range sections {
			// Digest gate: an unchanged section keeps its claims untouched.
			if prior := bySubject[sec.Subject]; len(prior) > 0 && allDigestsMatch(prior, sec.Digest) {
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
			if err != nil {
				return run, fmt.Errorf("semantic: compile %s: %w", sec.Subject, err)
			}
			run.SectionsCompiled++
			run.TokensIn += usage.InputTokens
			run.TokensOut += usage.OutputTokens
			recompiledSubjects = append(recompiledSubjects, sec.Subject)

			for _, d := range drafts {
				claim, links, verr := c.verifyDraft(in.ServiceID, d, sec, doc, in.IndexedFiles, in.Watermark, started)
				if verr != nil {
					run.ClaimsRejected++
					continue
				}
				newClaims = append(newClaims, claim)
				newLinks = append(newLinks, links...)
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

	// Supersede: prior claims of recompiled subjects that were not re-emitted.
	emitted := map[string]bool{}
	for _, cl := range newClaims {
		emitted[cl.ClaimID] = true
	}
	sort.Strings(recompiledSubjects)
	for _, subj := range recompiledSubjects {
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
// for dry-run estimates.
const promptOverheadTokens = 260

func allDigestsMatch(claims []core.SemanticClaim, digest string) bool {
	for _, cl := range claims {
		if cl.InputDigest != digest {
			return false
		}
	}
	return true
}

// compileSection runs one section through DLP + LLM and parses strict JSON.
func (c *Compiler) compileSection(ctx context.Context, sec Section) ([]claimDraft, llmclient.Usage, error) {
	prompt := buildSectionPrompt(sec)
	if c.Redactor != nil {
		prompt, _ = c.Redactor.Redact(prompt)
	}
	resp, err := c.LLM.Complete(ctx, llmclient.Request{
		Model:     c.Model,
		System:    systemPrompt,
		Prompt:    prompt,
		MaxTokens: 1200,
	})
	if err != nil {
		return nil, llmclient.Usage{}, err
	}
	drafts, err := parseDrafts(resp.Text)
	if err != nil {
		return nil, resp.Usage, err
	}
	return drafts, resp.Usage, nil
}

const systemPrompt = `You compile engineering obligations from governing documents into strict JSON.
An obligation is a rule a developer editing matching files MUST follow, restated
faithfully from the document — never invented. Output ONLY a JSON array (possibly
empty) of objects: {"rule", "scope", "cite_start", "cite_end"}.
- rule: imperative, <= 28 words, self-contained.
- scope: gitignore-style glob(s) for the code paths the rule governs. Use narrow
  globs the document justifies; if the document names no paths, use ["**"].
- cite_start/cite_end: the 1-based DOCUMENT line range (shown in the input) that
  states the rule. Cite the narrowest range that fully supports the rule.
Only emit rules the cited lines actually state. No commentary.`

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

// parseDrafts parses the model's strict-JSON array, tolerating a fenced code
// block wrapper (the one deviation models actually make).
func parseDrafts(text string) ([]claimDraft, error) {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "```") {
		t = strings.TrimPrefix(t, "```json")
		t = strings.TrimPrefix(t, "```")
		if i := strings.LastIndex(t, "```"); i >= 0 {
			t = t[:i]
		}
		t = strings.TrimSpace(t)
	}
	var drafts []claimDraft
	if err := json.Unmarshal([]byte(t), &drafts); err != nil {
		return nil, fmt.Errorf("strict-JSON parse: %w", err)
	}
	return drafts, nil
}

// verifyDraft applies compile-time verification (ADR-016 §4.4): the citation
// must resolve inside the document, and every scope glob must match at least
// one indexed file. Failures reject the claim — a claim is only as good as
// its anchors.
func (c *Compiler) verifyDraft(serviceID string, d claimDraft, sec Section, doc core.SourceFile, indexedFiles []string, watermark string, at time.Time) (core.SemanticClaim, []core.ClaimLink, error) {
	rule := strings.TrimSpace(d.Rule)
	if rule == "" {
		return core.SemanticClaim{}, nil, fmt.Errorf("empty rule")
	}
	if n := len(strings.Fields(rule)); n > maxRuleWords {
		return core.SemanticClaim{}, nil, fmt.Errorf("rule exceeds %d-word budget (%d)", maxRuleWords, n)
	}
	// Citation must lie within this section's document range and hash.
	if d.CiteStart < sec.StartLine || d.CiteEnd > sec.EndLine || d.CiteEnd < d.CiteStart {
		return core.SemanticClaim{}, nil, fmt.Errorf("citation outside section range")
	}
	hash, ok := core.HashLines(doc.Content, d.CiteStart, d.CiteEnd)
	if !ok {
		return core.SemanticClaim{}, nil, fmt.Errorf("citation range does not resolve")
	}
	// Scope globs must be non-empty and each match >= 1 indexed file.
	scope := append([]string(nil), d.Scope...)
	sort.Strings(scope)
	if len(scope) == 0 {
		return core.SemanticClaim{}, nil, fmt.Errorf("empty scope")
	}
	for _, g := range scope {
		m := filter.New([]string{g})
		matched := false
		for _, f := range indexedFiles {
			if m.Match(f) {
				matched = true
				break
			}
		}
		if !matched {
			return core.SemanticClaim{}, nil, fmt.Errorf("scope %q matches no indexed file", g)
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
		PromptVersion:       PromptVersion,
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
