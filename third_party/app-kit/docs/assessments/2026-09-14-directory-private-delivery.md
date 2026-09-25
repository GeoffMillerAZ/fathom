# P04 private directory/table delivery

The second typed adapter now delivers usable source files instead of requiring manual
JSON extraction. [ADR-0036](../adr/0036-share-private-delivery-assurance.md) resolves
ADR-0034's second-adapter trigger without merging models, renderers or artifact schemas.
This advances G1/G4; counts remain 439/657 syntax/compiler/DOM, five saved attempts,
two typed candidates and zero certified targets.

## Delivered capability

`appkit data assets-adapt-directory` retains its JSON default. Explicit `-out NEW_DIRECTORY`
exports the source-styled five-file package; add `-enhanced` for the eight-file package
containing the unchanged base and separate usability companion. `-check DIRECTORY`
repeats original archive access and current source-specific replay, then compares exact
inventory, bytes and modes. Always select the same variant for export and check.

Source-styled files: `directory.templ`, `directory.go`, `defaults.json`, `README.md`,
`artifact.json`. Enhanced adds `directory_enhanced.templ`, `directory_enhanced.go` and
`directory_enhanced.css`, with enhanced usage guidance and the existing nested-base
artifact. The artifact is last; no new manifest or certification schema is introduced.

Both variants retain private 0700/0600 exclusive creation under an existing real parent,
no-clobber and incomplete-output retention. Checks require bounded regular single-link
files, exact modes and names, no-follow/nonblocking reads and open-identity checks on
macOS/Linux. Other check platforms remain unsupported. The owner controls the parent
and output; this is not protection against a hostile concurrent same-user writer or an
ACL audit. Success is silent, drift exit 1, invalid/source/operational failure exit 2.

The source-specific entrypoints independently verify frozen replay before filesystem I/O.
Only internal filesystem assurance is shared with P02. P02's distinct drift sentinel,
six-file inventory, byte projections and JSON remain unchanged. Invalid fixed inventory
names, duplicates or an early artifact marker fail before I/O. No generic exported API,
public vendor entry, dependency install, compiler execution or backend is added by delivery.

## Evidence and review

- Synthetic tests exercise exact P04 inventories/bytes, private deterministic writes,
  no-clobber, per-file byte/missing/link/mode/special-entry drift, wrong variants,
  partial-write retention and rejection of non-frozen replay before I/O. Existing P02
  tests also exercise the shared mechanism, including FIFO refusal.
- Scoped race tests pass for `internal/designadapt` and `cmd/appkit` delivery/refusal
  cases. The full root Go suite and `go vet ./...` pass. These are local commands,
  not continuously enforced fleet checks.
- P02's explicit paid-source CLI test still passes with six files and 15,338-byte
  default JSON. Existing P04 base/enhanced JSON replay tests pass. Original archives
  and catalog are rehashed after the live tests; no licensed bodies are logged.
- The new live delivery test checks five/eight files, exact JSON/artifact equality,
  silent success and exit classifications, then compiles and renders copied sources.
  Initial review found that compilation still used app-kit's module. That run proves
  copied-source compilation only; the strengthened standalone-module run passes below.
- Independent static review found no P1/P2 issue in the bounded delivery/replay/CLI
  contract. This does not prove hostile filesystem isolation or visual quality.
- MCP document/resource parity passes. Updated agent guidance exposes both deliveries
  and preserves evidence limits; no agent host is automatically restarted or adopted.

Intermittent macOS `EPERM` affected source reads, go.mod and test cleanup rehashes.
Failed attempts remain failures, distinct from later passing runs. No permission,
global configuration, cache reset or checksum bypass was used to obtain a pass.

## Standalone module rehearsal

The final live test passes in 5.410s (package 5.852s) on Go 1.26.3 darwin/arm64 and
templ v0.3.1020. It creates separate base/enhanced modules from copied delivery inputs,
reusing the reviewed first-party P02 module/sum fixture with only the module name changed.
The resulting default/enhanced JSON sizes are 15,597/17,910 bytes; exact delivery artifact
bytes match those envelopes. No module file or dependency is added to the export itself.

Both modules reject app-kit dependencies/replacements, run every Go command from their
own directory, and verify `GOMOD` resolves there. GOENV/GOWORK are off, toolchain local,
module mode readonly and GOPROXY off; public checksum verification remains enabled and
private/no-sum overrides are cleared. `go mod verify` passes against existing caches.
A pre-generation compile must fail specifically for missing generated code, then pinned
templ generation, compilation and the named synthetic constructor/render test pass.
Nonempty component/compiler dependency inventories include their required modules and
exclude app-kit/sibling directories. Copied sources, module/sum/test inputs, original
five/eight-file deliveries and source archive/catalog remain unchanged.

This is **cached standalone-module resolution and compile/render proof**: not cold-cache
reproducibility, binary equality, denied-network/filesystem isolation, a real consumer
app or browser quality. P02's stronger sandbox rehearsal remains separate. The live
race attempt hit EPERM; only the distinct non-live delivery/refusal race suite is passed.
Final live harness SHA-256:
`fa337744d61ab550345ab0b76c7a30668a945918c78c1a5d039c0a156acca0d9`.

## Shared tooling discovery

The existing CUE catalog now has 26 records: its prior 24 entries are unchanged,
plus `reference.pricing-adapter` and `reference.directory-adapter`. These are
experimental first-party CLI-tool metadata (`profile=cli`, `kind=reference`, no web
layers), not licensed component bodies or certified target records. This implements
ADR-0020's shared discovery contract without a new schema, MCP tool or server.

Search MCP with `query="private adapter"`, `profile="cli"`, `kind="reference"`, then
use `get_capability` for exact requirements/usage. Gallery shares those records at
`/catalog/reference.pricing-adapter` and `/catalog/reference.directory-adapter` and
its existing JSON routes; the terminal catalog also uses the same snapshot.
Rebuild the relevant binaries to load the new catalog revision; this work does not
restart an agent host or claim that an already-running Gallery loaded it.

Catalog tests, actual MCP search/get/resource tests and Gallery HTML/JSON route tests
pass without licensed stores. Web-profile, available/proven, recipe and L4 filters
exclude the CLI candidates; summary responses omit full usage/source material.
Existing source-anchor checks remain intact. The first attempt's descriptive provenance
string failed the exact `first-party` guard; metadata now correctly identifies the
first-party tool, with frozen licensed-original requirements in dependencies. The guard
was not relaxed. Gallery, MCP, catalog and terminal package suites pass. No new browser
or host-adoption proof is claimed for this metadata-only addition.

## Reproduce and interpret

```sh
go test -race ./internal/designadapt ./cmd/appkit -run 'Test.*Delivery|TestAssetsAdaptDirectoryRefuses' -count=1
APPKIT_PROBE_CONVERSION_KIT_ROOT=/absolute/app-kit go test ./cmd/appkit -run '^TestAssetsAdaptDirectoryDeliveryLive$' -count=1 -v
go test ./cmd/appkit-mcp -count=1
```

The private live test is opt-in; its default skip is not delivery evidence. Ordinary
tests use synthetic markup. Compile copies outside the immutable input directory;
apps own Go/templ/Tailwind/font pins, routes, authorization and data. See the
[consumer guide](fixtures/table-adapter/README.md) for selection and boundaries.

Rendering inputs are unchanged: source-styled rule/model/CUE and enhanced CSS retain
their prior identities. The [existing browser evidence](2026-09-14-directory-enhanced-adapter.md)
still names original target-size failures and enhancement limits; no new graphical
certification is inferred. Actual app adoption, performance/zoom/assistive-input gates
and catalog certification remain open. No consumer, release, push or tag was changed.
