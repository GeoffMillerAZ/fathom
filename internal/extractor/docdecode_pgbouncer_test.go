package extractor

import (
	"reflect"
	"testing"
)

// TestDecodePgBouncerDoc (LB-Q-pgbouncer): the "pgbouncer" decode. The root maps
// a section name to a map of its entries in file order; an entry of [databases],
// [users] or [peers] is a map of the connect string's parameters, an entry of any
// other section (the [pgbouncer] settings) a scalar. Lines are 1-based; a comment
// opens with ";" or "#" at the head of a line; a header written twice extends one
// section; a line before the first header is dropped; a parameter value may be
// single-quoted, with ” for a quote, and "=" may have spaces around it.
func TestDecodePgBouncerDoc(t *testing.T) {
	src := "\xef\xbb\xbf;; comment\r\n" +
		"stray = line\n" +
		"[databases]\r\n" +
		"app = host=db1 port=6433 dbname=appdb auth_dbname=authdb\n" +
		"# another comment\n" +
		"authdb = host=db1 dbname='quo''ted db' connect_query = 'select 1'\n" +
		"* = host=fallback\n" +
		"empty =\n" +
		"\n" +
		"[pgbouncer]\n" +
		"pool_mode = transaction\n" +
		"listen_addr=*\n" +
		"[users]\n" +
		"alice = pool_mode=session max_user_connections=3\n" +
		"[databases]\n" +
		"late = host=db2\n" +
		"[peers]\n" +
		"1 = host=/tmp/pgb1 port=6432\n"
	docs, err := decodeDoc("pgbouncer", []byte(src), false, "")
	if err != nil || len(docs) != 1 {
		t.Fatalf("decode: %v %d", err, len(docs))
	}
	root := docs[0]
	if want := []string{"databases", "pgbouncer", "users", "peers"}; !reflect.DeepEqual(root.keys, want) {
		t.Fatalf("sections %q, want %q", root.keys, want)
	}
	dbs := root.get("databases")
	if want := []string{"app", "authdb", "*", "empty", "late"}; !reflect.DeepEqual(dbs.keys, want) {
		t.Fatalf("databases %q, want %q", dbs.keys, want)
	}
	if dbs.line != 3 || dbs.endLine != 16 {
		t.Errorf("[databases] spans %d-%d, want 3-16", dbs.line, dbs.endLine)
	}
	param := func(db, k string) string {
		if v := dbs.get(db).get(k); v != nil {
			return v.scalar
		}
		return "<none>"
	}
	for _, c := range []struct{ db, k, want string }{
		{"app", "host", "db1"}, {"app", "port", "6433"}, {"app", "dbname", "appdb"}, {"app", "auth_dbname", "authdb"},
		{"authdb", "dbname", "quo'ted db"}, {"authdb", "connect_query", "select 1"},
		{"*", "host", "fallback"}, {"late", "host", "db2"}, {"app", "user", "<none>"},
	} {
		if got := param(c.db, c.k); got != c.want {
			t.Errorf("%s.%s = %q, want %q", c.db, c.k, got, c.want)
		}
	}
	if n := dbs.get("empty"); n == nil || n.kind != docMap || len(n.keys) != 0 || n.line != 8 {
		t.Errorf("empty = %+v, want an empty map on line 8", n)
	}
	if n := dbs.get("app"); n.line != 4 || n.get("host").line != 4 {
		t.Errorf("app lines %d, %d, want 4", n.line, n.get("host").line)
	}
	settings := root.get("pgbouncer")
	if v := settings.get("pool_mode"); v == nil || v.kind != docScalar || v.scalar != "transaction" || v.line != 11 {
		t.Errorf("pool_mode = %+v", v)
	}
	if v := settings.get("listen_addr"); v == nil || v.scalar != "*" {
		t.Errorf("listen_addr = %+v", v)
	}
	if v := root.get("users").get("alice").get("max_user_connections"); v == nil || v.scalar != "3" {
		t.Errorf("alice max_user_connections = %+v", v)
	}
	if v := root.get("peers").get("1").get("port"); v == nil || v.scalar != "6432" {
		t.Errorf("peer 1 port = %+v", v)
	}
}
