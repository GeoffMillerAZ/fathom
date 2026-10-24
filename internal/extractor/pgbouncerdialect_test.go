package extractor

// pgbouncerdialect_test.go — goldens for the pgbouncer doc-engine dialect
// (LB-Q-pgbouncer): a pgbouncer.ini read as sections, databases, peers, users and
// settings, with the auth_dbname a database or the [pgbouncer] section names as an
// in-file reference to the database it declares. The fixtures are written for the
// dialect (reserved example hosts only, ADR-011) and each golden was written from
// reading its fixture.

import "testing"

// TestPgBouncerDialectGoldens: every fixture's entities and references against
// its golden.
func TestPgBouncerDialectGoldens(t *testing.T) {
	if n := checkObsGoldens(t, "pgbouncer"); n < 3 {
		t.Errorf("%d fixtures, want at least 3", n)
	}
}
