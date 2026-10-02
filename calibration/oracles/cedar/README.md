# cedar — tier-A Cedar oracle (policies, schemas, the governed scope)

`cedar` (cedar-policy-cli, the reference implementation's command line) is Cedar's own parser and validator (design
§1.3, tier A). This program reads every `.cedar` and `.cedarschema` file of a subject through it and writes the neutral
oracle file `indexbench lang-oracle -tool-oracle` reads. Its own Go module (standard library only); bench-only, never a
product dependency.

    cargo install --locked cedar-policy-cli --version 4.13.0   # CARGO_HOME and CARGO_TARGET_DIR under scratch
    cd calibration/oracles/cedar
    go run . -subject <checkout> -cedar <cedar> [-files <list of subject-relative paths>] > workspace/tmp/oracle/cedar-<subject>.json

Per file: `cedar check-parse --policies|--schema` (a file the CLI rejects is named on stderr and writes nothing), then
`cedar translate-policy --direction cedar-to-json` or `cedar translate-schema --direction
cedar-to-json-with-resolved-types`. What it writes:

- `definitions`: each static policy (`policy`) and template (`template`), named by the id the CLI gives it (its `@id`
  annotation, else `policy<i>` by its place in the file); each namespace (`namespace`), entity type (`entity`), action
  (`action`, by its id) and common type (`type`) of a schema, entity and common types qualified by their namespace
  (`PhotoApp::User`).
- `calls`, a schema's own references (L2): an entity type to its parent types and to each entity or common type its shape
  and tags name; a common type to the types it names; an action to its principal and resource types, the types its context
  names and its parent actions. Types are as the CLI resolves them; primitives and extension types are not written.
- `cross_references`, the governed scope: a policy's principal and resource (`is T`, `== T::"x"`, `in T::"x"`) and action
  (`== Action::"a"`, `in [...]`) to each schema file of the subject declaring T, or the action a.

**Lines are the writer's, not the CLI's.** Neither JSON form carries a source position, so a small statement scan
(comments and strings skipped) locates them: the i-th policy statement, from its first annotation to its `;`, is the
CLI's policy of that id (the ids the scan derives must be the CLI's, or the file is named and skipped); a schema
declaration (`namespace X {…}`, `entity A, B …;`, `action "a", b …;`, `type T = …;`) gives each name it declares its
lines. Names, kinds and references are tier A; a reading measured with this file says that the lines are located.

First runs (2026-10-02, cedar-policy-cli 4.13.0), for LB-Q-cedar's refusal (LB-Q-cedar-rest) and its re-plan:
cedar-policy/cedar-examples c251f7d1 (34 files: 20 policy files, 14 schemas; 117 policies, 21 templates, 3 namespaces,
62 entity types, 127 actions, 9 common types; 350 schema reference pairs; 484 scope references) and
cedar-policy/cedar-integration-tests v4.13.0 (25 files: 22 policy files, 3 schemas; 30 policies, 22 entity types, 21
actions; 71 pairs; 180 scope references). The CLI accepts all 59 files. No reading was taken: no parser in scope reads
them under the 5% line (chrnorm/tree-sitter-cedar 5092204d leaves an ERROR node in 5 of the 42 policy files and has no
schema grammar; cedar-go v1.8.0 rejects 6 of the 42, every file with a template, and its schema AST carries no source
position).
