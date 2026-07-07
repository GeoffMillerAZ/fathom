package sqlmap_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/sqlmap"
)

const basicSrc = `package repo

import "database/sql"

func getUsers(db *sql.DB) {
	_ = db.QueryRow("SELECT id FROM users WHERE active = true")
}

func createOrder(db *sql.DB) {
	db.Exec("INSERT INTO orders (user_id, amount) VALUES (?, ?)", 1, 100)
}

func updateProduct(db *sql.DB) {
	db.Exec("UPDATE products SET price = ? WHERE id = ?", 9.99, 42)
}

func listItems(db *sql.DB) {
	db.Query("SELECT * FROM items JOIN categories ON items.cat_id = categories.id")
}
`

const multiLineSrc = `package repo

const q1 = ` + "`" + `SELECT a, b
FROM accounts
WHERE deleted = false` + "`" + `

const q2 = ` + "`" + `SELECT *
FROM orders
JOIN order_lines ON orders.id = order_lines.order_id` + "`" + `
`

const noSQLSrc = `package foo

func nothing() {
	x := 42
	_ = x
}
`

const duplicateSrc = `package repo

func f() {
	_ = "SELECT id FROM users WHERE id = 1"
	_ = "SELECT name FROM users WHERE active = 1"
}
`

// ─── Extract ──────────────────────────────────────────────────────────────────

func TestExtract_BasicTables(t *testing.T) {
	got := sqlmap.Extract("repo.go", basicSrc)

	// Collect just the table names for a human-readable assertion.
	names := tableNames(got)

	wantNames := map[string]bool{
		"users":      true,
		"orders":     true,
		"products":   true,
		"items":      true,
		"categories": true,
	}
	for _, n := range names {
		if !wantNames[n] {
			t.Errorf("unexpected table name %q in %v", n, names)
		}
	}
	for want := range wantNames {
		if !contains(names, want) {
			t.Errorf("expected table name %q not found in %v", want, names)
		}
	}
}

func TestExtract_NoSQL(t *testing.T) {
	got := sqlmap.Extract("foo.go", noSQLSrc)
	if len(got) != 0 {
		t.Errorf("Extract(noSQL) = %v, want empty", got)
	}
}

func TestExtract_FileAndLine(t *testing.T) {
	got := sqlmap.Extract("repo.go", basicSrc)
	for _, tbl := range got {
		if tbl.File != "repo.go" {
			t.Errorf("Table.File = %q, want %q", tbl.File, "repo.go")
		}
		if tbl.Line < 1 {
			t.Errorf("Table.Line = %d, want ≥1", tbl.Line)
		}
	}
}

func TestExtract_Sorted(t *testing.T) {
	got := sqlmap.Extract("repo.go", basicSrc)
	for i := 1; i < len(got); i++ {
		a, b := got[i-1], got[i]
		if a.File > b.File || (a.File == b.File && a.Line > b.Line) ||
			(a.File == b.File && a.Line == b.Line && a.Name > b.Name) {
			t.Errorf("results not sorted at index %d: %+v, %+v", i, a, b)
		}
	}
}

func TestExtract_Determinism(t *testing.T) {
	a := sqlmap.Extract("repo.go", basicSrc)
	b := sqlmap.Extract("repo.go", basicSrc)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Extract not deterministic:\n  %v\n  %v", a, b)
	}
}

func TestExtract_MultiLine(t *testing.T) {
	got := sqlmap.Extract("q.go", multiLineSrc)
	names := tableNames(got)
	for _, want := range []string{"accounts", "orders", "order_lines"} {
		if !contains(names, want) {
			t.Errorf("expected table %q, not found in %v", want, names)
		}
	}
}

func TestExtract_DuplicateTable_SameLine(t *testing.T) {
	// users appears twice but on different lines.
	got := sqlmap.Extract("dup.go", duplicateSrc)
	names := tableNames(got)
	count := 0
	for _, n := range names {
		if n == "users" {
			count++
		}
	}
	// Two distinct lines, so two Table entries for "users" are expected.
	if count != 2 {
		t.Errorf("expected 2 entries for 'users' (different lines), got %d; tables=%v", count, got)
	}
}

func TestExtract_EmptySource(t *testing.T) {
	got := sqlmap.Extract("empty.go", "")
	if len(got) != 0 {
		t.Errorf("Extract('') = %v, want empty", got)
	}
}

func TestExtract_JoinVariants(t *testing.T) {
	src := `SELECT * FROM orders LEFT JOIN customers ON orders.cid = customers.id`
	got := sqlmap.Extract("q.sql", src)
	names := tableNames(got)
	if !contains(names, "orders") {
		t.Errorf("expected 'orders' in %v", names)
	}
	if !contains(names, "customers") {
		t.Errorf("expected 'customers' in %v", names)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func tableNames(tables []sqlmap.Table) []string {
	names := make([]string, len(tables))
	for i, t := range tables {
		names[i] = t.Name
	}
	return names
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
