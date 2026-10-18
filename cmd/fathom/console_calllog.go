package main

// console_calllog.go — OC-2's wiring: every metrics, instrument and
// flywheel verb, the fleet report and every Observatory page resolve the
// call log through console.ResolveCallLog (internal/console/calllog.go).
//
// The survey (2026-09-25) found the daemon rendering its pages from a log
// other than the one it writes, and verbs reading a log relative to their
// working directory. Two changes close it:
//
//   - serve-http records itself (~/.fathom/daemon/<id>/record.json since
//     RT-166; the legacy ~/.fathom/daemons/<host>-<port>.json still reads) once
//     its listener is bound: its absolute --call-log, store, pid, commit.
//     A verb run from any directory reads the record for the daemon it
//     talks to (FATHOM_HTTP, else localhost:7575) and gets the log the
//     daemon actually writes — rule "daemon".
//   - inside the daemon process the pages read the daemon's OWN --call-log
//     (servingDaemonCallLog), never the legacy default.
//
// An explicit --call-log or FATHOM_CALL_LOG still wins for its invocation;
// without a daemon record the pre-existing chain applies unchanged (J5's
// named-service identity, then [daemon].call_log, then the legacy default
// with doctor's daemon-workspace probes).

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// servingCallLog is the running daemon's own --call-log, absolute. It is
// set by runServeHTTP only after the listener is bound, so a serve-http
// invocation that fails early never leaks it into anything else.
var servingCallLog struct {
	sync.RWMutex
	set        bool
	path, note string
}

func setServingCallLog(path, note string) {
	servingCallLog.Lock()
	servingCallLog.set, servingCallLog.path, servingCallLog.note = true, path, note
	servingCallLog.Unlock()
}

// servingDaemonCallLog reports this process's own daemon log. set is true
// inside a serving daemon even when its --call-log is empty (logging off).
func servingDaemonCallLog() (path, note string, set bool) {
	servingCallLog.RLock()
	defer servingCallLog.RUnlock()
	return servingCallLog.path, servingCallLog.note, servingCallLog.set
}

// daemonRecordPathFor is the LEGACY record path for the daemon at baseURL
// (~/.fathom/daemons/<host>-<port>.json), read for one release (RT-166).
func daemonRecordPathFor(baseURL string) (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", false
	}
	addr, err := console.DaemonAddrFromURL(baseURL)
	if err != nil {
		return "", false
	}
	p, err := console.DaemonRecordPath(home, addr)
	if err != nil {
		return "", false
	}
	return p, true
}

// readDaemonRecordFor reads the record the daemon at baseURL wrote, if any.
// A record whose call_log is "" is still a record: that daemon runs with
// logging off (rule none), and it is still a previous daemon for the next
// start's restart/adopt event.
//
// RT-166: records live at ~/.fathom/daemon/<id>/record.json; the legacy
// ~/.fathom/daemons/<host>-<port>.json is still read (daemon_records.go).
func readDaemonRecordFor(baseURL string) (console.DaemonRecord, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return console.DaemonRecord{}, false
	}
	addr, err := console.DaemonAddrFromURL(baseURL)
	if err != nil {
		return console.DaemonRecord{}, false
	}
	return findDaemonRecord(home, addr)
}

// recordServingDaemon writes this daemon's record and remembers its own log
// for the in-process pages. Best effort: a daemon that cannot write its
// record still serves; verbs then fall back to the pre-existing chain.
// absStorePath is p made absolute against the working directory ("" stays "").
func absStorePath(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

func recordServingDaemon(addr, callLog, storePath, commit string, now time.Time) (console.DaemonRecord, error) {
	rec, _, err := recordServingDaemonAs(daemonMeta{PPID: os.Getppid()}, addr, callLog, storePath, commit, now)
	return rec, err
}

// callLogQuery parameterizes resolveConsoleCallLogFor: the invocation's own
// --call-log, a named --service (J5's identity rule), the daemon the verb
// talks to (default FATHOM_HTTP, else localhost:7575) and the root doctor's
// daemon-workspace probes start from (default ".").
type callLogQuery struct {
	Flag, Service, BaseURL, ProbeRoot string
}

// resolveConsoleCallLog is the one call-log resolution every verb and page
// uses. flag is the invocation's own --call-log ("" when not given);
// service is a named --service ("" when none) for J5's identity rule.
func resolveConsoleCallLog(flag, service string) console.ResolvedCallLog {
	return resolveConsoleCallLogFor(callLogQuery{Flag: flag, Service: service})
}

func resolveConsoleCallLogFor(q callLogQuery) console.ResolvedCallLog {
	in := console.CallLogInputs{Flag: q.Flag}
	if p, note, set := servingDaemonCallLog(); set {
		// Inside the daemon: the pages read what the daemon writes — and a
		// daemon started with logging off has no log to read (rule none).
		in.Daemon, in.DaemonNote, in.DaemonOff = p, note, p == ""
		return console.ResolveCallLog(in)
	}
	in.Env = os.Getenv("FATHOM_CALL_LOG")
	if in.Flag != "" || in.Env != "" {
		return console.ResolveCallLog(in)
	}
	base := q.BaseURL
	if base == "" {
		base = defaultFathomURL()
	}
	if rec, ok := readDaemonRecordFor(base); ok {
		// The record is the daemon's own answer, logging off included: a
		// verb must agree with the daemon's pages (P10), never fall through
		// to a config or legacy log that daemon does not write.
		in.Daemon, in.DaemonNote, in.DaemonOff = rec.CallLog, rec.Note(), rec.CallLog == ""
		return console.ResolveCallLog(in)
	}
	if q.Service != "" {
		hc := &http.Client{Timeout: 2 * time.Second}
		if p, root := resolveServiceCallLog(hc, base, q.Service); root != "" {
			in.Service, in.ServiceNote = p, "service "+q.Service+" at "+root
			return console.ResolveCallLog(in)
		}
	}
	if loaded, err := config.Load(config.DefaultUserPath(), config.DefaultProjectPath()); err == nil {
		in.Config = loaded.Config.Daemon.CallLog
	}
	in.Legacy = defaultCallLogPath()
	if in.Config == "" {
		if _, err := os.Stat(in.Legacy); err != nil {
			root := q.ProbeRoot
			if root == "" {
				root = "."
			}
			hc := &http.Client{Timeout: 2 * time.Second}
			in.LegacyProbes = resolveCallLogCandidates(hc, base, true, root)
		}
	}
	return console.ResolveCallLog(in)
}

// callLogFactsFor folds one read of the resolved log into the shared
// metric's facts (rows, unattributed rows, absence or read error).
func callLogFactsFor(resolved console.ResolvedCallLog, entries []adoptionCallEntry, readErr error) console.CallLogFacts {
	return foldCallLog(entries).facts(resolved, readErr)
}

// callLogFactsState is the facts' absence or read error, and whether the
// log was read (only then do its rows count).
func callLogFactsState(resolved console.ResolvedCallLog, readErr error) (console.CallLogFacts, bool) {
	f := console.CallLogFacts{Log: resolved}
	switch {
	case resolved.Path == "":
		return f, false
	case readErr != nil && os.IsNotExist(readErr):
		f.Missing = true
		return f, false
	case readErr != nil:
		f.ReadErr = readErr.Error()
		return f, false
	}
	return f, true
}

// callLogLine is the text line a verb prints naming the log it read and the
// rule that chose it (OC-2): "call log: <path> (rule daemon — …)".
func callLogLine(r console.ResolvedCallLog) string {
	path := r.Path
	if path == "" {
		path = "none"
	}
	return "call log: " + path + " (rule " + string(r.Rule) + " — " + r.Note + ")"
}

// callLogJSON is a verb's JSON value with the call-log resolution it read
// appended as "call_log": {path, rule, note} (OC-2) — additive, so every
// existing key keeps its name, meaning and order.
type callLogJSON struct {
	v   any
	log console.ResolvedCallLog
}

// withCallLog wraps v for JSON output with the resolution that chose its log.
func withCallLog(v any, log console.ResolvedCallLog) callLogJSON { return callLogJSON{v: v, log: log} }

// MarshalJSON appends "call_log" to v's object; a non-object v is written
// unchanged (there is no key to add it under).
func (c callLogJSON) MarshalJSON() ([]byte, error) {
	raw, err := json.Marshal(c.v)
	if err != nil {
		return nil, err
	}
	logJSON, err := json.Marshal(c.log)
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) < 2 || trimmed[0] != '{' {
		return raw, nil
	}
	body := bytes.TrimSpace(trimmed[1 : len(trimmed)-1])
	var b bytes.Buffer
	b.WriteByte('{')
	if len(body) > 0 {
		b.Write(body)
		b.WriteByte(',')
	}
	b.WriteString(`"call_log":`)
	b.Write(logJSON)
	b.WriteByte('}')
	return b.Bytes(), nil
}

// reviewCallLogPath is where `fathom review` appends its T5-7 gate row: the
// one resolution the readers use (the daemon's record first), so the
// forced-channel verdict lands in the log instrument --adoption counts —
// never a cwd-relative legacy path the readers no longer read.
func reviewCallLogPath(flag, service string) string {
	return resolveConsoleCallLog(flag, service).Path
}

// consoleOriginTransport stamps every request with the console's origin
// (mcp.OriginConsole), overriding any process origin: a read the console
// makes to render a page is the console reading itself, never usage.
type consoleOriginTransport struct{ base http.RoundTripper }

// RoundTrip stamps the console's origin. A request to this daemon's own
// bare JSON-RPC endpoint or its statusz is answered in process (RT-115:
// console pages make no loopback self-calls) — the RPC through the
// server's gate as origin console-inprocess, like every in-process call.
func (t consoleOriginTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if resp, ok := consoleInProcessRoundTrip(r); ok {
		return resp, nil
	}
	r = r.Clone(r.Context())
	r.Header.Set(originHeaderName, mcp.OriginConsole)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(r)
}

// consoleInProcessRoundTrip answers r in process when it asks this daemon
// for POST /rpc or GET /statusz; false otherwise (a CLI process, another
// daemon, any other route).
func consoleInProcessRoundTrip(r *http.Request) (*http.Response, bool) {
	if r.URL == nil {
		return nil, false
	}
	base := r.URL.Scheme + "://" + r.URL.Host
	reg := inProcessDaemon(base)
	if reg == nil {
		return nil, false
	}
	var body []byte
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/rpc" && reg.rpc != nil:
		if r.Body != nil {
			raw, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
			r.Body.Close()
			if err != nil {
				return nil, false
			}
			body = raw
		}
		resp, _ := inProcessCall(r.Context(), base, "", body)
		body = resp
	case r.Method == http.MethodGet && r.URL.Path == "/statusz" && r.URL.Query().Get("format") == "json" && !r.URL.Query().Has("root") && reg.statusz != nil:
		raw, err := json.Marshal(reg.statusz(r.Context()))
		if err != nil {
			return nil, false
		}
		body = raw
	default:
		return nil, false
	}
	status := http.StatusOK
	if len(body) == 0 {
		status = http.StatusNoContent
	}
	return &http.Response{
		Status: http.StatusText(status), StatusCode: status, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header:        http.Header{"Content-Type": []string{"application/json"}},
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)), Request: r,
	}, true
}

// consoleRPCClient is the HTTP client for the console's own reads of the
// daemon (the observatory model's registry read, the fleet page's report):
// the call log records them with the console's origin, and every usage
// reader leaves them out (classifyCallLogRead).
func consoleRPCClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: consoleOriginTransport{}}
}
