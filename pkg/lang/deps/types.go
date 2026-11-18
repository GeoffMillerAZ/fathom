// Package deps reads direct file dependencies using a caller-supplied repository
// view. Results disclose uncertainty so consumers can widen or run when inputs
// are incomplete. The package performs no toolchain calls, closure or hashing.
package deps

// Kind is the dependency construct.
type Kind string

const (
	KindImport     Kind = "import"
	KindInclude    Kind = "include"
	KindEmbed      Kind = "embed"
	KindBuildInput Kind = "build-input"
	KindExec       Kind = "exec"
	KindConfigRef  Kind = "config-ref"
)

// Resolution describes the target's identity.
type Resolution string

const (
	ResolvedFile      Resolution = "file"
	ResolvedDirectory Resolution = "directory"
	ResolvedExternal  Resolution = "external"
	ResolvedUnknown   Resolution = "unknown"
)

// Provenance names the provider of an edge.
type Provenance string

const (
	ProvenanceBuiltin   Provenance = "builtin"
	ProvenancePack      Provenance = "pack"
	ProvenanceExtension Provenance = "extension"
	ProvenanceDeclared  Provenance = "declared"
)

// UnknownCode gives the reason a file's dependency evidence is incomplete.
type UnknownCode string

const (
	UnknownDynamicImport        UnknownCode = "dynamic-import"
	UnknownComputedPath         UnknownCode = "computed-path"
	UnknownUnresolvedAlias      UnknownCode = "unresolved-alias"
	UnknownMissingConfig        UnknownCode = "missing-config"
	UnknownUnboundedGlob        UnknownCode = "unbounded-glob"
	UnknownUnsupportedConstruct UnknownCode = "unsupported-construct"
	UnknownParseError           UnknownCode = "parse-error"
	UnknownUnmeasuredReader     UnknownCode = "unmeasured-reader"
	UnknownExternalUnpinned     UnknownCode = "external-unpinned"
	UnknownReadLimit            UnknownCode = "read-limit"
)

// DecidableBy names evidence that could resolve an unknown.
type DecidableBy string

const (
	DecideToolchain   DecidableBy = "toolchain"
	DecideLockfile    DecidableBy = "lockfile"
	DecideConfigFile  DecidableBy = "config-file"
	DecideObservation DecidableBy = "observation"
	DecideDeclaration DecidableBy = "declaration"
)

// Edge is one direct dependency. From is a repository-relative slash path. To
// is a logical repository path or an external ecosystem:name ID. Version is
// populated only from a lockfile or manifest pin. Line is one-based when known,
// and zero for evidence without a source line.
type Edge struct {
	From       string     `json:"from"`
	Kind       Kind       `json:"kind"`
	Raw        string     `json:"raw"`
	To         string     `json:"to"`
	Resolution Resolution `json:"resolution"`
	Version    string     `json:"version"`
	Line       int        `json:"line"`
	Provenance Provenance `json:"provenance"`
}

// Unknown discloses missing evidence. Line is zero when no source line is
// available. An empty DecidableBy grants no trust.
type Unknown struct {
	From        string        `json:"from"`
	Code        UnknownCode   `json:"code"`
	Raw         string        `json:"raw"`
	Line        int           `json:"line"`
	DecidableBy []DecidableBy `json:"decidable_by"`
}

// FileDeps holds a file's sorted edges and unknowns, including empty arrays.
// Complete requires zero unknowns and either passing admission (Measured) or
// explicit opt-in to an unmeasured reader (Trusted).
type FileDeps struct {
	Path     string    `json:"path"`
	Dialect  string    `json:"dialect"`
	Measured bool      `json:"measured"`
	Trusted  bool      `json:"trusted"`
	Complete bool      `json:"complete"`
	Edges    []Edge    `json:"edges"`
	Unknowns []Unknown `json:"unknowns"`
}

// Input names a repository-relative file. A blank Dialect infers the reader
// from Path where possible.
type Input struct {
	Path    string
	Dialect string
}
