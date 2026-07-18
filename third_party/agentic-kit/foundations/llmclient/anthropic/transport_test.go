package anthropic

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// TestRedirectNeverFollowed proves the default HTTP clients refuse to follow
// redirects for BOTH auth schemes: a redirecting (proxy) origin must NOT be
// able to exfiltrate the credential header to a second host. Go strips only
// Authorization/Cookie on cross-host redirects (which would shield the bearer
// scheme but not x-api-key, and shields neither on same-host redirects);
// refusing all redirects protects both schemes uniformly without relying on
// that partial stripping. With ErrUseLastResponse the 302 itself becomes the
// response and maps through the standard status mapping to a typed error.
func TestRedirectNeverFollowed(t *testing.T) {
	for _, scheme := range []string{AuthSchemeAPIKey, AuthSchemeBearer} {
		t.Run(scheme, func(t *testing.T) {
			var leakHits atomic.Int32
			leak := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				leakHits.Add(1)
			}))
			t.Cleanup(leak.Close)

			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, leak.URL+r.URL.Path, http.StatusFound)
			}), Config{AuthScheme: scheme})

			t.Run("Complete", func(t *testing.T) {
				_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
				if err == nil {
					t.Fatal("a 302 response must surface as an error, not be followed")
				}
				var ire *llmclient.InvalidRequestError
				if !errors.As(err, &ire) {
					t.Fatalf("err = %T %v, want typed *llmclient.InvalidRequestError via the status mapping", err, err)
				}
				if strings.Contains(err.Error(), "test-key") {
					t.Errorf("error output leaked the credential: %v", err)
				}
			})
			t.Run("StreamPreStream", func(t *testing.T) {
				ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "p"})
				if ch != nil {
					t.Error("a 302 response must fail pre-stream, no channel")
				}
				var ire *llmclient.InvalidRequestError
				if !errors.As(err, &ire) {
					t.Fatalf("err = %T %v, want typed *llmclient.InvalidRequestError via the status mapping", err, err)
				}
			})
			t.Run("Ping", func(t *testing.T) {
				err := c.Ping(context.Background())
				var ire *llmclient.InvalidRequestError
				if !errors.As(err, &ire) {
					t.Fatalf("err = %T %v, want typed *llmclient.InvalidRequestError via the status mapping", err, err)
				}
			})

			if n := leakHits.Load(); n != 0 {
				t.Fatalf("redirect target received %d request(s); the credential must never follow a redirect", n)
			}
		})
	}
}

// TestTransportErrorTyped proves transport-level failures (here: the server
// hangs up before sending any response) surface as a typed retryable
// ServerError on both paths, so WithRetry treats transient network blips like
// a 5xx instead of failing fast.
func TestTransportErrorTyped(t *testing.T) {
	hangup := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("response writer is not a hijacker")
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		conn.Close() // sever the connection pre-response
	})
	c := newTestClient(t, hangup, Config{})

	t.Run("Complete", func(t *testing.T) {
		_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
		var se *llmclient.ServerError
		if !errors.As(err, &se) {
			t.Fatalf("err = %T %v, want retryable *llmclient.ServerError", err, err)
		}
		if strings.Contains(se.Message, "test-key") {
			t.Errorf("transport error leaked the API key: %v", se)
		}
	})
	t.Run("StreamPreStream", func(t *testing.T) {
		ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "p"})
		if ch != nil {
			t.Error("transport failure must fail pre-stream, no channel")
		}
		var se *llmclient.ServerError
		if !errors.As(err, &se) {
			t.Fatalf("err = %T %v, want retryable *llmclient.ServerError", err, err)
		}
	})
}

// TestTransportCtxErrorsPassThrough proves context cancellation and deadline
// expiry are NOT mapped to retryable ServerError: the caller chose to stop,
// and WithRetry must fail fast (a ctx error is neither RateLimitError nor
// ServerError).
func TestTransportCtxErrorsPassThrough(t *testing.T) {
	cases := []struct {
		name string
		ctx  func() (context.Context, context.CancelFunc)
		want error
	}{
		{
			name: "Cancelled",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			want: context.Canceled,
		},
		{
			name: "DeadlineExceeded",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), -1)
			},
			want: context.DeadlineExceeded,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("no HTTP request should complete under a dead context")
			}), Config{})
			ctx, cancel := tc.ctx()
			defer cancel()

			_, err := c.Complete(ctx, llmclient.Request{Prompt: "p"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want errors.Is(_, %v)", err, tc.want)
			}
			var se *llmclient.ServerError
			if errors.As(err, &se) {
				t.Fatalf("ctx error must NOT be typed retryable ServerError, got %v", err)
			}
		})
	}
}

// writeOrStop writes s, reporting false when the peer hung up (expected once
// the client hits its read cap and closes the body).
func writeOrStop(w http.ResponseWriter, s string) bool {
	_, err := io.WriteString(w, s)
	return err == nil
}

// assertTerminalCapError drains the stream and asserts the terminal chunk is
// a typed ServerError naming the byte cap.
func assertTerminalCapError(t *testing.T, ch <-chan llmclient.StreamChunk) {
	t.Helper()
	chunks := drainStream(t, ch)
	if len(chunks) == 0 {
		t.Fatal("want a terminal Err chunk")
	}
	last := chunks[len(chunks)-1]
	var se *llmclient.ServerError
	if !errors.As(last.Err, &se) {
		t.Fatalf("terminal Err = %T %v, want retryable *llmclient.ServerError", last.Err, last.Err)
	}
	if !strings.Contains(se.Message, "cap") {
		t.Errorf("message %q should name the byte cap", se.Message)
	}
}

// TestStreamOversizeLineFailsTyped proves a single SSE line larger than
// maxBodyBytes fails the stream with a typed retryable ServerError instead of
// growing memory without bound.
func TestStreamOversizeLineFailsTyped(t *testing.T) {
	filler := strings.Repeat("x", 1<<20)
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		if !writeOrStop(w, "event: content_block_delta\ndata: ") {
			return
		}
		// One endless data line: a megabyte past the cap, never terminated.
		for written := 0; written <= maxBodyBytes; written += len(filler) {
			if !writeOrStop(w, filler) {
				return // client hit its cap and hung up — expected
			}
		}
	}), Config{})

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	assertTerminalCapError(t, ch)
}

// TestStreamOversizeEventFailsTyped proves per-event data accumulation is
// capped too: many individually small data lines whose total exceeds
// maxBodyBytes fail the stream with a typed retryable ServerError.
func TestStreamOversizeEventFailsTyped(t *testing.T) {
	// ~1 MiB per data line, well under the line cap; no blank line in
	// between, so it is all ONE event accumulating past the cap.
	line := "data: " + strings.Repeat("y", 1<<20) + "\n"
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		if !writeOrStop(w, "event: content_block_delta\n") {
			return
		}
		for i := 0; i <= 33; i++ {
			if !writeOrStop(w, line) {
				return // client hit its cap and hung up — expected
			}
		}
	}), Config{})

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "go"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	assertTerminalCapError(t, ch)
}

// TestCompleteOversizeBodyFailsTyped proves a success-status response body
// larger than maxBodyBytes fails Complete with a typed retryable ServerError
// instead of an unbounded io.ReadAll.
func TestCompleteOversizeBodyFailsTyped(t *testing.T) {
	filler := strings.Repeat("z", 1<<20)
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		if !writeOrStop(w, `{"content":[{"type":"text","text":"`) {
			return
		}
		for written := 0; written <= maxBodyBytes; written += len(filler) {
			if !writeOrStop(w, filler) {
				return // client hit its cap and hung up — expected
			}
		}
	}), Config{})

	_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
	var se *llmclient.ServerError
	if !errors.As(err, &se) {
		t.Fatalf("err = %T %v, want retryable *llmclient.ServerError", err, err)
	}
	if !strings.Contains(se.Message, "cap") {
		t.Errorf("message %q should name the byte cap", se.Message)
	}
}
