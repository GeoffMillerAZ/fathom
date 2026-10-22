package main

import (
	"os"
	"testing"
	"time"
)

func TestAD12AliveMapsReadsLogOnce(t *testing.T) {
	repo := hookMapRepo(t)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	fileMapEntry(t, repo, "live", "g1.go#L3-L3", "handler", "sess-parent", now)
	old := readThreadsFile
	reads := 0
	readThreadsFile = func(path string) ([]byte, error) {
		reads++
		return os.ReadFile(path)
	}
	t.Cleanup(func() { readThreadsFile = old })
	_, alive := loadAliveMaps(repo, now)
	if reads != 1 || len(alive) != 1 || alive[0] != "live" {
		t.Errorf("reads=%d alive=%v, want one snapshot with live map", reads, alive)
	}
}

func TestAD12SpentMapLoadDoesNoRead(t *testing.T) {
	repo := hookMapRepo(t)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	fileMapEntry(t, repo, "live", "g1.go#L3-L3", "handler", "sess-parent", now)
	old := readThreadsFile
	readThreadsFile = func(path string) ([]byte, error) {
		t.Fatal("read after deadline")
		return nil, nil
	}
	t.Cleanup(func() { readThreadsFile = old })
	spentHookGuard(t)
	if set, alive := loadAliveMaps(repo, now); len(set.maps) != 0 || len(alive) != 0 {
		t.Errorf("spent deadline yielded maps: %v", alive)
	}
}

func TestAD12MapLogIndexUsesFirstHeaderAndAppendOrder(t *testing.T) {
	// Equal timestamps and a later restatement cannot move creation past
	// an end note. A resumed session's new header is still alive.
	cond := &threadCondition{Kind: "session_ended", SessionID: "parent"}
	rows := []threadItem{
		{ID: "end-before", Kind: "session-end", Pointers: map[string]string{"session_id": "parent"}},
		{ID: "map:old", Kind: mapHeaderKind},
		{ID: "map:old", Kind: mapHeaderKind, Pointers: map[string]string{"session_id": "parent", "created_by": "creator"}},
		{ID: "end-after", Kind: "session-end", Pointers: map[string]string{"session_id": "parent"}},
		{ID: "map:old", Kind: mapHeaderKind, Pointers: map[string]string{"session_id": "restater"}},
		{ID: "map:new", Kind: mapHeaderKind, Pointers: map[string]string{"session_id": "parent"}},
	}
	index := indexMapLog(rows)
	if index.firstHeader["map:old"] != 1 || index.creators["map:old"].Session != "parent" || index.creators["map:old"].CreatedBy != "creator" {
		t.Errorf("first-header identity lost: %+v", index)
	}
	for _, id := range []string{"map:old", "map:new"} {
		if got := mapConditionHolds(threadItem{ID: id, CompletesWhen: cond}, "", nil, index); got != (id == "map:old") {
			t.Errorf("%s ended = %v", id, got)
		}
	}
}
