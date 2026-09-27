package console

import (
	"reflect"
	"strings"
	"testing"
)

// TestLedgerFoldCloneIsIndependent (RT-113): a tail hands its reader a
// clone and keeps folding; the clone's answers never change after.
func TestLedgerFoldCloneIsIndependent(t *testing.T) {
	f := NewLedgerFold()
	f.Add([]byte(`{"session_id":"a","repo":"r","at":"2026-09-20T10:00:00Z","arm":"on"}`))
	c := f.Clone()
	lower := strings.ToLower
	wantBy, wantTbl := c.DistinctBy(lower), SessionTableFold("p", c, nil, SessionFilter{}, 1, 50)
	f.Add([]byte(`{"session_id":"b","repo":"r","at":"2026-09-21T10:00:00Z","arm":"off"}`))
	f.Add([]byte(`{"session_id":"a","repo":"s","at":"2026-09-22T10:00:00Z"}`))
	if got := c.DistinctBy(lower); !reflect.DeepEqual(got, wantBy) {
		t.Errorf("clone by repo = %v after the fold grew; want %v", got, wantBy)
	}
	if got := SessionTableFold("p", c, nil, SessionFilter{}, 1, 50); !reflect.DeepEqual(got, wantTbl) {
		t.Errorf("clone table changed after the fold grew: %+v", got)
	}
	if s, rows := f.Sessions(); s != 2 || rows != 3 {
		t.Errorf("fold = %d sessions, %d rows; want 2, 3", s, rows)
	}
}
