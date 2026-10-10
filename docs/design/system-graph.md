# The system graph: contract keys, links, and dependencies read from disk

Status: accepted 2026-09-27 (ADR-050). Vision: `docs/vision/38-proof-layer-roadmap.md`.
Plan: `docs/plans/system-graph.md` (SG-). Goals G17 (the system graph), G18
(dependencies) and G25 (the catalog, U10, ADR-051). Constraints carried in: ADR-044 (inventory keys, conservation), ADR-041
(SQLite is the record, no graph database), ADR-047 and invariants I47 to I51 (tenancy),
ADR-048 (dialects; the framework extractors ride on them), vision 15 §3 (ci-kit owns the
depgraph engine), law 4 (no tool per edge kind), invariants I56 and I57.

## 1. Problem

A call graph ends at a process boundary. The questions that matter most in a fleet cross it:
"if I change this handler's response, which services break", "who reads this column", "who
reads this flag", "what do we call in this library, and what does the v2 bump remove". Fathom
answers one of these today, badly: `service_ref` joins Go HTTP client calls to route
registrations at heuristic tier (VB-30). Inventory keys (`route:`, `rpc:`, `env:`, `cli:`)
exist for conservation only. `dep_drift` and `cross_repo_blast_radius` have zero lifetime
calls. Nothing grades a cross-boundary link's evidence, so nothing can safely gate on one.

## 2. Decisions (ADR-050 carries the record)

1. One model: nodes provide or consume typed contract keys; a link is a key match.
2. Four evidence tiers; links below `typed` never gate (I57).
3. One source port, one key table, one link table, three verbs.
4. Dependencies are the `pkg:` kind, indexed from local toolchain caches, never downloaded (I56).
5. The lockfile graph arrives through a port whose engine is ci-kit's.
6. Measured on a fixture fleet per kind and tier; co-change history is an independent oracle.
7. The decision port has no role in matching.

## 3. The contract key grammar

A key is `<kind>:<body>`, ASCII, whitespace-free except the single space in `route:`,
normalized by the kind's normalizer before storage. `ParseContractKey` refuses anything else
by name.

| Kind | Body | Normalization | Example |
|---|---|---|---|
| `route` | `METHOD /path/template` | method upper-cased; path params become `{}`; trailing slash dropped; query stripped (`serviceref.NormalizeTemplate`, kept) | `route:GET /orders/{}` |
| `rpc` | `package.Service/Method` | as ADR-044's inventory | `rpc:shop.v1.Orders/Create` |
| `gql` | `Type.field` (root types `Query`, `Mutation`, `Subscription` for operations) | type names as declared | `gql:Order.total` |
| `topic` | `name` | exact; a broker kind (`kafka`, `nats`, `sqs`, `pubsub`, `amqp`) is an attribute, not part of the key, so a rename of broker is not a new contract | `topic:orders.created` |
| `table` | `table` or `table.column`, optional `schema.` prefix | lower-cased unless quoted in DDL | `table:orders.total_cents` |
| `env` | `NAME` | as ADR-044 | `env:ORDERS_DB_URL` |
| `config` | `namespace/key` (a config file key path or a feature flag) | dots for nesting | `config:flags/checkout.v2` |
| `pkg` | `ecosystem/name@version#symbol` (purl-shaped; `#symbol` optional for package-level) | ecosystem lower-case (`golang`, `npm`, `pypi`, `maven`, `cargo`, `nuget`); version as the lockfile pins it | `pkg:npm/lodash@4.17.21#debounce` |
| `image` | `registry/repo` (tag and digest are attributes) | registry host lower-case | `image:registry.example/orders` |
| `deploy` | `k8s/<namespace>/<service>` or `host/<hostname>` | lower-case | `deploy:host/orders.example` |
| `cli` | `command path` | as ADR-044 | `cli:fathom verify` |

ADR-044's four kinds keep their exact spelling, so conservation reads the same keys.

## 4. Provides, consumes, and the four tiers

Every key row names a **role**: `provides` (a route registration, a proto service, an SDL
field, a producer's topic declaration, a migration creating a column, a k8s Service, a
package export) or `consumes` (a client call, a stub call, a query document, a subscriber, a
SELECT or ORM read, an `os.Getenv`, an import of a package symbol). Writes and reads of
tables and topics are both recorded; `table:` rows carry `access: read|write` so lineage
("who writes this column") is a filter, not a new kind.

A link is (consumer node, provider node, key, tier, evidence). Tiers, strongest first:

| Tier | Meaning | Example |
|---|---|---|
| `spec` | both ends bound through a contract document | a Go handler registered for an OpenAPI `operationId`, consumed by a client generated from the same spec |
| `typed` | a typed pass or a generated stub resolves the consumer to the provider's declaration | `pb.NewOrdersClient(conn).Create(...)` resolved by the Go typed pass to the proto's `Create` |
| `literal` | the normalized literal key is equal at both ends | `http.Get(base + "/orders/" + id)` resolving to `route:GET /orders/{}` with no hole left unexplained |
| `template` | a key with unexplained holes matched to a template | `fetch(`${api}/${kind}/list`)` matched to `route:GET /orders/list` |

A link's tier is the weaker of its two ends. `service_ref` becomes the `literal` and
`template` tiers of `route:` links. **I57:** a `literal` or `template` link is served with its
tier named and never passes or fails a verify row, a conservation plane or a review verdict.

## 5. Sources: extractors per framework

One port, `ContractSource`: `Keys(file, content, dialect) -> []KeyRow` for code and
contract documents, plus `FleetKeys(service) -> []KeyRow` for infrastructure that names other
services. Sources register by dialect id (ADR-048), so a framework extractor is a query or
selector file where possible and Go only where the framework needs resolution.

| Kind | Providers | Consumers | Wave |
|---|---|---|---|
| `route` | Go net/http, chi, gin, echo, gorilla (exists, `routes.ExtractFile`); Express, Fastify, Nest; FastAPI, Flask, Django; Spring MVC and WebFlux; ASP.NET minimal and controllers; Rails routes | Go net/http (exists); fetch, axios, ky; requests, httpx, aiohttp; RestTemplate, WebClient, Feign; HttpClient (.NET); Faraday, Net::HTTP; generated OpenAPI clients recognized by generator headers | 2 (Go, JS/TS, Python); 3 (JVM, .NET, Ruby) |
| `route` (spec) | OpenAPI 3.x documents (`paths.*.*`, `operationId`) | clients generated from them | 2 |
| `rpc` | `.proto` service definitions (exists in inventory); Connect, Thrift IDL | Go gRPC stubs (typed); TS and Python stubs by generated-file markers | 2 |
| `gql` | GraphQL SDL; resolver maps (gqlgen, Apollo, Strawberry) | `.graphql` operation documents; `gql` tagged templates | 3 |
| `topic` | AsyncAPI channels; producer calls (Kafka, NATS, SQS, Pub/Sub, AMQP clients) with literal names; Avro schema subjects | consumer and subscriber registrations | 3 |
| `table` | SQL migrations (CREATE/ALTER), ORM models (GORM, SQLAlchemy, Django, ActiveRecord, JPA, Prisma schema) | SQL string literals parsed by the SQL dialect; ORM query calls resolved to their model | 3 |
| `env`, `config` | k8s manifests, compose, Helm values, `.env` examples, Terraform outputs; feature-flag definition files | `os.Getenv`, `process.env`, `os.environ`, `System.getenv`, config library lookups with literal keys, flag client calls | 3 |
| `image`, `deploy` | Dockerfile build targets, k8s Deployments and Services, Ingress hosts, compose services | k8s and compose references, client base URLs that name a cluster host | 3 |
| `pkg` | the dependency API indexer (§8) | import-resolved and typed references to package symbols | 3 |
| `cli` | as ADR-044 | scripts and CI steps invoking the command | 3 |

Every source records its extractor id and epoch on the row, so a changed extractor
re-derives its keys without guessing.

**What peers cover (a check on this list, not a source of it).** The survey of about 30 peer stacks
(`docs/assessments/2026-09-27-peer-stacks-and-rust.md`, data in
`docs/assessments/data/2026-09-27-peer-stacks.json`, read 2026-09-27) found cross-service linking in four of
them: CodeGraph (route nodes and handler edges across 17 web frameworks, plus native-to-JS bridge resolution),
GitNexus (`route_map` and `api_impact`, Spring Actuator runtime endpoints, AsyncAPI channel modeling with
direction-safety refusals), codebase-memory-mcp (cross-service HTTP linking and IaC nodes for Dockerfiles,
Kubernetes and Kustomize) and Sourcegraph (cross-repository search and navigation, no HTTP-route feature
confirmed). The `route`, `topic`, `env` and `image` rows above already cover those shapes; a framework named
there that this table lacks is a candidate row, judged by the same evidence tiers (I57).

### Framework routes (wave c2)

The route seam (`internal/analysis/routes`, LB-F321) is how a framework's routes reach the `route:` key
without a table or column. Its contract, which every framework reader codes against:

- **Key.** `route:` is the method plus `serviceref.NormalizeTemplate(path)` (`internal/syslink/key.go`
  normRoute). `internal/fusion/routeref.go` is the `deploy:` half only.
- **Readers.** `Reader{Framework, Langs, Read(path, src) Result}`, registered by `Register` from an
  `init()` in the reader's own file. A duplicate `Framework` panics. Readers run in `Framework` order.
  `Langs` are `connector.LanguageForPath` tokens (`go`, `java`, `php`, `py`, `js`, `ts`, ...).
  `HasReader(lang)` says whether any reader reads a language.
- **One file.** `ExtractFile(lang, path, src) Result` runs the Go AST reader (for `go`) plus every reader
  registered for `lang`. Routes come back sorted and deduplicated on (method, path, handler, line).
  (`Extract`, its routes alone, went in wave lf2 with its last caller.)
- **Mounts.** `Result.Mounts` holds `Mount{Prefix, Target, File, Line}`: every route of the file(s)
  `Target` names is served under `Prefix`. `Target` takes one of three forms:
  - `./x` or `../x`, relative to the mounting file's directory;
  - `/x`, repo-relative;
  - a dotted module `a.b.urls`, matched as the path suffix `a/b/urls.<ext>` or `a/b/urls/__init__.py`.

  For the first two forms, a trailing `/` names every file directly inside a directory, `/**` every
  file under it to a depth of 8 folders (Symfony's recursive directory import, wave lf2), and
  `/**?dirs` the same files with each file's folders below the directory joined to the prefix
  (@fastify/autoload's `dirNameRoutePrefix`). Otherwise the target is the file itself, else
  `x.<ext>`, else `x/index.<ext>` or `x/__init__.py`.
- **Compose.** `Compose(files []Result) []Route` applies the mounts across one service's files:
  - the prefix is joined to each path with exactly one `/`;
  - a mounted file's own mounts compose recursively, to depth 8 and cycle-guarded;
  - a file mounted N times yields its routes N times;
  - a file no mount names keeps its local paths, and a file's own mount never mounts it (a
    directory tree that holds its mounting file);
  - handler, file and line are unchanged, and the output is sorted.
- **Storage.** `serviceref.ExtractFile` writes each mount as a `service_routes` row with method
  `MOUNT`, path = prefix and handler = target, every Target form as written. `serviceref.BuildFromStore`
  (through `RoutesFromRows`) and `inventory.Extract` compose before use, so a mount row never reaches a
  hop or an inventory item (`TestServiceRefs_NoMountRowLeaks`).
- **Keys.** Every consumer of a `route:` key reads composed routes (wave lf2, LB-F353). syslink's
  `routeProvides` is a whole-service source, like `protoRPCKeys`: it reads a service's files as
  ingest stores them and composes them through `RoutesFromRows`, so its `route:` provides rows name
  the paths the hops join on, never a local path or a mount (`TestRouteKeys_ComposedInEveryConsumer`).
  A per-file `ContractSource` cannot compose, so `route:` provides have none; the link refresh that
  will run the sources (SG-10) calls `routeProvides` per service.
- **Handlers.** A handler is named as the language's dialect names the symbol (Java `Class.method`,
  nested `Outer.Inner.method`), so a route binds to its indexed handler by name.
- **Measurement.** `indexbench route-oracle` scores what ingest stores and `RoutesFromRows`
  composes against a running framework's own route dump (`calibration/oracles/routes/README.md`).
  It reports route recall and precision on (method, normalized path), and bound recall and
  precision, which also require the handler. The verdict is PASS when both bound metrics are at
  least .90.

The seam moved the legacy regex pattern sets unchanged into two readers: `express-regex` (js, ts) and
`python-regex` (py). Spring MVC (`spring`) and JAX-RS (`jaxrs`) read java over one byte-level declaration
scanner. Spring's class-level `@RequestMapping` and JAX-RS's class `@Path` compose inside the file, so
neither reports a mount. Two shapes are ceilings, with a design kept open
(`docs/bench/readings/routes/java.md`):

- A JAX-RS sub-resource locator would be a mount whose target is its return type's dotted name. The
  dotted-module form already resolves it.
- A deployment root path (`quarkus.rest.path`, a servlet context path, `@ApplicationPath`) needs a
  target that names a whole module. The `/**` form is that target (wave lf2); what is missing is a
  reader of the configuration that sets the root (O693).

## 6. Store tables (additive to store format v0)

```
contract_keys(service_id, key, kind, role, node_kind, node_id, file, line,
              tier_hint, access, attrs_json, extractor, extractor_epoch)
  index (key, role), (service_id, file)
system_links(consumer_service, consumer_node, provider_service, provider_node,
             key, kind, tier, evidence_json, refreshed_at)
  index (provider_service, provider_node), (consumer_service, consumer_node), (key)
dep_packages(pkg_id, ecosystem, name, version, digest, source_path, api_bytes,
             indexed_at, last_used_at, state)          -- state: indexed|pending|refused
dep_api(pkg_id, symbol, kind, signature, file, line)   -- public surface only
```

`pkg_id` is the digest-addressed identity `ecosystem/name@version` plus the content digest,
so two services pinning the same version share one row set (the fleet store is already
shared, ADR-025). All four tables are additive; no existing column changes.

## 7. The verbs (three, for every kind)

- **`links`** `{service, node | file | key, direction, kinds, min_tier}`: what a node provides
  and consumes and who is on the other side, each link with its tier and both ends' file:line.
- **`system_impact`** `{service, changed_files | symbol | key, depth, min_tier}`: the change's
  reach across services and dependents. It walks the local call graph to the nodes that
  provide keys, then the links to consumers, then each consumer's local callers, to `depth`.
  Output groups by kind and tier; `literal` and `template` results are labelled hints.
- **`contract_drift`** `{service, base, head}` or `{key}` or `{pkg, from, to}`: a producer's
  contract against its consumers' usage. Per kind, a closed list of breaking changes (a
  removed route, a changed method, a removed or retyped field, a removed topic, a dropped
  column, a removed env key, a removed or re-signatured package symbol). Upgrade impact is the
  `pkg` form: the removed or changed symbols of `to` that our code consumes, with call sites.

Each verb is a CLI verb first with a `--json` envelope (law 10), then an MCP tool in an
opt-in group; it joins the default agent surface only on call-log justification. When the
three land, `dep_drift` and `cross_repo_blast_radius` are retired (SG-13, SG-25).

## 8. Dependencies (N1)

### 8.1 The lockfile graph, through a port

`DepGraph(ecosystem, root) -> nodes, edges, lockDigests` (vision 15 §3's signature). The
in-tree adapter (`internal/depplane/lockfile`) reads `go.mod`/`go.sum`, `package-lock.json`,
`pnpm-lock.yaml`, `yarn.lock`, `uv.lock`, `poetry.lock`, pinned `requirements*.txt`,
`Cargo.lock`, Gradle lockfiles and Maven `pom.xml` (declared-only tier when no lock
exists). It is deleted when ci-kit ships the engine.

### 8.2 Reading what is already on disk

The cache locator resolves each locked (ecosystem, name, version) to a directory the
toolchain already populated, read-only:

| Ecosystem | Where | What is read |
|---|---|---|
| Go | `$GOMODCACHE` (`go env GOMODCACHE` value is read from the environment, never by running the tool on a request) | exported declarations via `go/parser` |
| npm | the service's `node_modules`, the pnpm content store | `.d.ts` typings (the TypeScript dialect), else the package `exports` map |
| PyPI | a project `.venv`, the uv cache | `.pyi` stubs, else top-level `def`/`class` honoring `__all__` |
| Maven | `~/.m2/repository`, Gradle's module cache | `-sources.jar` with the Java dialect; without sources, public members from class files by a pure-Go reader (wave 3b) |
| Cargo | `~/.cargo/registry/src` | `pub` items via the Rust dialect |
| NuGet | `~/.nuget/packages` | deferred to wave 4 (reference assemblies need a metadata reader) |

A package absent from every cache is recorded `refused` with the reason "not on disk; run
your package manager", never fetched (I56).

### 8.3 Indexing once per machine

A package version is indexed at most once per machine: `dep_packages.digest` keys it, the
first service that needs it enqueues it in the background ingest class (ADR-047), and every
later service links to the same rows. Only the public API surface is indexed (symbols,
kinds, signatures, file:line), capped at 8 MiB of source read and 2 MiB per file. A request
touching an unindexed package answers `pending` with the reason and the queue position.
Rows unused for 30 days are evicted by `fathom gc`.

### 8.4 Power features

- **Upgrade impact:** `contract_drift {pkg, from, to}` diffs the two versions' `dep_api` and
  joins removed and re-signatured symbols to our `consumes` rows. Both versions must be on
  disk; if `to` is not, the answer names the one command the user would run.
- **Usage map:** `links {key: "pkg:npm/lodash@4.17.21"}` lists which of our code touches which
  API symbol.
- **Vuln reach (wave 4):** the user supplies an OSV JSON file; an advisory naming a function is
  mapped to a `pkg:` key and `system_impact` walks backwards to our entrypoints. No advisory is
  fetched.
- **Version-exact docs:** served by knowledge retrieval (`docs/design/knowledge-retrieval.md`
  §7) from the same cache paths.

## 9. Co-change: an independent oracle

`blast_radius` and `system_impact` are graded against what git history shows changing
together: for each commit touching a provider node in the last 12 months, the files changed
in the same commit or in a linked commit in a consumer repository within 7 days. The miner
is bounded (at most 2,000 commits per service, streamed through `gitro`), runs in the
measurement harness, and never feeds the verbs, so the oracle cannot grade the graph with
itself.

## 10. Live updates (wave 4)

A file watcher refreshes keys and links for changed files under the budget, gated by an
equivalence check: after N incremental updates, a full re-derivation of the same service
must produce byte-identical `contract_keys` and `system_links` rows. A mismatch disables the
watcher for that service with a named reason until the next full ingest.

## 11. The service catalog that cannot rot (U10, ADR-051)

A per-service view over the system graph, computed, never hand-written: owners (CODEOWNERS
through `internal/ownership`, and the most recent committers of the service's provider nodes
shown as agent or commit identities, never ranked), APIs provided and consumed (`links` by
role), dependencies (`pkg:` keys by ecosystem), data touched (`table:` by access), config
read (`env:` and `config:`), certificates (live, flaky, broken counts for the service's
providers), and docs (the knowledge-retrieval units linked to the service's keys). Rendered on
the console's project workspace. Export to a Backstage `catalog-info.yaml` shape happens only
when asked (`fathom catalog export --service S`), written to a path the user names; fathom
never commits it. SG-M6 reads the catalog for every fleet service with each field's source
named.

## 12. Tenancy and cost

| Cost | Bound |
|---|---|
| Key extraction | inside ingest's existing pass per file; no second read of a file |
| Link refresh | only keys an ingest changed; an indexed equality join; must stay under 10% of ingest wall time (SG-M5) |
| Request path | index lookups on `key` and node; no table scan, no request proportional to repository size (I48) |
| Dependency index | lazy, background class, one per package version per machine, 8 MiB source cap, evictable; total on-disk rows at most 256 MiB before the Revisit trigger |
| Memory | parser modules come from ADR-048's pool (at most six live); link and key caches are `memo` groups under the byte budget (I51) |
| Network | none (I56) |

## 13. Measurement

**The fixture fleet.** Fauxtown gains services until every kind has at least one provider and
two consumers across two languages: an OpenAPI-first HTTP service, a gRPC service with Go and
TypeScript clients, a GraphQL gateway, a producer and consumer pair on a topic, a service
with migrations and an ORM, a config-heavy service with k8s and compose manifests, and a
service pinning dependencies in each of the five G18 ecosystems with two versions of one
package on disk. An answer key (`internal/connector/testdata/fauxtown/links.answer.json`)
lists every true link with its expected tier; seeded negatives (a dead route, a renamed
topic, a string that looks like a URL) guard precision.

**The instrument.** `indexbench link-oracle` ingests the fleet, reads `system_links`, and
writes precision and recall per kind and tier to `docs/bench/readings/system-graph/`.

| Reading | Threshold |
|---|---|
| SG-M1 | spec+typed tiers per kind: precision at least 0.95, recall at least 0.85 |
| SG-M2 | `contract_drift` catches at least 90% of 40 seeded breaking changes, at most 1 false breaking per 20 non-breaking diffs |
| SG-M3 | `system_impact` on the home fleet, every link's tier named, compared with co-change (§9) |
| SG-M4 | upgrade impact precision at least 0.9 on the seeded-upgrade corpus, per ecosystem |
| SG-M5 | one index per package version fleet-wide; link refresh under 10% of ingest; `phys_footprint` within budget; zero network dials |
| SG-M6 | the service catalog for every fleet service, each field's source named |

## 14. Jev, weighed

- **Where it could help:** ranking `template`-tier candidates when several routes match a
  templated URL.
- **Where it cannot:** deciding a link. Links gate (at `spec` and `typed`) and must be
  deterministic (P2); the route site already failed calibration (45.7% right at confidence at
  least 0.9, 2026-09-24).
- **Cost:** a call per ambiguous candidate set on the ingest path, which is a tenancy cost on
  the hottest background job.
- **The measurement that would reopen it:** an adjudicated sample of at least 100 template
  candidates where the port's top choice is right nine in ten at its shown confidence. Until
  then it has no role.

## 15. Refused

- A tool per edge kind (law 4).
- A graph database for links (ADR-041).
- Downloading packages, advisories or docs (I56).
- Indexing private dependency bodies.
- Building the depgraph engine in fathom (vision 15 §3).
- Letting `literal` or `template` links gate (I57).
- Jev in link matching (§13).
