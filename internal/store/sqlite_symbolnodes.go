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
	rows, err := s.db.QueryContext(ctx, queryObservationsSQL, serviceID)
	if err != nil {
		return nil, fmt.Errorf("QueryObservations query: %w", err)
	}
	defer rows.Close()
	nodes, err := scanSymbolNodes(ctx, rows)
	return filterScoped(ctx, s.gate, serviceID, nodes, err, func(n core.SymbolNode) string { return n.FilePath })
}

type symbolNodeRows interface {
	Next() bool
	Scan(...any) error
	Err() error
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
