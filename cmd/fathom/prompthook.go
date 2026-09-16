package main

// prompthook.go — fathom prompt-hook: a Claude Code UserPromptSubmit hook
// (O57 items 4/5, the guards pilot's second reading). edit-hook can only
// answer "what governs this file" once something has already been
// written to it; a refusal-shaped task (ADR-025: no org-specific names)
// needs the governing rule BEFORE the first edit. Claude Code hands
// fathom the user's own prompt text on UserPromptSubmit — this verb pulls
// the paths, packages and bare symbol names the prompt itself mentions,
// resolves each through the index, and serves the union of rules that
// govern them: capped, deduped, and silent when nothing resolves.
//
// Same silence contract as every other hook (ADR-016 §5.3): any error,
// timeout, or an unindexed/empty prompt prints nothing and exits 0.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

const (
	// maxPromptCandidates bounds how many distinct path/package/symbol
	// tokens one prompt resolves — a long prompt pasting a stack trace or
	// a file listing must not turn one UserPromptSubmit into dozens of
	// RPCs (the same discipline maxSpecIDsPerEdit applies to edit-hook's
	// own spec-line injection).
	maxPromptCandidates = 5
	// maxPromptSymbolLookups additionally bounds the more expensive
	// find_symbol calls (one RPC per bare identifier token) separately
	// from path/package candidates, which cost one obligations call each
	// and no resolution round trip of their own.
	maxPromptSymbolLookups = 3
)

// promptHookDeadlineBudget is the ONE wall-clock budget every RPC this
// hook makes shares — symbol resolution, candidate-identity resolution,
// and one obligations call per resolved file all draw against it, rather
// than each getting its own fresh timeout. Up to nine sequential 3s
// calls against a 10s hook timeout was the actual defect (O57 review): a
// prompt naming several files could spend several multiples of a single
// call's own budget. 8s leaves headroom inside the 10s hook timeout for
// encode/decode and process startup. A var, not a const, so a test can
// shorten it against a deliberately slow fake daemon.
var promptHookDeadlineBudget = 8 * time.Second

// cmdPromptHook implements the "prompt-hook" subcommand.
func cmdPromptHook(args []string, stdout, stderr io.Writer) int {
	if fathomChannelsDisabled() {
		return 0 // raw kill switch: total silence, no beat, no ledger row
	}
	armOff := assistiveChannelsDisabled()
	rpcOrigin = "prompt-hook"
	fs := newFlagSet("prompt-hook", stderr)
	urlFlag := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	harnessFlag := addHarnessFlag(fs)
	if err := fs.Parse(args); err != nil {
		return 0 // hook contract: silent
	}
	url := *urlFlag
	codec, ok := resolveHookHarness(*harnessFlag, stderr)
	if !ok {
		return 0
	}

	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		return 0
	}
	hook, err := codec.DecodeInput(harness.EventUserPromptSubmit, raw)
	if err != nil {
		return 0
	}
	sessionID := hook.SessionID
	prompt := strings.TrimSpace(hook.Prompt)
	if prompt == "" {
		return 0
	}

	emitBeat("prompt-hook", "seen", firingMetaArm(map[string]string{}, sessionID, "", armOff))
	if armOff {
		// Counting-only baseline stops here, matching edit-hook's own
		// OFF-arm gate (IT-3): no daemon call, no stdout.
		emitBeat("prompt-hook", "answered", firingMetaArm(map[string]string{"emitted": "false"}, sessionID, "", armOff))
		return 0
	}

	projDir := resolveProjDirFor(codec.ID(), hook.Cwd)
	deadline := time.Now().Add(promptHookDeadlineBudget)
	text, meta, ok := renderPromptObligations(url, projDir, prompt, sessionID, deadline)
	if !ok {
		return 0 // renderPromptObligations already emitted the beat that names the silence
	}
	out, err := codec.EncodeOutput(harness.EventUserPromptSubmit, harness.HookOutput{Context: text})
	if err != nil {
		return 0
	}
	answeredMeta := map[string]string{"emitted": "true", "candidates": fmt.Sprint(meta.candidates), "files": fmt.Sprint(meta.files)}
	if meta.displaced > 0 {
		answeredMeta["displaced"] = fmt.Sprint(meta.displaced)
	}
	if meta.resolved < meta.files {
		answeredMeta["resolved"] = fmt.Sprint(meta.resolved)
	}
	emitBeat("prompt-hook", "answered", firingMetaArm(answeredMeta, sessionID, "", armOff))
	fmt.Fprintln(stdout, string(out))
	return 0
}

// promptObligationsMeta is what renderPromptObligations learned, for the
// answered beat.
type promptObligationsMeta struct {
	candidates int // tokens extracted from the prompt
	files      int // distinct files obligations were attempted for
	displaced  int
	resolved   int // of files, how many got a definite answer before the deadline
}

// renderPromptObligations builds the UserPromptSubmit payload for prompt as
// harness-free text: extract candidate names, resolve each to a
// (service, file), fetch and merge obligations across every resolved
// file, cap and render. ok=false is the silence contract — nothing named,
// nothing resolved, nothing governs it, or the capability is off.
func renderPromptObligations(url, projDir, prompt, sessionID string, deadline time.Time) (string, promptObligationsMeta, bool) {
	if capabilityOffForFile(projDir, "", "obligations") {
		return "", promptObligationsMeta{}, false
	}
	candidates := extractPromptCandidates(prompt)
	if len(candidates) == 0 {
		return "", promptObligationsMeta{}, false
	}
	hc := &http.Client{Timeout: deadlineRemaining(deadline)}
	resolved := resolvePromptCandidates(hc, url, projDir, candidates, deadline)
	if len(resolved) == 0 {
		return "", promptObligationsMeta{}, false
	}
	svc := resolved[0].service // resolvePromptCandidates resolves ONE service per call
	files := make([]string, len(resolved))
	for i, r := range resolved {
		files[i] = r.file
	}
	m := mergeObligationsAcrossFiles(url, svc, files, deadline, func(f string) {
		emitBeat("prompt-hook", "error", firingMeta(map[string]string{"target": f}, sessionID, ""))
	})
	if len(m.rules) == 0 {
		emitBeat("prompt-hook", "answered", firingMeta(map[string]string{"emitted": "false"}, sessionID, ""))
		return "", promptObligationsMeta{}, false
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "⚖️ fathom obligations — this prompt names %s:", promptNamesLabel(m.namedFiles))
	for _, r := range m.rules {
		writeRuleBullet(&sb, r)
	}
	if m.displaced > 0 {
		fmt.Fprintf(&sb, "\n⚠️ %d more eligible rule(s) not shown (cap)", m.displaced)
	}
	if m.stale {
		sb.WriteString("\n⚠️ index is behind your working tree for one of these paths")
	}
	if m.resolved < m.total {
		fmt.Fprintf(&sb, "\n⚠️ partial: %d of %d resolved (time budget)", m.resolved, m.total)
	}
	return sb.String(), promptObligationsMeta{candidates: len(candidates), files: m.total, displaced: m.displaced, resolved: m.resolved}, true
}

// deadlineRemaining is deadline's remaining wall-clock time, floored at
// one nanosecond — an http.Client Timeout of exactly ZERO means
// UNLIMITED, never "no time left", so a caller with nothing left in its
// budget still gets a real (if minuscule) Timeout that fails the call
// almost immediately, rather than one with none at all.
func deadlineRemaining(deadline time.Time) time.Duration {
	if d := time.Until(deadline); d > 0 {
		return d
	}
	return time.Nanosecond
}

// promptNamesLabel renders the resolved files a prompt's rules actually
// came from, capped for readability — the payload names WHAT governed,
// this names WHERE it came from.
func promptNamesLabel(files []string) string {
	if len(files) == 0 {
		return "what it names"
	}
	const maxNamed = 3
	if len(files) > maxNamed {
		return strings.Join(files[:maxNamed], ", ") + fmt.Sprintf(" (+%d more)", len(files)-maxNamed)
	}
	return strings.Join(files, ", ")
}

// promptResolved is one candidate resolved to a concrete (service, file)
// obligations can be queried for.
type promptResolved struct {
	service, file, kind string
}

// resolvePromptCandidates resolves candidates against projDir's service —
// the daemon-resolved identity when reachable, else the basename guess
// (candidatesWithDaemon, the same fallback ladder edit-hook uses). Path
// and package tokens resolve locally (a prompt names REPO-RELATIVE text,
// not a filesystem path to relativize); symbol tokens cost one find_symbol
// RPC each, capped at maxPromptSymbolLookups AND stopped early once
// deadline arrives — every RPC here draws against the SAME shared budget
// the caller's later per-file obligations calls also draw against.
func resolvePromptCandidates(hc *http.Client, url, projDir string, candidates []promptCandidate, deadline time.Time) []promptResolved {
	cands := candidatesWithDaemon(hc, url, projDir)
	if len(cands) == 0 {
		return nil
	}
	svc := strings.ToLower(cands[0].name)
	seen := map[string]bool{}
	var out []promptResolved
	add := func(file string) {
		file = strings.TrimPrefix(strings.TrimSpace(file), "./")
		if file == "" || seen[file] {
			return
		}
		seen[file] = true
		out = append(out, promptResolved{service: svc, file: file})
	}
	symbolLookups := 0
	for _, c := range candidates {
		switch c.kind {
		case candidatePath:
			add(c.text)
		case candidatePackage:
			// BP-32/scopeFloor-style package globs anchor at the
			// directory ("<dir>/**"); a synthetic filename inside it
			// lets a bare package mention match the SAME scope a real
			// file there would, with no new matching logic in
			// semantic.ObligationsFor.
			add(strings.TrimSuffix(c.text, "/") + "/__prompt__.go")
		case candidateSymbol:
			if symbolLookups >= maxPromptSymbolLookups || !time.Now().Before(deadline) {
				continue // cap reached, or nothing left in the shared budget to spend on a lookup
			}
			symbolLookups++
			symHC := &http.Client{Timeout: deadlineRemaining(deadline)}
			if fp, ok := fetchSymbolFile(symHC, url, svc, c.text); ok {
				add(fp)
			}
		}
	}
	return out
}

// fetchSymbolFile resolves a bare symbol name to its defining file via the
// find_symbol MCP tool — ok=false on any error, no match, or an ambiguous
// name the tool declines to pick for (did_you_mean without an exact hit).
func fetchSymbolFile(hc *http.Client, baseURL, service, name string) (string, bool) {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{
			"name":      "find_symbol",
			"arguments": map[string]any{"service": service, "name": name},
		},
	})
	if err != nil {
		return "", false
	}
	req, err := originHeader(baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", false
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", false
	}
	inner, err := unwrapRPCResult(raw)
	if err != nil {
		return "", false
	}
	var payload struct {
		Symbols []struct {
			FilePath string `json:"file_path"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil || len(payload.Symbols) == 0 {
		return "", false
	}
	if payload.Symbols[0].FilePath == "" {
		return "", false
	}
	return payload.Symbols[0].FilePath, true
}
