package store

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestStatementJournalSpillIsRaisedBeforeFirstOpen (LB-F208): SQLITE_CONFIG_STMTJRNL_SPILL
// only takes effect before SQLite initializes, and this test binary has opened stores in
// other tests, so a fresh process of the same binary opens one and reads the result. The
// child also writes observations whose call lists fire the call-index trigger and reads
// the index back, so the option changes cost, never rows.
func TestStatementJournalSpillIsRaisedBeforeFirstOpen(t *testing.T) {
	if os.Getenv("FATHOM_STMTSPILL_CHILD") == "1" {
		stmtSpillChild(t)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestStatementJournalSpillIsRaisedBeforeFirstOpen$", "-test.v")
	cmd.Env = append(os.Environ(), "FATHOM_STMTSPILL_CHILD=1", "TMPDIR="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "PASS") {
		t.Fatalf("child: %v\n%s", err, out)
	}
}

func stmtSpillChild(t *testing.T) {
	st, err := NewSQLiteStore(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if !stmtSpillApplied {
		t.Fatal("the statement journal spill was not raised before the first connection opened")
	}
	calls := make([]string, 300)
	for i := range calls {
		calls[i] = "pkg.callee" + string(rune('a'+i%26)) + strings.Repeat("x", i%7)
	}
	var obs []core.Observation
	for i := 0; i < 40; i++ {
		obs = append(obs, core.Observation{ServiceID: "svc", ConcernID: "c", SourceRef: "f.go", Symbol: "S" + string(rune('A'+i%26)) + strings.Repeat("y", i/26), Calls: calls})
	}
	if err := st.PutObservations(context.Background(), obs); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := st.db.QueryRow(`SELECT count(*) FROM observation_calls WHERE service_id='svc'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if want := 40 * 300; n != want {
		t.Fatalf("observation_calls rows = %d, want %d", n, want)
	}
}
