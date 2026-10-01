# protobuf — provenance

- Grammar: `coder3101/tree-sitter-proto` tag `0.6.0`, commit `6c878d18628ebbff3474479d2fdd6d8ba1954c3e` (MIT; the
  top-level `LICENSE`, Mohammad Ashar Khan 2024-2025; no licence file under `queries/` or `src/`), ABI 15, no external
  scanner, `src/parser.c` shipped (no generate step), grammar at the repository root (`tree_sitter_proto`), built into
  `internal/extractor/tsengine/grammars/protobuf.wasm.gz` (204,748 bytes raw, 68,444 bytes gzip). Pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json` (that directory's `PROVENANCE.md` regenerates its own tables
  from it). proto2, proto3 and editions files parse: none of the 169 `.proto` files of the two calibration subjects has an
  ERROR or MISSING node.
- `tags.scm`: written in this repository (the grammar ships no `tags.scm`; neither aider nor nvim-treesitter carries
  protobuf tags). The kinds are the incumbent `protoextract.go`'s four, their `SourceKind` strings byte-identical
  (`ProtoMessage`, `ProtoEnum`, `ProtoService`, `ProtoRPC`, design §11 step 3). A message and a proto2 group (the nested
  message protoc makes of it) are `ProtoMessage`; a message and an enum nested in a message are named by their parents
  (`Order.Line`, `Order.Line.Unit`, the protobuf full name inside the file), an rpc by its service (`Greeter.SayHello`);
  the incumbent named each by its bare name. A message is `callable` so that it owns the type references its fields make
  (it then carries a `Signature`, `message Order`); an rpc's `Signature` runs from `rpc` to its `{` or `;`, whitespace
  collapsed, so it names both types and any `stream` keyword.
- References (`@reference.call`): every `message_or_enum_type` — a field's, a map value's, a oneof member's, an extension
  field's, an rpc's request and response — named by its last identifier (`acme.v1.Money`, `.acme.v1.Money` and `Money` are
  `Money`). A trailing `.` anchor does not select the last identifier of that node in this grammar (every identifier
  matched), so each pattern fixes the path's length with `#match?` on the whole reference and walks to its last
  identifier with leading and sibling anchors (paths of one to eight segments). A reference belongs to the smallest
  enclosing message or rpc, so a file-level `extend` block's fields belong to nothing. Scalar types, option names, an
  `extend`'s target and reserved names are not references.
- `imports.scm`: written in this repository. `import "a/b.proto";` (and `public`, `weak`) is a whole-file import of that
  path; `imports.strategy: file-symbol` with no parameters resolves the path under the repository root (the `protoc -I .`
  layout). A buf workspace roots its imports at buf.yaml's module paths instead, which no strategy reads (needs-strategy,
  `LB-Q-protobuf-rest`).
- `testdata/`: fixtures written in this repository, each golden written by hand from reading the fixture before the
  queries were final: `greeter.proto` (the incumbent's own fixture), `shapes.proto` (nesting three deep, a map, a oneof, a
  package-qualified and a fully qualified reference, a multi-line streaming rpc with an option block, an import),
  `legacy.proto` (proto2: a group, an extension range, an `extend` inside a message and one at file level) and
  `broken.proto` (a malformed field; the messages on both sides of it and its sibling field still extract). The goldens
  failed first on the group (no pattern yet) and on the import (no `imports.scm` yet), then matched entry for entry.
- **Measurement (docs/bench/readings/lang/protobuf.json, calibration/corpus.d/protobuf.yaml):** protoc 34.1 (tier A, the
  format's reference compiler, `calibration/oracles/protoc-descriptors`) on grpc/grpc-go v1.84.0 (10 files) and
  bufbuild/protovalidate v1.2.2 (118 of its files: protoc rejects the 30 that import a file outside the repository):
  `level_met` L2, L1 and L2 excellent (sym_recall and sym_precision 1 over 1,192 definitions, ref_recall and ref_precision
  1 over 456 pairs); buf build 1.70.0 reads the same definitions, references and imports on the 37 files its workspace
  holds, and universal-ctags 6.2.1 (tier B, unscored) agrees on 1,188 of the 1,192 definitions. L3 is not measured (the
  harness scores a tool format's cross-file references only for a doc-engine extractor) and the import strategy is short
  (needs-strategy): `LB-Q-protobuf-rest`.
- Dispatch: not switched. `protoextract.go` stays `.proto`'s extractor in `fusion.New` (design §11 step 2: `indexbench
  lang-parity -incumbent protoextract` refuses the port while its L3 target is unmet, although the dialect raises every
  metric: sym_recall .9967 to 1, sym_precision .9363 to .9696 and ref_recall 0 to 1 on the unrestricted subjects), and the
  incumbent's rpc `Signature` (the request and response type names joined by an arrow) is text no source spells, which a
  tags-engine signature cannot emit (`TestProtobufDialectRPCSignature`). No `ExtractorEpoch` bump: no stored output changes
  until the switch.
- `references.scm` (LB-F74, LB-F77, wave lc): written in this repository; no upstream query adapted
  (a type reference with its package path).
