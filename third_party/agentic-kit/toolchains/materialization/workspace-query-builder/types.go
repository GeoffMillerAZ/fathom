// Package workspacequerybuilder provides a fluent Go DSL for constructing
// workspace queries that compile to deterministic DuckDB-compatible SQL.
//
// A WorkspaceQuery is an immutable value type: every builder method clones
// the receiver and returns the copy, so queries can be safely shared across
// goroutines and used as templates for derived queries.
package workspacequerybuilder

// JoinKind identifies the SQL join type.
type JoinKind string

const (
	// JoinKindInner is a SQL INNER JOIN (default).
	JoinKindInner JoinKind = "INNER"
	// JoinKindLeft is a SQL LEFT JOIN.
	JoinKindLeft JoinKind = "LEFT"
	// JoinKindRight is a SQL RIGHT JOIN.
	JoinKindRight JoinKind = "RIGHT"
	// JoinKindCross is a SQL CROSS JOIN (no ON clause).
	JoinKindCross JoinKind = "CROSS"
)

// FilterOp is the comparison operator used in a FilterExpr.
type FilterOp string

const (
	// FilterOpEq compiles to SQL "=".
	FilterOpEq FilterOp = "eq"
	// FilterOpNeq compiles to SQL "!=".
	FilterOpNeq FilterOp = "neq"
	// FilterOpLt compiles to SQL "<".
	FilterOpLt FilterOp = "lt"
	// FilterOpLte compiles to SQL "<=".
	FilterOpLte FilterOp = "lte"
	// FilterOpGt compiles to SQL ">".
	FilterOpGt FilterOp = "gt"
	// FilterOpGte compiles to SQL ">=".
	FilterOpGte FilterOp = "gte"
	// FilterOpContains compiles to a SQL LIKE substring match.
	FilterOpContains FilterOp = "contains"
)

// FieldSelector declares which fields to project from a given entity.
// When Fields is empty, all fields from that entity are projected (SELECT *
// for that entity).
type FieldSelector struct {
	Entity string
	// Fields is the ordered list of field names to project. Empty means all.
	Fields []string
	// Alias is an optional column alias applied to the first field when len(Fields)==1.
	Alias string
}

// JoinSpec describes a single join between two entities.
type JoinSpec struct {
	LeftEntity  string
	RightEntity string
	// LeftKey is the field on LeftEntity used as the join predicate.
	LeftKey string
	// RightKey is the field on RightEntity used as the join predicate.
	// When empty it defaults to LeftKey.
	RightKey string
	Kind     JoinKind
}

// FilterExpr is a single predicate applied as part of a WHERE clause.
// Multiple filters are combined with AND.
type FilterExpr struct {
	Entity string
	Field  string
	Op     FilterOp
	Value  string
}

// FieldDescriptor describes a single projected column in a QueryResult schema.
type FieldDescriptor struct {
	Entity string
	Field  string
	// Kind is the inferred column kind; currently always "string" for in-memory
	// execution because rows are map[string]string.
	Kind string
}

// QueryResult is the result of an in-memory Execute call.
type QueryResult struct {
	Schema           []FieldDescriptor
	Rows             []map[string]string
	TotalBeforeLimit int
}

// WorkspaceQuery is the central immutable query value.  Build one with
// SelectEntity, then chain builder methods to add joins, filters, and field
// projections.
type WorkspaceQuery struct {
	Entities []string
	Select   []FieldSelector
	Joins    []JoinSpec
	Filters  []FilterExpr
	// MaxRows is the row cap set by Limit(n). Zero means no limit.
	MaxRows int
}

// DatasetEntry is a single entry in a ReadsetDescriptor: an alias (the name
// used in generated SQL) and the file path of the Parquet dataset.
type DatasetEntry struct {
	Alias string
	Path  string
}

// ReadsetDescriptor declares the set of Parquet files (datasets) that form a
// workspace.  Each entry's Alias must be unique and matches an entity name
// used in a WorkspaceQuery.
type ReadsetDescriptor struct {
	ReadsetID string
	Datasets  []DatasetEntry
}

// ParquetQueryPack bundles a ReadsetDescriptor with a WorkspaceQuery and a
// stable pack identifier.  EmitSQL uses it to produce fully-qualified DuckDB
// SQL.
type ParquetQueryPack struct {
	PackID     string
	Descriptor ReadsetDescriptor
	Query      WorkspaceQuery
}
