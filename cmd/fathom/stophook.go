package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// stopHookVerifyTimeout bounds each verify_plan HTTP call the Stop hook
// makes. It is sized to verify_plan's own p95 latency SLO
// (docs/design/coding-agent-excellence.md §2: p95 ≤ 5s) rather than the
// blanket 2s convention other, cheap hook calls use (§5.3) — verify_plan is
// explicitly budgeted as the one expensive companion tool (B3,
// docs/plans/falsifiability.md). A call that exceeds this degrades to an
// advisory warning (fail-open, §5.3: hooks never block on infrastructure)
// instead of stalling session end.
const stopHookVerifyTimeout = 5 * time.Second

// cmdStopHook implements "stop-hook": a Claude Code Stop hook that checks
// whether any plan task was newly claimed ([x]) during this session without
// verifying against the index (ADR-018 §4 — done means proven, at session
// granularity).
//
// Output policy (mirrors edit-hook doctrine — hooks never block on
// operational failure):
//   - ANY error / daemon-down / no git / no plans → print nothing, exit 0.
//   - stop_hook_active true → exit 0 (no re-entry loops).
//   - Newly claimed tasks all verify (or are Unknown) → exit 0 silently.
//   - Newly claimed tasks FAIL verification → advisory additionalContext by
//     default; {"decision":"block"} with --block.
func cmdStopHook(args []string, stdout, stderr io.Writer) int {
	rpcOrigin = "stop-hook"
	fs := newFlagSet("stop-hook", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	block := fs.Bool("block", false, "block session end on disproven claims (advisory otherwise)")
	if err := fs.Parse(args); err != nil {
		return 0 // silent (hook contract)
	}
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		return 0
	}
	var hook struct {
		StopHookActive bool   `json:"stop_hook_active"`
		CWD            string `json:"cwd"`
	}
	if err := json.Unmarshal(raw, &hook); err != nil {
		return 0
	}
	if hook.StopHookActive {
		return 0
	}
	projDir := os.Getenv("CLAUDE_PROJECT_DIR")
	if projDir == "" {
		projDir = hook.CWD
	}
	if projDir == "" {
		projDir, _ = os.Getwd()
	}
	hc := &http.Client{Timeout: stopHookVerifyTimeout}
	return runStopHook(projDir, hc, *url, *block, stdout)
}

// runStopHook is the testable core of cmdStopHook: everything after stdin
// parsing and project-dir resolution.
func runStopHook(projDir string, hc *http.Client, baseURL string, block bool, stdout io.Writer) int {
	plansDir := filepath.Join(projDir, "docs", "plans")
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		return 0 // no plans → nothing to prove
	}

	// Resolve the service identity: first candidate the daemon has indexed
	// wins — same discipline as the edit-hook.
	service := ""
	for _, cand := range serviceCandidates(projDir) {
		name := strings.ToLower(cand.name)
		ok, probeErr := daemonServiceIndexed(hc, baseURL, name)
		if probeErr != nil {
			return 0 // daemon down → silent
		}
		if ok {
			service = name
			break
		}
	}
	if service == "" {
		return 0 // unindexed project → silent
	}

	var complaints, timedOutPlans []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || strings.EqualFold(e.Name(), "README.md") {
			continue
		}
		rel := filepath.Join("docs", "plans", e.Name())
		newMD, err := os.ReadFile(filepath.Join(projDir, rel))
		if err != nil {
			continue
		}
		oldMD := headFileVersion(projDir, rel) // nil when new file / no git
		newly := newlyClaimedTasks(oldMD, newMD, service)
		if len(newly) == 0 {
			continue
		}
		localPlan := verify.ParseMarkdownPlan(service, newMD)
		rep, err := verifyViaHTTP(hc, baseURL, service, "", newMD, localPlan)
		if err != nil {
			if isTimeoutErr(err) {
				// B3-1: a slow verify degrades to a warning for THIS plan —
				// fail open, never silent, never a stall — but other plan
				// files still get a chance to verify.
				timedOutPlans = append(timedOutPlans, rel)
				continue
			}
			return 0 // other operational errors (daemon down, bad response) → silent
		}
		verdictByID := map[string]verify.TaskVerdict{}
		for _, tv := range rep.Tasks {
			verdictByID[tv.TaskID] = tv
		}
		for _, id := range newly {
			tv, ok := verdictByID[id]
			if !ok || tv.Status != verify.Fail {
				continue // Unknown never fires the hook — fail-open honesty
			}
			detail := tv.Label
			for _, c := range tv.Criteria {
				if c.Status == verify.Fail {
					detail = fmt.Sprintf("%s: %s", c.Name, c.Detail)
					break
				}
			}
			complaints = append(complaints, fmt.Sprintf("%s [%s] %s — %s", rel, tv.TaskID, tv.Title, detail))
		}
	}
	if len(complaints) == 0 && len(timedOutPlans) == 0 {
		return 0
	}
	sort.Strings(complaints)
	sort.Strings(timedOutPlans)

	var sections []string
	if len(complaints) > 0 {
		sections = append(sections, "⛔ done-means-proven: task(s) claimed [x] this session fail verification:\n• "+
			strings.Join(complaints, "\n• ")+
			"\nProve the claim (fix the code / contract) or uncheck it before ending the session.")
	}
	if len(timedOutPlans) > 0 {
		sections = append(sections, fmt.Sprintf(
			"⚠️ done-means-proven: verify timed out (>%s) for %s — newly claimed task(s) there could not be confirmed this session; treat them as unconfirmed until verify runs clean.",
			stopHookVerifyTimeout, strings.Join(timedOutPlans, ", "),
		))
	}
	msg := strings.Join(sections, "\n\n")

	var out []byte
	// A timeout is operational, never evidence of a false claim — it must
	// stay advisory even when --block is set (§5.3: hooks never block on
	// infrastructure). Only a proven-failing claim may block.
	if block && len(complaints) > 0 {
		out, _ = json.Marshal(map[string]any{"decision": "block", "reason": msg})
	} else {
		out, _ = json.Marshal(map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "Stop",
				"additionalContext": msg,
			},
		})
	}
	fmt.Fprintln(stdout, string(out))
	return 0
}

// isTimeoutErr reports whether err (possibly wrapped, e.g. by verifyViaHTTP's
// fmt.Errorf("...: %w", err)) is a client-side timeout — either an
// *http.Client deadline (surfaced as a net.Error with Timeout() true,
// typically *url.Error wrapping context.DeadlineExceeded) or a bare
// context.DeadlineExceeded from a canceled request context.
func isTimeoutErr(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded)
}

// newlyClaimedTasks returns the IDs of tasks that are claimed in newMD but
// were not claimed in oldMD (oldMD nil ⇒ every claimed task is new). Matching
// is by task ID: exact with stable `id:` keys, best-effort with positional
// IDs — acceptable for an advisory channel (ADR-018 §4).
func newlyClaimedTasks(oldMD, newMD []byte, service string) []string {
	oldClaimed := map[string]bool{}
	if oldMD != nil {
		for _, t := range verify.ParseMarkdownPlan(service, oldMD).Tasks {
			if t.Claimed {
				oldClaimed[t.ID] = true
			}
		}
	}
	var out []string
	for _, t := range verify.ParseMarkdownPlan(service, newMD).Tasks {
		if t.Claimed && !oldClaimed[t.ID] {
			out = append(out, t.ID)
		}
	}
	return out
}

// headFileVersion returns rel's content at git HEAD, or nil when unavailable
// (new file, not a git repo, git missing).
func headFileVersion(repoDir, rel string) []byte {
	cmd := exec.Command("git", "-C", repoDir, "show", "HEAD:"+filepath.ToSlash(rel))
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return out
}
