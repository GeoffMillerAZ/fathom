package testevidence

// runtime.go — the attested-run read verify's runtime_pass makes (TC-29;
// I8 and ADR-023 as amended by ADR-067): the newest case observation of
// every joined test symbol in test_runs, loaded once per verify run
// (store.NewestTestCaseRuns, one query), and matched to the test names a
// plan row gives. Read-only: nothing here writes or runs anything.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// RuntimeRuns implements verify.RuntimeEvidence over one service's
// attested runs.
type RuntimeRuns struct {
	byKey  map[string]store.TestCaseRun
	byBare map[string][]string
	now    func() time.Time
}

// LoadRuntimeRuns reads service's newest attested run per joined test
// symbol. now is the clock a run's age is read against (nil: time.Now).
func LoadRuntimeRuns(ctx context.Context, st store.Store, service string, now func() time.Time) (*RuntimeRuns, error) {
	rows, err := st.NewestTestCaseRuns(ctx, service)
	if err != nil {
		return nil, fmt.Errorf("attested runs: %w", err)
	}
	if now == nil {
		now = time.Now
	}
	r := &RuntimeRuns{byKey: make(map[string]store.TestCaseRun, len(rows)), byBare: map[string][]string{}, now: now}
	for _, row := range rows {
		r.byKey[row.TestSymbolKey] = row
		bare := bareName(row.TestSymbolKey)
		r.byBare[bare] = append(r.byBare[bare], row.TestSymbolKey)
	}
	return r, nil
}

// NewestAttestedRuns implements verify.RuntimeEvidence: a canonical key
// ("svc::dir/pkg.TestX") matches its own row; a qualified name
// ("pkg.TestX") matches the joined tests with that bare name in a package
// so named; a bare name matches every joined test with that name. The
// newest of the matches answers.
func (r *RuntimeRuns) NewestAttestedRuns(tests []string) (map[string]verify.AttestedRun, error) {
	out := map[string]verify.AttestedRun{}
	for _, t := range tests {
		keys := []string{t}
		if !strings.Contains(t, "::") {
			keys = r.byBare[bareName(t)]
			if q, _, ok := strings.Cut(t, "."); ok {
				keys = inPackage(keys, q)
			}
		}
		var best store.TestCaseRun
		found := false
		for _, k := range keys {
			row, ok := r.byKey[k]
			if !ok {
				continue
			}
			if !found || newer(row, best) {
				best, found = row, true
			}
		}
		if found {
			out[t] = attested(best)
		}
	}
	return out, nil
}

// Now implements verify.RuntimeEvidence.
func (r *RuntimeRuns) Now() time.Time { return r.now() }

// newer orders two runs the way store.TestCaseRunsForSymbol does: the
// later StartedAt, then the later ingest.
func newer(a, b store.TestCaseRun) bool {
	if a.StartedAt != b.StartedAt {
		return a.StartedAt > b.StartedAt
	}
	if a.IngestedAt != b.IngestedAt {
		return a.IngestedAt > b.IngestedAt
	}
	return a.TestSymbolKey < b.TestSymbolKey
}

// attested maps a stored case observation onto verify's AttestedRun. Its
// time is the envelope's own timing.startedAt, else when it was ingested.
func attested(row store.TestCaseRun) verify.AttestedRun {
	var at time.Time
	if t, err := time.Parse(time.RFC3339, row.StartedAt); err == nil {
		at = t
	} else if row.IngestedAt > 0 {
		at = time.Unix(0, row.IngestedAt)
	}
	return verify.AttestedRun{
		Test: row.TestSymbolKey, Status: row.Status, Attempts: row.Attempts,
		Commit: row.Commit, Dirty: row.Dirty, At: at, Run: row.RunDigest, Suite: row.Suite,
	}
}

// inPackage keeps the keys whose package path is pkg or ends in /pkg.
func inPackage(keys []string, pkg string) []string {
	var out []string
	for _, k := range keys {
		p := k
		if i := strings.LastIndex(p, "::"); i >= 0 {
			p = p[i+2:]
		}
		if i := strings.LastIndex(p, "."); i >= 0 {
			p = p[:i]
		}
		if p == pkg || strings.HasSuffix(p, "/"+pkg) {
			out = append(out, k)
		}
	}
	return out
}

// bareName reduces a canonical key or a qualified name to the test's bare
// name: "svc::dir/pkg.TestX" and "pkg.TestX" are both "TestX".
func bareName(name string) string {
	if i := strings.LastIndex(name, "::"); i >= 0 {
		name = name[i+2:]
	}
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	return name
}

var _ verify.RuntimeEvidence = (*RuntimeRuns)(nil)
