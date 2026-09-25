package main

// observatory_story_dedupe_test.go — the week story counted session-ledger
// ROWS as sessions and listed the first eight rows' ids: Claude Code's Stop
// fires at every turn, so one long session wrote many rows and the live
// page read "1,053 sessions" (for far fewer) with the SAME id listed eight
// times (survey 2026-09-25). The impact page already deduped, keeping the
// last row per session; the story must answer the same way.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildWeekStory_OneSessionPerIDNewestFirst(t *testing.T) {
	dir := hermeticObsEnv(t)
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	rows := []string{
		// "long" stops five times; only its LAST row's figures count.
		`{"schema_version":2,"session_id":"long","at":"2026-09-20T10:00:00Z","verify_catches":1}`,
		`{"schema_version":2,"session_id":"long","at":"2026-09-20T10:05:00Z","verify_catches":1}`,
		`{"schema_version":2,"session_id":"early","at":"2026-09-20T11:00:00Z"}`,
		`{"schema_version":2,"session_id":"long","at":"2026-09-20T12:00:00Z","verify_catches":2}`,
		`{"schema_version":2,"session_id":"long","at":"2026-09-21T09:00:00Z","verify_catches":3,"first_edit_turn":4}`,
		`{"schema_version":2,"session_id":"late","at":"2026-09-22T09:00:00Z","first_edit_turn":2}`,
		`{"schema_version":2,"session_id":"long","at":"2026-09-23T09:00:00Z","verify_catches":3,"first_edit_turn":4}`,
		// before the window: never counted, never listed
		`{"schema_version":2,"session_id":"old","at":"2026-09-01T09:00:00Z","verify_catches":7}`,
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ws := buildWeekStory(now)
	if ws.Sessions != 3 {
		t.Errorf("Sessions = %d, want 3 distinct sessions (long, early, late) — not 7 ledger rows", ws.Sessions)
	}
	if ws.LedgerRows != 7 {
		t.Errorf("LedgerRows = %d, want 7 (the in-window rows the sessions were deduped from)", ws.LedgerRows)
	}
	if ws.Catches != 3 || ws.CatchesRan != 1 {
		t.Errorf("Catches/CatchesRan = %d/%d, want 3/1 — long's last row, counted once", ws.Catches, ws.CatchesRan)
	}
	if ws.EditSessions != 2 {
		t.Errorf("EditSessions = %d, want 2 (long, late)", ws.EditSessions)
	}
	want := []string{"long", "late", "early"} // by each session's latest row, newest first
	if strings.Join(ws.RecentSessions, ",") != strings.Join(want, ",") {
		t.Errorf("RecentSessions = %v, want %v (distinct ids, newest first)", ws.RecentSessions, want)
	}
}

func TestBuildWeekStory_RecentSessionsCappedAtEightDistinct(t *testing.T) {
	dir := hermeticObsEnv(t)
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	var rows []string
	for i := 0; i < 12; i++ {
		id := string(rune('a' + i))
		at := time.Date(2026, 9, 20, 0, i, 0, 0, time.UTC).Format(time.RFC3339)
		// every session stops twice
		rows = append(rows, `{"schema_version":2,"session_id":"`+id+`","at":"`+at+`"}`, `{"schema_version":2,"session_id":"`+id+`","at":"`+at+`"}`)
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := buildWeekStory(now)
	if ws.Sessions != 12 {
		t.Errorf("Sessions = %d, want 12", ws.Sessions)
	}
	if got := strings.Join(ws.RecentSessions, ","); got != "l,k,j,i,h,g,f,e" {
		t.Errorf("RecentSessions = %s, want the eight newest distinct ids l..e", got)
	}
}
