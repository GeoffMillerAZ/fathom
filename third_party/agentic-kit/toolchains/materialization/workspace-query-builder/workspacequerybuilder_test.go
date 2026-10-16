package workspacequerybuilder_test

import (
	"strings"
	"testing"

	wqb "github.com/geoffmilleraz/fathom/third_party/agentic-kit/toolchains/materialization/workspace-query-builder"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

func mustCompile(t *testing.T, q wqb.WorkspaceQuery) string {
	t.Helper()
	sql, err := q.Compile()
	if err != nil {
		t.Fatalf("Compile() unexpected error: %v", err)
	}
	return sql
}

// ─── Determinism ─────────────────────────────────────────────────────────────

func TestCompile_Determinism(t *testing.T) {
	q := wqb.SelectEntity("finding").
		WithJoin("evidence", "finding_id").
		WithJoin("service", "service_id").
		WithField("finding", "id", "").
		WithField("finding", "severity", "").
		WithField("service", "name", "").
		Where(wqb.FilterExpr{Entity: "finding", Field: "severity", Op: wqb.FilterOpGt, Value: "3"}).
		WithLimit(100)

	first := mustCompile(t, q)
	for i := 0; i < 20; i++ {
		got := mustCompile(t, q)
		if got != first {
			t.Fatalf("Compile() is non-deterministic: iteration %d differs\nfirst:\n%s\ngot:\n%s", i+1, first, got)
		}
	}
}

// ─── Multi-dataset join: findings + evidence + service ───────────────────────

func TestCompile_MultiDatasetJoin(t *testing.T) {
	q := wqb.SelectEntity("finding").
		WithJoin("evidence", "finding_id").
		WithJoin("service", "service_id").
		WithField("finding", "id", "").
		WithField("finding", "severity", "").
		WithField("evidence", "description", "").
		WithField("service", "name", "").
		Where(wqb.FilterExpr{Entity: "finding", Field: "severity", Op: wqb.FilterOpGt, Value: "3"}).
		WithLimit(100)

	want := strings.Join([]string{
		"SELECT finding.id, finding.severity, evidence.description, service.name",
		"FROM finding",
		"INNER JOIN evidence ON finding.finding_id = evidence.finding_id",
		"INNER JOIN service ON finding.service_id = service.service_id",
		"WHERE finding.severity > '3'",
		"LIMIT 100",
	}, "\n")

	got := mustCompile(t, q)
	if got != want {
		t.Errorf("Compile() mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

// ─── SELECT * (no explicit field selection) ───────────────────────────────────

func TestCompile_SelectStar(t *testing.T) {
	q := wqb.SelectEntity("finding")

	got := mustCompile(t, q)
	want := "SELECT *\nFROM finding"
	if got != want {
		t.Errorf("Compile() want %q got %q", want, got)
	}
}

// ─── LEFT / RIGHT join kinds ──────────────────────────────────────────────────

func TestCompile_LeftJoin(t *testing.T) {
	q := wqb.SelectEntity("finding").
		WithJoinKind("evidence", "finding_id", wqb.JoinKindLeft)

	sql := mustCompile(t, q)
	if !strings.Contains(sql, "LEFT JOIN evidence ON finding.finding_id = evidence.finding_id") {
		t.Errorf("expected LEFT JOIN, got:\n%s", sql)
	}
}

func TestCompile_RightJoin(t *testing.T) {
	q := wqb.SelectEntity("finding").
		WithJoinKind("evidence", "finding_id", wqb.JoinKindRight)

	sql := mustCompile(t, q)
	if !strings.Contains(sql, "RIGHT JOIN evidence ON finding.finding_id = evidence.finding_id") {
		t.Errorf("expected RIGHT JOIN, got:\n%s", sql)
	}
}

// ─── Field alias ─────────────────────────────────────────────────────────────

func TestCompile_FieldAlias(t *testing.T) {
	q := wqb.SelectEntity("finding").
		WithField("finding", "severity", "sev_level")

	sql := mustCompile(t, q)
	if !strings.Contains(sql, "finding.severity AS sev_level") {
		t.Errorf("expected aliased column, got:\n%s", sql)
	}
}

// ─── Error: no entity ─────────────────────────────────────────────────────────

func TestCompile_ErrorNoEntity(t *testing.T) {
	q := wqb.WorkspaceQuery{}
	_, err := q.Compile()
	if err == nil {
		t.Fatal("expected error for empty query, got nil")
	}
}

// ─── Error: unknown join entity ───────────────────────────────────────────────

func TestValid_ErrorUnknownJoinEntity(t *testing.T) {
	// Manually build a query with a join referencing an entity not in the
	// Entities list — should fail Valid().
	q := wqb.WorkspaceQuery{
		Entities: []string{"finding"},
		Joins: []wqb.JoinSpec{
			{
				LeftEntity:  "finding",
				RightEntity: "ghost", // not in Entities
				LeftKey:     "id",
				RightKey:    "id",
				Kind:        wqb.JoinKindInner,
			},
		},
	}
	if err := q.Valid(); err == nil {
		t.Fatal("expected error for unknown join entity, got nil")
	}
}

// ─── EmitSQL with ReadsetDescriptor ──────────────────────────────────────────

func TestEmitSQL_MultiDataset(t *testing.T) {
	pack := wqb.ParquetQueryPack{
		PackID: "gap-analysis-workspace",
		Descriptor: wqb.ReadsetDescriptor{
			ReadsetID: "rs-001",
			Datasets: []wqb.DatasetEntry{
				{Alias: "finding", Path: "/data/findings.parquet"},
				{Alias: "evidence", Path: "/data/evidence.parquet"},
				{Alias: "service", Path: "/data/service.parquet"},
			},
		},
		Query: wqb.SelectEntity("finding").
			WithJoin("evidence", "finding_id").
			WithJoin("service", "service_id"),
	}

	sql, err := wqb.EmitSQL(pack)
	if err != nil {
		t.Fatalf("EmitSQL() unexpected error: %v", err)
	}

	// Verify read_parquet() calls are present.
	checks := []string{
		"read_parquet('/data/findings.parquet') AS finding",
		"read_parquet('/data/evidence.parquet') AS evidence",
		"read_parquet('/data/service.parquet') AS service",
		"INNER JOIN read_parquet('/data/evidence.parquet') AS evidence ON finding.finding_id = evidence.finding_id",
		"INNER JOIN read_parquet('/data/service.parquet') AS service ON finding.service_id = service.service_id",
	}
	for _, c := range checks {
		if !strings.Contains(sql, c) {
			t.Errorf("EmitSQL() missing fragment %q\nFull SQL:\n%s", c, sql)
		}
	}
}

func TestEmitSQL_MissingDataset(t *testing.T) {
	pack := wqb.ParquetQueryPack{
		PackID: "bad-pack",
		Descriptor: wqb.ReadsetDescriptor{
			Datasets: []wqb.DatasetEntry{
				{Alias: "finding", Path: "/data/findings.parquet"},
				// evidence is missing
			},
		},
		Query: wqb.SelectEntity("finding").WithJoin("evidence", "finding_id"),
	}
	_, err := wqb.EmitSQL(pack)
	if err == nil {
		t.Fatal("expected error for missing dataset alias, got nil")
	}
}

// ─── In-memory Execute ────────────────────────────────────────────────────────

func TestExecute_FilterAndProject(t *testing.T) {
	rows := []map[string]string{
		{"finding.id": "F1", "finding.severity": "5", "finding.notes": "high"},
		{"finding.id": "F2", "finding.severity": "2", "finding.notes": "low"},
		{"finding.id": "F3", "finding.severity": "4", "finding.notes": "mid"},
	}

	q := wqb.SelectFields("finding", "id", "severity").
		Where(wqb.FilterExpr{Entity: "finding", Field: "severity", Op: wqb.FilterOpGt, Value: "3"})

	result, err := wqb.Execute(q, rows)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result.TotalBeforeLimit != 2 {
		t.Errorf("TotalBeforeLimit: want 2, got %d", result.TotalBeforeLimit)
	}
	// notes field should be projected out
	for _, r := range result.Rows {
		if _, ok := r["finding.notes"]; ok {
			t.Errorf("projected row still contains finding.notes: %v", r)
		}
	}
}

func TestExecute_Limit(t *testing.T) {
	rows := make([]map[string]string, 10)
	for i := range rows {
		rows[i] = map[string]string{"finding.id": string(rune('A' + i))}
	}
	q := wqb.SelectEntity("finding").WithLimit(3)
	result, err := wqb.Execute(q, rows)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("want 3 rows after limit, got %d", len(result.Rows))
	}
	if result.TotalBeforeLimit != 10 {
		t.Errorf("TotalBeforeLimit: want 10, got %d", result.TotalBeforeLimit)
	}
}

// ─── In-memory ExecuteJoin ───────────────────────────────────────────────────

func TestExecuteJoin_Inner(t *testing.T) {
	left := []map[string]string{
		{"finding.id": "F1", "finding.finding_id": "fid1"},
		{"finding.id": "F2", "finding.finding_id": "fid2"},
		{"finding.id": "F3", "finding.finding_id": "fid99"}, // no match
	}
	right := []map[string]string{
		{"evidence.evidence_id": "E1", "evidence.finding_id": "fid1"},
		{"evidence.evidence_id": "E2", "evidence.finding_id": "fid2"},
	}
	spec := wqb.JoinSpec{
		LeftEntity:  "finding",
		RightEntity: "evidence",
		LeftKey:     "finding_id",
		RightKey:    "finding_id",
		Kind:        wqb.JoinKindInner,
	}
	result := wqb.ExecuteJoin(spec, left, right)
	if len(result) != 2 {
		t.Errorf("inner join: want 2 rows, got %d", len(result))
	}
}

func TestExecuteJoin_Left(t *testing.T) {
	left := []map[string]string{
		{"finding.id": "F1", "finding.finding_id": "fid1"},
		{"finding.id": "F2", "finding.finding_id": "fid99"}, // no match → nil right
	}
	right := []map[string]string{
		{"evidence.finding_id": "fid1", "evidence.detail": "present"},
	}
	spec := wqb.JoinSpec{
		LeftEntity:  "finding",
		RightEntity: "evidence",
		LeftKey:     "finding_id",
		RightKey:    "finding_id",
		Kind:        wqb.JoinKindLeft,
	}
	result := wqb.ExecuteJoin(spec, left, right)
	if len(result) != 2 {
		t.Errorf("left join: want 2 rows, got %d", len(result))
	}
}

func TestExecuteJoin_Cross(t *testing.T) {
	left := []map[string]string{
		{"l.v": "a"},
		{"l.v": "b"},
	}
	right := []map[string]string{
		{"r.v": "x"},
		{"r.v": "y"},
		{"r.v": "z"},
	}
	spec := wqb.JoinSpec{
		LeftEntity:  "l",
		RightEntity: "r",
		Kind:        wqb.JoinKindCross,
	}
	result := wqb.ExecuteJoin(spec, left, right)
	if len(result) != 6 {
		t.Errorf("cross join: want 6 rows, got %d", len(result))
	}
}

// ─── Immutability: chaining must not mutate the original ─────────────────────

func TestImmutability(t *testing.T) {
	base := wqb.SelectEntity("finding")
	_ = base.WithJoin("evidence", "finding_id").WithLimit(50)

	if len(base.Joins) != 0 {
		t.Errorf("base query was mutated: Joins=%v", base.Joins)
	}
	if base.MaxRows != 0 {
		t.Errorf("base query was mutated: MaxRows=%d", base.MaxRows)
	}
}

// ─── Filter operators in SQL emission ────────────────────────────────────────

func TestCompile_AllFilterOps(t *testing.T) {
	cases := []struct {
		op   wqb.FilterOp
		want string
	}{
		{wqb.FilterOpEq, "finding.status = 'open'"},
		{wqb.FilterOpNeq, "finding.status != 'open'"},
		{wqb.FilterOpLt, "finding.status < 'open'"},
		{wqb.FilterOpLte, "finding.status <= 'open'"},
		{wqb.FilterOpGt, "finding.status > 'open'"},
		{wqb.FilterOpGte, "finding.status >= 'open'"},
		{wqb.FilterOpContains, "finding.status LIKE '%open%'"},
	}
	for _, tc := range cases {
		q := wqb.SelectEntity("finding").
			Where(wqb.FilterExpr{Entity: "finding", Field: "status", Op: tc.op, Value: "open"})
		sql := mustCompile(t, q)
		if !strings.Contains(sql, tc.want) {
			t.Errorf("op %s: want fragment %q in:\n%s", tc.op, tc.want, sql)
		}
	}
}

// ─── SQL injection safety ─────────────────────────────────────────────────────

func TestCompile_SQLInjectionEscaping(t *testing.T) {
	q := wqb.SelectEntity("finding").
		Where(wqb.FilterExpr{
			Entity: "finding", Field: "name", Op: wqb.FilterOpEq, Value: "O'Reilly",
		})
	sql := mustCompile(t, q)
	if strings.Contains(sql, "O'Reilly") && !strings.Contains(sql, "O''Reilly") {
		t.Errorf("single-quote not escaped in SQL: %s", sql)
	}
	if !strings.Contains(sql, "O''Reilly") {
		t.Errorf("expected O''Reilly in SQL, got: %s", sql)
	}
}

// ─── No limit / no filter ─────────────────────────────────────────────────────

func TestCompile_NoLimitNoFilter(t *testing.T) {
	q := wqb.SelectEntity("finding").WithJoin("evidence", "finding_id")
	sql := mustCompile(t, q)
	if strings.Contains(sql, "LIMIT") {
		t.Errorf("LIMIT should be absent when not set: %s", sql)
	}
	if strings.Contains(sql, "WHERE") {
		t.Errorf("WHERE should be absent when no filters: %s", sql)
	}
}

// ─── Exact SQL regression for the canonical three-way join ───────────────────

// TestCompile_ExactSQL_ThreeWayJoin is the primary regression test.  It asserts
// the full SQL string byte-for-byte so any future change to emission order or
// formatting is immediately visible.
func TestCompile_ExactSQL_ThreeWayJoin(t *testing.T) {
	q := wqb.SelectEntity("finding").
		WithJoin("evidence", "finding_id").
		WithJoin("service", "service_id").
		WithField("finding", "id", "").
		WithField("finding", "severity", "").
		WithField("evidence", "description", "").
		WithField("service", "name", "").
		Where(wqb.FilterExpr{Entity: "finding", Field: "severity", Op: wqb.FilterOpGt, Value: "3"}).
		WithLimit(100)

	const want = `SELECT finding.id, finding.severity, evidence.description, service.name
FROM finding
INNER JOIN evidence ON finding.finding_id = evidence.finding_id
INNER JOIN service ON finding.service_id = service.service_id
WHERE finding.severity > '3'
LIMIT 100`

	got := mustCompile(t, q)
	if got != want {
		t.Errorf("exact SQL mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}
