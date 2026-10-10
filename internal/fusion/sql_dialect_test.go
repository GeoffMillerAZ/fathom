package fusion

import (
	"context"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// sqlDialectCorpus is a small schema: a table with a foreign key to another,
// a view, an index and a function, the shapes both the regex incumbent and the
// dialect read, and the columns only the dialect reads.
func sqlDialectCorpus() map[string]string {
	return map[string]string{
		"db/schema.sql": "CREATE TABLE author (\n  id integer PRIMARY KEY,\n  name text NOT NULL\n);\n\n" +
			"CREATE TABLE book (\n  id integer PRIMARY KEY,\n  author_id integer REFERENCES author (id)\n);\n\n" +
			"CREATE VIEW book_titles AS SELECT id FROM book;\n\n" +
			"CREATE INDEX book_author_idx ON book (author_id);\n\n" +
			"CREATE FUNCTION book_count() RETURNS bigint AS $$ SELECT count(*) FROM book $$ LANGUAGE sql;\n",
	}
}

// TestSQLDialectIsWhatFusionDispatches (wave lf4, item shell; O794): lang-parity
// reads the sql tags dialect MERGE against the regex sqlextract.go on pagila and
// sqitch (docs/bench/readings/lang/parity/sql.json), so .sql is no longer the
// regex's: the hand line in fusion.New is gone and the manifest loop claims it.
func TestSQLDialectIsWhatFusionDispatches(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	e, _, ok, unclassified := p.resolveExtractor("db/schema.sql", []byte(sqlDialectCorpus()["db/schema.sql"]), buildSiblingIndex(nil))
	if !ok || unclassified {
		t.Fatalf("db/schema.sql: ok=%v unclassified=%v", ok, unclassified)
	}
	if ts, isTS := e.(*extractor.TSExtractor); !isTS || ts.Name() != "sql" {
		t.Fatalf(".sql dispatches to %T, want the sql tags dialect", e)
	}
	if got := DispatchTable()[".sql"]; got != "tags dialect `sql`" {
		t.Errorf("DispatchTable[.sql] = %q, want tags dialect `sql`", got)
	}
}

// TestIngest_SQLDialect is the product-reach proof: an ingest stores the
// dialect's tables, view, index, function and columns, and a table's foreign
// key as a call.
func TestIngest_SQLDialect(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, sqlDialectCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "sqlx", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "sqlx", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[string(o.SourceKind)+"|"+o.Symbol] = o
	}
	for _, want := range []string{"SQLTable|author", "SQLTable|book", "SQLView|book_titles", "SQLIndex|book_author_idx", "SQLFunction|book_count", "SQLColumn|author_id"} {
		if _, ok := got[want]; !ok {
			var have []string
			for k := range got {
				have = append(have, k)
			}
			sort.Strings(have)
			t.Errorf("%s not stored; have %q", want, have)
		}
	}
	book := got["SQLTable|book"]
	if len(book.Calls) != 1 || book.Calls[0] != "author" {
		t.Errorf("book calls = %q, want [author] (its foreign key)", book.Calls)
	}
	if book.EndLine != 9 {
		t.Errorf("book ends at line %d, want 9", book.EndLine)
	}
}
