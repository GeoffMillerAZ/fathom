package lang

// Skip reasons: the closed set Result.Skipped holds. A skipped file has no
// symbols; the reason says why.
const (
	SkipBinary       = "binary"       // the bytes are not source text
	SkipOversize     = "oversize"     // over the file limit (2 MiB, lowerable by WithLimits)
	SkipGenerated    = "generated"    // a generated file the dialect excludes
	SkipAmbiguous    = "ambiguous"    // the file could belong to two dialects and the content does not settle it
	SkipUnclassified = "unclassified" // no dialect reads this path and content
	SkipNotLinked    = "not-linked"   // a dialect owns the file but its extractor is not linked into this Reader
)

// Level values Result.Level and DialectInfo.Level hold besides L1 to L4.
const (
	LevelUnmeasured = "unmeasured" // no reading exists for the dialect
	LevelNone       = "none"       // measured, and no level was met
)

// File is one file to read. Path is repository-relative with forward slashes
// and is the only thing the Reader knows about where the file lives.
type File struct {
	Path string
	Src  []byte
	// Siblings is the optional list of the other repository-relative paths in
	// the same repository. Some classifications read it (an Ansible role, a
	// Helm chart's templates, a Timoni module). Empty is legal: the file is
	// then classified alone, and Result.Notes says so when that could change
	// the verdict.
	Siblings []string
}

// Result is what one file holds. Every slice is non-nil and sorted, so equal
// inputs give byte-equal JSON.
type Result struct {
	// Dialect is the id of the dialect or generic reader (generic-json,
	// generic-toml) that read the file, and "" only when nothing read it: a
	// skipped file. SkipNotLinked names the dialect that owns the file but is
	// not linked into this Reader.
	Dialect string `json:"dialect"`
	// Language is the file's language class from its path.
	Language string `json:"language"`
	// Skipped is a Skip reason, or "" when a dialect read the file.
	Skipped    string       `json:"skipped"`
	Symbols    []Symbol     `json:"symbols"`
	References []Reference  `json:"references"`
	Unresolved []Unresolved `json:"unresolved"`
	Facts      []Fact       `json:"facts"`
	// Level is the dialect's measured level: L1 to L4, LevelNone or
	// LevelUnmeasured. It is what readings proved about the dialect, not a
	// promise about this file. It is "" when Skipped is set: no dialect read
	// the file, so no level applies.
	Level string `json:"level"`
	// Incomplete is true when an engine could not fully read the file: the
	// symbols, references and facts above are what it did read, and Notes says
	// why. False means no engine reported a failure, which is not a promise
	// that every construct was read: the tree-sitter dialects recover from a
	// syntax error without reporting it. A policy tool that must not pass a
	// file it could not read should refuse when Incomplete is true.
	Incomplete bool `json:"incomplete"`
	// Notes disclose what the Reader could not know and that may have changed
	// the result. Empty when nothing did.
	Notes []string `json:"notes"`
}

// Symbol is one definition.
type Symbol struct {
	// Key is the symbol's identity within its file: Package and Name joined by
	// a dot, or Name alone when Package is empty. Reference.From and
	// Reference.ToKey hold Keys. A dialect names a reference target as it
	// writes it (Reference.To), which equals a Key only when the dialect
	// writes targets the way it names definitions; ToKey is the join.
	Key  string `json:"key"`
	Name string `json:"name"`
	// Package is the scope the extractor groups the symbol under, and the part
	// of Key before Name. It is the extractor's own scope, not a normalized
	// package: for most dialects it is the directory or package a definition
	// sits in, and for the Go extractor it is the package name for a function,
	// type, constant and variable but the receiver type for a method and the
	// owner type for a field (method M on type T reads Package "T", Name "T.M",
	// Key "T.T.M"). The scope is part of Key and so of Reference.From; it is
	// pinned by the tests and changes only as a Breaking change.
	Package string `json:"package"`
	// Kind is the dialect's own name for what the symbol is (for example
	// TerraformResource). Kinds may be added and renamed as languages
	// improve; do not branch on a Kind across dialects.
	Kind string `json:"kind"`
	// Canonical is the closed cross-dialect vocabulary. Values only grow. It
	// is empty for a kind that has no canonical meaning (a fact an extractor
	// observes, for example) and for a kind of an extractor no table maps. A
	// Go type reads struct, interface or type_alias by its declaration.
	Canonical string `json:"canonical"`
	Line      int    `json:"line"`
	// EndLine is the last line, 0 when the extractor does not compute it.
	EndLine   int    `json:"end_line"`
	Signature string `json:"signature"`
	// Tier is how the symbol was established: verified, pre-scan,
	// doc-claimed, sme-stated or assumed.
	Tier string `json:"tier"`
}

// Reference is one edge from a symbol (or the document, From "") to a target.
type Reference struct {
	// From is the Key of the referencing symbol, or "" for a reference the
	// document itself makes.
	From string `json:"from"`
	// To is the target exactly as the dialect names it: a Go call names
	// package.Func, a Terraform reference var.name, a CloudFormation Ref the
	// logical id, a cross-file reference the path, image or address the
	// Strategy resolved. It is not necessarily a Key.
	To string `json:"to"`
	// ToKey is the Key of the symbol in the same file that To names, when To
	// equals exactly one symbol's Key or Name, and empty otherwise (a target
	// in another file, an ambiguous name, a cross-file reference).
	ToKey string `json:"to_key"`
	// Edge names the relation: "call" for a call, "reference" for a plain
	// reference from a document dialect that declares no relation, and
	// otherwise the edge the dialect declares (uses, imports, depends_on and
	// so on). Do not branch on an Edge across dialects.
	Edge string `json:"edge"`
	// Raw is the value as written, when it differs from To.
	Raw string `json:"raw"`
	// Strategy is how a document reference resolved (repo-path, image-ref,
	// module-source and so on), empty for a call.
	Strategy string `json:"strategy"`
	// Line is the line the reference is written on. 0 means the engine does
	// not record one: calls read by the tree-sitter and Go engines carry no
	// line.
	Line int `json:"line"`
}

// Unresolved is a reference the Reader read and refused to guess.
type Unresolved struct {
	From   string `json:"from"`
	Edge   string `json:"edge"`
	Raw    string `json:"raw"`
	Reason string `json:"reason"`
	Line   int    `json:"line"`
}

// Fact is one scalar a dialect declares an entity writes.
type Fact struct {
	// Entity is the Key of the symbol the fact belongs to, or the entity's name
	// when it is no symbol of this file.
	Entity string `json:"entity"`
	Key    string `json:"key"`
	Value  string `json:"value"`
	Line   int    `json:"line"`
}

// DialectInfo describes one dialect a Reader can read.
type DialectInfo struct {
	ID string `json:"id"`
	// Kind is "language" or "tooling".
	Kind string `json:"kind"`
	// Engine is how the dialect is read: tags, doc, tags+doc or native.
	Engine string `json:"engine"`
	// Matches is the sorted extensions, basenames and globs the dialect claims.
	Matches []string `json:"matches"`
	// Level and Grade are what readings proved: Level is L1 to L4, LevelNone
	// or LevelUnmeasured, Grade the grade at that level ("" when none).
	Level string `json:"level"`
	Grade string `json:"grade"`
}
