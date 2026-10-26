package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testJournal(t *testing.T, budget int64) (*journal, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "journal")
	j, err := openJournal(path, []byte("study"), budget, func() time.Time { return time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { j.close() })
	return j, path
}

func testLane(amount int64) lane {
	return lane{Provider: "openrouter-jev", Model: "model", PricingSource: "synthetic test", MaxInputTokens: amount, InputRate: 1_000_000, ReserveInputRate: 1_000_000}
}

func TestJournalUnknownUsageCostsReservation(t *testing.T) {
	j, path := testJournal(t, 100)
	r, err := j.reserve("a", testLane(70), []byte("request"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = j.reserve("b", testLane(20), []byte("r")); err == nil {
		t.Fatal("pending call admitted")
	}
	if err = j.finish(r, outcome{Status: "failed"}); err != nil {
		t.Fatal(err)
	}
	if j.charged != 70 {
		t.Fatal("unknown charge released")
	}
	if _, err = j.reserve("c", testLane(31), []byte("r")); err == nil {
		t.Fatal("budget overflow admitted")
	}
	if err = j.close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := openJournal(path, []byte("study"), 100, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.close()
	if reopened.charged != 70 {
		t.Fatal("restart forgot failed charge")
	}
}

func TestJournalActualAndBinding(t *testing.T) {
	j, path := testJournal(t, 100)
	r, err := j.reserve("a", testLane(70), []byte("request"))
	if err != nil {
		t.Fatal(err)
	}
	actual := int64(15)
	if err = j.finish(r, outcome{Status: "completed", ActualMicroUSD: &actual}); err == nil {
		t.Fatal("unreported usage priced")
	}
	if err = j.finish(r, outcome{Status: "completed", Model: "model", UsagePresent: true, ActualMicroUSD: &actual, Response: `{"model":"model","usage":{"input_tokens":15,"output_tokens":0}}`}); err != nil {
		t.Fatal(err)
	}
	if j.charged != 15 {
		t.Fatal("actual cost not settled")
	}
	if err = j.finish(r, outcome{Status: "completed"}); err == nil {
		t.Fatal("replay admitted")
	}
	if err = j.close(); err != nil {
		t.Fatal(err)
	}
	if _, err = openJournal(path, []byte("foreign"), 100, time.Now); err == nil {
		t.Fatal("foreign study admitted")
	}
	if _, err = openJournal(path, []byte("study"), 200, time.Now); err == nil {
		t.Fatal("raised budget admitted")
	}
}

func TestJournalCrashAndExclusiveOwner(t *testing.T) {
	j, path := testJournal(t, 100)
	if _, err := openJournal(path, []byte("study"), 100, time.Now); err == nil {
		t.Fatal("concurrent journal owner admitted")
	}
	if _, err := j.reserve("a", testLane(70), []byte("r")); err != nil {
		t.Fatal(err)
	}
	j.close()
	if _, err := openJournal(path, []byte("study"), 100, time.Now); err == nil {
		t.Fatal("crash lock ignored")
	}
	if err := os.Remove(filepath.Join(path, ".lock")); err != nil {
		t.Fatal(err)
	}
	if _, err := openJournal(path, []byte("study"), 100, time.Now); err == nil {
		t.Fatal("unfinished reservation ignored after lock removal")
	}
}

func TestJournalOverrunStopsAndRetains(t *testing.T) {
	j, path := testJournal(t, 100)
	r, err := j.reserve("a", testLane(10), []byte("r"))
	if err != nil {
		t.Fatal(err)
	}
	actual := int64(11)
	if err = j.finish(r, outcome{Status: "completed", Model: "model", UsagePresent: true, ActualMicroUSD: &actual, Response: `{"model":"model","usage":{"input_tokens":11,"output_tokens":0}}`}); err == nil {
		t.Fatal("overrun not surfaced")
	}
	if _, err = j.reserve("b", testLane(1), []byte("r")); err == nil {
		t.Fatal("overrun did not stop dispatch")
	}
	j.close()
	if _, err = os.Stat(filepath.Join(path, ".lock")); err != nil {
		t.Fatal("overrun lock removed")
	}
	if err = os.Remove(filepath.Join(path, ".lock")); err != nil {
		t.Fatal(err)
	}
	if _, err = openJournal(path, []byte("study"), 100, time.Now); err == nil {
		t.Fatal("uncertain overrun reopened")
	}
}

func TestJournalRejectsCorruptOrForeignFiles(t *testing.T) {
	for _, tc := range []struct{ name, contents string }{
		{"outcome-999999.json", `{}`},
		{"reserve-000001.json", `{"version":"x","version":"y"}`},
		{"unexpected.json", `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j, path := testJournal(t, 100)
			j.close()
			if err := os.WriteFile(filepath.Join(path, tc.name), []byte(tc.contents), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := openJournal(path, []byte("study"), 100, time.Now); err == nil {
				t.Fatal("corrupt history admitted")
			}
		})
	}
}

func TestJournalSymlinksAndPermissions(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := openJournal(link, []byte("study"), 10, time.Now); err == nil {
		t.Fatal("symlink journal admitted")
	}
	if err := os.Chmod(target, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := openJournal(target, []byte("study"), 10, time.Now); err == nil {
		t.Fatal("public journal admitted")
	}
}

func TestJournalStorageAndAttemptBoundsBeforeDispatch(t *testing.T) {
	j, _ := testJournal(t, 100)
	j.storedBytes = 64 << 20
	if _, err := j.reserve("full", testLane(10), []byte("r")); err == nil {
		t.Fatal("storage bound ignored")
	}
	j.storedBytes = 0
	j.next = maxJournalAttempts + 1
	if _, err := j.reserve("count", testLane(10), []byte("r")); err == nil {
		t.Fatal("restart-incompatible attempt count admitted")
	}
}
