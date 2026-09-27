package tenancy

// lifecycle.go — the lifecycle ledger and the crash-loop breaker (design
// §2.3; RT-37, RT-38, RT-156). INC-23: launchd relaunched a 30 GB daemon
// five times in 17 minutes and nothing recorded why any of them ended.
// Every start and exit is a row in ~/.fathom/daemon/<id>/lifecycle.jsonl;
// a start with no following exit names its predecessor unclean at the next
// start (with jetsam or resource-report evidence when the system kept any);
// and the breaker, evaluated before the store or the bind, puts a daemon
// that keeps dying into safe mode and then stops it. Pure: the caller does
// the file I/O and injects the clock.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Lifecycle row kinds (§2.3). quarantine rows are quarantine.go's.
const (
	RowStart         = "start"
	RowExit          = "exit"
	RowAdopt         = "adopt"
	RowMode          = "mode"
	RowBreaker       = "breaker"
	RowStop          = "stop"
	RowStoreWait     = "store_wait"
	RowDeploy        = "deploy"
	RowOperatorReset = "operator_reset"
)

// Exit reasons (§2.3). The breaker counts over_budget, unclean and a
// port_held exit with a non-zero code (a foreign holder); every other
// reason — and an already-served port_held, code 0 — is not counted.
const (
	ExitOverBudget = "over_budget"
	ExitDrain      = "drain"
	ExitAdopt      = "adopt"
	ExitStop       = "stop"
	ExitUnclean    = "unclean"
	ExitParent     = "parent_exit"
	ExitIdle       = "idle"
	ExitTTL        = "ttl"
	ExitUnlinked   = "unlinked"
	ExitPortHeld   = "port_held"
	ExitSignal     = "signal" // an unmanaged daemon's SIGTERM/SIGINT/SIGHUP
)

// Daemon modes a start row records.
const (
	ModeNormal = "normal"
	ModeSafe   = "safe"
)

// LifecycleRow is one ledger line: the union of every row kind's fields
// (row names the kind; schema_version as I32's pattern). Readers skip the
// fields a kind does not carry.
type LifecycleRow struct {
	SchemaVersion int    `json:"schema_version"`
	FathomVersion string `json:"fathom_version,omitempty"`
	Row           string `json:"row"`
	At            string `json:"at"`
	PID           int    `json:"pid,omitempty"`

	// start
	BinaryCommit string `json:"binary_commit,omitempty"`
	Managed      bool   `json:"managed,omitempty"`
	PlistSchema  int    `json:"plist_schema,omitempty"`
	Mode         string `json:"mode,omitempty"`
	LedgerID     string `json:"id,omitempty"`

	// exit
	Code        *int     `json:"code,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Detail      string   `json:"detail,omitempty"`
	Uptime      string   `json:"uptime,omitempty"`
	LastSample  string   `json:"last_sample,omitempty"`
	TopInFlight []string `json:"top_in_flight,omitempty"`
	Diag        string   `json:"diag,omitempty"`
	Evidence    []string `json:"evidence,omitempty"`
	// NotCounted names why the breaker skips an exit it would otherwise
	// count (a deploy-intent kickstart).
	NotCounted string `json:"not_counted,omitempty"`

	// mode
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`

	// stop, breaker, operator_reset
	By    string `json:"by,omitempty"`
	Until string `json:"until,omitempty"`

	// store_wait
	Errno string `json:"errno,omitempty"`
	Label string `json:"label,omitempty"`

	// deploy
	ReplacedPID int    `json:"replaced_pid,omitempty"`
	BinaryHash  string `json:"binary_hash,omitempty"`

	// adopt (selfrestart), quarantine
	OldBinary string `json:"old_binary,omitempty"`
	NewBinary string `json:"new_binary,omitempty"`
	Service   string `json:"service,omitempty"`
	Footprint int64  `json:"footprint,omitempty"`
}

// Time is the row's timestamp (zero when unparseable).
func (r LifecycleRow) Time() time.Time {
	t, _ := time.Parse(time.RFC3339Nano, r.At)
	return t
}

// NewLifecycleRow stamps a row of kind at t.
func NewLifecycleRow(kind string, t time.Time, pid int, version string) LifecycleRow {
	return LifecycleRow{SchemaVersion: 1, FathomVersion: version, Row: kind, At: t.UTC().Format(time.RFC3339Nano), PID: pid}
}

// IntPtr is a helper for LifecycleRow.Code.
func IntPtr(v int) *int { return &v }

// LedgerRotateBytes is the size at which the ledger rotates, keeping one
// generation (<ledger>.1).
const LedgerRotateBytes = 1 << 20

// ReadLedger parses ledger lines in order. A line that is not JSON (a torn
// last line after a kill) or is overlong is skipped and counted; a read
// error is returned, never folded into "no rows" (I38).
func ReadLedger(r io.Reader) (rows []LifecycleRow, skipped int, err error) {
	rd := bufio.NewReaderSize(r, 64*1024)
	for {
		line, isPrefix, rerr := rd.ReadLine()
		if isPrefix {
			for isPrefix && rerr == nil {
				_, isPrefix, rerr = rd.ReadLine()
			}
			skipped++
			line = nil
		}
		if len(bytes.TrimSpace(line)) > 0 {
			var row LifecycleRow
			if json.Unmarshal(line, &row) != nil || row.Row == "" {
				skipped++
			} else {
				rows = append(rows, row)
			}
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				return rows, skipped, nil
			}
			return rows, skipped, rerr
		}
	}
}

// closesStart reports whether row ends the life that pid's start began.
func closesStart(row LifecycleRow, pid int) bool {
	return (row.Row == RowExit || row.Row == RowAdopt) && (row.PID == pid || row.PID == 0)
}

// UncleanPredecessor returns the newest start row that no exit or adopt
// row closes, when it is not self's own (§2.3: SIGKILL, jetsam,
// force-quit, a runtime fatal).
func UncleanPredecessor(rows []LifecycleRow, self int) (LifecycleRow, bool) {
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].Row != RowStart {
			continue
		}
		st := rows[i]
		if st.PID == self || st.PID == 0 {
			return LifecycleRow{}, false
		}
		for _, later := range rows[i+1:] {
			if closesStart(later, st.PID) {
				return LifecycleRow{}, false
			}
		}
		return st, true
	}
	return LifecycleRow{}, false
}

// EvidenceFile is one system diagnostic report the caller read (a
// JetsamEvent-*.ips or a fathom_*.diag resource report), newest first.
type EvidenceFile struct {
	Name    string
	Content []byte
}

// MaxEvidenceFiles bounds the evidence scan (§2.3: best effort, 20 files).
const MaxEvidenceFiles = 20

// DeployIntentWindow is how recent a deploy row naming the predecessor
// must be for its unclean death not to count (§2.3).
const DeployIntentWindow = 60 * time.Second

var (
	diagPIDRe   = regexp.MustCompile(`(?m)^PID:\s+(\d+)\s*$`)
	diagEventRe = regexp.MustCompile(`(?m)^Event:\s+(.+?)\s*$`)
)

// jetsamEvidence reads a JetsamEvent report (a JSON header line, then a
// JSON body) for pid: "jetsam: largestProcess" when pid's process was the
// largest, "jetsam: <reason>" when it was killed with one.
func jetsamEvidence(content []byte, pid int) string {
	_, body, found := bytes.Cut(content, []byte("\n"))
	if !found {
		return ""
	}
	var ev struct {
		LargestProcess string `json:"largestProcess"`
		Processes      []struct {
			Name   string `json:"name"`
			PID    int    `json:"pid"`
			Reason string `json:"reason"`
		} `json:"processes"`
	}
	if json.Unmarshal(body, &ev) != nil {
		return ""
	}
	for _, p := range ev.Processes {
		if p.PID != pid {
			continue
		}
		switch {
		case p.Reason != "":
			return "jetsam: " + p.Reason
		case p.Name != "" && p.Name == ev.LargestProcess:
			return "jetsam: largestProcess"
		default:
			return "jetsam: listed"
		}
	}
	return ""
}

// resourceEvidence reads a resource report (fathom_*.diag) for pid.
func resourceEvidence(content []byte, pid int) string {
	m := diagPIDRe.FindSubmatch(content)
	if m == nil || string(m[1]) != strconv.Itoa(pid) {
		return ""
	}
	if e := diagEventRe.FindSubmatch(content); e != nil {
		return "resource: " + string(e[1])
	}
	return "resource: report"
}

// SynthesizeUncleanExit builds the exit row the next start writes for an
// unclean predecessor (§2.3): reason unclean, the predecessor's last
// sample when the caller has one, the evidence the system kept for its pid
// (at most MaxEvidenceFiles scanned, each finding once), and not_counted
// when a deploy row naming that pid was written after its start and
// within DeployIntentWindow of now (a `task deploy` kickstart whose
// SIGTERM did not reach a drained exit, RT-156).
func SynthesizeUncleanExit(pred LifecycleRow, rows []LifecycleRow, files []EvidenceFile, now time.Time, version string) LifecycleRow {
	row := NewLifecycleRow(RowExit, now, pred.PID, version)
	row.Reason = ExitUnclean
	row.LastSample = pred.LastSample
	started := pred.Time()
	row.Detail = fmt.Sprintf("pid %d started %s has no exit row — SIGKILL, jetsam, force-quit or a runtime fatal (crash.log)", pred.PID, pred.At)
	seen := map[string]bool{}
	for i, f := range files {
		if i == MaxEvidenceFiles {
			break
		}
		var ev string
		if strings.HasPrefix(f.Name, "JetsamEvent") {
			ev = jetsamEvidence(f.Content, pred.PID)
		} else {
			ev = resourceEvidence(f.Content, pred.PID)
		}
		if ev != "" && !seen[ev] {
			seen[ev] = true
			row.Evidence = append(row.Evidence, ev+" ("+f.Name+")")
		}
	}
	if len(row.Evidence) > 0 {
		sort.Strings(row.Evidence)
	}
	for _, r := range rows {
		if r.Row != RowDeploy || r.ReplacedPID != pred.PID {
			continue
		}
		t := r.Time()
		if !t.Before(started) && !t.After(now) && now.Sub(t) <= DeployIntentWindow {
			row.NotCounted = fmt.Sprintf("deploy intent at %s (binary %s): a kickstart, not a crash", r.At, firstOr(r.BinaryHash, "unknown"))
		}
	}
	return row
}

func firstOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// Breaker defaults (§1 [tenancy.breaker]).
const (
	DefaultSafeAfter  = 3
	DefaultSafeWindow = 30 * time.Minute
	DefaultStopAfter  = 5
	DefaultStopWindow = 60 * time.Minute
	DefaultWarmup     = 120 * time.Second
	// SafeModeRecovery is how long safe mode must sit at normal before it
	// steps down (§2.3).
	SafeModeRecovery = 30 * time.Minute
)

// Resolved fills the defaults of every zero field.
func (b BreakerSettings) Resolved() BreakerSettings {
	if b.SafeAfter <= 0 {
		b.SafeAfter = DefaultSafeAfter
	}
	if b.SafeWindow <= 0 {
		b.SafeWindow = DefaultSafeWindow
	}
	if b.StopAfter <= 0 {
		b.StopAfter = DefaultStopAfter
	}
	if b.StopWindow <= 0 {
		b.StopWindow = DefaultStopWindow
	}
	if b.Warmup <= 0 {
		b.Warmup = DefaultWarmup
	}
	return b
}

// BreakerAction is what a starting daemon does.
type BreakerAction uint8

const (
	BreakerRun BreakerAction = iota
	BreakerSafe
	BreakerStop
)

func (a BreakerAction) String() string {
	switch a {
	case BreakerSafe:
		return "safe"
	case BreakerStop:
		return "stop"
	}
	return "run"
}

// BreakerVerdict is EvaluateBreaker's answer.
type BreakerVerdict struct {
	Action      BreakerAction
	Counted     int // counted exits in the stop window
	CountedSafe int // counted exits in the safe window
	Reason      string
}

// countedExit reports whether the breaker counts row, and the reason word.
func countedExit(row LifecycleRow, rows []LifecycleRow) bool {
	if row.Row != RowExit || row.NotCounted != "" {
		return false
	}
	switch row.Reason {
	case ExitOverBudget:
		return true
	case ExitPortHeld:
		return row.Code != nil && *row.Code != 0
	case ExitUnclean:
		// A deploy row naming the pid within the window before the
		// synthesized exit (rows an older writer synthesized without
		// not_counted).
		t := row.Time()
		for _, r := range rows {
			if r.Row == RowDeploy && r.ReplacedPID == row.PID {
				if dt := t.Sub(r.Time()); dt >= 0 && dt <= DeployIntentWindow {
					return false
				}
			}
		}
		return true
	}
	return false
}

// EvaluateBreaker is the crash-loop breaker (§2.3), evaluated at start
// after the kill switch and before the store or the bind: ≥ SafeAfter
// counted exits in SafeWindow → safe mode; ≥ StopAfter in StopWindow, or a
// safe-mode process that exited over budget within StopWindow → stop.
// Only rows after the newest operator_reset count. Pure.
func EvaluateBreaker(rows []LifecycleRow, now time.Time, cfg BreakerSettings) BreakerVerdict {
	cfg = cfg.Resolved()
	from := 0
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].Row == RowOperatorReset {
			from = i + 1
			break
		}
	}
	window := rows[from:]
	modeOf := map[int]string{}
	for _, r := range window {
		if r.Row == RowStart {
			modeOf[r.PID] = r.Mode
		}
	}
	var v BreakerVerdict
	stopKinds, safeKinds := map[string]int{}, map[string]int{}
	safeOverBudget := ""
	for _, r := range window {
		if !countedExit(r, window) {
			continue
		}
		age := now.Sub(r.Time())
		if age < 0 || age > cfg.StopWindow {
			continue
		}
		v.Counted++
		stopKinds[r.Reason]++
		if age <= cfg.SafeWindow {
			v.CountedSafe++
			safeKinds[r.Reason]++
		}
		if r.Reason == ExitOverBudget && modeOf[r.PID] == ModeSafe {
			safeOverBudget = fmt.Sprintf("pid %d exited over budget in safe mode at %s", r.PID, r.At)
		}
	}
	switch {
	case v.Counted >= cfg.StopAfter:
		v.Action = BreakerStop
		v.Reason = fmt.Sprintf("%d counted exits in %s (%s)", v.Counted, cfg.StopWindow, kindList(stopKinds))
	case safeOverBudget != "":
		v.Action = BreakerStop
		v.Reason = safeOverBudget
	case v.CountedSafe >= cfg.SafeAfter:
		v.Action = BreakerSafe
		v.Reason = fmt.Sprintf("%d counted exits in %s (%s)", v.CountedSafe, cfg.SafeWindow, kindList(safeKinds))
	}
	return v
}

func kindList(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s ×%d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

// SafeModeTracker steps safe mode down after SafeModeRecovery
// continuously at normal (§2.3).
type SafeModeTracker struct {
	Safe        bool
	normalSince time.Time
}

// Step records one ladder reading; it reports true exactly when safe mode
// ends at this step.
func (t *SafeModeTracker) Step(now time.Time, level Level) bool {
	if !t.Safe {
		return false
	}
	if level != LevelNormal {
		t.normalSince = time.Time{}
		return false
	}
	if t.normalSince.IsZero() {
		t.normalSince = now
		return false
	}
	if now.Sub(t.normalSince) >= SafeModeRecovery {
		t.Safe = false
		t.normalSince = time.Time{}
		return true
	}
	return false
}

// WarmupSlots is the slot table during warm-up (§2.3, RT-40): for warmup
// (DefaultWarmup when zero) after start, each heavy class — tree, fleet,
// ingest — gets one slot; point, index and egress are unchanged. It
// returns a copy; after warm-up the copy equals slots.
func WarmupSlots(slots map[Class]int, start, now time.Time, warmup time.Duration) map[Class]int {
	if warmup <= 0 {
		warmup = DefaultWarmup
	}
	out := make(map[Class]int, len(slots))
	for c, n := range slots {
		out[c] = n
	}
	if now.Sub(start) >= warmup {
		return out
	}
	for _, c := range []Class{ClassTree, ClassFleet, ClassIngest} {
		if n, ok := out[c]; !ok || n > 1 {
			out[c] = 1
		}
	}
	return out
}
