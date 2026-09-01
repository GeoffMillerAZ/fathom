package expertchat

import (
	"context"
	"errors"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/sink"
)

func evidence(module, claim, tier, status string) EvidenceItem {
	return EvidenceItem{
		Module:        module,
		ModuleVersion: "v1",
		Watermark:     "manifest:" + claim,
		ClaimID:       claim,
		Tier:          tier,
		Status:        status,
		SourceRef:     "src/" + claim,
		Anchor:        "anchor for " + claim,
		Text:          "evidence text for " + claim,
	}
}

func mockEntryFor(items []EvidenceItem) llmclient.MockEntry {
	tokens := ""
	for i, it := range items {
		if i > 0 {
			tokens += ", "
		}
		tokens += citeToken(it)
	}
	return llmclient.MockEntryOK("ANSWER: composed answer.\nCITES: " + tokens)
}

func TestRun_RefusesNoEvidence(t *testing.T) {
	deps := Deps{Retriever: &FakeRetriever{}, Client: llmclient.NewMockClient()}
	_, err := Run(context.Background(), deps, "what is X?")
	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatalf("got err %v, want *Refusal", err)
	}
	if r.Reason != RefusalNoEvidence {
		t.Fatalf("got reason %v, want %v", r.Reason, RefusalNoEvidence)
	}
}

func TestRun_RefusesNoEvidence_AllNonCitableStatus(t *testing.T) {
	items := []EvidenceItem{
		evidence("m", "c1", "verified", StatusContradicted),
		evidence("m", "c2", "verified", StatusRefuted),
	}
	deps := Deps{Retriever: &FakeRetriever{Items: items}, Client: llmclient.NewMockClient()}
	_, err := Run(context.Background(), deps, "q")
	var r *Refusal
	if !errors.As(err, &r) || r.Reason != RefusalNoEvidence {
		t.Fatalf("got err %v, want *Refusal{Reason: RefusalNoEvidence}", err)
	}
}

func TestRun_RefusesBelowTierThreshold(t *testing.T) {
	items := []EvidenceItem{evidence("m", "c1", "assumed", StatusFresh)}
	deps := Deps{
		Retriever: &FakeRetriever{Items: items},
		Client:    llmclient.NewMockClient(),
		Policy:    Policy{TierThreshold: "verified"},
	}
	_, err := Run(context.Background(), deps, "q")
	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatalf("got err %v, want *Refusal", err)
	}
	if r.Reason != RefusalBelowTierThreshold {
		t.Fatalf("got reason %v, want %v", r.Reason, RefusalBelowTierThreshold)
	}
}

func TestRun_RefusesStaleOnlyEvidence(t *testing.T) {
	items := []EvidenceItem{evidence("m", "c1", "verified", StatusStale)}
	deps := Deps{
		Retriever: &FakeRetriever{Items: items},
		Client:    llmclient.NewMockClient(),
		Policy:    Policy{RequireFresh: true},
	}
	_, err := Run(context.Background(), deps, "q")
	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatalf("got err %v, want *Refusal", err)
	}
	if r.Reason != RefusalStaleOnlyEvidence {
		t.Fatalf("got reason %v, want %v", r.Reason, RefusalStaleOnlyEvidence)
	}
}

func TestRun_AllowsStaleEvidence_WhenRequireFreshIsFalse_HonestDefault(t *testing.T) {
	items := []EvidenceItem{evidence("m", "c1", "verified", StatusStale)}
	client := llmclient.NewMockClient(mockEntryFor(items))
	deps := Deps{Retriever: &FakeRetriever{Items: items}, Client: client} // Policy zero value: RequireFresh=false

	answer, err := Run(context.Background(), deps, "q")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := answer.Citations()
	if len(got) != 1 {
		t.Fatalf("got %d citations, want 1", len(got))
	}
	if got[0].Status != StatusStale {
		t.Fatalf("citation Status = %q, want %q — staleness must be flagged honestly, not hidden", got[0].Status, StatusStale)
	}
}

func TestRun_ComposesAnswerWithCitations_HappyPath(t *testing.T) {
	items := []EvidenceItem{
		evidence("agentic-kit", "c1", "verified", StatusFresh),
		evidence("agentic-kit", "c2", "doc-claimed", StatusFresh),
	}
	client := llmclient.NewMockClient(mockEntryFor(items))
	deps := Deps{Retriever: &FakeRetriever{Items: items}, Client: client}

	answer, err := Run(context.Background(), deps, "how does it work?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if answer.Text() != "composed answer." {
		t.Fatalf("Text() = %q", answer.Text())
	}
	citations := answer.Citations()
	if len(citations) != 2 {
		t.Fatalf("got %d citations, want 2", len(citations))
	}
	for _, c := range citations {
		if c.Tier == "" || c.Module == "" || c.ClaimID == "" || c.SourceRef == "" || c.Anchor == "" {
			t.Fatalf("citation missing a provenance field: %+v", c)
		}
	}
}

func TestRun_RejectsFabricatedCitation_RetriesOnceThenSucceeds(t *testing.T) {
	items := []EvidenceItem{evidence("m", "c1", "verified", StatusFresh)}
	client := llmclient.NewMockClient(
		llmclient.MockEntryOK("ANSWER: bad.\nCITES: m#does-not-exist"),
		mockEntryFor(items),
	)
	deps := Deps{Retriever: &FakeRetriever{Items: items}, Client: client}

	answer, err := Run(context.Background(), deps, "q")
	if err != nil {
		t.Fatalf("Run: %v (want success after one retry)", err)
	}
	if len(answer.Citations()) != 1 {
		t.Fatalf("got %d citations, want 1", len(answer.Citations()))
	}
	if got := client.CallCount(); got != 2 {
		t.Fatalf("model was called %d times, want exactly 2 (one retry)", got)
	}
}

func TestRun_RefusesAfterExhaustedRetries_FabricatedCitation(t *testing.T) {
	items := []EvidenceItem{evidence("m", "c1", "verified", StatusFresh)}
	client := llmclient.NewMockClient(
		llmclient.MockEntryOK("ANSWER: bad.\nCITES: m#does-not-exist"),
		llmclient.MockEntryOK("ANSWER: still bad.\nCITES: m#also-fake"),
	)
	deps := Deps{Retriever: &FakeRetriever{Items: items}, Client: client}

	_, err := Run(context.Background(), deps, "q")
	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatalf("got err %v, want *Refusal", err)
	}
	if r.Reason != RefusalFabricatedCitation {
		t.Fatalf("got reason %v, want %v", r.Reason, RefusalFabricatedCitation)
	}
	if got := client.CallCount(); got != 2 {
		t.Fatalf("model was called %d times, want exactly 2 (default MaxRetries=1)", got)
	}
}

// TestRun_ZeroCitationsFromModel_TreatedAsRejection pins the HONEST
// behavior for an explicit "CITES: (none)" response (critic finding 2): the
// model was shown eligible evidence and truthfully reported it doesn't
// answer the query. This must NOT be coerced into a retry (which would
// pressure the model into fabricating a citation just to satisfy the loop)
// and must NOT be reported as RefusalFabricatedCitation (nothing was
// fabricated) — it is an immediate RefusalEvidenceDoesNotAnswer, and the
// model is called exactly once.
func TestRun_ZeroCitationsFromModel_TreatedAsRejection(t *testing.T) {
	items := []EvidenceItem{evidence("m", "c1", "verified", StatusFresh)}
	client := llmclient.NewMockClient(
		llmclient.MockEntryOK("ANSWER: none of this evidence answers the question.\nCITES: (none)"),
	)
	deps := Deps{Retriever: &FakeRetriever{Items: items}, Client: client}

	_, err := Run(context.Background(), deps, "q")
	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatalf("got err %v, want *Refusal", err)
	}
	if r.Reason != RefusalEvidenceDoesNotAnswer {
		t.Fatalf("got reason %v, want %v", r.Reason, RefusalEvidenceDoesNotAnswer)
	}
	if got := client.CallCount(); got != 1 {
		t.Fatalf("model was called %d times, want exactly 1 — an honest \"(none)\" must never be retried", got)
	}
}

// TestRun_MixedFabrication_RejectsWholeResponse_RetriesThenSucceeds is the
// Run-level counterpart to TestResolveCitations_RejectsAnyFabricatedToken
// (critic finding 6): a response citing one real token plus one fabricated
// token must reject the WHOLE response (not silently keep the valid half),
// retry once, and succeed when the retry cites only real tokens.
func TestRun_MixedFabrication_RejectsWholeResponse_RetriesThenSucceeds(t *testing.T) {
	items := []EvidenceItem{
		evidence("m", "c1", "verified", StatusFresh),
		evidence("m", "c2", "verified", StatusFresh),
	}
	client := llmclient.NewMockClient(
		llmclient.MockEntryOK("ANSWER: bad.\nCITES: m#c1, m#fake"),
		mockEntryFor(items),
	)
	deps := Deps{Retriever: &FakeRetriever{Items: items}, Client: client}

	answer, err := Run(context.Background(), deps, "q")
	if err != nil {
		t.Fatalf("Run: %v (want success after one retry)", err)
	}
	if len(answer.Citations()) != 2 {
		t.Fatalf("got %d citations, want 2", len(answer.Citations()))
	}
	if got := client.CallCount(); got != 2 {
		t.Fatalf("model was called %d times, want exactly 2 (one retry)", got)
	}
}

func TestRun_RespectsExplicitMaxRetries(t *testing.T) {
	items := []EvidenceItem{evidence("m", "c1", "verified", StatusFresh)}
	client := llmclient.NewMockClient(
		llmclient.MockEntryOK("ANSWER: bad.\nCITES: m#fake"),
		llmclient.MockEntryOK("ANSWER: bad again.\nCITES: m#fake"),
		llmclient.MockEntryOK("ANSWER: bad again.\nCITES: m#fake"),
	)
	deps := Deps{Retriever: &FakeRetriever{Items: items}, Client: client, MaxRetries: 2}

	_, err := Run(context.Background(), deps, "q")
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if got := client.CallCount(); got != 3 {
		t.Fatalf("model was called %d times, want exactly 3 (MaxRetries=2 => 3 attempts)", got)
	}
}

func TestRun_MaxRetriesNegative_MeansSingleAttempt(t *testing.T) {
	items := []EvidenceItem{evidence("m", "c1", "verified", StatusFresh)}
	client := llmclient.NewMockClient(llmclient.MockEntryOK("ANSWER: bad.\nCITES: m#fake"))
	deps := Deps{Retriever: &FakeRetriever{Items: items}, Client: client, MaxRetries: -1}

	_, err := Run(context.Background(), deps, "q")
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if got := client.CallCount(); got != 1 {
		t.Fatalf("model was called %d times, want exactly 1 (MaxRetries=-1 => no retry)", got)
	}
}

func TestRun_ValidatesDeps(t *testing.T) {
	items := []EvidenceItem{evidence("m", "c1", "verified", StatusFresh)}
	validRetriever := &FakeRetriever{Items: items}
	validClient := llmclient.NewMockClient(mockEntryFor(items))

	cases := []struct {
		name  string
		deps  Deps
		query string
	}{
		{"nil retriever", Deps{Client: validClient}, "q"},
		{"nil client", Deps{Retriever: validRetriever}, "q"},
		{"empty query", Deps{Retriever: validRetriever, Client: validClient}, "   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Run(context.Background(), tc.deps, tc.query)
			if err == nil {
				t.Fatal("expected an error")
			}
			var r *Refusal
			if errors.As(err, &r) {
				t.Fatalf("got a *Refusal for a Deps-validation failure; want a plain operational error: %v", err)
			}
		})
	}
}

func TestRun_DeterministicRendering(t *testing.T) {
	items := []EvidenceItem{
		evidence("agentic-kit", "c2", "doc-claimed", StatusFresh),
		evidence("agentic-kit", "c1", "verified", StatusFresh),
	}

	run := func() string {
		client := llmclient.NewMockClient(mockEntryFor(items))
		deps := Deps{Retriever: &FakeRetriever{Items: items}, Client: client}
		answer, err := Run(context.Background(), deps, "how does it work?")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		return answer.Render()
	}

	first := run()
	second := run()
	if first != second {
		t.Fatalf("Render() not byte-identical across two independent Run calls with identical inputs:\nfirst=%q\nsecond=%q", first, second)
	}
}

func TestRun_EmitsTelemetry_OnAnswerAndOnRefusal(t *testing.T) {
	items := []EvidenceItem{evidence("m", "c1", "verified", StatusFresh)}

	t.Run("answer", func(t *testing.T) {
		mem := &sink.MemorySink{}
		client := llmclient.NewMockClient(mockEntryFor(items))
		deps := Deps{Retriever: &FakeRetriever{Items: items}, Client: client, Sink: mem}
		if _, err := Run(context.Background(), deps, "q"); err != nil {
			t.Fatalf("Run: %v", err)
		}
		if mem.Len() != 1 {
			t.Fatalf("got %d telemetry events, want 1", mem.Len())
		}
		if got := mem.Events()[0].Attributes["outcome"]; got != "answer" {
			t.Fatalf("event outcome = %q, want %q", got, "answer")
		}
	})

	t.Run("refusal", func(t *testing.T) {
		mem := &sink.MemorySink{}
		deps := Deps{Retriever: &FakeRetriever{}, Client: llmclient.NewMockClient(), Sink: mem}
		if _, err := Run(context.Background(), deps, "q"); err == nil {
			t.Fatal("expected a refusal")
		}
		if mem.Len() != 1 {
			t.Fatalf("got %d telemetry events, want 1", mem.Len())
		}
		ev := mem.Events()[0]
		if ev.Attributes["outcome"] != "refusal" || ev.Attributes["reason"] != string(RefusalNoEvidence) {
			t.Fatalf("unexpected event attributes: %+v", ev.Attributes)
		}
	})
}

func TestRun_NilSink_NoTelemetryPanic(t *testing.T) {
	items := []EvidenceItem{evidence("m", "c1", "verified", StatusFresh)}
	client := llmclient.NewMockClient(mockEntryFor(items))
	deps := Deps{Retriever: &FakeRetriever{Items: items}, Client: client} // Sink left nil
	if _, err := Run(context.Background(), deps, "q"); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// TestRun_RejectsUnrecognizedTierThreshold pins critic finding 3: a
// misspelled or made-up TierThreshold must fail loudly (a plain operational
// error, not silently treated as "no floor" — scoring.TierRank returns 0 for
// ANY unrecognized string, which would otherwise let every tier through).
func TestRun_RejectsUnrecognizedTierThreshold(t *testing.T) {
	cases := []string{"Verified", "high", "not-a-real-tier"}
	for _, threshold := range cases {
		t.Run(threshold, func(t *testing.T) {
			items := []EvidenceItem{evidence("m", "c1", "verified", StatusFresh)}
			deps := Deps{
				Retriever: &FakeRetriever{Items: items},
				Client:    llmclient.NewMockClient(),
				Policy:    Policy{TierThreshold: threshold},
			}
			_, err := Run(context.Background(), deps, "q")
			if err == nil {
				t.Fatalf("Policy.TierThreshold=%q: expected an error, got nil", threshold)
			}
			var r *Refusal
			if errors.As(err, &r) {
				t.Fatalf("Policy.TierThreshold=%q: got a *Refusal; want a plain operational error naming the bad policy: %v", threshold, err)
			}
		})
	}
}

// TestRun_AllowsAssumedTierThreshold_AsNoFloor proves scoring.TierAssumed
// remains a legitimate, deliberate no-floor choice (finding 3) even though
// its TierRank is 0, same as an unrecognized string — the validation must
// distinguish the two, not reject both.
func TestRun_AllowsAssumedTierThreshold_AsNoFloor(t *testing.T) {
	items := []EvidenceItem{evidence("m", "c1", "assumed", StatusFresh)}
	client := llmclient.NewMockClient(mockEntryFor(items))
	deps := Deps{
		Retriever: &FakeRetriever{Items: items},
		Client:    client,
		Policy:    Policy{TierThreshold: "assumed"},
	}
	if _, err := Run(context.Background(), deps, "q"); err != nil {
		t.Fatalf("Run: %v, want success — \"assumed\" threshold must not be rejected", err)
	}
}

// TestRun_DropsItemsWithForbiddenTokenChars pins critic finding 4(a)/(b): an
// item whose Module or ClaimID contains '#' (token-separator collision) or a
// comma/whitespace (breaks the CITES wire format) must never be offered to
// the model — it is dropped from the citable set rather than producing an
// ambiguous or permanently-uncitable token.
func TestRun_DropsItemsWithForbiddenTokenChars(t *testing.T) {
	cases := []struct {
		name string
		item EvidenceItem
	}{
		{"hash in ClaimID", evidence("m", "c#1", "verified", StatusFresh)},
		{"hash in Module", evidence("m#od", "c1", "verified", StatusFresh)},
		{"comma in ClaimID", evidence("m", "c1,c2", "verified", StatusFresh)},
		{"newline in ClaimID", evidence("m", "c1\nc2", "verified", StatusFresh)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps := Deps{Retriever: &FakeRetriever{Items: []EvidenceItem{tc.item}}, Client: llmclient.NewMockClient()}
			_, err := Run(context.Background(), deps, "q")
			var r *Refusal
			if !errors.As(err, &r) || r.Reason != RefusalNoEvidence {
				t.Fatalf("got err %v, want *Refusal{Reason: RefusalNoEvidence} (the only item is unsafe to cite)", err)
			}
		})
	}
}

// TestRun_DropsItemsWithEmptyTier pins critic finding 4(c): an item with an
// empty Tier passes Citable() but would always fail NewCitedAnswer if ever
// cited — it must be dropped before the model ever sees it, not discovered
// only after burning a retry.
func TestRun_DropsItemsWithEmptyTier(t *testing.T) {
	item := evidence("m", "c1", "", StatusFresh)
	deps := Deps{Retriever: &FakeRetriever{Items: []EvidenceItem{item}}, Client: llmclient.NewMockClient()}
	_, err := Run(context.Background(), deps, "q")
	var r *Refusal
	if !errors.As(err, &r) || r.Reason != RefusalNoEvidence {
		t.Fatalf("got err %v, want *Refusal{Reason: RefusalNoEvidence}", err)
	}
}

// TestRun_ErrorsOnDuplicateClaimToken pins critic finding 4: two distinct
// EvidenceItems sharing one (Module, ClaimID) pair is a module-host protocol
// violation (ClaimID is documented unique within its module) — resolving a
// cited token back to "whichever came first" would silently attach the
// WRONG provenance to a governed answer, so Run must fail loudly (a plain
// operational error) rather than guess.
func TestRun_ErrorsOnDuplicateClaimToken(t *testing.T) {
	first := evidence("m", "c1", "verified", StatusFresh)
	second := evidence("m", "c1", "doc-claimed", StatusFresh) // same (module, claim_id), different tier
	deps := Deps{Retriever: &FakeRetriever{Items: []EvidenceItem{first, second}}, Client: llmclient.NewMockClient()}

	_, err := Run(context.Background(), deps, "q")
	if err == nil {
		t.Fatal("expected an error for a duplicate claim token")
	}
	var r *Refusal
	if errors.As(err, &r) {
		t.Fatalf("got a *Refusal for a duplicate-token host protocol violation; want a plain operational error: %v", err)
	}
}

func TestRun_RetrieverError_IsPlainError(t *testing.T) {
	deps := Deps{Retriever: &FakeRetriever{Err: errors.New("host unreachable")}, Client: llmclient.NewMockClient()}
	_, err := Run(context.Background(), deps, "q")
	if err == nil {
		t.Fatal("expected an error")
	}
	var r *Refusal
	if errors.As(err, &r) {
		t.Fatal("a retriever transport failure must not be reported as a Refusal (it isn't an evidence-shaped outcome)")
	}
}
