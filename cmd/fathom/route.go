package main

// route.go — POST /route (DP-9, ADR-042 decision 2, docs/vision/
// 32-decision-port.md §2 item 3): one decision-port `choice` question
// asking which MCP tool's question-shape best fits a caller-supplied
// prompt.
//
// This route NEVER asserts a code fact — it is a SUGGESTION about the
// PROMPT's shape (which lookup verb usually answers a question like this
// one), never a claim about the repository itself. The index's own tools
// (find_symbol, references, obligations, ...) remain the only source of
// truth; /route only points at one of them.
import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/pkg/client"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// routeDecideTimeoutDefault is the fallback bound on the decider call this
// handler makes when ports.Cfg.RouteTimeoutMS is unset (0) — a Ports built
// outside config.Load (a direct test construction, say) that never got a
// RouteTimeoutMS still gets a sane budget rather than a zero/immediate
// timeout. config.DefaultDecide.RouteTimeoutMS carries the SAME value
// (3000ms) for every real caller; this constant exists only as this
// function's own defensive floor.
const routeDecideTimeoutDefault = 3 * time.Second

// routeDecideTimeout resolves ports.Cfg.RouteTimeoutMS (O109: [decide]
// route_timeout_ms, default 3000ms) into a time.Duration, bounding the
// decider call this handler makes — derived from the request's own
// context, so a slow/unreachable LLM lane can never outlast this on top of
// whatever deadline the caller (the prompt hook) already enforces on the
// HTTP round trip itself. A non-positive configured value (Ports built
// without going through config.Load/DefaultDecide) falls back to
// routeDecideTimeoutDefault rather than firing an immediate/zero timeout.
func routeDecideTimeout(ports *decide.Ports) time.Duration {
	if ports == nil || ports.Cfg.RouteTimeoutMS <= 0 {
		return routeDecideTimeoutDefault
	}
	return time.Duration(ports.Cfg.RouteTimeoutMS) * time.Millisecond
}

// routeVerbs is DP-9's fixed, ordered verb set: each Name is an MCP tool
// this route can suggest (or "none"), and Criteria is the one-line
// disambiguating text a decider judges each option against. Order is the
// order routeVerbs are presented to the decider — stable across calls, not
// meaningful to the answer itself (decision.KindChoice scores every option
// independently).
var routeVerbs = []decision.Option{
	{Name: "localize", Criteria: "A bug report, stack trace, error string, or failing test — the problem's location in the code is not yet known."},
	{Name: "find_symbol", Criteria: "Where is X defined — the prompt asks for a symbol's own definition location."},
	{Name: "references", Criteria: "Who uses X — the prompt asks for every call site or mention of a symbol."},
	{Name: "find_callers", Criteria: "Who calls X, or what would break if X changes — the prompt asks for a symbol's direct callers."},
	{Name: "obligations", Criteria: "What rules govern this file or this repository — the prompt asks for binding conventions, not code."},
	{Name: "staleness_report", Criteria: "Is this still true, or what was known/believed at some point — a freshness or history question, not a lookup."},
	{Name: "search", Criteria: "A literal string, a config value, or a log line — a text search, not a symbol question."},
	{Name: "none", Criteria: "A conversation, a question no index can answer, or a task that needs no lookup at all."},
}

// routeShapeOf maps each routeVerbs Name to DP-9's fixed shape label —
// what the response's "shape" field names, independent of the verb string
// itself (references and find_callers share "who-calls").
//
//fathom:tenancy-allow pkg-map read-only lookup table of constants, never written after init: not a cache
var routeShapeOf = map[string]string{
	"localize":         "issue",
	"find_symbol":      "where",
	"references":       "who-calls",
	"find_callers":     "who-calls",
	"obligations":      "what-governs",
	"staleness_report": "what-was-known",
	"search":           "literal",
	"none":             "none",
}

// routeRequest is POST /route's JSON body.
type routeRequest struct {
	Prompt string `json:"prompt"`
}

// routeMaxPromptBytes caps the prompt text this route ever hands to a
// decider — DP-9: "truncate the prompt to 4 KB before it reaches the
// decider". A runaway prompt (a pasted file, a long transcript) must not
// turn one route suggestion into an open-ended token spend.
const routeMaxPromptBytes = 4096

// routeResponse is POST /route's JSON body. Always 200 (routeHandler never
// writes a non-200 status): Error is set only alongside the none-shaped
// fields, on a decider failure or an invalid answer — the caller (the
// prompt hook's routeLine) must be able to fail open without special-
// casing a status code.
type routeResponse struct {
	Shape      string  `json:"shape"`
	Verb       string  `json:"verb"`
	Confidence float64 `json:"confidence"`
	Band       string  `json:"band"`
	Provider   string  `json:"provider"`
	Error      string  `json:"error,omitempty"`
}

// noneRouteResponse is the fixed body DP-9 documents for provider "none" —
// reused, plus an Error, as the degrade shape for every other failure path
// (a decider error, an Invalid answer, ctx deadline) too.
func noneRouteResponse() routeResponse {
	return routeResponse{Shape: "", Verb: "", Confidence: 0, Band: string(decision.BandUnsure), Provider: "none"}
}

// routeHandler serves POST /route (DP-9). ports nil, ports.Decider nil, or
// ports.Decider a decision.None all answer the fixed none-shaped body with
// NO decider call and no recorder row — the same identity fast path
// PrefilterSections/rerankWithinTiers already give a None decider
// elsewhere in this port. Otherwise it asks ONE decision.KindChoice
// question ("verb") over routeVerbs, State = the (4KB-truncated) prompt,
// and renders the answer into {shape, verb, confidence, band, provider}.
// The decision is logged to ports.Recorder under site "route": applied is
// band proceed AND confidence >= ports.Cfg.RouteConfidence — the confirm
// band is never applied, even though the caller still renders a hedged
// line for it (DP-10's own distinction between "logged as acted on" and
// "worth mentioning").
//
// Every failure — wrong method, unparseable body, a decider error, or an
// Invalid answer — degrades to a 200 (405 for the wrong method is the one
// exception: routing itself never fails a caller, but a non-POST request
// is a caller bug worth surfacing plainly). POST /route never 5xxs.
func routeHandler(ports *decide.Ports) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var req routeRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, routeMaxPromptBytes*4)).Decode(&req); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		prompt := req.Prompt
		if len(prompt) > routeMaxPromptBytes {
			prompt = prompt[:routeMaxPromptBytes]
		}

		if !routePortActive(ports) {
			writeRouteResponse(w, noneRouteResponse())
			return
		}

		// ADR-044 decision 8 (I14): a second, independent gate a real
		// provider must also clear — "route" absent from [decide] sites
		// makes NO call at all, same shape as the port-inactive path above,
		// but the disclosure names WHY (routePortActive's own degrade
		// carries no Error at all, since "off" needs no explanation; a
		// configured-but-site-blocked provider does).
		if !decide.SiteAllowed(ports.Cfg, decide.SiteRoute) {
			out := noneRouteResponse()
			out.Error = fmt.Sprintf("decision port: site %q not allowed by [decide] sites (no call made)", decide.SiteRoute)
			writeRouteResponse(w, out)
			return
		}

		// RT-65 (vision 36 §8): the call is admitted as egress; at the
		// refuse rung and in safe mode it is skipped, disclosed, never made.
		// KS-14 (O1131): a live kill is disclosed once per session and
		// site; /route is the site, X-Fathom-Caller's session= the session.
		rctx := decide.WithSite(r.Context(), decide.SiteRoute)
		if c, ok := client.ParseCallerHeader(r.Header.Get(client.CallerHeaderName)); ok && c.Session != "" {
			rctx = decide.WithSession(rctx, c.Session)
		}
		dctx, release, skip := admitDecisionCall(rctx, decide.SiteRoute)
		defer release()
		if skip != "" {
			out := noneRouteResponse()
			out.Error = skip
			writeRouteResponse(w, out)
			return
		}
		ctx, cancel := context.WithTimeout(dctx, routeDecideTimeout(ports))
		defer cancel()

		callStart := time.Now()
		resp, err := ports.Decider.Decide(ctx, decision.Request{
			State: prompt,
			Questions: []decision.Question{{
				ID:           "verb",
				Kind:         decision.KindChoice,
				Instructions: "Which of these actions best fits what this prompt is asking for? Pick exactly one.",
				Options:      routeVerbs,
			}},
		})
		if err != nil {
			// DP-22 (O109): a hard Decide error here produced no
			// decision.Response at all, so Record has nothing to log —
			// RecordError is the parallel path, naming the failure instead
			// of leaving decisions.jsonl silent about it. model is "" — the
			// Decider interface has no Model() accessor to read one off a
			// call that never returned a Response.
			ports.Recorder.RecordError("route", ports.Decider.Name(), "", 1, time.Since(callStart), err)
			out := noneRouteResponse()
			out.Error = err.Error()
			writeRouteResponse(w, out)
			return
		}

		answer, ok := resp.Answers["verb"]
		if !ok {
			out := noneRouteResponse()
			if reason, iok := resp.Invalid["verb"]; iok {
				out.Error = reason
			} else {
				out.Error = "decision port: no answer for question \"verb\""
			}
			writeRouteResponse(w, out)
			return
		}

		band := decision.BandOf(answer.Confidence)
		applied := band == decision.BandProceed && answer.Confidence >= ports.Cfg.RouteConfidence
		ports.Recorder.Record("route", resp, func(decision.Answer) bool { return applied })

		writeRouteResponse(w, routeResponse{
			Shape:      routeShapeOf[answer.Choice],
			Verb:       answer.Choice,
			Confidence: answer.Confidence,
			Band:       string(band),
			Provider:   resp.Provider,
		})
	})
}

// routePortActive reports whether ports carries a real decider worth
// calling — nil ports, a nil Decider, or the kit's own decision.None stub
// all answer false, matching pkg/client.Client.decisionActive's identical
// gate for the compile/obligations sites of this same port.
func routePortActive(ports *decide.Ports) bool {
	if ports == nil || ports.Decider == nil {
		return false
	}
	_, isNone := ports.Decider.(decision.None)
	return !isNone
}

func writeRouteResponse(w http.ResponseWriter, resp routeResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
