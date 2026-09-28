# docker-compose — provenance and divergences

LB-Q-docker-compose, design `docs/design/language-breadth.md` §4, §13. A doc-engine
dialect (no `tags.scm`, no upstream grammar): this file records the port decisions,
what the dialect deliberately does not do and why, and where the calibration oracle
comes from.

## Port of `internal/extractor/composeextract.go`

`ComposeService` is the incumbent's own `SourceKind`, kept verbatim (§2.2, §13.2 step
15). Name, line, tier and the `depends_on` calls are identical to the incumbent's on its
own fixtures (`TestDockerComposeDialectIncumbentParity`) and on both calibration
subjects (`lang-parity`: sym_recall/sym_precision/ref_recall/ref_precision all equal).
Dispatch stays on the incumbent (§11 step 2): the merge gate wants the target level (L3)
and this reading reaches L2, so `fusion` still routes `compose.y*ml` /
`docker-compose.y*ml` to `ComposeExtractor`, the dialect only classifies and is measured.

Divergences, all forced by §4's selector subset (no literals, no string operations):

- **Signature.** The incumbent spells `build: <context>` for a service with no `image:`;
  the doc engine can only emit the value, so the signature is the bare context (`./api`).
  `image:` still wins over `build:`, as before. A `build:` map without a `context` reads
  `build: .` from the incumbent and an empty signature here.
- **Ownership.** The incumbent is keyed on four basenames and, through fusion's content
  classifier, any YAML file with a compose shape. The dialect claims the four basenames
  plus `docker-compose.*.yml`, `docker-compose-*.yml`, `compose.*.yml` and `compose-*.yml`
  (and `.yaml`) at any depth — override and per-environment files — and nothing by
  content. A compose fragment with an arbitrary name (`common.yaml`, `base.yml`) is not
  the dialect's; it stays with the kubernetes dialect's `.yaml` extension claim.
  A workflow file named `docker-compose.yml` under `.github/workflows/` is claimed by its
  basename before github-actions' glob (§5 orders basename first); fusion's own dispatch
  still sends it to the GHA extractor, so only the reported language tag is affected.

## What is not modelled, and why

- **Only services are entities.** Compose's other top-level maps (`networks`, `volumes`,
  `secrets`, `configs`) are not emitted. Two independent reasons, either sufficient: the
  selector engine drops a null-valued map entry (`docNode` null is filtered from every
  match), and `networks:\n  front:` / `volumes:\n  dbdata:` — the common spelling — is
  null-valued, so a dialect could see only the minority declared with options; and
  `core.SymbolKey` carries no kind, so a volume named like its service (`db` and `db`,
  the norm) would share one symbol key and one of the two would vanish from the store the
  day dispatch switches. The engine has no literal qualifier that could keep them apart.
- **References are `depends_on` only** (list and map forms). `extends: {service: s}`
  is a real edge, but `docker compose config` merges the base into the extending service
  and erases the `extends` key, so no tier-A oracle attests it and the harness scores
  every emitted call against the oracle. `links:`, `volumes_from:` and
  `network_mode: service:x` need a string split (`db:alias`, `x:ro`), which the engine
  cannot do. The oracle runs `config --no-normalize` precisely so that the dependencies
  the tool infers from those keys do not count as `depends_on` truth.
- **Cross-file references are recorded, not resolved.** `extends.file` (from the
  service) and `include:` (a path, `{path}`, or `{path: [..]}`) are captured as
  `repo-path` cross-file references (`DocResult.CrossFile`, goldens in
  `extends-include.yaml` and `include-only.yaml`). `fusion.ResolveDocXrefs` joins a
  `repo-path` reference to the target file's *root entity* (an entity selected at `$`),
  and a compose file has none: it carries no scalar that is always present (top-level
  `name:` is optional) and the engine has no file-name value to name one with. Nor is a `build:` edge
  resolvable: a Dockerfile is a tags-engine file, absent from the doc-xref index
  (`LB-X-docker-image-ref`, deferred). No `build` cross-file entry is declared: the
  design's prototype (`testdata/doc/compose.doc.json`) resolves `build` through
  `repo-path`, which would join a service to the sole *doc* file in its context
  directory — a `package.json`, say — a guess, not the Dockerfile the edge means.
  Both `extends.file` and `include` will start resolving, with no dialect change, when a
  compose file has a root entity.
- **YAML merge keys (`<<: *anchor`) are not applied by the doc decoder.** Aliases are
  followed (`depends_on: *deps` works, `depends-forms.yaml`), but a `<<` key stays a key,
  so a service that inherits `image:` or `depends_on:` through a merge loses them (checked:
  `web: {<<: *base}` reads signature "" and no calls). The incumbent's typed decode did
  apply merge keys to `image:`/`build:`, so its signature survives a merge where the
  dialect's does not; its `depends_on` walk never did. Neither calibration subject uses
  `<<:`, so the reading does not exercise it; it is a decoder property shared by every doc
  dialect, not something the doc spec can express.

## The calibration oracle

Tier A, and the tool the queue entry names: `docker compose config --format json`
(Docker Compose 5.1.4, no daemon needed). `calibration/oracles/docker-compose/` runs it
once per compose file with `--profile '*'`, `--no-interpolate`, `--no-env-resolution`,
`--no-path-resolution`, `--no-consistency` and `--no-normalize`, so the tool resolves
anchors, merge keys and `extends:` exactly as `docker compose up` would and answers
which services the file defines and what each declares under `depends_on`.

Two things the tool cannot say, supplied by a `yaml.v3` walk of the file itself: the JSON
carries no source lines, so each service's line is its key's line in the file; and `include:` merges other files' services into the
answer, so a name the file does not itself declare under `services:` is dropped from its
entry (it is scored under its own file). A file the tool rejects contributes nothing:
`docker/compose`'s `pkg/e2e/fixtures/wrong-composefile/compose.yaml` (an unknown key,
invalid on purpose) is the one, and it is the reading's one dialect-only definition
(sym_precision .9962, not 1.0). Fathom still reads what it can from an invalid file,
which is the product behaviour the number should show.

A second, tier-C mode (`-xref`) writes the cross-file references a raw walk finds:
`extends.file` and `build:` (context + `dockerfile:`, when the Dockerfile exists in the
subject). Over the two subjects that is 36 edges — 35 `build:` edges onto Dockerfiles and
one `extends.file` edge onto `common.yaml` — and the harness drops every one as out of
scope (§1.4: the target is a file the dialect does not read), counting them in the
reading's notes. That count is the disclosure: L3 is not measured because there is
nothing scorable, not because it scored well. `include:` is not emitted: its origin is
the document, and a document with no root entity has no origin to name.

## Subjects

`docker/awesome-compose` (CC0-1.0, on §8.2's allowlist; 39 files, one sample per
directory) and `docker/compose` v2.39.4, `pkg/e2e/fixtures` only (Apache-2.0; 106 files:
multi-file, profiles, dependency shapes). The queue's second subject,
`getsentry/self-hosted`, is substituted out: its licence is the Functional Source
License, on §8.2's refused list.
