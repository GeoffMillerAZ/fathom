package main

import (
	"os"
	"path/filepath"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

// beatLogPath resolves the central hooks beat log (GI-1): FATHOM_HOOKS_LOG,
// defaulting to ~/.fathom/hooks.jsonl. Hooks fire in every fleet repo, so the
// log is per-user central, never per-repo.
func beatLogPath() string {
	if p := os.Getenv("FATHOM_HOOKS_LOG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".fathom", "hooks.jsonl")
}

// emitBeat appends one channel beat to the hooks beat log. It is the
// activity/effect denominator for forced channels (docs/vision/04 Gap 1).
// ALL failures are swallowed: hooks never block, and instrumentation must
// never be the reason a hook breaks its silent-exit contract.
func emitBeat(channel, kind string, meta map[string]string) {
	path := beatLogPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	l, err := heartbeat.NewLog(f, time.Now)
	if err != nil {
		return
	}
	_ = l.Beat(channel, kind, meta)
}
