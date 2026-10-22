package store

// Isolated experiment: production sqlite_symbolnodes.go is unchanged.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

const capacityExperimentMaxHint = 131072
const capacityExperimentChunk = 512

type capacityExperimentItem struct {
	node  core.SymbolNode
	stamp int64
}

// Same projection, row governor, and stable timestamp sort as production.
// hint < 0 falls back to append growth. Chunks keep a fixed scan allocation.
func scanCapacityExperiment(ctx context.Context, rows symbolNodeRows, hint, chunkSize int) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode
	var stamps []int64
	if hint >= 0 {
		nodes = make([]core.SymbolNode, 0, hint)
		stamps = make([]int64, 0, hint)
	} else {
		nodes = make([]core.SymbolNode, 0)
	}
	var chunks [][]capacityExperimentItem
	var chunk []capacityExperimentItem
	gov := tenancy.NewRowGovernor(ctx, tenancy.ClassIndex)
	for rows.Next() {
		if err := gov.Row(); err != nil {
			return nil, err
		}
		var n *core.SymbolNode
		if chunkSize > 0 {
			if len(chunk) == cap(chunk) {
				chunk = make([]capacityExperimentItem, 0, chunkSize)
				chunks = append(chunks, chunk)
			}
			chunk = append(chunk, capacityExperimentItem{})
			chunks[len(chunks)-1] = chunk
			n = &chunk[len(chunk)-1].node
		} else {
			nodes = append(nodes, core.SymbolNode{})
			n = &nodes[len(nodes)-1]
		}
		var concern, hash, callsJSON string
		var stamp int64
		var generated bool
		if err := rows.Scan(&n.Service, &concern, &n.FilePath, (*string)(&n.Kind), (*string)(&n.Tier), &n.Name, &n.Package, &n.Signature, &hash, &callsJSON, &n.Complexity, &n.Line, &n.EndLine, &stamp, &n.ToolKind, &generated); err != nil {
			return nil, fmt.Errorf("scanObservations scan: %w", err)
		}
		if err := json.Unmarshal([]byte(callsJSON), &n.Calls); err != nil {
			return nil, fmt.Errorf("scanObservations unmarshal calls: %w", err)
		}
		if chunkSize > 0 {
			chunk[len(chunk)-1].stamp = stamp
		} else {
			stamps = append(stamps, stamp)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scanObservations rows: %w", err)
	}
	if chunkSize > 0 {
		total := 0
		for _, c := range chunks {
			total += len(c)
		}
		nodes = make([]core.SymbolNode, 0, total)
		stamps = make([]int64, 0, total)
		for _, c := range chunks {
			for _, item := range c {
				nodes = append(nodes, item.node)
				stamps = append(stamps, item.stamp)
			}
		}
	}
	sort.Stable(symbolNodeOrder{nodes: nodes, stamps: stamps})
	return nodes, nil
}

func capacityExperimentHint(count int64, err error) int {
	if err != nil || count < 0 {
		return -1
	}
	if count > capacityExperimentMaxHint {
		count = capacityExperimentMaxHint
	}
	return int(count)
}

func listCapacityExperiment(ctx context.Context, s *SQLiteStore, service, variant string, forcedHint int) ([]core.SymbolNode, error) {
	if variant == "production" {
		return s.ListSymbolNodes(ctx, service)
	}
	hint := -1
	if variant == "count" {
		hint = forcedHint
		if hint < 0 {
			var count int64
			// Count must finish before QueryContext/rows can hold the sole connection.
			err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM observations WHERE service_id = ?", service).Scan(&count)
			hint = capacityExperimentHint(count, err)
		}
	}
	rows, err := s.db.QueryContext(ctx, queryObservationsSQL, service)
	if err != nil {
		return nil, fmt.Errorf("QueryObservations query: %w", err)
	}
	defer rows.Close()
	var nodes []core.SymbolNode
	if variant == "baseline" {
		nodes, err = scanSymbolNodes(ctx, rows)
	} else if variant == "chunks" {
		nodes, err = scanCapacityExperiment(ctx, rows, -1, capacityExperimentChunk)
	} else {
		nodes, err = scanCapacityExperiment(ctx, rows, hint, 0)
	}
	return filterScoped(ctx, s.gate, service, nodes, err, func(n core.SymbolNode) string { return n.FilePath })
}

func capacityExperimentStore(tb testing.TB, n int) *SQLiteStore {
	tb.Helper()
	s, err := NewSQLiteStore(filepath.Join(tb.TempDir(), "capacity.sqlite"))
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = s.Close() })
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		tb.Fatal(err)
	}
	stmt, err := tx.Prepare(`INSERT INTO observations (service_id,concern_id,source_ref,source_kind,evidence_tier,symbol,package,claim,content_hash,calls,complexity,line,end_line,ingested_at,tool_kind,generated) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		tb.Fatal(err)
	}
	for i := 0; i < n; i++ {
		calls := "[]"
		if i%3 == 0 {
			calls = `["pkg.Call","pkg.Helper"]`
		}
		if _, err := stmt.Exec("svc", fmt.Sprintf("concern-%d", i%17), fmt.Sprintf("src/%04d/file.go", i%1500), "GoFunc", "verified", fmt.Sprintf("Name%06d", i), fmt.Sprintf("pkg%d", i%70), fmt.Sprintf("func Name%06d()", i), "hash", calls, i%11, i%400+1, i%400+5, int64(i%11), "go", i%2); err != nil {
			tb.Fatal(err)
		}
	}
	if err := stmt.Close(); err != nil {
		tb.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO observations (service_id,source_ref,source_kind,evidence_tier,symbol,ingested_at) VALUES ('other','secret.go','GoFunc','verified','Foreign',0)`); err != nil {
		tb.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		tb.Fatal(err)
	}
	return s
}

func TestCapacityExperimentParity(t *testing.T) {
	for _, n := range []int{0, 1, 127, 128, 129, 85555} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := capacityExperimentStore(t, n)
			ctx := context.Background()
			original, err := s.QueryObservations(ctx, "svc", "")
			if err != nil {
				t.Fatal(err)
			}
			want := make([]core.SymbolNode, 0, len(original))
			for _, o := range original {
				want = append(want, o.ToSymbolNode())
			}
			for _, variant := range []string{"baseline", "production", "count", "chunks"} {
				got, err := listCapacityExperiment(ctx, s, "svc", variant, -1)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("%s/%d differs: %v", variant, n, err)
				}
			}
			// A concurrent count may be stale. Hint size never bounds the rows.
			for _, hint := range []int{0, 1, n / 2} {
				got, err := listCapacityExperiment(ctx, s, "svc", "count", hint)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("stale hint %d/%d: %v", hint, n, err)
				}
			}
		})
	}
}

func TestCapacityExperimentErrorAndScope(t *testing.T) {
	if capacityExperimentHint(85555, errors.New("count failed")) != -1 || capacityExperimentHint(-1, nil) != -1 || capacityExperimentHint(1<<30, nil) != capacityExperimentMaxHint {
		t.Fatal("count hint fallback/clamp changed")
	}
	if boundedSymbolNodeCapacity(0, nil) != 0 || boundedSymbolNodeCapacity(1, nil) != 1 || boundedSymbolNodeCapacity(85555, nil) != 85555 || boundedSymbolNodeCapacity(1<<30, nil) != maxSymbolNodeCapacityHint || boundedSymbolNodeCapacity(1, errors.New("count failed")) != -1 {
		t.Fatal("production count hint fallback/clamp changed")
	}
	s := capacityExperimentStore(t, 129)
	ctx := context.Background()
	for _, v := range []string{"production", "count", "chunks"} {
		empty, err := listCapacityExperiment(ctx, s, "other", v, -1)
		if err != nil || len(empty) != 1 || empty[0].Name != "Foreign" {
			t.Fatalf("tenant %s: %v %v", v, empty, err)
		}
	}
	if _, err := s.db.Exec(`UPDATE observations SET calls='{' WHERE service_id='svc' AND symbol='Name000000'`); err != nil {
		t.Fatal(err)
	}
	_, baseErr := listCapacityExperiment(ctx, s, "svc", "baseline", -1)
	for _, v := range []string{"production", "count", "chunks"} {
		got, err := listCapacityExperiment(ctx, s, "svc", v, -1)
		if baseErr == nil || err == nil || err.Error() != baseErr.Error() || got != nil {
			t.Fatalf("malformed %s: %v %v", v, got, err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, baseErr = listCapacityExperiment(canceled, s, "svc", "baseline", -1)
	for _, v := range []string{"production", "count", "chunks"} {
		got, err := listCapacityExperiment(canceled, s, "svc", v, -1)
		if baseErr == nil || err == nil || err.Error() != baseErr.Error() || got != nil {
			t.Fatalf("cancel %s: %v %v", v, got, err)
		}
	}
}

var capacityExperimentSink []core.SymbolNode

func BenchmarkCapacityExperiment(b *testing.B) {
	s := capacityExperimentStore(b, 85555)
	ctx := context.Background()
	for _, v := range []string{"baseline", "production", "count", "chunks"} {
		b.Run(v, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var err error
				capacityExperimentSink, err = listCapacityExperiment(ctx, s, "svc", v, -1)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
