// PrepareSubject (SB-12) turns a Task's pinned subject into a git worktree
// ready for sessions: a shared clone under
// workspace/scenariobench/subjects/<sha256(repo_url)>, one cell worktree
// per (base_commit, arm) checked out from that clone at base_commit and
// given its own single-commit history (O55 item 1 — see
// checkoutCommitIntoWorktree: a cell worktree must have NO future, so
// `git log --all` inside it can
// never resolve a commit that only merged after base_commit), the subject's
// and the arm's setup commands, and — for a wired arm — a scratch daemon
// that ingests ONLY that worktree (never the fleet store) so `fathom init
// --harness claude-code --json` can wire it for real.
package scenariobench

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// CompileCost is what PrepareSubject spent compiling a semantic layer for a
// wired, CompileLayers arm — recorded on the cell so a run that asked for
// compiled obligations shows what that cost, separately from the session
// spend RunSessions measures.
type CompileCost struct {
	Cached    bool          `json:"cached,omitempty"` // served from a store an earlier cell compiled
	TokensIn  int64         `json:"tokens_in"`
	TokensOut int64         `json:"tokens_out"`
	CostUSD   float64       `json:"cost_usd"`
	Duration  time.Duration `json:"duration_ns"`
}

// Worktree is what PrepareSubject prepared for one (task, arm) cell.
type Worktree struct {
	// Path is the git worktree's absolute path — SessionRequest.Worktree.
	Path string
	// SubjectClone is the shared clone this worktree was added from.
	SubjectClone string
	// CellDir is where this cell's own working artifacts (oracle output,
	// raw session transcripts) belong — distinct from the content-addressed
	// cell store (cell.go), whose key is not known until scoring time.
	CellDir string
	// Arm is what RenderArm wrote into this worktree.
	Arm ArmFiles
	// DaemonURL is "" for an unwired arm; the scratch daemon's base URL
	// otherwise (e.g. "http://127.0.0.1:53219").
	DaemonURL string
	// StorePath is the scratch daemon's sqlite store (wired only).
	StorePath string
	// Service is the id wireScratchDaemon ingested this worktree under
	// ("" for an unwired arm) — task.Subject.Service, or wt.Path's own
	// basename when the task named none, the SAME resolution `fathom
	// init`/`ingest` were called with. workerDiscoveryBrief (sessions.go)
	// passes it to `fathom discoveries -service` explicitly, so that
	// subprocess never falls back to auto-resolving a service against
	// whatever daemon FATHOM_HTTP/localhost:7575 happens to be — the
	// operator's own, in the environment every subprocess here inherits
	// (SubjectEnv).
	Service string
	// CompileCost is set only when arm.CompileLayers asked for it.
	CompileCost *CompileCost

	daemon *exec.Cmd // unexported: StopWorktree's handle on the scratch daemon, if one was started
	// fathomBin is the built fathom binary wireScratchDaemon used for the
	// first ingest — unexported: applyPostWireSetup's re-ingest (O45 item
	// 4) reuses it rather than rebuilding, and it is "" for an unwired
	// cell.
	fathomBin string
}

// PrepareSubject clones task.Subject.RepoURL once under
// <root>/workspace/scenariobench/subjects/<sha256(repo_url)> (reusing the
// clone across every cell of every task that shares a subject), adds a
// worktree at base_commit, runs the subject's and then the arm's Setup
// commands in it, renders the arm (RenderArm), and — for a wired arm —
// starts a scratch daemon over ONLY this worktree, runs `fathom init
// --harness claude-code --json` against it, and points the arm's MCP
// config at the daemon (alwaysLoad: true) before compiling layers when
// asked. root is this repo's root (where `go build ./cmd/fathom` runs
// from); it is never the subject.
func PrepareSubject(ctx context.Context, task Task, arm Arm, root string) (Worktree, error) {
	// Every path below is handed to subprocesses that run in OTHER
	// directories (git worktree add runs in the clone, ingest in the
	// worktree): a relative root silently lands the worktree inside the
	// clone and the next chdir fails (dry cell, 2026-09-13).
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	subjectsDir := filepath.Join(root, "workspace", "scenariobench", "subjects")
	cloneDir := filepath.Join(subjectsDir, shortHash(task.Subject.RepoURL))
	if err := ensureClone(ctx, task.Subject.RepoURL, cloneDir); err != nil {
		return Worktree{}, err
	}

	worktreesDir := cloneDir + "-worktrees"
	if err := os.MkdirAll(worktreesDir, 0o755); err != nil {
		return Worktree{}, fmt.Errorf("preparing subject: creating %s: %w", worktreesDir, err)
	}
	// One worktree per (subject, base_commit, arm), REUSED across the
	// tasks and trials that share it: a compiled obligations layer costs
	// ~7 minutes and ~$0.12 per store (dry cell, 2026-09-13), and every
	// task of a corpus on the same commit needs the same one. Between
	// cells the tree is reset to base_commit and cleaned; the scratch
	// store beside it persists and is re-ingested hash-gated.
	wtPath := filepath.Join(worktreesDir, task.Subject.BaseCommit+"-"+armContentHash(arm))

	if err := ensureCommit(ctx, cloneDir, task.Subject.BaseCommit); err != nil {
		return Worktree{}, err
	}
	// git's worktree registration files are not safe under concurrent
	// `worktree add` in one clone (eight parallel cells read a half-written
	// commondir, "Result too large", 2026-09-14): every worktree operation
	// on a clone runs under that clone's lock, like the clone itself.
	if err := func() error {
		unlock := lockClone(cloneDir)
		defer unlock()
		if _, err := os.Stat(filepath.Join(wtPath, ".git")); err == nil {
			// O52: a scratch daemon that outlived its cell (a driver killed
			// mid-run, a teardown that never ran) keeps running git in this
			// shared worktree and holds index.lock while the next cell
			// resets it. Reap it by pidfile first, then clear a lock no
			// live git process owns.
			reapStaleScratchDaemon(wtPath)
			removeStaleIndexLock(ctx, wtPath)
			// O57 finding 1: unlike a stray scratch daemon (fathom's own
			// process, safe to kill on sight), a live SESSION pidfile here
			// names an agent process that may still be actively editing —
			// refuse the reset (LOST) rather than race or destroy it.
			if reason := RefuseIfLiveSessionInWorktree(wtPath); reason != "" {
				return fmt.Errorf("preparing subject: refusing to reset the shared worktree: %s", reason)
			}
			// wtPath is its own repository (checkoutCommitIntoWorktree) whose
			// history starts at its own root commit — that commit's sha is
			// NOT task.Subject.BaseCommit (it was re-committed by `git
			// commit`, never `git reset` to the original sha, which this
			// repo's object store does not even contain). A session may have
			// added commits on top since the last cell; the root commit is
			// always the one to reset back to, regardless of how many.
			root, err := runIn(ctx, wtPath, "git", "rev-list", "--max-parents=0", "HEAD")
			if err != nil {
				return fmt.Errorf("preparing subject: finding the worktree's base commit: %w (%s)", err, root)
			}
			if out, err := runIn(ctx, wtPath, "git", "reset", "--hard", strings.TrimSpace(root)); err != nil {
				return fmt.Errorf("preparing subject: resetting the shared worktree: %w (%s)", err, out)
			}
			if out, err := runIn(ctx, wtPath, "git", "clean", "-fdx"); err != nil {
				return fmt.Errorf("preparing subject: cleaning the shared worktree: %w (%s)", err, out)
			}
			return nil
		}
		if err := os.MkdirAll(wtPath, 0o755); err != nil {
			return fmt.Errorf("preparing subject: creating %s: %w", wtPath, err)
		}
		if err := checkoutCommitIntoWorktree(ctx, cloneDir, task.Subject.BaseCommit, wtPath); err != nil {
			_ = os.RemoveAll(wtPath)
			return err
		}
		return nil
	}(); err != nil {
		return Worktree{}, err
	}

	wt := Worktree{Path: wtPath, SubjectClone: cloneDir}
	wt.CellDir = filepath.Join(root, "workspace", "artifacts", "scenariobench", "worktrees", task.ID+"-"+filepath.Base(wtPath))
	if err := os.MkdirAll(wt.CellDir, 0o755); err != nil {
		return Worktree{}, fmt.Errorf("preparing subject: creating %s: %w", wt.CellDir, err)
	}

	// O53 defect 3: after checkout, before ingest, for EVERY arm — a task
	// copied from the home corpus must never carry that corpus (or its
	// scenariobench/docs siblings) inside its own worktree, where an
	// agent's Read/Grep/Bash tools can surface it as an answer key.
	if err := removeExcludedPaths(wtPath, task.Subject.ExcludePaths); err != nil {
		return Worktree{}, fmt.Errorf("preparing subject: %w", err)
	}

	if err := runSetup(ctx, wtPath, task.Subject.Setup, filepath.Join(wt.CellDir, "subject-setup.log")); err != nil {
		return Worktree{}, fmt.Errorf("preparing subject: subject setup: %w", err)
	}
	if err := runSetup(ctx, wtPath, arm.Setup, filepath.Join(wt.CellDir, "arm-setup.log")); err != nil {
		return Worktree{}, fmt.Errorf("preparing subject: arm setup: %w", err)
	}

	renderedArm := arm
	if arm.Wired {
		if err := wireScratchDaemon(ctx, &wt, task, &renderedArm, root); err != nil {
			return Worktree{}, err
		}
	}

	files, err := RenderArm(renderedArm, wtPath)
	if err != nil {
		_ = StopWorktree(&wt) // never leave a scratch daemon behind (two did, 2026-09-13)
		return Worktree{}, fmt.Errorf("preparing subject: %w", err)
	}
	wt.Arm = files

	if arm.Wired && arm.CompileLayers {
		cost, err := compileLayers(ctx, root, task, wt)
		if err != nil {
			_ = StopWorktree(&wt)
			return Worktree{}, fmt.Errorf("preparing subject: compiling layers: %w", err)
		}
		wt.CompileCost = cost
	}

	// O45 item 4: drift belongs HERE, after the first ingest/compile above
	// — never in Subject.Setup, which ran before any of it and would
	// already be visible to both. A wired cell also re-ingests afterward
	// so its symbol index reflects the drift; a compiled obligations layer
	// is deliberately NOT recompiled, so a staleness task's claim stays
	// pinned to the pre-drift citation.
	var runIngest func(context.Context) error
	if arm.Wired && wt.fathomBin != "" {
		runIngest = func(ctx context.Context) error {
			_, err := runIn(ctx, wt.Path, wt.fathomBin, "ingest", "-service", wt.Service, "-path", wt.Path, "-store", wt.StorePath)
			return err
		}
	}
	if err := applyPostWireSetup(ctx, wt, task, runIngest); err != nil {
		_ = StopWorktree(&wt)
		return Worktree{}, fmt.Errorf("preparing subject: %w", err)
	}

	// O45 item 1: a task whose prompt assumes a compiled obligations layer
	// already serves rules must never silently run against one that
	// doesn't (compile_layers unset on the arm, or a compile that
	// produced nothing reachable for this repo) — probe it and refuse the
	// cell rather than measure an empty layer.
	if arm.Wired && requiresCompiledLayer(task) {
		probeFile := task.ProbeFile
		if probeFile == "" {
			probeFile = DefaultProbeFile
		}
		ok, err := probeCompiledLayerServesRules(ctx, wt.DaemonURL, wt.Service, probeFile)
		if err != nil {
			_ = StopWorktree(&wt)
			return Worktree{}, fmt.Errorf("preparing subject: probing compiled layer: %w", err)
		}
		if !ok {
			_ = StopWorktree(&wt)
			return Worktree{}, fmt.Errorf("compiled layer serves no rules for %s", probeFile)
		}
	}

	return wt, nil
}

// DefaultProbeFile is the probe target a layer-requiring task uses when it
// names no ProbeFile of its own (O45 item 1): this repo's own convention
// (root CLAUDE.md's "extend internal/core additively") reaches every file
// under internal/core, so a freshly compiled obligations layer almost
// always serves at least one rule for it.
const DefaultProbeFile = "internal/core/model.go"

// layerRequiredCategories are the lookup-corpus category strings (SB-32,
// lookups-vs.json) whose prompts assume a wired arm's compiled obligations
// layer already serves rules ("Per fathom's compiled obligations layer
// …", or a staleness verdict against one) — requiresCompiledLayer's
// default when a task sets no RequiresLayer of its own (O45 item 1). None
// of these collide with the change-bench's own Category consts
// (CategoryBugFix etc.), which use different literal strings.
var layerRequiredCategories = map[Category]bool{
	"obligations":      true,
	"staleness":        true,
	"claim_provenance": true,
	"verify":           true,
}

// requiresCompiledLayer reports whether task assumes a wired arm's
// compiled obligations layer already serves rules — task.RequiresLayer
// verbatim when set, else layerRequiredCategories' default for its
// Category.
func requiresCompiledLayer(task Task) bool {
	if task.RequiresLayer != nil {
		return *task.RequiresLayer
	}
	return layerRequiredCategories[task.Category]
}

// probeCompiledLayerServesRules calls daemonURL's own "obligations" MCP
// tool for probeFile and reports whether it served at least one rule
// (O45 item 1's instrument-truth probe): a wired cell whose compiled
// layer serves zero rules for a file the corpus expects it to govern
// means every layer-dependent task in the cell would measure an empty
// layer, not fathom's obligations feature — PrepareSubject refuses the
// cell instead.
func probeCompiledLayerServesRules(ctx context.Context, daemonURL, service, probeFile string) (bool, error) {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "obligations",
			"arguments": map[string]any{
				"service":          service,
				"file":             probeFile,
				"include_snippets": false,
			},
		},
	})
	if err != nil {
		return false, fmt.Errorf("encoding obligations probe request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, daemonURL+"/rpc/agent", bytes.NewReader(reqBody))
	if err != nil {
		return false, fmt.Errorf("building obligations probe request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("calling obligations for probe file %s: %w", probeFile, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("reading obligations probe response: %w", err)
	}
	return obligationsProbeServesRules(raw)
}

// obligationsProbeServesRules reads a tools/call response for `obligations`
// and reports whether it carried at least one rule. A real MCP response
// puts the tool's JSON inside result.content[].text (a text block), not
// under result.rules — the first probe read only the latter, so against a
// real daemon it always saw zero rules and voided every layer-required
// wired cell (O51, vs-pilot-3 2026-09-14). Both shapes are accepted.
func obligationsProbeServesRules(raw []byte) (bool, error) {
	var envelope struct {
		Result struct {
			Rules   []json.RawMessage `json:"rules"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return false, fmt.Errorf("parsing obligations probe response: %w (%s)", err, string(raw))
	}
	if envelope.Error != nil {
		return false, fmt.Errorf("obligations probe: %s", envelope.Error.Message)
	}
	if len(envelope.Result.Rules) > 0 {
		return true, nil
	}
	for _, c := range envelope.Result.Content {
		if c.Type != "" && c.Type != "text" || c.Text == "" {
			continue
		}
		var body struct {
			Rules []json.RawMessage `json:"rules"`
		}
		if err := json.Unmarshal([]byte(c.Text), &body); err != nil {
			return false, fmt.Errorf("parsing obligations probe text block: %w (%s)", err, c.Text)
		}
		if len(body.Rules) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// applyPostWireSetup runs task.Subject.PostWireSetup in wt.Path — AFTER
// the daemon's first ingest/compile has already happened, the correct
// place for staleness/freshness drift (O45 item 4) — then, when runIngest
// is non-nil (a wired cell), re-ingests so the symbol index reflects it.
// runIngest is a seam (the real one shells out to the built binary's own
// "ingest" subcommand) so this ordering is testable without a real daemon
// or binary. A task with no PostWireSetup is a no-op, regardless of
// runIngest.
func applyPostWireSetup(ctx context.Context, wt Worktree, task Task, runIngest func(context.Context) error) error {
	if len(task.Subject.PostWireSetup) == 0 {
		return nil
	}
	if err := runSetup(ctx, wt.Path, task.Subject.PostWireSetup, filepath.Join(wt.CellDir, "post-wire-setup.log")); err != nil {
		return fmt.Errorf("post-wire setup: %w", err)
	}
	if runIngest == nil {
		return nil
	}
	if err := runIngest(ctx); err != nil {
		return fmt.Errorf("post-wire setup: re-ingesting: %w", err)
	}
	return nil
}

// StopWorktree stops a wired cell's scratch daemon (SB-12: "stop it after
// the last session"). A control/unwired worktree has none; calling it on
// one is a no-op.
func StopWorktree(wt *Worktree) error {
	if wt == nil || wt.daemon == nil || wt.daemon.Process == nil {
		return nil
	}
	pid := wt.daemon.Process.Pid
	_ = syscall.Kill(-pid, syscall.SIGKILL) // the process group (O52)
	if err := wt.daemon.Process.Kill(); err != nil && !strings.Contains(err.Error(), "process already finished") {
		return fmt.Errorf("stopping scratch daemon: %w", err)
	}
	_ = wt.daemon.Wait()
	unregisterLiveProcessGroup(pid) // O57 finding 1
	wt.daemon = nil
	_ = os.Remove(scratchDaemonPIDFile(wt.Path))
	return nil
}

// scratchDaemonPIDFile is the pidfile a wired cell's scratch daemon leaves
// beside the SHARED worktree it runs in — shared across every cell on the
// same (subject, commit, arm), which is exactly the scope a leftover
// daemon damages.
func scratchDaemonPIDFile(wtPath string) string {
	return strings.TrimSuffix(wtPath, "/") + ".scratch-daemon.pid"
}

// scratchDaemonCommandMarker is the substring a reaped process's command
// line must carry; a pid recycled by an unrelated process is left alone.
// Tests point it at their own fake daemon.
var scratchDaemonCommandMarker = "serve-http"

// reapStaleScratchDaemon kills a scratch daemon a previous cell left
// running in wtPath (O52) and removes its pidfile. A missing pidfile, a
// dead pid, or a recycled pid are all no-ops.
func reapStaleScratchDaemon(wtPath string) {
	pidfile := scratchDaemonPIDFile(wtPath)
	data, err := os.ReadFile(pidfile)
	if err != nil {
		return
	}
	defer os.Remove(pidfile)
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 1 {
		return
	}
	out, err := exec.Command("ps", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil || !strings.Contains(string(out), scratchDaemonCommandMarker) {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
	for i := 0; i < 50; i++ {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// removeStaleIndexLock clears wtPath's git index.lock when it is older
// than ten seconds — a lock no live git command is holding (a reaped
// daemon's, or a driver killed mid-reset). A young lock is left alone.
func removeStaleIndexLock(ctx context.Context, wtPath string) {
	gitDir, err := runIn(ctx, wtPath, "git", "rev-parse", "--absolute-git-dir")
	if err != nil {
		return
	}
	lock := filepath.Join(strings.TrimSpace(gitDir), "index.lock")
	if st, err := os.Stat(lock); err == nil && time.Since(st.ModTime()) > 10*time.Second {
		_ = os.Remove(lock)
	}
}

// ─── clone / worktree plumbing ──────────────────────────────────────────

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// armContentHash is a short, stable identifier for one Arm value, used
// only to keep a cell's worktree directory name readable and distinct from
// another arm's; it is not the cell key (cell.go's ScenarioCellKey is).
func armContentHash(arm Arm) string {
	raw, _ := json.Marshal(arm)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:6])
}

// cloneLocks serializes ensureClone per cloneDir, process-wide: under
// -parallel-arms, every arm/trial of one task shares the SAME subject
// clone (cloneDir is keyed only on repo_url), so two cells racing
// PrepareSubject at once could both see cloneDir absent and both run `git
// clone` into it — one `git clone` losing to "destination path already
// exists and is not an empty directory", the other landing a clone with
// the loser's partial state still in it. A lazily-created *sync.Mutex per
// distinct cloneDir (never one global lock across unrelated subjects)
// makes the check-then-clone atomic without serializing clones of
// different repos.
var cloneLocks sync.Map // map[string]*sync.Mutex

func lockClone(cloneDir string) func() {
	v, _ := cloneLocks.LoadOrStore(cloneDir, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func ensureClone(ctx context.Context, repoURL, cloneDir string) error {
	unlock := lockClone(cloneDir)
	defer unlock()
	if _, err := os.Stat(filepath.Join(cloneDir, ".git")); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("preparing subject: statting %s: %w", cloneDir, err)
	}
	if err := os.MkdirAll(filepath.Dir(cloneDir), 0o755); err != nil {
		return fmt.Errorf("preparing subject: creating %s: %w", filepath.Dir(cloneDir), err)
	}
	if out, err := runIn(ctx, "", "git", "clone", "--no-single-branch", repoURL, cloneDir); err != nil {
		return fmt.Errorf("preparing subject: git clone %s: %w (%s)", repoURL, err, out)
	}
	return nil
}

// checkoutCommitIntoWorktree makes wtPath (an existing, empty directory)
// its own git repository containing exactly base's tree, in exactly one
// commit with no parents — O55 item 1: a full clone's worktree carries the
// WHOLE history, including any merge that lands after base (a control
// planner ran `git log --all | grep -i autolink` against a shared-clone
// worktree and found the very fix PR #10124 the task was measuring), so a
// cell worktree built off one can leak the answer.
//
// Its own source of base's tree is checkoutCommitTree (read-tree +
// checkout-index against a TEMPORARY index), never `git archive`
// (fix-first review, 2026-09-14): archive
// honors .gitattributes' export-ignore, silently DROPPING any file a
// subject marks that way from the tarball — export-ignore exists
// specifically to shape distribution archives, and a real checkout
// (`git worktree add`, `checkout-index`) never consults it, so the first
// version of this fix (archive-based) could silently lose files a normal
// `git worktree add` would have kept. `git init` + `add -A` + `commit`
// then gives wtPath a single root commit, authored "bench
// <bench@example.invalid>" so it is never mistaken for a real contributor.
// cloneDir is read here only as the checkout source — the worktree shares
// none of its history, refs, or object store afterward.
func checkoutCommitIntoWorktree(ctx context.Context, cloneDir, base, wtPath string) error {
	if err := checkoutCommitTree(ctx, cloneDir, base, wtPath); err != nil {
		return err
	}
	if out, err := runIn(ctx, wtPath, "git", "init", "-q", "-b", "bench"); err != nil {
		return fmt.Errorf("preparing subject: git init: %w (%s)", err, out)
	}
	if out, err := runIn(ctx, wtPath, "git", "add", "-A"); err != nil {
		return fmt.Errorf("preparing subject: git add: %w (%s)", err, out)
	}
	commitEnv := map[string]string{
		"GIT_AUTHOR_NAME": "bench", "GIT_AUTHOR_EMAIL": "bench@example.invalid",
		"GIT_COMMITTER_NAME": "bench", "GIT_COMMITTER_EMAIL": "bench@example.invalid",
	}
	if out, err := runInEnv(ctx, wtPath, commitEnv, "git", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "base "+base); err != nil {
		return fmt.Errorf("preparing subject: git commit: %w (%s)", err, out)
	}
	return nil
}

// checkoutCommitTree populates wtPath (an existing, empty directory) with
// base's FULL tree read from cloneDir, via `git read-tree` + `git
// checkout-index` against a TEMPORARY index file (GIT_INDEX_FILE) —
// never cloneDir's own main index, so this never interferes with anything
// else the shared clone's index is used for (concurrent cells already
// serialize on lockClone(cloneDir), but there is no reason to touch
// persistent state here at all when a throwaway index does the same job).
// Unlike `git archive`, checkout-index never consults .gitattributes'
// export-ignore — it is a real checkout, the same shape `git worktree add`
// would have produced.
func checkoutCommitTree(ctx context.Context, cloneDir, base, wtPath string) error {
	gitDir := filepath.Join(cloneDir, ".git")
	tmpIndex, err := os.CreateTemp("", "fathom-scenariobench-index-")
	if err != nil {
		return fmt.Errorf("preparing subject: creating a temporary index: %w", err)
	}
	tmpIndexPath := tmpIndex.Name()
	if err := tmpIndex.Close(); err != nil {
		return fmt.Errorf("preparing subject: closing the temporary index: %w", err)
	}
	defer os.Remove(tmpIndexPath)
	extra := map[string]string{"GIT_INDEX_FILE": tmpIndexPath}

	if out, err := runInEnv(ctx, cloneDir, extra, "git", "--git-dir="+gitDir, "read-tree", base); err != nil {
		return fmt.Errorf("preparing subject: reading %s's tree: %w (%s)", base, err, out)
	}
	if out, err := runInEnv(ctx, cloneDir, extra, "git", "--git-dir="+gitDir, "--work-tree="+wtPath, "checkout-index", "-a", "-f"); err != nil {
		return fmt.Errorf("preparing subject: checking out %s into %s: %w (%s)", base, wtPath, err, out)
	}
	return nil
}

func ensureCommit(ctx context.Context, cloneDir, commit string) error {
	if _, err := runIn(ctx, cloneDir, "git", "cat-file", "-e", commit+"^{commit}"); err == nil {
		return nil
	}
	// Best-effort: the commit may have landed after the clone. A local
	// (file://, plain-path) fixture with no remote still fails this
	// harmlessly, and ensureCommit's caller surfaces the ORIGINAL missing
	// commit as the real error below.
	_, _ = runIn(ctx, cloneDir, "git", "fetch", "--all", "--quiet")
	if _, err := runIn(ctx, cloneDir, "git", "cat-file", "-e", commit+"^{commit}"); err != nil {
		return fmt.Errorf("preparing subject: base_commit %s not found in %s after fetch: %w", commit, cloneDir, err)
	}
	return nil
}

func runSetup(ctx context.Context, dir string, cmds []string, logPath string) error {
	if len(cmds) == 0 {
		return nil
	}
	var log strings.Builder
	for _, c := range cmds {
		log.WriteString("$ " + c + "\n")
		out, err := runIn(ctx, dir, "sh", "-c", c)
		log.WriteString(out)
		log.WriteString("\n")
		if err != nil {
			_ = os.WriteFile(logPath, []byte(log.String()), 0o644)
			return fmt.Errorf("running %q: %w", c, err)
		}
	}
	return os.WriteFile(logPath, []byte(log.String()), 0o644)
}

func runIn(ctx context.Context, dir string, name string, args ...string) (string, error) {
	return runInEnv(ctx, dir, nil, name, args...)
}

// runInEnv is runIn plus extra environment variables (SubjectEnv's own
// extra map, which wins on a key collision with the inherited operator
// environment) — workerDiscoveryBrief (sessions.go) uses this to point a
// wired cell's own `fathom discoveries` subprocess at the CELL's threads
// file and daemon, never the operator's real ones that a bare gitEnv()
// inheritance would otherwise silently expose it to.
func runInEnv(ctx context.Context, dir string, extra map[string]string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = SubjectEnv(dir, extra)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ─── scratch daemon (wired arms only) ───────────────────────────────────

// wireScratchDaemon builds the fathom binary from root, ingests wt.Path
// (never the fleet store) into a fresh sqlite store under
// workspace/scenariobench/stores/<cell>/, serves it on a free loopback
// port, runs `fathom init --harness claude-code --json` against it, and
// points renderedArm's MCP entry at the daemon (alwaysLoad: true) so
// RenderArm's later write carries the real URL.
func wireScratchDaemon(ctx context.Context, wt *Worktree, task Task, renderedArm *Arm, root string) error {
	binDir := wt.CellDir
	bin := filepath.Join(binDir, "fathom")
	if out, err := buildFathomBinary(ctx, root, bin); err != nil {
		return fmt.Errorf("wiring scratch daemon: %w (%s)", err, out)
	}
	wt.fathomBin = bin

	storeDir := filepath.Join(root, "workspace", "scenariobench", "stores", filepath.Base(wt.Path))
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		return fmt.Errorf("wiring scratch daemon: creating %s: %w", storeDir, err)
	}
	wt.StorePath = filepath.Join(storeDir, "store.sqlite")

	service := task.Subject.Service
	if service == "" {
		service = filepath.Base(wt.Path)
	}
	wt.Service = service
	if out, err := runIn(ctx, wt.Path, bin, "ingest", "-service", service, "-path", wt.Path, "-store", wt.StorePath); err != nil {
		return fmt.Errorf("wiring scratch daemon: ingest: %w (%s)", err, out)
	}

	port, err := freePort()
	if err != nil {
		return fmt.Errorf("wiring scratch daemon: finding a free port: %w", err)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	daemon := exec.Command(bin, "serve-http", "-store", wt.StorePath, "-addr", addr, "-auto-refresh=false", "-call-log", "")
	daemon.Dir = wt.Path
	// A scratch daemon never watches its binary for a deploy (it would exit
	// and nothing relaunches it), and it runs in its own process group so
	// StopWorktree can take down anything it spawned (O52).
	daemon.Env = append(os.Environ(), "FATHOM_SELF_RESTART=off")
	daemon.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	logFile, err := os.Create(filepath.Join(wt.CellDir, "scratch-daemon.log"))
	if err == nil {
		daemon.Stdout = logFile
		daemon.Stderr = logFile
	}
	if err := daemon.Start(); err != nil {
		return fmt.Errorf("wiring scratch daemon: starting serve-http: %w", err)
	}
	wt.daemon = daemon
	wt.DaemonURL = "http://" + addr
	_ = os.WriteFile(scratchDaemonPIDFile(wt.Path), []byte(strconv.Itoa(daemon.Process.Pid)+"\n"), 0o644)
	registerLiveProcessGroup(daemon.Process.Pid, "scratch-daemon:"+wt.Path) // O57 finding 1

	if err := waitDaemonReady(wt.DaemonURL, 30*time.Second); err != nil {
		_ = StopWorktree(wt)
		return fmt.Errorf("wiring scratch daemon: %w", err)
	}

	// --hooks-scope project: a wired cell's session runs under
	// `claude --restricted --settings <this worktree's .claude/settings.json>`
	// (docs/bench/scenarios.md "The clean room"), which never sees Claude
	// Code USER-scope hooks — so on any machine where fathom's hooks are
	// already wired globally (the operator's own dev machine, most often),
	// init's default `auto` scope would skip writing them at PROJECT scope
	// as "already wired at user scope", and the cell would run with none
	// of fathom's hooks at all. Forcing project scope here is what makes
	// assertWiredHooksPresent's promise (RenderArm never invents a hook
	// stand-in; `fathom init` already wrote the real one) true regardless
	// of the machine running the bench. Getting these hooks to actually
	// show up in the transcript this package reads — PostToolUse/Stop
	// included, not only SessionStart — additionally needs
	// buildClaudeArgs' `--include-hook-events` (clean room v3,
	// harness_claude.go's own comment); this init call and that flag are
	// independent fixes for independent gaps.
	out, err := runIn(ctx, wt.Path, bin, "init", "--path", wt.Path, "--service", service, "--url", wt.DaemonURL, "--harness", "claude-code", "--hooks-scope", "project", "--json")
	if err != nil {
		return fmt.Errorf("wiring scratch daemon: fathom init: %w (%s)", err, out)
	}
	if err := checkInitJSON(out); err != nil {
		return fmt.Errorf("wiring scratch daemon: fathom init: %w (%s)", err, out)
	}

	// renderedArm.MCP may alias the corpus's own Arm.MCP map (arm was
	// shallow-copied into *renderedArm by the caller); a fresh map here
	// means this write can never mutate a map another cell still holds.
	merged := make(map[string]any, len(renderedArm.MCP)+1)
	for k, v := range renderedArm.MCP {
		merged[k] = v
	}
	merged["fathom"] = map[string]any{
		"type":       "http",
		"url":        wt.DaemonURL + "/rpc/agent",
		"alwaysLoad": true,
	}
	renderedArm.MCP = merged
	return nil
}

// buildFathomBinary mirrors scripts/context-bench.sh's own scratch build:
// GOWORK=off (a linked worktree cannot always resolve ../agentic-kit) and
// CGO_ENABLED=1 (tree-sitter, ADR-013).
// buildFathomBinary puts the fathom binary a wired cell runs at out. It is
// built from a clean export of the repository's HEAD — never from the
// working tree — and cached per commit under workspace/tmp/, so (1) a cell
// runs exactly the commit its ledger row names, and (2) another editor's
// in-progress edits in the checkout cannot break or leak into a cell
// (guards-pilot-4 lost a wired cell to an uncommitted compile error,
// 2026-09-13). The cache key is the HEAD sha; a dirty tree changes nothing.
func buildFathomBinary(ctx context.Context, root, out string) (string, error) {
	sha, err := gitHeadSHA(ctx, root)
	if err != nil {
		return "", err
	}
	cached := filepath.Join(root, "workspace", "tmp", "fathom-bin-"+sha, "fathom")
	if _, statErr := os.Stat(cached); statErr != nil {
		if output, buildErr := buildFathomFromExport(ctx, root, sha, cached); buildErr != nil {
			return output, buildErr
		}
	}
	buildBinaryMu.Lock()
	defer buildBinaryMu.Unlock()
	return "", copyFile(cached, out)
}

// buildBinaryMu serializes the copy of a cached binary into a cell dir —
// two arms of one task wire their daemons concurrently under -parallel-arms.
var buildBinaryMu sync.Mutex

func gitHeadSHA(ctx context.Context, root string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("building fathom: git rev-parse HEAD in %s: %w", root, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// buildFathomFromExport exports HEAD into a temporary directory with
// `git archive` (tracked files only — the in-tree module mirror included,
// the working tree's edits excluded) and builds ./cmd/fathom there.
func buildFathomFromExport(ctx context.Context, root, sha, dest string) (string, error) {
	if err := os.MkdirAll(filepath.Join(root, "workspace", "tmp"), 0o755); err != nil {
		return "", fmt.Errorf("building fathom: %w", err)
	}
	tmp, err := os.MkdirTemp(filepath.Join(root, "workspace", "tmp"), "fathom-src-")
	if err != nil {
		return "", fmt.Errorf("building fathom: %w", err)
	}
	defer os.RemoveAll(tmp)
	archive := exec.CommandContext(ctx, "sh", "-c", "git -C \"$1\" archive --format=tar \"$2\" | tar -x -C \"$3\"", "sh", root, sha, tmp)
	if output, err := archive.CombinedOutput(); err != nil {
		return string(output), fmt.Errorf("building fathom: exporting %s: %w", sha[:12], err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("building fathom: %w", err)
	}
	// -buildvcs=false: the export is not a repository, yet it sits under
	// the checkout's tree, so go would try to read git's status for the
	// version stamp and fail under a wedged volume (dry-r3, 2026-09-14). The
	// cell records the fathom commit itself.
	cmd := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", dest, "./cmd/fathom")
	cmd.Dir = tmp
	cmd.Env = append(gitEnv(), "GOWORK=off", "CGO_ENABLED=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(dest)
		return string(output), fmt.Errorf("building fathom from export of %s: %w", sha[:12], err)
	}
	return string(output), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitDaemonReady(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	body := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"capabilities","arguments":{}}}`)
	for time.Now().Before(deadline) {
		resp, err := http.Post(baseURL+"/rpc/agent", "application/json", body)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		body = strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"capabilities","arguments":{}}}`)
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("scratch daemon at %s did not become ready within %s", baseURL, timeout)
}

// compileLayers runs `fathom compile -dry-run=false` (real LLM spend — an
// operator-initiated wired+CompileLayers cell only) for the ingested
// service's "obligations" layer against the scratch store directly
// (bypassing the daemon, per cmd/fathom/compile.go's --store mode), and
// reports its cost from the prose lines it prints ("tokens_in:",
// "tokens_out:", "cost_usd:"). An unrecognized output shape still returns
// a zero-cost CompileCost rather than failing the cell, since the compile
// step itself already succeeded (its exit code is what compileLayers
// propagates as an error).
func compileLayers(ctx context.Context, root string, task Task, wt Worktree) (*CompileCost, error) {
	bin := filepath.Join(wt.CellDir, "fathom")
	service := task.Subject.Service
	if service == "" {
		service = filepath.Base(wt.Path)
	}
	marker := filepath.Join(filepath.Dir(wt.StorePath), "compiled.ok")
	if _, err := os.Stat(marker); err == nil {
		// The shared store was compiled by an earlier cell on this
		// (subject, commit, arm); the layer is served, nothing is spent.
		return &CompileCost{Cached: true}, nil
	}
	start := time.Now()
	out, err := runIn(ctx, wt.Path, bin, "compile", "-service", service, "-store", wt.StorePath, "-layer", "obligations", "-dry-run=false")
	dur := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("fathom compile: %w (%s)", err, out)
	}
	if werr := os.WriteFile(marker, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644); werr != nil {
		return nil, fmt.Errorf("fathom compile: writing %s: %w", marker, werr)
	}
	cost := &CompileCost{Duration: dur}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "tokens_in:"):
			fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "tokens_in:")), "%d", &cost.TokensIn)
		case strings.HasPrefix(line, "tokens_out:"):
			fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "tokens_out:")), "%d", &cost.TokensOut)
		case strings.HasPrefix(line, "cost_usd:"):
			fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "cost_usd:")), "%f", &cost.CostUSD)
		}
	}
	return cost, nil
}

// InstrumentOwnedPaths returns the repo-relative paths (slash-separated,
// matching `git diff --name-only`'s own spelling) that PrepareSubject's own
// instrument — `fathom init`'s wiring, never the agent — OWNS OUTRIGHT
// (O55 item 4): every file internal/wiring's own ledger
// (`.fathom/owned.json`) records a key for — a whole owned file (KindFile)
// or a structured key in a file like `.claude/settings.json`/lefthook.yml
// (KindKey) — plus `.fathomignore` and the ledger file itself, which carry
// no entry of their own. A MARKED REGION (KindRegion — CLAUDE.md/
// AGENTS.md's own fathom stanza) is deliberately NOT here: apply.go only
// calls ledger.Record for KindKey/KindFile, so a region's ownership lives
// entirely in-band (the marker line's own digest, region.go) and never
// touches this ledger at all (fix-first review, 2026-09-14: the original
// version of this fix assumed it did, and so never actually excluded a
// real wired cell's stanza). cell.go's FilterInstrumentOwnedDiff uses this
// set for whole-file exclusion; InstrumentRegionRanges below is the
// separate, region-aware path captureAgentDiff also runs, which strips
// only a region's own lines rather than the whole file. A worktree
// PrepareSubject never wired (no `.fathom/owned.json` at all, an unwired
// arm's ordinary case) returns just the two static entries, never an
// error.
func InstrumentOwnedPaths(wt Worktree) (map[string]bool, error) {
	// The ledger names what it records ownership of — never itself — but
	// it is exactly as instrument-written as anything it lists.
	owned := map[string]bool{".fathomignore": true, ".fathom/owned.json": true}
	ledger, err := wiring.LoadLedger(wt.Path)
	if err != nil {
		return nil, fmt.Errorf("finding instrument-owned files: %w", err)
	}
	for file := range ledger.Files {
		owned[filepath.ToSlash(file)] = true
	}
	return owned, nil
}

// instrumentRegionFiles are the only files internal/wiring ever places a
// MARKED REGION into, always with wiring.HTMLMarkers (region.go's own
// doc comment: "HTMLMarkers delimits regions in Markdown files (CLAUDE.md,
// AGENTS.md)").
var instrumentRegionFiles = []string{"CLAUDE.md", "AGENTS.md"}

// InstrumentRegionRanges returns, for each of instrumentRegionFiles that
// currently carries fathom's own marked region on either side of the
// change, the 1-based inclusive line range that region occupies in the
// BASE commit's version of the file and in the worktree's CURRENT version
// — cell.go's stripFathomRegionsFromDiff strips exactly those lines from
// agent.diff's hunks for that file, keeping any other edit to it byte for
// byte. Base content comes from `git show HEAD:<path>` run IN wt.Path,
// which — since PrepareSubject gives every cell worktree a single root
// commit at base_commit (O55 item 1) — is always base_commit's own
// content, regardless of what the shared clone's own history looks like.
// A file absent at base, or carrying no region on EITHER side, is simply
// omitted from the result (nothing to strip for it); this never errors —
// an unreadable git show or a missing file are both "no base region",
// exactly as intended for a file that did not exist yet.
func InstrumentRegionRanges(ctx context.Context, wt Worktree) map[string]regionRange {
	out := map[string]regionRange{}
	for _, name := range instrumentRegionFiles {
		var rng regionRange
		if base, err := runIn(ctx, wt.Path, "git", "show", "HEAD:"+name); err == nil {
			if start, end, ok := regionLineRange(base, wiring.HTMLMarkers); ok {
				rng.oldStart, rng.oldEnd = start, end
			}
		}
		if cur, err := os.ReadFile(filepath.Join(wt.Path, name)); err == nil {
			if start, end, ok := regionLineRange(string(cur), wiring.HTMLMarkers); ok {
				rng.newStart, rng.newEnd = start, end
			}
		}
		if rng.oldStart == 0 && rng.newStart == 0 {
			continue
		}
		out[name] = rng
	}
	return out
}

// GitIgnoredWorkspacePaths reports which of the workspace/-prefixed paths
// diff names (in its own "diff --git a/X b/X" headers) are actually
// ignored by the SUBJECT's own .gitignore, checked with `git check-ignore
// --stdin` run in wt.Path (O55 item 4, fix-first review): workspace/ is
// THIS repo's own convention (root CLAUDE.md), never assumed of an
// arbitrary subject repo (ADR-011 keeps fathom itself org/repo-agnostic) —
// a subject that genuinely tracks real source under its own workspace/
// directory must not lose it wholesale. Only a workspace/ path the
// subject's own .gitignore actually ignores is reported; paths outside
// workspace/ are never queried at all. A git failure (not a repository,
// check-ignore unavailable) reports no paths ignored, never an error — the
// safer default is to KEEP a path in the diff, not silently drop a
// subject's real source over a git failure. nil when diff names no
// workspace/ path at all.
func GitIgnoredWorkspacePaths(ctx context.Context, wt Worktree, diff []byte) map[string]bool {
	var candidates []string
	for _, line := range strings.Split(string(diff), "\n") {
		if !strings.HasPrefix(line, "diff --git ") {
			continue
		}
		if p := diffGitHeaderPath(line); p == "workspace" || strings.HasPrefix(p, "workspace/") {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	// --no-index: a pure gitignore-pattern question ("would the subject's
	// own rules ignore this pathname"), not an index-state one. Without
	// it, git check-ignore reports an ALREADY-TRACKED path as "not
	// ignored" regardless of matching a pattern (its own documented
	// behavior — a later .gitignore rule does not retroactively un-track
	// a file already in the index), which would silently defeat this
	// filter for the realistic case captureAgentDiff hits: a workspace/
	// path that was tracked before a .gitignore rule started covering it,
	// then the agent edits it again.
	cmd := exec.CommandContext(ctx, "git", "check-ignore", "--no-index", "--stdin")
	cmd.Dir = wt.Path
	cmd.Stdin = strings.NewReader(strings.Join(candidates, "\n") + "\n")
	// git check-ignore exits 1 when NONE of the stdin paths are ignored —
	// not a failure worth reporting, just an empty result; its stdout
	// (the ignored subset, one per line) is populated regardless of exit
	// code, so the error return is deliberately ignored here.
	out, _ := cmd.Output()
	ignored := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			ignored[line] = true
		}
	}
	return ignored
}

// StripInstrumentRegions is InstrumentRegionRanges +
// stripFathomRegionsFromDiff (cell.go) as one exported call —
// cmd/scenariobench's captureAgentDiff's entry point for the region-aware
// half of O55 item 4, so regionRange and stripFathomRegionsFromDiff itself
// can stay package-private.
func StripInstrumentRegions(ctx context.Context, wt Worktree, diff []byte) ([]byte, int) {
	return stripFathomRegionsFromDiff(diff, InstrumentRegionRanges(ctx, wt))
}

// regionLineRange finds internal/wiring's own marker pair (syn —
// wiring.HTMLMarkers for CLAUDE.md/AGENTS.md) in content and returns the
// 1-based inclusive line range from the begin marker line through the end
// marker line — exactly the lines a region occupies on disk. ok is false
// when no complete marker pair is present (content carries no region at
// all, or — treated the same here — an unterminated one; a malformed
// marker is internal/wiring's own concern to refuse at write time, not
// this read-only filter's).
func regionLineRange(content string, syn wiring.MarkerSyntax) (start, end int, ok bool) {
	if content == "" {
		return 0, 0, false
	}
	lines := strings.Split(content, "\n")
	begin := -1
	for i, line := range lines {
		if strings.HasPrefix(line, syn.BeginPrefix) {
			begin = i
			break
		}
	}
	if begin < 0 {
		return 0, 0, false
	}
	for i := begin; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], syn.End) {
			return begin + 1, i + 1, true
		}
	}
	return 0, 0, false
}

// SubjectEnv is the environment every subprocess that runs INSIDE a subject
// worktree gets — the oracle's tests and detectors, the subject's and the
// arm's setup, and the agent's own session. A linked worktree of a repo
// with a go.work whose `use` lines name sibling directories cannot resolve
// them from workspace/scenariobench/…, so `go test` fails with "[setup
// failed]" before the agent has done anything and every detector reads
// "violated" (dry cells, 2026-09-13). With a go.work present the worktree
// builds from its own go.mod (GOWORK=off); extra is merged in sorted order.
func SubjectEnv(worktree string, extra map[string]string) []string {
	env := gitEnv()
	// Always off, not only when the subject has a go.work: a subject
	// worktree lives under workspace/ of THIS repo, and Go walks up from
	// it to this repo's go.work, whose main module then "does not contain
	// package …/workspace/scenariobench/…" (dry cell 5, 2026-09-13). A
	// subject that genuinely needs a workspace sets GOWORK in its Setup
	// or the arm's Env, which is merged after this and wins.
	env = append(env, "GOWORK=off")
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		env = append(env, k+"="+extra[k])
	}
	return env
}
