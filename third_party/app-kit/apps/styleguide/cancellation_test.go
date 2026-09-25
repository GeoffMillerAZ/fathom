package styleguide_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/apps/styleguide"
)

func TestStyleGuidePreviewCancellationDoesNotLeakPartialSpecimen(t *testing.T) {
	source, err := os.ReadFile("../../cue/tokens/kit.cue")
	if err != nil {
		t.Fatal(err)
	}
	engine, err := styleguide.NewEngine(styleguide.EngineOptions{Source: source, SourceLabel: "cancellation-fixture", KitRevision: "test", Fonts: map[string]string{"sans": "system-ui"}, Checks: []styleguide.ContrastCheck{{ID: "body", Foreground: "ink", Background: "surface", Scheme: "light", Minimum: 4.5}}})
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	type requestMarker struct{}
	cause := errors.New("synthetic preview cancellation")
	observed := make(chan error, 1)
	var calls atomic.Int32
	const origin = "http://127.0.0.1:18766"
	handler, err := styleguide.New(styleguide.Options{Engine: engine, BasePath: "/theme", Origin: origin, Title: "Cancellation", Specimens: []styleguide.Specimen{{ID: "fixture", Name: "Fixture", Render: func(ctx context.Context, w io.Writer) error {
		if calls.Add(1) == 1 {
			// Write before cancellation: the handler must discard partial output.
			if _, err := io.WriteString(w, "PRIVATE-PREFIX"); err != nil {
				return err
			}
			close(started)
			select {
			case <-ctx.Done():
				if ctx.Value(requestMarker{}) != "request-owned" {
					observed <- errors.New("request context marker lost")
				} else {
					observed <- context.Cause(ctx)
				}
			case <-time.After(5 * time.Second):
				err := errors.New("callback did not receive cancellation")
				observed <- err
				return err
			}
			return ctx.Err()
		}
		_, err := io.WriteString(w, "recovered specimen")
		return err
	}}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancelCause(context.WithValue(context.Background(), requestMarker{}, "request-owned"))
	defer cancel(cause)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, origin+"/theme/preview?specimen=fixture&scheme=light", nil)
	record := httptest.NewRecorder()
	req.RemoteAddr = "127.0.0.1:18767"
	result := make(chan struct{})
	go func() {
		handler.ServeHTTP(record, req)
		close(result)
	}()
	select {
	case <-started:
	case <-result:
		t.Fatal("request completed before callback entered")
	case <-time.After(5 * time.Second):
		t.Fatal("callback did not start")
	}
	cancel(cause)
	select {
	case <-result:
	case <-time.After(10 * time.Second):
		t.Fatal("handler did not return after cancellation")
	}
	select {
	case got := <-observed:
		if !errors.Is(got, cause) {
			t.Fatalf("callback context: got %v; want request cancellation cause", got)
		}
	default:
		t.Fatal("callback did not observe cancellation")
	}
	if record.Code != http.StatusInternalServerError || bytes.Contains(record.Body.Bytes(), []byte("PRIVATE-PREFIX")) {
		t.Fatalf("unexpected canceled response: status=%d bytes=%d", record.Code, record.Body.Len())
	}
	second := httptest.NewRecorder()
	fresh := httptest.NewRequest(http.MethodGet, origin+"/theme/preview?specimen=fixture&scheme=light", nil)
	fresh.RemoteAddr = "127.0.0.1:18767"
	handler.ServeHTTP(second, fresh)
	if second.Code != http.StatusOK || !bytes.Contains(second.Body.Bytes(), []byte("recovered specimen")) {
		t.Fatalf("subsequent request did not recover: status=%d", second.Code)
	}
	if calls.Load() != 2 {
		t.Fatalf("got %d callback calls; want canceled request and fresh retry", calls.Load())
	}
}
