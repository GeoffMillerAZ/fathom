package tenancy

import (
	"context"
	"errors"
	"testing"
)

func TestRunUntilDoneReturnsFnResultWhenFnFinishesFirst(t *testing.T) {
	got, err := RunUntilDone(context.Background(), func() int { return 7 })
	if err != nil || got != 7 {
		t.Fatalf("got %d, %v, want 7, nil", got, err)
	}
}

func TestRunUntilDoneReturnsAtCtxDoneWhileFnBlocks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	release := make(chan struct{})
	defer close(release)
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := RunUntilDone(ctx, func() string { close(started); <-release; return "late" })
		done <- err
	}()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v, want context.Canceled", err)
	}
}

func TestRunUntilDoneDoesNotStartFnOnDoneCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RunUntilDone(ctx, func() int { return 1 })
	if err == nil {
		t.Fatal("want ctx error")
	}
}
