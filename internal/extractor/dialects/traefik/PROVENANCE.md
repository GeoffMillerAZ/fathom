# traefik — provenance and divergences

LB-Q-traefik, design `docs/design/language-breadth.md` §4, §13; the px wave. A doc-engine dialect (no
`tags.scm`, no upstream grammar, no incumbent: a Traefik static or dynamic file was unclassified YAML or the
generic TOML extractor's bare keys). Fixtures are written for this dialect from the file-provider examples in
Traefik's documentation (MIT, traefik/traefik).

## YAML and TOML through one spec

Traefik reads both. A doc spec names one decoder, so `doc.json` decodes `yaml`, and `decodeDoc` reads a file
that OPENS as a TOML document (a table header `[a.b]` or an assignment `key = value` as its first line that
is neither blank nor a comment) with the TOML decoder under the same selectors. No YAML document a dialect
reads opens with either; a file that looks like TOML but is not falls through to the YAML decoder. (This is
the one change to the doc decode: `internal/extractor/docdecode.go`, `looksTOML`; no spec field is added.)

## How a file is claimed: a name AND the content, never either alone

Manifest basenames `traefik.yml|yaml|toml`; globs `**/traefik*.{yml,yaml,toml}`,
`**/traefik/**/*.{yml,yaml,toml}`, `**/dynamic*.{yml,yaml,toml}`, `**/rules/*.{yml,yaml,toml}`. The guard,
`detect.content_sniff`, is Traefik-shaped, not a bare key: `http|tcp|udp:` followed by `routers|services|
middlewares|serversTransports|models`, `entryPoints:`, `providers:` followed by a provider name, `tls:`
followed by `certificates|options|stores`, or a TOML table of those. `fusion.gatewayDispatch` (LB-F104)
claims a file only when both hold.

## What it models

- **The file** (`TraefikConfig`, `package`) named by the file: what `providers.file` resolves to.
- **Entry points** (`TraefikEntryPoint`, `resource`, a static file's `entryPoints.*`), **routers**
  (`route`), **services** (`service`) and **middlewares** (`rule`) of the http layer, and the tcp
  (`TraefikTCPRouter`, `TraefikTCPService`, `TraefikTCPMiddleware`) and udp (`TraefikUDPRouter`,
  `TraefikUDPService`) layers, each named by its key.
- **References** (`Calls`, L2, `declared_only`): a router's `service`, `middlewares[]` and `entryPoints[]`; a
  service's weighted, mirroring and failover services; a middleware chain.
- **Cross-file references** (L3): `repo-path` from a static file's `providers.file.filename` and `directory`
  (relative to the static file) to the dynamic file's package entity; `service-name` for a router's service,
  middlewares and entry points that live in another file (an entry point is a static file's, a router a
  dynamic file's).
- **Route targets** (`route_ref`, `route-host`, LB-F107): an http service's `loadBalancer.servers[].url`; a
  tcp or udp service's `.address`, as written.

## Divergences

- **A provider suffix is not stripped.** `service: api@file` names the file provider's `api`; the engine
  cannot strip `@file`, so only an unsuffixed name is a reference (a name that is not declared in the file is
  dropped). The oracle's truth keeps the suffixed ones, so recall pays.
- **Docker labels and Kubernetes IngressRoute are the route-reference item's**, not this dialect's.
- **A Go template is not decoded.** Traefik renders `{{ }}` in a file-provider file before decoding it
  (integration/fixtures use `{{ .SelfFilename }}`); a file that is not valid YAML or TOML as written yields
  nothing, and the oracle counts it.
- **Static-file keys other than entryPoints and providers.file** (log, metrics, certificatesResolvers) are
  not modelled.
