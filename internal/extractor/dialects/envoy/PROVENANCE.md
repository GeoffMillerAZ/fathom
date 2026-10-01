# envoy — provenance and divergences

LB-Q-envoy, design `docs/design/language-breadth.md` §4, §13; the px wave (proxies and gateways).
A doc-engine dialect (no `tags.scm`, no upstream grammar, no incumbent: an Envoy file was a generic
YAML or JSON file read as bare keys, or an unclassified YAML). The decode is `gopkg.in/yaml.v3`
through the doc engine, for `.json` too (YAML reads JSON). Fixtures are written for this dialect (no
upstream text is copied); the shapes follow the Envoy documentation's front-proxy, xDS and
file-based-dynamic-configuration examples (Apache-2.0, envoyproxy/envoy and envoyproxy/examples).

## How a file is claimed: a name AND the content, never either alone

The manifest names `envoy.yaml|yml|json` and the globs `**/*envoy*.{yaml,yml,json}` and
`**/envoy/**/*.{yaml,yml,json}`. `doc.json`'s `detect.content_sniff` is the guard: a top-level
`static_resources` or `dynamic_resources` (a column-0 YAML key, or a JSON key), or an `@type` that names
an `envoy.` type URL (an xDS resource file). `fusion.gatewayDispatch` (LB-F104) claims a file only when a
manifest basename or glob matches AND the sniff passes, in ingest (`resolveExtractor`) and in
`ReingestFile`; a file that opens as a Kubernetes object (a top-level `apiVersion` and `kind`) is never
claimed, so `envoy-deployment.yaml` stays a Deployment. The harness (`lang-oracle`) runs the dialect on
the files the manifest owns, so its `-only` list is the files the oracle accepted.

## What it models

- **The file** (`EnvoyConfig`, canonical `package`): one entity per file, named by the file (`$file`), at
  its first key. It is what a file-based xDS path resolves to.
- **Listeners** (`EnvoyListener`, `resource`): `static_resources.listeners[*]` and an xDS `Listener`
  resource. A listener without a name is named by its bind port, then its pipe path (Envoy gives an
  unnamed listener no stable identity beyond its address).
- **Route configurations** (`EnvoyRouteConfiguration`, `section`): an inline `route_config` at any depth
  (an HttpConnectionManager's) and an xDS `RouteConfiguration` resource; **virtual hosts**
  (`EnvoyVirtualHost`, `selector`) and **routes** (`EnvoyRoute`, `route`) anywhere under one. A route is
  named by its `name`, else its match `prefix`, `path` or `safe_regex.regex`; a route with none is not an
  entity.
- **Clusters** (`EnvoyCluster`, `service`): `static_resources.clusters[*]` and an xDS `Cluster` resource.
- **References** (`Calls`, L2, each only when the target is declared in the same file: `declared_only`):
  a route's `cluster`, each `weighted_clusters.clusters[].name` and `request_mirror_policies[].cluster`; a
  listener's `rds.route_config_name`.
- **Cross-file references** (L3): `service-name` for the same four (the one entity of that name in
  another file); `repo-path` for every file-based xDS path (`path_config_source.path`, a `lds_config`,
  `cds_config`, `eds_config` or `config_source` `path`), resolved from the referring file's directory to
  the target file's package entity.
- **Route targets** (`route_ref`, strategy `route-host`, LB-F105): every `socket_address.address` under a
  cluster, as written (the `load_assignment` endpoint's, and the v2 `hosts[]` spelling). The routeref item's
  `fusion.ResolveRouteRefs` joins them to a deployed service and discloses the rest by reason; a cluster
  name a route writes is an L2 reference, never a route target.

## Divergences, each disclosed (and where it is a ceiling, a -rest row)

- **A container path is not a repo path.** The examples and Supabase write `path: /etc/envoy/cds.yaml`, the
  path inside the container; the compose file mounts `./volumes/api/envoy/cds.yaml` there. `repo-path` reads a
  leading `/` as the repository root, so those edges resolve to nothing; joining them through the compose
  volume mount is the compose dialect's (tl) and the route layer's work (LB-Q-envoy-rest).
- **An extension the schema does not know is skipped, not read.** A filter's `typed_config` is never
  entered except the HttpConnectionManager's route configuration; a route inside another extension's
  config (a thrift or dubbo proxy) is not an entity.
- **A route's own `cluster_header`, `cluster_specifier_plugin`, a TCP proxy's `cluster` and an
  `eds_cluster_config`** name clusters the dialect does not draw as references.
- **No parent context in the doc engine.** A reference is read downward from its entity, so nothing here
  needs it; the same limit shapes Kong (nested routes).
- **A scalar is a string.** `port_value: 8080` is `"8080"`, which is what names an unnamed listener.
- **Templates.** `*.template.yaml` files that are Jinja, not YAML, do not decode and yield nothing.
