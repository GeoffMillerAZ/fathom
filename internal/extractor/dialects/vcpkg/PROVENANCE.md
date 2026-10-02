# vcpkg — provenance and divergences

LB-Q-vcpkg, design `docs/design/language-breadth.md` §4, §13. A doc-engine dialect (no `tags.scm`, no
upstream grammar): the decode is the engine's `json` decoder (`encoding/json`), and the incumbent was the
generic `JSONExtractor`'s key dump, which a `vcpkg.json` or `vcpkg-configuration.json` no longer reaches
(the basename lookup runs first). The fixtures are written for the dialect, not copied from a project.

## What it models

- **The package** (`VcpkgPackage`, canonical `package`): the `vcpkg.json` document itself, named by `name`
  and signed by whichever of `version`, `version-semver`, `version-date`, `version-string` is written. A
  manifest with no `name` (a manifest-mode project may omit it) has no package entity, so its dependencies
  are not references of anything. It is a port (`ports/<name>/vcpkg.json`, an overlay port) or a project
  manifest, the same document.
- **Features** (`VcpkgFeature`, canonical `key`): each entry of `features` that is a table, named
  `features.<name>` (the engine's `$path`, so a feature never collides with a package of the same name).
- **Registries** (`VcpkgRegistry`, canonical `resource`): `default-registry` and each `registries[]` entry of a
  `vcpkg-configuration.json`, named by `name`, then `location`, then `repository`; signed by `kind`. Not
  scored (no tool lists them), counted under `excluded_by_kind`.
- **References** (`Calls` on the package): the name of every dependency, a string or an object's `name`, from
  `dependencies` and from every feature's `dependencies`. A dependency on a feature of the port itself
  (`{"name": "<this port>", "default-features": false}`) is a reference to the package's own name.
- **Cross-file references** (`fusion.ResolveDocXrefs`, file-tier edges entity to entity): each dependency name
  to the one other manifest declaring a package of that name (`module-source`, edge `depends_on`). That is how a
  port reaches the port it depends on and how an overlay port is found: an overlay port is a `vcpkg.json` in
  the repository, so a dependency on its name resolves to it.

## What it deliberately does not model

- **Which registry a name means.** A dependency resolves by package name alone. vcpkg resolves a name
  through the overlay ports, then the configured registries, then the built-in one; a project that overlays a
  registry port (azure-sdk-for-cpp's `vcpkg-custom-ports/openssl`) is read as depending on the overlay.
- **Platform and feature conditions.** A `platform` expression, `host`, `default-features` and a dependency's
  `features` are qualifiers of an edge, not separate edges: a platform-excluded dependency is still a
  dependency of the manifest.
- **`overrides`, `builtin-baseline`, `supports`, `default-features`** and the `version>=` constraints are not
  entities or references.
- **`overlay-ports` and `vcpkg-configuration.json` cross-file edges.** An `overlay-ports` entry names a
  directory of ports (or a port); the resolver joins a directory to its one doc file and a directory of ports
  has none, so the edge would resolve for a port directory only, and no subject carries one. The dependency
  join above already reaches an overlay port by name; no `overlay_port` edge is declared.
- **A name two manifests declare** resolves to nothing, as for every `module-source` doc dialect (the resolver
  refuses to guess). On the subjects there is none; a repository that carries a port twice (a registry
  and an in-tree copy) loses those edges.
- **The module-source join is not scoped to a dialect** (`internal/fusion/docxref.go`, wave tl's file): a
  `package.json` or `Cargo.toml` that names itself like a port makes that port's name ambiguous at ingest
  (observation O370), which the harness, resolving over the dialect's own files only, does not see.
- **Templates that are not JSON** (azure-sdk-for-cpp's `sdk/*/vcpkg/vcpkg.json` open with a licence comment and a
  version placeholder) decode as nothing; vcpkg itself refuses them. They are 20 of the 42 vcpkg files of that
  subject, claimed by the dialect and yielding no entity.
- **Ownership is the basename** `vcpkg.json` and `vcpkg-configuration.json`, exact case, at any depth. A
  `versions/*.json` registry file and a `package.json` are not claimed.

## The calibration oracle

`calibration/oracles/vcpkg-depend` (its own Go module, never a product dependency; `calibration/oracles/README.md`
has the invocation), tier A, vcpkg `2026-09-26-Homebrew` (`brew install vcpkg`) with `VCPKG_ROOT` the pinned
`microsoft/vcpkg` checkout (tag 2026.07.29).

- **Definitions and references** come from `vcpkg x-package-info <name> --overlay-ports=<dir> --x-json`, run
  once per `vcpkg.json` on a scratch copy of the manifest (an overlay needs no portfile for the query). vcpkg
  parses and validates the manifest and prints it normalized: the name, the version, every dependency with its
  qualifiers, every feature with its own dependencies. A manifest it refuses is not attested. The tool writes
  no lines; the package is at the line of the document's opening brace and a feature at its key's line, found
  by a stdlib token scan. The exit status of `x-package-info` is 1 even when it answers, so the answer is the
  judge.
- **Cross-file references** are the dependency names the tool listed, joined by name to the attested manifests
  of the subject (a name two manifests declare is skipped; none on the subjects). The join is the oracle
  adapter's, the names are the tool's. With `-depend-info` the declared edges are cross-checked against
  `vcpkg depend-info '<name>[*]' --max-recurse=1 --triplet=<t>` (every feature on) on arm64-osx,
  x64-windows and x64-linux: on `microsoft/vcpkg` ports 11,971 of 12,228 declared edges are resolved by the
  tool and none resolved is undeclared (the 257 others are platforms or features no triplet of the three
  selects); on azure-sdk-for-cpp all 25.

## Subjects

`microsoft/vcpkg` 2026.07.29 (MIT; 2,858 port manifests under `ports/`, 6,312 definitions, 12,394 references,
12,228 cross-file edges) and `Azure/azure-sdk-for-cpp` at 6b0886c9 (MIT; a manifest-mode root `vcpkg.json`, 21
manifests vcpkg accepts of 41 plus a `vcpkg-configuration.json` sample; 24 definitions, 75 references, 25
edges).
