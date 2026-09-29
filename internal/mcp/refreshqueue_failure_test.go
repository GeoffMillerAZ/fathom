package mcp

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/hookcache"
	"github.com/geoffmilleraz/fathom/internal/refresh"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

type diskFullIngestFake struct{ refreshFake }

func (f *diskFullIngestFake) IngestService(context.Context, string) error {
	return errors.New("write observations: database or disk is full (13)")
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

// TestQueuedIngestFailureIsNeverSilent (I38, RT-41, RT-111): a queued
// ingest nobody waits on — a post-commit /refresh-request — that fails is
// logged, notes store distress, busts the identity cache and is left on the
// queue as the service's last failure, exactly as the ingest tool's own
// failure path did before the hook moved onto the queue.
func TestQueuedIngestFailureIsNeverSilent(t *testing.T) {
	tenancy.ResetLifecycleStateForTest()
	t.Cleanup(tenancy.ResetLifecycleStateForTest)
	ident := t.TempDir()
	orig := hookcache.IdentDir
	hookcache.IdentDir = ident
	t.Cleanup(func() { hookcache.IdentDir = orig })
	if err := os.WriteFile(filepath.Join(ident, "x"), []byte("cached"), 0o644); err != nil {
		t.Fatal(err)
	}

	var logs syncBuf
	fake := &diskFullIngestFake{refreshFake{path: "/r", indexed: "old"}}
	q := NewRefreshQueueLogged(fake, refresh.Config{
		StartWait: -1, Stagger: -1, Warmup: func(time.Time) bool { return false },
		Gate: func(string, bool, time.Time) refresh.Verdict { return refresh.Verdict{} },
	}, dlog.New(&logs, dlog.LevelInfo, time.Now))
	ctx, cancel := context.WithCancel(context.Background())
	q.Start(ctx)
	t.Cleanup(func() { cancel(); q.Wait() })
	q.RequestRefresh(ctx, refresh.Request{Service: "svc", Reason: refresh.ReasonPostCommit}, false)

	deadline := time.Now().Add(waitSafety)
	for {
		if _, ok := q.LastFailure("svc"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the failed queued ingest left no failure on the queue")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !strings.Contains(logs.String(), "svc") || !strings.Contains(logs.String(), "disk is full") {
		t.Fatalf("no failure line logged: %q", logs.String())
	}
	if _, distressed := tenancy.StoreDistressNow(time.Now()); !distressed {
		t.Fatal("SQLITE_FULL from a queued ingest was not noted as store distress")
	}
	if _, err := os.Stat(ident); !os.IsNotExist(err) {
		t.Fatal("the identity cache was not busted after the queued ingest")
	}
	srv := NewServer(fake)
	srv.SetRefreshQueue(q)
	v, rerr := srv.handleToolsCallWithCtx(context.Background(), []byte(`{"name":"staleness_report","arguments":{"service":"svc"}}`))
	if rerr != nil {
		t.Fatalf("staleness_report: %v", rerr.Message)
	}
	text := v.(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "refresh_failure") || !strings.Contains(text, "post-commit refresh failed") {
		t.Fatalf("staleness_report does not name the queued refresh's failure: %s", text)
	}
}
