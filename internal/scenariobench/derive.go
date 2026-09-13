// derive.go builds testdata/scenariobench/public-go.json from pinned
// public snapshots (SB-34, vision 27 §7, ADR-033): no LLM in the loop, no
// authored ground truth — every task's gold comes from a merged developer
// patch at a pinned base commit. The runbook is docs/bench/scenarios.md.
//
// Two upstream sources are read, exactly as internal/contextbench/publicloc
// reads them for the sibling localization corpus (docs/bench/public-loc.md):
// the Multi-SWE-bench Go split (org, repo, base.sha, fix_patch — CC0-1.0)
// and the SWE-Explore-Bench Go JSONL. The two are NOT interchangeable here:
// the published SWE-Explore-Bench snapshot (verified 2026-09-13, 848 rows)
// carries instance_id and ground_truth{read_core_files, modified_core_files,
// ...} but neither a base_commit nor a patch, so it cannot seed a Subject
// or an Oracle under this corpus's contract. It is still pinned and
// recorded in Sources (URL, revision, sha256, license) for provenance; its
// contribution is recorded honestly in Notes rather than silently dropped
// (ADR-027) — see docs/bench/scenarios.md "What SWE-Explore contributes".
package scenariobench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing/fstest"
	"time"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

// canonicalMarshal is the one encoding of a derived Corpus: two-space
// indent, no HTML escaping, trailing newline — mirroring
// publicloc.MarshalCanonical so the committed file and a fresh
// re-derivation compare byte-for-byte (TestPublicScenarioCorpusIsDerived).
func canonicalMarshal(c Corpus) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(c); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RegisteredN is the plan's pre-registered corpus size (docs/plans/
// strengths-bench.md SB-34: "the first 30–40"); 35 is the midpoint, fixed
// here so "the first N" has one meaning, mirroring publicloc.RegisteredRule.
const RegisteredN = 35

// DefaultCutoffDate is preferred: merged_at after this date is sorted
// ahead of everything else (vision 27 §7 "tasks merged after the model's
// cutoff preferred").
const DefaultCutoffDate = "2026-06-01"

// DefaultGitTimeout bounds one subject-commit fetch.
const DefaultGitTimeout = 90 * time.Second

// SnapshotPin is one pinned input file to DeriveScenarioTasks: a path on
// disk (the operator fetches it — docs/bench/scenarios.md step 1) and the
// upstream revision it was fetched at, recorded on the corpus's Sources.
type SnapshotPin struct {
	Path     string
	Revision string
}

// DeriveOptions configures DeriveScenarioTasks. The two snapshot lists are
// typically one entry per upstream file (Multi-SWE-bench ships the Go
// split as three per-repo JSONL files: cli/cli, grpc-go, go-zero).
type DeriveOptions struct {
	MultiSWE   []SnapshotPin
	SWEExplore []SnapshotPin

	// SubjectsDir is where subject repos are fetched (shallow, one commit
	// at a time) so Gold.Functions can be derived via
	// publicloc.EnclosingFuncsForHunks and ClassifyChange can compare
	// before/after exported signatures. Empty disables cloning: every task
	// then carries Requires ["subject-clone"], Gold.Files from the diff's
	// paths, Gold.Functions empty, and Derivation says why.
	SubjectsDir string
	// SkipClone forces the no-clone fallback even when SubjectsDir is set
	// (fast, network-free derivation for tests).
	SkipClone bool
	// GitTimeout bounds one subject-commit fetch; 0 uses DefaultGitTimeout.
	GitTimeout time.Duration

	// N is the corpus size target (the registered window is 30-40); 0
	// uses RegisteredN.
	N int
	// CutoffDate (YYYY-MM-DD) is preferred; 0 uses DefaultCutoffDate. Only
	// takes effect for instances whose merged_at the source actually
	// carries — neither pinned Multi-SWE-bench nor SWE-Explore snapshot
	// verified so far carries one (recorded in Notes), so today this is a
	// documented no-op kept ready for a source revision that adds it.
	CutoffDate string
}

// Description is fixed text: no timestamp or hostname, so two identical
// derivations byte-match (mirrors publicloc.Description's rule).
const Description = "Public Go change-scenario corpus (docs/plans/strengths-bench.md SB-34, vision 27 ADR-033): every task is a merged developer patch at a pinned base commit from the pinned upstream snapshots recorded in sources, selected by ADR-031's rule (gold patch touches 1-5 non-test Go files; sort by instance_id; first N), categorised by ClassifyChange, with a prediction per arm registered before any run. Rebuild with the derivation harness named in docs/bench/scenarios.md; identical snapshots and subject clones reproduce this file byte-for-byte (canonical JSON, two-space indent, no HTML escaping)."

// DeriveScenarioTasks derives a Corpus from the pinned snapshots named in
// opts (SB-34). It never spends an LLM call and never reaches the fleet
// store; the only network use is fetching upstream repository commits
// (git) when opts.SubjectsDir is set.
func DeriveScenarioTasks(opts DeriveOptions) (Corpus, error) {
	if len(opts.MultiSWE) == 0 {
		return Corpus{}, fmt.Errorf("scenariobench: DeriveOptions.MultiSWE is empty — at least one Multi-SWE-bench Go snapshot pin is required")
	}
	n := opts.N
	if n == 0 {
		n = RegisteredN
	}
	cutoff := opts.CutoffDate
	if cutoff == "" {
		cutoff = DefaultCutoffDate
	}
	cutoffTime, err := time.Parse("2006-01-02", cutoff)
	if err != nil {
		return Corpus{}, fmt.Errorf("scenariobench: CutoffDate %q: %w", cutoff, err)
	}

	var instances []publicloc.Instance
	var sources []SourcePin
	extra := map[string]extraFields{}

	for _, pin := range opts.MultiSWE {
		raw, err := os.ReadFile(pin.Path)
		if err != nil {
			return Corpus{}, fmt.Errorf("scenariobench: reading %s: %w", pin.Path, err)
		}
		insts, err := publicloc.ReadMultiSWEBench(bytes.NewReader(raw))
		if err != nil {
			return Corpus{}, fmt.Errorf("scenariobench: parsing %s: %w", pin.Path, err)
		}
		instances = append(instances, insts...)
		info := publicloc.Sources[publicloc.SourceMultiSWEBench]
		sources = append(sources, SourcePin{
			Name: "multi-swe-bench:" + filepath.Base(pin.Path), URL: info.URL,
			Revision: pin.Revision, License: info.License, SHA256: publicloc.SHA256Hex(raw),
		})
		ex, exErr := scanExtraFields(raw)
		if exErr != nil {
			return Corpus{}, fmt.Errorf("scenariobench: scanning %s for test/merge fields: %w", pin.Path, exErr)
		}
		for id, f := range ex {
			extra[id] = f
		}
	}

	notes := map[string]any{}
	sweExploreRows, sweExploreUsable := 0, 0
	for _, pin := range opts.SWEExplore {
		raw, err := os.ReadFile(pin.Path)
		if err != nil {
			return Corpus{}, fmt.Errorf("scenariobench: reading %s: %w", pin.Path, err)
		}
		info := publicloc.Sources[publicloc.SourceSWEExplore]
		// publicloc.Sources records "MIT" for this dataset; the Hugging
		// Face API for SWE-Explore-Bench/SWE-Explore-Bench reports
		// "cc-by-nc-nd-4.0" (verified 2026-09-13 via /api/datasets/...),
		// a no-derivatives license. This corpus's own SourcePin records
		// what was actually verified rather than repeating the stale
		// constant (docs/bench/scenarios.md flags the discrepancy).
		license := info.License
		if pin.Path != "" && strings.Contains(strings.ToLower(pin.Path), "swe-explore") {
			license = "CC-BY-NC-ND-4.0 (verified 2026-09-13; publicloc.Sources records \"MIT\" — see docs/bench/scenarios.md)"
		}
		sp := SourcePin{Name: "swe-explore:" + filepath.Base(pin.Path), URL: info.URL, Revision: pin.Revision, License: license, SHA256: publicloc.SHA256Hex(raw)}
		rows := countJSONLRows(raw)
		sweExploreRows += rows
		insts, readErr := publicloc.ReadSWEExplore(bytes.NewReader(raw))
		if readErr != nil {
			notes["swe_explore_unusable_reason"] = readErr.Error()
		} else {
			sweExploreUsable += len(insts)
			instances = append(instances, insts...)
		}
		sources = append(sources, sp)
	}
	if len(opts.SWEExplore) > 0 {
		notes["swe_explore_rows"] = sweExploreRows
		notes["swe_explore_usable_instances"] = sweExploreUsable
		if sweExploreUsable == 0 {
			notes["swe_explore_contribution"] = "0 tasks: the published snapshot's schema (instance_id, repo_dir, ground_truth{read_core_files,modified_core_files,...}) carries neither base_commit nor a patch, so no Subject or Oracle can be derived from it (see docs/bench/scenarios.md)"
		}
	}

	// Every eligible instance, deduped and filtered to 1-5 non-test gold
	// files, sorted by instance_id — ADR-031's rule, reused verbatim.
	// RegisteredRule.N is deliberately NOT applied yet: N-capping happens
	// below, after the merged_at preference, so "first N" means "first N
	// of the preferred order", not "first N of instance_id order".
	rule := publicloc.SelectionRule{MinFiles: publicloc.RegisteredRule.MinFiles, MaxFiles: publicloc.RegisteredRule.MaxFiles, N: len(instances)}
	eligible, selRep := publicloc.Select(instances, rule)

	preferred, other, postCount, otherCount := partitionByCutoff(eligible, extra, cutoffTime)
	ordered := append(append([]publicloc.Instance{}, preferred...), other...)
	notes["eligible_candidates"] = len(eligible)
	notes["excluded_no_gold_files"] = selRep.ExcludedNoGoldFiles
	notes["excluded_too_many_files"] = selRep.ExcludedTooManyFiles
	notes["excluded_duplicate"] = selRep.ExcludedDuplicate
	notes["merged_at_after_cutoff"] = postCount
	notes["merged_at_before_cutoff_or_unknown"] = otherCount
	notes["merged_at_cutoff"] = cutoff
	if postCount == 0 {
		notes["merged_at_availability"] = "neither pinned source's rows carry a merge/close timestamp (verified 2026-09-13); the cutoff preference is a documented no-op until a source revision adds one"
	}
	if len(ordered) > n {
		ordered = ordered[:n]
	}
	notes["selected"] = len(ordered)
	repoSet := map[string]bool{}
	for _, in := range ordered {
		repoSet[in.RepoURL()] = true
	}
	repos := make([]string, 0, len(repoSet))
	for r := range repoSet {
		repos = append(repos, r)
	}
	sort.Strings(repos)
	notes["repos_selected"] = repos
	if len(repos) == 1 {
		notes["repo_diversity"] = "all " + fmt.Sprint(len(ordered)) + " selected instances are from one repo — ADR-031's rule sorts by instance_id and this repo alone supplied >= N eligible candidates; a future revision could stratify by repo, not done here (out of SB-34's scope)"
	}

	tasks := make([]Task, 0, len(ordered))
	for _, in := range ordered {
		task, derivErr := deriveTask(in, extra[in.InstanceID], opts, cutoffTime)
		if derivErr != nil {
			return Corpus{}, fmt.Errorf("scenariobench: deriving task for %s: %w", in.InstanceID, derivErr)
		}
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })

	sort.Slice(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })

	return Corpus{
		Version:     1,
		Description: Description,
		Arms: map[string]Arm{
			"full":    {Wired: true, Instructions: "stanza", Surface: "fathom"},
			"control": {Wired: false, Instructions: "plain", NotesFile: true},
		},
		Tasks: tasks,
		Criterion: Criterion{
			ID: "M2-1", Treatment: "full", Control: "control",
			MinTrials: 2, RequireSuccessAtLeastEqual: true,
			MaxTokensPerSuccessRatio: 1.0, Registered: "2026-09-13",
		},
		Sources: sources,
		Notes:   notes,
	}, nil
}

// partitionByCutoff sorts eligible (already instance_id-sorted by Select)
// into merged-after-cutoff first, everything else (before cutoff or
// unknown) second — each half keeping Select's instance_id order — so
// "prefer merged_at after cutoff" never disturbs determinism.
func partitionByCutoff(eligible []publicloc.Instance, extra map[string]extraFields, cutoff time.Time) (preferred, other []publicloc.Instance, postCount, otherCount int) {
	for _, in := range eligible {
		if t, ok := mergedAtTime(extra[in.InstanceID]); ok && t.After(cutoff) {
			preferred = append(preferred, in)
			postCount++
			continue
		}
		other = append(other, in)
		otherCount++
	}
	return preferred, other, postCount, otherCount
}

func mergedAtTime(f extraFields) (time.Time, bool) {
	if f.MergedAt == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, f.MergedAt)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// deriveTask builds one scenariobench.Task from a selected instance.
func deriveTask(in publicloc.Instance, ex extraFields, opts DeriveOptions, cutoff time.Time) (Task, error) {
	files, err := publicloc.GoldFiles(in)
	if err != nil {
		return Task{}, err
	}
	patchFiles, err := publicloc.ParsePatch(in.Patch)
	if err != nil {
		return Task{}, err
	}

	var (
		requires   []string
		goldFuncs  []string
		category   Category
		evidence   Evidence
		cloneNotes []string
	)

	before, after, cloneErr := buildTrees(opts, in, files, patchFiles)
	switch {
	case cloneErr != nil:
		requires = append(requires, "subject-clone")
		cloneNotes = append(cloneNotes, "subject clone unavailable ("+cloneErr.Error()+"): gold files come from the diff's paths only, gold functions are empty")
		category, evidence = ClassifyChange([]byte(in.Patch), nil, nil)
	default:
		funcs, funcErr := deriveGoldFunctions(before, patchFiles, files)
		if funcErr != nil {
			cloneNotes = append(cloneNotes, "gold-function derivation failed ("+funcErr.Error()+"): gold functions are empty")
		} else {
			goldFuncs = funcs
		}
		category, evidence = ClassifyChange([]byte(in.Patch), before, after)
	}

	sessions := Sessions{Count: 2, MaxTurns: 40, Mode: ModeSingle}
	oracle := Oracle{FailToPass: ex.FailToPass, PassToPass: ex.PassToPass, Coverage: true}
	gold := Gold{Files: files, Functions: goldFuncs, PatchSHA: publicloc.SHA256Hex([]byte(in.Patch))}
	if ex.TestPatch != "" {
		gold.TestPatch = ex.TestPatch
		gold.TestPatchSHA256 = publicloc.SHA256Hex([]byte(ex.TestPatch))
	}

	var mergedAt string
	if t, ok := mergedAtTime(ex); ok {
		mergedAt = t.Format(time.RFC3339)
	}

	var oracleNote string
	if len(oracle.FailToPass) == 0 && len(oracle.PassToPass) == 0 {
		oracleNote = "; the source recorded no fail-to-pass or pass-to-pass test names for this instance"
	}
	if gold.TestPatch == "" {
		oracleNote += "; the source recorded no test patch for this instance"
	}

	derivation := fmt.Sprintf(
		"%s %s @ %s (%s). Selected under ADR-031's rule: gold patch touches %d non-test Go file(s). Category %q by %s%s.%s",
		in.Source, in.InstanceID, in.BaseCommit, publicloc.Sources[in.Source].License,
		len(files), category, evidence.Rule, formatSkipped(evidence.Skipped), oracleNote,
	)
	if len(cloneNotes) > 0 {
		derivation += " " + strings.Join(cloneNotes, "; ") + "."
	}

	taskID := "SB-" + in.SafeID()
	task := Task{
		ID:          taskID,
		Category:    category,
		Prompt:      taskPrompt(in),
		Subject:     Subject{RepoURL: in.RepoURL(), BaseCommit: in.BaseCommit, Service: in.Service(), Language: "go"},
		Sessions:    sessions,
		Oracle:      oracle,
		Gold:        gold,
		Predictions: predictionsFor(category),
		Derivation:  derivation,
		Requires:    requires,
		MergedAt:    mergedAt,
	}
	if reason, ok := droppedTaskReasons[taskID]; ok {
		task.Disposition = "dropped"
		task.Reason = reason
	}
	return task, nil
}

// droppedTaskReasons names every derived task the driver must never run
// (RC-15, vision 28 §2 F10): an oracle that cannot be won from the task
// text at all, discovered by hand after the fact — never something
// ClassifyChange or the source snapshot can detect on its own, so it is
// applied here as a keyed override rather than a derivation rule, and
// still reproduces the committed corpus byte-for-byte
// (TestPublicScenarioCorpusIsDerived).
var droppedTaskReasons = map[string]string{
	"SB-cli__cli-10009": "oracle honesty (vision 28 F10): fail_to_pass names internal/safepaths tests (TestJoin, TestParseAbsolutePath, TestAbsoluteEmptyPathStringPanic, TestPathTraversalErrorMessage) of a package that does not exist at base_commit — the gold patch's own test_patch INTRODUCES internal/safepaths/absolute_test.go. No agent, however correct, can pass an oracle asserting a package it has not yet been told to create; the driver skips this task.",
}

func formatSkipped(skipped []string) string {
	if len(skipped) == 0 {
		return ""
	}
	return " (skipped: " + strings.Join(skipped, "; ") + ")"
}

func taskPrompt(in publicloc.Instance) string {
	return fmt.Sprintf(
		"The following issue was filed against %s/%s at commit %s. Fix it. Keep the fix scoped to what the issue describes.\n\nIssue (verbatim):\n\n%s",
		in.Org, in.Repo, in.BaseCommit, strings.TrimSpace(in.Problem),
	)
}

// predictionsFor is vision 27 §1's M2-M4 rows applied per SB-34: a
// single-file mechanical change (bug_fix, and by the same reasoning a
// dependency_upgrade) predicts no effect; a change that spans packages, is
// governance-bound, or changes an exported signature predicts the wired
// arm wins. Every corpus arm gets an entry (SB-26's gate).
func predictionsFor(cat Category) map[string]string {
	switch cat {
	case CategoryFeature, CategorySignature, CategoryGovernance:
		return map[string]string{
			"full":    "wins — vision 27 M2/M3: a cross-package, signature, or governance-bound change is where index-grounded plans and the verify gate diverge from a plain checklist",
			"control": "loses relative to full on this task",
		}
	default: // bug_fix, dependency_upgrade, and anything not yet registered
		return map[string]string{
			"full":    "no effect — vision 27 M2: a single-file mechanical change is where a plain checklist already succeeds",
			"control": "no effect — same",
		}
	}
}

// ReuseDescription is fixed text for DeriveReuseTasks (vision 27 M5,
// docs/bench/scenarios.md §exploration-reuse), written for the registered
// call (n=3, workers=3) that produced testdata/scenariobench/reuse-go.json
// — like Description, no timestamp or hostname, so two identical
// derivations byte-match.
const ReuseDescription = "Exploration reuse (vision 27 M5): the three lowest-id cross_package_feature tasks of public-go.json run in planner-workers mode — one planning session decomposes the change into rows, then K=3 worker sessions each build one row cold. The reading is ReexplorationOverlap: bytes each worker reads that the planner already read, over bytes it reads at all. Derived by scripts in docs/bench/scenarios.md §exploration-reuse from public-go.json (same pins, same oracle); never hand-edited."

// reuseSessionMaxTurns is the fixed turn budget of a reuse-corpus session —
// independent of the base task's own Sessions.MaxTurns, since a
// planner-workers session shape is not the same budget as the base
// task's single-mode sessions.
const reuseSessionMaxTurns = 30

// reuseArms is the two-arm design testdata/scenariobench/reuse-go.json
// registers: full (wired, compiled obligations) against a plain, notes-file
// control, each carrying its own pre-registered corpus-level prediction
// (Arm.Prediction) alongside the per-task predictions.
func reuseArms() map[string]Arm {
	return map[string]Arm{
		"full": {
			Wired: true, CompileLayers: true, Instructions: "stanza", Surface: "fathom",
			Prediction: "workers inherit the planner's map through the plan rows' ctx packs and the index: overlap ratio well under the control's, tokens to first edit lower per worker",
		},
		"control": {
			Wired: false, Instructions: "plain", NotesFile: true,
			Prediction: "each worker re-reads what the planner read: overlap ratio high (the vision's falsifier is a control overlap under 30%)",
		},
	}
}

// reusePredictionsK is the K>1 (multi-worker) per-task predictions row.
func reusePredictionsK() map[string]string {
	return map[string]string{
		"full":    "planner explores once through repo_map/pack_context/blast radius; workers start from the row's ctx pack — overlap low, workers reach first edit sooner",
		"control": "planner explores with grep/Read and writes prose rows; each worker re-explores the same files — overlap high",
	}
}

// reusePredictionsK1 is the pre-registered no-effect cell (vision 27 M5):
// with one worker there is nothing to compare overlap against.
func reusePredictionsK1() map[string]string {
	return map[string]string{
		"full":    "no effect: with one worker there is nothing to reuse across workers — the overlap reading is defined only against the planner, and vision 27 M5 predicts the pack advantage vanishes at K = 1",
		"control": "no effect for the same reason; this variant is the matrix's no-effect cell",
	}
}

// DeriveReuseTasks derives the exploration-reuse corpus (vision 27 M5,
// docs/bench/scenarios.md §exploration-reuse) from an already-derived
// public corpus pub: the n lowest-id cross_package_feature tasks,
// re-shaped into planner-workers mode with `workers` worker sessions
// (oracle, gold, prompt and subject copied verbatim from the base task —
// only the session shape, predictions, and derivation note differ), plus
// a K=1 no-effect variant of the first (lowest-id) selected task. Never
// spends an LLM call and never reads a file; pub is expected to already
// be validated (DeriveScenarioTasks or LoadScenarioCorpus). Calling this
// with n=3, workers=3 reproduces testdata/scenariobench/reuse-go.json
// byte-for-byte from testdata/scenariobench/public-go.json.
func DeriveReuseTasks(pub Corpus, n, workers int) Corpus {
	var candidates []Task
	for _, t := range pub.Tasks {
		if t.Category == CategoryFeature {
			candidates = append(candidates, t)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	if n >= 0 && n < len(candidates) {
		candidates = candidates[:n]
	}

	tasks := make([]Task, 0, len(candidates)+1)
	for _, base := range candidates {
		tasks = append(tasks, reshapeForReuse(base, workers, false))
	}
	if len(candidates) > 0 {
		tasks = append(tasks, reshapeForReuse(candidates[0], 1, true))
	}

	return Corpus{
		Version:     1,
		Description: ReuseDescription,
		Arms:        reuseArms(),
		Tasks:       tasks,
		Criterion: Criterion{
			ID: "M5-1", Treatment: "full", Control: "control",
			MinTrials: 2, RequireSuccessAtLeastEqual: true,
			MaxTokensPerSuccessRatio: 1.0, Registered: "2026-09-13",
		},
		Sources: pub.Sources,
	}
}

// reshapeForReuse builds one reuse-corpus task from a base public-corpus
// task: the ID gains an "RE-" prefix (and, for the K=1 variant, a "-K1"
// suffix), Sessions becomes planner-workers with the given worker count,
// Predictions and Derivation are replaced/extended — everything else
// (Prompt, Subject, Oracle, Gold, Category) is copied verbatim.
func reshapeForReuse(base Task, workers int, k1 bool) Task {
	id := "RE-" + base.ID
	derivation := base.Derivation + " | RE-: re-shaped for planner-workers mode (K=3, one planning session, one worker per row) by the exploration-reuse derivation; oracle and gold unchanged."
	predictions := reusePredictionsK()
	if k1 {
		id += "-K1"
		derivation += " | -K1: the same task with one worker, the pre-registered no-effect cell for M5."
		predictions = reusePredictionsK1()
	}
	t := base
	t.ID = id
	t.Sessions = Sessions{Count: 1, MaxTurns: reuseSessionMaxTurns, Workers: workers, Mode: ModePlannerWorkers}
	t.Predictions = predictions
	t.Derivation = derivation
	return t
}

// extraFields is what the pinned Multi-SWE-bench snapshot carries beyond
// what publicloc.ReadMultiSWEBench normalises: the fail-to-pass /
// pass-to-pass test names (SB-34 "from the source fields") and, if a
// future revision adds one, a merge timestamp.
type extraFields struct {
	FailToPass []string
	PassToPass []string
	MergedAt   string
	// TestPatch is the source's own hidden test patch (SWE-bench
	// semantics — docs/bench/scenarios.md "Test patch and calibration"),
	// not read by publicloc.ReadMultiSWEBench.
	TestPatch string
}

type extraFieldsRow struct {
	InstanceID string                     `json:"instance_id"`
	Org        string                     `json:"org"`
	Repo       string                     `json:"repo"`
	Number     int                        `json:"number"`
	MergedAt   string                     `json:"merged_at"`
	ClosedAt   string                     `json:"closed_at"`
	F2P        map[string]json.RawMessage `json:"f2p_tests"`
	N2P        map[string]json.RawMessage `json:"n2p_tests"`
	P2P        map[string]json.RawMessage `json:"p2p_tests"`
	TestPatch  string                     `json:"test_patch"`
}

// scanExtraFields is a best-effort side read of the same JSONL bytes
// publicloc.ReadMultiSWEBench parses strictly: a row this cannot decode is
// skipped here (never fatal — the strict reader already validated the
// file's required fields), because f2p_tests/n2p_tests/p2p_tests/merged_at
// are optional extras this corpus wants, not the sibling corpus's contract.
func scanExtraFields(raw []byte) (map[string]extraFields, error) {
	out := map[string]extraFields{}
	err := publicloc.ReadJSONL(bytes.NewReader(raw), func(_ int, line []byte) error {
		var row extraFieldsRow
		if err := json.Unmarshal(line, &row); err != nil {
			return nil
		}
		id := row.InstanceID
		if id == "" && row.Org != "" && row.Repo != "" {
			id = fmt.Sprintf("%s__%s-%d", row.Org, row.Repo, row.Number)
		}
		if id == "" {
			return nil
		}
		f := extraFields{FailToPass: sortedKeys(row.F2P, row.N2P), PassToPass: sortedKeys(row.P2P), TestPatch: row.TestPatch}
		if row.MergedAt != "" {
			f.MergedAt = row.MergedAt
		} else if row.ClosedAt != "" {
			f.MergedAt = row.ClosedAt
		}
		out[id] = f
		return nil
	})
	return out, err
}

func sortedKeys(maps ...map[string]json.RawMessage) []string {
	seen := map[string]bool{}
	for _, m := range maps {
		for k := range m {
			seen[k] = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func countJSONLRows(raw []byte) int {
	n := 0
	_ = publicloc.ReadJSONL(bytes.NewReader(raw), func(_ int, _ []byte) error {
		n++
		return nil
	})
	return n
}

// deriveGoldFunctions is publicloc.EnclosingFuncsForHunks applied per gold
// file, reading each file's base_commit content from the subject clone
// (SB-34: "gold files/functions via EnclosingFuncsForHunks at base_commit").
func deriveGoldFunctions(before fs.FS, patchFiles []publicloc.PatchFile, files []string) ([]string, error) {
	var names []string
	seen := map[string]bool{}
	for _, path := range files {
		var hunks []publicloc.Hunk
		for _, pf := range patchFiles {
			if pf.OldPath == path && !pf.Added {
				hunks = append(hunks, pf.Hunks...)
			}
		}
		if len(hunks) == 0 {
			continue
		}
		src, err := fs.ReadFile(before, path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		spans, err := publicloc.EnclosingFuncsForHunks(src, path, hunks)
		if err != nil {
			return nil, err
		}
		for _, s := range spans {
			if !seen[s.Name] {
				seen[s.Name] = true
				names = append(names, s.Name)
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

// buildTrees fetches the gold files at base_commit from a shallow subject
// clone (the "before" tree) and reconstructs the "after" tree by applying
// each file's hunks in memory — no second network fetch, and no
// dependency on the upstream PR's merge/head commit existing at all.
func buildTrees(opts DeriveOptions, in publicloc.Instance, files []string, patchFiles []publicloc.PatchFile) (fs.FS, fs.FS, error) {
	if opts.SubjectsDir == "" || opts.SkipClone {
		return nil, nil, fmt.Errorf("cloning disabled")
	}
	clone, err := ensureRepoClone(opts.SubjectsDir, in.RepoURL())
	if err != nil {
		return nil, nil, err
	}
	timeout := opts.GitTimeout
	if timeout == 0 {
		timeout = DefaultGitTimeout
	}
	if err := clone.fetchCommit(in.BaseCommit, timeout); err != nil {
		return nil, nil, err
	}

	before := fstest.MapFS{}
	after := fstest.MapFS{}
	for _, path := range files {
		var hunks []publicloc.Hunk
		for _, pf := range patchFiles {
			if pf.OldPath == path && !pf.Added {
				hunks = pf.Hunks
			}
		}
		raw, err := clone.readFile(in.BaseCommit, path)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", path, err)
		}
		before[path] = &fstest.MapFile{Data: raw, Mode: 0o644}
		afterRaw, err := applyHunks(raw, hunks)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: applying patch: %w", path, err)
		}
		after[path] = &fstest.MapFile{Data: afterRaw, Mode: 0o644}
	}
	return before, after, nil
}

// applyHunks reconstructs a file's after-content from its base_commit
// content and the hunks a unified diff records against it — no shelling
// out to `patch`, no second commit fetch.
func applyHunks(original []byte, hunks []publicloc.Hunk) ([]byte, error) {
	// git always diffs complete lines; a trailing empty element from a
	// trailing newline is intentional and preserved by the join below.
	lines := strings.Split(string(original), "\n")
	var out []string
	pos := 0
	for _, h := range hunks {
		start := h.OldStart - 1
		if start < 0 {
			start = 0
		}
		if start > len(lines) {
			return nil, fmt.Errorf("hunk starts at line %d, file has %d lines", h.OldStart, len(lines))
		}
		out = append(out, lines[pos:start]...)
		pos = start
		for _, bl := range h.Body {
			line := bl
			if line == "" {
				line = " "
			}
			switch line[0] {
			case ' ':
				if pos >= len(lines) {
					return nil, fmt.Errorf("context line beyond end of file (line %d)", pos+1)
				}
				out = append(out, lines[pos])
				pos++
			case '-':
				if pos >= len(lines) {
					return nil, fmt.Errorf("deleted line beyond end of file (line %d)", pos+1)
				}
				pos++
			case '+':
				out = append(out, line[1:])
			case '\\':
				// "\ No newline at end of file" — not a counted line.
			default:
				return nil, fmt.Errorf("unexpected hunk body prefix %q", line[:1])
			}
		}
	}
	out = append(out, lines[pos:]...)
	return []byte(strings.Join(out, "\n")), nil
}

// repoClone is a shallow, incrementally-fetched local mirror of one
// subject repository under DeriveOptions.SubjectsDir, keyed by clone
// directory so one repo is cloned once no matter how many of its commits
// are used across the corpus.
type repoClone struct {
	dir string
}

var repoClones = map[string]*repoClone{}

// ensureRepoClone returns the (possibly cached) clone for repoURL,
// creating a bare, checkout-free clone on first use.
func ensureRepoClone(subjectsDir, repoURL string) (*repoClone, error) {
	name := safeRepoDirName(repoURL)
	dir := filepath.Join(subjectsDir, name)
	if c, ok := repoClones[dir]; ok {
		return c, nil
	}
	if _, err := os.Stat(filepath.Join(dir, "HEAD")); err != nil {
		if err := os.MkdirAll(subjectsDir, 0o755); err != nil {
			return nil, err
		}
		cmd := exec.Command("git", "clone", "--quiet", "--bare", "--filter=blob:none", repoURL, dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git clone %s: %w: %s", repoURL, err, strings.TrimSpace(string(out)))
		}
	}
	c := &repoClone{dir: dir}
	repoClones[dir] = c
	return c, nil
}

func safeRepoDirName(repoURL string) string {
	s := strings.TrimSuffix(repoURL, ".git")
	s = strings.TrimPrefix(s, "https://github.com/")
	s = strings.ReplaceAll(s, "/", "__")
	return s + ".git"
}

// fetchCommit is idempotent: a commit already present (fetched for an
// earlier task in the same repo, or in a previous run against the same
// SubjectsDir) is not re-fetched.
func (c *repoClone) fetchCommit(sha string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := exec.CommandContext(ctx, "git", "-C", c.dir, "cat-file", "-e", sha+"^{commit}").Run(); err == nil {
		return nil
	}
	cmd := exec.CommandContext(ctx, "git", "-C", c.dir, "fetch", "--quiet", "--depth", "1", "origin", sha)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch %s: %w: %s", sha, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *repoClone) readFile(sha, path string) ([]byte, error) {
	out, err := exec.Command("git", "-C", c.dir, "show", sha+":"+path).Output()
	if err != nil {
		return nil, fmt.Errorf("git show %s:%s: %w", sha, path, err)
	}
	return out, nil
}
