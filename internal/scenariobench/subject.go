// PrepareSubject (SB-12) turns a Task's pinned subject into a git worktree
// ready for sessions: a shared clone under
// workspace/scenariobench/subjects/<sha256(repo_url)>, one `git worktree
// add` per cell at base_commit, the subject's and the arm's setup commands,
// and — for a wired arm — a scratch daemon that ingests ONLY that worktree
// (never the fleet store) so `fathom init --harness claude-code --json` can
// wire it for real.
package scenariobench

import (
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
	"strings"
	"sync"
	"time"
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
	if _, err := os.Stat(filepath.Join(wtPath, ".git")); err == nil {
		if out, err := runIn(ctx, wtPath, "git", "reset", "--hard", task.Subject.BaseCommit); err != nil {
			return Worktree{}, fmt.Errorf("preparing subject: resetting the shared worktree: %w (%s)", err, out)
		}
		if out, err := runIn(ctx, wtPath, "git", "clean", "-fdx"); err != nil {
			return Worktree{}, fmt.Errorf("preparing subject: cleaning the shared worktree: %w (%s)", err, out)
		}
	} else if out, err := runIn(ctx, cloneDir, "git", "worktree", "add", "--detach", wtPath, task.Subject.BaseCommit); err != nil {
		return Worktree{}, fmt.Errorf("preparing subject: git worktree add: %w (%s)", err, out)
	}

	wt := Worktree{Path: wtPath, SubjectClone: cloneDir}
	wt.CellDir = filepath.Join(root, "workspace", "artifacts", "scenariobench", "worktrees", task.ID+"-"+filepath.Base(wtPath))
	if err := os.MkdirAll(wt.CellDir, 0o755); err != nil {
		return Worktree{}, fmt.Errorf("preparing subject: creating %s: %w", wt.CellDir, err)
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

	return wt, nil
}

// StopWorktree stops a wired cell's scratch daemon (SB-12: "stop it after
// the last session"). A control/unwired worktree has none; calling it on
// one is a no-op.
func StopWorktree(wt *Worktree) error {
	if wt == nil || wt.daemon == nil || wt.daemon.Process == nil {
		return nil
	}
	if err := wt.daemon.Process.Kill(); err != nil {
		return fmt.Errorf("stopping scratch daemon: %w", err)
	}
	_ = wt.daemon.Wait()
	wt.daemon = nil
	return nil
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
// file and daemon, never the operator's real ones that a bare os.Environ()
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
	// of the machine running the bench.
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
	cmd := exec.CommandContext(ctx, "go", "build", "-o", dest, "./cmd/fathom")
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(), "GOWORK=off", "CGO_ENABLED=1")
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

// SubjectEnv is the environment every subprocess that runs INSIDE a subject
// worktree gets — the oracle's tests and detectors, the subject's and the
// arm's setup, and the agent's own session. A linked worktree of a repo
// with a go.work whose `use` lines name sibling directories cannot resolve
// them from workspace/scenariobench/…, so `go test` fails with "[setup
// failed]" before the agent has done anything and every detector reads
// "violated" (dry cells, 2026-09-13). With a go.work present the worktree
// builds from its own go.mod (GOWORK=off); extra is merged in sorted order.
func SubjectEnv(worktree string, extra map[string]string) []string {
	env := os.Environ()
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
