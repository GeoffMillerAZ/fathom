package main

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/adr"
)

// K1's pre-clock baseline (EP-24, ADR-026 §9).
//
// The kill criterion for the intent plane is "decisions get answered sooner".
// Law 12 forbids that claim without a counterfactual, so the number this
// measures is how long a decision record sat between arriving and having its
// `Status` answered — BEFORE any of this shipped.
//
// It is measured over the DECISION RECORDS, not over docs/intent/, because
// docs/intent/ is two days old here and a 90-day lookback over it returns
// nothing. Measuring the corpus that exists is the honest instrument; a
// pipeline that provably returns zero is theatre.

// intentBaseline walks the decisions file's git history and returns, per
// record, the days between the commit that introduced it and the commit that
// first changed its `Status`. The third return NAMES why a measurement is
// missing or partial — it is never empty.
func intentBaseline(root, decisionsFile string, now time.Time, lookbackDays int) (days []int, median int, note string) {
	if decisionsFile == "" {
		return nil, 0, "no decisions home resolved, so there is no record history to measure"
	}
	out, err := gitOut(root, "log", "--reverse", "--format=%H %ct", "--", decisionsFile)
	if err != nil {
		return nil, 0, fmt.Sprintf("git history for %s is unreadable (%v)", decisionsFile, err)
	}
	lines := strings.Fields(strings.TrimSpace(out))
	if len(lines) < 4 {
		return nil, 0, fmt.Sprintf("%s has too little history to measure (fewer than two commits)", decisionsFile)
	}

	cutoff := now.AddDate(0, 0, -lookbackDays).Unix()
	type seen struct {
		at     int64
		status adr.Status
	}
	first := map[string]seen{}
	answeredAt := map[string]int64{}
	commits := 0

	for i := 0; i+1 < len(lines); i += 2 {
		sha := lines[i]
		ct, convErr := strconv.ParseInt(lines[i+1], 10, 64)
		if convErr != nil || ct < cutoff {
			continue
		}
		blob, blobErr := gitOut(root, "show", sha+":"+decisionsFile)
		if blobErr != nil {
			continue
		}
		commits++
		for _, sec := range adr.ParseSections([]byte(blob)) {
			prev, ok := first[sec.ID]
			if !ok {
				first[sec.ID] = seen{at: ct, status: sec.Status}
				continue
			}
			if _, done := answeredAt[sec.ID]; !done && sec.Status != prev.status {
				answeredAt[sec.ID] = ct
			}
		}
	}

	if commits == 0 {
		return nil, 0, fmt.Sprintf("no commit to %s inside the %d-day lookback", decisionsFile, lookbackDays)
	}
	ids := make([]string, 0, len(answeredAt))
	for id := range answeredAt {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		days = append(days, int((answeredAt[id]-first[id].at)/86400))
	}
	if len(days) == 0 {
		return nil, 0, fmt.Sprintf("%d commit(s) to %s in %d days, and NO record's Status ever changed — "+
			"the pre-plane rate of answering a decision is zero, which is itself the baseline",
			commits, decisionsFile, lookbackDays)
	}

	sorted := append([]int(nil), days...)
	sort.Ints(sorted)
	median = sorted[len(sorted)/2]
	return days, median, fmt.Sprintf("%d record(s) answered across %d commit(s) to %s in the last %d days",
		len(days), commits, decisionsFile, lookbackDays)
}

// gitOut runs git in root and returns stdout.
func gitOut(root string, args ...string) (string, error) {
	//fathom:tenancy-allow exec-noctx one-shot CLI verb with no request context; the git/child call ends with the process
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	b, err := cmd.Output()
	return string(b), err
}
