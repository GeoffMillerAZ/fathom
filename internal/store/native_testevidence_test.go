package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestNativeTestJoinsAreUnknownToLegacySQLReaders(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "native.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sqlStore := st.(*SQLiteStore)
	ctx := context.Background()
	native := TestRun{Suite: "EditMode", RunDigest: "native", NativeRunID: "n1", NativeJSON: `{"native":"binding"}`, Cases: []TestCase{{CaseID: "1", Status: "pass", TestSymbolKey: "svc::Workshop.BowlTests.Fits"}}}
	legacy := TestRun{Suite: "unit", RunDigest: "go", Cases: []TestCase{{CaseID: "1", Status: "pass", TestSymbolKey: "svc::cart.TestAdd"}}}
	if err = st.PutTestRuns(ctx, "svc", []TestRun{native, legacy}); err != nil {
		t.Fatal(err)
	}
	// This is exactly the old reader's joined-case predicate. It must not expose
	// any native key when that reader cannot validate its receipt or whole cohort.
	var count int
	if err = sqlStore.db.QueryRow(`SELECT COUNT(*) FROM test_cases WHERE service_id='svc' AND run_digest='native' AND test_symbol_key!=''`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("old reader sees %d native joins without native validation", count)
	}
	got, err := st.NewestTestCaseRuns(ctx, "svc")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("current reader lost joins %+v", got)
	}
	for _, r := range got {
		if r.RunDigest == "native" && r.NativeJSON == "" {
			t.Fatal("native key escaped binding")
		}
	}
}

func TestNativeTestCaseColumnMigrationPreservesLegacyJoins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.sqlite")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	row := TestRun{Suite: "unit", RunDigest: "legacy", Cases: []TestCase{{CaseID: "1", Status: "pass", TestSymbolKey: "svc::cart.TestAdd"}}}
	if err = st.PutTestRuns(ctx, "svc", []TestRun{row}); err != nil {
		t.Fatal(err)
	}
	sqlStore := st.(*SQLiteStore)
	if _, err = sqlStore.db.Exec(`ALTER TABLE test_cases DROP COLUMN native_symbol_key`); err != nil {
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	got, err := st.NewestTestCaseRuns(ctx, "svc")
	if err != nil || len(got) != 1 || got[0].TestSymbolKey != "svc::cart.TestAdd" || got[0].NativeJSON != "" {
		t.Fatalf("legacy changed %+v %v", got, err)
	}
}

func TestNativeColumnsRemainOptionalForReadOnlyLegacyEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-readonly.sqlite")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err = st.PutTestSuites(ctx, "svc", []TestSuite{{Component: "cart", Suite: "unit"}}); err != nil {
		t.Fatal(err)
	}
	if err = st.PutTestRuns(ctx, "svc", []TestRun{{Suite: "unit", RunDigest: "legacy", Cases: []TestCase{{CaseID: "1", Status: "pass", TestSymbolKey: "svc::cart.TestAdd"}}}}); err != nil {
		t.Fatal(err)
	}
	sqlStore := st.(*SQLiteStore)
	for _, ddl := range []string{`ALTER TABLE test_cases DROP COLUMN native_symbol_key`, `ALTER TABLE test_runs DROP COLUMN native_run_id`, `ALTER TABLE test_runs DROP COLUMN native_json`} {
		if _, err = sqlStore.db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	mounted, err := MountReadOnly(path, LevelL0)
	if err != nil {
		t.Fatal(err)
	}
	defer mounted.Close()
	newest, err := mounted.NewestTestCaseRuns(ctx, "svc")
	if err != nil || len(newest) != 1 || newest[0].Status != "pass" {
		t.Fatalf("legacy newest %+v %v", newest, err)
	}
	history, err := mounted.TestCaseRunsForSymbol(ctx, "svc", "svc::cart.TestAdd", 0)
	if err != nil || len(history) != 1 {
		t.Fatalf("legacy history %+v %v", history, err)
	}
	status, err := mounted.TestSuiteStatuses(ctx, "svc")
	if err != nil || len(status) != 1 || status[0].CasesJoined != 1 {
		t.Fatalf("legacy status %+v %v", status, err)
	}
}
