package mcp

// admission.go — dispatch admits before the handler runs (design §3.3,
// §3.4; RT-56, RT-57, RT-60, RT-64, RT-65). A tools/call is admitted by the
// server's gate in its cost class (costclass.go); a refusal is JSON-RPC
// -32050 naming the reason, and the HTTP layer's sink learns it so POST
// /rpc answers 503 with Retry-After. After the handler, the answer names
// itself: a call whose context ended is a named abandonment, a call whose
// producers passed the per-call ceiling carries budget.partial, and the
// call-log row carries class, queued_ms, bytes_read, outcome and caller.
// A server with no gate (stdio, most tests) admits everything.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/pprof"
	"strconv"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// ErrCodeBusy is the refusal's JSON-RPC error code (§3.4; the
// compatibility surface).
const ErrCodeBusy = -32050

// OriginConsoleInProcess is the origin of the daemon's own in-process
// console reads (§3.3): excluded from agent traffic like OriginConsole,
// included in every resource instrument.
const OriginConsoleInProcess = "console-inprocess"

// SetGate installs the admission gate. Set once before serving.
func (s *Server) SetGate(g *tenancy.Gate) { s.gate = g }

// Gate is the installed admission gate, or nil.
func (s *Server) Gate() *tenancy.Gate { return s.gate }

// CallerInfo is the caller's identity (X-Fathom-Caller, §10.4).
type CallerInfo struct {
	PID, PPID int
	Session   string
	Test      bool
}

type callerKey struct{}

// WithCaller carries the caller's identity for admission and the call log.
func WithCaller(ctx context.Context, c CallerInfo) context.Context {
	return context.WithValue(ctx, callerKey{}, c)
}

func callerFrom(ctx context.Context) (CallerInfo, bool) {
	c, ok := ctx.Value(callerKey{}).(CallerInfo)
	return c, ok
}

type refusalSink struct {
	mu  sync.Mutex
	ref *tenancy.Refusal
}

type refusalSinkKey struct{}

// WithRefusalSink returns a ctx whose top-level tools/call refusal is
// recorded, and the func that reads it (nil: admitted). The HTTP layer
// uses it to answer 503 with Retry-After (§3.4).
func WithRefusalSink(ctx context.Context) (context.Context, func() *tenancy.Refusal) {
	sink := &refusalSink{}
	return context.WithValue(ctx, refusalSinkKey{}, sink), func() *tenancy.Refusal {
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.ref
	}
}

// RefusalHTTP is a refusal's HTTP status and Retry-After value (whole
// seconds, at least 1).
func RefusalHTTP(ref *tenancy.Refusal) (status int, retryAfter string) {
	secs := int64((ref.RetryAfter + time.Second - 1) / time.Second)
	return 503, strconv.FormatInt(max(1, secs), 10)
}

// busyError is the -32050 error a refusal answers with.
func busyError(ref *tenancy.Refusal) *rpcError {
	inFlight := ref.InFlight
	if inFlight == nil {
		inFlight = []string{}
	}
	return &rpcError{Code: ErrCodeBusy, Message: "fathom busy: " + ref.Reason, Data: map[string]any{
		"reason":         ref.Reason,
		"class":          ref.Class,
		"level":          ref.Level,
		"retry_after_ms": ref.RetryAfter.Milliseconds(),
		"in_flight":      inFlight,
		"outcome":        string(tenancy.OutcomeRefused),
	}}
}

type detachHookKey struct{}

// WithDetachHook returns ctx whose ingest-class tools/call (top level or
// batch sub-call) runs hook before its handler: the HTTP layer lifts the
// /rpc write deadline for work that runs detached and may outlast it.
func WithDetachHook(ctx context.Context, hook func()) context.Context {
	return context.WithValue(ctx, detachHookKey{}, hook)
}

// callMeta is what the call log records of one admission.
type callMeta struct {
	class     tenancy.Class
	queued    time.Duration
	bytesRead int64
	outcome   string
}

// admitToolCall admits one tools/call (or batch sub-call) before its
// handler runs. It returns the handler's ctx (the ticket's: the class
// deadline, the in-flight registration, the meter), the ticket, or the
// named refusal. top marks a top-level call, whose refusal the HTTP sink
// learns. batch holds no ticket; ingest-class work runs detached from the
// requester (§3.5: only refresh, Keep flights and ingest outlive their
// caller).
func (s *Server) admitToolCall(ctx context.Context, te toolEntry, args json.RawMessage, top bool) (context.Context, *tenancy.Ticket, *rpcError) {
	class := costClassOf(te, args)
	actx := ctx
	if class == tenancy.ClassIngest {
		actx = tenancy.Detach(ctx)
		if hook, ok := ctx.Value(detachHookKey{}).(func()); ok {
			hook()
		}
	}
	if s.gate == nil || te.desc.Name == "batch" {
		return actx, nil, nil
	}
	caller, _ := callerFrom(ctx)
	req := tenancy.Request{
		Tool:    te.desc.Name,
		Service: serviceOfArgs(args),
		Class:   class,
		Origin:  tenancy.ParseOrigin(originFrom(ctx), caller.Test),
	}
	if top {
		// RT-179: the class deadline runs from the request's arrival.
		req.Arrived = arrivalFrom(ctx)
	}
	tk, ref := s.gate.Admit(actx, req)
	if ref != nil {
		if top {
			if sink, ok := ctx.Value(refusalSinkKey{}).(*refusalSink); ok {
				sink.mu.Lock()
				sink.ref = ref
				sink.mu.Unlock()
			}
		}
		return ctx, nil, busyError(ref)
	}
	return tk.Context(), tk, nil
}

// settleCall names how a call ended and releases its ticket: a call the
// refuse rung cancelled is abandoned-by-governor, one whose context ended
// otherwise is a named abandonment (never whatever the handler returned
// under a dead context), one whose producers passed the per-call ceiling
// is partial. partial is the budget block for the envelope, or nil.
func (s *Server) settleCall(ctx context.Context, tool string, tk *tenancy.Ticket, result any, rpcErr *rpcError) (any, *rpcError, map[string]any, callMeta) {
	meta := callMeta{outcome: string(tenancy.OutcomeOK)}
	var partial map[string]any
	release := tenancy.OutcomeOK
	switch ab := abandonedByGovernor(ctx); {
	case ab != nil:
		result, rpcErr = nil, ab
		meta.outcome, release = outcomeAbandonedByGovernor, tenancy.OutcomeAbandoned
		tenancy.RecordAbandoned(tool, tenancy.AbandonGovernor)
	case tk != nil && ctx.Err() != nil:
		cause := "the client went away"
		why := tenancy.AbandonClient
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			cause = "the call's deadline passed (the " + tk.Class().String() + "-class server deadline or X-Fathom-Deadline)"
			why = tenancy.AbandonDeadline
		}
		tenancy.RecordAbandoned(tool, why)
		result, rpcErr = nil, &rpcError{Code: ErrCodeInternal, Message: "abandoned: " + cause + "; " + tool + " did not finish and no partial answer is served",
			Data: map[string]any{"outcome": string(tenancy.OutcomeAbandoned), "cause": cause}}
		meta.outcome, release = string(tenancy.OutcomeAbandoned), tenancy.OutcomeAbandoned
	case rpcErr != nil:
		meta.outcome, release = string(tenancy.OutcomeError), tenancy.OutcomeError
	case tk != nil && tk.Stopped():
		// Only a producer the meter stopped makes a partial answer; in
		// observe mode an over-ceiling call ran to completion.
		p := tk.Partial()
		partial = map[string]any{"partial": true, "elided": p.Elided, "bytes_read": p.BytesRead,
			"reason": fmt.Sprintf("%s; at least %d item(s) not reached (a lower bound)", p.Reason, p.Elided)}
		meta.outcome, release = string(tenancy.OutcomePartial), tenancy.OutcomePartial
	}
	if tk != nil {
		meta.class, meta.queued, meta.bytesRead = tk.Class(), tk.Queued(), tk.BytesRead()
		tenancy.RecordBytesRead(tool, meta.bytesRead)
		tk.Release(release)
	}
	return result, rpcErr, partial, meta
}

// runHandler runs one tool handler and turns a panic into a named error,
// so the settle that follows always releases the admission ticket (its
// slot, in-flight entry and running count). net/http's conn.serve would
// otherwise recover the panic with the slot still held — in a 1-slot
// class, a refusal for the life of the process.
func runHandler(ctx context.Context, te toolEntry, args json.RawMessage) (result any, rpcErr *rpcError) {
	defer func() {
		if v := recover(); v != nil {
			result, rpcErr = nil, &rpcError{Code: ErrCodeInternal, Message: fmt.Sprintf("internal error: %s panicked: %v", te.desc.Name, v),
				Data: map[string]any{"outcome": string(tenancy.OutcomeError), "panic": fmt.Sprint(v)}}
		}
	}()
	// RT-122 (§13.4): the handler's goroutine — and every goroutine it
	// starts — carries the tool and origin as pprof labels, so a
	// diagnostic capture's goroutine.txt names whose work is running.
	pprof.Do(ctx, pprof.Labels("tool", te.desc.Name, "origin", firstNonEmptyOrigin(originFrom(ctx))), func(lctx context.Context) {
		result, rpcErr = te.handler(lctx, args)
	})
	return result, rpcErr
}

func firstNonEmptyOrigin(o string) string {
	if o == "" {
		return "unknown"
	}
	return o
}

// serviceOfArgs is args' "service", best-effort.
func serviceOfArgs(args json.RawMessage) string {
	var a struct {
		Service string `json:"service"`
	}
	_ = json.Unmarshal(args, &a)
	return a.Service
}

// AdmitDecision admits one decision-port call (/route, the obligations
// re-rank) as the egress class (vision 36 §8). At the refuse rung and in
// safe mode it is skipped: skip is the disclosure the caller renders, and
// no call is made. The release is always safe to call.
func (s *Server) AdmitDecision(ctx context.Context, site string) (context.Context, func(), string) {
	return s.admitDecision(ctx, site)
}

func (s *Server) admitDecision(ctx context.Context, site string) (context.Context, func(), string) {
	if s.gate == nil {
		return ctx, func() {}, ""
	}
	caller, _ := callerFrom(ctx)
	tk, ref := s.gate.Admit(ctx, tenancy.Request{Tool: "decide:" + site, Class: tenancy.ClassEgress, Origin: tenancy.ParseOrigin(originFrom(ctx), caller.Test)})
	if ref != nil {
		return ctx, func() {}, "decision port skipped (no call made): " + ref.Reason
	}
	return tk.Context(), func() { tk.Release(tenancy.OutcomeOK) }, ""
}
