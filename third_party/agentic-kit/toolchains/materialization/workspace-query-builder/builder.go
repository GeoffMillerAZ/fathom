package workspacequerybuilder

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ─── Public constructors ──────────────────────────────────────────────────────

// SelectEntity starts a new WorkspaceQuery scoped to the given entity kind.
// Additional entities are added by chaining WithJoin.
func SelectEntity(entity string) WorkspaceQuery {
	return WorkspaceQuery{
		Entities: []string{entity},
	}
}

// SelectFields starts a new WorkspaceQuery scoped to the given entity kind,
// projecting only the specified field names.  When no fields are given all
// fields for that entity are projected (SELECT *).
func SelectFields(entity string, fields ...string) WorkspaceQuery {
	sel := FieldSelector{Entity: entity, Fields: append([]string(nil), fields...)}
	return WorkspaceQuery{
		Entities: []string{entity},
		Select:   []FieldSelector{sel},
	}
}

// ─── Builder methods (all return a new copy) ─────────────────────────────────

// WithSelect appends a FieldSelector to the query's projection list.
func (q WorkspaceQuery) WithSelect(sel FieldSelector) WorkspaceQuery {
	out := q.clone()
	out.Select = append(out.Select, sel)
	out.Entities = addIfMissing(out.Entities, sel.Entity)
	return out
}

// WithField adds a single-field projection for the given entity, with an
// optional alias.  It is a convenience wrapper around WithSelect.
func (q WorkspaceQuery) WithField(entity, field, alias string) WorkspaceQuery {
	return q.WithSelect(FieldSelector{Entity: entity, Fields: []string{field}, Alias: alias})
}

// WithJoin appends a JoinSpec to the query using a pre-built JoinSpec.
// Both LeftEntity and RightEntity are registered in the Entities list if absent.
func (q WorkspaceQuery) WithJoin(entity, key string) WorkspaceQuery {
	if len(q.Entities) == 0 {
		return q
	}
	spec := JoinSpec{
		LeftEntity:  q.Entities[0],
		RightEntity: entity,
		LeftKey:     key,
		RightKey:    key,
		Kind:        JoinKindInner,
	}
	return q.withJoinSpec(spec)
}

// WithJoinKind is like WithJoin but lets the caller specify the join kind.
func (q WorkspaceQuery) WithJoinKind(entity, key string, kind JoinKind) WorkspaceQuery {
	if len(q.Entities) == 0 {
		return q
	}
	spec := JoinSpec{
		LeftEntity:  q.Entities[0],
		RightEntity: entity,
		LeftKey:     key,
		RightKey:    key,
		Kind:        kind,
	}
	return q.withJoinSpec(spec)
}

// WithJoinSpec appends a fully-specified JoinSpec.
func (q WorkspaceQuery) WithJoinSpec(spec JoinSpec) WorkspaceQuery {
	return q.withJoinSpec(spec)
}

func (q WorkspaceQuery) withJoinSpec(spec JoinSpec) WorkspaceQuery {
	if spec.Kind == "" {
		spec.Kind = JoinKindInner
	}
	if spec.RightKey == "" {
		spec.RightKey = spec.LeftKey
	}
	out := q.clone()
	out.Joins = append(out.Joins, spec)
	out.Entities = addIfMissing(out.Entities, spec.LeftEntity)
	out.Entities = addIfMissing(out.Entities, spec.RightEntity)
	return out
}

// Where appends a WHERE predicate using a raw FilterExpr.
func (q WorkspaceQuery) Where(expr FilterExpr) WorkspaceQuery {
	out := q.clone()
	out.Filters = append(out.Filters, expr)
	return out
}

// WithFilter is an alias for Where (mirrors the reference API surface).
func (q WorkspaceQuery) WithFilter(expr FilterExpr) WorkspaceQuery {
	return q.Where(expr)
}

// Limit sets the maximum number of result rows.  Zero means no limit.
func (q WorkspaceQuery) Limit(n int) WorkspaceQuery {
	out := q.clone()
	out.MaxRows = n
	return out
}

// WithLimit is an alias for Limit (mirrors the reference API surface).
func (q WorkspaceQuery) WithLimit(n int) WorkspaceQuery {
	return q.Limit(n)
}

// ─── Validation ──────────────────────────────────────────────────────────────

// Valid returns a non-nil error if the query is malformed.
func (q WorkspaceQuery) Valid() error {
	if len(q.Entities) == 0 {
		return errors.New("workspacequerybuilder: query has no entities")
	}
	entitySet := make(map[string]struct{}, len(q.Entities))
	for _, e := range q.Entities {
		entitySet[e] = struct{}{}
	}
	for i, j := range q.Joins {
		if _, ok := entitySet[j.RightEntity]; !ok {
			return fmt.Errorf("workspacequerybuilder: join[%d] references unknown entity %q", i, j.RightEntity)
		}
	}
	return nil
}

// ─── SQL emission (Compile) ───────────────────────────────────────────────────

// Compile produces a deterministic SQL string from the query.
// The SQL uses entity names directly as table aliases (suitable for
// read_parquet() wrappers when used via EmitSQL).
//
// Determinism guarantees:
//   - Column order: entities in declaration order, fields in declaration order
//     (or sorted alphabetically when derived from a wildcard).
//   - Join order: joins in declaration order.
//   - WHERE predicates in declaration order.
//   - No timestamps, no randomness, no global state.
func (q WorkspaceQuery) Compile() (string, error) {
	if err := q.Valid(); err != nil {
		return "", err
	}

	var b strings.Builder

	// SELECT clause
	b.WriteString("SELECT ")
	cols := buildColumnList(q)
	b.WriteString(strings.Join(cols, ", "))

	// FROM clause — primary entity
	b.WriteString("\nFROM ")
	b.WriteString(q.Entities[0])

	// JOIN clauses — stable order (declaration order)
	for _, j := range q.Joins {
		if j.Kind == JoinKindCross {
			b.WriteString(fmt.Sprintf("\nCROSS JOIN %s", j.RightEntity))
		} else {
			lk := qualifyKey(j.LeftEntity, j.LeftKey)
			rk := qualifyKey(j.RightEntity, j.RightKey)
			b.WriteString(fmt.Sprintf("\n%s JOIN %s ON %s = %s",
				string(j.Kind), j.RightEntity, lk, rk))
		}
	}

	// WHERE clause
	if len(q.Filters) > 0 {
		preds := make([]string, 0, len(q.Filters))
		for _, f := range q.Filters {
			preds = append(preds, filterSQL(f))
		}
		b.WriteString("\nWHERE ")
		b.WriteString(strings.Join(preds, " AND "))
	}

	// LIMIT clause
	if q.MaxRows > 0 {
		b.WriteString(fmt.Sprintf("\nLIMIT %d", q.MaxRows))
	}

	return b.String(), nil
}

// EmitSQL produces a fully-qualified DuckDB SQL string that wraps each entity
// in a read_parquet() call using paths from the ReadsetDescriptor.
//
// Returns an error if the query is invalid or if an entity has no matching
// dataset alias in the descriptor.
func EmitSQL(pack ParquetQueryPack) (string, error) {
	if err := pack.Query.Valid(); err != nil {
		return "", err
	}

	// Build alias→path map.
	aliasToPath := make(map[string]string, len(pack.Descriptor.Datasets))
	for _, d := range pack.Descriptor.Datasets {
		aliasToPath[d.Alias] = d.Path
	}

	// Verify every entity has a dataset entry.
	for _, e := range pack.Query.Entities {
		if _, ok := aliasToPath[e]; !ok {
			return "", fmt.Errorf("workspacequerybuilder: entity %q has no dataset in descriptor", e)
		}
	}

	// Rewrite q into a version whose entity references will appear in SELECT
	// and JOIN clauses, but replace the FROM / JOIN table refs with
	// read_parquet('path') AS alias form.

	q := pack.Query
	var b strings.Builder

	// SELECT
	b.WriteString("SELECT ")
	cols := buildColumnList(q)
	b.WriteString(strings.Join(cols, ", "))

	// FROM primary entity
	primary := q.Entities[0]
	b.WriteString(fmt.Sprintf("\nFROM read_parquet('%s') AS %s", aliasToPath[primary], primary))

	// JOINs
	for _, j := range q.Joins {
		if j.Kind == JoinKindCross {
			b.WriteString(fmt.Sprintf("\nCROSS JOIN read_parquet('%s') AS %s",
				aliasToPath[j.RightEntity], j.RightEntity))
		} else {
			lk := qualifyKey(j.LeftEntity, j.LeftKey)
			rk := qualifyKey(j.RightEntity, j.RightKey)
			b.WriteString(fmt.Sprintf("\n%s JOIN read_parquet('%s') AS %s ON %s = %s",
				string(j.Kind), aliasToPath[j.RightEntity], j.RightEntity, lk, rk))
		}
	}

	// WHERE
	if len(q.Filters) > 0 {
		preds := make([]string, 0, len(q.Filters))
		for _, f := range q.Filters {
			preds = append(preds, filterSQL(f))
		}
		b.WriteString("\nWHERE ")
		b.WriteString(strings.Join(preds, " AND "))
	}

	// LIMIT
	if q.MaxRows > 0 {
		b.WriteString(fmt.Sprintf("\nLIMIT %d", q.MaxRows))
	}

	return b.String(), nil
}

// ─── In-memory execution ──────────────────────────────────────────────────────

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
	if q.MaxRows > 0 && len(filtered) > q.MaxRows {
		filtered = filtered[:q.MaxRows]
	}

	schema := deriveSchema(filtered)

	return QueryResult{
		Schema:           schema,
		Rows:             filtered,
		TotalBeforeLimit: totalBeforeLimit,
	}, nil
}

// ExecuteJoin performs a simple in-memory equi-join between two row sets.
// It supports all four JoinKind values.
func ExecuteJoin(spec JoinSpec, leftRows, rightRows []map[string]string) []map[string]string {
	if spec.Kind == "" {
		spec.Kind = JoinKindInner
	}
	if spec.RightKey == "" {
		spec.RightKey = spec.LeftKey
	}

	leftKey := qualifyKey(spec.LeftEntity, spec.LeftKey)
	rightKey := qualifyKey(spec.RightEntity, spec.RightKey)

	var result []map[string]string

	switch spec.Kind {
	case JoinKindInner:
		// Index right on join key; O(n+m).
		index := buildIndex(rightRows, rightKey)
		for _, lrow := range leftRows {
			k := lrow[leftKey]
			for _, rrow := range index[k] {
				result = append(result, mergeRows(lrow, rrow))
			}
		}

	case JoinKindLeft:
		index := buildIndex(rightRows, rightKey)
		for _, lrow := range leftRows {
			k := lrow[leftKey]
			matches := index[k]
			if len(matches) == 0 {
				result = append(result, mergeRows(lrow, nil))
			} else {
				for _, rrow := range matches {
					result = append(result, mergeRows(lrow, rrow))
				}
			}
		}

	case JoinKindRight:
		leftIndex := buildIndex(leftRows, leftKey)
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

// ─── Internal helpers ─────────────────────────────────────────────────────────

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
	return append(slice, val)
}

// buildProjectionKeys converts a list of FieldSelectors into a set of
// "entity.field" keys for O(1) lookup.  An empty Fields list becomes a
// "*:<entity>" sentinel meaning "all fields from that entity".
func buildProjectionKeys(selectors []FieldSelector) map[string]struct{} {
	if len(selectors) == 0 {
		return nil
	}
	keys := make(map[string]struct{})
	for _, sel := range selectors {
		if len(sel.Fields) == 0 {
			keys["*:"+sel.Entity] = struct{}{}
		} else {
			for _, f := range sel.Fields {
				keys[qualifyKey(sel.Entity, f)] = struct{}{}
			}
		}
	}
	return keys
}

// projectRow copies fields from row that match keys.
// nil keys means project everything.
func projectRow(row map[string]string, keys map[string]struct{}) map[string]string {
	if keys == nil {
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
			// Absent field: only neq passes (missing != anything is true).
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

// deriveSchema infers the schema from the first result row.
func deriveSchema(rows []map[string]string) []FieldDescriptor {
	if len(rows) == 0 {
		return nil
	}
	// Collect keys from the first row and sort for determinism.
	first := rows[0]
	order := make([]string, 0, len(first))
	for k := range first {
		order = append(order, k)
	}
	sort.Strings(order)

	schema := make([]FieldDescriptor, 0, len(order))
	for _, k := range order {
		schema = append(schema, FieldDescriptor{
			Entity: entityOf(k),
			Field:  fieldOf(k),
			Kind:   "string",
		})
	}
	return schema
}

// buildColumnList returns the ordered SELECT column expressions for a query.
// When q.Select is empty, SELECT * is returned.
// Otherwise columns are emitted in declaration order; an entity wildcard
// (empty Fields) becomes "<entity>.*".
func buildColumnList(q WorkspaceQuery) []string {
	if len(q.Select) == 0 {
		return []string{"*"}
	}
	var cols []string
	for _, sel := range q.Select {
		if len(sel.Fields) == 0 {
			cols = append(cols, sel.Entity+".*")
			continue
		}
		for _, f := range sel.Fields {
			col := qualifyKey(sel.Entity, f)
			if sel.Alias != "" && len(sel.Fields) == 1 {
				col = col + " AS " + sel.Alias
			}
			cols = append(cols, col)
		}
	}
	return cols
}

// filterSQL renders a FilterExpr as a SQL predicate fragment.
func filterSQL(f FilterExpr) string {
	key := qualifyKey(f.Entity, f.Field)
	switch f.Op {
	case FilterOpEq:
		return fmt.Sprintf("%s = '%s'", key, escapeSQLString(f.Value))
	case FilterOpNeq:
		return fmt.Sprintf("%s != '%s'", key, escapeSQLString(f.Value))
	case FilterOpLt:
		return fmt.Sprintf("%s < '%s'", key, escapeSQLString(f.Value))
	case FilterOpLte:
		return fmt.Sprintf("%s <= '%s'", key, escapeSQLString(f.Value))
	case FilterOpGt:
		return fmt.Sprintf("%s > '%s'", key, escapeSQLString(f.Value))
	case FilterOpGte:
		return fmt.Sprintf("%s >= '%s'", key, escapeSQLString(f.Value))
	case FilterOpContains:
		return fmt.Sprintf("%s LIKE '%%%s%%'", key, escapeSQLString(f.Value))
	}
	return "1=1"
}

// escapeSQLString replaces single quotes with doubled single quotes.
func escapeSQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
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

func buildIndex(rows []map[string]string, key string) map[string][]map[string]string {
	index := make(map[string][]map[string]string, len(rows))
	for _, row := range rows {
		k := row[key]
		index[k] = append(index[k], row)
	}
	return index
}
