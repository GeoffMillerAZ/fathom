package console

// calllog.go — the call log is resolved once (design §1, OC-2). The survey
// of 2026-09-25 found the daemon writing one log while its own pages
// rendered from another, and CLI verbs reading a log relative to whatever
// directory they ran in: the same verb reported 88,875 calls from one
// directory and 1,631 from another. ResolveCallLog is the one rule every
// metrics verb and every page now shares, and its answer names which rule
// chose the path.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CallLogRule names the rule that chose a call-log path.
type CallLogRule string

const (
	// RuleFlag: an explicit --call-log on this invocation.
	RuleFlag CallLogRule = "flag"
	// RuleEnv: FATHOM_CALL_LOG names the log for this process.
	RuleEnv CallLogRule = "env"
	// RuleDaemon: the log the daemon was configured to write (its own
	// --call-log, recorded in its daemon record) — the default whenever one
	// is known, because the daemon is the only writer.
	RuleDaemon CallLogRule = "daemon"
	// RuleService: a named service's own recorded root resolved the log
	// (J5's identity rule, used only when no daemon record exists).
	RuleService CallLogRule = "service"
	// RuleConfig: [daemon].call_log in config.toml.
	RuleConfig CallLogRule = "config"
	// RuleLegacy: the historic default path; the least trustworthy answer.
	RuleLegacy CallLogRule = "legacy"
	// RuleNone: nothing named a log (the daemon was started with logging off).
	RuleNone CallLogRule = "none"
)

// CallLogInputs are the candidates ResolveCallLog chooses among, most
// authoritative first. Empty fields are skipped.
type CallLogInputs struct {
	Flag string // --call-log on this invocation
	Env  string // FATHOM_CALL_LOG

	Daemon     string // the daemon's configured --call-log (absolute)
	DaemonNote string // where that came from: "serve-http pid 123 on 127.0.0.1:7575 since …"
	// DaemonOff: the daemon's record says it was started with logging off
	// (--call-log ""). It writes no log, so nothing after it may name one:
	// a config or legacy log would be a log this daemon never wrote.
	DaemonOff bool

	Service     string // J5: a named service's own root resolved this path
	ServiceNote string

	Config string // [daemon].call_log

	Legacy string // the historic default
	// LegacyProbes are alternatives tried, in order, when Legacy does not
	// exist (doctor's daemon-workspace candidates). The first existing one
	// replaces Legacy under the same rule, with the probe named in the note.
	LegacyProbes []string
	// Exists reports whether a path exists; nil uses os.Stat.
	Exists func(string) bool
}

// ResolvedCallLog is ResolveCallLog's answer: the path, the rule that chose
// it, and a note a page or verb renders beside any call-log figure.
type ResolvedCallLog struct {
	Path string      `json:"path"`
	Rule CallLogRule `json:"rule"`
	Note string      `json:"note"`
}

// ResolveCallLog picks the call log. An explicit flag or FATHOM_CALL_LOG
// wins (the operator named a file for this run); otherwise the daemon's
// configured log wins over config.toml, which wins over the legacy default.
// The note always says which rule applied, so a figure computed from the
// log can say where it came from.
func ResolveCallLog(in CallLogInputs) ResolvedCallLog {
	exists := in.Exists
	if exists == nil {
		exists = func(p string) bool { _, err := os.Stat(p); return err == nil }
	}
	switch {
	case in.Flag != "":
		return ResolvedCallLog{Path: in.Flag, Rule: RuleFlag, Note: "call log named by --call-log on this invocation"}
	case in.Env != "":
		return ResolvedCallLog{Path: in.Env, Rule: RuleEnv, Note: "call log named by FATHOM_CALL_LOG"}
	case in.DaemonOff:
		note := "the daemon was started with logging off (--call-log \"\") — call figures are not measured"
		if in.DaemonNote != "" {
			note += " (" + in.DaemonNote + ")"
		}
		return ResolvedCallLog{Rule: RuleNone, Note: note}
	case in.Daemon != "":
		note := "the daemon's configured call log"
		if in.DaemonNote != "" {
			note += " (" + in.DaemonNote + ")"
		}
		return ResolvedCallLog{Path: in.Daemon, Rule: RuleDaemon, Note: note}
	case in.Service != "":
		note := "no daemon record; resolved from the named service's recorded root"
		if in.ServiceNote != "" {
			note += " (" + in.ServiceNote + ")"
		}
		return ResolvedCallLog{Path: in.Service, Rule: RuleService, Note: note}
	case in.Config != "":
		return ResolvedCallLog{Path: in.Config, Rule: RuleConfig, Note: "no daemon record; [daemon].call_log in config.toml"}
	case in.Legacy != "":
		if !exists(in.Legacy) {
			for _, p := range in.LegacyProbes {
				if p != "" && exists(p) {
					return ResolvedCallLog{Path: p, Rule: RuleLegacy, Note: "no daemon record or configured log; found the daemon workspace's default log at " + p}
				}
			}
		}
		return ResolvedCallLog{Path: in.Legacy, Rule: RuleLegacy, Note: "no daemon record or configured log; the legacy default path — figures from it may not be the daemon's"}
	}
	return ResolvedCallLog{Rule: RuleNone, Note: "no call log is configured anywhere — call figures are not measured"}
}

// DaemonRecord is what a serve-http process records about itself when it
// starts, so a CLI verb run from any directory can find the log the daemon
// actually writes. Names and paths only.
//
// Schema 2 (ADR-047 §10.2, RT-166) lives at ~/.fathom/daemon/<id>/record.json
// and adds the fields `fathom daemon ps` joins with the live census; a
// schema-1 record (the legacy ~/.fathom/daemons/<host>-<port>.json) still
// reads, with those fields empty.
type DaemonRecord struct {
	SchemaVersion int    `json:"schema_version"`
	Addr          string `json:"addr"`
	PID           int    `json:"pid"`
	CallLog       string `json:"call_log"`
	Store         string `json:"store,omitempty"`
	BinaryCommit  string `json:"binary_commit,omitempty"`
	StartedAt     string `json:"started_at"`

	ID             string `json:"id,omitempty"`               // the record's directory name: label when managed, else addr-<port>-<pid>
	Managed        bool   `json:"managed,omitempty"`          // launchd/systemd-owned (§10.1)
	PPID           int    `json:"ppid,omitempty"`             // parent at start
	Exe            string `json:"exe,omitempty"`              // resolved executable path
	ExeInode       uint64 `json:"exe_inode,omitempty"`        // its inode at start (a replaced binary changes it)
	CmdlineSHA     string `json:"cmdline_sha,omitempty"`      // sha256 of the argv, hex
	IdleExit       string `json:"idle_exit,omitempty"`        // Go duration; "" = none (managed)
	TTL            string `json:"ttl,omitempty"`              // Go duration; "" = none (managed)
	HeartbeatAt    string `json:"heartbeat_at,omitempty"`     // rewritten every 60 s
	LastActivityAt string `json:"last_activity_at,omitempty"` // the last request, as of the heartbeat
	StateRoot      string `json:"state_root,omitempty"`       // the ~/.fathom/daemon/<id> directory
}

// DaemonRecordSchemaVersion is the record's wire version.
const DaemonRecordSchemaVersion = 2

// daemonRecordLegacySchema is the schema-1 record, still read for one
// release (RT-166).
const daemonRecordLegacySchema = 1

// Note renders the record's provenance for ResolvedCallLog.Note.
func (r DaemonRecord) Note() string {
	return fmt.Sprintf("recorded by serve-http pid %d on %s since %s", r.PID, r.Addr, r.StartedAt)
}

// DaemonRecordPath is where the daemon listening on addr records itself:
// <home>/.fathom/daemons/<host>-<port>.json, with localhost and the IPv6
// loopback folded into 127.0.0.1 so a verb's http://localhost:7575 finds the
// record a daemon listening on 127.0.0.1:7575 wrote.
func DaemonRecordPath(home, addr string) (string, error) {
	key, err := daemonKey(addr)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".fathom", "daemons", key+".json"), nil
}

// DaemonAddrFromURL turns a base URL (http://localhost:7575) into host:port.
func DaemonAddrFromURL(base string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", errors.New("console: daemon URL has no host: " + base)
	}
	return u.Host, nil
}

func daemonKey(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("console: daemon address %q: %w", addr, err)
	}
	switch host {
	case "", "localhost", "::1", "0.0.0.0", "::":
		host = "127.0.0.1"
	}
	key := host + "-" + port
	if strings.ContainsAny(key, `/\`) || strings.Contains(key, "..") {
		return "", fmt.Errorf("console: daemon address %q cannot name a record", addr)
	}
	return key, nil
}

// WriteDaemonRecord writes rec atomically (temp file + rename) to path.
func WriteDaemonRecord(path string, rec DaemonRecord) error {
	if rec.SchemaVersion == 0 {
		rec.SchemaVersion = DaemonRecordSchemaVersion
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadDaemonRecord reads the record at path. A missing file is an
// os.IsNotExist error — the honest "no daemon has recorded itself here".
func ReadDaemonRecord(path string) (DaemonRecord, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return DaemonRecord{}, err
	}
	var rec DaemonRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return DaemonRecord{}, fmt.Errorf("console: daemon record %s: %w", path, err)
	}
	if rec.SchemaVersion != DaemonRecordSchemaVersion && rec.SchemaVersion != daemonRecordLegacySchema {
		return DaemonRecord{}, fmt.Errorf("console: daemon record %s: schema_version %d, want %d or %d", path, rec.SchemaVersion, daemonRecordLegacySchema, DaemonRecordSchemaVersion)
	}
	return rec, nil
}

// NewDaemonRecord builds a record stamped with now.
func NewDaemonRecord(addr string, pid int, callLog, store, commit string, now time.Time) DaemonRecord {
	return DaemonRecord{
		SchemaVersion: DaemonRecordSchemaVersion, Addr: addr, PID: pid,
		CallLog: callLog, Store: store, BinaryCommit: commit,
		StartedAt: now.UTC().Format(time.RFC3339),
	}
}
