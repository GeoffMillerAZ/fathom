package llmegress_test

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	llmegress "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/egress"
	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
	prellmdlpgate "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/pre-llm-dlp-gate"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/tokenization"
)

// allowAll grants every detokenize request — test wiring only.
type allowAll struct{}

func (allowAll) Allow(tokenization.Principal, tokenization.TokenClass, string) bool { return true }

// newTestVault builds a MemoryVault with a fixed clock and a deterministic
// counter-based randomness source, so token values are stable per run order.
func newTestVault(t *testing.T) *tokenization.MemoryVault {
	t.Helper()
	clock := func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
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
	return tokenization.NewMemoryVault(tokenization.NewTokenizer(nil, clock, randFn), allowAll{}, clock)
}

const ssnSentinel = "123-45-6789"

// TestGateFromVaultSentinelNeverReachesInner proves the adapter's purpose
// end-to-end: a sensitive sentinel in the system prompt and message text is
// vaulted before the inner client sees anything, every occurrence becomes an
// opaque token, the tokenizations are audited, and an authorized principal
// can detokenize the original back out of the vault.
func TestGateFromVaultSentinelNeverReachesInner(t *testing.T) {
	vault := newTestVault(t)
	transformer, err := llmegress.GateFromVault(vault, "case-7", aiprovenance.NewHuman("tester"),
		llmegress.VaultRule{Pattern: `\b\d{3}-\d{2}-\d{4}\b`, Class: tokenization.ClassPII})
	if err != nil {
		t.Fatalf("GateFromVault: %v", err)
	}

	mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	client := llmegress.WithEgress(mock, transformer)

	req := llmclient.Request{
		System: "subject ssn is " + ssnSentinel,
		Messages: []llmclient.Message{{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
			llmclient.TextBlock("compare " + ssnSentinel + " against " + ssnSentinel + " please"),
		}}},
	}
	if _, err := client.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	rec := mock.Calls()[0].Request
	outbound := rec.System + "\n" + rec.Messages[0].Content[0].Text
	if strings.Contains(outbound, ssnSentinel) {
		t.Fatalf("sentinel reached the inner client: %q", outbound)
	}

	tokens := regexp.MustCompile(`tok_pii_[0-9a-f]+`).FindAllString(outbound, -1)
	if len(tokens) != 3 {
		t.Fatalf("found %d pii tokens in outbound text, want 3 (one per occurrence): %q", len(tokens), outbound)
	}
	distinct := map[string]bool{}
	for _, tok := range tokens {
		distinct[tok] = true
	}
	if len(distinct) != 3 {
		t.Errorf("distinct tokens = %d, want 3 (each occurrence tokenized individually)", len(distinct))
	}

	// Every tokenization is audited.
	var tokenizations int
	for _, ev := range vault.AuditLog() {
		if ev.Action == tokenization.ActionTokenize && ev.Allowed {
			tokenizations++
		}
	}
	if tokenizations != 3 {
		t.Errorf("audited tokenize events = %d, want 3", tokenizations)
	}

	// The vault wiring is faithful: an authorized principal recovers the
	// original value from any of the minted tokens.
	parsed, err := tokenization.Parse(tokens[0])
	if err != nil {
		t.Fatalf("Parse(%q): %v", tokens[0], err)
	}
	got, err := vault.Detokenize(parsed, tokenization.Principal{ID: "auditor"}, "test round-trip")
	if err != nil {
		t.Fatalf("Detokenize: %v", err)
	}
	if got != ssnSentinel {
		t.Errorf("Detokenize = %q, want %q", got, ssnSentinel)
	}
}

// TestGateFromVaultConstructorErrors pins the constructor's validation: bad
// wiring is rejected at construction, not discovered mid-call.
func TestGateFromVaultConstructorErrors(t *testing.T) {
	vault := newTestVault(t)
	author := aiprovenance.NewHuman("tester")
	okRule := llmegress.VaultRule{Pattern: `x`, Class: tokenization.ClassPII}

	cases := []struct {
		name      string
		vault     tokenization.Vault
		namespace string
		rules     []llmegress.VaultRule
	}{
		{name: "nil vault", vault: nil, namespace: "ns", rules: []llmegress.VaultRule{okRule}},
		{name: "empty namespace", vault: vault, namespace: "", rules: []llmegress.VaultRule{okRule}},
		{
			name: "invalid class", vault: vault, namespace: "ns",
			rules: []llmegress.VaultRule{{Pattern: `x`, Class: "bogus"}},
		},
		{
			name: "invalid pattern", vault: vault, namespace: "ns",
			rules: []llmegress.VaultRule{{Pattern: `(`, Class: tokenization.ClassPII}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := llmegress.GateFromVault(tc.vault, tc.namespace, author, tc.rules...); err == nil {
				t.Errorf("GateFromVault succeeded, want error")
			}
		})
	}
}

// failingVault is a Vault whose Tokenize always fails, to prove the gate
// fails closed on vault failure.
type failingVault struct{}

func (failingVault) Tokenize(string, tokenization.TokenClass, string, aiprovenance.Author) (tokenization.Token, error) {
	return tokenization.Token{}, errors.New("vault unavailable")
}

func (failingVault) Detokenize(tokenization.Token, tokenization.Principal, string) (string, error) {
	return "", errors.New("vault unavailable")
}

func (failingVault) AuditLog() []tokenization.AuditEvent { return nil }

// TestGateFromVaultFailsClosedOnVaultError proves a vault failure aborts the
// gated call: typed error out, zero inner invocations, nothing leaves.
func TestGateFromVaultFailsClosedOnVaultError(t *testing.T) {
	transformer, err := llmegress.GateFromVault(failingVault{}, "ns", aiprovenance.NewHuman("tester"),
		llmegress.VaultRule{Pattern: `\b\d{3}-\d{2}-\d{4}\b`, Class: tokenization.ClassPII})
	if err != nil {
		t.Fatalf("GateFromVault: %v", err)
	}

	mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	client := llmegress.WithEgress(mock, transformer)

	_, err = client.Complete(context.Background(), llmclient.Request{Prompt: "ssn " + ssnSentinel})
	var te *llmegress.TransformError
	if !errors.As(err, &te) {
		t.Fatalf("error = %v, want *TransformError", err)
	}
	if mock.CallCount() != 0 {
		t.Errorf("inner client invoked %d times after vault failure, want 0 (fail closed)", mock.CallCount())
	}
}

// TestGateFromDLPSentinelNeverReachesInner proves the DLP adapter: secret-
// shaped sentinels in the prompt and a tool_result are redacted to their
// labels before the inner client sees them.
func TestGateFromDLPSentinelNeverReachesInner(t *testing.T) {
	const (
		bearerSecret = "xyzzy-s3cr3t-token"
		cardSecret   = "4111 1111 1111 1111"
	)
	mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	client := llmegress.WithEgress(mock, llmegress.GateFromDLP(prellmdlpgate.MustDefaultGate()))

	req := llmclient.Request{
		Prompt: "use Authorization: Bearer " + bearerSecret + " for the call",
		Messages: []llmclient.Message{{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
			llmclient.ToolResultBlock("tu_1", "card on file: "+cardSecret, false),
		}}},
	}
	if _, err := client.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	rec := mock.Calls()[0].Request
	if strings.Contains(rec.Prompt, bearerSecret) {
		t.Errorf("bearer secret reached the inner client: %q", rec.Prompt)
	}
	if !strings.Contains(rec.Prompt, "[REDACTED:BEARER]") {
		t.Errorf("prompt missing redaction label: %q", rec.Prompt)
	}
	toolOut := rec.Messages[0].Content[0].Content
	if strings.Contains(toolOut, cardSecret) {
		t.Errorf("card number reached the inner client: %q", toolOut)
	}
	if !strings.Contains(toolOut, "[REDACTED:CARD]") {
		t.Errorf("tool_result missing redaction label: %q", toolOut)
	}
}

// TestGateFromDLPNilGateFailsClosed pins the documented nil-gate behavior:
// the transformer errors (aborting the gated call) instead of passing text
// through unredacted.
func TestGateFromDLPNilGateFailsClosed(t *testing.T) {
	mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	client := llmegress.WithEgress(mock, llmegress.GateFromDLP(nil))

	_, err := client.Complete(context.Background(), llmclient.Request{Prompt: "anything"})
	var te *llmegress.TransformError
	if !errors.As(err, &te) {
		t.Fatalf("error = %v, want *TransformError", err)
	}
	if mock.CallCount() != 0 {
		t.Errorf("inner client invoked %d times with a nil DLP gate, want 0 (fail closed)", mock.CallCount())
	}
}

// ─── F1: WithDeterministicTokens ─────────────────────────────────────────────

// TestGateFromVaultDeterministicSameValueSameToken proves the core F1
// guarantee: with WithDeterministicTokens, the same PII value in the same
// namespace produces the same opaque token across two separate Complete calls.
func TestGateFromVaultDeterministicSameValueSameToken(t *testing.T) {
	// Use a fixed HMAC key so DeterministicToken output is stable.
	hmacKey := []byte("test-hmac-key-32-bytes-long-pad0")
	clock := func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	var mu sync.Mutex
	var ctr byte
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
	vault := tokenization.NewMemoryVault(tokenization.NewTokenizer(hmacKey, clock, randFn), allowAll{}, clock)
	author := aiprovenance.NewHuman("tester")

	transformer, err := llmegress.GateFromVaultOpts(
		vault, "case-99", author,
		[]llmegress.VaultGateOption{llmegress.WithDeterministicTokens()},
		llmegress.VaultRule{Pattern: `\b\d{3}-\d{2}-\d{4}\b`, Class: tokenization.ClassPII},
	)
	if err != nil {
		t.Fatalf("GateFromVaultOpts: %v", err)
	}

	mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"), llmclient.MockEntryOK("ok"))
	client := llmegress.WithEgress(mock, transformer)

	ssn := "123-45-6789"
	req := llmclient.Request{Prompt: "ssn is " + ssn}

	if _, err := client.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete (call 1): %v", err)
	}
	if _, err := client.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete (call 2): %v", err)
	}

	calls := mock.Calls()
	tok1 := regexp.MustCompile(`tok_pii_[0-9a-f]+`).FindString(calls[0].Request.Prompt)
	tok2 := regexp.MustCompile(`tok_pii_[0-9a-f]+`).FindString(calls[1].Request.Prompt)
	if tok1 == "" || tok2 == "" {
		t.Fatalf("expected pii tokens in prompts; call1=%q call2=%q", calls[0].Request.Prompt, calls[1].Request.Prompt)
	}
	if tok1 != tok2 {
		t.Errorf("same SSN produced different tokens across calls: %q vs %q (want stable)", tok1, tok2)
	}
}

// TestGateFromVaultDeterministicDifferentNamespacesDifferentTokens proves
// namespace isolation: same value → different tokens in different namespaces.
func TestGateFromVaultDeterministicDifferentNamespacesDifferentTokens(t *testing.T) {
	hmacKey := []byte("test-hmac-key-32-bytes-long-pad0")
	clock := func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	var mu sync.Mutex
	var ctr byte
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
	author := aiprovenance.NewHuman("tester")
	rule := llmegress.VaultRule{Pattern: `\b\d{3}-\d{2}-\d{4}\b`, Class: tokenization.ClassPII}
	opts := []llmegress.VaultGateOption{llmegress.WithDeterministicTokens()}

	// Two vaults, two namespaces.
	vaultA := tokenization.NewMemoryVault(tokenization.NewTokenizer(hmacKey, clock, randFn), allowAll{}, clock)
	tA, err := llmegress.GateFromVaultOpts(vaultA, "ns-A", author, opts, rule)
	if err != nil {
		t.Fatalf("GateFromVaultOpts ns-A: %v", err)
	}
	vaultB := tokenization.NewMemoryVault(tokenization.NewTokenizer(hmacKey, clock, randFn), allowAll{}, clock)
	tB, err := llmegress.GateFromVaultOpts(vaultB, "ns-B", author, opts, rule)
	if err != nil {
		t.Fatalf("GateFromVaultOpts ns-B: %v", err)
	}

	ctx := context.Background()
	ssn := "123-45-6789"
	outA, errA := tA.TransformOutbound(ctx, "ssn "+ssn)
	outB, errB := tB.TransformOutbound(ctx, "ssn "+ssn)
	if errA != nil || errB != nil {
		t.Fatalf("TransformOutbound: A=%v B=%v", errA, errB)
	}
	tokA := regexp.MustCompile(`tok_pii_[0-9a-f]+`).FindString(outA)
	tokB := regexp.MustCompile(`tok_pii_[0-9a-f]+`).FindString(outB)
	if tokA == "" || tokB == "" {
		t.Fatalf("expected pii tokens; A=%q B=%q", outA, outB)
	}
	if tokA == tokB {
		t.Errorf("same SSN in different namespaces produced identical tokens %q — expected different tokens", tokA)
	}
}

// ─── F2: VaultActionBlock ────────────────────────────────────────────────────

// TestGateFromVaultBlockActionRefusesEgress proves that VaultActionBlock rules
// cause the Transformer to return *BlockedError and the inner client is never
// invoked (fail closed).
func TestGateFromVaultBlockActionRefusesEgress(t *testing.T) {
	vault := newTestVault(t)
	transformer, err := llmegress.GateFromVault(
		vault, "ns", aiprovenance.NewHuman("tester"),
		llmegress.VaultRule{
			Pattern: `\bPRIVILEGED\b`,
			Class:   tokenization.ClassPrivileged,
			Action:  llmegress.VaultActionBlock,
		},
	)
	if err != nil {
		t.Fatalf("GateFromVault: %v", err)
	}

	mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	client := llmegress.WithEgress(mock, transformer)

	_, err = client.Complete(context.Background(), llmclient.Request{Prompt: "this is PRIVILEGED information"})
	if err == nil {
		t.Fatal("Complete with blocked content should return error")
	}
	var be *llmegress.BlockedError
	if !errors.As(err, &be) {
		t.Fatalf("error = %v (%T), want *llmegress.BlockedError", err, err)
	}
	if be.RuleIndex != 0 {
		t.Errorf("BlockedError.RuleIndex = %d, want 0", be.RuleIndex)
	}
	if be.Class != tokenization.ClassPrivileged {
		t.Errorf("BlockedError.Class = %q, want %q", be.Class, tokenization.ClassPrivileged)
	}
	if !llmegress.IsBlocked(err) {
		t.Error("IsBlocked(err) = false, want true")
	}
	if mock.CallCount() != 0 {
		t.Errorf("inner client invoked %d times after block, want 0 (fail closed)", mock.CallCount())
	}
}

// TestGateFromVaultBlockActionNoMatchPassesThrough proves the block rule only
// fires when the pattern actually matches.
func TestGateFromVaultBlockActionNoMatchPassesThrough(t *testing.T) {
	vault := newTestVault(t)
	transformer, err := llmegress.GateFromVault(
		vault, "ns", aiprovenance.NewHuman("tester"),
		llmegress.VaultRule{
			Pattern: `\bPRIVILEGED\b`,
			Class:   tokenization.ClassPrivileged,
			Action:  llmegress.VaultActionBlock,
		},
	)
	if err != nil {
		t.Fatalf("GateFromVault: %v", err)
	}

	mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	client := llmegress.WithEgress(mock, transformer)

	// No "PRIVILEGED" in the prompt — should pass through.
	if _, err := client.Complete(context.Background(), llmclient.Request{Prompt: "nothing sensitive here"}); err != nil {
		t.Fatalf("Complete with clean content should succeed: %v", err)
	}
	if mock.CallCount() != 1 {
		t.Errorf("inner client invoked %d times, want 1", mock.CallCount())
	}
}

// TestGateFromVaultBlockAndTokenizeRulesCoexist proves that block and tokenize
// rules can coexist in one gate: block rule fires first (index 0), tokenize
// rule (index 1) never runs.
func TestGateFromVaultBlockAndTokenizeRulesCoexist(t *testing.T) {
	vault := newTestVault(t)
	transformer, err := llmegress.GateFromVault(
		vault, "ns", aiprovenance.NewHuman("tester"),
		llmegress.VaultRule{
			Pattern: `BLOCK_ME`,
			Class:   tokenization.ClassPrivileged,
			Action:  llmegress.VaultActionBlock,
		},
		llmegress.VaultRule{
			Pattern: `\b\d{3}-\d{2}-\d{4}\b`,
			Class:   tokenization.ClassPII,
			// Action defaults to tokenize
		},
	)
	if err != nil {
		t.Fatalf("GateFromVault: %v", err)
	}

	mock := llmclient.NewMockClient()
	client := llmegress.WithEgress(mock, transformer)

	_, err = client.Complete(context.Background(), llmclient.Request{
		Prompt: "ssn 123-45-6789 and BLOCK_ME marker",
	})
	var be *llmegress.BlockedError
	if !errors.As(err, &be) {
		t.Fatalf("expected BlockedError, got %v", err)
	}
	if be.RuleIndex != 0 {
		t.Errorf("wrong rule index %d, want 0", be.RuleIndex)
	}
}
