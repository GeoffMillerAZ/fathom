package store

// archviolations.go — arch_violation_trend (CX-20,
// docs/plans/change-and-compliance.md; design
// docs/design/change-and-compliance.md §5 step 2, "layer violations over
// time"): at each ingest of a repository with an archconformance rule pack,
// internal/fusion.recordViolationTrend appends one reading — the violation
// count per deny rule, stamped with the ingest's commit, the previous
// reading's commit and the change since it. The trend and "the commits
// that added violations" are then reads over stored rows (§6: the radar's
// counts are an ingest-time fact), never a request-time graph walk.
//
// Same additive-table convention as symbol_centrality (centrality.go): its
// own DDL const and createXSchema. Unlike the per-service replace tables it
// is append-only, bounded to the newest ArchViolationKeep readings per
// service. Its rows carry no file paths (a rule key and a count), so no
// purge location names it and no suppression filter applies.

import (
	"context"
	"fmt"
	"sort"
)

// ArchViolationKeep bounds the readings kept per service: the oldest
// reading is dropped once a service has more.
const ArchViolationKeep = 500

// ArchRuleCount is one deny rule's violation count at one ingest.
type ArchRuleCount struct {
	Rule  string // "from -> to", or "(deny by default)"
	Count int
}

// ArchViolationReading is one arch_violation_trend row: one rule's count at
// one reading.
type ArchViolationReading struct {
	Seq        int64  // the reading's ordinal within the service, ascending
	Commit     string // the ingest's watermark (git HEAD when known)
	PrevCommit string // the previous reading's commit; "" for the first
	RecordedAt int64  // unix nanoseconds
	Rule       string
	Count      int
	// Delta is Count minus the previous reading's count for the same rule;
	// 0 when the rule has no earlier reading (a baseline, never "added").
	Delta int
}

const archViolationTrendDDL = `
CREATE TABLE IF NOT EXISTS arch_violation_trend (
    service_id   TEXT    NOT NULL,
    seq          INTEGER NOT NULL,
    commit_sha   TEXT    NOT NULL DEFAULT '',
    prev_commit  TEXT    NOT NULL DEFAULT '',
    recorded_at  INTEGER NOT NULL DEFAULT 0,
    rule         TEXT    NOT NULL,
    violations   INTEGER NOT NULL DEFAULT 0,
    delta        INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (service_id, seq, rule)
);
`

func (s *SQLiteStore) createArchViolationTrendSchema() error {
	_, err := s.db.Exec(archViolationTrendDDL)
	return err
}

// deltaReadings turns one ingest's counts into readings against prev (the
// service's latest stored reading), sorted by rule.
func deltaReadings(seq int64, commit string, at int64, counts []ArchRuleCount, prev []ArchViolationReading) []ArchViolationReading {
	prevCount := map[string]int{}
	prevCommit := ""
	for _, r := range prev {
		prevCount[r.Rule] = r.Count
		prevCommit = r.Commit
	}
	out := make([]ArchViolationReading, 0, len(counts))
	for _, c := range counts {
		r := ArchViolationReading{Seq: seq, Commit: commit, PrevCommit: prevCommit, RecordedAt: at, Rule: c.Rule, Count: c.Count}
		if n, ok := prevCount[c.Rule]; ok {
			r.Delta = c.Count - n
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rule < out[j].Rule })
	return out
}

// latestReading returns the rows of the newest reading in rs (sorted by
// seq then rule).
func latestReading(rs []ArchViolationReading) []ArchViolationReading {
	if len(rs) == 0 {
		return nil
	}
	last := rs[len(rs)-1].Seq
	i := len(rs)
	for i > 0 && rs[i-1].Seq == last {
		i--
	}
	return rs[i:]
}

// AppendArchViolations records one reading for serviceID and returns it.
func (s *SQLiteStore) AppendArchViolations(ctx context.Context, serviceID, commit string, at int64, counts []ArchRuleCount) ([]ArchViolationReading, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("AppendArchViolations begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var last int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) FROM arch_violation_trend WHERE service_id = ?`, serviceID).Scan(&last); err != nil {
		return nil, fmt.Errorf("AppendArchViolations last: %w", err)
	}
	var prev []ArchViolationReading
	if last > 0 {
		rows, err := tx.QueryContext(ctx, `SELECT seq, commit_sha, rule, violations FROM arch_violation_trend WHERE service_id = ? AND seq = ? ORDER BY rule`, serviceID, last)
		if err != nil {
			return nil, fmt.Errorf("AppendArchViolations prev: %w", err)
		}
		for rows.Next() {
			var r ArchViolationReading
			if err := rows.Scan(&r.Seq, &r.Commit, &r.Rule, &r.Count); err != nil {
				rows.Close()
				return nil, fmt.Errorf("AppendArchViolations prev scan: %w", err)
			}
			prev = append(prev, r)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	out := deltaReadings(last+1, commit, at, counts, prev)
	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO arch_violation_trend (service_id, seq, commit_sha, prev_commit, recorded_at, rule, violations, delta)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, fmt.Errorf("AppendArchViolations prepare: %w", err)
	}
	defer stmt.Close()
	for _, r := range out {
		if _, err := stmt.ExecContext(ctx, serviceID, r.Seq, r.Commit, r.PrevCommit, r.RecordedAt, r.Rule, r.Count, r.Delta); err != nil {
			return nil, fmt.Errorf("AppendArchViolations insert: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM arch_violation_trend WHERE service_id = ? AND seq <= ?`, serviceID, last+1-ArchViolationKeep); err != nil {
		return nil, fmt.Errorf("AppendArchViolations bound: %w", err)
	}
	return out, tx.Commit()
}

// ArchViolationTrend returns serviceID's readings sorted by seq then rule.
func (s *SQLiteStore) ArchViolationTrend(ctx context.Context, serviceID string) ([]ArchViolationReading, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT seq, commit_sha, prev_commit, recorded_at, rule, violations, delta
FROM arch_violation_trend WHERE service_id = ? ORDER BY seq, rule`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("ArchViolationTrend query: %w", err)
	}
	defer rows.Close()
	var out []ArchViolationReading
	for rows.Next() {
		var r ArchViolationReading
		if err := rows.Scan(&r.Seq, &r.Commit, &r.PrevCommit, &r.RecordedAt, &r.Rule, &r.Count, &r.Delta); err != nil {
			return nil, fmt.Errorf("ArchViolationTrend scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AppendArchViolations records one reading for serviceID and returns it.
func (m *MemoryStore) AppendArchViolations(_ context.Context, serviceID, commit string, at int64, counts []ArchRuleCount) ([]ArchViolationReading, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.archViolations == nil {
		m.archViolations = map[string][]ArchViolationReading{}
	}
	all := m.archViolations[serviceID]
	prev := latestReading(all)
	var last int64
	if len(prev) > 0 {
		last = prev[0].Seq
	}
	out := deltaReadings(last+1, commit, at, counts, prev)
	all = append(all, out...)
	floor := last + 1 - ArchViolationKeep
	kept := all[:0:0]
	for _, r := range all {
		if r.Seq > floor {
			kept = append(kept, r)
		}
	}
	m.archViolations[serviceID] = kept
	cp := make([]ArchViolationReading, len(out))
	copy(cp, out)
	return cp, nil
}

// ArchViolationTrend returns serviceID's readings sorted by seq then rule.
func (m *MemoryStore) ArchViolationTrend(_ context.Context, serviceID string) ([]ArchViolationReading, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rs := m.archViolations[serviceID]
	if len(rs) == 0 {
		return nil, nil
	}
	out := make([]ArchViolationReading, len(rs))
	copy(out, rs)
	return out, nil
}
