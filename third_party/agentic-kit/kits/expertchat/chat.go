package expertchat

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	scoring "github.com/geoffmilleraz/agentic-kit/foundations/review/scoring"
	ev "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/sink"
)

// telemetrySource attributes the events Run emits when Deps.Sink is set.
const telemetrySource = "kits/expertchat"

// Deps are Run's dependencies. Retriever and Client are required; every
// other field has a documented zero-value default.
type Deps struct {
	// Retriever fetches candidate evidence. Required.
	Retriever Retriever

	// Client makes the model turn. Required.
	Client llmclient.LLMClient

	// Policy is the tier/staleness policy applied to retrieved evidence. The
	// zero value is the honest default — see Policy's doc.
	Policy Policy

	// Limit bounds how much evidence is requested from Retriever. Zero means
	// DefaultLimit.
	Limit int

	// MaxRetries bounds how many times Run re-prompts the model after a
	// citation-validation failure (fabricated or missing citations) before
	// refusing. Zero means DefaultMaxRetries (one retry, two attempts
	// total); a negative value is the explicit opt-out (zero retries, one
	// attempt only) — since zero itself means "use the default," a negative
	// sentinel is the only way to ask for no retries at all.
	MaxRetries int

	// Model names the model to request, passed through to
	// llmclient.Request.Model. Empty uses the client's configured default.
	Model string

	// MaxTokens caps the model response length. Zero uses the model default.
	MaxTokens int

	// Now is the injected clock, used only for telemetry event timestamps.
	// Nil means time.Now — the documented bare-default this kit allows
	// (AGENTS.md: "kits do this routinely").
	Now func() time.Time

	// Sink, when non-nil, receives one telemetry event per Run outcome
	// (answer composed, or refusal, with its reason). Nil disables
	// telemetry — the existence-check instrumentation named in
	// docs/vision/12-governed-knowledge-engine.md §2 law 7 is opt-in, wired
	// exactly like kits/agent-harness's Deps.Sink.
	Sink sink.Sink
}

// now returns d.Now, or time.Now if unset.
func (d Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// emit sends a best-effort telemetry event when d.Sink is set. Telemetry
// failures never affect Run's result — Emit's error is discarded, matching
// kits/agent-harness's own sink-is-observability-only contract.
func (d Deps) emit(outcome string, attrs map[string]string) {
	if d.Sink == nil {
		return
	}
	e := ev.Event{
		Kind:       ev.KindInfo,
		Source:     telemetrySource,
		Timestamp:  d.now(),
		Attributes: attrs,
	}
	if e.Attributes == nil {
		e.Attributes = map[string]string{}
	}
	e.Attributes["outcome"] = outcome
	e.Normalize()
	_ = d.Sink.Emit(e)
}

// Run is the governed answer loop: retrieve → filter by policy → compose
// via the model, citing only supplied evidence → return a CitedAnswer, or a
// *Refusal (wrapped as the returned error) when it will not answer. See the
// package doc for the full normative contract and refusal taxonomy.
//
// Run never returns a non-error CitedAnswer with zero citations —
// NewCitedAnswer enforces that at construction, so the only way Run can fail
// to compose an answer is to return a *Refusal (or a plain error for an
// operational failure: bad Deps, a Retriever/Client error).
func Run(ctx context.Context, deps Deps, query string) (CitedAnswer, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return CitedAnswer{}, fmt.Errorf("expertchat: query must not be empty")
	}
	if deps.Retriever == nil {
		return CitedAnswer{}, fmt.Errorf("expertchat: Deps.Retriever must not be nil")
	}
	if deps.Client == nil {
		return CitedAnswer{}, fmt.Errorf("expertchat: Deps.Client must not be nil")
	}
	if err := validateTierThreshold(deps.Policy.TierThreshold); err != nil {
		return CitedAnswer{}, err
	}

	limit := deps.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}

	items, err := deps.Retriever.Retrieve(ctx, query, limit)
	if err != nil {
		return CitedAnswer{}, fmt.Errorf("expertchat: retrieve: %w", err)
	}

	refuse := func(reason RefusalReason, detail string) (CitedAnswer, error) {
		r := &Refusal{Reason: reason, Query: query, Detail: detail}
		deps.emit("refusal", map[string]string{
			"reason": string(reason),
		})
		return CitedAnswer{}, r
	}

	if len(items) == 0 {
		return refuse(RefusalNoEvidence, fmt.Sprintf("the module host returned no evidence for query %q", query))
	}

	statusEligible := make([]EvidenceItem, 0, len(items))
	for _, it := range items {
		if it.Citable() {
			statusEligible = append(statusEligible, it)
		}
	}
	if len(statusEligible) == 0 {
		return refuse(RefusalNoEvidence, fmt.Sprintf(
			"the module host returned %d item(s) for query %q, but none is in a citable status (fresh/stale) — all are contradicted or refuted",
			len(items), query,
		))
	}

	// Identity hygiene (critic finding 4): an item with an empty or
	// token-unsafe Module/ClaimID/Tier can never be safely cited —
	// citeToken's "#"-join would collide across fields (a#b#c is ambiguous
	// between Module="a" ClaimID="b#c" and Module="a#b" ClaimID="c"), a
	// comma/whitespace/control char would make the item permanently
	// uncitable in the comma-separated CITES format (burning retries into a
	// misleading refusal), and a newline-bearing field could forge extra
	// prompt content. These are host data-quality problems, not the model's
	// fault — dropped here, silently as far as the model ever sees (it
	// never gets offered a token it structurally cannot cite), but NOT
	// silently as far as the caller is concerned: excluded items are
	// reflected honestly in the refusal detail below when they leave
	// nothing citable.
	citable := make([]EvidenceItem, 0, len(statusEligible))
	seenTokens := make(map[string]bool, len(statusEligible))
	for _, it := range statusEligible {
		if !validCiteIdentity(it) {
			continue
		}
		tok := citeToken(it)
		if seenTokens[tok] {
			// Two distinct claims sharing one (Module, ClaimID) pair is a
			// module-host protocol violation (docs/spec/expert-module-v0.md
			// §4: ClaimID unique WITHIN its module) — resolving citeToken
			// back to "whichever came first" would silently attach the
			// WRONG provenance to a governed answer. Fail loudly rather
			// than guess.
			return CitedAnswer{}, fmt.Errorf(
				"expertchat: module host returned duplicate claim token %q (module=%q claim_id=%q) — a module host must not emit two distinct claims with the same (module, claim_id) pair",
				tok, it.Module, it.ClaimID,
			)
		}
		seenTokens[tok] = true
		citable = append(citable, it)
	}
	if len(citable) == 0 {
		return refuse(RefusalNoEvidence, fmt.Sprintf(
			"%d item(s) were retrieved for query %q with a citable status, but all have an empty or unsafe-to-cite identity (module/claim_id/tier) and cannot be safely cited",
			len(statusEligible), query,
		))
	}

	tierEligible := filterByTier(citable, deps.Policy.TierThreshold)
	if len(tierEligible) == 0 {
		return refuse(RefusalBelowTierThreshold, fmt.Sprintf(
			"%d citable item(s) were retrieved for query %q, but none meets the required tier threshold %q",
			len(citable), query, deps.Policy.TierThreshold,
		))
	}

	eligible := tierEligible
	if deps.Policy.RequireFresh {
		eligible = filterFresh(tierEligible)
		if len(eligible) == 0 {
			return refuse(RefusalStaleOnlyEvidence, fmt.Sprintf(
				"%d tier-eligible item(s) were retrieved for query %q, but all are stale and Policy.RequireFresh is set",
				len(tierEligible), query,
			))
		}
	}

	maxRetries := DefaultMaxRetries
	switch {
	case deps.MaxRetries > 0:
		maxRetries = deps.MaxRetries
	case deps.MaxRetries < 0:
		maxRetries = 0 // explicit opt-out: single attempt, no retry
	}

	var correction string
	for attempt := 0; attempt <= maxRetries; attempt++ {
		prompt := buildPrompt(query, eligible, correction)
		resp, err := deps.Client.Complete(ctx, llmclient.Request{
			System:    systemPrompt,
			Prompt:    prompt,
			Model:     deps.Model,
			MaxTokens: deps.MaxTokens,
		})
		if err != nil {
			return CitedAnswer{}, fmt.Errorf("expertchat: model call: %w", err)
		}

		text, tokens, noneDeclared, parseErr := parseModelResponse(resp.Text)
		if parseErr != nil {
			correction = parseErr.Error()
			continue
		}
		if noneDeclared {
			// Honest "(none)": the model was shown eligible evidence and
			// truthfully reported that it doesn't answer the query. This is
			// NOT a fabrication (nothing was invented) and must not be
			// coerced into one by retrying with "cite something anyway" —
			// that would pressure the model into a dishonestly-grounded
			// answer, the exact failure mode this package exists to
			// prevent. Refuse immediately, no retry.
			return refuse(RefusalEvidenceDoesNotAnswer, fmt.Sprintf(
				"the model reported that none of the %d eligible evidence item(s) answers query %q; its stated reason: %s",
				len(eligible), query, text,
			))
		}
		citations, ok := resolveCitations(tokens, eligible)
		if !ok {
			correction = "you cited a token that was not in the exact list of tokens supplied, or cited none at all — cite ONLY the exact tokens shown to you, and cite at least one"
			continue
		}
		answer, err := NewCitedAnswer(text, citations)
		if err != nil {
			correction = err.Error()
			continue
		}
		deps.emit("answer", map[string]string{
			"citation_count": strconv.Itoa(len(answer.citations)),
			"attempt":        strconv.Itoa(attempt),
		})
		return answer, nil
	}

	return refuse(RefusalFabricatedCitation, fmt.Sprintf(
		"the model failed to cite only the %d supplied evidence token(s) after %d attempt(s); last rejection: %s",
		len(eligible), maxRetries+1, correction,
	))
}

// validateTierThreshold rejects a Policy.TierThreshold that scoring.TierRank
// cannot distinguish from "no floor." TierRank returns 0 both for the
// legitimate scoring.TierAssumed value AND for any unrecognized string (a
// typo like "Verified", or a made-up tier like "high") — silently treating
// the latter as "assumed" would mean a policy typo quietly disables the
// tier floor instead of failing loudly. Recognized non-zero-rank tiers
// (scoring.TierVerified/TierPreScan/TierDocClaimed/TierSMEStated, and their
// underscore variants) and the empty string (no floor at all) are always
// valid; scoring.TierAssumed is explicitly valid too — it remains a
// legitimate, deliberate no-floor choice. Everything else is rejected with
// a plain operational error: a policy typo must fail loudly, not silently
// admit every tier.
func validateTierThreshold(threshold scoring.Tier) error {
	if threshold == "" || threshold == scoring.TierAssumed {
		return nil
	}
	if scoring.TierRank(threshold) > 0 {
		return nil
	}
	return fmt.Errorf(
		"expertchat: Policy.TierThreshold %q is not a recognized evidence tier (want one of %q, %q, %q, %q, %q, or empty for no floor)",
		threshold, scoring.TierVerified, scoring.TierPreScan, scoring.TierDocClaimed, scoring.TierSMEStated, scoring.TierAssumed,
	)
}

// filterByTier returns the items ranking at or above threshold
// (scoring.TierRank). An empty threshold applies no floor.
func filterByTier(items []EvidenceItem, threshold scoring.Tier) []EvidenceItem {
	if threshold == "" {
		out := make([]EvidenceItem, len(items))
		copy(out, items)
		return out
	}
	min := scoring.TierRank(threshold)
	out := make([]EvidenceItem, 0, len(items))
	for _, it := range items {
		if scoring.TierRank(it.Tier) >= min {
			out = append(out, it)
		}
	}
	return out
}

// filterFresh returns only StatusFresh items.
func filterFresh(items []EvidenceItem) []EvidenceItem {
	out := make([]EvidenceItem, 0, len(items))
	for _, it := range items {
		if it.Status == StatusFresh {
			out = append(out, it)
		}
	}
	return out
}
