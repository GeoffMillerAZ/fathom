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

	"github.com/geoffmilleraz/fathom/internal/cost"
	"github.com/geoffmilleraz/fathom/internal/sdd"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// specLineTokenBudget is the ≤150-token discipline for the whole edit-hook
// payload (docs/design/coding-agent-excellence.md, ADR.md#L241): obligations
// rules are already word-budgeted at compile time, so spec-line injection
// (OP-4) is the one part of the payload that can still grow unboundedly (one
// line per cited spec ID) — it is the part dropped first when the budget is
// already spent.
const specLineTokenBudget = 150

// maxSpecIDsPerEdit caps how many distinct spec IDs cited in one edited file
// trigger a spec_refs lookup — a file citing a dozen specs must not turn one
// edit into a dozen RPCs.
const maxSpecIDsPerEdit = 3

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
	rpcOrigin = "edit-hook"
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
	emitBeat("edit-hook", "seen", nil) // GI-1: the coverage denominator

	// Determine project dir.
	projDir := os.Getenv("CLAUDE_PROJECT_DIR")
	if projDir == "" {
		projDir = hook.CWD
	}
	if projDir == "" {
		projDir, _ = os.Getwd()
	}

	// Try each service-identity candidate (dir basename, then the enclosing
	// git repo for subdirectory-rooted projects), resolving the file against
	// that candidate's root. First candidate with something to say wins; a
	// candidate the index doesn't know simply yields an empty payload.
	var (
		obl         semantic.Obligations
		indexStale  bool
		repoRelFile string
		svcName     string
		found       bool
	)
	for _, cand := range serviceCandidates(projDir) {
		rel := filePath
		if filepath.IsAbs(filePath) {
			r, relErr := filepath.Rel(cand.root, filePath)
			if relErr != nil || strings.HasPrefix(r, "..") {
				// The file does not relativize under this candidate's root
				// at all — O8 addendum: this used to fall back to the RAW
				// absolute path and still query the daemon with it (e.g. an
				// edit outside every indexed repo). Never query with an
				// out-of-repo path; try the next candidate instead.
				continue
			}
			rel = r
		}
		for _, name := range withNameAliases(cand.name) {
			o, stale, ok := fetchObligations(*url, strings.ToLower(name), rel)
			if !ok {
				emitBeat("edit-hook", "error", nil)
				return 0 // daemon down or error → silent
			}
			if len(o.Rules) > 0 || stale {
				obl, indexStale, repoRelFile, svcName, found = o, stale, rel, strings.ToLower(name), true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		emitBeat("edit-hook", "answered", map[string]string{"emitted": "false"})
		return 0 // clean file (or unindexed project) → silent
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

	// OP-4 (spec-line injection, docs/vision/04 §4 item 3): when the edited
	// file cites bracketed spec IDs, tie those knowledge concepts to code by
	// appending where each one is actually defined — beneath the obligations
	// payload, dropped first if the ≤150-token budget is already spent.
	// Silent on any error (hook doctrine): a missing/unreadable file, a
	// pattern error, or a daemon error simply means no spec lines, never a
	// failure of the obligations payload already built above.
	if raw, rerr := os.ReadFile(filePath); rerr == nil {
		if ids, ierr := sdd.ExtractRefsFromText(string(raw), ""); ierr == nil && len(ids) > 0 {
			if len(ids) > maxSpecIDsPerEdit {
				ids = ids[:maxSpecIDsPerEdit]
			}
			for _, id := range ids {
				specs, ok := fetchSpecRefs(*url, svcName, id)
				if !ok {
					continue
				}
				for _, loc := range specs[id] {
					if loc.File == repoRelFile {
						continue // the citing file itself, not the definition
					}
					line := fmt.Sprintf("\n▸ implements [%s] — defined at %s:%d", id, loc.File, loc.Line)
					if cost.Estimate(sb.String()+line) > specLineTokenBudget {
						continue // budget already spent; spec lines drop first
					}
					sb.WriteString(line)
					break // one defining location per spec ID
				}
			}
		}
	}

	out, _ := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PostToolUse",
			"additionalContext": sb.String(),
		},
	})
	emitBeat("edit-hook", "answered", map[string]string{"emitted": "true"})
	fmt.Fprintln(stdout, string(out))
	return 0
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
	req, err := originHeader(baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return semantic.Obligations{}, false, false
	}
	resp, err := hc.Do(req)
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

// specRefLoc is one {file, line} location the spec_refs MCP tool reports for
// a spec ID.
type specRefLoc struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// fetchSpecRefs calls the spec_refs MCP tool for a single normalised spec ID
// (OP-4), returning its {spec_id: [{file, line}]} map and ok=false on any
// error — mirrors fetchObligations's silent-on-error contract.
func fetchSpecRefs(baseURL, service, spec string) (map[string][]specRefLoc, bool) {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "spec_refs",
			"arguments": map[string]any{
				"service": service,
				"spec":    spec,
			},
		},
	})
	if err != nil {
		return nil, false
	}

	hc := &http.Client{Timeout: 2 * time.Second}
	req, err := originHeader(baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, false
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, false
	}

	inner, err := unwrapRPCResult(raw)
	if err != nil {
		return nil, false
	}

	var payload struct {
		Specs map[string][]specRefLoc `json:"specs"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return nil, false
	}
	return payload.Specs, true
}
