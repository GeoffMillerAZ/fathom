package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// ListSymbolNodes reads the same complete observation rows as QueryObservations,
// directly into their public symbol projection. It avoids retaining an entire
// Observation slice beside the SymbolNode slice. Other Store implementations may
// omit this optional capability; the client retains its existing fallback.
func (s *SQLiteStore) ListSymbolNodes(ctx context.Context, serviceID string) ([]core.SymbolNode, error) {
	// Finish the bounded count before opening rows, including when the
	// store has only one connection. A failed/stale count never limits rows.
	var count int64
	countErr := s.db.QueryRowContext(ctx, symbolNodeCountSQL, serviceID).Scan(&count)
	hint := boundedSymbolNodeCapacity(count, countErr)
	rows, err := s.db.QueryContext(ctx, queryObservationsSQL, serviceID)
	if err != nil {
		return nil, fmt.Errorf("QueryObservations query: %w", err)
	}
	defer rows.Close()
	nodes, err := scanSymbolNodesWithHint(ctx, rows, hint)
	return filterScoped(ctx, s.gate, serviceID, nodes, err, func(n core.SymbolNode) string { return n.FilePath })
}

type symbolNodeRows interface {
	Next() bool
	Scan(...any) error
	Err() error
}

// Bound SQL work as well as reservation. This is a hint, never a row cap.
const symbolNodeCountSQL = `SELECT COUNT(*) FROM (
 SELECT 1 FROM observations WHERE service_id = ? LIMIT 131072
)`

const maxSymbolNodeCapacityHint = 131072

func boundedSymbolNodeCapacity(count int64, err error) int {
	if err != nil || count < 0 {
		return -1
	}
	return int(min(count, maxSymbolNodeCapacityHint))
}

// symbolNodeOrder preserves QueryObservations' stable timestamp ordering without
// storing complete Observations or a second node slice. Equal timestamps retain
// the row order delivered by the unchanged query.
type symbolNodeOrder struct {
	nodes  []core.SymbolNode
	stamps []int64
}

func (s symbolNodeOrder) Len() int           { return len(s.nodes) }
func (s symbolNodeOrder) Less(i, j int) bool { return s.stamps[i] < s.stamps[j] }
func (s symbolNodeOrder) Swap(i, j int) {
	s.nodes[i], s.nodes[j] = s.nodes[j], s.nodes[i]
	s.stamps[i], s.stamps[j] = s.stamps[j], s.stamps[i]
}

func scanSymbolNodesWithHint(ctx context.Context, rows symbolNodeRows, hint int) ([]core.SymbolNode, error) {
	nodes := make([]core.SymbolNode, 0)
	var stamps []int64
	if hint >= 0 {
		hint = min(hint, maxSymbolNodeCapacityHint)
		nodes = make([]core.SymbolNode, 0, hint)
		stamps = make([]int64, 0, hint)
	}
	gov := tenancy.NewRowGovernor(ctx, tenancy.ClassIndex)
	for rows.Next() {
		if err := gov.Row(); err != nil {
			return nil, err
		}
		nodes = append(nodes, core.SymbolNode{})
		n := &nodes[len(nodes)-1]
		var concern, hash, callsJSON string
		var stamp int64
		var generated bool
		// Read every original column, including fields not exposed by
		// ToSymbolNode: row/JSON failures must not disappear in a projection.
		if err := rows.Scan(&n.Service, &concern, &n.FilePath, (*string)(&n.Kind),
			(*string)(&n.Tier), &n.Name, &n.Package, &n.Signature, &hash,
			&callsJSON, &n.Complexity, &n.Line, &n.EndLine, &stamp,
			&n.ToolKind, &generated); err != nil {
			return nil, fmt.Errorf("scanObservations scan: %w", err)
		}
		if err := json.Unmarshal([]byte(callsJSON), &n.Calls); err != nil {
			return nil, fmt.Errorf("scanObservations unmarshal calls: %w", err)
		}
		stamps = append(stamps, stamp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scanObservations rows: %w", err)
	}
	sort.Stable(symbolNodeOrder{nodes: nodes, stamps: stamps})
	return nodes, nil
}
