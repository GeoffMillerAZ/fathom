package main

// portholder.go — a held port (design §8.4 step 5, RT-43). A managed
// daemon relaunched while its predecessor still serves the port must not
// crash-loop: a managed fathom of the same label that answers already
// serves it — log it and exit 0 (launchd stays down under schema 2). Any
// other holder exits 75 (counted by the breaker as port_held) and is named
// — pid and command — for `fathom resources`, doctor and `daemon reap`.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// portHolderFacts is what serve-http could learn about the holder.
type portHolderFacts struct {
	ListenerPID int    // lsof's listener, 0 when unknown
	ListenerCmd string // its command line
	Record      *console.DaemonRecord
	Responds    bool // GET /healthz answered
}

// resolvePortHolder decides a held port: (0, port_held, "already served by
// pid N …") for a managed fathom of self's label that answers; else (75,
// port_held, the holder named).
func resolvePortHolder(selfManaged bool, selfLabel string, f portHolderFacts) (code int, reason, detail string) {
	if selfManaged && selfLabel != "" && f.Record != nil && f.Record.Managed && f.Record.ID == selfLabel &&
		f.Record.PID > 0 && (f.ListenerPID == 0 || f.ListenerPID == f.Record.PID) && f.Responds {
		return tenancy.ExitStayDown, tenancy.ExitPortHeld, fmt.Sprintf("already served by pid %d (label %s) — exiting 0", f.Record.PID, selfLabel)
	}
	holder := "an unknown process (lsof found no listener)"
	if f.ListenerPID > 0 {
		holder = fmt.Sprintf("pid %d (%s)", f.ListenerPID, firstNonEmpty(f.ListenerCmd, "command unknown"))
	}
	why := "not a managed fathom of this label"
	switch {
	case f.Record != nil && f.Record.ID != selfLabel:
		why = fmt.Sprintf("its record names label %q", f.Record.ID)
	case f.Record != nil && f.ListenerPID > 0 && f.Record.PID != f.ListenerPID:
		why = fmt.Sprintf("the record's pid is %d", f.Record.PID)
	case f.Record != nil && !f.Responds:
		why = "it does not answer /healthz"
	case !selfManaged:
		why = "this daemon is unmanaged"
	}
	return tenancy.ExitRetryLater, tenancy.ExitPortHeld, fmt.Sprintf("port held by %s — %s; exiting %d (fathom daemon reap handles a scratch holder)", holder, why, tenancy.ExitRetryLater)
}

// gatherPortHolder reads the holder's facts: the listener's pid and
// command, the daemon record for addr, and whether /healthz answers.
func gatherPortHolder(addr string, ops daemonOps) portHolderFacts {
	var f portHolderFacts
	f.ListenerPID = ops.ListenerPID(addr)
	if f.ListenerPID > 0 {
		if pi, ok := ops.Proc(f.ListenerPID); ok {
			f.ListenerCmd = pi.Command
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rec, ok := findDaemonRecord(home, addr); ok {
			f.Record = &rec
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+dialableAddr(addr)+"/healthz", nil); err == nil {
		if resp, err := http.DefaultClient.Do(req); err == nil {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
			_ = resp.Body.Close()
			f.Responds = healthzIsFathom(resp.StatusCode, b)
		}
	}
	return f
}

// healthzIsFathom reports whether a /healthz answer is a serving fathom's:
// 200 "ok", or 503 "waiting-for-store: …" — a predecessor waiting for its
// store (RT-42) still holds and serves the port, so its successor is not a
// counted port_held.
func healthzIsFathom(status int, body []byte) bool {
	s := string(body)
	return (status == http.StatusOK && s == "ok") ||
		(status == http.StatusServiceUnavailable && strings.HasPrefix(s, "waiting-for-store: "))
}
