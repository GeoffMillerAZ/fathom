package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mapClosureRows counts the closure rows the log carries for map id's header.
func mapClosureRows(t *testing.T, id string) int {
	t.Helper()
	n := 0
	for _, r := range loadThreadRowsRaw(threadsPath()) {
		if r.ID == mapHeaderIDPrefix+id && r.ClosedAt != "" && r.Kind == "" {
			n++
		}
	}
	return n
}

// mapListed is `fathom map list --json`'s lifetime per listed map.
func mapListed(t *testing.T, repo string) map[string]string {
	t.Helper()
	code, out, errs := runCLI(t, mapCLI, "list", "--json", "--repo", repo)
	if code != 0 {
		t.Fatalf("list: %s", errs)
	}
	var got struct {
		Maps []struct{ ID, Lifetime string } `json:"maps"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("list json: %v\n%s", err, out)
	}
	life := map[string]string{}
	for _, m := range got.Maps {
		life[m.ID] = m.Lifetime
	}
	return life
}

func setSession(t *testing.T, sid string) {
	t.Helper()
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", sid)
}

// TestMapReadNeverClosesAnotherSessionsMap pins XM-34 (O176): a read closes a
// map on its condition only when the reader created it. Another session's
// show or list evaluates nothing on the creator's behalf: it writes no closure
// row and lists the map alive; the creator's own read closes it. A header
// filed with no session is closed only by a read with no session. A read that
// names an agent id (an append's --agent) is judged by it against the
// header's creator. A map filed under the session-end default ends for every
// reader once its session's end note is in the shared log: its condition is
// the creator's own end, read from the log, never from the reader's tree.
func TestMapReadNeverClosesAnotherSessionsMap(t *testing.T) {
	repo := mapRepo(t) // session sess-parent
	plan := filepath.Join(repo, "docs", "plans", "p.md")
	if err := os.MkdirAll(filepath.Dir(plan), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan, []byte("- [ ] the row {id: R-1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cond := "plan_row_checked:docs/plans/p.md#R-1"
	newMap := func(id string) {
		t.Helper()
		if code, _, errs := runCLI(t, mapCLI, "new", id, "--purpose", "Where is "+id+" handled?", "--completes-when", cond, "--repo", repo); code != 0 {
			t.Fatalf("new %s: %s", id, errs)
		}
		if code, _, errs := runCLI(t, mapCLI, "append", id, "--anchor", "a.go#L3-L5", "--why", "the handler", "--repo", repo); code != 0 {
			t.Fatalf("append %s: %s", id, errs)
		}
	}

	setSession(t, "sess-a")
	newMap("shared")
	setSession(t, "")
	newMap("sessionless")
	setSession(t, "sess-a")
	newMap("by-a")

	// The condition now holds (a worker's clone, or anyone's tree, flips it).
	if err := os.WriteFile(plan, []byte("- [x] the row {id: R-1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("another session reads: no closure row, the map stays alive", func(t *testing.T) {
		setSession(t, "sess-b")
		if code, out, errs := runCLI(t, mapCLI, "show", "shared", "--repo", repo); code != 0 || !strings.Contains(out, "map shared") {
			t.Fatalf("show: exit %d out %q err %q", code, out, errs)
		}
		life := mapListed(t, repo)
		for _, id := range []string{"shared", "sessionless", "by-a"} {
			if n := mapClosureRows(t, id); n != 0 {
				t.Errorf("%s: session B's read wrote %d closure rows", id, n)
			}
			if life[id] != "open" {
				t.Errorf("%s: lifetime %q in %v for session B, want open", id, life[id], life)
			}
		}
	})

	t.Run("a read naming another agent id is not the creator's", func(t *testing.T) {
		setSession(t, "sess-a") // a sub-agent sharing the creator's session
		if code, _, errs := runCLI(t, mapCLI, "append", "by-a", "--anchor", "a.go#L7-L7", "--why", "the other", "--agent", "w1", "--repo", repo); code != 0 {
			t.Fatalf("append --agent: %s", errs)
		}
		if n := mapClosureRows(t, "by-a"); n != 0 {
			t.Errorf("an append by agent w1 closed a map created by session sess-a (%d closure rows)", n)
		}
	})

	t.Run("the creator's read closes its map", func(t *testing.T) {
		setSession(t, "sess-a")
		life := mapListed(t, repo)
		for _, id := range []string{"shared", "by-a"} {
			if n := mapClosureRows(t, id); n != 1 {
				t.Errorf("%s: the creator's read wrote %d closure rows, want 1", id, n)
			}
			if _, listed := life[id]; listed {
				t.Errorf("%s: still listed for its creator after its condition held: %v", id, life)
			}
		}
		if n := mapClosureRows(t, "sessionless"); n != 0 || life["sessionless"] != "open" {
			t.Errorf("a session closed a header filed with no session: %d rows, lifetime %q", n, life["sessionless"])
		}
		// Once closed, the map is ended for every reader.
		setSession(t, "sess-b")
		if life := mapListed(t, repo); life["shared"] != "" {
			t.Errorf("a closed map is still listed for session B: %v", life)
		}
	})

	t.Run("a header with no session is closed by a read with no session", func(t *testing.T) {
		setSession(t, "")
		life := mapListed(t, repo)
		if n := mapClosureRows(t, "sessionless"); n != 1 {
			t.Errorf("a sessionless read wrote %d closure rows for a sessionless header, want 1", n)
		}
		if _, listed := life["sessionless"]; listed {
			t.Errorf("sessionless still listed: %v", life)
		}
	})

	t.Run("a session-end map ends for the next session", func(t *testing.T) {
		setSession(t, "sess-c")
		if code, _, errs := runCLI(t, mapCLI, "append", "scratch", "--anchor", "a.go#L3-L5", "--why", "the handler", "--repo", repo); code != 0 {
			t.Fatalf("append scratch: %s", errs)
		}
		seedItem(t, "se-c", "session-end", "session ended", func(it *threadItem) {
			it.Pointers = map[string]string{"session_id": "sess-c"}
		})
		setSession(t, "sess-d")
		life := mapListed(t, repo)
		if _, listed := life["scratch"]; listed || mapClosureRows(t, "scratch") != 1 {
			t.Errorf("session C's scratch map after C's end: listed %v, %d closure rows; want ended with one row", life, mapClosureRows(t, "scratch"))
		}
	})

	// The creator is the session that filed the first header row: a later
	// restatement by another session (a name, an include) does not make that
	// session the creator.
	t.Run("a restatement does not transfer the creator", func(t *testing.T) {
		if err := os.WriteFile(plan, []byte("- [ ] the row {id: R-1}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		setSession(t, "sess-a")
		newMap("later")
		setSession(t, "sess-b")
		m := loadMapSet(repo, time.Now()).maps["later"]
		if err := writeMapHeader(loadMapSet(repo, time.Now()), m, "", repo, nil, time.Now()); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(plan, []byte("- [x] the row {id: R-1}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_ = mapListed(t, repo)
		if n := mapClosureRows(t, "later"); n != 0 {
			t.Errorf("session B restated the header and then closed a map session A created (%d rows)", n)
		}
	})
}
