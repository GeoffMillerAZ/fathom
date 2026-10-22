package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

type reviewPhaseCancelBackend struct {
	refreshFake
	phase  string
	cancel context.CancelFunc
	after  []string
}

func (b *reviewPhaseCancelBackend) phaseRead(ctx context.Context, phase string) {
	if ctx.Err() != nil {
		b.after = append(b.after, phase)
	}
	if phase == b.phase {
		b.cancel()
	}
}

func (b *reviewPhaseCancelBackend) Obligations(ctx context.Context, _, _ string, _ int) (semantic.Obligations, error) {
	b.phaseRead(ctx, "obligations")
	return semantic.Obligations{}, nil
}
func (*reviewPhaseCancelBackend) ObligationsIndexStale(context.Context, string, string) bool {
	return false
}
func (b *reviewPhaseCancelBackend) ServiceMeta(ctx context.Context, _ string) (string, string, bool, error) {
	b.phaseRead(ctx, "metadata")
	return "", "", false, nil
}
func (b *reviewPhaseCancelBackend) SourcePaths(ctx context.Context, _ string) ([]string, error) {
	b.phaseRead(ctx, "paths")
	return []string{"a.go", "b.go"}, nil
}
func (b *reviewPhaseCancelBackend) ReadSourceFiles(ctx context.Context, _ string, paths []string) ([]core.SourceFile, error) {
	b.phaseRead(ctx, "source")
	return []core.SourceFile{{Path: paths[0], Content: []byte("package sample\n")}}, nil
}

func TestBuildReviewInputCancellationStopsBetweenPhases(t *testing.T) {
	testBuildReviewInputStopsBetweenPhases(t, false)
}

// Translate the cancellation signal into an injected expired deadline. No
// elapsed-time assertion or timer is needed to exercise the deadline path.
type reviewDeadlineContext struct{ context.Context }

func (c reviewDeadlineContext) Err() error {
	if c.Context.Err() != nil {
		return context.DeadlineExceeded
	}
	return nil
}

func TestBuildReviewInputDeadlineStopsBetweenPhases(t *testing.T) {
	testBuildReviewInputStopsBetweenPhases(t, true)
}

func testBuildReviewInputStopsBetweenPhases(t *testing.T, deadline bool) {
	t.Helper()
	for _, phase := range []string{"obligations", "metadata", "paths", "source"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var requestContext context.Context = ctx
			want := "canceled"
			if deadline {
				requestContext = reviewDeadlineContext{ctx}
				want = "deadline exceeded"
			}
			b := &reviewPhaseCancelBackend{phase: phase, cancel: cancel}
			_, _, err := buildReviewInput(requestContext, b, "sample", []string{"a.go", "b.go"}, "", "HEAD", nil)
			if err == nil || !strings.Contains(err.Message, want) {
				t.Fatalf("cancellation not named: %v", err)
			}
			if len(b.after) != 0 {
				t.Fatalf("reads after cancellation: %v", b.after)
			}
		})
	}
}
