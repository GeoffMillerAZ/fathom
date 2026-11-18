// Package deps reads direct file dependencies using a caller-supplied repository
// view. Results disclose uncertainty so consumers can widen or run when inputs
// are incomplete. The package performs no toolchain calls, closure or hashing.
package deps

// Kind is the dependency construct.
type Kind string

const (
	Import     Kind = "import"
	Include    Kind = "include"
	Embed      Kind = "embed"
	BuildInput Kind = "build-input"
	Exec       Kind = "exec"
	ConfigRef  Kind = "config-ref"
)

// Resolution describes the target's identity.
type Resolution string

const (
	File          Resolution = "file"
	Directory     Resolution = "directory"
	External      Resolution = "external"
	UnknownTarget Resolution = "unknown"
)

// Provenance names the provider of an edge.
type Provenance string

const (
	Builtin   Provenance = "builtin"
	Pack      Provenance = "pack"
	Extension Provenance = "extension"
	Declared  Provenance = "declared"
)

// UnknownCode gives the reason a file's dependency evidence is incomplete.
type UnknownCode string

const (
	DynamicImport        UnknownCode = "dynamic-import"
	ComputedPath         UnknownCode = "computed-path"
	UnresolvedAlias      UnknownCode = "unresolved-alias"
	MissingConfig        UnknownCode = "missing-config"
	UnboundedGlob        UnknownCode = "unbounded-glob"
	UnsupportedConstruct UnknownCode = "unsupported-construct"
	ParseError           UnknownCode = "parse-error"
	UnmeasuredReader     UnknownCode = "unmeasured-reader"
	ExternalUnpinned     UnknownCode = "external-unpinned"
	ReadLimit            UnknownCode = "read-limit"
)

// DecidableBy names evidence that could resolve an unknown.
type DecidableBy string

const (
	Toolchain   DecidableBy = "toolchain"
	Lockfile    DecidableBy = "lockfile"
	ConfigFile  DecidableBy = "config-file"
	Observation DecidableBy = "observation"
	Declaration DecidableBy = "declaration"
)

// Edge is one direct dependency. From is a repository-relative slash path. To
// is a logical repository path or an external ecosystem:name ID. Version is
// populated only from a lockfile or manifest pin. Line is one-based, with 1
// used for file-wide evidence.
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

// Unknown discloses missing evidence. An empty DecidableBy grants no trust.
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
