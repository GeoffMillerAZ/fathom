package lang_test

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// hangingExtractor reads a file whose first line is "block" by waiting on
// release, ignoring its context the way a parser stuck in a loop does. Any
// other file reads as one symbol.
type hangingExtractor struct {
	once    sync.Once
	release chan struct{}
	started atomic.Int64
	calls   atomic.Int64
}

func (e *hangingExtractor) Extract(_ context.Context, f lang.File) (lang.Extracted, error) {
	e.calls.Add(1)
	if strings.HasPrefix(string(f.Src), "block") {
		e.started.Add(1)
		<-e.release
	}
	return lang.Extracted{Symbols: []lang.Symbol{{Name: "S", Package: "p", Kind: "RowDef", Line: 1, Tier: "verified"}}}, nil
}

func (e *hangingExtractor) unblock() { e.once.Do(func() { close(e.release) }) }

func hangingReader(t *testing.T, deadline time.Duration) (*lang.Reader, *hangingExtractor) {
	t.Helper()
	e := &hangingExtractor{release: make(chan struct{})}
	t.Cleanup(e.unblock)
	r := newReader(t, lang.WithLimits(lang.Limits{ReadDeadline: deadline}), lang.WithExtractor(rowMatch("row-spec"), e))
	return r, e
}

func TestExtractReturnsByTheDeadlineAndSaysSo(t *testing.T) {
	r, e := hangingReader(t, 100*time.Millisecond)
	start := time.Now()
	res, err := r.Extract(context.Background(), lang.File{Path: "a.row", Src: []byte("block\n")})
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 10*time.Second {
		t.Fatalf("Extract took %v against a 100ms deadline", took)
	}
	if !res.Incomplete || len(res.Symbols) != 0 || len(res.Facts) != 0 || len(res.Notes) != 1 || !strings.HasPrefix(res.Notes[0], "read-deadline:") || !strings.Contains(res.Notes[0], "100ms") {
		t.Fatalf("an overrun reads Incomplete with one note naming the deadline and no facts: %+v", res)
	}
	if res.Dialect != "" || res.Level != "" || res.Skipped != "" {
		t.Fatalf("an overrun names no dialect and no level (none answered), and is not a skip: %+v", res)
	}
	// The next file still reads: one overrun does not end the Reader.
	e.unblock()
	if got := extract(t, r, "b.row", "ok\n"); got.Incomplete || len(got.Symbols) != 1 {
		t.Fatalf("the Reader after an overrun: %+v", got)
	}
}

func TestExtractContextEndAbandonsTheRead(t *testing.T) {
	r, e := hangingReader(t, 0)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for e.started.Load() == 0 {
			time.Sleep(time.Millisecond)
		}
		cancel()
	}()
	_, err := r.Extract(ctx, lang.File{Path: "a.row", Src: []byte("block\n")})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Extract returned %v, want the context's error", err)
	}
}

func TestAbandonedReadsPastTheCapStartNothingAndRecover(t *testing.T) {
	prev := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(prev)
	r, e := hangingReader(t, 30*time.Millisecond)
	for i := 0; i < 2; i++ {
		res, err := r.Extract(context.Background(), lang.File{Path: "a.row", Src: []byte("block\n")})
		if err != nil || !res.Incomplete || res.Skipped != "" {
			t.Fatalf("overrun %d: %+v %v", i, res, err)
		}
	}
	before := e.calls.Load()
	res, err := r.Extract(context.Background(), lang.File{Path: "b.row", Src: []byte("ok\n")})
	if err != nil || res.Skipped != lang.SkipBusy || len(res.Notes) != 1 || !strings.HasPrefix(res.Notes[0], "busy:") || len(res.Symbols) != 0 {
		t.Fatalf("past the cap the Reader starts nothing: %+v %v", res, err)
	}
	if e.calls.Load() != before {
		t.Fatal("a skipped file reached the extractor")
	}
	e.unblock()
	deadline := time.Now().Add(10 * time.Second)
	for {
		got, err := r.Extract(context.Background(), lang.File{Path: "b.row", Src: []byte("ok\n")})
		if err == nil && got.Skipped == "" && len(got.Symbols) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the Reader did not recover when the abandoned reads finished: %+v %v", got, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCloseDoesNotWaitForAnAbandonedRead(t *testing.T) {
	r, _ := hangingReader(t, 30*time.Millisecond)
	if res, err := r.Extract(context.Background(), lang.File{Path: "a.row", Src: []byte("block\n")}); err != nil || !res.Incomplete {
		t.Fatalf("%+v %v", res, err)
	}
	closed := make(chan struct{})
	go func() { r.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("Close waited for the abandoned read")
	}
	if _, err := r.Extract(context.Background(), lang.File{Path: "b.row", Src: []byte("ok\n")}); !errors.Is(err, lang.ErrClosed) {
		t.Fatalf("a closed Reader reads %v", err)
	}
}

func TestWithLimitsReadDeadlineOnlyLowers(t *testing.T) {
	for _, d := range []time.Duration{-time.Second, lang.DefaultReadDeadline + time.Nanosecond, time.Hour} {
		if r, err := lang.New(lang.WithLimits(lang.Limits{ReadDeadline: d})); err == nil {
			r.Close()
			t.Errorf("ReadDeadline %v was accepted", d)
		}
	}
	for _, d := range []time.Duration{0, time.Millisecond, lang.DefaultReadDeadline} {
		r, err := lang.New(lang.WithLimits(lang.Limits{ReadDeadline: d}))
		if err != nil {
			t.Errorf("ReadDeadline %v: %v", d, err)
			continue
		}
		r.Close()
	}
}

// A tree-sitter read holds its extractor for the whole parse, and a hostile
// file's parse outlives a short deadline by seconds. Close must not wait for it,
// and a read that arrives while Close runs must not queue behind it.
func TestCloseDoesNotWaitForAnAbandonedTreeSitterRead(t *testing.T) {
	r := newReader(t, lang.WithLimits(lang.Limits{ReadDeadline: 200 * time.Millisecond}))
	hostile := []byte(strings.Repeat("({[\n", 40000))
	res, err := r.Extract(context.Background(), lang.File{Path: "a.js", Src: hostile})
	if err != nil || !res.Incomplete {
		t.Fatalf("the hostile file did not overrun its deadline: %+v %v", res, err)
	}
	closed := make(chan struct{})
	go func() { r.Close(); close(closed) }()
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	if _, err := r.Extract(context.Background(), lang.File{Path: "b.go", Src: []byte("package p\n")}); !errors.Is(err, lang.ErrClosed) {
		t.Fatalf("a read during Close returned %v, want ErrClosed", err)
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("a read during Close waited %v", took)
	}
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close waited for the abandoned tree-sitter read")
	}
}
