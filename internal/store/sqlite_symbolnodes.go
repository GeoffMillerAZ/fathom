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
	// This indexed count is only a bounded capacity hint. Finish it before
	// opening rows: a store with one available connection must not deadlock.
	var count int64
	countErr := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM observations WHERE service_id = ?`, serviceID).Scan(&count)
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

// A stale count can be smaller than the actual result; append still reads all
// rows. Cap the eager backing array at about 22 MiB on 64-bit platforms.
const maxSymbolNodeCapacityHint = 131072

func boundedSymbolNodeCapacity(count int64, err error) int {
	if err != nil || count < 0 {
		return -1
	}
	if count > maxSymbolNodeCapacityHint {
		count = maxSymbolNodeCapacityHint
	}
	return int(count)
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

func scanSymbolNodes(ctx context.Context, rows symbolNodeRows) ([]core.SymbolNode, error) {
	nodes := make([]core.SymbolNode, 0)
	var stamps []int64
	return scanSymbolNodesInto(ctx, rows, nodes, stamps)
}

func scanSymbolNodesWithHint(ctx context.Context, rows symbolNodeRows, hint int) ([]core.SymbolNode, error) {
	if hint < 0 {
		return scanSymbolNodes(ctx, rows)
	}
	return scanSymbolNodesInto(ctx, rows, make([]core.SymbolNode, 0, hint), make([]int64, 0, hint))
}

func scanSymbolNodesInto(ctx context.Context, rows symbolNodeRows, nodes []core.SymbolNode, stamps []int64) ([]core.SymbolNode, error) {
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
