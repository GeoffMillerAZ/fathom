# thrift-json -- tier-A Apache Thrift oracle (definitions, references, imports, cross references)

`thrift` is the Apache Thrift compiler, the IDL's reference parser (design 1.3, tier A). This program compiles every
`.thrift` file of a subject on its own with `thrift --gen json -I <root> -o <dir> <file>`, reads the JSON the compiler
writes, and writes the neutral oracle file `indexbench lang-oracle -tool-oracle` reads. Its own Go module, standard
library only; bench-only, never a product dependency.

    cd calibration/oracles/thrift-json
    go run . -subject <checkout> [-thrift <thrift>] [-I <root>]... > workspace/tmp/oracle/thrift-<subject>.json

A file the compiler rejects (an include no root holds, a syntax the Apache compiler does not read) is named on stderr and
writes nothing; measure such a subject with `lang-oracle -only` over the files that compiled (the paths of the oracle
file's definitions). Roots: the `-I` flags when given, else the repository root; an include is looked up beside the
including file first, as the compiler does.

**What the compiler attests and what the harness adds.** `thrift --gen json` carries no source positions and writes a
typedef's target type where a member names the typedef. So `scan.go` (a comment- and string-aware tokenizer and a small
reader of the IDL's declarations, independent of the tree-sitter grammar under test) places each declaration, member and
type name, and the program refuses a file unless the scanner equals the compiler: every namespace scope, typedef, const,
enum and value, struct, union, exception and field, service and function the compiler reports is found by the scanner and
the reverse, and every user type the compiler writes into a field, argument, return, throws or extends is a type the
scanner found at that member (a member written through a typedef is checked only for the typedef). What the scanner
adds beyond the compiler's attestation is where each name is written and the references through a typedef name.
Constants whose value is a map with non-string keys make the compiler write invalid JSON: those files are read without
the constants section and their constants are not cross-checked.

What it writes, from the scanner's spans:

- `definitions`: `namespace` (named by its scope: `java`, `py`, `*`, because the grammar under test splits a dotted
  namespace value into sibling nodes), `typedef`, `const`, `enum`, `enum_value`, `struct`, `union`, `exception`, `field`,
  `service` and `function`, each by its own name, at its first line, with its last line as end.
- `references`: (enclosing member, referenced type's own name) for every user type written in a field, a function's
  argument, return or throws list, a typedef's or const's type, and a service's `extends`. The enclosing member is the
  field, function, typedef, const or service. A built-in type (`i32`, `string`, `uuid`, `list<...>`) is none.
- `imports`: every include statement whose file is a file of the subject, at the statement's line.
- `cross_references`: a reference written `file.Type` whose `file` an include names, resolved to the definition in that
  file.

Tests: `go test ./...` (the scanner on a fixture of every declaration form).
