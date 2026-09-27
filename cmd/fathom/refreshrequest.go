package main

// refreshrequest.go — POST /refresh-request {service} (RT-111, docs/design/
// responsible-tenant.md §12.3): the post-commit hook's refresh. It used to
// call the `ingest` tool, which ran the whole ingest inside the hook's HTTP
// request (every commit in every wired repo, in parallel with everything
// else). Now the request is queued on the daemon's refresh queue and
// answered at once: 202 with the queue's state (coalesced into a queued job
// or the running ingest's one follow-up), 503 with the named reason when the
// queue refuses (refuse, store distress, and — for a background request —
// safe mode and a quarantine), 404 for a service the daemon has not indexed
// (a hook never onboards one; the `ingest` tool with a path does).

import (
	"encoding/json"
	"net/http"

	"github.com/geoffmilleraz/fathom/internal/refresh"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// refreshRequestBody is POST /refresh-request's JSON body.
type refreshRequestBody struct {
	Service string `json:"service"`
}

// refreshRequestResponse is its answer.
type refreshRequestResponse struct {
	Service   string `json:"service"`
	State     string `json:"state"`
	Coalesced bool   `json:"coalesced,omitempty"`
	FollowUp  bool   `json:"follow_up,omitempty"`
	Paused    string `json:"paused,omitempty"`
	Refused   string `json:"refused,omitempty"`
	Error     string `json:"error,omitempty"`
}

// refreshRequestHandler serves POST /refresh-request.
func refreshRequestHandler(c *client.Client, queue func() *refresh.Queue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body refreshRequestBody
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil || body.Service == "" {
			http.Error(w, "a JSON body {\"service\": …} is required", http.StatusBadRequest)
			return
		}
		reply := func(code int, resp refreshRequestResponse) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			_ = json.NewEncoder(w).Encode(resp)
		}
		q := refreshQueueOf(queue)
		if q == nil {
			reply(http.StatusServiceUnavailable, refreshRequestResponse{Service: body.Service, State: string(refresh.StateRefused), Refused: "this daemon runs no refresh queue"})
			return
		}
		if _, _, found, err := c.ServiceMeta(r.Context(), body.Service); err != nil || !found {
			msg := "service is not indexed by this daemon — a hook never onboards one; use the ingest tool with a path"
			if err != nil {
				msg = err.Error()
			}
			reply(http.StatusNotFound, refreshRequestResponse{Service: body.Service, State: string(refresh.StateRefused), Error: msg})
			return
		}
		// The request is the queue's, not this handler's: it returns at
		// once and the ingest runs under the daemon's lifetime.
		st := q.RequestRefresh(r.Context(), refresh.Request{Service: body.Service, Reason: refresh.ReasonPostCommit}, false)
		resp := refreshRequestResponse{Service: body.Service, State: string(st.State), Coalesced: st.Coalesced, FollowUp: st.FollowUp, Paused: st.Paused, Refused: st.Refused}
		if st.State == refresh.StateRefused {
			w.Header().Set("Retry-After", "60")
			reply(http.StatusServiceUnavailable, resp)
			return
		}
		reply(http.StatusAccepted, resp)
	}
}

// refreshQueueOf is queue(), or nil when there is no getter.
func refreshQueueOf(queue func() *refresh.Queue) *refresh.Queue {
	if queue == nil {
		return nil
	}
	return queue()
}
