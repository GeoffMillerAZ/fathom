package semantic

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// TestCompile_SizingEstimateSkipsPrefilterOnDryRunOnly pins EW-77 (G12): a
// dry run under WithSizingEstimate asks the decision port nothing and counts
// every section, while a spending compile under the same mark still asks
// the prefilter (the mark is a dry run's only).
func TestCompile_SizingEstimateSkipsPrefilterOnDryRunOnly(t *testing.T) {
	st, err := store.NewSQLiteStore("")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	keepBoth := decision.MockEntry{Response: decision.Response{
		Provider: "mock-decide",
		Answers: map[string]decision.Answer{
			"docs/twosections.md#rule-a": {ID: "docs/twosections.md#rule-a", Kind: decision.KindPredicate, Noul: 0.9, Confidence: 0.9},
			"docs/twosections.md#rule-b": {ID: "docs/twosections.md#rule-b", Kind: decision.KindPredicate, Noul: 0.9, Confidence: 0.9},
		},
	}}
	docs := []core.SourceFile{{Path: "docs/twosections.md", Content: []byte(twoRuleDoc)}}
	ctx := WithSizingEstimate(context.Background())

	dryDecider := decision.NewMockDecider("mock-decide", keepBoth)
	dry := &Compiler{St: st, Model: "mock-model", Now: fixedNow, Decider: dryDecider, PrefilterThreshold: 0.5}
	run, err := dry.Compile(ctx, CompileInput{ServiceID: "svc", Docs: docs, IndexedFiles: indexedFiles, Watermark: "manifest:aaa", DryRun: true})
	if err != nil {
		t.Fatalf("dry Compile: %v", err)
	}
	if n := len(dryDecider.Calls()); n != 0 {
		t.Errorf("a sizing dry run asked the prefilter %d time(s), want none", n)
	}
	if run.SectionsCompiled != 2 || run.SectionsPrefiltered != 0 {
		t.Errorf("sizing dry run: sections_compiled %d prefiltered %d, want 2 and 0 (every section, before the prefilter)", run.SectionsCompiled, run.SectionsPrefiltered)
	}

	spendDecider := decision.NewMockDecider("mock-decide", keepBoth)
	empty := llmclient.MockEntry{Response: llmclient.Response{Text: `{"claims":[]}`, Usage: llmclient.Usage{InputTokens: 60, OutputTokens: 5}}}
	llmMock := llmclient.NewMockClient(empty, empty)
	spend := &Compiler{St: st, LLM: llmMock, Model: "mock-model", Now: fixedNow, Decider: spendDecider, PrefilterThreshold: 0.5}
	if _, err := spend.Compile(ctx, CompileInput{ServiceID: "svc", Docs: docs, IndexedFiles: indexedFiles, Watermark: "manifest:aaa"}); err != nil {
		t.Fatalf("spending Compile: %v", err)
	}
	if n := len(spendDecider.Calls()); n != 1 {
		t.Errorf("a spending compile under the sizing mark asked the prefilter %d time(s), want 1 (the mark is a dry run's only)", n)
	}
}
