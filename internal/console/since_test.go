package console

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestSinceLastLook_FromLedgers pins OC-18: "since you last looked" lists
// pushes, deploys, measurements and restarts read from the ledgers that
// recorded them — the page-view log for the last look, a remote-tracking
// reflog, the call log's binary stamps, the daemon's own record and the
// commits that added bench readings — newest first, each marked new when it
// is newer than the last look.
func TestSinceLastLook_FromLedgers(t *testing.T) {
	now := time.Date(2026, 9, 25, 20, 0, 0, 0, time.UTC)

	// The page-view log: two views of Now, one of Fleet, and the view being
	// rendered right now (not its own last look).
	views := []byte(`{"schema_version":1,"at":"2026-09-25T09:00:00Z","route":"/{$}","page":"home"}
{"schema_version":1,"at":"2026-09-25T11:00:00Z","route":"/{$}","page":"home"}
{"schema_version":1,"at":"2026-09-25T19:00:00Z","route":"/fleet","page":"fleet"}
{"schema_version":1,"at":"2026-09-25T20:00:00Z","route":"/{$}","page":"home"}
`)
	last, ok := LastLook(views, "home", now)
	if !ok || !last.Equal(time.Date(2026, 9, 25, 11, 0, 0, 0, time.UTC)) {
		t.Fatalf("last look = %s %v, want the previous view of Now (11:00)", last, ok)
	}

	// The remote-tracking reflog: a push from here, a push fetched from
	// elsewhere, a no-op fetch, the ref's creation, and one too old.
	reflog := []byte(strings.Join([]string{
		"0000000000000000000000000000000000000000 1111111111111111111111111111111111111111 G <g@x> 1780000000 -0700\tfetch: storing head",
		"1111111111111111111111111111111111111111 2222222222222222222222222222222222222222 G <g@x> " + unix("2026-09-25T12:30:00Z") + " -0700\tupdate by push",
		"2222222222222222222222222222222222222222 2222222222222222222222222222222222222222 G <g@x> " + unix("2026-09-25T13:00:00Z") + " -0700\tfetch -q origin: fast-forward",
		"2222222222222222222222222222222222222222 3333333333333333333333333333333333333333 G <g@x> " + unix("2026-09-25T10:00:00Z") + " -0700\tfetch -q origin: fast-forward",
		"3333333333333333333333333333333333333333 4444444444444444444444444444444444444444 G <g@x> " + unix("2026-09-01T10:00:00Z") + " -0700\tupdate by push",
		"",
	}, "\n"))
	subjects := map[string]string{"2222222222222222222222222222222222222222": "feat: the attention queue"}
	pushes := PushChanges("fathom", "origin/main", ParseReflog(reflog), func(c string) string { return subjects[c] })
	if len(pushes) != 3 {
		t.Fatalf("pushes = %d, want the push, the fetched push and the old push (no-op fetch and ref creation dropped)", len(pushes))
	}

	// The call log: the log's first binary (not a deploy), a CLI-written
	// row, then two binaries the daemon began serving, one flip back.
	callLog := `{"ts":"2026-09-20T00:00:00Z","tool":"a","fathom_version":"aaaaaaaaaaaa"}
{"ts":"2026-09-25T08:00:00Z","tool":"review","surface":"cli","fathom_version":"cccccccccccc"}
{"ts":"2026-09-25T09:30:00Z","tool":"b","fathom_version":"bbbbbbbbbbbb"}
{"ts":"2026-09-25T09:31:00Z","tool":"c","fathom_version":"aaaaaaaaaaaa"}
{"ts":"2026-09-25T15:05:00Z","tool":"d","fathom_version":"dddddddddddd"}
{"ts":"2026-09-25T15:06:00Z","tool":"e","fathom_version":"dddd`
	var scan DeployScan
	// Fed in two pieces, split mid-row: the scan resumes where it stopped.
	scan.Feed([]byte(callLog[:170]))
	scan.Feed([]byte(callLog[170:] + "dddddddd\"}\n"))
	deploys := DeployChanges(scan, "call log")
	if len(deploys) != 2 || deploys[0].Ref != "bbbbbbbbbbbb" || deploys[1].Ref != "dddddddddddd" {
		t.Fatalf("deploys = %+v, want b then d (the log's first binary and a CLI row are not deploys)", deploys)
	}
	// The daemon record: started with binary d at 15:00 — d's deploy is
	// dated by the start, so there is no separate restart.
	rec := DaemonRecord{SchemaVersion: 1, Addr: "127.0.0.1:7581", PID: 42, BinaryCommit: "dddddddddddd0123456789012345678901234567", StartedAt: "2026-09-25T15:00:00Z"}
	restart, deploys := DaemonStart(rec, deploys, "daemon record")
	if restart != nil || deploys[1].At != "2026-09-25T15:00:00Z" || !strings.Contains(deploys[1].Detail, "pid 42") {
		t.Fatalf("a deploy's own start is not a restart: restart %v, deploy %+v", restart, deploys[1])
	}
	// A later start of the same binary is a restart.
	rec.StartedAt = "2026-09-25T18:00:00Z"
	restart, _ = DaemonStart(rec, DeployChanges(scan, "call log"), "daemon record")
	if restart == nil || restart.Kind != ChangeRestart || restart.At != "2026-09-25T18:00:00Z" {
		t.Fatalf("restart = %+v", restart)
	}

	// Bench readings, from `git log --diff-filter=A --name-only`.
	gitLog := []byte("\x00abcdef1234567890\t2026-09-25T14:00:00-07:00\n\ndocs/bench/readings/cv-m14.md\ndocs/bench/readings/cv-m14-method/run.sh\ndocs/bench/readings/README.md\n" +
		"\x00fedcba9876543210\t2026-09-24T10:00:00Z\n\ndocs/measurements/2026-09-24-latency.md\n")
	readings := ParseReadings("fathom", gitLog, func(p string) string {
		if strings.HasSuffix(p, "cv-m14.md") {
			return "CV-M14 PASS — 6 of 6 cases"
		}
		return ""
	})
	if len(readings) != 2 || readings[0].Detail != "fathom · CV-M14 PASS — 6 of 6 cases" || readings[0].At != "2026-09-25T21:00:00Z" {
		t.Fatalf("readings = %+v", readings)
	}

	changes := append(append(append(pushes, deploys...), readings...), *restart)
	s := SinceLastLook(SinceInputs{
		Now: now, LastLook: last, Changes: changes,
		Sources: []SourceRead{{Name: "page views", State: StatePresent}, {Name: "call log", State: StatePresent}, {Name: "reflog", State: StatePresent}},
	})
	// The reading dated 21:00 UTC is in the future of "now" — never listed;
	// the push of 09-01 is outside the week.
	var got []string
	for _, c := range s.Items {
		mark := ""
		if c.New {
			mark = "*"
		}
		got = append(got, string(c.Kind)+"@"+c.At[11:16]+mark)
	}
	want := "restart@18:00*,deploy@15:00*,push@12:30*,push@10:00,deploy@09:30,measurement@10:00"
	if strings.Join(got, ",") != want {
		t.Errorf("since = %s\nwant    %s", strings.Join(got, ","), want)
	}
	if s.New != 3 || s.LastLook != "2026-09-25T11:00:00Z" || s.State != StatePresent || s.From != "2026-09-18T20:00:00Z" {
		t.Errorf("since: new %d, last look %s, state %s, from %s", s.New, s.LastLook, s.State, s.From)
	}
	for _, c := range s.Items {
		if c.Kind == ChangePush && c.Ref == "2222222222222222222222222222222222222222" && (c.Title != "Pushed" || !strings.Contains(c.Detail, "feat: the attention queue") || !strings.Contains(c.Detail, "pushed from this clone")) {
			t.Errorf("the push from here = %+v", c)
		}
		if c.Kind == ChangePush && c.Ref == "3333333333333333333333333333333333333333" && (c.Title != "Landed" || !strings.Contains(c.Detail, "fetched here")) {
			t.Errorf("the fetched push = %+v", c)
		}
	}

	// A last look older than the week reaches back to it (at most 30 days).
	old := SinceLastLook(SinceInputs{Now: now, LastLook: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Changes: changes, Sources: []SourceRead{{Name: "reflog", State: StatePresent}}})
	if old.From != "2026-09-01T00:00:00Z" || old.Items[len(old.Items)-1].At != "2026-09-01T10:00:00Z" {
		t.Errorf("an old last look: from %s, oldest %s", old.From, old.Items[len(old.Items)-1].At)
	}
	// Never looked: everything is new.
	never := SinceLastLook(SinceInputs{Now: now, Changes: changes, LastLookNote: "no view of Now is recorded", Sources: []SourceRead{{Name: "reflog", State: StatePresent}}})
	if never.New != never.Total || never.LastLook != "" {
		t.Errorf("never looked: %d new of %d", never.New, never.Total)
	}
	// An unreadable ledger: a named lower bound.
	partial := SinceLastLook(SinceInputs{Now: now, Changes: changes, Sources: []SourceRead{{Name: "call log", State: StateError, Reason: "operation not permitted"}}})
	if partial.State != StatePartial || !strings.Contains(partial.Reason, "operation not permitted") {
		t.Errorf("unreadable ledger = %s %q", partial.State, partial.Reason)
	}
	// Nothing readable at all: absent, never an empty "nothing happened".
	if none := SinceLastLook(SinceInputs{Now: now}); none.State != StateAbsent {
		t.Errorf("no ledgers = %s", none.State)
	}
}

func unix(ts string) string {
	t, _ := time.Parse(time.RFC3339, ts)
	return strconv.FormatInt(t.Unix(), 10)
}

// TestSinceLastLook_PagesEveryChangeAndCountsEveryNew: the list is paged,
// never cut — every change in the window is on some page (Page, Pages), the
// new count covers every change and not only the first page's, and a page
// past the end lands on the last one.
func TestSinceLastLook_PagesEveryChangeAndCountsEveryNew(t *testing.T) {
	now := time.Date(2026, 9, 25, 20, 0, 0, 0, time.UTC)
	var changes []Change
	for i := 0; i < 136; i++ {
		changes = append(changes, Change{Kind: ChangePush, At: now.Add(-time.Duration(i+1) * time.Hour).Format(time.RFC3339), Service: "svc", Title: "Pushed", Ref: strconv.Itoa(i)})
	}
	src := []SourceRead{{Name: "git reflogs", State: StatePresent}}
	never := SinceLastLook(SinceInputs{Now: now, Changes: changes, Sources: src, Limit: 50})
	if never.Total != 136 || never.New != 136 || len(never.Items) != 50 || never.Page != 1 || never.Pages != 3 || never.PerPage != 50 {
		t.Errorf("no last look: total %d, new %d, %d items, page %d of %d (per %d)", never.Total, never.New, len(never.Items), never.Page, never.Pages, never.PerPage)
	}
	// A last look 60 h ago: 59 changes are newer, wherever they are listed.
	looked := SinceLastLook(SinceInputs{Now: now, LastLook: now.Add(-60 * time.Hour), Changes: changes, Sources: src, Limit: 50})
	if looked.New != 59 {
		t.Errorf("last look 60 h ago: %d new, want 59 (every newer change, not only the first page's)", looked.New)
	}
	last := SinceLastLook(SinceInputs{Now: now, Changes: changes, Sources: src, Limit: 50, Page: 3})
	if last.Page != 3 || len(last.Items) != 36 || last.Items[0].Ref != "100" || last.Items[35].Ref != "135" {
		t.Errorf("page 3 = page %d, %d items from %s", last.Page, len(last.Items), last.Items[0].Ref)
	}
	if past := SinceLastLook(SinceInputs{Now: now, Changes: changes, Sources: src, Limit: 50, Page: 9}); past.Page != 3 || len(past.Items) != 36 {
		t.Errorf("a page past the end = page %d with %d items", past.Page, len(past.Items))
	}
}
