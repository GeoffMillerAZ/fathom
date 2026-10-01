package semantic

import (
	"context"
	"fmt"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// EW-35: layer_coverage names why each uncompiled section is uncompiled.
func TestLayerCoverageNamesWhyEachSectionIsUncompiled(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	llm := &scriptedLLM{script: map[int]func() (llmclient.Response, error){
		2: func() (llmclient.Response, error) {
			return llmclient.Response{}, fmt.Errorf("claudecli: %w", context.DeadlineExceeded)
		},
	}}
	c := &Compiler{St: st, LLM: llm, Model: "mock-model", Now: fixedNow}
	in := fourDocs()
	run, err := c.Compile(ctx, in)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if got := run.UncompiledReasons["docs/b.md#b"]; got != core.UncompiledFailed {
		t.Fatalf("run reasons = %v, want docs/b.md#b failed", run.UncompiledReasons)
	}
	if len(run.UncompiledScopes) != 1 || run.UncompiledScopes[0] != "docs/b.md" {
		t.Fatalf("UncompiledScopes = %v, want the failed section's doc", run.UncompiledScopes)
	}

	layer, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatal(err)
	}
	grown := append(append([]core.SourceFile{}, in.Docs...), core.SourceFile{Path: "docs/e.md", Content: []byte("# E\n\n- Rule in e.\n")})
	cov, found, err := LiveLayerCoverage(ctx, st, "svc", layer, grown)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	want := map[string]string{"docs/b.md#b": core.UncompiledFailed, "docs/e.md#e": core.UncompiledCorpusGrew}
	if len(cov.UncompiledReasons) != len(want) {
		t.Fatalf("UncompiledReasons = %v, want %v", cov.UncompiledReasons, want)
	}
	for k, v := range want {
		if cov.UncompiledReasons[k] != v {
			t.Fatalf("UncompiledReasons = %v, want %v", cov.UncompiledReasons, want)
		}
	}
	if cov.CorpusGrew != 1 {
		t.Fatalf("CorpusGrew = %d, want 1 (a failed section is not growth)", cov.CorpusGrew)
	}

	// The store-only reading carries the recorded reasons too.
	base, _, err := ObligationsLayerCoverage(ctx, st, "svc", LayerObligations)
	if err != nil || base.UncompiledReasons["docs/b.md#b"] != core.UncompiledFailed {
		t.Fatalf("ObligationsLayerCoverage reasons = %v err=%v", base.UncompiledReasons, err)
	}
}

func TestUncompiledReasonsGroupsFirstWins(t *testing.T) {
	mk := func(subj string) orderedSection { return orderedSection{sec: Section{Subject: subj}} }
	got := uncompiledReasons(
		reasonedSections{core.UncompiledPrefiltered, []orderedSection{mk("p")}},
		reasonedSections{core.UncompiledFailed, []orderedSection{mk("f"), mk("p")}},
		reasonedSections{core.UncompiledBudget, []orderedSection{mk("b")}},
		reasonedSections{core.UncompiledRefused, []orderedSection{mk("r")}},
	)
	want := map[string]string{"p": "prefiltered", "f": "failed", "b": "budget", "r": "refused"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	if uncompiledReasons() != nil {
		t.Fatal("no groups must be nil")
	}
}
