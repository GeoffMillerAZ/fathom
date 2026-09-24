package store

// memory_testevidence.go — the MemoryStore mirror of
// sqlite_testevidence.go (vision 33 / ADR-043 stage 0, TC-2/TC-5):
// test_suites/test_runs/test_cases and certify_declarations, backed by
// plain maps for tests and session-mode operation.

import (
	"context"
	"sort"
)

// PutTestSuites replaces serviceID's entire declared-suite set — see the
// Store interface doc comment for the wholesale-replace contract.
func (m *MemoryStore) PutTestSuites(_ context.Context, serviceID string, suites []TestSuite) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]TestSuite, len(suites))
	copy(out, suites)
	for i := range out {
		out[i].ServiceID = serviceID
		if out[i].DeclaredCertifiesJSON == "" {
			out[i].DeclaredCertifiesJSON = "[]"
		}
	}
	m.testSuites[serviceID] = out
	return nil
}

// PutTestRuns appends runs, deduplicated by (RunDigest, Suite) — see the
// Store interface doc comment for the append/idempotent-by-digest
// contract.
func (m *MemoryStore) PutTestRuns(_ context.Context, serviceID string, runs []TestRun) error {
	if len(runs) == 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	existing := m.testRuns[serviceID]
	seen := make(map[[2]string]bool, len(existing))
	for _, r := range existing {
		seen[[2]string{r.RunDigest, r.Suite}] = true
	}
	for _, r := range runs {
		key := [2]string{r.RunDigest, r.Suite}
		if seen[key] {
			continue
		}
		seen[key] = true
		r.ServiceID = serviceID
		existing = append(existing, r)
	}
	m.testRuns[serviceID] = existing
	return nil
}

// TestSuiteStatuses returns every declared suite for serviceID with its
// most-recently-ingested run (by StartedAt, ties broken by insertion
// order — the same "last wins" a caller gets from an ORDER BY started_at
// DESC, ingested_at DESC in the SQLite backend) and that run's join
// stats.
func (m *MemoryStore) TestSuiteStatuses(_ context.Context, serviceID string) ([]TestSuiteStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	suites := append([]TestSuite(nil), m.testSuites[serviceID]...)
	sort.Slice(suites, func(i, j int) bool {
		if suites[i].Component != suites[j].Component {
			return suites[i].Component < suites[j].Component
		}
		return suites[i].Suite < suites[j].Suite
	})

	runs := m.testRuns[serviceID]
	out := make([]TestSuiteStatus, 0, len(suites))
	for _, su := range suites {
		status := TestSuiteStatus{TestSuite: su}
		var last *TestRun
		for i := range runs {
			r := runs[i]
			if r.Suite != su.Suite {
				continue
			}
			if last == nil || r.StartedAt >= last.StartedAt {
				rr := r
				last = &rr
			}
		}
		if last != nil {
			status.HasRun = true
			status.LastRun = *last
			status.CasesTotal = len(last.Cases)
			for _, c := range last.Cases {
				if c.TestSymbolKey != "" {
					status.CasesJoined++
				}
			}
		}
		out = append(out, status)
	}
	return out, nil
}

// TestCaseRunsForSymbol returns testSymbolKey's case observations across
// every attested run for serviceID, most-recent-run-first (by StartedAt,
// ties broken by insertion order — the most-recently-INGESTED run wins,
// the same "last wins" TestSuiteStatuses documents) — see the Store
// interface doc comment (vision 33 stage 1, TC-6/TC-8).
func (m *MemoryStore) TestCaseRunsForSymbol(_ context.Context, serviceID, testSymbolKey string, limit int) ([]TestCaseRun, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	runs := m.testRuns[serviceID]
	var out []TestCaseRun
	// Walk runs newest-ingested-first (reverse append order) so that when
	// sort.SliceStable below finds a StartedAt TIE, the run ingested more
	// recently already sits first and the stable sort leaves it there —
	// matching TestSuiteStatuses' `r.StartedAt >= last.StartedAt` scan
	// (which, walking oldest-ingested-first, lets the LAST-appended run
	// win a tie). Iterating forward here would do the opposite: a stable
	// sort on a strict `>` comparator preserves the OLDEST-ingested run
	// on a tie, silently reporting a stale pass over a newer failure.
	for i := len(runs) - 1; i >= 0; i-- {
		r := runs[i]
		for _, c := range r.Cases {
			if c.TestSymbolKey != testSymbolKey {
				continue
			}
			out = append(out, TestCaseRun{
				RunDigest: r.RunDigest, Suite: r.Suite, Component: r.Component,
				StartedAt: r.StartedAt, Status: c.Status, Attempts: c.Attempts,
			})
		}
	}
	// Most-recent-run-first: StartedAt descending, stable so a tie keeps
	// the most-recently-ingested run (already ordered first above).
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartedAt > out[j].StartedAt })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// PutCertifyDeclarations replaces serviceID's entire certify_declarations
// set — see the Store interface doc comment for the wholesale-replace
// contract.
func (m *MemoryStore) PutCertifyDeclarations(_ context.Context, serviceID string, decls []CertifyDeclaration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]CertifyDeclaration, len(decls))
	copy(out, decls)
	for i := range out {
		out[i].ServiceID = serviceID
	}
	m.certifyDeclarations[serviceID] = out
	return nil
}

// CertifyDeclarationsForService returns every declared certification fact
// for serviceID, ordered by (file, line).
func (m *MemoryStore) certifyDeclarationsForServiceRaw(_ context.Context, serviceID string) ([]CertifyDeclaration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := append([]CertifyDeclaration(nil), m.certifyDeclarations[serviceID]...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, nil
}
