package iacfacts

import (
	"io/fs"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"
)

// hangFS blocks every Open until released, the way a read stuck on a slow or hostile input would.
type hangFS struct {
	inner fstest.MapFS // not embedded: its ReadFile and ReadDir would answer without Open

	release chan struct{}
	once    sync.Once
	opens   atomic.Int64
}

func newHangFS() *hangFS {
	h := &hangFS{inner: fstest.MapFS{"main.tf": {Data: []byte("terraform {}\n")}}, release: make(chan struct{})}
	return h
}

func (h *hangFS) unblock() { h.once.Do(func() { close(h.release) }) }

func (h *hangFS) Open(name string) (fs.File, error) {
	h.opens.Add(1)
	<-h.release
	return h.inner.Open(name)
}

func lowerBindingDeadline(t *testing.T, d time.Duration) {
	t.Helper()
	prev := bindingDeadline
	bindingDeadline = d
	t.Cleanup(func() { bindingDeadline = prev })
}

// A binding read that does not return is refused with a reason at its deadline, and starts nothing more past
// GOMAXPROCS abandoned ones. The reader functions stay synchronous (the door wraps them), so only this answer has a deadline here.
func TestBindingPastItsDeadlineIsRefusedWithAReason(t *testing.T) {
	prev := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(prev)
	lowerBindingDeadline(t, 40*time.Millisecond)
	h := newHangFS()
	t.Cleanup(h.unblock)
	q := IaCBindingQuery{Repo: h, Block: IaCBlockRef{Dir: ".", Type: "aws_s3_bucket"}}

	got := ResolveIaCBinding(q)
	if got.Status != BindUnbound || got.Refusal == nil || got.Refusal.Code != RefuseDeadline || !strings.Contains(got.Refusal.Reason, "40ms") || got.Version != "" || got.Catalog != "" {
		t.Fatalf("an overrun is refused with a reason: %+v", got)
	}
	opens := h.opens.Load()
	again := ResolveIaCBinding(q)
	if again.Refusal == nil || again.Refusal.Code != RefuseDeadline || !strings.Contains(again.Refusal.Reason, "not started") {
		t.Fatalf("past the cap nothing starts: %+v", again)
	}
	if h.opens.Load() != opens {
		t.Fatal("a refused answer read the repository")
	}
	h.unblock()
	deadline := time.Now().Add(10 * time.Second)
	for bindingGroup.Abandoned() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the abandoned read never finished")
		}
		time.Sleep(10 * time.Millisecond)
	}
	ok := ResolveIaCBinding(IaCBindingQuery{Repo: fstest.MapFS{"main.tf": {Data: []byte("terraform {}\n")}}, Block: q.Block})
	if ok.Refusal != nil && ok.Refusal.Code == RefuseDeadline {
		t.Fatalf("the answer did not recover when the abandoned read finished: %+v", ok)
	}
}
