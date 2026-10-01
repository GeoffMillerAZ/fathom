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
- **Cross-file references resolve through a root entity and the image-tag index**
  (LB-X-docker-image-ref). Every compose file has a root entity, `ComposeFile`, named by
  the file itself (`$file`, signed by the optional top-level `name:`): the origin of its
  `include:` references and the target that `include:` and `extends.file` (both
  `repo-path`) resolve to in `fusion.ResolveDocXrefs`. A service's `image:` is an
  `image-ref` reference (edge `runs`), resolved through the doc-xref pass's in-memory
  image-tag index to the root stage of the in-repo Dockerfile that builds that image. Its
  build is two `repo-path` references the resolver joins: `build` (the context, `build:` or
  `build.context`) and `build_dockerfile` (`build.dockerfile`, relative to the context);
  together they name one Dockerfile, Docker's rule (`<context>/Dockerfile` by default, the
  referring file's directory when no context is written). A service carrying both
  `image:` and a build is one of the index's three build sites (with docker-bake targets
  and a literal `docker build -t` in a script, a Makefile or a CI run step). The design's
  prototype (`testdata/doc/compose.doc.json`) resolved `build` as a plain directory, which
  joined a service to the sole doc file in its context (a `package.json`, say); the
  resolver's build rule names the Dockerfile instead. `extends: {file, service}` resolves
  to the file's root entity, not to the service it names (the engine records the file).
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
once per compose file with `--env-file /dev/null`, `--profile '*'`, `--no-interpolate`,
`--no-env-resolution`, `--no-path-resolution`, `--no-consistency` and `--no-normalize`, so
the tool resolves anchors, merge keys, `include:` and `extends:` exactly as `docker compose
up` would and answers which services the file defines and what each declares under
`depends_on`. No `.env` is read — the project's (`--env-file`) nor an included project's
(a file with `include:` is read through a scratch copy beside it whose entries carry
`env_file: /dev/null`): with `--no-interpolate` nothing in the answer depends on one, and
Viren070/docker-compose-template's app `.env` files require variables the subject leaves
unset (`${AIOLISTS_HOSTNAME?}`), which would fail its root file and every include in it.

Two things the tool cannot say, supplied by a `yaml.v3` walk of the file itself: the JSON
carries no source lines, so each service's line is its key's line in the file; and `include:`
merges other files' services into the answer, so a name the file does not itself declare
under `services:` is dropped from its entry (it is scored under its own file). A file the
tool rejects contributes no definition: `docker/compose`'s
`pkg/e2e/fixtures/wrong-composefile/compose.yaml` (an unknown key, invalid on purpose) is the
one, and it is the reading's one dialect-only definition (sym_precision .9981, not 1.0).
Fathom still reads what it can from an invalid file, which is the product behaviour the
number should show. The tool names no file entity, so the root entity (`ComposeFile`) is
outside the L1 kind intersection (the reading's `excluded_by_kind`) and is checked through
L3, where every include and extends edge lands on one. An oracle that synthesized a
`package` per accepted file was tried: it scores the root of the invalid fixture above as a
second dialect-only definition, and `lang-parity` then refuses the dispatch on a kind the
incumbent never had (sym_precision .9976 against .9981), so it is not the oracle of record.

The same tier-A run attests the cross-file references (`cross_references`, file
granularity): an `include:` entry whose included file declares a service that appears in
the tool's answer for the including file, an `extends.file` of a file the tool accepted (an
extends whose file or service does not resolve fails the whole file), and a `build:` as the
tool evaluated it, naming a Dockerfile of the subject. For a file the tool rejects, each
include and extends is probed alone (a scratch file beside it holding only that entry).
The build edges land on Dockerfiles, files the dialect does not read, so the harness drops
them as out of scope (§1.4) and counts them in the reading's notes; so are the includes of
fragments no compose name rule claims (`common.yaml`, `apps/plex.yaml`,
`compose-include/minio.yaml`): an ingest reads those by content (a top-level `services:`)
and resolves the edge, the harness does not own them (fragment ownership is the harness's
`ownedSourceRefs`).

A second, tier-C mode (`-xref`) writes the same cross-file references from the raw YAML
alone, unattested: `include:`, `extends.file` and `build:` (context + `dockerfile:`, when
the Dockerfile exists in the subject). On the five subjects below it finds exactly the
tier-A set.

## Subjects

`docker/awesome-compose` (CC0-1.0, on §8.2's allowlist; 39 files, one sample per
directory), `docker/compose` v2.39.4, `pkg/e2e/fixtures` only (Apache-2.0; 106 files:
multi-file, profiles, dependency shapes), and three subjects for L3, each a public
repository whose compose files include or extend other compose files, licence checked
against §8.2: `Viren070/docker-compose-template` 25b1b1f (MIT; 127 files, a root
`compose.yaml` including 117 `apps/*/compose.yaml`, and one app including two more),
`juftin/homelab` ff207d3 (MIT; 4 files, a root including one compose-named file and 14
fragments), and `qclaogui/codelab-monitoring` 8e947fb (Apache-2.0; 24 files, 4 includes of
compose-named files and 62 of shared fragments). The in-scope L3 truth is 125 include edges;
no `extends.file` of these subjects lands on a compose-named file (docker/compose's one lands
on `common.yaml`), so extends is covered by the resolver's tests, not by the reading. The queue's second subject, `getsentry/self-hosted`, is substituted out: its
licence is the Functional Source License, on §8.2's refused list.
