package console

// proof.go — the verify ledger as the console reads it (vision 35 §3, Now
// and Fleet). Every `fathom verify` run appends one row per plan to the
// user-central verify ledger (cmd/fathom/verifyledger.go). The console never
// runs a verify to render a page — 45 plans is about 70 seconds — so a
// plan's proof is a recorded run, and the page says which one.
//
// What the gate says of the code as it is, is a run at the project's HEAD
// commit. The pre-push gate verifies in a detached clone (fathom's own
// pushclone), so a run at HEAD may be recorded at any root: the commit, not
// the directory, names the code it proved (with the service, which names
// the index it proved it against). PE-24 (O121): a reader keeps only rows
// recorded at a registered service root (RegisteredRuns), so a clone's run
// reaches PlanRuns only when its root is registered. Only when no run at HEAD exists does the
// console fall back to the newest run at the project's registered root —
// and then it is stale: it proved an older commit, and says which and when
// (design §2: stale carries its age). Runs in scratch clones and bench
// subjects at other commits are other code, never the project's state.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
)

// VerifyRun is one verify-ledger row: one plan, one run.
type VerifyRun struct {
	At       string `json:"at"`
	RepoRoot string `json:"repo_root,omitempty"`
	Service  string `json:"service,omitempty"`
	Plan     string `json:"plan"`
	Commit   string `json:"commit,omitempty"`
	Total    int    `json:"total"`
	Claimed  int    `json:"claimed"`
	Verified int    `json:"verified"`
	Failing  int    `json:"failing"`
	Origin   string `json:"origin,omitempty"`
	// StaleReason is set by PlanRuns (never read from the ledger) when the
	// run is not at the project's HEAD: it proved other code, and this
	// says which commit and why the console cannot call it current.
	StaleReason string `json:"stale,omitempty"`
}

// Current reports whether the run proved the project's HEAD.
func (r VerifyRun) Current() bool { return r.StaleReason == "" }

// Unproven is how many claimed rows the run could neither prove nor refute
// (no contract the gate can test, an unknown verdict): `verify --strict`
// refuses them, the default gate does not.
func (r VerifyRun) Unproven() int {
	if n := r.Claimed - r.Verified - r.Failing; n > 0 {
		return n
	}
	return 0
}

// ParseVerifyLedger reads ledger bytes into runs; malformed counts lines
// that did not parse (named by the caller, never silently dropped).
func ParseVerifyLedger(data []byte) (runs []VerifyRun, malformed int) {
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var r VerifyRun
		if err := json.Unmarshal(line, &r); err != nil || r.Plan == "" || r.At == "" {
			malformed++
			continue
		}
		runs = append(runs, r)
	}
	return runs, malformed
}

// RegisteredRuns keeps the runs whose repo_root is a registered service
// root under roots and names what it ignored (PE-24, O121): a scratch
// clone's or a worktree's run is not the fleet's proof. An unread registry
// keeps every run and says why.
func RegisteredRuns(runs []VerifyRun, roots ledgerroots.Roots) ([]VerifyRun, ledgerroots.Ignored) {
	f := roots.NewFilter()
	out := make([]VerifyRun, 0, len(runs))
	for _, r := range runs {
		if f.Keep(r.RepoRoot) {
			out = append(out, r)
		}
	}
	return out, f.Ignored()
}

// ProjectRunsQuery names one project for PlanRuns.
type ProjectRunsQuery struct {
	Service string
	Root    string
	// Head is the root's HEAD commit; "" when it could not be read (then
	// no run can be confirmed current). HeadNote says why, when known.
	Head     string
	HeadNote string
	// SameRoot reports whether a recorded root is Root (a symlinked
	// spelling of the same directory counts).
	SameRoot func(recorded string) bool
	// Exists, when non-nil, drops plans no longer on disk: an archived
	// plan's last run is history, not the project's state.
	Exists func(plan string) bool
}

// PlanRuns returns each plan's proof, in plan order: the newest run
// recorded at the project's HEAD commit — at the registered root or in any
// clone of the same service (a run elsewhere names its plan relative to
// its own root) — else the newest run at the registered root, marked stale
// with the commit it proved.
func PlanRuns(runs []VerifyRun, q ProjectRunsQuery) []VerifyRun {
	if q.Root == "" {
		return nil
	}
	atHead := map[string]VerifyRun{}
	atRoot := map[string]VerifyRun{}
	for _, r := range runs {
		atRootHere := q.SameRoot != nil && q.SameRoot(r.RepoRoot)
		headHere := sameCommit(r.Commit, q.Head) && (strings.EqualFold(r.Service, q.Service) || (atRootHere && r.Service == ""))
		if !atRootHere && !headHere {
			continue
		}
		base := q.Root
		if !atRootHere {
			base = r.RepoRoot
		}
		r.Plan = RelPlan(base, r.Plan)
		r.StaleReason = ""
		if headHere {
			if prev, ok := atHead[r.Plan]; !ok || r.At > prev.At {
				atHead[r.Plan] = r
			}
		}
		if atRootHere {
			if prev, ok := atRoot[r.Plan]; !ok || r.At > prev.At {
				atRoot[r.Plan] = r
			}
		}
	}
	out := make([]VerifyRun, 0, len(atRoot)+len(atHead))
	for plan, r := range atHead {
		if q.Exists == nil || q.Exists(plan) {
			out = append(out, r)
		}
	}
	for plan, r := range atRoot {
		if _, ok := atHead[plan]; ok || (q.Exists != nil && !q.Exists(plan)) {
			continue
		}
		r.StaleReason = staleRunReason(r, q)
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Plan < out[j].Plan })
	return out
}

// staleRunReason says why a run at the root is not the project's current
// proof: it recorded another commit, no commit, or HEAD is unknown.
func staleRunReason(r VerifyRun, q ProjectRunsQuery) string {
	switch {
	case q.Head == "":
		why := "HEAD could not be read"
		if q.HeadNote != "" {
			why += " (" + q.HeadNote + ")"
		}
		return why + ", so no run can be confirmed current"
	case r.Commit == "":
		return "the run recorded no commit; HEAD is " + shortCommit(q.Head)
	}
	return "recorded at " + shortCommit(r.Commit) + "; HEAD is " + shortCommit(q.Head)
}

// sameCommit reports whether two commit ids (either may be abbreviated,
// at least 7 hex digits) name the same commit.
func sameCommit(a, b string) bool {
	a, b = strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b))
	if len(a) < 7 || len(b) < 7 {
		return false
	}
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

// RelPlan names a recorded plan path relative to root when it lies under
// it; anything else is returned cleaned.
func RelPlan(root, plan string) string {
	p := filepath.Clean(plan)
	if filepath.IsAbs(p) && root != "" {
		if rel, err := filepath.Rel(filepath.Clean(root), p); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(p)
}

// PlanProof totals a project's proof: the newest run of each plan (see
// PlanRuns). Rows, Claimed and Verified count every plan's run. Failing and
// Unproven are what the gate says of HEAD — runs at HEAD only; the same
// counts from runs that proved an older commit are history, kept apart in
// StaleFailing and StaleUnproven.
type PlanProof struct {
	Plans    int `json:"plans"`
	Rows     int `json:"rows"`
	Claimed  int `json:"claimed"`
	Verified int `json:"verified"`
	Failing  int `json:"failing"`
	Unproven int `json:"unproven"`
	// Unclaimed rows are open, in progress or skipped: the ledger does not
	// tell them apart, so neither does this total.
	Unclaimed int    `json:"unclaimed"`
	Oldest    string `json:"oldest_run"`
	Newest    string `json:"newest_run"`
	// Stale is how many plans' newest run predates HEAD; StaleOldest the
	// oldest of those runs.
	Stale         int    `json:"stale_plans"`
	StaleFailing  int    `json:"stale_failing"`
	StaleUnproven int    `json:"stale_unproven"`
	StaleOldest   string `json:"stale_oldest_run,omitempty"`
	// Unrun names active plans no recorded run ever checked (filled by
	// the caller that knows the plan files; SumPlanProof sees only runs).
	Unrun []string `json:"unrun_plans,omitempty"`
}

// SumPlanProof folds runs (one per plan) into one project's proof.
func SumPlanProof(runs []VerifyRun) PlanProof {
	p := PlanProof{Plans: len(runs)}
	for _, r := range runs {
		p.Rows += r.Total
		p.Claimed += r.Claimed
		p.Verified += r.Verified
		if r.Current() {
			p.Failing += r.Failing
			p.Unproven += r.Unproven()
		} else {
			p.Stale++
			p.StaleFailing += r.Failing
			p.StaleUnproven += r.Unproven()
			if p.StaleOldest == "" || r.At < p.StaleOldest {
				p.StaleOldest = r.At
			}
		}
		if u := r.Total - r.Claimed; u > 0 {
			p.Unclaimed += u
		}
		if p.Oldest == "" || r.At < p.Oldest {
			p.Oldest = r.At
		}
		if r.At > p.Newest {
			p.Newest = r.At
		}
	}
	return p
}

// UnrunPlans are the project's active plan files no recorded run checked.
func UnrunPlans(r ProjectFacts) []string {
	seen := map[string]bool{}
	for _, run := range r.Runs {
		seen[run.Plan] = true
	}
	var out []string
	for _, f := range r.PlanFiles {
		if !seen[f] {
			out = append(out, f)
		}
	}
	return out
}

// ProjectProof is SumPlanProof over a project's runs with its unrun plans.
func ProjectProof(r ProjectFacts) PlanProof {
	p := SumPlanProof(r.Runs)
	p.Unrun = UnrunPlans(r)
	return p
}

// StaleProofReason says, in words, how a project's proof predates its
// HEAD: which plan (or how many), the commit and date the oldest such run
// proved, and its age at now. "" when every run is current.
func StaleProofReason(runs []VerifyRun, head string, now time.Time) string {
	var stale []VerifyRun
	for _, r := range runs {
		if !r.Current() {
			stale = append(stale, r)
		}
	}
	if len(stale) == 0 {
		return ""
	}
	sort.SliceStable(stale, func(i, j int) bool { return stale[i].At < stale[j].At })
	oldest := stale[0]
	when := runWhen(oldest)
	if age := ageText(oldest.At, now); age != "" {
		when += ", " + age + " old"
	}
	subject := path.Base(oldest.Plan) + " was"
	if len(stale) > 1 {
		subject = fmt.Sprintf("%d of %d plans were", len(stale), len(runs))
		when = "the oldest " + when
	}
	if head == "" {
		return subject + " verified at a commit that cannot be confirmed current (" + when + ") — " + oldest.StaleReason
	}
	return subject + " last verified before HEAD " + shortCommit(head) + " (" + when + ")"
}

// ageText is "6 d", "5 h" or "12 min" between an RFC 3339 time and now;
// "" when either is unknown.
func ageText(at string, now time.Time) string {
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil || now.IsZero() {
		return ""
	}
	d := now.Sub(t)
	switch {
	case d < 0:
		return ""
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h", int(d.Hours()))
	}
	return fmt.Sprintf("%d d", int(d.Hours()/24))
}
