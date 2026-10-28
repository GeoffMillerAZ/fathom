package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/benchprep"
	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

func init() { offlineCommands["outcome"] = cmdOutcome }

// outcomeNoRead is the reading of a run whose provenance holds, until the
// classifier lands (D11).
const outcomeNoRead = "no-read: ClassifyOutcome is not built (BX-63)"

// cmdOutcome reads one run against its prereg (NP-B1, BX-88). It loads the
// prereg, refuses a shallow repository by name (its history cannot date the
// prereg), reads the prereg's last commit time with `git log -1
// --format=%cI`, and runs CheckPreregProvenance over every ledger row of
// the run. Output is sorted key: value lines naming the prereg id, its sha,
// the commit time, first_cell_at, the rows read and every refusal. Exit 2
// for a refusal, a usage error or an IO error; exit 3 when provenance holds,
// a no-read naming BX-63 until ClassifyOutcome exists (D11). Every git call
// is local (rev-parse, log) under the scrubbed environment (I14).
func cmdOutcome(args []string) int {
	flags := newFlagSet("outcome")
	preregPath := flags.String("prereg", "", "docs/bench/prereg/<id>.json the run was preregistered under (required)")
	ledgerPath := flags.String("ledger", filepath.Join("docs", "bench", "scenarios-ledger.jsonl"), "scenario ledger holding the run's rows")
	runID := flags.String("run-id", "", "the run to read (required)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *preregPath == "" || *runID == "" || *ledgerPath == "" {
		fmt.Fprintln(os.Stderr, "scenariobench outcome: -prereg, -ledger and -run-id are required")
		return 2
	}
	// Resolve symlinks first, as `run -prereg` does: the sha is the target's,
	// so the commit time must be the target's too, never a link's own.
	resolved, err := filepath.Abs(*preregPath)
	if err == nil {
		resolved, err = filepath.EvalSymlinks(resolved)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench outcome: -prereg %s: %v\n", *preregPath, err)
		return 2
	}
	prereg, sha, err := benchprep.LoadPrereg(resolved)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench outcome: %v\n", err)
		return 2
	}
	if _, err := os.Stat(*ledgerPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "scenariobench outcome: ledger %s does not exist\n", *ledgerPath)
		} else {
			fmt.Fprintf(os.Stderr, "scenariobench outcome: %v\n", err)
		}
		return 2
	}
	rows, err := scenariobench.LoadScenarioLedger(*ledgerPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench outcome: %v\n", err)
		return 2
	}

	dir := filepath.Dir(resolved)
	shallow, err := preregGitOutput(dir, "rev-parse", "--is-shallow-repository")
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench outcome: %v\n", err)
		return 2
	}
	if strings.TrimSpace(shallow) == "true" {
		fmt.Fprintf(os.Stderr, "scenariobench outcome: refused: the prereg's repository is a shallow clone, so its history cannot date %s; unshallow it first\n", *preregPath)
		return 2
	}
	logged, err := preregGitOutput(dir, "log", "-1", "--format=%cI", "--", filepath.Base(resolved))
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench outcome: %v\n", err)
		return 2
	}
	var committedAt time.Time
	if text := strings.TrimSpace(logged); text != "" {
		if committedAt, err = time.Parse(time.RFC3339, text); err != nil {
			fmt.Fprintf(os.Stderr, "scenariobench outcome: reading the prereg's commit time %q: %v\n", text, err)
			return 2
		}
	}

	provenanceRows := make([]benchprep.ProvenanceRow, 0, len(rows))
	for _, r := range rows {
		provenanceRows = append(provenanceRows, benchprep.ProvenanceRow{
			RunID: r.RunID, TaskID: r.TaskID, Arm: r.Arm, PreregSHA256: r.PreregSHA256, FirstCellAt: r.FirstCellAt,
		})
	}
	reading := benchprep.CheckPreregProvenance(*runID, committedAt, sha, provenanceRows)

	lines := []string{
		"commit_time: " + outcomeTime(reading.CommittedAt, "unknown"),
		"first_cell_at: " + outcomeTime(reading.FirstCellAt, "none"),
		"prereg: " + prereg.ID,
		fmt.Sprintf("rows: %d", reading.Rows),
		"run_id: " + *runID,
		"sha256: " + sha,
	}
	for _, r := range reading.Refusals {
		lines = append(lines, "refusal: "+r)
	}
	code := 3
	if reading.Refused() {
		lines = append(lines, fmt.Sprintf("outcome: refused (%d refusal(s))", len(reading.Refusals)))
		code = 2
	} else {
		lines = append(lines, "outcome: "+outcomeNoRead)
	}
	sort.Strings(lines)
	fmt.Println(strings.Join(lines, "\n"))
	return code
}

func outcomeTime(t time.Time, zero string) string {
	if t.IsZero() {
		return zero
	}
	return t.UTC().Format(time.RFC3339)
}
