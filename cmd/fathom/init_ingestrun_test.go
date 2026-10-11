package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// ingestRunDaemon is an httptest daemon for EW-70: its ingest tool answers a
// path ingest past its wait budget with the run that serves it (EW-66), and
// answers `ingest {service, run}` from runStates in turn (the last repeats).
// /indexed answers indexed. It counts the ingests it queued.
func ingestRunDaemon(t *testing.T, pastBudget string, runStates []string, indexed bool) (*httptest.Server, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	var queued, polls atomic.Int32
	text := func(w http.ResponseWriter, inner string) {
		b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}})
		_, _ = w.Write(b)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/indexed"):
			_, _ = io.WriteString(w, map[bool]string{true: `{"indexed":true}`, false: `{"indexed":false}`}[indexed])
		case r.URL.Path == "/rpc":
			var req struct {
				Params struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				} `json:"params"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			switch {
			case req.Params.Name == "ingest" && req.Params.Arguments["run"] != nil:
				if _, has := req.Params.Arguments["path"]; has {
					t.Errorf("a run poll carried a path: it would queue an ingest")
				}
				n := int(polls.Add(1)) - 1
				if n >= len(runStates) {
					n = len(runStates) - 1
				}
				text(w, runStates[n])
			case req.Params.Name == "ingest":
				queued.Add(1)
				text(w, pastBudget)
			case req.Params.Name == "list_services":
				text(w, `{"services":[{"service":"svc","symbol_count":42}]}`)
			default:
				http.NotFound(w, r)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts, &queued, &polls
}

const pastBudgetAnswer = `{"service":"svc","completed":false,"state":"running","waited":"2m0s","run":"r-7","note":"still running"}`

// TestInitStopsWaitingWhenDaemonReportsIngestFailed is EW-70: when the
// daemon answers init's ingest past its wait budget, init polls the run the
// answer names (`ingest {service, run}`, EW-66) and stops the moment the run
// reads done with ok=false (failed, refused, dropped by the gate), printing
// a MANUAL line that names the daemon's error, instead of polling the
// indexed probe for 30 minutes; it never queues a second ingest.
func TestInitStopsWaitingWhenDaemonReportsIngestFailed(t *testing.T) {
	ts, queued, polls := ingestRunDaemon(t, pastBudgetAnswer, []string{
		`{"service":"svc","run":"r-7","completed":false,"state":"running"}`,
		`{"service":"svc","run":"r-7","completed":true,"state":"done","ok":false,"error":"ingest refused: swap volume free < 5 GiB"}`,
	}, false)
	var out strings.Builder
	start := time.Now()
	ok, err := initIngestWith(ts.URL, "svc", "/repo", &out, initIngestTiming{request: 5 * time.Second, pollEvery: 10 * time.Millisecond, pollFor: 30 * time.Second})
	if err != nil {
		t.Fatalf("a failed run read as daemon-down: %v\n%s", err, out.String())
	}
	if ok {
		t.Fatalf("a failed run read as indexed:\n%s", out.String())
	}
	if el := time.Since(start); el > 10*time.Second {
		t.Errorf("init waited %s after the daemon reported the run failed", el)
	}
	if !strings.Contains(out.String(), "MANUAL:") || !strings.Contains(out.String(), "swap volume free < 5 GiB") || !strings.Contains(out.String(), "r-7") {
		t.Errorf("want a MANUAL line naming the run and the daemon's error:\n%s", out.String())
	}
	if queued.Load() != 1 || polls.Load() != 2 {
		t.Errorf("ingests queued %d, run polls %d: want one ingest and two polls", queued.Load(), polls.Load())
	}
}

// A run that finishes ok prints the existing ok line; a run the daemon has
// no record of (it restarted) falls back to the indexed probe.
func TestInitIngestRunDoneOKAndUnknown(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state string
	}{
		{"done ok", `{"service":"svc","run":"r-7","completed":true,"state":"done","ok":true}`},
		{"unknown after a restart", `{"service":"svc","run":"r-7","completed":false,"state":"unknown"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts, queued, _ := ingestRunDaemon(t, pastBudgetAnswer, []string{tc.state}, true)
			var out strings.Builder
			ok, err := initIngestWith(ts.URL, "svc", "/repo", &out, initIngestTiming{request: 5 * time.Second, pollEvery: 10 * time.Millisecond, pollFor: 5 * time.Second})
			if err != nil || !ok {
				t.Fatalf("ok=%v err=%v, want indexed:\n%s", ok, err, out.String())
			}
			if !strings.Contains(out.String(), `ok ingest (service "svc" indexed`) {
				t.Errorf("want the ok line:\n%s", out.String())
			}
			if queued.Load() != 1 {
				t.Errorf("ingests queued %d, want 1", queued.Load())
			}
		})
	}
}

// A past-budget answer from a daemon that names no run (before EW-66)
// falls back to the indexed probe, never to a second ingest.
func TestInitIngestPastBudgetWithoutRunWaitsForIndexed(t *testing.T) {
	ts, queued, polls := ingestRunDaemon(t, `{"service":"svc","completed":false,"state":"running"}`, []string{`{}`}, true)
	var out strings.Builder
	ok, err := initIngestWith(ts.URL, "svc", "/repo", &out, initIngestTiming{request: 5 * time.Second, pollEvery: 10 * time.Millisecond, pollFor: 5 * time.Second})
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v:\n%s", ok, err, out.String())
	}
	if queued.Load() != 1 || polls.Load() != 0 {
		t.Errorf("ingests queued %d, run polls %d: want 1 and 0", queued.Load(), polls.Load())
	}
}

// init's ingest request outlasts the daemon's own wait budget and its RPC
// write deadline, so the past-budget answer, and the run it names, reaches
// init (before EW-70 init gave up at 60 s, under the 120 s budget, and lost
// the run id).
func TestInitIngestRequestOutlastsDaemonWaitBudget(t *testing.T) {
	if initIngestRequestTimeout <= daemonRPCWriteTimeout || initIngestRequestTimeout <= mcp.DefaultIngestWaitBudget {
		t.Fatalf("initIngestRequestTimeout %s must exceed the daemon's write deadline %s and ingest wait budget %s", initIngestRequestTimeout, daemonRPCWriteTimeout, mcp.DefaultIngestWaitBudget)
	}
}
