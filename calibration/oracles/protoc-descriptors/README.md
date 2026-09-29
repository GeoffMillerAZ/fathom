# protoc-descriptors — tier-A protobuf oracle (definitions, references, imports)

protoc is protobuf's reference compiler (design §1.3, tier A: "the format's reference parser"). This program compiles
every `.proto` file of a subject on its own with `protoc --include_source_info --descriptor_set_out=<f>`, decodes the
`FileDescriptorSet` protoc writes (`google.golang.org/protobuf/types/descriptorpb`) and writes the neutral oracle file
`indexbench lang-oracle -tool-oracle` reads. Its own Go module; bench-only, never a product dependency.

    cd calibration/oracles/protoc-descriptors
    go run . -subject <checkout> [-protoc <protoc>] [-I <root>]... > workspace/tmp/oracle/protobuf-<subject>.json
    go run . -subject <checkout> -image <buf image json> > workspace/tmp/oracle/protobuf-<subject>-buf.json

Roots: the `-I` flags when given, else every buf module root the subject declares (a v2 `buf.yaml`'s `modules[].path`,
a v1 `buf.yaml`'s own directory, a `buf.work.yaml`'s `directories`) and the repository root. A file is compiled under
the longest root that holds it with every root on the proto path, so an import resolves as buf would resolve it inside
the workspace. A file protoc rejects (an import no root holds: an external dependency) is named on stderr and writes
nothing; measure such a subject with `lang-oracle -only` over the files protoc compiles. `-image` reads a `buf build -o
<f>#format=json` image instead (protojson): the same walk over a second compiler's descriptors, a cross-check.

What it writes, every line from `source_code_info` spans (first line, last line):

- `definitions`: `message` (nested ones too; a proto2 group is the nested message protoc makes of it; the synthetic
  map-entry messages are not written), `enum` (nested too), `service`, `rpc`, each named by its own name.
- `references`: (enclosing message or rpc, the referenced type's own name) for every message- or enum-typed field, oneof
  member, map value (attributed to the message holding the map, not to the synthetic entry) and extension field declared
  inside a message, and an rpc's request and response. A group field (the group declares its type in place), a map
  field's own entry type, a file-level extension (nothing encloses it) and an `extend`'s target are not references.
- `imports`: every import statement whose path protoc resolved to a file of the subject, at the statement's line; a
  well-known (`google/protobuf/*`) or external import is not written.
- `cross_references`: a reference whose type protoc resolved to a definition in another file of the subject.

First runs (2026-09-29, libprotoc 34.1, buf 1.70.0): grpc/grpc-go v1.84.0, 13 files compiled (10 measured: discovery
excludes `testdata/`); bufbuild/protovalidate v1.2.2, 126 of 156 compiled (29 files under
`proto/protovalidate-testing/tests/migrate` import protoc-gen-validate's `validate/validate.proto` and
`examples/cel_field_presence.proto` googleapis' `google/type/postal_address.proto`, neither in the repository). buf's image
of the repository's workspace (37 files) gives the same 919 definitions, 287 references, 42 imports and 11 cross-file
references as protoc.
