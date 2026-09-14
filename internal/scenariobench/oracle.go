// RunOracle (SB-16) decides success after the last session, in the
// worktree, by the driver — never by the agent (vision 27 §2). It runs the
// subject's own test command once, separately, over fail-to-pass and
// pass-to-pass so each can be reported independently, with a coverage
// profile when the task asks for one, then runs every detector.
//
// A task carrying Gold.TestPatch follows SWE-bench semantics: the test
// patch is hidden from every session and applied by the oracle itself,
// after the sessions and before the tests, then calibrated against a
// throwaway worktree at base_commit (CalibrateSubject) so a fail_to_pass
// test that never failed at base, or a pass_to_pass test broken by the
// environment rather than the agent, cannot decide the verdict — see
// docs/bench/scenarios.md "Test patch and calibration".
package scenariobench

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// defaultTestCommand is used when Subject.TestCommand is unset (types.go).
const defaultTestCommand = "go test -run '{{tests}}' ./..."

// goTestPassRE / goTestFailRE match `go test -v`'s per-test result lines:
// "--- PASS: TestName (0.00s)" / "--- FAIL: TestName (0.00s)".
var (
	goTestPassRE = regexp.MustCompile(`(?m)^\s*--- PASS:\s+(\S+)`)
	goTestFailRE = regexp.MustCompile(`(?m)^\s*--- FAIL:\s+(\S+)`)
)

// RunOracle runs task's oracle against wt: the test command over the union
// of fail-to-pass and pass-to-pass test names (with -coverprofile when
// Oracle.Coverage), decided per-test from `go test -v`'s own PASS/FAIL
// lines so FailToPassOK and PassToPassOK are independently meaningful,
// then every detector, then the answer-only oracle (Oracle.AnswerAllOf)
// against the last of sessions. Output (test stdout, the coverage
// profile) is written under wt.CellDir and referenced from the result.
// root is this repo's root, passed through to CalibrateSubject when task
// carries a Gold.TestPatch — otherwise unused.
func RunOracle(ctx context.Context, wt Worktree, task Task, sessions []SessionResult, root string) (OracleResult, error) {
	var result OracleResult

	hasTestPatch := task.Gold.TestPatch != ""
	if hasTestPatch {
		if err := ApplyTestPatch(ctx, wt, task); err != nil {
			result.Calibration = &CalibrationSummary{NotMeasurable: fmt.Sprintf("test patch did not apply: %v", err)}
			result.Passed = false
			return result, nil
		}
	}

	names := append(append([]string(nil), task.Oracle.FailToPass...), task.Oracle.PassToPass...)
	var passed map[string]bool
	if len(names) > 0 {
		p, outputRef, coverRef, err := runOracleTests(ctx, wt, task, names)
		if err != nil {
			return OracleResult{}, fmt.Errorf("running oracle tests: %w", err)
		}
		passed = p
		result.TestOutputRef = outputRef
		result.CoverProfile = coverRef
	}

	switch {
	case hasTestPatch:
		calib, err := CalibrateSubject(ctx, task, wt, root)
		if err != nil {
			return OracleResult{}, fmt.Errorf("calibrating subject: %w", err)
		}
		if calib.NotMeasurable != "" {
			result.Calibration = &CalibrationSummary{
				FailToPassTotal: len(task.Oracle.FailToPass), PassToPassTotal: len(task.Oracle.PassToPass),
				NotMeasurable: calib.NotMeasurable,
			}
			result.Passed = false
			return result, nil
		}
		failedAtBase := toSet(calib.FailToPassFailedAtBase)
		if len(failedAtBase) == 0 {
			result.Calibration = &CalibrationSummary{
				FailToPassTotal: len(task.Oracle.FailToPass), PassToPassTotal: len(task.Oracle.PassToPass),
				PassToPassEffective: len(calib.PassToPassPassedAtBase),
				NotMeasurable:       "no fail_to_pass test fails at base with the test patch applied",
			}
			result.Passed = false
			return result, nil
		}
		passedAtBase := toSet(calib.PassToPassPassedAtBase)
		result.FailToPassOK = allPassedInSet(passed, failedAtBase)
		result.PassToPassOK = allPassedInSet(passed, passedAtBase)
		result.Calibration = &CalibrationSummary{
			FailToPassFailedAtBase: len(failedAtBase), FailToPassTotal: len(task.Oracle.FailToPass),
			PassToPassEffective: len(passedAtBase), PassToPassTotal: len(task.Oracle.PassToPass),
		}
	case len(names) > 0:
		result.FailToPassOK = allPassed(passed, task.Oracle.FailToPass)
		result.PassToPassOK = allPassed(passed, task.Oracle.PassToPass)
	default:
		// No named tests: nothing to fail, vacuously OK.
		result.FailToPassOK = true
		result.PassToPassOK = true
	}

	for _, d := range task.Oracle.Detectors {
		dr, err := runDetector(ctx, wt, task, d)
		if err != nil {
			return OracleResult{}, fmt.Errorf("running detector %s: %w", d.Name, err)
		}
		result.Detectors = append(result.Detectors, dr)
	}

	// O57 finding 2: a refusal-aware task's own outcome (refusal.go)
	// SUPERSEDES the ordinary detector-based Passed computation below —
	// the change-present detector and the any-edits floor both read a
	// correct refusal identically to a violation, which a rule that
	// forbids an action can never adjudicate correctly. Detectors above
	// still ran (kept on result.Detectors for visibility/debugging), but
	// do not decide Passed for this task.
	if RefusalAware(task) {
		outcome, passed, rerr := ScoreRefusal(ctx, wt, task, sessions)
		if rerr != nil {
			return OracleResult{}, rerr
		}
		result.Outcome = outcome
		result.Passed = passed
		return result, nil
	}

	answerOK := true
	if len(task.Oracle.AnswerAllOf) > 0 || len(task.Oracle.AnswerAnyOf) > 0 {
		answerOK = oracleAnswerMatches(lastFinalAnswer(sessions), task.Oracle.AnswerAllOf, task.Oracle.AnswerAnyOf)
	}

	localizationOK := true
	if task.Oracle.Localization {
		loc := ScoreLocalization(lastFinalAnswer(sessions), task.Gold.Files)
		result.Localization = &loc
		// Success is the standard file-level accuracy@1 bar the
		// literature reports (R10: LocAgent 92.7% file-level) — the
		// top-ranked file in the answer must be a gold file. HitAt5/
		// Recall/Precision are still readings on every cell regardless
		// of this verdict (VB-25's own ledger fields), never gated on it.
		localizationOK = loc.NotMeasurable == "" && loc.HitAt1
	}

	result.Passed = result.FailToPassOK && result.PassToPassOK && !anyViolated(result.Detectors) && answerOK && localizationOK
	return result, nil
}

// LocalizationOnly reports whether task's oracle is localization-only: an
// Oracle.Localization with no tree-level check at all (fail_to_pass,
// pass_to_pass, detectors) — the LCA bug-localization shape, where
// nothing was ever meant to be edited (mirrors AnswerOnly).
func LocalizationOnly(task Task) bool {
	o := task.Oracle
	return o.Localization && len(o.FailToPass) == 0 && len(o.PassToPass) == 0 && len(o.Detectors) == 0
}

// AnswerOnly reports whether task's oracle is answer-only: an
// Oracle.AnswerAllOf and/or Oracle.AnswerAnyOf with no tree-level check
// at all (fail_to_pass, pass_to_pass, detectors) — SB-32's
// lookups-vs.json shape, where success cannot require an edited file
// because nothing was meant to be edited.
func AnswerOnly(task Task) bool {
	o := task.Oracle
	return (len(o.AnswerAllOf) > 0 || len(o.AnswerAnyOf) > 0) &&
		len(o.FailToPass) == 0 && len(o.PassToPass) == 0 && len(o.Detectors) == 0
}

// SuccessFor is the ONE any-edits-floor formula every caller that sets a
// CellResult's Success must use — runOneCell (cmd/scenariobench/main.go),
// runExternalCell (external_corpus.go) and cmdScore's rescoring
// (cmd/scenariobench/main.go) all called their own copy before the
// fix-first review that merged this: runExternalCell's copy lacked the
// RefusalAware exemption (O57 finding 2), and cmdScore's carried none of
// the three at all, so a refusal-aware or answer/localization-only task
// rescored via `scenariobench score` (no edit, by design) came back
// FAIL. oracle.Passed alone is never enough — an untouched tree cannot
// pass a change task (guards pilot 1) — UNLESS task's oracle is one of
// the three shapes where nothing was ever necessarily meant to be
// edited: answer-only (SB-32 lookups-vs.json), localization-only (VB-25),
// or refusal-aware (O57 finding 2, a correct refusal never edits
// anything by design).
func SuccessFor(task Task, oracle OracleResult, sessions []SessionResult) bool {
	return oracle.Passed && (AnswerOnly(task) || LocalizationOnly(task) || RefusalAware(task) || AnyEdits(sessions))
}

// lastFinalAnswer is the LAST session's FinalAnswer, exactly as the
// answer-only oracle is specified: not the first non-empty one, the last
// session's, empty if there were none.
func lastFinalAnswer(sessions []SessionResult) string {
	if len(sessions) == 0 {
		return ""
	}
	return sessions[len(sessions)-1].FinalAnswer
}

// oracleAnswerMatches is O53 defect 5's answer-only oracle: every allOf
// token AND every anyOf group (satisfied by any ONE of its members) must
// each find a match in answer — restricted to answer's own concluding
// span when it carries an explicit conclusion marker (concludingSpan,
// defect 5c), and counting only occurrences that are neither inside a
// negated clause nor solely inside a quoted line (tokenCounts, defect
// 5b). anyOf mirrors internal/contextbench's Score/AnswerAnyOf: a group
// is a spelling-equivalent alternative for ONE required fact, not a
// second, looser requirement.
func oracleAnswerMatches(answer string, allOf []string, anyOf [][]string) bool {
	span := answer
	if cs := concludingSpan(answer); cs != "" {
		span = cs
	}
	for _, tok := range allOf {
		if !tokenCounts(span, tok) {
			return false
		}
	}
	for _, group := range anyOf {
		matched := false
		for _, alt := range group {
			if tokenCounts(span, alt) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// boldConclusionRE matches a markdown **bold** span — concludingSpan's
// signal that a paragraph states the answer's own conclusion, not merely
// emphasizes a word in passing.
var boldConclusionRE = regexp.MustCompile(`\*\*[^*]+\*\*`)

// concludingSpanMarkers are the plain-text conclusion phrases
// concludingSpan looks for, checked case-insensitively, independent of
// paragraph position (unlike the bold-span rule, which is restricted to
// the LAST paragraph — a **bold** phrase elsewhere is common emphasis,
// not necessarily the conclusion).
var concludingSpanMarkers = []string{"the answer is", "answer:"}

// concludingSpan returns the portion of answer O53 defect 5c restricts
// oracle token matching to, when answer carries an explicit conclusion
// marker: a **bold** span in the answer's LAST paragraph (split on a
// blank line), or the LAST occurrence of a concludingSpanMarkers phrase,
// from there to the end of the answer. "" when answer carries no such
// marker — oracleAnswerMatches then searches the whole answer, exactly
// as before this fix (the H1-T13 false pass: a gold ADR id appeared only
// inside an earlier, quoted aside about a DIFFERENT decision, while the
// answer's own concluding paragraph named the wrong one — restricting
// the search to that concluding paragraph is what turns the false pass
// into a correct fail).
func concludingSpan(answer string) string {
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" {
		return ""
	}
	paras := strings.Split(trimmed, "\n\n")
	if last := paras[len(paras)-1]; boldConclusionRE.MatchString(last) {
		return last
	}
	lower := strings.ToLower(answer)
	bestIdx := -1
	for _, marker := range concludingSpanMarkers {
		if idx := strings.LastIndex(lower, marker); idx > bestIdx {
			bestIdx = idx
		}
	}
	if bestIdx >= 0 {
		return answer[bestIdx:]
	}
	return ""
}

// tokenCounts reports whether tok has at least one case-insensitive
// occurrence in text that COUNTS toward an oracle match: not inside a
// negated clause (clauseIsNegated, localization.go's own negation-marker
// logic — reused here rather than duplicated, per O53 defect 5b) and not
// solely inside a quoted line/span (isQuotedPosition). A token with every
// occurrence excluded this way does not count — the answer only CITED or
// REJECTED it, never asserted it as its own conclusion.
func tokenCounts(text, tok string) bool {
	if tok == "" {
		return true
	}
	lowerText := strings.ToLower(text)
	lowerTok := strings.ToLower(tok)
	for start := 0; start < len(lowerText); {
		idx := strings.Index(lowerText[start:], lowerTok)
		if idx < 0 {
			return false
		}
		pos := start + idx
		if !isQuotedPosition(text, pos) && !clauseIsNegated(text, pos) {
			return true
		}
		start = pos + 1
	}
	return false
}

// isQuotedPosition reports whether pos sits on a QUOTED LINE — a
// markdown blockquote (first non-whitespace character ">") or a line
// whose ENTIRE trimmed content is fenced start-to-end by the same
// backtick or quote mark — text the answer is CITING rather than
// asserting, so a gold token appearing only there must not count as the
// answer's own claim (O53 defect 5b).
//
// PIN (fix-first review, wave-n1): an inline backtick span inside an
// otherwise ordinary prose line — "whose distinctive identifier is
// `ADR-014`" — does NOT count as quoting, and never did by design;
// models routinely format identifiers, paths and symbol names as inline
// code, and excluding every backtick-wrapped span would reject the most
// common way a model states its own conclusion. Only a line that is
// ENTIRELY a quotation counts. "A line the prose rejects" is a SEPARATE
// concern — clauseIsNegated, checked independently by tokenCounts — this
// function only ever answers the QUOTING half.
func isQuotedPosition(text string, pos int) bool {
	lineStart := strings.LastIndexByte(text[:pos], '\n') + 1
	lineEnd := len(text)
	if rel := strings.IndexByte(text[pos:], '\n'); rel >= 0 {
		lineEnd = pos + rel
	}
	trimmed := strings.TrimSpace(text[lineStart:lineEnd])
	if strings.HasPrefix(trimmed, ">") {
		return true
	}
	return isWhollyDelimited(trimmed, '`') ||
		isWhollyDelimited(trimmed, '"') ||
		isWhollyDelimited(trimmed, '\'')
}

// isWhollyDelimited reports whether trimmed (a line's own trimmed
// content, at least 2 bytes) both starts AND ends with delim — a line
// that is nothing but "`ADR-014`" on its own, never an inline span
// inside a longer sentence (isQuotedPosition's whole point, per the pin
// above).
func isWhollyDelimited(trimmed string, delim byte) bool {
	return len(trimmed) >= 2 && trimmed[0] == delim && trimmed[len(trimmed)-1] == delim
}

func allPassed(passed map[string]bool, names []string) bool {
	for _, n := range names {
		if !passed[n] {
			return false
		}
	}
	return true
}

// allPassedInSet is allPassed over a set rather than a slice — used for
// the test-patch calibration path, where the requirement is "every test
// IN THIS SET passed", not "every named test passed".
func allPassedInSet(passed map[string]bool, set map[string]bool) bool {
	for n := range set {
		if !passed[n] {
			return false
		}
	}
	return true
}

func toSet(names []string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

func anyViolated(drs []DetectorResult) bool {
	for _, d := range drs {
		if d.Violated {
			return true
		}
	}
	return false
}

// ApplyTestPatch applies task.Gold.TestPatch into wt.Path. SWE-bench
// semantics: the patch was cut against base_commit, so BEFORE applying it
// every path the patch's own `diff --git a/X b/X` headers name is reset to
// its base_commit content — `git checkout base_commit -- path` for a path
// that exists there, `rm -f` for one the patch creates — so a patch does
// not fail with "does not match index"/"patch does not apply" merely
// because the agent's own (non-test) edit also touched one of those files
// (public pilot 1, 2026-09-13). Every OTHER file the agent touched is left
// exactly as the agent left it. `git apply --3way` is then tried first (a
// patch whose context has otherwise drifted still merges when the blobs
// it names are reachable), falling back to a plain `git apply`. A task
// with no test patch is a no-op. RunOracle calls this AFTER the sessions
// and BEFORE the tests — the patch is hidden from every session, exactly
// as SWE-bench evaluates.
func ApplyTestPatch(ctx context.Context, wt Worktree, task Task) error {
	if task.Gold.TestPatch == "" {
		return nil
	}
	if task.Subject.BaseCommit != "" {
		// wt.LocalBase, never task.Subject.BaseCommit — see
		// Worktree.LocalBase's own doc comment; the guard above still
		// reads the task record (does THIS task even have a pinned base
		// commit at all), never the worktree-local ref used below.
		if err := resetTestPatchTargets(ctx, wt, wt.LocalBase, task.Gold.TestPatch); err != nil {
			return fmt.Errorf("resetting test patch targets to base_commit: %w", err)
		}
	}
	patch := []byte(task.Gold.TestPatch)
	if err := runGitApply(ctx, wt.Path, patch, true); err != nil {
		if err2 := runGitApply(ctx, wt.Path, patch, false); err2 != nil {
			return fmt.Errorf("git apply --3way: %v; git apply: %w", err, err2)
		}
	}
	return nil
}

// diffGitHeaderRE matches a unified diff's own "diff --git a/X b/X"
// header line; the b/ side is the patch's post-image path (identical to
// a/ except for a rename — resetTestPatchTargets treats a rename's a/
// side the same as any other path the patch does not itself name: left
// alone, since the patch's hunks address it only by its b/ name).
var diffGitHeaderRE = regexp.MustCompile(`(?m)^diff --git a/(.+) b/(.+)$`)

// testPatchPaths returns the deduplicated b/-side paths a patch's
// `diff --git a/X b/X` headers name, in header order.
func testPatchPaths(patch string) []string {
	var paths []string
	seen := map[string]bool{}
	for _, m := range diffGitHeaderRE.FindAllStringSubmatch(patch, -1) {
		p := m[2]
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	return paths
}

// resetTestPatchTargets resets every path patch's `diff --git` headers
// name, in wt.Path, to its localBase content (wt.LocalBase — NEVER
// task.Subject.BaseCommit, which is not a valid ref inside wt at all; see
// Worktree.LocalBase): `git checkout localBase -- path` when the path
// exists at localBase (undoing whatever the agent did to it, so the
// patch — cut against exactly that content — applies cleanly), `rm -f`
// when it does not (the patch creates a new file at that path; an agent
// that independently created something at the same path must not
// collide with it). Every path the patch does NOT name is left
// untouched — an agent's non-test edits survive intact.
func resetTestPatchTargets(ctx context.Context, wt Worktree, localBase, patch string) error {
	for _, path := range testPatchPaths(patch) {
		existsAtBase := exec.CommandContext(ctx, "git", "cat-file", "-e", localBase+":"+path)
		existsAtBase.Dir = wt.Path
		if err := existsAtBase.Run(); err == nil {
			if out, err := runIn(ctx, wt.Path, "git", "checkout", localBase, "--", path); err != nil {
				return fmt.Errorf("checking out %s at %s: %w (%s)", path, localBase, err, out)
			}
			continue
		}
		full := filepath.Join(wt.Path, filepath.FromSlash(path))
		if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing %s (not present at %s, the test patch creates it): %w", path, localBase, err)
		}
	}
	return nil
}

func runGitApply(ctx context.Context, dir string, patch []byte, threeWay bool) error {
	args := []string{"apply"}
	if threeWay {
		args = append(args, "--3way")
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(patch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w (%s)", strings.Join(cmd.Args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Calibration is one (subject repo, base_commit, test-patch sha256)
// reading: which of the task's fail_to_pass tests actually FAILED at base
// with the test patch applied (only those are this task's to prove — a
// f2p test that already passed at base proves nothing about the agent's
// fix), and which of its pass_to_pass tests actually PASSED at base (only
// those can be broken BY the agent — one already failing at base is an
// environment problem, not the agent's). Cached under
// <root>/workspace/scenariobench/calibration/<key>.json, computed once no
// matter how many tasks or trials share the triple.
type Calibration struct {
	FailToPassFailedAtBase []string `json:"fail_to_pass_failed_at_base"`
	PassToPassPassedAtBase []string `json:"pass_to_pass_passed_at_base"`
	NotMeasurable          string   `json:"not_measurable,omitempty"`
}

// calibrationKey identifies one (subject repo, base_commit, test-patch
// sha256) triple — the granularity CalibrateSubject caches at.
func calibrationKey(task Task) string {
	sha := task.Gold.TestPatchSHA256
	if sha == "" && task.Gold.TestPatch != "" {
		sum := sha256.Sum256([]byte(task.Gold.TestPatch))
		sha = hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256([]byte(task.Subject.RepoURL + "\n" + task.Subject.BaseCommit + "\n" + sha))
	return hex.EncodeToString(sum[:])
}

// CalibrateSubject computes (or, after the first call, reads back) the
// Calibration for task: a SEPARATE throwaway worktree of the shared clone
// (wt.SubjectClone, falling back to wt.Path when unset) at base_commit —
// NEVER the cell's own worktree, so calibration never disturbs a cell in
// flight — with the test patch applied and no fix, running task's test
// command once over fail_to_pass ∪ pass_to_pass. root is this repo's
// root; the cache lives at
// <root>/workspace/scenariobench/calibration/<key>.json.
// calibrationLocks serializes CalibrateSubject per calibration key,
// process-wide (see the comment at its use).
var calibrationLocks sync.Map // map[string]*sync.Mutex

// prepareCalibrationWorktree clears any leftover of a previous calibration
// at calibWT — the directory AND git's registration of it (a crashed or
// killed run leaves a "missing but already registered worktree" that
// `git worktree add` refuses) — then adds a detached worktree at commit.
// It returns git's combined output for the error message.
func prepareCalibrationWorktree(ctx context.Context, clone, calibWT, commit string) ([]byte, error) {
	if err := os.RemoveAll(calibWT); err != nil {
		return nil, fmt.Errorf("clearing %s: %w", calibWT, err)
	}
	_ = exec.CommandContext(ctx, "git", "-C", clone, "worktree", "prune").Run()
	addCmd := exec.CommandContext(ctx, "git", "-C", clone, "worktree", "add", "--detach", calibWT, commit)
	return addCmd.CombinedOutput()
}

func CalibrateSubject(ctx context.Context, task Task, wt Worktree, root string) (Calibration, error) {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	cacheDir := filepath.Join(root, "workspace", "scenariobench", "calibration")
	key := calibrationKey(task)
	cachePath := filepath.Join(cacheDir, key+".json")
	// Two arms of one task calibrate the same (subject, commit) — under
	// -parallel-arms concurrently, in one process. Serialize per key so the
	// second caller finds the cache the first one wrote instead of racing it
	// for the calibration worktree (reuse-pilot-2 lost a cell to that race).
	mu, _ := calibrationLocks.LoadOrStore(key, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()

	if raw, err := os.ReadFile(cachePath); err == nil {
		var cached Calibration
		if err := json.Unmarshal(raw, &cached); err != nil {
			return Calibration{}, fmt.Errorf("calibrating subject: parsing cached %s: %w", cachePath, err)
		}
		return cached, nil
	} else if !os.IsNotExist(err) {
		return Calibration{}, fmt.Errorf("calibrating subject: reading cache %s: %w", cachePath, err)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return Calibration{}, fmt.Errorf("calibrating subject: creating %s: %w", cacheDir, err)
	}

	clone := wt.SubjectClone
	if clone == "" {
		clone = wt.Path
	}
	calibWT := filepath.Join(cacheDir, "wt-"+key)
	if out, err := prepareCalibrationWorktree(ctx, clone, calibWT, task.Subject.BaseCommit); err != nil {
		return Calibration{}, fmt.Errorf("calibrating subject: git worktree add: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	defer func() {
		_ = exec.Command("git", "-C", clone, "worktree", "remove", "--force", calibWT).Run()
		_ = os.RemoveAll(calibWT)
	}()

	calibCellDir := filepath.Join(cacheDir, "cell-"+key)
	if err := os.MkdirAll(calibCellDir, 0o755); err != nil {
		return Calibration{}, fmt.Errorf("calibrating subject: creating %s: %w", calibCellDir, err)
	}
	defer os.RemoveAll(calibCellDir)
	// A REAL git worktree (prepareCalibrationWorktree: `git worktree add`
	// FROM clone), sharing clone's own object store — unlike a
	// checkoutCommitIntoWorktree'd cell worktree, task.Subject.BaseCommit
	// IS a valid, resolvable ref here, so LocalBase is simply that commit
	// itself (fix-first review: ApplyTestPatch/resetTestPatchTargets now
	// read wt.LocalBase unconditionally).
	throwaway := Worktree{Path: calibWT, SubjectClone: clone, CellDir: calibCellDir, LocalBase: task.Subject.BaseCommit}

	var result Calibration
	if err := ApplyTestPatch(ctx, throwaway, task); err != nil {
		result.NotMeasurable = fmt.Sprintf("test patch did not apply during calibration: %v", err)
		if werr := writeCalibrationCache(cachePath, result); werr != nil {
			return Calibration{}, werr
		}
		return result, nil
	}

	names := dedupSorted(append(append([]string(nil), task.Oracle.FailToPass...), task.Oracle.PassToPass...))
	passed := map[string]bool{}
	if len(names) > 0 {
		p, _, _, err := runOracleTests(ctx, throwaway, task, names)
		if err != nil {
			return Calibration{}, fmt.Errorf("calibrating subject: running tests: %w", err)
		}
		passed = p
	}
	for _, n := range task.Oracle.FailToPass {
		if !passed[n] {
			result.FailToPassFailedAtBase = append(result.FailToPassFailedAtBase, n)
		}
	}
	for _, n := range task.Oracle.PassToPass {
		if passed[n] {
			result.PassToPassPassedAtBase = append(result.PassToPassPassedAtBase, n)
		}
	}
	result.FailToPassFailedAtBase = dedupSorted(result.FailToPassFailedAtBase)
	result.PassToPassPassedAtBase = dedupSorted(result.PassToPassPassedAtBase)

	if err := writeCalibrationCache(cachePath, result); err != nil {
		return Calibration{}, err
	}
	return result, nil
}

func writeCalibrationCache(path string, c Calibration) error {
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("calibrating subject: marshaling: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("calibrating subject: writing %s: %w", path, err)
	}
	return nil
}

func dedupSorted(names []string) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// runOracleTests runs the subject's test command once (-v appended for
// per-test parsing, plus -coverprofile when asked), over the regex-OR of
// names, in wt.Path.
func runOracleTests(ctx context.Context, wt Worktree, task Task, names []string) (passed map[string]bool, outputRef, coverRef string, err error) {
	tmpl := task.Subject.TestCommand
	// Test names are DATA, never shell: upstream subtest names carry
	// apostrophes, commas and regex metacharacters ("TestX/ghe.com's_…"
	// broke `sh -c` on the first public dry cell, 2026-09-13). Each name
	// is regex-quoted per slash-separated element and anchored; the
	// default command is exec'd as argv with no shell at all, and a
	// subject's own template gets the pattern single-quote-escaped.
	testsArg := goTestRunPattern(names)
	var cmd *exec.Cmd
	var cmdStr string
	if tmpl == "" {
		args := []string{"test", "./...", "-run", testsArg, "-v"}
		if task.Oracle.Coverage {
			coverRef = filepath.Join(wt.CellDir, "coverage.out")
			args = append(args, "-coverprofile="+coverRef)
		}
		cmd = exec.CommandContext(ctx, "go", args...)
		cmdStr = "go " + strings.Join(args, " ")
	} else {
		cmdStr = strings.ReplaceAll(tmpl, "{{tests}}", shellSingleQuoteEscape(testsArg)) + " -v"
		if task.Oracle.Coverage {
			coverRef = filepath.Join(wt.CellDir, "coverage.out")
			cmdStr += " -coverprofile=" + coverRef
		}
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}
	cmd.Dir = wt.Path
	cmd.Env = SubjectEnv(wt.Path, oracleEnv(wt))
	out, runErr := cmd.CombinedOutput() // a failing test set is a normal, expected exit != 0

	outputRef = filepath.Join(wt.CellDir, "oracle-test-output-"+shortHash(cmdStr)[:8]+".txt")
	if werr := os.WriteFile(outputRef, out, 0o644); werr != nil {
		return nil, "", "", fmt.Errorf("writing test output %s: %w", outputRef, werr)
	}

	passed = map[string]bool{}
	for _, m := range goTestPassRE.FindAllStringSubmatch(string(out), -1) {
		passed[m[1]] = true
	}
	for _, m := range goTestFailRE.FindAllStringSubmatch(string(out), -1) {
		passed[m[1]] = false
	}
	// A test command that ran but produced no per-test lines at all (a
	// non-Go test runner, or a build failure before any test ran) is
	// distinguishable from "every named test passed": every requested name
	// with no PASS/FAIL line stays unset (false via the map's zero value),
	// so allPassed correctly reports not-OK rather than vacuously OK.
	_ = runErr
	return passed, outputRef, coverRef, nil
}

func runDetector(ctx context.Context, wt Worktree, task Task, d Detector) (DetectorResult, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", d.Command)
	cmd.Dir = wt.Path
	cmd.Env = SubjectEnv(wt.Path, oracleEnv(wt))
	out, err := cmd.CombinedOutput()
	exit := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exit = ee.ExitCode()
		} else {
			return DetectorResult{}, fmt.Errorf("running %q: %w", d.Command, err)
		}
	}
	logPath := filepath.Join(wt.CellDir, "detector-"+sanitizeName(d.Name)+".log")
	_ = os.WriteFile(logPath, out, 0o644)
	return DetectorResult{Name: d.Name, Exit: exit, Violated: exit != d.ExpectExit}, nil
}

func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

// goTestRunPattern builds the -run pattern for a set of test names: each
// slash-separated element regex-quoted (go test matches each element of
// the pattern against the corresponding subtest level), anchored so
// "TestRun" never also selects "TestRunCancel", joined by |.
func goTestRunPattern(names []string) string {
	parts := make([]string, 0, len(names))
	for _, n := range names {
		elems := strings.Split(n, "/")
		for i, e := range elems {
			elems[i] = regexp.QuoteMeta(e)
		}
		parts = append(parts, "^"+strings.Join(elems, "$/^")+"$")
	}
	return strings.Join(parts, "|")
}

// shellSingleQuoteEscape makes s safe inside a single-quoted sh word.
func shellSingleQuoteEscape(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}

// oracleEnv is what a detector or test command may rely on beyond the
// subject environment: SCENARIO_BASE_COMMIT, so a change-present check can
// diff against base whether or not the agent committed (`git diff` alone
// is empty after a commit, and a single-file diff misses a change the
// agent made elsewhere — guards-adr-pilot-1, 2026-09-13). Fix-first
// review: this is wt.LocalBase, NEVER task.Subject.BaseCommit directly —
// the latter is not even a valid ref inside wt (see Worktree.LocalBase's
// own doc comment); a detector command that ran `git diff
// $SCENARIO_BASE_COMMIT` before this fix always failed with "unknown
// revision" on a freshly prepared (never reused) worktree.
func oracleEnv(wt Worktree) map[string]string {
	return map[string]string{"SCENARIO_BASE_COMMIT": wt.LocalBase}
}
