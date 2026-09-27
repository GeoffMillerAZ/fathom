package tenancy

// quarantine.go — quarantine records (design §2.1, §2.3; RT-155). A service
// whose ingest was halted at the exit mark, or was in flight when the
// daemon exited over budget, is written to the lifecycle ledger as a
// `quarantine` row. ensure(), the refresh queue and warm-up never re-queue
// it; /statusz, `resources` and the primer name it; an explicit ingest
// clears it with a `quarantine_clear` row. Without this,
// ingest_state = in_progress would re-ingest the same service at every
// start, exit again, and walk the breaker to a fleet-wide stop.

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
)

// Ledger row kinds this file writes and reads.
const (
	RowQuarantine      = "quarantine"
	RowQuarantineClear = "quarantine_clear"
)

// QuarantineRow is a lifecycle-ledger line (`row` names its kind, the
// ledger's shape; schema_version as I32's pattern).
type QuarantineRow struct {
	SchemaVersion int    `json:"schema_version"`
	FathomVersion string `json:"fathom_version,omitempty"`
	Row           string `json:"row"`
	At            string `json:"at"`
	PID           int    `json:"pid,omitempty"`
	Service       string `json:"service"`
	Footprint     int64  `json:"footprint,omitempty"`
	Reason        string `json:"reason,omitempty"`
	By            string `json:"by,omitempty"`
}

// Quarantine is one quarantined service.
type Quarantine struct {
	Service   string
	Since     time.Time
	Footprint int64
	Reason    string
}

// Attention is the one-line form /statusz, `resources` and the primer use.
func (q Quarantine) Attention() string {
	return fmt.Sprintf("quarantined: %s %s — fathom ingest %s to retry", q.Service, q.Reason, q.Service)
}

// QuarantineReasonIngest is the reason every exit-rung quarantine carries.
const QuarantineReasonIngest = "exceeded budget during ingest"

// NewQuarantineRow builds a quarantine row.
func NewQuarantineRow(service string, footprint int64, pid int, at time.Time, version string) QuarantineRow {
	return QuarantineRow{SchemaVersion: 1, FathomVersion: version, Row: RowQuarantine, At: at.UTC().Format(time.RFC3339Nano),
		PID: pid, Service: service, Footprint: footprint, Reason: QuarantineReasonIngest}
}

// NewQuarantineClearRow builds the row an explicit ingest writes.
func NewQuarantineClearRow(service, by string, at time.Time, version string) QuarantineRow {
	return QuarantineRow{SchemaVersion: 1, FathomVersion: version, Row: RowQuarantineClear, At: at.UTC().Format(time.RFC3339Nano),
		Service: service, By: by}
}

// QuarantinedServices folds a lifecycle ledger into the services still
// quarantined, sorted by service. Rows of other kinds are skipped; a line
// that is not JSON (a torn last line after a kill) is skipped and counted.
// A read error is returned, never folded into "none quarantined" (I38).
func QuarantinedServices(ledger io.Reader) (qs []Quarantine, skipped int, err error) {
	cur := map[string]Quarantine{}
	sc := bufio.NewScanner(ledger)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var r QuarantineRow
		if json.Unmarshal(line, &r) != nil {
			skipped++
			continue
		}
		switch r.Row {
		case RowQuarantine:
			if r.Service == "" {
				skipped++
				continue
			}
			at, _ := time.Parse(time.RFC3339Nano, r.At)
			cur[r.Service] = Quarantine{Service: r.Service, Since: at, Footprint: r.Footprint, Reason: r.Reason}
		case RowQuarantineClear:
			delete(cur, r.Service)
		}
	}
	if serr := sc.Err(); serr != nil && !errors.Is(serr, io.EOF) {
		return nil, skipped, serr
	}
	for _, q := range cur {
		qs = append(qs, q)
	}
	sort.Slice(qs, func(i, j int) bool { return qs[i].Service < qs[j].Service })
	return qs, skipped, nil
}

// The process's quarantine set: loaded from the ledger at start, grown by
// the exit rung, cleared by an explicit ingest. ensure() and (from wave D)
// the refresh queue read IsQuarantined before queueing a service.
var (
	quarantineMu sync.Mutex
	quarantined  = map[string]Quarantine{}
	clearer      func(service, by string)
)

// SetQuarantined replaces the set (nil clears it).
func SetQuarantined(qs []Quarantine) {
	quarantineMu.Lock()
	defer quarantineMu.Unlock()
	quarantined = map[string]Quarantine{}
	for _, q := range qs {
		quarantined[q.Service] = q
	}
}

// AddQuarantined adds one service; it reports false when it was already
// quarantined.
func AddQuarantined(q Quarantine) bool {
	quarantineMu.Lock()
	defer quarantineMu.Unlock()
	if _, ok := quarantined[q.Service]; ok {
		return false
	}
	quarantined[q.Service] = q
	return true
}

// IsQuarantined reports whether service is quarantined.
func IsQuarantined(service string) bool {
	quarantineMu.Lock()
	defer quarantineMu.Unlock()
	_, ok := quarantined[service]
	return ok
}

// QuarantinedNow is the set, sorted by service.
func QuarantinedNow() []Quarantine {
	quarantineMu.Lock()
	out := make([]Quarantine, 0, len(quarantined))
	for _, q := range quarantined {
		out = append(out, q)
	}
	quarantineMu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Service < out[j].Service })
	return out
}

// SetQuarantineClearer registers what ClearQuarantine records (the daemon
// appends a quarantine_clear row to its ledger).
func SetQuarantineClearer(f func(service, by string)) {
	quarantineMu.Lock()
	clearer = f
	quarantineMu.Unlock()
}

// ClearQuarantine lifts service's quarantine (an explicit ingest, by names
// who asked) and records it; a service that was not quarantined is a no-op.
func ClearQuarantine(service, by string) bool {
	quarantineMu.Lock()
	_, ok := quarantined[service]
	delete(quarantined, service)
	f := clearer
	quarantineMu.Unlock()
	if ok && f != nil {
		f(service, by)
	}
	return ok
}
