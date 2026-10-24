package extractor

// proxysqldialect_test.go — goldens for the proxysql doc-engine dialect
// (LB-Q-proxysql): a proxysql.cnf read as sections, variables, servers, users,
// query rules, replication hostgroups, scheduler entries and cluster peers, with
// the hostgroups the file declares and every server, user, rule and replication
// entry that names one as an in-file reference. The fixtures are written for the
// dialect (reserved example hosts only, ADR-011) and each golden was written from
// reading its fixture.

import "testing"

// TestProxySQLDialectGoldens: every fixture's entities and references against
// its golden.
func TestProxySQLDialectGoldens(t *testing.T) {
	if n := checkObsGoldens(t, "proxysql"); n < 3 {
		t.Errorf("%d fixtures, want at least 3", n)
	}
}
