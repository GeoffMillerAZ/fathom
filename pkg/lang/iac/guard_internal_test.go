package iac

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/iacfacts"
)

// block returns a reader that does not return until release is closed.
func block(release chan struct{}) func() (*iacfacts.ResourceSet, error) {
	return func() (*iacfacts.ResourceSet, error) {
		<-release
		return &iacfacts.ResourceSet{}, nil
	}
}

func waitIdle(t *testing.T) {
	t.Helper()
	for i := 0; i < 500 && abandoned.Abandoned() != 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if n := abandoned.Abandoned(); n != 0 {
		t.Fatalf("%d abandoned reads did not finish", n)
	}
}

// The read guarantee (ADR-092): a read that overruns its deadline returns ErrDeadline and is abandoned, and once
// GOMAXPROCS abandoned reads are running a read starts nothing and returns ErrBusy until they finish.
func TestDeadlineAndBusy(t *testing.T) {
	waitIdle(t)
	release := make(chan struct{})
	start := time.Now()
	_, err := guarded(context.Background(), config{deadline: 30 * time.Millisecond}, block(release))
	if !errors.Is(err, ErrDeadline) {
		t.Fatalf("an overrunning read: want ErrDeadline, got %v", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("the read returned after %v, not by its deadline", d)
	}
	for i := 1; i < runtime.GOMAXPROCS(0); i++ {
		if _, err := guarded(context.Background(), config{deadline: 10 * time.Millisecond}, block(release)); !errors.Is(err, ErrDeadline) {
			t.Fatalf("read %d: want ErrDeadline, got %v", i, err)
		}
	}
	ran := false
	_, err = ReadFile(context.Background(), "a.tf", []byte("resource \"a\" \"b\" {}\n"))
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("with GOMAXPROCS abandoned reads running: want ErrBusy, got %v", err)
	}
	_, err = guarded(context.Background(), config{deadline: time.Second}, func() (*iacfacts.ResourceSet, error) { ran = true; return nil, nil })
	if !errors.Is(err, ErrBusy) || ran {
		t.Fatalf("a busy read must not start: err %v, ran %v", err, ran)
	}
	close(release)
	waitIdle(t)
	if _, err := ReadFile(context.Background(), "a.tf", []byte("resource \"a\" \"b\" {}\n")); err != nil {
		t.Fatalf("after the abandoned reads finished: %v", err)
	}
}

func TestContextEndsTheRead(t *testing.T) {
	waitIdle(t)
	release := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := guarded(ctx, config{deadline: time.Minute}, block(release))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want the context's error, got %v", err)
	}
	close(release)
	waitIdle(t)
}

func TestPanicIsTheReadsFailureAndEchoesNothing(t *testing.T) {
	waitIdle(t)
	_, err := guarded(context.Background(), config{deadline: time.Second}, func() (*iacfacts.ResourceSet, error) {
		panic("SECRET-IN-PANIC")
	})
	if err == nil || err.Error() != "iac: the reader failed on this input" {
		t.Fatalf("got %v", err)
	}
}
