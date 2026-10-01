package semantic

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// scriptedLLM is a fake LLMClient whose nth Complete call (1-based) runs
// script[n]; an absent entry answers one valid claim. Never a real model.
type scriptedLLM struct {
	mu     sync.Mutex
	calls  int
	script map[int]func() (llmclient.Response, error)
}

func (s *scriptedLLM) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	s.mu.Lock()
	s.calls++
	n := s.calls
	fn := s.script[n]
	s.mu.Unlock()
	if fn != nil {
		return fn()
	}
	return llmclient.Response{Text: fmt.Sprintf(`{"claims":[{"rule":"Rule number %d applies.","scope":["internal/mcp/**"],"cite_start":3,"cite_end":3}]}`, n)}, nil
}

func (s *scriptedLLM) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return nil, fmt.Errorf("scriptedLLM: Stream unused")
}

func fourDocs() CompileInput {
	var docs []core.SourceFile
	for _, n := range []string{"a", "b", "c", "d"} {
		docs = append(docs, core.SourceFile{Path: "docs/" + n + ".md", Content: []byte("# " + strings.ToUpper(n) + "\n\n- Rule in " + n + ".\n")})
	}
	return CompileInput{ServiceID: "svc", Docs: docs, IndexedFiles: indexedFiles, Watermark: "manifest:aaa"}
}

// EW-33: a per-call deadline from the claude-cli subprocess (the caller's
// ctx is still live) fails ONE section and the run continues.
func TestCompilePerCallDeadlineCountsSectionFailedAndContinues(t *testing.T) {
	st := store.NewMemoryStore()
	llm := &scriptedLLM{script: map[int]func() (llmclient.Response, error){
		2: func() (llmclient.Response, error) {
			return llmclient.Response{}, fmt.Errorf("claudecli: call timed out: %w", context.DeadlineExceeded)
		},
	}}
	c := &Compiler{St: st, LLM: llm, Model: "mock-model", Now: fixedNow}
	run, err := c.Compile(context.Background(), fourDocs())
	if err != nil {
		t.Fatalf("a per-call deadline aborted the run: %v", err)
	}
	if run.SectionsCompiled != 3 || run.SectionsFailed != 1 {
		t.Fatalf("compiled/failed = %d/%d, want 3/1", run.SectionsCompiled, run.SectionsFailed)
	}
	if len(run.SectionErrors) != 1 || !strings.Contains(run.SectionErrors[0], "docs/b.md") {
		t.Fatalf("SectionErrors = %v, want the failed section named", run.SectionErrors)
	}
	claims, _ := st.ClaimsForService(context.Background(), "svc", LayerObligations)
	if len(claims) != 3 {
		t.Fatalf("stored %d claims, want 3", len(claims))
	}
}

// EW-33: only the caller's ctx aborts, and no compiled section is dropped.
func TestCompileCallerCancelFlushesCompiledSections(t *testing.T) {
	st := store.NewMemoryStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	llm := &scriptedLLM{script: map[int]func() (llmclient.Response, error){
		3: func() (llmclient.Response, error) {
			cancel()
			return llmclient.Response{}, ctx.Err()
		},
	}}
	c := &Compiler{St: st, LLM: llm, Model: "mock-model", Now: fixedNow}
	run, err := c.Compile(ctx, fourDocs())
	if err == nil {
		t.Fatal("a cancelled caller ctx must return an error")
	}
	claims, _ := st.ClaimsForService(context.Background(), "svc", LayerObligations)
	if len(claims) != 2 {
		t.Fatalf("stored %d claims after cancel, want sections 1-2's (2)", len(claims))
	}
	if run.SectionsCompiled != 2 {
		t.Fatalf("SectionsCompiled = %d, want 2", run.SectionsCompiled)
	}
	runs, _ := st.CompileRuns(context.Background(), "svc", LayerObligations)
	if len(runs) != 1 {
		t.Fatalf("compile_runs rows = %d, want the aborted run recorded", len(runs))
	}
}

func TestCallerAborted(t *testing.T) {
	live := context.Background()
	if callerAborted(live, context.DeadlineExceeded) {
		t.Fatal("a deadline error with a live ctx is a per-call failure, not an abort")
	}
	if callerAborted(live, context.Canceled) {
		t.Fatal("a canceled error with a live ctx is a per-call failure, not an abort")
	}
	dead, cancel := context.WithCancel(context.Background())
	cancel()
	if !callerAborted(dead, fmt.Errorf("x")) {
		t.Fatal("a dead caller ctx must abort")
	}
}
