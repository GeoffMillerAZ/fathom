// rescore.go — wave N18 defect 2: a run whose cells were LOST to a
// scorer defect (O65's SandboxEscapeScan false positive, this wave's own
// fix — answerkey.go) already spent the real tokens; the raw transcripts
// it wrote under workspace/artifacts/scenariobench/raw/<run-id>/ are
// still on disk. RescoreSessionsFromRaw re-parses those stored streams
// through the SAME session parser (ParseSessionStream) and the SAME
// post-session gate runOneSession applies to a session it just ran
// (AnswerKeyLeakScan, SandboxEscapeScan, BaselineRanFathomScan, the four
// instrument-truth checks) — never a looser bar than the live path's —
// without launching `claude` again and without touching the (shared,
// long since reset and reused) subject worktree at all.
package scenariobench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// CellRawBase is cellRawBase's own formula (sessions.go), exposed for a
// caller with no RunOptions to build (cmd/scenariobench's own rescore
// path): the raw transcript directory identifying one cell key (task,
// arm, trial) under one run, before RunSessions' own per-attempt
// "attempt-<k>" subdirectory.
func CellRawBase(rawDir, runID, taskID, armName string, trial int) string {
	base := rawDir
	if runID != "" {
		base = filepath.Join(rawDir, runID)
	}
	return filepath.Join(base, fmt.Sprintf("%s-%s-t%d", taskID, armName, orOne(trial)))
}

// sessionStreamFileRE matches ClaudeHarness's own raw stream file name
// (harness_claude.go: fmt.Sprintf("session-%d-%s.stream.jsonl", req.Index,
// safeRole(req.Role))) — never its "session-<n>-mcp.json" sibling or a
// ".stream.jsonl.stderr" companion.
var sessionStreamFileRE = regexp.MustCompile(`^session-(\d+)-(.+)\.stream\.jsonl$`)

// rawSessionFile is one raw stream file found directly inside an attempt
// directory, in the session index ClaudeHarness itself named it with.
type rawSessionFile struct {
	Path  string
	Index int
	// Role is ClaudeHarness's own safeRole(req.Role) spelling: "session"
	// stands for an originally-empty Role (safeRole's own fallback) —
	// indistinguishable, from the file name alone, from a role that was
	// genuinely spelled "session" (no task in this corpus ever uses that
	// spelling for a real role, so this ambiguity is accepted, not solved).
	Role string
}

// rawSessionFilesIn returns dir's own session-<n>-<role>.stream.jsonl
// files, in session-index order. A dir that does not exist is zero
// results, not an error — a caller (LatestCompleteAttempt) walks several
// candidate attempt dirs and must not fail merely because one is absent.
func rawSessionFilesIn(dir string) ([]rawSessionFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading raw attempt dir %s: %w", dir, err)
	}
	var out []rawSessionFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := sessionStreamFileRE.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		idx, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		out = append(out, rawSessionFile{Path: filepath.Join(dir, e.Name()), Index: idx, Role: m[2]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out, nil
}

// roleFromSafeRole reverses ClaudeHarness's own safeRole: "session" (the
// fallback spelling for an originally-empty Role) becomes "" again; every
// other spelling is returned verbatim.
func roleFromSafeRole(safe string) string {
	if safe == "session" {
		return ""
	}
	return safe
}

// IsCompleteSessionStream reports whether path's own raw transcript ends
// with a terminal `{"type":"result",...}` event — ParseSessionStream's
// own completeness signal (parsedStream.sawResult). A stream a killed
// driver or a crashed machine cut off mid-session never reaches one;
// rescoring it would silently score a HALF session as if it were whole,
// so LatestCompleteAttempt refuses to pick an attempt carrying one.
func IsCompleteSessionStream(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", path, err)
	}
	complete := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] != '{' {
			continue
		}
		var ev struct {
			Type string `json:"type"`
		}
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		if ev.Type == "result" {
			complete = true
		}
	}
	return complete, nil
}

// attemptDirRE matches allocateRawAttemptDir's own naming (sessions.go).
var attemptDirRE = regexp.MustCompile(`^attempt-(\d+)$`)

// LatestCompleteAttempt returns the highest-numbered "attempt-<k>"
// subdirectory of cellRawBase whose own raw session stream files all
// exist and are complete (IsCompleteSessionStream) — never a partial
// attempt a killed driver or a retried cell left behind, and never one
// with zero stream files at all. "", false, nil when cellRawBase does not
// exist, or no attempt under it qualifies.
func LatestCompleteAttempt(cellRawBase string) (string, bool, error) {
	entries, err := os.ReadDir(cellRawBase)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("reading %s: %w", cellRawBase, err)
	}
	var attempts []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if m := attemptDirRE.FindStringSubmatch(e.Name()); m != nil {
			if k, err := strconv.Atoi(m[1]); err == nil {
				attempts = append(attempts, k)
			}
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(attempts)))
	for _, k := range attempts {
		dir := filepath.Join(cellRawBase, fmt.Sprintf("attempt-%d", k))
		files, err := rawSessionFilesIn(dir)
		if err != nil {
			return "", false, err
		}
		if len(files) == 0 {
			continue
		}
		allComplete := true
		for _, f := range files {
			complete, err := IsCompleteSessionStream(f.Path)
			if err != nil {
				return "", false, err
			}
			if !complete {
				allComplete = false
				break
			}
		}
		if allComplete {
			return dir, true, nil
		}
	}
	return "", false, nil
}

// NeedsLiveWorktree reports whether task's oracle needs a LIVE worktree
// that a rescore-from-raw path (never touching the subject worktree at
// all — it may long since have been reset and reused by a later cell)
// cannot supply: applying a hidden test patch, running named
// fail_to_pass/pass_to_pass tests, a tree-level detector (runDetector,
// oracle.go), or refusal scoring (ScoreRefusal's own git diff against
// wt.LocalBase, refusal.go). See RunOracle's own doc comment for exactly
// which of its branches touch wt — every other shape (AnswerOnly,
// LocalizationOnly, or a task with no oracle checks at all) never reads
// wt for anything. A task where this is true is never guessed at: the
// caller must report it not rescorable instead of calling RunOracle with
// no real worktree.
func NeedsLiveWorktree(task Task) bool {
	return task.Gold.TestPatch != "" ||
		len(task.Oracle.FailToPass) > 0 ||
		len(task.Oracle.PassToPass) > 0 ||
		len(task.Oracle.Detectors) > 0 ||
		RefusalAware(task)
}

// HistoricalWorktreePath returns the git worktree path PrepareSubject
// would have given (task, arm) under root — subject.go's own formula
// (subjectsDir/cloneDir-worktrees/<base_commit>-<armContentHash>),
// computed with no I/O at all so a caller with no live Worktree
// (RescoreSessionsFromRaw's own safeDirs / path-relativization) can still
// name the cell's own directories exactly as the live run did, whether or
// not they still exist or have since been reset and reused by a later
// cell.
func HistoricalWorktreePath(root string, task Task, arm Arm) string {
	subjectsDir := HistoricalSubjectsDir(root)
	cloneDir := filepath.Join(subjectsDir, shortHash(task.Subject.RepoURL))
	worktreesDir := cloneDir + "-worktrees"
	return filepath.Join(worktreesDir, task.Subject.BaseCommit+"-"+armContentHash(arm))
}

// HistoricalSubjectsDir returns the shared clone-and-worktree pool
// directory PrepareSubject's own subjectsDir formula names under root —
// the same no-I/O guarantee as HistoricalWorktreePath, and (wave N19
// defect 1) an ADDITIONAL safe directory alongside a cell's own
// worktree/cellDir/rawBase: subjectsDir holds nothing but bare,
// sha256-hash-named clone and "<commit>-<armhash>"-named worktree
// directories shared by EVERY cell of this run — a bare listing of it (an
// agent orienting itself, `ls` of the pool root) names no task-specific
// content, unlike cd'ing or reading INTO one specific OTHER cell's own
// worktree, which remains unsafe because only THIS cell's own directories
// are ever in the list.
func HistoricalSubjectsDir(root string) string {
	return filepath.Join(root, "workspace", "scenariobench", "subjects")
}

// HistoricalCellDir returns the per-trial artifact directory runOneCell
// would have given (task, arm, trial) under root — PrepareSubject's own
// wt.CellDir formula (workspace/artifacts/scenariobench/worktrees/
// <task.ID>-<basename of HistoricalWorktreePath>) plus PerTrialCellDir's
// own "trial-<n>" suffix. Same no-I/O guarantee as HistoricalWorktreePath.
func HistoricalCellDir(root string, task Task, arm Arm, trial int) string {
	wtPath := HistoricalWorktreePath(root, task, arm)
	cellDir := filepath.Join(root, "workspace", "artifacts", "scenariobench", "worktrees", task.ID+"-"+filepath.Base(wtPath))
	return PerTrialCellDir(cellDir, "", trial)
}

// mcpServerNamesFromRawConfig reads the "mcpServers" object keys out of a
// session's own captured mcp.json (ClaudeHarness's own
// "session-<n>-mcp.json", the RENDERED config copy — see RunSession's own
// doc comment), sorted. nil, no error, for a missing or unparsable file:
// fail-open, exactly like primerInstrumentError's own missing-signal
// handling elsewhere in this package — a caller rescoring a cell whose
// mcp.json somehow went missing should not refuse the whole cell over it,
// only skip the instrument-truth check that needed it.
func mcpServerNamesFromRawConfig(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}
	names := make([]string, 0, len(cfg.MCPServers))
	for name := range cfg.MCPServers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// RescoreSessionsFromRaw reconstructs every session recorded under
// attemptDir (rawSessionFilesIn's own file shape) into []SessionResult,
// in session-index order, applying the EXACT post-session gate
// runOneSession (sessions.go) applies to a session it just ran:
//
//   - each session's own wantServers (instrumentTruthError) comes from its
//     sibling "session-<n>-mcp.json" — the RENDERED config ClaudeHarness
//     itself captured for that exact session, never a re-derivation of
//     arm rendering;
//   - AnswerKeyLeakScan against excludePaths;
//   - SandboxEscapeScan against safeDirs — wave N18's own fix, so a
//     rescored cell benefits from the corrected rule exactly like a live
//     one now does;
//   - BaselineRanFathomScan for an unwired arm;
//   - hooksInstrumentError / primerInstrumentError / postToolUseInstrumentError.
//
// worktree relativizes FilesRead/FilesEdited exactly as ClaudeHarness's
// own RunSession does (relativizeSessionPaths) — a path name only, never
// read from disk. A cell rescored this way is held to EXACTLY the same
// bar a live session was, never a looser one.
func RescoreSessionsFromRaw(attemptDir string, excludePaths []string, wired bool, worktree string, safeDirs []string) ([]SessionResult, error) {
	files, err := rawSessionFilesIn(attemptDir)
	if err != nil {
		return nil, fmt.Errorf("rescoring sessions from raw: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("rescoring sessions from raw: no session stream files under %s", attemptDir)
	}
	out := make([]SessionResult, 0, len(files))
	for _, f := range files {
		res, err := ParseSessionStream(f.Path)
		if err != nil {
			return nil, fmt.Errorf("rescoring sessions from raw: %w", err)
		}
		res.Index = f.Index
		res.Role = roleFromSafeRole(f.Role)
		res.RawPath = f.Path
		res.FilesRead = relativizeSessionPaths(worktree, res.FilesRead)
		res.FilesEdited = relativizeSessionPaths(worktree, res.FilesEdited)

		if res.Error == "" {
			mcpPath := filepath.Join(attemptDir, fmt.Sprintf("session-%d-mcp.json", f.Index))
			wantServers := mcpServerNamesFromRawConfig(mcpPath)
			msg := instrumentTruthError(wantServers, res)
			if msg == "" {
				msg = hooksInstrumentError(wired, res)
			}
			if msg == "" {
				msg = primerInstrumentError(wired, res)
			}
			if msg == "" {
				msg = postToolUseInstrumentError(wired, res)
			}
			if msg != "" {
				res.Error = msg
				res.InstrumentError = msg
			}
		}
		if AnswerKeyLeakScan(res, excludePaths) {
			res.AnswerKeyLeak = true
		}
		if SandboxEscapeScan(res, safeDirs...) {
			res.SandboxEscape = true
		}
		if !wired && BaselineRanFathomScan(res) {
			res.BaselineRanFathom = true
		}
		out = append(out, res)
	}
	return out, nil
}
