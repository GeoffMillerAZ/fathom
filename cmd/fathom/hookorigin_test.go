package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// RT-146: the daemon marks a hook's request so a whole-tree read under it
// never waits for a slot; agents and the CLI are not hooks.
func TestMarkHookOrigin(t *testing.T) {
	for origin, want := range map[string]bool{
		"edit-hook": true, "session-hook": true, "stop-hook": true, "prompt-hook": true, "prime": true,
		"": false, "verify-cli": false, "doctor": false, "console": false, "init": false,
	} {
		if got := client.IsHookOrigin(markHookOrigin(context.Background(), origin)); got != want {
			t.Errorf("markHookOrigin(%q) hook = %v, want %v", origin, got, want)
		}
	}
}

type oneFileConnector struct{}

func (oneFileConnector) Discover(_ context.Context, svc core.ServiceRef) ([]core.SourceRef, error) {
	return []core.SourceRef{{Service: svc.ID, Path: "a.go"}}, nil
}
func (oneFileConnector) Fetch(context.Context, core.SourceRef) ([]byte, error) {
	return []byte("package a\n"), nil
}
func (oneFileConnector) Watermark(context.Context, core.ServiceRef) (string, error) { return "", nil }

// RT-146 review: every /rpc request is one whole-tree scope — its slot is
// held until the response is written (the tree lives that long), and a
// client disconnect ends its wait for one (the /rpc ctx itself is detached
// from the connection and has no deadline).
func TestRPCSourcesScope(t *testing.T) {
	c := client.New(oneFileConnector{}, store.NewMemoryStore())
	newReq := func() (*http.Request, context.CancelFunc) {
		rctx, cancel := context.WithCancel(context.Background())
		return httptest.NewRequest(http.MethodPost, "/rpc", nil).WithContext(rctx), cancel
	}
	r1, cancel1 := newReq()
	defer cancel1()
	r2, cancel2 := newReq()
	defer cancel2()
	ctx1, done1 := rpcSourcesScope(context.Background(), r1)
	ctx2, done2 := rpcSourcesScope(context.Background(), r2)
	defer done1()
	defer done2()
	for _, ctx := range []context.Context{ctx1, ctx2} {
		if _, err := c.Sources(ctx, "svc"); err != nil {
			t.Fatalf("Sources: %v", err)
		}
	}
	if _, err := c.Sources(client.WithHookOrigin(context.Background()), "svc"); !errors.Is(err, client.ErrSourcesBusy) {
		t.Fatalf("two open /rpc scopes hold both slots; a hook read = %v, want ErrSourcesBusy", err)
	}
	r3, cancel3 := newReq()
	ctx3, done3 := rpcSourcesScope(context.Background(), r3)
	defer done3()
	errc := make(chan error, 1)
	go func() { _, err := c.Sources(ctx3, "svc"); errc <- err }()
	time.Sleep(20 * time.Millisecond)
	cancel3() // the client hung up
	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("a disconnected request's wait returned a tree")
		}
	case <-time.After(waitSafety):
		t.Fatal("a disconnected /rpc request kept waiting for a whole-tree slot")
	}
}
