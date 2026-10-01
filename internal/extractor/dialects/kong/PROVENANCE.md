# kong — provenance and divergences

LB-Q-kong, design `docs/design/language-breadth.md` §4, §13; the px wave. A doc-engine dialect (no
`tags.scm`, no upstream grammar, no incumbent: a Kong file was a generic YAML or JSON file). The decode is
`gopkg.in/yaml.v3`, for `.json` too. Fixtures are written for this dialect from the declarative-configuration
shapes in decK's and Kong's documentation (Apache-2.0, Kong/deck, Kong/docker-kong).

## How a file is claimed: a name AND the content, never either alone

Manifest basenames `kong.yml|yaml|json`, `deck.yml|yaml` and the globs `**/kong*.{yaml,yml,json}`,
`**/*kong.{yaml,yml}`, `**/kong/**/*.{yaml,yml,json}`, `**/deck*.{yaml,yml}`; the guard is a top-level
`_format_version` (the spec's `detect.require_keys` holds it per document as well as the content sniff).
`fusion.gatewayDispatch` (LB-F104) claims a file only when both hold, in ingest and in `ReingestFile`. A
declarative file with another name (`expected.yaml`, `input.yaml` in decK's testdata) is not claimed:
claiming by content alone would leave the harness unable to measure it (`ContentDialect` is classify.go's).

## What it models

- **Services** (`KongService`, `service`), **routes** (`KongRoute`, `route`; top level and nested under a
  service), **upstreams** (`KongUpstream`, `resource`), **targets** (`KongTarget`, `element`; nested under
  an upstream and the top-level `targets` of the DB-less form) and **plugins** (`KongPlugin`, `policy`) at
  any depth. An entity is named by its `name` (a target by its `target` address); one without a name is not
  an entity.
- **References** (`Calls`, L2, `declared_only`): a route's `service` (a name or `{name: ...}`), a service's
  `host` naming an upstream of the file, a target's `upstream`, a plugin's `service` and `route`.
- **Cross-file references** (L3): `service-name` for the same five.
- **Route targets** (`route_ref`, `route-host`, LB-F106): a service's `host` and `url` and an upstream's
  `targets[].target`, as written; a top-level target's address too.

## Divergences

- **A nested route's service is containment, not a reference.** A route written under a service
  (`services[].routes[]`) names no `service:`; the doc engine reads downward from an entity and has no parent
  context, so no route-to-service edge is drawn for it. Its range sits inside its service's. decK's own
  rendering nests a flat route the same way, so the oracle's references are the ones WRITTEN (route.service,
  plugin.service|route, a service host naming an upstream), cross-checked by decK's nesting.
- **A service `host` that names an upstream is also a route target.** `route-host` is a syntactic claim the
  engine cannot condition on a declared upstream (no `undeclared_only`); `fusion.ResolveRouteRefs` sees the
  same name as an upstream, and the L2 reference says it is one (LB-Q-kong-rest: the resolver drops a
  route-host whose Raw names an entity of its own file, the integrator's call).
- **decK's strictness differs from Kong's DB-less loader.** decK rejects `route.service: name` (a string)
  and top-level `targets`, which DB-less Kong accepts; the dialect reads both, the oracle only the files decK
  accepts.
- **Consumers, ACLs, credentials** are not modelled.
