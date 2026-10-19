// leases.go — WF-5/WF-7 (docs/vision/24-work-frontier.md §4): declared
// intent, derived state. A lease is one JSON row appended to
// .fathom/leases.jsonl; "leased" is never a stored status — it is always
// recomputed from the rows at read time (row exists ∧ now < expires ∧
// !released). Multiple holders can file fresh leases on the same row
// simultaneously and BOTH stay visible — that coexistence is deliberate:
// it is the collision `fathom plans leases` counts, not an error this
// layer resolves. Refused by design (vision 24 §4): heartbeats,
// supervisors, reminder hooks — TTL-only expiry, because Codex has no
// lifecycle surface to fake.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// leaseRow is one JSONL row: a filed lease, or a release of one. released
// is omitted (zero value) on a filing row so a filing and its eventual
// release are byte-distinguishable in the raw file.
type leaseRow struct {
	Plan     string    `json:"plan"`
	RowID    string    `json:"row_id"`
	Holder   string    `json:"holder"`
	Harness  string    `json:"harness"`
	Filed    time.Time `json:"filed"`
	Expires  time.Time `json:"expires"`
	Released bool      `json:"released,omitempty"`
}

// leasesPath is where a repo's leases live — never inside docs/plans/
// (leases are not plan content) and never store/verify state.
func leasesPath(root string) string {
	return projectdir.Leases.Path(root)
}

// appendLease appends one row to .fathom/leases.jsonl under a cross-process
// flock: unlike observe.go's advisory-id/O_APPEND-only approach (fine
// there because entry order is the only thing that matters and O_APPEND
// alone keeps single writes atomic), a lease writer must never have its
// line interleave with a concurrent writer's — the file's contract IS the
// authoritative event ledger the overlap detector counts on, so this is
// the flock upgrade observe.go's own comment names as the escalation path.
func appendLease(root string, row leaseRow) error {
	path := leasesPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("locking %s: %w", path, err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	line, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("encoding lease row: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("appending to %s: %w", path, err)
	}
	return nil
}

// readLeases reads every row in .fathom/leases.jsonl. A missing file is
// "no leases yet" (nil, nil), not an error. A malformed line is skipped —
// this is a best-effort derived-state instrument, never a gate, so one bad
// line must never hide every other row.
func readLeases(root string) ([]leaseRow, error) {
	data, err := os.ReadFile(leasesPath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", leasesPath(root), err)
	}
	var rows []leaseRow
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var r leaseRow
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// latestLeaseEvents reduces the raw append-only rows to the latest event
// per (plan, row_id, holder) triple — file order is chronological
// (append-only, same convention observe.go's advisory ids rely on), so the
// last row for a triple is that holder's current claim (or release) on
// that row.
func latestLeaseEvents(rows []leaseRow) map[[3]string]leaseRow {
	latest := make(map[[3]string]leaseRow, len(rows))
	for _, r := range rows {
		latest[[3]string{r.Plan, r.RowID, r.Holder}] = r
	}
	return latest
}

// activeLeases returns every FRESH lease at now: latest event per
// (plan, row_id, holder) that is not released and has not expired.
// Deliberately not collapsed to one-per-row — two holders can both be
// fresh on the same (plan, row_id) at once (the overlap WF-7 counts).
func activeLeases(rows []leaseRow, now time.Time) []leaseRow {
	var out []leaseRow
	for _, r := range latestLeaseEvents(rows) {
		if !r.Released && now.Before(r.Expires) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Plan != out[j].Plan {
			return out[i].Plan < out[j].Plan
		}
		if out[i].RowID != out[j].RowID {
			return out[i].RowID < out[j].RowID
		}
		return out[i].Holder < out[j].Holder
	})
	return out
}

// ─── fathom plans lease (WF-5) ──────────────────────────────────────────────

func cmdPlansLease(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: plans lease requires a row-id argument")
		printPlansLeaseUsage(stderr)
		return 1
	}
	rowID := args[0]

	fs := newFlagSet("plans lease", stderr)
	planFile := fs.String("plan", "", "plan file this row belongs to (required)")
	ttl := fs.Duration("ttl", 2*time.Hour, "lease time-to-live")
	release := fs.Bool("release", false, "release this row's lease instead of filing one")
	holder := fs.String("holder", "", "lease holder (defaults to $CLAUDE_SESSION_ID; required otherwise)")
	root := fs.String("root", ".", "project root — .fathom/leases.jsonl lives under here")
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}
	if *planFile == "" {
		fmt.Fprintln(stderr, "error: --plan is required")
		fs.Usage()
		return 1
	}

	// HP-14: the label and the session id come from the registry's env
	// contract (hookharness.go), never a CLAUDE* literal here.
	harnessName, sessionID := harnessLabel()
	h := *holder
	if sessionID != "" && h == "" {
		h = sessionID
	}
	if h == "" {
		fmt.Fprintln(stderr, "error: --holder is required when no harness session variable (e.g. $CLAUDE_SESSION_ID) is set")
		fs.Usage()
		return 1
	}

	now := nowFunc()
	row := leaseRow{
		Plan: *planFile, RowID: rowID, Holder: h, Harness: harnessName,
		Filed: now.UTC(), Expires: now.Add(*ttl).UTC(), Released: *release,
	}
	if err := appendLease(*root, row); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if *release {
		fmt.Fprintf(stdout, "released %s (%s) held by %s\n", rowID, *planFile, h)
	} else {
		fmt.Fprintf(stdout, "leased %s (%s) for %s, holder %s, expires %s\n",
			rowID, *planFile, harnessName, h, row.Expires.Format(time.RFC3339))
	}
	return 0
}

func printPlansLeaseUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: fathom plans lease <row-id> --plan <file> [--ttl 2h] [--release] [--holder s]")
}

// ─── fathom plans leases (WF-7) ─────────────────────────────────────────────

// leaseView is one row of `fathom plans leases`' output — the latest,
// non-released event per (plan, row_id, holder), tagged fresh/expired.
type leaseView struct {
	Plan    string `json:"plan"`
	RowID   string `json:"row_id"`
	Holder  string `json:"holder"`
	Harness string `json:"harness"`
	Filed   string `json:"filed"`
	Expires string `json:"expires"`
	Fresh   bool   `json:"fresh"`
}

func cmdPlansLeases(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("plans leases", stderr)
	root := fs.String("root", ".", "project root — .fathom/leases.jsonl lives under here")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human table")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	rows, err := readLeases(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	now := nowFunc()

	var views []leaseView
	// overlapKey → count of currently-fresh holders on that (plan, row_id):
	// the collision instrument itself (WF-7's own evidence case).
	freshCount := map[string]int{}
	for _, r := range latestLeaseEvents(rows) {
		if r.Released {
			// This holder currently holds nothing on this row — neither
			// active nor expired; omitted from the listing entirely.
			continue
		}
		fresh := now.Before(r.Expires)
		views = append(views, leaseView{
			Plan: r.Plan, RowID: r.RowID, Holder: r.Holder, Harness: r.Harness,
			Filed: r.Filed.UTC().Format(time.RFC3339), Expires: r.Expires.UTC().Format(time.RFC3339),
			Fresh: fresh,
		})
		if fresh {
			freshCount[filepath.Base(r.Plan)+"|"+r.RowID]++
		}
	}
	sort.Slice(views, func(i, j int) bool {
		if views[i].Plan != views[j].Plan {
			return views[i].Plan < views[j].Plan
		}
		if views[i].RowID != views[j].RowID {
			return views[i].RowID < views[j].RowID
		}
		return views[i].Holder < views[j].Holder
	})
	overlaps := 0
	for _, c := range freshCount {
		if c >= 2 {
			overlaps++
		}
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"leases": views, "overlaps": overlaps})
		return 0
	}

	if len(views) == 0 {
		fmt.Fprintln(stdout, "no leases on file")
		return 0
	}
	fmt.Fprintf(stdout, "%-30s %-8s %-18s %-8s %-9s %s\n", "plan", "row", "holder", "harness", "state", "expires")
	for _, v := range views {
		state := "expired"
		if v.Fresh {
			state = "fresh"
		}
		fmt.Fprintf(stdout, "%-30s %-8s %-18s %-8s %-9s %s\n",
			filepath.Base(v.Plan), v.RowID, v.Holder, v.Harness, state, v.Expires)
	}
	fmt.Fprintf(stdout, "\noverlaps: %d\n", overlaps)
	return 0
}
