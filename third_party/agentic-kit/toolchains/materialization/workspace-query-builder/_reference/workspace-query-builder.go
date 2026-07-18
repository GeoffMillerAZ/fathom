/*
ANNOTATED COPY – toolchains/materialization/workspace-query-builder/query.go
337 lines, 53 branches. The fluent DSL that compiles to deterministic DuckDB SQL.

WHY IT'S COMPLEX

A query builder has to be simultaneously: immutable (each method returns a copy,
never mutating the receiver), correct across all four join types, deterministic in
output, and able to handle both in-memory execution and SQL emission. The same query
goes out to both Execute(q, rows) for in-memory execution (inner/right/cross), filter
operator dispatch (eq/neq/lt/lte/gt/gte/contains), projection, and schema derivation,
explicit fields, implicit fields, entity wildcard.

WHAT IT DOES

WorkspaceQuery is an immutable value type. Every builder method (WithSelect,
WithFilter, WithJoin, WithLimit) calls clone() and returns the copy. No shared state.
This makes it safe to fan out – you can pass the same base query to multiple goroutines
from each without interference.

Two execution modes:

1. Execute(q, rows) – in-memory evaluation.
   Takes a flat []map[string]string (rows keyed by "entity.field").
   Applies filters (AND-joined), then projection, then limit.
   Returns QueryResult{Schema, Rows, TotalBeforeLimit}.
   The schema is derived from the first result row – deterministic if callers
   pass ordered data.

2. EmitSQL(pack) – DuckDB SQL generation (in parquet_query_pack.go, not this file).
   Takes a ParquetQueryPack (descriptor + query), emits a fully-qualified
   DuckDB SQL string with read_parquet() calls, JOINs, WHERE predicates, and LIMIT.

ExecuteJoin(spec, leftRows, rightRows) – in-memory equi-join.
   Implements all four join kinds against two flat row sets.
   Inner: index right on join key, scan left, emit merged rows where key matches.
   Left: same as inner but emit left row with nil right when no match.
   Right: re-indexes left, scans right, emits right row with nil left when no match.
   Cross: nested loop, every left × every right.
   O(n+m) for inner/left/right via hash index; O(n×m) for cross (expected to be rare).

KEY INTERNALS
———————————————————————
buildProjectionKeys(selectors) – converts FieldSelector list to a set of qualified
   "entity.field" keys for O(1) lookup during projectRow. Empty Fields on a selector
   means "all fields from that entity" – represented as a "*:<entity>" sentinel key.

projectRow(row, keys) – copies matching fields from a row. Handles three cases:
   – keys is nil (project all): deep-copy the row
   – key in set: include
   – "*:<entity>" sentinel in set: include if entity prefix matches

matchesFilters(row, filters) – AND-evaluates all FilterExpr against a row.
   Absent fields pass only FilterOpNeq (missing != anything is true).
   applyOp dispatches the 7 filter operators.

qualifyKey(entity, field) – returns "entity.field" unless field already contains
   a dot (already qualified). Used throughout to normalize key formats.

IMMUTABILITY DISCIPLINE
———————————————————————
clone() copies Entities, Select, Joins, Filters slices with append([]T(nil), src...),
This is Go's idiomatic slice-copy – it allocates a new backing array so the
receiver and the returned copy never share memory. Missing this in any one method
would cause subtle aliasing bugs where one caller's filter appears in another's query.
*/

package workspacequerybuilder

import "strings"

// SelectEntity starts a new WorkspaceQuery scoped to the given entity kind.
// Additional entities can be added by chaining WithJoin.
func SelectEntity(entity string) WorkspaceQuery {
	return WorkspaceQuery{
		Entities: []string{entity},
	}
}

// SelectFields starts a new WorkspaceQuery scoped to the given entity kind,
// projecting only the specified field names. When no fields are given, all
// fields are projected (equivalent to SELECT *).
func SelectFields(entity string, fields ...string) WorkspaceQuery {
	sel := FieldSelector{Entity: entity, Fields: append([]string(nil), fields...)}
	return WorkspaceQuery{
		Entities: []string{entity},
		Select:   []FieldSelector{sel},
	}
}

// WithSelect appends a FieldSelector to the query's projection list.
func (q WorkspaceQuery) WithSelect(sel FieldSelector) WorkspaceQuery {
	out := q.clone()
	out.Select = append(out.Select, sel)
	out.Entities = addIfMissing(out.Entities, sel.Entity)
	return out
}

// WithJoin appends a JoinSpec to the query.
// Both LeftEntity and RightEntity are registered in the Entities list if absent.
func (q WorkspaceQuery) WithJoin(spec JoinSpec) WorkspaceQuery {
	out := q.clone()
	out.Joins = append(out.Joins, spec)
	out.Entities = addIfMissing(out.Entities, spec.LeftEntity)
	out.Entities = addIfMissing(out.Entities, spec.RightEntity)
	return out
}

// WithFilter appends a FilterExpr predicate to the query (logical AND).
func (q WorkspaceQuery) WithFilter(expr FilterExpr) WorkspaceQuery {
	out := q.clone()
	out.Filters = append(out.Filters, expr)
	return out
}

// WithLimit sets the maximum number of result rows. Zero means no limit.
func (q WorkspaceQuery) WithLimit(n int) WorkspaceQuery {
	out := q.clone()
	out.Limit = n
	return out
}

// Execute evaluates the WorkspaceQuery against the supplied rows.
// Each row is a map from "entity.field" to string value.
func Execute(q WorkspaceQuery, rows []map[string]string) (QueryResult, error) {
	if err := q.Valid(); err != nil {
		return QueryResult{}, err
	}

	projectKeys := buildProjectionKeys(q.Select)

	filtered := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		if matchesFilters(row, q.Filters) {
			filtered = append(filtered, projectRow(row, projectKeys))
		}
	}

	totalBeforeLimit := len(filtered)
	if q.Limit > 0 && len(filtered) > q.Limit {
		filtered = filtered[:q.Limit]
	}

	schema := deriveSchema(filtered, q.Select)

	return QueryResult{
		Schema:           schema,
		Rows:             filtered,
		TotalBeforeLimit: totalBeforeLimit,
	}, nil
}

// ExecuteJoin performs a simple in-memory equi-join.
func ExecuteJoin(spec JoinSpec, leftRows, rightRows []map[string]string) []map[string]string {

	leftKey := qualifyKey(spec.LeftEntity, spec.LeftKey)
	rightKey := qualifyKey(spec.RightEntity, spec.RightKey)

	index := make(map[string][]map[string]string, len(rightRows))
	for _, row := range rightRows {
		k := row[rightKey]
		index[k] = append(index[k], row)
	}

	var result []map[string]string
// [scan gap: lines 169-179 not captured]
			if len(matches) == 0 {
				result = append(result, mergeRows(lrow, nil))
			} else {
				for _, rrow := range matches {
					rightKeySeen[k] = true
					result = append(result, mergeRows(lrow, rrow))
				}
			}
		}
		_ = rightKeySeen

	case JoinKindRight:
		leftIndex := make(map[string][]map[string]string, len(leftRows))
		for _, row := range leftRows {
			k := row[leftKey]
			leftIndex[k] = append(leftIndex[k], row)
		}
		for _, rrow := range rightRows {
			k := rrow[rightKey]
			matches := leftIndex[k]
			if len(matches) == 0 {
				result = append(result, mergeRows(nil, rrow))
			} else {
				for _, lrow := range matches {
					result = append(result, mergeRows(lrow, rrow))
				}
			}
		}

	case JoinKindCross:
		for _, lrow := range leftRows {
			for _, rrow := range rightRows {
				result = append(result, mergeRows(lrow, rrow))
			}
		}
	}

	return result
}

func (q WorkspaceQuery) clone() WorkspaceQuery {
	out := q
	out.Entities = append([]string(nil), q.Entities...)
	out.Select = append([]FieldSelector(nil), q.Select...)
	out.Joins = append([]JoinSpec(nil), q.Joins...)
	out.Filters = append([]FilterExpr(nil), q.Filters...)
	return out
}

func addIfMissing(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
// [scan gap: lines 235-255 not captured]
		out := make(map[string]string, len(row))
		for k, v := range row {
			out[k] = v
		}
		return out
	}
	out := make(map[string]string)
	for k, v := range row {
		if _, ok := keys[k]; ok {
			out[k] = v
			continue
		}
		entity := entityOf(k)
		if _, ok := keys["*:"+entity]; ok {
			out[k] = v
		}
	}
	return out
}

func matchesFilters(row map[string]string, filters []FilterExpr) bool {
	for _, f := range filters {
		key := qualifyKey(f.Entity, f.Field)
		val, ok := row[key]
		if !ok {
			if f.Op != FilterOpNeq {
				return false
			}
			continue
		}
		if !applyOp(val, f.Op, f.Value) {
			return false
		}
	}
	return true
}

func applyOp(val string, op FilterOp, cmp string) bool {
	switch op {
	case FilterOpEq:
		return val == cmp
	case FilterOpNeq:
		return val != cmp
	case FilterOpLt:
		return val < cmp
	case FilterOpLte:
		return val <= cmp
	case FilterOpGt:
		return val > cmp
	case FilterOpGte:
		return val >= cmp
	case FilterOpContains:
		return strings.Contains(val, cmp)
	}
	return false
}

func deriveSchema(rows []map[string]string, selectors []FieldSelector) []FieldDescriptor {
	seen := make(map[string]struct{})
	var order []string
	for _, row := range rows {
		for k := range row {
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				order = append(order, k)
			}
		}
		break
	}
	schema := make([]FieldDescriptor, 0, len(order))
	for _, k := range order {
		entity := entityOf(k)
		field := fieldOf(k)
		schema = append(schema, FieldDescriptor{Entity: entity, Field: field, Kind:
	}
	return schema
}

func qualifyKey(entity, field string) string {
	if strings.Contains(field, ".") {
		return field
	}
	return entity + "." + field
}

func entityOf(key string) string {
	if i := strings.Index(key, "."); i >= 0 {
		return key[:i]
	}
	return key
}

func fieldOf(key string) string {
	if i := strings.Index(key, "."); i >= 0 {
		return key[i+1:]
	}
	return key
}

func mergeRows(left, right map[string]string) map[string]string {
	out := make(map[string]string, len(left)+len(right))
	for k, v := range left {
		out[k] = v
	}
	for k, v := range right {
		out[k] = v
	}
	return out
}
