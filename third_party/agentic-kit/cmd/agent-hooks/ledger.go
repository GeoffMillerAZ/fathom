package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Ledger record kinds.
const (
	kindSpawn        = "spawn"         // a spawn decision (PreToolUse)
	kindSpawnResult  = "spawn_result"  // a finished Claude Code spawn (PostToolUse)
	kindSubagentStop = "subagent_stop" // a subagent's final report (SubagentStop)
	kindToolOutput   = "tool_output"   // the size of a tool result (PostToolUse)
	kindFault        = "fault"         // an internal fault the hook failed open on
)

// rotationLockTTL bounds how long a crashed rotation can block the next one.
const rotationLockTTL = time.Minute

// ledgerRecord is one JSONL line. It stores sizes, labels and outcomes, never
// prompt, command or output text: the ledger must be safe to keep and share.
type ledgerRecord struct {
	TS          string   `json:"ts"`
	Host        string   `json:"host"`
	Kind        string   `json:"kind"`
	Session     string   `json:"session,omitempty"`
	AgentID     string   `json:"agent_id,omitempty"` // set when a subagent made the call
	Tool        string   `json:"tool,omitempty"`
	AgentType   string   `json:"agent_type,omitempty"`
	Model       string   `json:"model,omitempty"`
	Effort      string   `json:"effort,omitempty"`
	Tier        string   `json:"tier,omitempty"`
	Class       string   `json:"class,omitempty"`
	Decision    string   `json:"decision,omitempty"`
	Codes       []string `json:"codes,omitempty"`
	Fork        bool     `json:"fork,omitempty"`
	ContextFork bool     `json:"context_fork,omitempty"`
	BriefChars  int      `json:"brief_chars,omitempty"`
	ParentModel string   `json:"parent_model,omitempty"`
	Resolved    string   `json:"resolved_model,omitempty"`
	TotalTokens int64    `json:"total_tokens,omitempty"`
	DurationMs  int64    `json:"duration_ms,omitempty"`
	ToolUses    int64    `json:"tool_uses,omitempty"`
	ReportChars int      `json:"report_chars,omitempty"`
	Status      string   `json:"status,omitempty"`
	Command     string   `json:"command,omitempty"` // program name only
	OutputChars int      `json:"output_chars,omitempty"`
	Contained   bool     `json:"contained,omitempty"`
	Notes       []string `json:"notes,omitempty"`      // advice keys delivered
	Transcript  string   `json:"transcript,omitempty"` // a finished subagent's transcript
	// SpawnedAgent is the id of the agent a spawn created (spawn_result), so
	// report can match it to the same agent's subagent_stop and count it once.
	SpawnedAgent string `json:"spawned_agent,omitempty"`
	Policy       string `json:"policy,omitempty"`
	Fault        string `json:"fault,omitempty"`
}

// ledgerPath is the active ledger file.
func (a *app) ledgerPath() string { return filepath.Join(a.stateDir(), "ledger.jsonl") }

// appendLedger writes one record. One O_APPEND write per record keeps
// concurrent hook processes from interleaving partial lines. Errors are
// ignored: measurement must never block the host. Doctor checks that the
// ledger is actually being written (invariant I19: a channel that can vanish
// silently needs an existence check).
func (a *app) appendLedger(cfg LedgerConfig, rec ledgerRecord) {
	if !cfg.Enabled {
		return
	}
	rec.capLabels()
	rec.TS = a.now().UTC().Format(time.RFC3339Nano)
	line, err := json.Marshal(rec)
	if err != nil {
		return
	}
	line = append(line, '\n')
	if err := os.MkdirAll(a.stateDir(), 0o700); err != nil {
		return
	}
	path := a.ledgerPath()
	if info, err := os.Stat(path); err == nil && info.Size() >= cfg.MaxBytes {
		a.rotateLedger(path, cfg.MaxBytes, cfg.KeepRotated)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = f.Write(line)
	_ = f.Close()
}

// capLabels bounds every host-supplied string so one oversized value cannot
// make a ledger line unreadable.
func (r *ledgerRecord) capLabels() {
	for _, s := range []*string{
		&r.Session, &r.AgentID, &r.Tool, &r.AgentType, &r.Model, &r.Effort,
		&r.Tier, &r.ParentModel, &r.Resolved, &r.Status, &r.Command, &r.SpawnedAgent, &r.Fault,
	} {
		*s = label(*s)
	}
	if len(r.Transcript) > 4096 {
		r.Transcript = "" // not a usable path anyway
	}
}

// rotateLedger renames the full ledger aside under an O_EXCL lock and keeps
// the newest keep rotated files. The size is checked again under the lock:
// a process that saw a full ledger before another one rotated it finds a
// fresh, small file and leaves it alone. That re-check is also what makes a
// stale-lock race harmless: two processes that both clear a stale lock can
// at worst both reach the check, and only one finds a full ledger.
func (a *app) rotateLedger(path string, maxBytes int64, keep int) {
	lock := path + ".lock"
	if info, err := os.Stat(lock); err == nil && a.now().Sub(info.ModTime()) > rotationLockTTL {
		_ = os.Remove(lock) // a crashed rotation left it behind
	}
	f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return // another process is rotating
	}
	_ = f.Close()
	defer os.Remove(lock)

	if info, err := os.Stat(path); err != nil || info.Size() < maxBytes {
		return // already rotated
	}
	rotated := strings.TrimSuffix(path, ".jsonl") + "-" + a.now().UTC().Format("20060102T150405.000000000Z") + ".jsonl"
	if os.Rename(path, rotated) != nil {
		return
	}
	old := rotatedLedgers(path)
	for len(old) > keep {
		_ = os.Remove(old[0])
		old = old[1:]
	}
}

// rotatedLedgers lists rotated ledger files oldest first. It reads the
// directory rather than globbing, so a state directory whose path contains
// glob metacharacters still works.
func rotatedLedgers(path string) []string {
	dir, base := filepath.Dir(path), strings.TrimSuffix(filepath.Base(path), ".jsonl")+"-"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if n := e.Name(); !e.IsDir() && strings.HasPrefix(n, base) && strings.HasSuffix(n, ".jsonl") {
			files = append(files, filepath.Join(dir, n))
		}
	}
	sort.Strings(files) // timestamped names sort oldest first
	return files
}

// ledgerFiles lists the rotated ledgers oldest first, then the active one.
func (a *app) ledgerFiles() []string {
	path := a.ledgerPath()
	files := rotatedLedgers(path)
	if _, err := os.Stat(path); err == nil {
		files = append(files, path)
	}
	return files
}
