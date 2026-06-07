package llm_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/llm"
	"github.com/geoffmilleraz/sextant/internal/llmsec"
)

// newEnricher builds an Enricher backed by a MockClient for deterministic tests.
func newEnricher(entries ...llmclient.MockEntry) *llm.Enricher {
	mc := llmclient.NewMockClient(entries...)
	return llm.NewEnricher(mc, llmsec.NewRedactor(llmsec.DefaultPosture()))
}

func TestEnrich_Basic(t *testing.T) {
	sym := core.SymbolNode{
		Name:      "ValidateToken",
		Signature: "func ValidateToken(ctx context.Context, token string) error",
		FilePath:  "internal/auth/token.go",
		Kind:      core.KindGoFunc,
	}

	modelResp := `Summary: Validates an authentication token against the current context.
Tags: auth, token, security`

	e := newEnricher(llmclient.MockEntryOK(modelResp))
	card, err := e.Enrich(context.Background(), sym)
	if err != nil {
		t.Fatalf("Enrich error: %v", err)
	}

	if card.Symbol != sym.Name {
		t.Errorf("card.Symbol = %q, want %q", card.Symbol, sym.Name)
	}
	if card.File != sym.FilePath {
		t.Errorf("card.File = %q, want %q", card.File, sym.FilePath)
	}
	if !strings.Contains(card.Summary, "authentication token") {
		t.Errorf("card.Summary = %q, expected it to contain 'authentication token'", card.Summary)
	}
	if len(card.Tags) == 0 {
		t.Error("expected at least one tag")
	}
	found := false
	for _, tag := range card.Tags {
		if tag == "auth" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected tag 'auth' in %v", card.Tags)
	}
}

func TestEnrich_Error(t *testing.T) {
	boom := errors.New("backend unavailable")
	e := newEnricher(llmclient.MockEntryErr(boom))
	_, err := e.Enrich(context.Background(), core.SymbolNode{Name: "Foo"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, boom) {
		t.Errorf("expected wrapped boom error, got %v", err)
	}
}

func TestEnrich_SecretRedactedBeforeModelSees(t *testing.T) {
	// Signature contains a credential pattern that the DLP gate must strip.
	sym := core.SymbolNode{
		Name:      "SetupClient",
		Signature: `func SetupClient(apiKey="sk-secret999") *Client`,
		FilePath:  "internal/client/client.go",
		Kind:      core.KindGoFunc,
	}

	mc := llmclient.NewMockClient(llmclient.MockEntryOK("Summary: Sets up the client.\nTags: client"))
	e := llm.NewEnricher(mc, llmsec.NewRedactor(llmsec.DefaultPosture()))

	_, err := e.Enrich(context.Background(), sym)
	if err != nil {
		t.Fatalf("Enrich error: %v", err)
	}

	calls := mc.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	prompt := calls[0].Request.Prompt
	if strings.Contains(prompt, "sk-secret999") {
		t.Errorf("secret value leaked into prompt: %q", prompt)
	}
	// The prompt must contain the redaction marker so the model sees something
	// sensible instead of nothing.
	if !strings.Contains(prompt, "[REDACTED") {
		t.Errorf("expected [REDACTED marker in prompt, got: %q", prompt)
	}
}

func TestEnrichBatch_AllSucceed(t *testing.T) {
	syms := []core.SymbolNode{
		{Name: "Alpha", FilePath: "a.go", Kind: core.KindGoFunc},
		{Name: "Beta", FilePath: "b.go", Kind: core.KindGoFunc},
		{Name: "Gamma", FilePath: "c.go", Kind: core.KindGoFunc},
	}

	entries := []llmclient.MockEntry{
		llmclient.MockEntryOK("Summary: Alpha function.\nTags: alpha"),
		llmclient.MockEntryOK("Summary: Beta function.\nTags: beta"),
		llmclient.MockEntryOK("Summary: Gamma function.\nTags: gamma"),
	}
	e := newEnricher(entries...)

	cards, err := e.EnrichBatch(context.Background(), syms)
	if err != nil {
		t.Fatalf("EnrichBatch error: %v", err)
	}
	if len(cards) != len(syms) {
		t.Fatalf("expected %d cards, got %d", len(syms), len(cards))
	}
	for i, card := range cards {
		if card.Symbol != syms[i].Name {
			t.Errorf("cards[%d].Symbol = %q, want %q", i, card.Symbol, syms[i].Name)
		}
	}
}

func TestEnrichBatch_PartialError(t *testing.T) {
	syms := []core.SymbolNode{
		{Name: "Good", FilePath: "good.go"},
		{Name: "Bad", FilePath: "bad.go"},
	}
	entries := []llmclient.MockEntry{
		llmclient.MockEntryOK("Summary: Good.\nTags: ok"),
		llmclient.MockEntryErr(errors.New("transient")),
	}
	e := newEnricher(entries...)
	_, err := e.EnrichBatch(context.Background(), syms)
	if err == nil {
		t.Fatal("expected error from batch with one failing entry")
	}
}

func TestEnrich_EmptyResponse(t *testing.T) {
	e := newEnricher(llmclient.MockEntryOK(""))
	card, err := e.Enrich(context.Background(), core.SymbolNode{Name: "Empty", FilePath: "e.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should produce a card with empty summary and no tags — no panic.
	if card.Symbol != "Empty" {
		t.Errorf("card.Symbol = %q, want %q", card.Symbol, "Empty")
	}
}

func TestEnrich_TagsOnlyLine(t *testing.T) {
	resp := "Summary: Does something useful.\nTags: foo, bar, baz"
	e := newEnricher(llmclient.MockEntryOK(resp))
	card, err := e.Enrich(context.Background(), core.SymbolNode{Name: "X", FilePath: "x.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantTags := []string{"foo", "bar", "baz"}
	if len(card.Tags) != len(wantTags) {
		t.Fatalf("expected %d tags, got %d: %v", len(wantTags), len(card.Tags), card.Tags)
	}
	for i, tag := range wantTags {
		if card.Tags[i] != tag {
			t.Errorf("card.Tags[%d] = %q, want %q", i, card.Tags[i], tag)
		}
	}
}
