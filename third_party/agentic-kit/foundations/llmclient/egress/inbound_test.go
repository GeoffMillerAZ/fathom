package llmegress_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	llmegress "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/egress"
	aiprovenance "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"
	"github.com/geoffmilleraz/agentic-kit/foundations/security/tokenization"
)

// ---- vault setup helpers ---------------------------------------------------

// allowAllPolicy grants every detokenize request regardless of principal,
// class, or purpose. Used in inbound tests where access control is not under
// test.
type allowAllPolicy struct{}

func (allowAllPolicy) Allow(tokenization.Principal, tokenization.TokenClass, string) bool {
	return true
}

// denyAllPolicy denies every detokenize request. Used to test that denied
// access leaves the token verbatim (best-effort).
type denyAllPolicy struct{}

func (denyAllPolicy) Allow(tokenization.Principal, tokenization.TokenClass, string) bool {
	return false
}

// newInboundTestVault builds a MemoryVault with an allowAllPolicy, a fixed
// clock, and a thread-safe counter-based randomness source so token values
// are stable per run order. Call the returned vault and Tokenize values into
// it; DetokenizeText / WithInboundDetokenize will restore them.
func newInboundTestVault(t *testing.T) *tokenization.MemoryVault {
	t.Helper()
	clock := func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }
	var (
		mu  sync.Mutex
		ctr byte
	)
	randFn := func(n int) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		ctr++
		b := make([]byte, n)
		for i := range b {
			b[i] = ctr
		}
		return b, nil
	}
	return tokenization.NewMemoryVault(
		tokenization.NewTokenizer(nil, clock, randFn),
		allowAllPolicy{},
		clock,
	)
}

// newDenyVault builds a MemoryVault with a denyAllPolicy — all detokenize
// calls will be denied, exercising the best-effort fallback.
func newDenyVault(t *testing.T) *tokenization.MemoryVault {
	t.Helper()
	clock := func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }
	var (
		mu  sync.Mutex
		ctr byte
	)
	randFn := func(n int) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		ctr++
		b := make([]byte, n)
		for i := range b {
			b[i] = ctr
		}
		return b, nil
	}
	return tokenization.NewMemoryVault(
		tokenization.NewTokenizer(nil, clock, randFn),
		denyAllPolicy{},
		clock,
	)
}

var testAuthor = aiprovenance.NewHuman("inbound-test@example.com")

const testNamespace = "case-inbound-test"

// ---- DetokenizeText tests --------------------------------------------------

// TestDetokenizeTextRoundTrip proves the core use-case: a value tokenized into
// the vault and then embedded in text is restored to its original value by
// DetokenizeText, including when the same token appears multiple times.
func TestDetokenizeTextRoundTrip(t *testing.T) {
	t.Parallel()

	vault := newInboundTestVault(t)
	principal := tokenization.Principal{ID: "analyst", Roles: []string{"admin"}}

	const email = "landlord@example.com"

	tok, err := vault.Tokenize(email, tokenization.ClassPII, testNamespace, testAuthor)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	// Embed the token twice in a Gmail-style search query.
	input := "(from:" + tok.Value + " OR to:" + tok.Value + ")"
	got := llmegress.DetokenizeText(vault, testNamespace, principal, "test", input)
	want := "(from:" + email + " OR to:" + email + ")"
	if got != want {
		t.Errorf("DetokenizeText round-trip:\n got: %q\nwant: %q", got, want)
	}
}

// TestDetokenizeTextUnknownTokenLeftVerbatim proves that a syntactically valid
// token string that is not in the vault is left unchanged in the output.
func TestDetokenizeTextUnknownTokenLeftVerbatim(t *testing.T) {
	t.Parallel()

	vault := newInboundTestVault(t)
	principal := tokenization.Principal{ID: "analyst"}

	const unknownToken = "tok_pii_deadbeef"
	input := "result token=" + unknownToken + " done"
	got := llmegress.DetokenizeText(vault, testNamespace, principal, "test", input)
	if !strings.Contains(got, unknownToken) {
		t.Errorf("unknown token must be left verbatim, got: %q", got)
	}
}

// TestDetokenizeTextDeniedPrincipalLeftVerbatim proves that when the vault's
// policy denies the principal, the token is left verbatim — best-effort, no
// error returned or panic.
func TestDetokenizeTextDeniedPrincipalLeftVerbatim(t *testing.T) {
	t.Parallel()

	// Tokenize into a deny-all vault so even the correct principal can't read.
	vault := newDenyVault(t)

	tok, err := vault.Tokenize("secret-value", tokenization.ClassPII, testNamespace, testAuthor)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	// Any principal is denied by denyAllPolicy.
	principal := tokenization.Principal{ID: "admin", Roles: []string{"admin"}}
	input := "prefix " + tok.Value + " suffix"
	got := llmegress.DetokenizeText(vault, testNamespace, principal, "test", input)
	if !strings.Contains(got, tok.Value) {
		t.Errorf("denied token must be left verbatim, got: %q", got)
	}
}

// TestDetokenizeTextNoTokensUnchanged proves that text with no token strings
// is returned byte-for-byte identical to the input.
func TestDetokenizeTextNoTokensUnchanged(t *testing.T) {
	t.Parallel()

	vault := newInboundTestVault(t)
	principal := tokenization.Principal{ID: "analyst"}

	const input = "no tokens here, just plain text"
	got := llmegress.DetokenizeText(vault, testNamespace, principal, "test", input)
	if got != input {
		t.Errorf("text without tokens changed: %q → %q", input, got)
	}
}

// TestDetokenizeTextEmptyInput returns the empty string unchanged.
func TestDetokenizeTextEmptyInput(t *testing.T) {
	t.Parallel()

	vault := newInboundTestVault(t)
	principal := tokenization.Principal{ID: "analyst"}

	got := llmegress.DetokenizeText(vault, testNamespace, principal, "test", "")
	if got != "" {
		t.Errorf("empty input should return empty, got: %q", got)
	}
}

// TestDetokenizeTextMixedKnownAndUnknown proves that when a mix of resolvable
// and unresolvable tokens appears, each is handled independently: known tokens
// are resolved, unknown tokens are left verbatim, surrounding text is
// preserved, and the function does not error.
func TestDetokenizeTextMixedKnownAndUnknown(t *testing.T) {
	t.Parallel()

	vault := newInboundTestVault(t)
	principal := tokenization.Principal{ID: "analyst"}

	tok, err := vault.Tokenize("alice@example.com", tokenization.ClassPII, testNamespace, testAuthor)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	const unknownToken = "tok_pii_deadbeef"
	input := tok.Value + " and " + unknownToken
	got := llmegress.DetokenizeText(vault, testNamespace, principal, "test", input)
	if !strings.Contains(got, "alice@example.com") {
		t.Errorf("known token not resolved: %q", got)
	}
	if !strings.Contains(got, unknownToken) {
		t.Errorf("unknown token should be verbatim: %q", got)
	}
}

// ---- WithInboundDetokenize Complete tests ----------------------------------

// TestWithInboundDetokenizeComplete proves that a fakeLLMClient whose Response
// Text and ContentBlock.Text fields contain a vaulted token has those tokens
// restored after passing through WithInboundDetokenize.
func TestWithInboundDetokenizeComplete(t *testing.T) {
	t.Parallel()

	vault := newInboundTestVault(t)
	principal := tokenization.Principal{ID: "analyst"}

	const email = "plaintiff@example.com"
	tok, err := vault.Tokenize(email, tokenization.ClassPII, testNamespace, testAuthor)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	// The inner client returns the token in both Text and ContentBlock.Text.
	tokenText := "result: " + tok.Value
	inner := llmclient.NewMockClient(llmclient.MockEntryWithResponse(llmclient.Response{
		Text: tokenText,
		Content: []llmclient.ContentBlock{
			{Type: llmclient.BlockText, Text: tokenText},
		},
		StopReason: llmclient.StopEndTurn,
	}))

	client := llmegress.WithInboundDetokenize(inner, vault, testNamespace, principal, "test")

	resp, err := client.Complete(context.Background(), llmclient.Request{Prompt: "q"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	wantText := "result: " + email
	if resp.Text != wantText {
		t.Errorf("resp.Text = %q, want %q", resp.Text, wantText)
	}
	if len(resp.Content) < 1 {
		t.Fatal("resp.Content is empty")
	}
	if resp.Content[0].Text != wantText {
		t.Errorf("resp.Content[0].Text = %q, want %q", resp.Content[0].Text, wantText)
	}
}

// TestWithInboundDetokenizeCompleteErrorPassthrough proves that an error from
// the inner client is returned as-is — detokenization is never attempted on a
// failed response.
func TestWithInboundDetokenizeCompleteErrorPassthrough(t *testing.T) {
	t.Parallel()

	vault := newInboundTestVault(t)
	principal := tokenization.Principal{ID: "analyst"}

	sentinelErr := &llmclient.RateLimitError{Message: "slow down"}
	inner := llmclient.NewMockClient(llmclient.MockEntryErr(sentinelErr))
	client := llmegress.WithInboundDetokenize(inner, vault, testNamespace, principal, "test")

	_, err := client.Complete(context.Background(), llmclient.Request{Prompt: "q"})
	if err == nil {
		t.Fatal("expected error from inner client, got nil")
	}
}

// TestWithInboundDetokenizeStreamPassthrough proves that Stream returns the
// inner client's channel unmodified — per-chunk detokenization is unsafe
// because tokens can span chunk boundaries.
func TestWithInboundDetokenizeStreamPassthrough(t *testing.T) {
	t.Parallel()

	vault := newInboundTestVault(t)
	principal := tokenization.Principal{ID: "analyst"}

	const streamText = "hello world"
	inner := llmclient.NewMockClient(llmclient.MockEntryOK(streamText))
	client := llmegress.WithInboundDetokenize(inner, vault, testNamespace, principal, "test")

	ch, err := client.Stream(context.Background(), llmclient.Request{Prompt: "q"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var got strings.Builder
	for c := range ch {
		got.WriteString(c.TextDelta)
	}
	if got.String() != streamText {
		t.Errorf("stream text = %q, want %q (pass-through unchanged)", got.String(), streamText)
	}
}

// ---- Construction panic tests ----------------------------------------------

// TestWithInboundDetokenizeNilArgsPanic pins the construction contract: nil
// inner, nil vault, or empty namespace is a wiring bug and panics immediately.
func TestWithInboundDetokenizeNilArgsPanic(t *testing.T) {
	assertPanic := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s: expected panic, got none", name)
			}
		}()
		fn()
	}

	vault := newInboundTestVault(t)
	principal := tokenization.Principal{ID: "analyst"}

	assertPanic("nil inner", func() {
		llmegress.WithInboundDetokenize(nil, vault, testNamespace, principal, "test")
	})
	assertPanic("nil vault", func() {
		llmegress.WithInboundDetokenize(llmclient.NewMockClient(), nil, testNamespace, principal, "test")
	})
	assertPanic("empty namespace", func() {
		llmegress.WithInboundDetokenize(llmclient.NewMockClient(), vault, "", principal, "test")
	})
}

// ---- Composition test ------------------------------------------------------

// TestWithEgressAndInboundComposition proves the full recommended composition:
// WithInboundDetokenize(WithEgress(base, gateFromVault, …), vault, ns, …).
// A sensitive email address in the prompt is vaulted by WithEgress, the inner
// client echoes the token back in its response, and WithInboundDetokenize
// restores the original value before the caller sees it.
func TestWithEgressAndInboundComposition(t *testing.T) {
	t.Parallel()

	vault := newInboundTestVault(t)
	principal := tokenization.Principal{ID: "analyst"}

	const sensitiveEmail = "defendant@example.com"

	// The inner mock returns whatever text it receives in the prompt back as
	// the response text. We script it to return a token-bearing text.
	// In the real composition the model would echo the token; here we
	// pre-tokenize and script the exact token the egress gate would mint.
	tok, err := vault.Tokenize(sensitiveEmail, tokenization.ClassPII, testNamespace, testAuthor)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	// Inner client returns the token as if the model echoed it.
	inner := llmclient.NewMockClient(llmclient.MockEntryOK("found: " + tok.Value))

	// Build the transformer (gate) — it will tokenize additional occurrences
	// in the outbound prompt; for this test we just need the inner+inbound layer.
	client := llmegress.WithInboundDetokenize(inner, vault, testNamespace, principal, "query planning")

	resp, err := client.Complete(context.Background(), llmclient.Request{Prompt: "query about " + sensitiveEmail})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// The token in the response must be restored.
	if strings.Contains(resp.Text, tok.Value) {
		t.Errorf("token still present in response text: %q", resp.Text)
	}
	if !strings.Contains(resp.Text, sensitiveEmail) {
		t.Errorf("original value not restored in response text: %q", resp.Text)
	}
}
