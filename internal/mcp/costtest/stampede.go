package costtest

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// Stampede is what N concurrent callers of one call cost together.
type Stampede struct {
	Callers int
	// Flights is each memo group's builds started during the stampede.
	Flights map[string]int64
	// Cancelled is each memo group's flights cancelled during it.
	Cancelled map[string]int64
	BytesRead int64
	Errors    []string
}

func memoCounters() (flights, cancelled map[string]int64) {
	flights, cancelled = map[string]int64{}, map[string]int64{}
	for _, g := range memo.Stats() {
		flights[g.Name] += g.Flights
		cancelled[g.Name] += g.Cancelled
	}
	return flights, cancelled
}

func deltas(before, after map[string]int64) map[string]int64 {
	out := map[string]int64{}
	for k, v := range after {
		if d := v - before[k]; d != 0 {
			out[k] = d
		}
	}
	return out
}

// MeasureStampede releases callers concurrent tools/calls of tool on srv
// at once and returns the memo builds they started and the bytes they
// read between them. A cold stampede must start one build per key and
// read one walk's bytes (design §4.8).
func MeasureStampede(ctx context.Context, srv *mcp.Server, tool string, args json.RawMessage, callers int) (Stampede, error) {
	req, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	})
	if err != nil {
		return Stampede{}, err
	}
	st := Stampede{Callers: callers}
	f0, c0 := memoCounters()
	b0 := tenancy.BytesReadTotal(tool)
	start := make(chan struct{})
	errs := make([]string, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		//fathom:tenancy-allow bare-go the harness's own concurrent callers, joined by a WaitGroup before it returns
		go func(i int) {
			defer wg.Done()
			<-start
			raw, ok := srv.HandleRequest(ctx, req)
			if !ok {
				errs[i] = "no response"
				return
			}
			errs[i] = errorText(raw)
		}(i)
	}
	close(start)
	wg.Wait()
	f1, c1 := memoCounters()
	st.Flights, st.Cancelled = deltas(f0, f1), deltas(c0, c1)
	st.BytesRead = tenancy.BytesReadTotal(tool) - b0
	for _, e := range errs {
		if e != "" {
			st.Errors = append(st.Errors, e)
		}
	}
	return st, nil
}

// GatedBackend is a client whose symbol loads wait for Release (or their
// context): a cold build held open, so callers can be cancelled mid-build.
type GatedBackend struct {
	*client.Client
	Entered chan struct{} // receives once per load that started waiting
	Release chan struct{} // closed to let loads through
}

// NewGatedBackend gates c's symbol loads.
func NewGatedBackend(c *client.Client) *GatedBackend {
	return &GatedBackend{Client: c, Entered: make(chan struct{}, 64), Release: make(chan struct{})}
}

// ListAllSymbols waits for Release or ctx, then loads.
func (g *GatedBackend) ListAllSymbols(ctx context.Context, service string) ([]core.SymbolNode, error) {
	select {
	case g.Entered <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-g.Release:
	}
	return g.Client.ListAllSymbols(ctx, service)
}

// CancelResult is a cancelled cold stampede's aftermath.
type CancelResult struct {
	// Cancelled is each memo group's flights cancelled.
	Cancelled map[string]int64
	// Settle is how long the goroutine count took to return to its
	// baseline after the callers were cancelled; Leaked is how many
	// goroutines were still above it when MeasureStampedeCancel gave up.
	Settle time.Duration
	Leaked int
}

// MeasureStampedeCancel starts callers concurrent cold calls of tool on a
// server over gated, waits until the build is held open, cancels every
// caller, and measures how long the process's goroutines take to return
// to their count before the stampede (giving up after limit).
func MeasureStampedeCancel(srv *mcp.Server, gated *GatedBackend, tool string, args json.RawMessage, callers int, limit time.Duration) (CancelResult, error) {
	req, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	})
	if err != nil {
		return CancelResult{}, err
	}
	runtime.GC()
	baseline := runtime.NumGoroutine()
	_, c0 := memoCounters()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		//fathom:tenancy-allow bare-go the harness's own concurrent callers, joined by a WaitGroup before it returns
		go func() {
			defer wg.Done()
			srv.HandleRequest(ctx, req)
		}()
	}
	select {
	case <-gated.Entered:
	case <-time.After(10 * time.Second):
		cancel()
		wg.Wait()
		return CancelResult{}, fmt.Errorf("costtest: %s never started a gated build", tool)
	}
	// Let every caller reach the flight before the cancel.
	time.Sleep(20 * time.Millisecond)
	cancel()
	wg.Wait()
	began := time.Now()
	res := CancelResult{}
	for {
		n := runtime.NumGoroutine()
		if n <= baseline {
			res.Settle = time.Since(began)
			break
		}
		if time.Since(began) > limit {
			res.Settle, res.Leaked = time.Since(began), n-baseline
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	_, c1 := memoCounters()
	res.Cancelled = deltas(c0, c1)
	return res, nil
}
