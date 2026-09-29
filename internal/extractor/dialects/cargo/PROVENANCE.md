# cargo — provenance and divergences

LB-Q-cargo, design `docs/design/language-breadth.md` §4, §13. A doc-engine dialect (no
`tags.scm`, no upstream grammar): the decode is `github.com/pelletier/go-toml/v2` through the doc
engine's `toml` decoder, and the incumbent was the generic `TOMLExtractor`'s bare table/key dump
(`KindTOMLTable`/`KindTOMLKey`), which a `Cargo.toml` no longer reaches (the basename lookup runs
first). This file records what the dialect models, what it deliberately does not and why, and where
the calibration oracle comes from.

## What it models

- **The package** (`CargoPackage`, canonical `package`): the manifest itself. It is selected at the
  document root (`$` with a `package` table), named by `package.name` and signed by
  `package.version`, because the doc engine hangs a reference on the entity that contains it and the
  dependency tables sit beside `[package]`, not under it. So the entity's line is the file's first
  line and its end line the last line that holds a value: the root table begins the file, and the
  oracle names that same anchor. A version inherited from the workspace (`version.workspace = true`)
  is a table, not a scalar, and leaves the signature empty.
- **The workspace** (`CargoWorkspace`, canonical `namespace`): the `[workspace]` table, named
  `workspace`, at its header line (a table written only as `[workspace.metadata.x]` sits at the first
  line that writes under it). It is the origin of the member edges below, and the only way a virtual
  manifest (no `[package]`) owns an entity. Not scored (the tool has no notion of the table itself);
  counted under `excluded_by_kind`.
- **Features** (`CargoFeature`, canonical `key`): each key of `[features]` whose value is a list, at
  its key's line.
- **Explicit targets**: `[lib]` (`CargoLib`, canonical `module`), each `[[bin]]` (`CargoBin`,
  `route`), `[[example]]` (`CargoExample`), `[[test]]` (`CargoTest`) and `[[bench]]`
  (`CargoBench`), all `target`. Each is named `<kind>/<name>` (`bin/acme`, `lib/acme_core`,
  through the engine's `qualify` on `$path`), signed by its `path` when written. The prefix is why a
  binary named like its package (the usual case), a library named like it and a feature named like a
  dependency keep distinct symbol keys: `core.SymbolKey` carries no kind and one file's entities are
  keyed by name alone, the same collision that keeps GitLab CI's stages and Compose's volumes out. A
  `[lib]` or `[[bin]]` without a `name` is not an entity (the tool derives the name from the package;
  the engine has no such value).
- **References** (`Calls` on the package): the real package name of every dependency, from
  `[dependencies]`, `[dev-dependencies]`, `[build-dependencies]` and each of their
  `[target.<cfg>.*]` forms: the key, or the `package = "real"` rename when there is one. A dependency
  on a third party resolves to nothing, as an unresolvable call does.
- **Cross-file references** (`fusion.ResolveDocXrefs`, file-tier edges entity to entity):
  - `workspace_member` (`repo-path`, origin the `workspace` entity): each `members` entry to the
    manifest in that directory (a glob such as `crates/*` names every directory it matches that holds
    exactly one doc file), resolved to that manifest's package. A member path that climbs out of the
    repository resolves to nothing.
  - `depends_on`, `dev_depends_on`, `build_depends_on` (`module-source`, origin the package): each
    dependency's real name to the one other manifest declaring a package of that name.

## What it deliberately does not model

- **Which registry a name means.** A dependency resolves by package name alone, as Maven's resolve by
  artifactId. Cargo resolves a name to the registry unless the dependency names a `path`, a `git`
  source, or a `[patch]` redirects it, so a registry dependency that shares a name with a package of
  the repository (cargo's own root depends on the published `home`, `crates/home` is a workspace
  member) is drawn as an edge the tool does not draw: the one false edge in the reading
  (xref_precision .9895). The opposite case reads right: tokio lists each member it depends on by
  version and patches it in `[patch.crates-io]`, and the tool's resolved graph draws all 13 of
  those edges the dialect finds by name.
- **Implicit workspace members.** The root package of a manifest that also holds `[workspace]` is a
  member whether or not `members` names `.`, and so is every path dependency inside the root; only
  what `members` lists is an edge. The reading's one missed edge is cargo's own root package.
  `exclude` is not read either, so a glob that reaches an excluded directory holding a manifest
  draws an edge cargo does not (none on the subjects).
- **Implicit features.** Every optional dependency is a feature unless some feature names it with
  `dep:`; the engine has no way to say "unless", so only `[features]` keys are entities (tokio has 18
  such implicit features, cargo 2).
- **Auto-discovered targets** (`src/main.rs`, `src/lib.rs`, `src/bin/*.rs`, `examples/*.rs`,
  `tests/*.rs`, `benches/*.rs`): a target is an entity only when the manifest writes it (46 explicit
  against 240 discovered on tokio, 6 against 46 on cargo).
- **Workspace policy**: `[workspace.dependencies]`, `[workspace.package]`, `[patch]`, `[replace]`,
  `[profile.*]`, `[lints]`, `[package.metadata]` and `[badges]` are not entities or references. A
  dependency written `foo.workspace = true` is the name `foo`, which is what the tool reports too
  unless the workspace root renames it with `package =` (not seen on the subjects).
- **The deprecated `dev_dependencies` and `build_dependencies` spellings** are not read (edition
  2024 rejects them), nor a `[dependencies]` key under a non-`target` table.
- **Ownership is the basename `Cargo.toml`**, exact case, at any depth. `Cargo.lock`,
  `.cargo/config.toml`, `rust-toolchain.toml`, `rustfmt.toml`, `clippy.toml` and `deny.toml` are not
  claimed and stay with the generic TOML extractor (or, for the lockfile, unclassified).
- **Fixture manifests are indexed like any manifest, and the reading does not measure them.** A
  repository keeps `Cargo.toml` files that are no part of its build: `rust-lang/cargo` has 641
  that ingest reads, 27 in its workspace and 614 fixtures (612 under `tests/testsuite`, the
  cargo-add and workspace test cases, and 2 test containers under `crates/cargo-test-support`;
  356 workspace entities among them, most fixtures declare their own `[workspace]`). The tool
  builds a workspace, so its truth is the 27, and the reading measures `rust-lang/cargo` with
  `-only` (the root, `benches/*`, `crates/*`, `credential/*`). With every manifest in scope the same
  run reads `sym_precision` 0.0495 (546 dialect packages against 27 oracle ones), `xref_precision`
  0.2403 (xref_recall .9825) and `level_met` none: the fixtures are real manifests the oracle
  cannot attest, and one that shares a package name with a member makes references to it
  ambiguous. `tokio-rs/tokio` has no fixtures (13 manifests, its two fuzz crates each their own
  workspace) and is measured whole. The restriction is in the reading's notes and in the deferred
  row `LB-Q-cargo-rest`.
- **A member directory that also holds another doc-engine file** (a `pyproject.toml` beside the
  `Cargo.toml` of a PyO3 crate, a `package.json` beside a wasm crate) is dropped by the resolver,
  which joins a directory to its one doc file (`fileTargets`, `len(fs) == 1`); the harness resolves
  over the dialect's own files only and so does not see it.
- **Ranges are not scored for tool formats** (§1.2): the package's end line is the last line that
  holds a value (the closing bracket of a multi-line array is not a line the decoder tracks).

## The calibration oracle

One file per subject, written by `calibration/oracles/cargo` (its own Go module, never a product
dependency; `calibration/oracles/README.md` has the invocation), tier A, cargo 1.98.1 (the standalone
toolchain from static.rust-lang.org, sha256 checked, installed under the measurement scratch
directory; `rustc` 1.98.1 beside it for the resolving run).

- **Definitions and references** come from `cargo metadata --no-deps --format-version 1`, run at
  each workspace root (`rust-lang/cargo`: its `Cargo.toml`; `tokio-rs/tokio`: its `Cargo.toml` plus
  the two fuzz crates, each a workspace of its own). Cargo evaluates the workspace: which manifests
  are members (`members` globs, `exclude`, path dependencies inside the root), each package's name,
  and each package's dependencies with `workspace = true` inheritance, renames and target tables
  resolved. The tool writes no line, so a package sits at line 1, the anchor described above.
- **Cross-file references** come from the resolved graph, `cargo metadata --all-features
  --format-version 1` at the same roots (every feature on and no platform filter, so every declared
  dependency edge exists): an edge between two packages that live in the subject, [patch] applied,
  plus the `workspace` entity to each member `workspace_members` lists. A registry dependency that
  only shares a name with a package of the subject and resolves to the registry is not an edge (one
  on the subjects, printed as a namesake). The resolving run fetches the workspace's registry
  dependencies (774 MB into a scratch `CARGO_HOME` for the two subjects, deleted after measuring);
  `rust-lang/cargo` ships a `Cargo.lock`, `tokio-rs/tokio` does not (resolved 2026-09-29), which
  changes no edge between the subject's own packages.
- **Features and explicit targets are cross-checked, not scored.** The tool lists the implicit
  ones, so what would be scored is the set the manifest declares, which is the dialect's own
  decode. Each attested manifest's raw declarations are checked against the tool's instead:
  49 features declared on tokio (18 more implicit) and 9 on cargo (2 implicit), 46 explicit
  targets on tokio (240 discovered) and 6 on cargo (46), 101 and 229 dependencies, every one in the
  tool's list (0 declared-not-in-tool, 0 tool-not-declared). The 58 features and 52 explicit targets
  are counted under `excluded_by_kind` in the reading.

## Subjects

`rust-lang/cargo` 0.99.0 (MIT OR Apache-2.0; 27 workspace manifests of 641, 229 references, 57
cross-file edges: 27 members, 30 resolved dependencies; a root package that is also the workspace
root, `[workspace.dependencies]` inheritance, `crates/*` and `credential/*` globs) and
`tokio-rs/tokio` tokio-1.53.1 (MIT; 13 manifests, 12 packages, 101 references, 38 edges: 12
members, 26 resolved dependencies of which 13 are patched-registry dependencies; a virtual
workspace root, two fuzz crates that each name themselves as their workspace's member).
