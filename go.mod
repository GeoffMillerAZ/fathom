module github.com/geoffmilleraz/fathom

go 1.26.2

require (
	cuelang.org/go v0.16.1
	github.com/emicklei/proto v1.14.3
	github.com/geoffmilleraz/agentic-kit v0.7.1-0.20260922212050-beb53a4fb901
	github.com/geoffmilleraz/app-kit v0.0.0
	github.com/hashicorp/hcl/v2 v2.24.0
	github.com/pelletier/go-toml/v2 v2.3.1
	github.com/tetratelabs/wazero v1.12.0
	github.com/tree-sitter-grammars/tree-sitter-lua v0.5.0
	github.com/tree-sitter/go-tree-sitter v0.25.0
	github.com/tree-sitter/tree-sitter-bash v0.25.1
	github.com/tree-sitter/tree-sitter-c v0.24.2
	github.com/tree-sitter/tree-sitter-c-sharp v0.23.5
	github.com/tree-sitter/tree-sitter-cpp v0.23.4
	github.com/tree-sitter/tree-sitter-java v0.23.5
	github.com/tree-sitter/tree-sitter-javascript v0.25.0
	github.com/tree-sitter/tree-sitter-php v0.24.2
	github.com/tree-sitter/tree-sitter-python v0.25.0
	github.com/tree-sitter/tree-sitter-ruby v0.23.1
	github.com/tree-sitter/tree-sitter-rust v0.24.2
	github.com/tree-sitter/tree-sitter-scala v0.26.0
	github.com/tree-sitter/tree-sitter-typescript v0.23.2
	github.com/vektah/gqlparser/v2 v2.5.34
	github.com/yuin/goldmark v1.8.2
	golang.org/x/net v0.54.0
	golang.org/x/tools v0.45.0
	google.golang.org/protobuf v1.33.0
	gopkg.in/yaml.v3 v3.0.1
	modernc.org/sqlite v1.52.0
)

require (
	cuelabs.dev/go/oci/ociregistry v0.0.0-20251212221603-3adeb8663819 // indirect
	github.com/agext/levenshtein v1.2.1 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/cockroachdb/apd/v3 v3.2.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/protocolbuffers/txtpbfmt v0.0.0-20260217160748-a481f6a22f94 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rogpeppe/go-internal v1.15.0 // indirect
	github.com/zclconf/go-cty v1.16.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.36.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	modernc.org/libc v1.72.3 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)

// In-tree mirror of the private agentic-kit module at the exact pinned
// commit (third_party/agentic-kit, see docs/distribution.md, ADR-019) — a
// cold clone with no access to the private repo builds from this. Local dev
// with go.work active still resolves ../agentic-kit (the workspace `use`
// directive wins over this replace); GOWORK=off falls through to this line.
replace github.com/geoffmilleraz/agentic-kit => ./third_party/agentic-kit

// app-kit is mirrored in-tree for the same reason agentic-kit is (ADR-019):
// it is private, fathom's observatory does not compile without it, and a
// clone that cannot reach the private repo must still build. Before
// 2026-09-01 this pointed at ../app-kit and a fresh clone failed outright.
// The vendored commit is recorded in third_party/app-kit/.vendor-commit;
// app-kit has no published version, hence the v0.0.0 requirement above.
replace github.com/geoffmilleraz/app-kit => ./third_party/app-kit
