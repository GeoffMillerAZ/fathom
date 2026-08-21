package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	// IT-3: the total-silence gate used to be assistiveChannelsDisabled(),
	// which is ALSO true under an active OFF arm — so an armed session
	// exited here before emitting a single beat, deleting the counting-only
	// baseline (seen / observer beats) the arm needs to prove it ran at
	// all. fathomChannelsDisabled() alone (the raw env kill switch) is the
	// only condition that must skip beat emission too — matching
	// session-hook and stop-hook's own "NOT assistiveChannelsDisabled"
	// gates at this exact position.
	if fathomChannelsDisabled() {
		return 0 // raw kill switch: total silence, no beat, no ledger row
	}
	// armOff (env kill switch OR an active OFF arm) silences INJECTION and
	// OUTPUT only, from here down: stdout must stay byte-identical to a
	// fully-disabled run (empty), and no daemon call may be made, but the
	// counting-only beats below still fire, stamped "arm":"off".
	armOff := assistiveChannelsDisabled()
	rpcOrigin = "edit-hook"
	fs := newFlagSet("edit-hook", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	pre := fs.Bool("pre", false, "run as the PreToolUse guard hook (docs/design/guarded-edits.md) instead of the default PostToolUse obligations hook")
	if err := fs.Parse(args); err != nil {
		// On flag error: silent exit 0 (hook contract).
		return 0
	}

	// Read the hook JSON from stdin — the same envelope shape for
	// PostToolUse and PreToolUse (Claude Code's generic hook contract);
	// tool_name/tool_input.command are populated only for the PreToolUse
	// path (--pre), session_id is read defensively (guardstamp.go's
	// leakage-bound documents why "" is a safe, honest fallback).
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		return 0 // silent
	}

	var hook struct {
		ToolName  string `json:"tool_name"`
		ToolInput struct {
			FilePath string `json:"file_path"`
			Command  string `json:"command"`
		} `json:"tool_input"`
		CWD       string `json:"cwd"`
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(raw, &hook); err != nil {
		return 0 // silent
	}

	if *pre {
		return cmdEditHookPre(hook.ToolName, hook.ToolInput.FilePath, hook.ToolInput.Command, hook.CWD, hook.SessionID, *url, armOff, stdout)
	}

	filePath := hook.ToolInput.FilePath
	if filePath == "" {
		return 0
	}
	emitBeat("edit-hook", "seen", firingMetaArm(map[string]string{"phase": "post"}, hook.SessionID, filePath, armOff)) // GI-1: the coverage denominator
	if armOff {
		// Counting-only baseline stops here: no daemon call, no stdout —
		// injection/output stays fully silenced under OFF (IT-3).
		emitBeat("edit-hook", "answered", firingMetaArm(map[string]string{"emitted": "false", "phase": "post"}, hook.SessionID, filePath, armOff))
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
	// O8 addendum: an out-of-repo file must produce TOTAL silence — no
	// daemon traffic at all — so the daemon-resolved identity (one
	// list_services call, dogfood O15) is fetched only after a basename
	// candidate root actually contains the file.
	// Capability enforcement (capabilityenforce.go): `capabilities set
	// obligations off` must actually stop injection, not merely record
	// intent. Placed BEFORE any daemon traffic so an off capability is
	// genuinely silent, not silently-still-querying.
	if capabilityOffForFile(projDir, filePath, "obligations") {
		return 0
	}

	cands := serviceCandidates(projDir)
	containsFile := false
	for _, cand := range cands {
		if _, ok := relativeToRoot(cand.root, filePath); ok {
			containsFile = true
			break
		}
	}
	if containsFile {
		cands = candidatesWithDaemon(&http.Client{Timeout: 2 * time.Second}, *url, projDir)
	}
	for _, cand := range cands {
		rel, ok := relativeToRoot(cand.root, filePath)
		if !ok {
			continue
		}
		if !resolved {
			resolved, svcName, repoRelFile = true, strings.ToLower(cand.name), rel
		}
		name := strings.ToLower(cand.name)
		o, stale, ok := fetchObligations(*url, name, rel, 0, 2*time.Second)
		if !ok {
			emitBeat("edit-hook", "error", firingMeta(map[string]string{"phase": "post"}, hook.SessionID, filePath))
			return 0 // daemon down or error → silent
		}
		if len(o.Rules) > 0 || stale {
			obl, indexStale, repoRelFile, svcName, found = o, stale, rel, name, true
			break
		}
	}
	if !resolved {
		emitBeat("edit-hook", "answered", firingMeta(map[string]string{"emitted": "false", "phase": "post"}, hook.SessionID, filePath))
		return 0 // out-of-repo file (OP-3) → silent, daemon never queried
	}

	// Instrument B's edit-set exclusion (docs/design/production-
	// tokenomics.md §4, editset.go): mark this file as just-edited by
	// this session, regardless of whether there are obligations to
	// render — a subsequent same-session Read of it is verification, not
	// discovery, and must not inflate the discovery-share denominator.
	markFileEdited(hook.SessionID, repoRelFile, time.Now())

	// Guarded-edits dedup (docs/design/guarded-edits.md §2.3): when the
	// --pre hook already served rules for this exact (session, service,
	// file) within guardStampTTL, render only the DELTA here — rules newly
	// matched since the pre-hook fired, plus any staleness change —
	// otherwise collapse to a one-line "already served pre-edit" marker.
	// No stamp found (the overwhelmingly common case: no guard matched
	// this file, or the --pre hook isn't installed/didn't fire) renders
	// the full answer exactly as before this feature existed.
	renderRules := obl.Rules
	suppressedByStamp := 0
	staleChangedSincePre := false
	stampConsulted := false
	if stamp, ok := readGuardStamp(hook.SessionID, svcName, repoRelFile, time.Now()); ok {
		stampConsulted = true
		dedup := dedupAgainstStamp(stamp, toObligationLike(obl.Rules), indexStale)
		renderRules = fromObligationLike(dedup.DeltaRules, obl.Rules)
		suppressedByStamp = dedup.SuppressedCount
		staleChangedSincePre = dedup.StaleChanged
		// Critic finding 6 (phase-O merge): count actual suppressed RULES,
		// not just invocations, and cover the PARTIAL-dedup case too (some
		// rules delta-rendered, some already-served) — not only full
		// suppression. The count travels in beat meta; research_report.go
		// sums it.
		if suppressedByStamp > 0 {
			emitBeat("edit-hook", "dedup-suppressed", map[string]string{"phase": "post", "count": strconv.Itoa(suppressedByStamp)})
		}
		// Critic finding 9: a stamp is single-use — once consulted for
		// this render, delete it so it can never be replayed for a
		// second, unrelated edit of the same file.
		deleteGuardStamp(hook.SessionID, svcName, repoRelFile)
	}

	// Build compact text.
	var sb strings.Builder
	if found {
		if stampConsulted && suppressedByStamp > 0 && len(renderRules) == 0 && !staleChangedSincePre {
			// Never silent (design §2.3): the marker line always names the
			// suppression, even when there is nothing else to say.
			fmt.Fprintf(&sb, "⚖️ fathom obligations — %s: guarded — %d rule(s) already served pre-edit, unchanged", repoRelFile, suppressedByStamp)
		} else {
			fmt.Fprintf(&sb, "⚖️ fathom obligations — %s:", repoRelFile)
			for _, r := range renderRules {
				writeRuleBullet(&sb, r)
			}
			if suppressedByStamp > 0 {
				fmt.Fprintf(&sb, "\nguarded: %d rule(s) already served pre-edit, not repeated", suppressedByStamp)
			}
			// T5-2/O14: the cap can silently evict served-eligible rules on a
			// recompile that blends in a new layer — surface the count (a
			// single integer; never the evicted rules themselves, which would
			// reopen the exact unbounded-growth risk spec-line injection above
			// already guards against).
			if obl.Displaced > 0 {
				fmt.Fprintf(&sb, "\n⚠️ %d more eligible rule(s) not shown (cap)", obl.Displaced)
			}
			if indexStale {
				sb.WriteString("\n⚠️ index is behind your working tree for this file")
			}
		}
		// Platform-modules.md §3.3: a bound-but-unmounted module is an
		// explicit note, never silence — surfaced on the post channel
		// unconditionally (independent of any guard's include_modules,
		// which only affects the --pre preview's rendering).
		for _, n := range obl.ModuleNotes {
			fmt.Fprintf(&sb, "\n⚠️ module %s: %s", n.Module, n.Note)
		}
		reportModuleBeats(renderRules, "post", hook.SessionID)
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
		emitBeat("edit-hook", "answered", firingMeta(map[string]string{"emitted": "false", "phase": "post"}, hook.SessionID, filePath))
		return 0 // clean file (or unindexed project), no spec IDs → silent
	}
	emitPayloadCost("post-edit", sb.String(), len(renderRules), "post", hook.SessionID)

	out, _ := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PostToolUse",
			"additionalContext": sb.String(),
		},
	})
	answeredMeta := map[string]string{"emitted": "true", "phase": "post"}
	// IT-6: the cap-pressure count already rendered into the payload above
	// (T5-2/O14) must also reach the beat log — a COUNT only, never the
	// evicted rules themselves — so obligations-cap pressure becomes a
	// trendable number rather than a fact visible only in one hook's stdout.
	if obl.Displaced > 0 {
		answeredMeta["displaced"] = strconv.Itoa(obl.Displaced)
	}
	emitBeat("edit-hook", "answered", firingMeta(answeredMeta, hook.SessionID, filePath))
	recordStoryRow(projDir, "obligation-served", map[string]any{
		"file": repoRelFile, "rules": len(renderRules), "session": hook.SessionID,
	})
	fmt.Fprintln(stdout, string(out))
	return 0
}

// writeRuleBullet appends one rendered rule bullet to sb, in the exact
// compact format the edit-hook has always used — factored out so the
// --pre (guard) path and the --post delta-rendering path (guardstamp.go's
// dedup) share one rendering rule rather than two copies drifting apart.
// A module-tagged rule (r.Module != "", federated in from a bound platform
// module — docs/design/platform-modules.md §3.3) renders its module name
// as a leading bracket, exactly like the module-host federation surface
// already does; a repo-local rule (the overwhelmingly common case, and
// the ENTIRE case before phase O) is byte-identical to the pre-existing
// rendering.
func writeRuleBullet(sb *strings.Builder, r semantic.Obligation) {
	// FT-2's last hop: non-obligation kinds ("finding") are labeled in
	// the bracket, and remediation drift (FT-6) is surfaced — a
	// serving annotation the agent must see, never a status flip
	// (drifted code can mean the finding was fixed). T5-2/O14: a
	// rule from a NON-default layer (e.g. "adr-rules" blended in by
	// a recompile) is labeled too — the default "obligations" layer
	// is elided here to keep the common case compact; the field
	// itself is always present in the raw JSON payload.
	extra := ""
	if r.Kind != "" {
		extra += ", " + r.Kind
	}
	if r.Layer != "" && r.Layer != semantic.LayerObligations {
		extra += ", " + r.Layer
	}
	if r.Status != "" {
		extra += ", " + r.Status
	}
	if r.CodeDrifted {
		extra += ", code drifted since compile"
	}
	if r.Module != "" {
		fmt.Fprintf(sb, "\n• [%s] %s [%s, %s%s]", r.Module, r.Rule, r.Src, r.Tier, extra)
	} else {
		fmt.Fprintf(sb, "\n• %s [%s, %s%s]", r.Rule, r.Src, r.Tier, extra)
	}
}

// toObligationLike/fromObligationLike bridge semantic.Obligation and
// guardstamp.go's minimal semanticObligationLike (which intentionally does
// not import internal/semantic — see its doc comment). fromObligationLike
// re-resolves each delta entry back to its FULL semantic.Obligation (for
// every field writeRuleBullet needs, not just Rule/Src/Module) by matching
// on ruleKey against the original full-fidelity slice.
func toObligationLike(rules []semantic.Obligation) []semanticObligationLike {
	out := make([]semanticObligationLike, len(rules))
	for i, r := range rules {
		out[i] = semanticObligationLike{Rule: r.Rule, Src: r.Src, Module: r.Module}
	}
	return out
}

func fromObligationLike(delta []semanticObligationLike, full []semantic.Obligation) []semantic.Obligation {
	if len(delta) == 0 {
		return nil
	}
	byKey := make(map[string]semantic.Obligation, len(full))
	for _, r := range full {
		byKey[ruleKey(r.Rule, r.Src, r.Module)] = r
	}
	out := make([]semantic.Obligation, 0, len(delta))
	for _, d := range delta {
		if r, ok := byKey[ruleKey(d.Rule, d.Src, d.Module)]; ok {
			out = append(out, r)
		}
	}
	return out
}

// reportModuleBeats emits one "module-served" beat per DISTINCT module
// name present in rendered — the per-module instrumentation dimension
// (docs/design/platform-modules.md §3.5 / guarded-edits.md §2.4): cheap
// (bounded by MaxObligations, at most a handful of distinct modules per
// call) and never fired at all when no module-tagged rule was rendered
// (the pre-phase-O default). Critic finding 6 (phase-O merge): the design
// asks for RULES served per module, not serve-invocation counts — each
// beat carries the module's rule count from THIS call in its "rules" meta
// (research_report.go sums it), rather than one beat implicitly meaning
// "one rule."
func reportModuleBeats(rendered []semantic.Obligation, phase, sessionID string) {
	counts := map[string]int{}
	var order []string
	var moduleSB strings.Builder
	moduleRuleCount := 0
	for _, r := range rendered {
		if r.Module == "" {
			continue
		}
		if _, seen := counts[r.Module]; !seen {
			order = append(order, r.Module)
		}
		counts[r.Module]++
		// Instrument A (docs/design/production-tokenomics.md §3): the
		// "module rules" payload-cost channel is the bytes specifically
		// attributable to module-tagged bullets within whichever payload
		// (pre or post) rendered them — re-rendering via the SAME
		// writeRuleBullet the real payload used keeps this byte-identical
		// to what was actually injected, never an approximation.
		writeRuleBullet(&moduleSB, r)
		moduleRuleCount++
	}
	for _, m := range order {
		emitBeat("edit-hook", "module-served", map[string]string{"module": m, "phase": phase, "rules": strconv.Itoa(counts[m])})
	}
	emitPayloadCost("module-rules", moduleSB.String(), moduleRuleCount, phase, sessionID)
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

// fetchObligations calls the obligations MCP tool over HTTP. maxRules is
// forwarded as the tool's max_rules argument when > 0 (the guard --pre
// path passes a guard's own max_rules cap; the default post path passes 0,
// letting the server apply its own default — identical to this function's
// pre-phase-O behavior). timeout is this call's own HTTP client timeout —
// critic finding 5 (phase-O merge): the --pre path makes this call SECOND,
// after resolveServiceAndRel's own request, so a hardcoded 2s here on top
// of that call's own 2s could double the guard hook's worst-case latency
// past the design's stated <=2s budget; edithook_pre.go now passes the
// REMAINING share of one shared 2s budget instead of a fresh 2s. The post
// path (edithook.go) is unaffected — it always passes its own pre-existing
// 2*time.Second, byte-identical to this function's behavior before this
// parameter existed. Returns the Obligations payload, the index_stale
// flag, and ok=false on any error.
func fetchObligations(baseURL, service, file string, maxRules int, timeout time.Duration) (semantic.Obligations, bool, bool) {
	args := map[string]any{
		"service": service,
		"file":    file,
	}
	if maxRules > 0 {
		args["max_rules"] = maxRules
	}
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "obligations",
			"arguments": args,
		},
	})
	if err != nil {
		return semantic.Obligations{}, false, false
	}

	hc := &http.Client{Timeout: timeout}
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
		Displaced           int                   `json:"displaced"`
		DisplacedRepo       int                   `json:"displaced_repo"`
		DisplacedModule     int                   `json:"displaced_module"`
		ModuleNotes         []semantic.ModuleNote `json:"module_notes"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return semantic.Obligations{}, false, false
	}

	obl := semantic.Obligations{
		Service:         payload.Service,
		File:            payload.File,
		Rules:           payload.Rules,
		Watermark:       payload.CompiledAtWatermark,
		Displaced:       payload.Displaced,
		DisplacedRepo:   payload.DisplacedRepo,
		DisplacedModule: payload.DisplacedModule,
		ModuleNotes:     payload.ModuleNotes,
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
