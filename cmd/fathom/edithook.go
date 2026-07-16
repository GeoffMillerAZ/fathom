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

	// A relative file path would bypass the root-refusal check below (it
	// cannot be relativized against a root); absolutize it so OP-3 always
	// sees a real path. Claude Code sends absolute paths, so this is a
	// defensive normalization, not a hot path.
	if !filepath.IsAbs(filePath) {
		a, err := filepath.Abs(filePath)
		if err != nil {
			return 0 // silent
		}
		filePath = a
	}

	// Try each service-identity candidate (dir basename, then the enclosing
	// git repo for subdirectory-rooted projects), resolving the file against
	// that candidate's root. First candidate with rules (or a stale flag)
	// wins; the first candidate that merely RELATIVIZES supplies the
	// service/file identity for spec-line injection, which fires with or
	// without obligations (OP-4: "silent on none" means no spec IDs, not no
	// obligations — post-recompile, narrower scopes make rule-less files the
	// common case). A file that relativizes under NO candidate root is
	// refused outright (O8 addendum): never query the daemon with an
	// out-of-repo path.
	var (
		obl         semantic.Obligations
		indexStale  bool
		repoRelFile string
		svcName     string
		found       bool
		resolved    bool
	)
	for _, cand := range serviceCandidates(projDir) {
		rel, ok := relativeToRoot(cand.root, filePath)
		if !ok {
			continue
		}
		if !resolved {
			resolved, svcName, repoRelFile = true, strings.ToLower(cand.name), rel
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
	if !resolved {
		emitBeat("edit-hook", "answered", map[string]string{"emitted": "false"})
		return 0 // out-of-repo file (OP-3) → silent, daemon never queried
	}

	// Build compact text.
	var sb strings.Builder
	if found {
		fmt.Fprintf(&sb, "⚖️ fathom obligations — %s:", repoRelFile)
		for _, r := range obl.Rules {
			// FT-2's last hop: non-obligation kinds ("finding") are labeled in
			// the bracket, and remediation drift (FT-6) is surfaced — a
			// serving annotation the agent must see, never a status flip
			// (drifted code can mean the finding was fixed).
			extra := ""
			if r.Kind != "" {
				extra += ", " + r.Kind
			}
			if r.Status != "" {
				extra += ", " + r.Status
			}
			if r.CodeDrifted {
				extra += ", code drifted since compile"
			}
			fmt.Fprintf(&sb, "\n• %s [%s, %s%s]", r.Rule, r.Src, r.Tier, extra)
		}
		if indexStale {
			sb.WriteString("\n⚠️ index is behind your working tree for this file")
		}
	}

	// OP-4 (spec-line injection, docs/vision/04 §4 item 3): when the edited
	// file cites bracketed spec IDs, tie those knowledge concepts to code by
	// appending where each one is actually defined — beneath the obligations
	// payload when there is one, as a payload of their own when there isn't.
	// The ≤150-token budget covers the WHOLE payload and spec lines drop
	// first: when obligations alone already spend it, skip every spec_refs
	// RPC up front rather than paying hook latency for lines that cannot
	// fit. Silent on any error (hook doctrine): a missing/unreadable file, a
	// pattern error, or a daemon error simply means no spec lines, never a
	// failure of the obligations payload already built above.
	if cost.Estimate(sb.String()) <= specLineTokenBudget {
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
						header := ""
						if sb.Len() == 0 {
							header = fmt.Sprintf("⚖️ fathom spec refs — %s:", repoRelFile)
						}
						line := fmt.Sprintf("\n▸ implements [%s] — defined at %s:%d", id, loc.File, loc.Line)
						if cost.Estimate(sb.String()+header+line) > specLineTokenBudget {
							continue // budget already spent; spec lines drop first
						}
						sb.WriteString(header)
						sb.WriteString(line)
						break // one defining location per spec ID
					}
				}
			}
		}
	}

	if sb.Len() == 0 {
		emitBeat("edit-hook", "answered", map[string]string{"emitted": "false"})
		return 0 // clean file (or unindexed project), no spec IDs → silent
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

// relativeToRoot relativizes an absolute file path under a candidate root.
// It first tries the cheap lexical form; when that refuses, it retries with
// BOTH sides symlink-resolved. The lexical-only check false-refused whole
// sessions on symlinked layouts (e.g. ~/code → /Volumes/…/code, or macOS
// /var → /private/var): gitToplevel returns the PHYSICAL work-tree path
// while the session may address files through the symlink — or vice versa —
// so filepath.Rel sees two unrelated prefixes and every in-repo file
// ..-escapes. Pre-OP-3 that fallback-degraded; with root refusal it would
// be a fatal recall loss, hence the physical retry. Returns ok=false only
// when the file lives under the root by neither reading.
func relativeToRoot(root, file string) (string, bool) {
	if r, err := filepath.Rel(root, file); err == nil && !escapesRoot(r) {
		return r, true
	}
	if r, err := filepath.Rel(resolveSymlinks(root), resolveSymlinks(file)); err == nil && !escapesRoot(r) {
		return r, true
	}
	return "", false
}

// escapesRoot reports whether a filepath.Rel result points outside the root
// (".." or "../…"), without catching a root-level entry whose name merely
// begins with two dots (e.g. "..foo").
func escapesRoot(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// resolveSymlinks best-effort resolves symlinks in p. When p itself cannot
// be resolved (e.g. the path does not exist), its parent directory is
// resolved and the base rejoined; on any failure the lexical path is
// returned unchanged — never an error, per the hook's silent discipline.
func resolveSymlinks(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	if r, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
		return filepath.Join(r, filepath.Base(p))
	}
	return p
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
