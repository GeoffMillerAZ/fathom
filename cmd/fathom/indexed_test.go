package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// RT-80 (docs/design/responsible-tenant.md §4.4): "is service X indexed" —
// asked by verify, the stop and edit hooks, plans and the console — is a
// meta-only probe (GET /indexed, a service_meta row), never a list_services
// call that pays for every service's full answer.
func TestDaemonServiceIndexedUsesMetaProbe(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "demo", Path: t.TempDir(), IndexedCommit: "c1", IndexedAt: time.Now().UnixNano()}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "never", Path: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	c := client.New(newCountingConnector(), st)

	var rpcCalls atomic.Int64
	mux := http.NewServeMux()
	mux.Handle("/indexed", indexedHandler(c))
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		rpcCalls.Add(1)
		http.Error(w, "list_services must not be called", http.StatusTeapot)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	hc := &http.Client{Timeout: 5 * time.Second}
	for _, tc := range []struct {
		service string
		want    bool
	}{{"demo", true}, {"never", false}, {"absent", false}} {
		got, err := daemonServiceIndexed(hc, srv.URL, tc.service)
		if err != nil {
			t.Fatalf("%s: %v", tc.service, err)
		}
		if got != tc.want {
			t.Fatalf("daemonServiceIndexed(%s) = %v, want %v", tc.service, got, tc.want)
		}
	}
	if n := rpcCalls.Load(); n != 0 {
		t.Fatalf("the indexed probe called /rpc (list_services) %d time(s)", n)
	}

	// A daemon that predates GET /indexed (404) is still answered, through
	// the list_services probe it has always served.
	old := http.NewServeMux()
	old.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"services\":[{\"service\":\"demo\",\"symbol_count\":3}]}"}]}}`))
	})
	oldSrv := httptest.NewServer(old)
	defer oldSrv.Close()
	if got, err := daemonServiceIndexed(hc, oldSrv.URL, "demo"); err != nil || !got {
		t.Fatalf("against a pre-/indexed daemon: %v, %v; want true", got, err)
	}
}
