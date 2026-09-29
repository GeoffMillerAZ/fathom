// Package kitworkflow defines the versioned JSON envelope consumed by the
// local-only kit-workflow prose command. Go additionally rejects duplicate
// keys, trailing values, invalid UTF-8, and nesting over 64 levels before this
// structural contract is decoded.
package kitworkflow

import prose "github.com/geoffmilleraz/agentic-kit/kits/governed-prose-kit:governedprosekit"

_nonempty: string & !=""

#CurrentSource: {id: _nonempty, ref: prose.#SourceRef}

// #Input deliberately contains snapshots and current pointers in one envelope:
// callers can use the command without a Go adapter. They are declarations, not
// authenticated source or access attestations; the command only checks their
// hash/reference/evidence consistency. `audience` is required by the export
// operation and prohibited by `prose check` at the Go command boundary.
#Input: {
	version: "kit-workflow/prose/v1"
	sources: [prose.#SourceSnapshot, ...prose.#SourceSnapshot]
	current?: [...#CurrentSource] | null
	request: prose.#SourcedRequest
	draft:   prose.#ProseDraft
	config:  prose.#SourcedConfig
	non_factual?: [...string] | null
	audience?: "public" | "restricted"
}

// #Output describes actual stdout, including unsuccessful command invocations.
// Assertions describe local checks only and carry no publication authority.
#Assertions: {
	source_bytes: _nonempty
	source_links: _nonempty
	access:       _nonempty
	publication:  _nonempty
}
#Output: {
	version:    "kit-workflow/prose/v1"
	operation:  "prose/check"
	assertions: #Assertions
	artifact:   prose.#SourcedArtifact
} | {
	version:    "kit-workflow/prose/v1"
	operation:  "prose/export"
	assertions: #Assertions
	export:     prose.#ProseExport
} | {
	version: "kit-workflow/prose/v1"
	error: {kind: "usage" | "invalid" | "evidence" | "unsupported" | "artifact" | "canceled", message: _nonempty}
}
