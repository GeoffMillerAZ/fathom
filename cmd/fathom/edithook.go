package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// cmdEditHook implements the "edit-hook" subcommand: a Claude Code PostToolUse
// hook that injects obligation rules into session context whenever a file is
// edited.
//
// Reads a PostToolUse JSON from stdin:
//
//	{"tool_input":{"file_path":"..."},"cwd":"..."}
//
// Output policy (ADR-016 §5.3 — hooks never block, silence is free):
//   - ANY error / timeout / daemon-down → print nothing, exit 0.
//   - No rules + index not stale → print nothing, exit 0.
//   - Otherwise → print hookSpecificOutput JSON with compact obligations text.
func cmdEditHook(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("edit-hook", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	if err := fs.Parse(args); err != nil {
		// On flag error: silent exit 0 (hook contract).
		return 0
	}

	// Read PostToolUse JSON from stdin.
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		return 0 // silent
	}

	var hook struct {
		ToolInput struct {
			FilePath string `json:"file_path"`
		} `json:"tool_input"`
		CWD string `json:"cwd"`
	}
	if err := json.Unmarshal(raw, &hook); err != nil {
		return 0 // silent
	}

	filePath := hook.ToolInput.FilePath
	if filePath == "" {
		return 0
	}

	// Determine project dir.
	projDir := os.Getenv("CLAUDE_PROJECT_DIR")
	if projDir == "" {
		projDir = hook.CWD
	}
	if projDir == "" {
		projDir, _ = os.Getwd()
	}

	// Service ID: basename of project dir with sextant/fathom alias treatment.
	service := serviceIDFromDir(projDir)

	// Make file repo-relative if absolute.
	repoRelFile := filePath
	if filepath.IsAbs(filePath) && projDir != "" {
		if rel, relErr := filepath.Rel(projDir, filePath); relErr == nil && !strings.HasPrefix(rel, "..") {
			repoRelFile = rel
		}
	}

	// Call obligations tool over HTTP. Short timeout — never block the hook.
	obl, indexStale, ok := fetchObligations(*url, service, repoRelFile)
	if !ok {
		return 0 // daemon down or error → silent
	}
	if len(obl.Rules) == 0 && !indexStale {
		return 0 // clean file → silent
	}

	// Build compact text.
	var sb strings.Builder
	fmt.Fprintf(&sb, "⚖️ fathom obligations — %s:", repoRelFile)
	for _, r := range obl.Rules {
		status := ""
		if r.Status != "" {
			status = ", " + r.Status + ""
		}
		fmt.Fprintf(&sb, "\n• %s [%s, %s%s]", r.Rule, r.Src, r.Tier, status)
	}
	if indexStale {
		sb.WriteString("\n⚠️ index is behind your working tree for this file")
	}

	out, _ := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PostToolUse",
			"additionalContext": sb.String(),
		},
	})
	fmt.Fprintln(stdout, string(out))
	return 0
}

// serviceIDFromDir derives the service ID from a project directory path,
// applying the same sextant/fathom alias that session-hook uses.
func serviceIDFromDir(projDir string) string {
	base := filepath.Base(strings.TrimRight(projDir, "/"))
	switch strings.ToLower(base) {
	case "sextant":
		return "fathom"
	case "fathom":
		return "fathom"
	}
	return base
}

// fetchObligations calls the obligations MCP tool over HTTP. Returns the
// Obligations payload, the index_stale flag, and ok=false on any error.
func fetchObligations(baseURL, service, file string) (semantic.Obligations, bool, bool) {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "obligations",
			"arguments": map[string]any{
				"service": service,
				"file":    file,
			},
		},
	})
	if err != nil {
		return semantic.Obligations{}, false, false
	}

	hc := &http.Client{Timeout: 2 * time.Second}
	resp, err := hc.Post(baseURL+"/rpc", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return semantic.Obligations{}, false, false
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return semantic.Obligations{}, false, false
	}

	inner, err := unwrapRPCResult(raw)
	if err != nil {
		return semantic.Obligations{}, false, false
	}

	// Decode the obligations payload.
	var payload struct {
		Service             string                `json:"service"`
		File                string                `json:"file"`
		Rules               []semantic.Obligation `json:"rules"`
		CompiledAtWatermark string                `json:"compiled_at_watermark"`
		IndexStale          bool                  `json:"index_stale"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return semantic.Obligations{}, false, false
	}

	obl := semantic.Obligations{
		Service:   payload.Service,
		File:      payload.File,
		Rules:     payload.Rules,
		Watermark: payload.CompiledAtWatermark,
	}
	return obl, payload.IndexStale, true
}
