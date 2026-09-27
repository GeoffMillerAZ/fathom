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
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/memory"
	"github.com/geoffmilleraz/fathom/internal/sdd"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// specLineTokenBudget is the ≤320-token discipline for the whole edit-hook
// payload (ADR-016 amendment 2026-09-24, Q21: the measured post-edit p90 —
// docs/measurements/2026-08-27-obligation-ceiling.md): obligations
// rules are already word-budgeted at compile time, so spec-line injection
// (OP-4) is the one part of the payload that can still grow unboundedly (one
// line per cited spec ID) — it is the part dropped first when the budget is
// already spent.
const specLineTokenBudget = 320

// maxSpecIDsPerEdit caps how many distinct spec IDs cited in one edited file
// trigger a spec_refs lookup — a file citing a dozen specs must not turn one
// edit into a dozen RPCs.
const maxSpecIDsPerEdit = 3

// reasonMetaMaxBytes caps the "reason" meta value a degradation beat
// carries (DG-7): never the whole error chain (which can embed a URL or
// response fragment), just enough for a human to tell failure branches
// apart in the beat log.
const reasonMetaMaxBytes = 200

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
	urlFlag := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	pre := fs.Bool("pre", false, "run as the PreToolUse guard hook (docs/design/guarded-edits.md) instead of the default PostToolUse obligations hook")
	harnessFlag := addHarnessFlag(fs)
	if err := fs.Parse(args); err != nil {
		// On flag error: silent exit 0 (hook contract).
		return 0
	}
	url := *urlFlag
	// The codec is TOLD which harness called (ADR-029 rule 6); absent ⇒
	// claude-code. An unknown id is I9 fail-open: one stderr line, exit 0.
	codec, ok := resolveHookHarness(*harnessFlag, stderr)
	if !ok {
		return 0
	}
	marker := "fathom edit-hook"
	if *pre {
		marker = "fathom edit-hook --pre"
	}
	endGuard, silent := beginHookGuard("edit-hook", marker, codec.ID(), url)
	if silent {
		return 0 // §9: the managed daemon is disabled or paused — no RPC, no output
	}
	defer endGuard()

	// Read the hook JSON from stdin and decode it through the codec — the
	// same envelope for the post-edit and the pre-edit (--pre) paths;
	// tool name / command are populated only for the PreToolUse path,
	// session_id is read defensively (guardstamp.go's leakage-bound
	// documents why "" is a safe, honest fallback).
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		return 0 // silent
	}
	ev := harness.EventPostTool
	if *pre {
		ev = harness.EventPreTool
	}
	hook, err := codec.DecodeInput(ev, raw)
	if err != nil {
		return 0 // silent
	}
	sessionID := hook.SessionID

	if *pre {
		return cmdEditHookPre(codec, hook, url, armOff, stdout)
	}

	filePath := hook.FilePath
	if filePath == "" {
		// O57 item 4/5(b): a Bash command that WRITES a file never matched
		// Edit|Write|MultiEdit's file_path, so it fell straight through
		// here and got nothing. The Bash matcher group (claudeCode.Hooks,
		// CapObligationsPostShell) routes here too, with tool_input.command
		// where tool_input.file_path would be — every other tool (a read,
		// a search) still has neither and stays silent, exactly as before.
		if hook.Tool == harness.ToolShell && strings.TrimSpace(hook.Command) != "" {
			return cmdEditHookBashWrite(codec, hook, url, armOff, stdout)
		}
		return 0
	}
	emitBeat("edit-hook", "seen", firingMetaArm(map[string]string{"phase": "post"}, sessionID, filePath, armOff)) // GI-1: the coverage denominator
	if armOff {
		// Counting-only baseline stops here: no daemon call, no stdout —
		// injection/output stays fully silenced under OFF (IT-3).
		emitBeat("edit-hook", "answered", firingMetaArm(map[string]string{"emitted": "false", "phase": "post"}, sessionID, filePath, armOff))
		return 0
	}

	// Determine project dir: the harness's env contract first, then the
	// envelope's cwd, then the process cwd.
	projDir := resolveProjDirFor(codec.ID(), hook.Cwd)

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

	// decode → payload → encode (HP-5): the payload is harness-free text;
	// only the envelope around it belongs to the codec.
	text, meta, ok := renderPostEdit(url, projDir, filePath, sessionID, hook.NewContent)
	text, ok = withClaimMoment(url, projDir, filePath, sessionID, text, ok) // PC-22: a newly checked [x] on a depth-1 plan verifies now (edithook_claim.go)
	text, ok = withSkipGainedNote(projDir, filePath, text, ok)              // O56 fix item 3: note a row gaining skip_test, daemon-free (edithook_skipnote.go)
	if !ok {
		return 0 // renderPostEdit already emitted the beat that names the silence
	}
	emitPayloadCost("post-edit", text, meta.rules, "post", sessionID, nil)

	out, err := codec.EncodeOutput(harness.EventPostTool, harness.HookOutput{Context: text})
	if err != nil {
		return 0 // a codec that cannot answer is silence (§5.3)
	}
	answeredMeta := map[string]string{"emitted": "true", "phase": "post"}
	// IT-6: the cap-pressure count already rendered into the payload above
	// (T5-2/O14) must also reach the beat log — a COUNT only, never the
	// evicted rules themselves — so obligations-cap pressure becomes a
	// trendable number rather than a fact visible only in one hook's stdout.
	if meta.displaced > 0 {
		answeredMeta["displaced"] = strconv.Itoa(meta.displaced)
	}
	// AM-6: memory surfacing is measurable on day one through the same
	// beat — a COUNT of memories shown and of those the budget displaced.
	if meta.memShown > 0 {
		answeredMeta["memories"] = strconv.Itoa(meta.memShown)
	}
	if meta.memDisplaced > 0 {
		answeredMeta["memories_displaced"] = strconv.Itoa(meta.memDisplaced)
	}
	if meta.findingsShown > 0 {
		answeredMeta["findings"] = strconv.Itoa(meta.findingsShown)
	}
	if meta.findingsOmitted > 0 {
		answeredMeta["findings_omitted"] = strconv.Itoa(meta.findingsOmitted)
	}
	emitBeat("edit-hook", "answered", firingMeta(answeredMeta, sessionID, filePath))
	recordStoryRow(projDir, "obligation-served", map[string]any{
		"file": meta.repoRelFile, "rules": meta.rules, "session": sessionID,
	})
	fmt.Fprintln(stdout, string(out))
	return 0
}

// postEditMeta is what renderPostEdit learned while rendering — the counts
// the answered beat and the story row carry beside the payload.
type postEditMeta struct {
	rules        int
	repoRelFile  string
	displaced    int
	memShown     int
	memDisplaced int
	// findingsShown/findingsOmitted (FL-20) are renderFindingsBlock's own
	// counts — 0/0 when the block never rendered (no finding-kind rows, no
	// overlap with the edited range, or the shared budget was already
	// spent).
	findingsShown   int
	findingsOmitted int
}

// renderPostEdit builds the PostToolUse obligations payload for one edited
// file as harness-free text (HP-5): obligations (delta-rendered against a
// guard stamp), spec-ref lines, anchored memories — every budget and every
// counting beat exactly as before the seam. ok=false is the silence
// contract: nothing to say (or a refused/out-of-repo file, or a daemon
// error) after the beat that names WHY has been emitted. The caller wraps
// the text in the calling harness's envelope.
func renderPostEdit(url, projDir, filePath, sessionID, newContent string) (string, postEditMeta, bool) {
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
		svcRoot     string
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
		return "", postEditMeta{}, false
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
		cands = candidatesWithDaemon(&http.Client{Timeout: 2 * time.Second}, url, projDir)
	}
	for _, cand := range cands {
		rel, ok := relativeToRoot(cand.root, filePath)
		if !ok {
			continue
		}
		if !resolved {
			resolved, svcName, repoRelFile, svcRoot = true, strings.ToLower(cand.name), rel, cand.root
		}
		name := strings.ToLower(cand.name)
		o, stale, ok, ferr := fetchObligations(url, name, rel, 0, 2*time.Second, semantic.ObligationLayerAll)
		if !ok {
			emitBeat("edit-hook", "error", firingMeta(map[string]string{"phase": "post", "reason": truncateBytes(ferr.Error(), reasonMetaMaxBytes)}, sessionID, filePath))
			return "", postEditMeta{}, false // daemon down or error → silent
		}
		if len(o.Rules) > 0 || stale || o.CoverageSkipped != "" {
			obl, indexStale, repoRelFile, svcName, svcRoot, found = o, stale, rel, name, cand.root, true
			break
		}
		// DG-11 (O15 regression): a daemon-verified candidate — one the
		// daemon itself records a live ServiceMeta.Path for — that answers
		// successfully IS this file's true identity, even when it happens
		// to carry no rules for this particular file. Trying further,
		// unverified basename/git-toplevel guesses past this point could
		// only misattribute the file to the wrong (stale, coincidentally
		// name-colliding) service — the exact shape of the O15 incident.
		// An unverified candidate's empty answer, by contrast, leaves the
		// loop open to try the next guess exactly as before this fix.
		if cand.verified {
			break
		}
	}
	if !resolved {
		emitBeat("edit-hook", "answered", firingMeta(map[string]string{"emitted": "false", "phase": "post"}, sessionID, filePath))
		return "", postEditMeta{}, false // out-of-repo file (OP-3) → silent, daemon never queried
	}

	// Instrument B's edit-set exclusion (docs/design/production-
	// tokenomics.md §4, editset.go): mark this file as just-edited by
	// this session, regardless of whether there are obligations to
	// render — a subsequent same-session Read of it is verification, not
	// discovery, and must not inflate the discovery-share denominator.
	markFileEdited(sessionID, repoRelFile, time.Now())

	// FL-20 (ADR-040 decision 5): finding-kind rows (Kind=="finding") ride
	// the SAME obligations call (layer=all, see fetchObligations above) but
	// are never governing rules — split them out here so the guard-stamp
	// dedup below, writeRuleBullet's loop, and reportModuleBeats all keep
	// seeing EXACTLY the governs-role rows they always have (the "byte-
	// identical when no findings exist" pin). findingRows is rendered
	// separately, after the governing-rules block, by renderFindingsBlock.
	govRules, findingRows := splitFindingRows(obl.Rules)

	// Guarded-edits dedup (docs/design/guarded-edits.md §2.3): when the
	// --pre hook already served rules for this exact (session, service,
	// file) within guardStampTTL, render only the DELTA here — rules newly
	// matched since the pre-hook fired, plus any staleness change —
	// otherwise collapse to a one-line "already served pre-edit" marker.
	// No stamp found (the overwhelmingly common case: no guard matched
	// this file, or the --pre hook isn't installed/didn't fire) renders
	// the full answer exactly as before this feature existed.
	renderRules := govRules
	suppressedByStamp := 0
	staleChangedSincePre := false
	stampConsulted := false
	if stamp, ok := readGuardStamp(sessionID, svcName, repoRelFile, time.Now()); ok {
		stampConsulted = true
		dedup := dedupAgainstStamp(stamp, toObligationLike(govRules), indexStale)
		renderRules = fromObligationLike(dedup.DeltaRules, govRules)
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
		deleteGuardStamp(sessionID, svcName, repoRelFile)
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
			writeRuleBullets(&sb, renderRules, svcName)
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
			} else if obl.IndexUnchecked != "" {
				sb.WriteString("\n⚠️ " + indexUncheckedLine(obl.IndexUnchecked))
			}
			// RT-146 / I38: the daemon skipped this hook's whole-tree read,
			// so coverage (and any raw fallback rule) is unknown, not absent.
			if obl.CoverageSkipped != "" {
				sb.WriteString("\n⚠️ " + obl.CoverageSkipped)
			}
		}
		// Platform-modules.md §3.3: a bound-but-unmounted module is an
		// explicit note, never silence — surfaced on the post channel
		// unconditionally (independent of any guard's include_modules,
		// which only affects the --pre preview's rendering).
		for _, n := range obl.ModuleNotes {
			fmt.Fprintf(&sb, "\n⚠️ module %s: %s", n.Module, n.Note)
		}
		reportModuleBeats(renderRules, "post", sessionID)
	}

	// FL-20 (docs/vision/30-findings-loop.md §3.4): the post-edit findings
	// block renders ONLY when the edited region overlaps a finding's cited
	// lines — editedRangeFromFile best-effort locates that region from the
	// tool's own proposed new_content against the file as it now stands
	// (the edit already landed by PostToolUse time). Gated behind the SAME
	// budget spec-lines/memories already share below: findings are
	// advisory, lowest priority after the governing rules that precede
	// them, so a payload that already spent its budget on rules drops
	// findings first rather than growing further.
	findingsShown, findingsOmitted := 0, 0
	if len(findingRows) > 0 && cost.Estimate(sb.String()) <= specLineTokenBudget {
		var editedRange *lineRange
		if raw, rerr := os.ReadFile(filePath); rerr == nil {
			editedRange = editedRangeFromFile(string(raw), newContent)
		}
		block, fmeta := renderFindingsBlock(findingRows, editedRange, "post")
		if block != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(block)
			findingsShown, findingsOmitted = fmeta.Shown, fmeta.Omitted
			emitBeat("edit-hook", "finding_touched", firingMeta(map[string]string{"phase": "post", "shown": strconv.Itoa(fmeta.Shown)}, sessionID, filePath))
		}
	}

	// OP-4 (spec-line injection, docs/vision/04 §4 item 3): when the edited
	// file cites bracketed spec IDs, tie those knowledge concepts to code by
	// appending where each one is actually defined — beneath the obligations
	// payload when there is one, as a payload of their own when there isn't.
	// The ≤320-token budget covers the WHOLE payload and spec lines drop
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
					specs, ok := fetchSpecRefs(url, svcName, id)
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

	// AM-6 (docs/design/anchored-memory.md §4): anchored memories ABOUT the
	// edited file join the payload as a separate labeled block, through this
	// same path — measured, budgeted and governed exactly as obligations
	// are. Lowest priority in the ≤320-token budget: obligations are
	// compile-time budgeted, spec lines drop first, memories take only what
	// remains (drop lowest-ranked first, count the displaced). The daemon
	// (memory_search, file-scoped) evaluates drift and labels every line;
	// the hook only renders. Gated on the project's own memories file
	// existing so the overwhelmingly common no-memories case costs no RPC,
	// and silent on any error (hook doctrine) — silent on STDOUT, that is:
	// a failed fetch emits the same edit-hook/error beat fetchObligations
	// does, tagged target=memories, so the instrument can tell a memories
	// outage from "no memories" rather than reading both as silence.
	memShown, memDisplaced := 0, 0
	if _, serr := os.Stat(memory.Path(svcRoot)); serr == nil && cost.Estimate(sb.String()) <= specLineTokenBudget {
		if evs, more, ok := fetchMemories(url, svcName, repoRelFile, 2*time.Second); ok {
			memShown, memDisplaced = injectMemories(&sb, repoRelFile, evs, specLineTokenBudget)
			memDisplaced += more
		} else {
			emitBeat("edit-hook", "error", firingMeta(map[string]string{"phase": "post", "target": "memories"}, sessionID, filePath))
		}
	}

	if sb.Len() == 0 {
		emitBeat("edit-hook", "answered", firingMeta(map[string]string{"emitted": "false", "phase": "post"}, sessionID, filePath))
		return "", postEditMeta{}, false // clean file (or unindexed project), no spec IDs → silent
	}
	return sb.String(), postEditMeta{
		rules:           len(renderRules),
		repoRelFile:     repoRelFile,
		displaced:       obl.Displaced,
		memShown:        memShown,
		memDisplaced:    memDisplaced,
		findingsShown:   findingsShown,
		findingsOmitted: findingsOmitted,
	}, true
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
	emitPayloadCost("module-rules", moduleSB.String(), moduleRuleCount, phase, sessionID, nil)
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
//
// layer (FL-20, ADR-040 decision 5) is forwarded as the tool's "layer"
// argument only when non-empty — every pre-existing caller passes "" and
// sees the exact same wire request it always has (the server defaults an
// absent layer to "rules"). The edit-hook post/pre paths pass
// semantic.ObligationLayerAll so a finding-kind row (Kind=="finding") can
// reach the findings block (edithook_findings.go) in the SAME round trip
// obligations rules already make — §3.4's "one call, no compile, no
// model" cost claim.
//
// DG-7: every internal failure used to collapse to a bare ok=false,
// discarding the actual cause — the caller's beat then recorded a
// degradation with a session id and file path but no reason,
// structurally indistinguishable from "no guard names this file." The
// fourth return value names which branch failed (nil on success); the
// hook's fail-open contract to the AGENT is unchanged (stdout still stays
// silent on any error) — this only changes what a human can later learn
// from the beat log.
func fetchObligations(baseURL, service, file string, maxRules int, timeout time.Duration, layer semantic.ObligationLayerFilter) (semantic.Obligations, bool, bool, error) {
	args := map[string]any{
		"service": service,
		"file":    file,
		// BP-20 (ADR-032 §4 D3): the tool inlines each rule's cited lines by
		// default; the edit hook opts OUT. The decision-moment payload is
		// budgeted at ~320 tokens and its rendered bytes are pinned by
		// golden tests — a snippet here would pay a discovery cost the hook
		// never asked for, on every edit.
		"include_snippets": false,
	}
	if maxRules > 0 {
		args["max_rules"] = maxRules
	}
	if layer != "" {
		args["layer"] = string(layer)
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
		return semantic.Obligations{}, false, false, fmt.Errorf("marshal request: %w", err)
	}

	hc := &http.Client{Timeout: timeout}
	req, err := originHeader(baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return semantic.Obligations{}, false, false, fmt.Errorf("build request: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return semantic.Obligations{}, false, false, fmt.Errorf("daemon request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return semantic.Obligations{}, false, false, fmt.Errorf("daemon returned HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return semantic.Obligations{}, false, false, fmt.Errorf("read response body: %w", err)
	}

	inner, err := unwrapRPCResult(raw)
	if err != nil {
		return semantic.Obligations{}, false, false, fmt.Errorf("unwrap RPC result: %w", err)
	}

	// Decode the obligations payload.
	var payload struct {
		Service             string                `json:"service"`
		File                string                `json:"file"`
		Rules               []semantic.Obligation `json:"rules"`
		CompiledAtWatermark string                `json:"compiled_at_watermark"`
		IndexStale          bool                  `json:"index_stale"`
		IndexUnchecked      string                `json:"index_unchecked"`
		Displaced           int                   `json:"displaced"`
		DisplacedRepo       int                   `json:"displaced_repo"`
		DisplacedModule     int                   `json:"displaced_module"`
		ModuleNotes         []semantic.ModuleNote `json:"module_notes"`
		CoverageSkipped     string                `json:"coverage_skipped"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return semantic.Obligations{}, false, false, fmt.Errorf("decode obligations payload: %w", err)
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
		IndexUnchecked:  payload.IndexUnchecked,
		CoverageSkipped: payload.CoverageSkipped,
	}
	return obl, payload.IndexStale, true, nil
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

// maxMemoriesPerEdit caps how many memories one edit asks the daemon for.
// The ≤320-token payload fits a handful at most; the daemon's own
// `displaced` count for anything beyond this cap is folded into the beat
// meta so the truncation is never silent.
const maxMemoriesPerEdit = 20

// injectMemories appends the anchored-memory block (AM-6, docs/design/
// anchored-memory.md §4) to sb: a header naming the file, then one bullet
// per memory, each carrying its [tier · state] label, in the deterministic
// surfacing order (tier, then anchor specificity, then id — the same
// SortEvaluations memory_search uses). Retired and orphaned memories never
// render (memory.Surfacing). budget is the whole-payload token budget: the
// block keeps the highest-ranked prefix that fits, drops the rest and names
// the count, so the block can never quietly inflate an edit's context. The
// input order is irrelevant — the function sorts a copy. Returns how many
// memories rendered and how many the budget displaced; when nothing
// matches, sb is untouched.
func injectMemories(sb *strings.Builder, file string, evs []memory.Evaluation, budget int) (shown, displaced int) {
	live := memory.Surfacing(append([]memory.Evaluation(nil), evs...))
	memory.SortEvaluations(live)
	if len(live) == 0 {
		return 0, 0
	}
	nl := ""
	if sb.Len() > 0 {
		nl = "\n"
	}
	base := sb.String()
	header := fmt.Sprintf("%s🧠 fathom memory — %s:", nl, file)
	var block strings.Builder
	for i, e := range live {
		line := "\n• " + e.ShortLine()
		if cost.Estimate(base+header+block.String()+line) > budget {
			displaced = len(live) - i
			break
		}
		block.WriteString(line)
		shown++
	}
	if shown == 0 {
		// Nothing fit. Name the displacement in one short line when even
		// that fits; otherwise the beat meta is the only record (silence
		// is never a fabricated "no memories").
		note := fmt.Sprintf("%s🧠 %d memory(ies) about %s not shown (budget)", nl, displaced, file)
		if cost.Estimate(base+note) <= budget {
			sb.WriteString(note)
		}
		return shown, displaced
	}
	sb.WriteString(header)
	sb.WriteString(block.String())
	if displaced > 0 {
		fmt.Fprintf(sb, "\n⚠️ %d more memory(ies) not shown (budget)", displaced)
	}
	return shown, displaced
}

// fetchMemories calls the memory_search MCP tool file-scoped (AM-6): the
// daemon lists the service's memories, keeps those whose anchors touch
// file, evaluates drift against the index and labels every one. Returns
// the evaluations (already surfacing-only and sorted, re-sorted by the
// caller regardless), the daemon's own displaced count beyond
// maxMemoriesPerEdit, and ok=false on any error — mirroring
// fetchObligations's silent-on-error contract.
func fetchMemories(baseURL, service, file string, timeout time.Duration) ([]memory.Evaluation, int, bool) {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "memory_search",
			"arguments": map[string]any{
				"service": service,
				"file":    file,
				"limit":   maxMemoriesPerEdit,
			},
		},
	})
	if err != nil {
		return nil, 0, false
	}

	hc := &http.Client{Timeout: timeout}
	req, err := originHeader(baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, 0, false
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, 0, false
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, 0, false
	}

	inner, err := unwrapRPCResult(raw)
	if err != nil {
		return nil, 0, false
	}

	var payload struct {
		Memories  []memory.Evaluation `json:"memories"`
		Displaced int                 `json:"displaced"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return nil, 0, false
	}
	// A memory without a tier and state is a claim the daemon never made:
	// drop it here too rather than render an unlabelled line (§9).
	out := make([]memory.Evaluation, 0, len(payload.Memories))
	for _, e := range payload.Memories {
		if e.Tier == "" || e.State == "" {
			continue
		}
		out = append(out, e)
	}
	return out, payload.Displaced, true
}

// indexUncheckedLine (RT-26, I38) is the freshness line for an edited file
// the daemon did not read for its index check (over the hot-path read
// cap): named as unchecked, never left silent — silence reads as current.
func indexUncheckedLine(reason string) string {
	return "index freshness not checked for this file (" + reason + ")"
}
