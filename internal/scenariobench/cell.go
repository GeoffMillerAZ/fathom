// The scenario cell store (SB-17), mirroring ADR-032's contract
// (internal/contextbench/cell.go): a cell is remembered under a
// content-addressed key over the invariants that bind it, so a later run
// can borrow it instead of re-running, and a borrow either matches exactly
// or does not exist — never a silent, partial match.
package scenariobench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// CellInvariants are the invariants that bind one scored scenario cell.
// Every field is part of the key; a cell whose inputs differ in any of
// them is a different cell, not the same cell measured again.
type CellInvariants struct {
	// CorpusSHA256 is the hash of the corpus file's bytes (CorpusSHA256).
	CorpusSHA256 string
	TaskID       string
	Arm          string
	// ModelID is the model identity the run was pinned to.
	ModelID string
	// HarnessVersion is the harness's own version string (Harness.Version()).
	HarnessVersion string
	// Scoring names how this cell's readings were computed (an oracle/
	// readings version identifier) — the same transcripts scored two ways
	// are two cells.
	Scoring string
	// PromptSubject is the subject description named in the prompt.
	PromptSubject string
	// ArmSurface identifies the subject-under-test for arms that have one:
	// the fathom binary commit for a wired arm, a competitor's pinned
	// version for a competitor arm, empty for a pure control — so a fathom
	// change changes only the wired arm's key.
	ArmSurface string
}

// canonical renders the invariants as labeled, newline-separated lines —
// labeled so two different inputs can never collide into the same hash by
// concatenation alone.
func (in CellInvariants) canonical() string {
	var b strings.Builder
	for _, kv := range [][2]string{
		{"corpus_sha256", in.CorpusSHA256},
		{"task_id", in.TaskID},
		{"arm", in.Arm},
		{"model_id", in.ModelID},
		{"harness_version", in.HarnessVersion},
		{"scoring", in.Scoring},
		{"prompt_subject", in.PromptSubject},
		{"arm_surface", in.ArmSurface},
	} {
		b.WriteString(kv[0])
		b.WriteByte('=')
		b.WriteString(kv[1])
		b.WriteByte('\n')
	}
	return b.String()
}

// ScenarioCellKey is the content address of one scored scenario cell:
// sha256 over the labeled invariants, hex-encoded.
func ScenarioCellKey(inv CellInvariants) string {
	sum := sha256.Sum256([]byte(inv.canonical()))
	return hex.EncodeToString(sum[:])
}

// Validate reports the first invariant a key cannot be built without
// (ArmSurface is allowed to be empty — that is what a pure control means).
func (in CellInvariants) Validate() error {
	for _, kv := range [][2]string{
		{"corpus_sha256", in.CorpusSHA256},
		{"task_id", in.TaskID},
		{"arm", in.Arm},
		{"model_id", in.ModelID},
		{"harness_version", in.HarnessVersion},
		{"scoring", in.Scoring},
		{"prompt_subject", in.PromptSubject},
	} {
		if kv[1] == "" {
			return fmt.Errorf("scenario cell key: %s is empty — a key over an unknown invariant would let an unrelated cell be borrowed", kv[0])
		}
	}
	return nil
}

func cellTrialFile(trial int) string {
	if trial < 1 {
		trial = 1
	}
	return fmt.Sprintf("t%d.json", trial)
}

// ScenarioCellPath returns the content-addressed path of one trial of one
// cell: <cellsDir>/<key>/t<n>.json.
func ScenarioCellPath(cellsDir, key string, trial int) string {
	return filepath.Join(cellsDir, key, cellTrialFile(trial))
}

// StoreScenarioCell writes a scored cell into the content-addressed store
// (workspace/artifacts/scenariobench/cells/<key>/t<n>.json). A cell already
// present is left alone — the first writer owns the provenance — and a
// borrowed cell is never written back (it is already in the store under
// this exact key).
func StoreScenarioCell(cellsDir string, r CellResult) error {
	if r.CellKey == "" {
		return fmt.Errorf("storing scenario cell %s/%s: no cell_key", r.TaskID, r.Arm)
	}
	if r.Borrowed {
		return nil
	}
	dir := filepath.Join(cellsDir, r.CellKey)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating cell dir %s: %w", dir, err)
	}
	path := filepath.Join(dir, cellTrialFile(r.Trial))
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cell %s: %w", r.CellKey, err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing cell %s: %w", path, err)
	}
	return nil
}

// OverwriteScenarioCell rewrites an EXISTING cell record in place — the
// one place the content-addressed store is not write-once
// (StoreScenarioCell's own "first writer owns it" contract stands for
// every other caller). O53 defect 7's `scenariobench void` is the only
// caller: marking a filed cell's own InstrumentDefect after the fact.
// Unlike StoreScenarioCell, it refuses when the file does NOT already
// exist — nothing to overwrite — rather than silently creating one.
func OverwriteScenarioCell(cellsDir string, r CellResult) error {
	if r.CellKey == "" {
		return fmt.Errorf("overwriting scenario cell %s/%s: no cell_key", r.TaskID, r.Arm)
	}
	path := ScenarioCellPath(cellsDir, r.CellKey, r.Trial)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("overwriting scenario cell %s: no such cell on disk", path)
		}
		return fmt.Errorf("overwriting scenario cell %s: %w", path, err)
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cell %s: %w", r.CellKey, err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing cell %s: %w", path, err)
	}
	return nil
}

// FindScenarioCellsByRun walks cellsDir for every stored cell record
// whose TaskID, Arm and RunID match — O53 defect 7's own lookup for
// `scenariobench void`, which has no corpus (and so no cell_key) to hand
// BorrowScenarioCell directly; it only has the ledger row's own
// (run_id, task_id, arm). Returned in trial order. An empty cellsDir (or
// one that does not exist yet) is zero results, not an error.
func FindScenarioCellsByRun(cellsDir, taskID, arm, runID string) ([]CellResult, error) {
	var out []CellResult
	err := filepath.WalkDir(cellsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return fmt.Errorf("reading %s: %w", path, rerr)
		}
		var r CellResult
		if uerr := json.Unmarshal(raw, &r); uerr != nil {
			return fmt.Errorf("parsing %s: %w", path, uerr)
		}
		if r.TaskID == taskID && r.Arm == arm && r.RunID == runID {
			out = append(out, r)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("finding cells for run %s task %s arm %s: %w", runID, taskID, arm, err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Trial < out[j].Trial })
	return out, nil
}

// BorrowScenarioCell returns the stored cell for (key, trial) marked
// Borrowed, with the run that paid for it named in SourceRunID. The second
// return is false when no such cell exists — the caller then runs the cell
// for real. A read error is an error, never a miss (ADR-027): "I could not
// read it" and "it is not there" are different facts.
func BorrowScenarioCell(cellsDir, key string, trial int) (CellResult, bool, error) {
	path := ScenarioCellPath(cellsDir, key, trial)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return CellResult{}, false, nil
		}
		return CellResult{}, false, fmt.Errorf("reading cell %s: %w", path, err)
	}
	var r CellResult
	if err := json.Unmarshal(raw, &r); err != nil {
		return CellResult{}, false, fmt.Errorf("parsing cell %s: %w", path, err)
	}
	if r.CellKey != key {
		return CellResult{}, false, fmt.Errorf("cell %s records key %s — the store is inconsistent with its own addressing", path, r.CellKey)
	}
	r.Borrowed = true
	if r.SourceRunID == "" {
		// Unlike contextbench's ArmResult, CellResult already carries a
		// mandatory RunID naming the run that produced it — so a
		// first-generation borrow can report the real source instead of
		// ADR-032's "unrecorded" (that fallback stays for the genuinely
		// unknown case: a cell whose RunID was also never recorded, e.g.
		// hand-migrated from elsewhere).
		r.SourceRunID = orUnrecordedScenario(r.RunID)
	}
	// O55 item 5: cellsDir verbatim, exactly as the caller passed it — the
	// store this borrow actually resolved against, so a later consumer
	// (the ledger's own Borrowed list, cmdLedger's -strict-borrow error)
	// can name it. Ephemeral like Borrowed/SourceRunID above: never
	// written back (StoreScenarioCell already refuses to write a borrowed
	// cell at all).
	r.StoreRoot = cellsDir
	return r, true, nil
}

// AcceptBorrowedScenario is the evaluator's admission rule for a borrowed
// cell: key equality, and nothing else. want is the key THIS run's
// invariants produce; a borrowed reading whose recorded key differs was
// bought under different invariants and may not stand in for it.
func AcceptBorrowedScenario(want string, r CellResult) error {
	if !r.Borrowed {
		return nil
	}
	if r.CellKey == "" {
		return fmt.Errorf("borrowed cell %s/%s/t%d (from run %s) carries no cell_key — provenance unverifiable", r.TaskID, r.Arm, r.Trial, orUnrecordedScenario(r.SourceRunID))
	}
	if r.CellKey != want {
		return fmt.Errorf("borrowed cell %s/%s/t%d (from run %s) has key %s, but this run's invariants produce %s — refused", r.TaskID, r.Arm, r.Trial, orUnrecordedScenario(r.SourceRunID), shortScenario(r.CellKey), shortScenario(want))
	}
	return nil
}

func orUnrecordedScenario(s string) string {
	if s == "" {
		return "unrecorded"
	}
	return s
}

// shortScenario truncates a key to 12 chars for an error message, without
// panicking on a shorter (e.g. test-fixture) key.
func shortScenario(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12]
}

// ResolveArmSurface returns the ArmSurface a cell key should use for arm:
// the literal fathomCommit when arm.Surface == "fathom" (the corpus's
// sentinel for "the fathom this arm was talking to"), the pinned
// competitor version verbatim for any other non-empty Surface, and "" for
// a pure control (Surface == "").
func ResolveArmSurface(arm Arm, fathomCommit string) string {
	switch arm.Surface {
	case "":
		return ""
	case "fathom":
		return fathomCommit
	default:
		return arm.Surface
	}
}

// BorrowedScenarioCell is one borrowed reading's provenance, as a ledger
// row records it.
type BorrowedScenarioCell struct {
	TaskID      string `json:"task_id"`
	Arm         string `json:"arm"`
	Trial       int    `json:"trial"`
	CellKey     string `json:"cell_key"`
	SourceRunID string `json:"source_run_id"`
	// StoreRoot is the cell store (cellsDir) this reading was actually
	// borrowed from (O55 item 5, BorrowScenarioCell's own CellResult.
	// StoreRoot) — named on the ledger row so a reader can tell WHICH
	// store a borrowed cell's provenance points at, not just which run.
	StoreRoot string `json:"store_root,omitempty"`
}

// AggregateReadings computes the basic, always-available Readings fields
// (total tokens, turns, cost, payload bytes) from a cell's sessions. The
// wave-3 readings (plan coverage, false-done, resumption, overlap, drift,
// rule-served) are computed elsewhere and merged in by the caller — this
// is deliberately just the sum every cell can report regardless of what
// else has landed.
func AggregateReadings(sessions []SessionResult) Readings {
	var r Readings
	for _, s := range sessions {
		r.TotalTokens += s.TotalTokens
		r.TotalTurns += s.Turns
		r.TotalCostUSD += s.CostUSD
		r.PayloadBytes += s.PayloadBytes
		for name, n := range s.ToolCalls {
			if r.ToolCalls == nil {
				r.ToolCalls = map[string]int{}
			}
			r.ToolCalls[name] += n
		}
		for name, n := range s.HooksFired {
			if r.HooksFired == nil {
				r.HooksFired = map[string]int{}
			}
			r.HooksFired[name] += n
		}
	}
	return r
}

// AggregateRowReadings computes a ScenarioLedgerRow's own Readings from
// every cell filed into it (one row's own group of same-task/arm trials),
// plus the per-trial detail (TrialReading) that the row's own summed/
// averaged fields do not carry — the fix for a ledger-row defect
// (docs/bench/scenarios.md, 2026-09-13) where only 6 of 17 per-cell
// reading fields survived into the row: every count is summed
// (hooks_fired per event, tool_calls, facts_reused), every boolean-per-
// trial reading is reduced to a count of trials true (violation,
// rule_served_before_edit), every ratio-per-trial reading is averaged
// with its own n (overlap ratio, resumption tokens-to-first-edit), and
// the full structured readings (gold_source_tool, plan_coverage,
// false_done, drift, resumption, overlap) are preserved verbatim per
// trial — nothing is summarized away. Trial order in the input does not
// matter; the returned per-trial slice is sorted by CellResult.Trial.
func AggregateRowReadings(cells []CellResult) (Readings, []TrialReading) {
	var r Readings
	var violationMeasured, violationTrue int
	var ruleServedMeasured, ruleServedTrue int
	var overlapSum float64
	var overlapN int
	var resumptionSum int64
	var resumptionN int
	var hitAt1Sum, hitAt5Sum, recallSum float64
	var localizationN int
	trials := make([]TrialReading, 0, len(cells))

	for _, c := range cells {
		cr := c.Readings
		r.TotalTokens += cr.TotalTokens
		r.TotalTurns += cr.TotalTurns
		r.TotalCostUSD += cr.TotalCostUSD
		r.PayloadBytes += cr.PayloadBytes
		r.CompileCostUSD += cr.CompileCostUSD
		for name, n := range cr.ToolCalls {
			if r.ToolCalls == nil {
				r.ToolCalls = map[string]int{}
			}
			r.ToolCalls[name] += n
		}
		for name, n := range cr.HooksFired {
			if r.HooksFired == nil {
				r.HooksFired = map[string]int{}
			}
			r.HooksFired[name] += n
		}
		if cr.FactsReused != nil {
			if r.FactsReused == nil {
				zero := 0
				r.FactsReused = &zero
			}
			*r.FactsReused += *cr.FactsReused
		}
		if cr.Violation != nil {
			violationMeasured++
			if *cr.Violation {
				violationTrue++
			}
		}
		if cr.RuleServedBeforeEdit != nil {
			ruleServedMeasured++
			if *cr.RuleServedBeforeEdit {
				ruleServedTrue++
			}
		}
		if cr.Overlap != nil && cr.Overlap.NotMeasurable == "" {
			overlapSum += cr.Overlap.Ratio
			overlapN++
		}
		if cr.Resumption != nil && cr.Resumption.NotMeasurable == "" {
			resumptionSum += cr.Resumption.TokensToFirstEditAll
			resumptionN++
		}
		if cr.Localization != nil && cr.Localization.NotMeasurable == "" {
			if cr.Localization.HitAt1 {
				hitAt1Sum++
			}
			if cr.Localization.HitAt5 {
				hitAt5Sum++
			}
			recallSum += cr.Localization.Recall
			localizationN++
		}
		trials = append(trials, TrialReading{
			Trial:          c.Trial,
			GoldSourceTool: cr.GoldSourceTool,
			PlanCoverage:   cr.PlanCoverage,
			FalseDone:      cr.FalseDone,
			Drift:          cr.Drift,
			Resumption:     cr.Resumption,
			Overlap:        cr.Overlap,
			Localization:   cr.Localization,
		})
	}

	if violationMeasured > 0 {
		v := violationTrue
		r.ViolationTrials = &v
	}
	if ruleServedMeasured > 0 {
		v := ruleServedTrue
		r.RuleServedBeforeEditTrials = &v
	}
	if overlapN > 0 {
		r.OverlapRatioMean = overlapSum / float64(overlapN)
		r.OverlapRatioN = overlapN
	}
	if resumptionN > 0 {
		r.ResumptionTokensToFirstEditMean = float64(resumptionSum) / float64(resumptionN)
		r.ResumptionTokensToFirstEditN = resumptionN
	}
	if localizationN > 0 {
		r.LocalizationHitAt1Rate = hitAt1Sum / float64(localizationN)
		r.LocalizationHitAt5Rate = hitAt5Sum / float64(localizationN)
		r.LocalizationFileRecallMean = recallSum / float64(localizationN)
		r.LocalizationN = localizationN
	}
	sort.Slice(trials, func(i, j int) bool { return trials[i].Trial < trials[j].Trial })
	return r, trials
}

// StoredScenarioTrials returns every trial number actually stored for key
// under cellsDir (parsed from its "t<N>.json" filenames), sorted
// ascending — used by `scenariobench ledger` so a cell's row counts every
// trial on disk, never silently capping at a fixed -trials count (the
// defect that dropped a stored t2 because -trials defaulted to 1). A key
// with no directory at all reports zero trials, not an error.
func StoredScenarioTrials(cellsDir, key string) ([]int, error) {
	entries, err := os.ReadDir(filepath.Join(cellsDir, key))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading cell dir %s/%s: %w", cellsDir, key, err)
	}
	var trials []int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "t") || !strings.HasSuffix(name, ".json") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "t"), ".json"))
		if err != nil {
			continue
		}
		trials = append(trials, n)
	}
	sort.Ints(trials)
	return trials, nil
}

// readingsCompileCost derives the CompileCostReading a cell's Readings
// should carry from wt's own CompileCost (PrepareSubject/compileLayers,
// subject.go) — nil when the cell never compiled a layer (an unwired or
// non-CompileLayers arm leaves wt.CompileCost nil).
func readingsCompileCost(wt Worktree) *CompileCostReading {
	if wt.CompileCost == nil {
		return nil
	}
	return &CompileCostReading{
		TokensIn:  wt.CompileCost.TokensIn,
		TokensOut: wt.CompileCost.TokensOut,
		CostUSD:   wt.CompileCost.CostUSD,
		Cached:    wt.CompileCost.Cached,
	}
}

// ApplyCompileCost sets r.CompileCost/CompileCostUSD from wt
// (readingsCompileCost) and rolls the dollar figure into r.TotalCostUSD
// too: a wired cell's compile spend is real spend the row's total must
// include (RC-10, vision 28 §2 F6), not a number that only shows up in a
// side column. A no-op when wt never compiled a layer.
func ApplyCompileCost(r *Readings, wt Worktree) {
	cc := readingsCompileCost(wt)
	r.CompileCost = cc
	if cc != nil {
		r.CompileCostUSD = cc.CostUSD
		r.TotalCostUSD += cc.CostUSD
	}
}

// SurfaceToolPrefix returns the MCP tool-name prefix an arm's Surface maps
// to: the part before any "@" version suffix ("graphify@v0.9.61" ->
// "mcp__graphify__"; "fathom" -> "mcp__fathom__"), or "" for an empty
// Surface — a pure control names no surface to have called, by
// definition, not merely "not measured".
func SurfaceToolPrefix(surface string) string {
	if surface == "" {
		return ""
	}
	if i := strings.IndexByte(surface, '@'); i >= 0 {
		surface = surface[:i]
	}
	return "mcp__" + surface + "__"
}

// SurfaceCallCount sums toolCalls for every tool namespaced to surface's
// own MCP server (SurfaceToolPrefix). 0 for an empty surface.
func SurfaceCallCount(toolCalls map[string]int, surface string) int {
	prefix := SurfaceToolPrefix(surface)
	if prefix == "" {
		return 0
	}
	n := 0
	for name, c := range toolCalls {
		if strings.HasPrefix(name, prefix) {
			n += c
		}
	}
	return n
}

// pathIsInstrumentOwned reports whether repoRelPath (as `git diff
// --name-only`/a `diff --git a/X b/X` header spells it) is one
// PrepareSubject's own instrument wrote (O55 item 4): owned outright
// (InstrumentOwnedPaths, subject.go — internal/wiring's ledger plus
// .fathomignore), or anywhere under workspace/ — this repo's own
// convention (root CLAUDE.md's workspace/ rule) for instrument/runtime
// state, which never carries a ledger entry of its own and is never a
// subject's tracked source.
func pathIsInstrumentOwned(owned map[string]bool, repoRelPath string) bool {
	repoRelPath = filepath.ToSlash(repoRelPath)
	if owned[repoRelPath] {
		return true
	}
	if repoRelPath == ".fathom" || strings.HasPrefix(repoRelPath, ".fathom/") {
		// The whole .fathom/ namespace is fathom's own reserved runtime
		// state — not just owned.json: internal/wiring's own ApplyWiring
		// also writes .fathom/receipts.jsonl (undo receipts) unprompted,
		// found leaking into agent.diff while testing the region-aware
		// fix below (O55 item 4, fix-first review) — never a subject's own
		// tracked source by convention, so excluded wholesale like
		// workspace/ rather than enumerated file by file.
		return true
	}
	return repoRelPath == "workspace" || strings.HasPrefix(repoRelPath, "workspace/")
}

// diffGitHeaderPath extracts the "b/" (destination) path from one `diff
// --git a/X b/X` header line.
func diffGitHeaderPath(headerLine string) string {
	const marker = " b/"
	i := strings.LastIndex(headerLine, marker)
	if i < 0 {
		return ""
	}
	return headerLine[i+len(marker):]
}

// FilterInstrumentOwnedDiff drops every whole-file section of a unified
// diff (as `git diff` emits it, one "diff --git a/X b/X" header per file)
// whose path is instrument-owned (O55 item 4: everything internal/wiring's
// ledger records — KindKey/KindFile, never a KindRegion stanza, see
// stripFathomRegionsFromDiff for that — .fathomignore, anything under
// workspace/ — must never be credited to the agent as agent.diff or
// counted toward any byte metric derived from it). Returns the filtered
// diff and how many file sections were dropped, so the caller can record
// instrument_files_excluded. A nil or empty owned set, or an empty diff,
// returns diff unchanged with 0 excluded.
func FilterInstrumentOwnedDiff(diff []byte, owned map[string]bool) ([]byte, int) {
	if len(diff) == 0 || len(owned) == 0 {
		return diff, 0
	}
	lines := strings.Split(string(diff), "\n")
	out := make([]string, 0, len(lines))
	excluded := 0
	keep := true
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			path := diffGitHeaderPath(line)
			keep = !(path != "" && pathIsInstrumentOwned(owned, path))
			if !keep {
				excluded++
			}
		}
		if keep {
			out = append(out, line)
		}
	}
	return []byte(strings.Join(out, "\n")), excluded
}

// regionRange is one file's fathom-region line span (regionLineRange,
// subject.go), separately on each side of a diff: the base commit's own
// version (old) and the worktree's current version (new). 0 in either
// field means that side carries no region at all.
type regionRange struct {
	oldStart, oldEnd int
	newStart, newEnd int
}

// hunkHeaderRe parses one unified-diff hunk header: `@@ -oldStart[,oldLen]
// +newStart[,newLen] @@ optional trailer`. A missing ",len" means length 1
// (diff's own convention); the capture groups here only need the starts —
// stripRegionFromDiffSection recomputes the lengths itself from what it
// keeps.
var hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@(.*)$`)

// stripFathomRegionsFromDiff rewrites diff so that, for every file named in
// ranges, the lines internal/wiring's own marker pair delimits (region.go's
// HTMLMarkers) are removed from its hunks — never the whole file, so a
// genuine agent edit anywhere else in CLAUDE.md/AGENTS.md survives byte for
// byte (O55 item 4, fix-first review: a KindRegion stanza carries no ledger
// entry at all — apply.go only records KindKey/KindFile — so
// FilterInstrumentOwnedDiff's ledger-driven, whole-file exclusion can never
// catch it; the old approach silently never excluded a real wired cell's
// stanza). A hunk whose only content was the region is dropped entirely;
// one that mixes region and non-region lines keeps the latter with a
// recomputed header. Returns the rewritten diff and how many files had at
// least one hunk actually changed by this pass — added to
// instrument_files_excluded alongside FilterInstrumentOwnedDiff's own
// count, since the two operate on disjoint files.
func stripFathomRegionsFromDiff(diff []byte, ranges map[string]regionRange) ([]byte, int) {
	if len(diff) == 0 || len(ranges) == 0 {
		return diff, 0
	}
	lines := strings.Split(string(diff), "\n")
	var out []string
	excludedFiles := 0
	i := 0
	for i < len(lines) {
		line := lines[i]
		if !strings.HasPrefix(line, "diff --git ") {
			out = append(out, line)
			i++
			continue
		}
		start := i
		i++
		for i < len(lines) && !strings.HasPrefix(lines[i], "diff --git ") {
			i++
		}
		section := lines[start:i]
		rng, ok := ranges[diffGitHeaderPath(line)]
		if !ok || (rng.oldStart == 0 && rng.newStart == 0) {
			out = append(out, section...)
			continue
		}
		stripped, changed := stripRegionFromDiffSection(section, rng)
		if changed {
			excludedFiles++
		}
		if changed && !hasHunk(stripped) {
			// Nothing real survived this file's diff at all (its ENTIRE
			// change was the region) — drop the whole section, header
			// included, rather than emit a hunk-less "diff --git" stub.
			continue
		}
		out = append(out, stripped...)
	}
	return []byte(strings.Join(out, "\n")), excludedFiles
}

// hasHunk reports whether section (a file's own diff lines) still carries
// at least one "@@ " hunk header.
func hasHunk(section []string) bool {
	for _, l := range section {
		if strings.HasPrefix(l, "@@ ") {
			return true
		}
	}
	return false
}

// stripRegionFromDiffSection removes rng's region lines from one file's
// own diff section (header lines through its hunks): a '-' line is dropped
// when its OLD-file line number falls in [rng.oldStart, rng.oldEnd], a '+'
// line when its NEW-file line number falls in [rng.newStart, rng.newEnd];
// context lines are always kept (a region is either pure addition or pure
// replacement in every case this filter needs to handle — a context line
// inside one would mean an unchanged line sat inside a region fathom
// rewrote wholesale, which region.go's own render never produces). Each
// hunk's header is rewritten with the surviving counts; a hunk left with
// no '+'/'-' line at all (it WAS the region) is dropped outright rather
// than emitted as a no-op hunk. changed reports whether anything in this
// section was actually touched.
func stripRegionFromDiffSection(section []string, rng regionRange) (out []string, changed bool) {
	i := 0
	for i < len(section) && !strings.HasPrefix(section[i], "@@ ") {
		out = append(out, section[i])
		i++
	}
	for i < len(section) {
		header := section[i]
		m := hunkHeaderRe.FindStringSubmatch(header)
		if m == nil {
			out = append(out, header)
			i++
			continue
		}
		oldStart, _ := strconv.Atoi(m[1])
		newStart, _ := strconv.Atoi(m[2])
		trailer := m[3]
		i++
		var body []string
		for i < len(section) && !strings.HasPrefix(section[i], "@@ ") {
			body = append(body, section[i])
			i++
		}

		oldLine, newLine := oldStart, newStart
		var kept []string
		keptOld, keptNew := 0, 0
		hunkChanged := false
		for _, l := range body {
			if l == "" {
				kept = append(kept, l)
				continue
			}
			switch l[0] {
			case ' ':
				kept = append(kept, l)
				keptOld++
				keptNew++
				oldLine++
				newLine++
			case '-':
				if rng.oldStart > 0 && oldLine >= rng.oldStart && oldLine <= rng.oldEnd {
					hunkChanged = true
				} else {
					kept = append(kept, l)
					keptOld++
				}
				oldLine++
			case '+':
				if rng.newStart > 0 && newLine >= rng.newStart && newLine <= rng.newEnd {
					hunkChanged = true
				} else {
					kept = append(kept, l)
					keptNew++
				}
				newLine++
			default:
				// "\ No newline at end of file" and anything else unified
				// diff can emit mid-hunk: keep verbatim, no line advance.
				kept = append(kept, l)
			}
		}
		if !hunkChanged {
			out = append(out, header)
			out = append(out, body...)
			continue
		}
		changed = true
		hasRealChange := false
		for _, l := range kept {
			if l != "" && (l[0] == '+' || l[0] == '-') {
				hasRealChange = true
				break
			}
		}
		if !hasRealChange {
			continue // the whole hunk WAS the region — drop it outright
		}
		out = append(out, fmt.Sprintf("@@ -%d,%d +%d,%d @@%s", oldStart, keptOld, newStart, keptNew, trailer))
		out = append(out, kept...)
	}
	return out, changed
}

// BorrowedScenarioCellsOf lists every borrowed reading in results, sorted
// by (task, arm, trial).
func BorrowedScenarioCellsOf(results []CellResult) []BorrowedScenarioCell {
	var out []BorrowedScenarioCell
	for _, r := range results {
		if !r.Borrowed {
			continue
		}
		out = append(out, BorrowedScenarioCell{TaskID: r.TaskID, Arm: r.Arm, Trial: r.Trial, CellKey: r.CellKey, SourceRunID: orUnrecordedScenario(r.SourceRunID), StoreRoot: r.StoreRoot})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TaskID != out[j].TaskID {
			return out[i].TaskID < out[j].TaskID
		}
		if out[i].Arm != out[j].Arm {
			return out[i].Arm < out[j].Arm
		}
		return out[i].Trial < out[j].Trial
	})
	return out
}
