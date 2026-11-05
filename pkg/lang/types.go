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
	// Dialect is the id of the dialect that read the file ("" when none did).
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
	// promise about this file.
	Level string `json:"level"`
	// Notes disclose what the Reader could not know and that may have changed
	// the result. Empty when nothing did.
	Notes []string `json:"notes"`
}

// Symbol is one definition.
type Symbol struct {
	// Key is Package and Name joined the way References name a symbol.
	Key     string `json:"key"`
	Name    string `json:"name"`
	Package string `json:"package"`
	// Kind is the dialect's own name for what the symbol is (for example
	// TerraformResource). Kinds may be added and renamed as languages
	// improve; do not branch on a Kind across dialects.
	Kind string `json:"kind"`
	// Canonical is the closed cross-dialect vocabulary. Values only grow. It
	// is empty for a kind no dialect manifest declares.
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
	From string `json:"from"`
	To   string `json:"to"`
	// Edge names the relation: "call" for a call, otherwise the dialect's.
	Edge string `json:"edge"`
	// Raw is the value as written, when it differs from To.
	Raw string `json:"raw"`
	// Strategy is how a document reference resolved (repo-path, image-ref,
	// module-source and so on), empty for a call.
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
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
