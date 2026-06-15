package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
)

// sqlFixture exercises all supported DDL statement types including:
//   - quoted identifiers (double-quote, backtick)
//   - IF NOT EXISTS
//   - schema-qualified names (public.users)
//   - OR REPLACE variants
//   - UNIQUE INDEX
//   - TEMPORARY TABLE
const sqlFixture = `-- Schema for the fauxtown test database

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS "public"."orders" (
    id SERIAL PRIMARY KEY,
    user_id INT
);

CREATE TEMPORARY TABLE temp_staging (
    row_data TEXT
);

CREATE OR REPLACE VIEW active_users AS
    SELECT * FROM users WHERE active = TRUE;

CREATE MATERIALIZED VIEW IF NOT EXISTS order_summary AS
    SELECT user_id, COUNT(*) FROM orders GROUP BY user_id;

CREATE UNIQUE INDEX idx_users_email ON users(email);

CREATE INDEX IF NOT EXISTS idx_orders_user ON orders(user_id);

CREATE OR REPLACE FUNCTION calculate_total(p_order_id INT)
RETURNS NUMERIC AS $$
BEGIN
    RETURN 0;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE PROCEDURE process_order(IN p_id INT)
LANGUAGE plpgsql AS $$
BEGIN
    -- no-op
END;
$$;

CREATE TYPE order_status AS ENUM ('pending', 'shipped', 'delivered');

CREATE TABLE ` + "`" + `schema_b` + "`" + `.` + "`" + `products` + "`" + ` (
    id SERIAL PRIMARY KEY
);
`

func TestSQLExtractor_Basic(t *testing.T) {
	x := extractor.NewSQLExtractor()
	nodes, err := x.Extract("svc", "db/schema.sql", []byte(sqlFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	tests := []struct {
		name string
		kind core.SourceKind
		sig  string
	}{
		{"users", core.KindSQLTable, "CREATE TABLE"},
		{"public.orders", core.KindSQLTable, "CREATE TABLE"},   // IF NOT EXISTS + schema-qualified + double-quoted
		{"temp_staging", core.KindSQLTable, "CREATE TABLE"},    // TEMPORARY TABLE
		{"active_users", core.KindSQLView, "CREATE VIEW"},      // OR REPLACE VIEW
		{"order_summary", core.KindSQLView, "CREATE VIEW"},     // MATERIALIZED VIEW IF NOT EXISTS
		{"idx_users_email", core.KindSQLIndex, "CREATE INDEX"}, // UNIQUE INDEX
		{"idx_orders_user", core.KindSQLIndex, "CREATE INDEX"}, // IF NOT EXISTS INDEX
		{"calculate_total", core.KindSQLFunction, "CREATE FUNCTION"},
		{"process_order", core.KindSQLFunction, "CREATE PROCEDURE"},
		{"order_status", core.KindSQLType, "CREATE TYPE"},
		{"schema_b.products", core.KindSQLTable, "CREATE TABLE"}, // backtick-quoted schema.table
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			n, ok := byName[tc.name]
			if !ok {
				t.Fatalf("expected symbol %q; got nodes: %v", tc.name, nodeNames(nodes))
			}
			if n.Kind != tc.kind {
				t.Errorf("Kind: got %q, want %q", n.Kind, tc.kind)
			}
			if n.Signature != tc.sig {
				t.Errorf("Signature: got %q, want %q", n.Signature, tc.sig)
			}
			if n.Tier != core.TierPreScan {
				t.Errorf("Tier: got %q, want TierPreScan", n.Tier)
			}
			if n.Line <= 0 {
				t.Errorf("Line: got %d, want > 0", n.Line)
			}
			if n.Service != "svc" {
				t.Errorf("Service: got %q, want %q", n.Service, "svc")
			}
		})
	}
}

func TestSQLExtractor_LineNumbers(t *testing.T) {
	// Verify that line numbers correspond to the actual CREATE statement lines.
	src := "-- comment\n\nCREATE TABLE alpha (id INT);\nCREATE VIEW beta AS SELECT 1;\n"
	x := extractor.NewSQLExtractor()
	nodes, err := x.Extract("svc", "schema.sql", []byte(src))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %v", len(nodes), nodeNames(nodes))
	}
	// alpha on line 3, beta on line 4
	if nodes[0].Name != "alpha" || nodes[0].Line != 3 {
		t.Errorf("nodes[0]: got name=%q line=%d, want alpha/3", nodes[0].Name, nodes[0].Line)
	}
	if nodes[1].Name != "beta" || nodes[1].Line != 4 {
		t.Errorf("nodes[1]: got name=%q line=%d, want beta/4", nodes[1].Name, nodes[1].Line)
	}
}

func TestSQLExtractor_Determinism(t *testing.T) {
	x := extractor.NewSQLExtractor()
	n1, err := x.Extract("svc", "schema.sql", []byte(sqlFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	n2, err := x.Extract("svc", "schema.sql", []byte(sqlFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(n1) != len(n2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name || n1[i].Kind != n2[i].Kind || n1[i].Line != n2[i].Line {
			t.Errorf("non-deterministic at [%d]: %+v vs %+v", i, n1[i], n2[i])
		}
	}
}

func TestSQLExtractor_Empty(t *testing.T) {
	x := extractor.NewSQLExtractor()
	nodes, err := x.Extract("svc", "empty.sql", []byte(""))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty input, got %d", len(nodes))
	}
}

func TestSQLExtractor_CommentsOnly(t *testing.T) {
	src := "-- This is a comment\n-- Another comment\n"
	x := extractor.NewSQLExtractor()
	nodes, err := x.Extract("svc", "comments.sql", []byte(src))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for comment-only input, got %d", len(nodes))
	}
}

func TestSQLExtractor_CaseInsensitive(t *testing.T) {
	src := "create table lower_case_tbl (id int);\nCREATE TABLE UPPER_CASE_TBL (id INT);\n"
	x := extractor.NewSQLExtractor()
	nodes, err := x.Extract("svc", "case.sql", []byte(src))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %v", len(nodes), nodeNames(nodes))
	}
	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}
	if _, ok := byName["lower_case_tbl"]; !ok {
		t.Error("expected lower_case_tbl")
	}
	if _, ok := byName["UPPER_CASE_TBL"]; !ok {
		t.Error("expected UPPER_CASE_TBL")
	}
}
