package store

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSymbolProjectionCapacityAllocationBound(t *testing.T) {
	s := projectionStore(t, 4096)
	ctx := context.Background()
	unhinted := func() {
		rows, err := s.db.QueryContext(ctx, queryObservationsSQL, "svc")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		projectionSink, err = scanSymbolNodesWithHint(ctx, rows, -1)
		if err != nil {
			t.Fatal(err)
		}
	}
	hinted := func() {
		var err error
		projectionSink, err = s.ListSymbolNodes(ctx, "svc")
		if err != nil {
			t.Fatal(err)
		}
	}
	unhinted()
	hinted()
	baseline := projectionAllocatedBytes(unhinted)
	candidate := projectionAllocatedBytes(hinted)
	t.Logf("unhinted=%d hinted=%d allocated bytes", baseline, candidate)
	if candidate > baseline*85/100 {
		t.Fatalf("capacity reservation allocated %d bytes, bound 85%% of unhinted %d", candidate, baseline)
	}
}

func TestSymbolProjectionCapacityHintParity(t *testing.T) {
	s := projectionStore(t, 129)
	ctx := context.Background()
	want, err := s.ListSymbolNodes(ctx, "svc")
	if err != nil {
		t.Fatal(err)
	}
	for _, hint := range []int{-1, 0, 1, 64, 129, 256} {
		rows, err := s.db.QueryContext(ctx, queryObservationsSQL, "svc")
		if err != nil {
			t.Fatal(err)
		}
		got, err := scanSymbolNodesWithHint(ctx, rows, hint)
		rows.Close()
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("stale hint=%d changes output: %v", hint, err)
		}
	}
	for _, tc := range []struct {
		count int64
		err   error
		want  int
	}{
		{0, nil, 0}, {129, nil, 129}, {-1, nil, -1}, {1, errors.New("count failed"), -1}, {1 << 30, nil, 131072},
	} {
		if got := boundedSymbolNodeCapacity(tc.count, tc.err); got != tc.want {
			t.Fatalf("hint(%d,%v)=%d want %d", tc.count, tc.err, got, tc.want)
		}
	}
}

func TestSymbolProjectionCountIsBounded(t *testing.T) {
	s := projectionStore(t, 0)
	// The virtual corpus exceeds the reservation ceiling without storing it.
	// SQLite stops this recursive producer when the count subquery's LIMIT
	// is reached. An outer clamp after COUNT would return 131075 here.
	query := `WITH RECURSIVE seq(n) AS (VALUES(1) UNION ALL SELECT n+1 FROM seq WHERE n<131075), observations AS (SELECT 'svc' AS service_id FROM seq) ` + symbolNodeCountSQL
	var count int64
	if err := s.db.QueryRowContext(context.Background(), query, "svc").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 131072 {
		t.Fatalf("COUNT scanned beyond bound: %d", count)
	}
	if !strings.Contains(strings.ToUpper(symbolNodeCountSQL), "LIMIT 131072") {
		t.Fatal("count must bound SQL work")
	}
}
