package llmclient

import (
	"context"
	"sort"
)

// Pinger is the credential-validity and reachability probe interface. Any
// provider adapter that can issue a zero-token (or minimal-token) signed
// request to verify its configuration satisfies this interface. Eval-runners
// and failover routers call ProbeAll at startup so misconfigured lanes fail
// before tokens are spent.
//
// Ping returns nil when the lane is healthy. Errors are typed per the shared
// llmclient taxonomy:
//   - *AuthError: the credential is missing, malformed, or lacks permission —
//     do not retry; prompt for reconfiguration.
//   - *RateLimitError: the endpoint is alive but throttling — retryable.
//   - *ServerError: the endpoint is alive but having a transient problem — retryable.
//   - *InvalidRequestError: the configuration is wrong (bad BaseURL, unknown
//     endpoint, etc.) — non-retryable without reconfiguration.
//   - context errors pass through untyped so callers can distinguish
//     caller-chosen stop from provider failure.
//
// A nil Ping result proves reachability and credential acceptance at the time
// of the call. It does NOT prove that a specific model is accessible, that the
// calling token has inference entitlements, or that the account has quota for
// any particular request shape — those failures surface on the first real call.
type Pinger interface {
	Ping(ctx context.Context) error
}

// ProbeAll probes every lane in the supplied map concurrently, waits for all
// of them to complete regardless of any individual failure, and returns a map
// from lane name to error. Healthy lanes map to nil; failed lanes carry their
// typed error. The result map has the same key set as the input — every lane's
// outcome is present, which lets callers inspect the full configuration picture
// rather than reacting to only the first failure.
//
// Concurrency: ProbeAll launches one goroutine per lane and cancels no
// goroutines early — every lane is probed even when others have already
// failed. All goroutines finish before ProbeAll returns, so there are no
// background goroutines after the call. The caller's ctx governs each
// individual Ping; when ctx is cancelled all in-flight Pings receive the
// cancellation simultaneously.
//
// An empty map is valid and returns an empty result immediately.
//
// Determinism: the result map is keyed by lane name; callers that need
// deterministic ordering should range over sorted keys.
func ProbeAll(ctx context.Context, lanes map[string]Pinger) map[string]error {
	if len(lanes) == 0 {
		return map[string]error{}
	}

	// Collect results from all lanes concurrently without stopping early.
	// Each goroutine writes to a dedicated index in the parallel arrays;
	// no synchronisation is needed beyond the channel used to join all goroutines.
	type result struct {
		name string
		err  error
	}
	ch := make(chan result, len(lanes))

	// Dispatch probes in sorted key order so test output is stable, but since
	// they run concurrently the completion order is non-deterministic.
	names := make([]string, 0, len(lanes))
	for name := range lanes {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		p := lanes[name]
		n := name
		go func() {
			ch <- result{name: n, err: p.Ping(ctx)}
		}()
	}

	out := make(map[string]error, len(lanes))
	for range names {
		r := <-ch
		if r.err != nil {
			out[r.name] = r.err
		}
		// Healthy lanes (nil error) are intentionally omitted from the result:
		// the absence of a key means "healthy". Callers that need an exhaustive
		// map (present even for healthy lanes) can use `if _, ok := out[name];
		// !ok { /* healthy */ }`. We do NOT store nil values so callers can
		// range over the result to find failures without a nil check per entry.
	}
	return out
}
