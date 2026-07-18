// valid.cue — concrete values that satisfy the #Source constraint defined in
// schema.cue. Running `cue vet ./...` from the package root validates these.
package sourceregistry

// A markdown source.
_src1: #Source & {
	ID:   "src-readme-payments"
	Kind: "markdown"
	URI:  "docs/payments/README.md"
}

// A readme source with an empty URI (in-memory / synthetic).
_src2: #Source & {
	ID:   "src-readme-core"
	Kind: "readme"
	URI:  ""
}

// A formal doc portal source.
_src3: #Source & {
	ID:   "src-confluence-arch"
	Kind: "doc"
	URI:  "https://wiki.example.com/arch"
}

// An "other" source.
_src4: #Source & {
	ID:   "src-sme-interview-2024"
	Kind: "other"
	URI:  "sme://interview/2024-01-15"
}
