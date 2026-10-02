# envoy-oracle (tier A)

The language-breadth harness's oracle for the `envoy` doc-engine dialect (LB-Q-envoy, design
`docs/design/language-breadth.md` §1.3, §10). A separate Go module (`go.mod`: yaml.v3, protobuf-go,
go-control-plane v1.39.0), never imported by fathom, never a product dependency.

```
cd calibration/oracles/envoy && go build -o <scratch>/envoy-oracle . &&
  <scratch>/envoy-oracle -subject <checkout> -envoy <envoy> \
    -refs-out workspace/tmp/oracle/envoy-<subject>-refs.json \
    -routes-out workspace/tmp/oracle/envoy-<subject>-routes.json \
    -files-out workspace/tmp/oracle/envoy-<subject>-files.txt \
    > workspace/tmp/oracle/envoy-<subject>.json
indexbench lang-oracle -dialect envoy -subject <checkout> -tool-oracle <defs.json> -tool-oracle <refs.json> \
    -only "$(paste -sd, <files.txt>)" -out docs/bench/readings/lang/envoy.json
```

## The instruments

- **Acceptance: Envoy itself.** `envoy --mode validate -c <file>` (brew envoy 1.39.1, run in the file's
  directory so a relative xDS path resolves as the dialect resolves it) loads the bootstrap as the proxy does:
  unknown fields, bad addresses, a missing xDS file, a listener with no filter chain are refused. A file it
  refuses contributes nothing and is counted with its first error line on stderr. An xDS resource file (a
  `resources:` list of `@type` objects: no bootstrap) has no `-c` to validate; it is accepted when it decodes
  strictly as a DiscoveryResponse.
- **Entities and references: the API's own schema.** go-control-plane v1.39.0's generated v3 messages
  (the same `.proto` files Envoy's C++ parser reads), decoded with protojson, strictly (no unknown field is
  discarded), into a `Bootstrap` or a `DiscoveryResponse`. Every `typed_config` the registry knows is
  decoded (the HttpConnectionManager, which carries the route configuration, and the API's core messages); an
  extension it does not carry (a filter, a transport socket) is wrapped as `google.protobuf.Struct`, so it is
  skipped, not guessed. A reflective walk over the decoded messages finds every Listener, RouteConfiguration,
  VirtualHost, Route and Cluster, and what they name: no entity is read off the YAML.
- **Lines** are not parser output in either instrument. They come from `gopkg.in/yaml.v3` by the field path
  of each decoded message: a list item's own first line, a message held in a field at its key's line (the
  dialect's "where its key is written"). yaml.v3 is the decoder the dialect's engine reads lines with too; a
  line is a decoder detail, never an entity decision.

## Why this is tier A (design §1.3)

§1.3's ladder: the format's reference parser is A; a widely used third-party parser tool is B; an independent
library written against the format is C. Envoy's reference parser is its C++ protobuf JSON loader inside the
proxy; its reference SCHEMA is the `.proto` set. This oracle uses both ends: the proxy's own acceptance
decides which files are Envoy configurations (the reference parser's verdict), and the schema's generated Go
bindings, built from the same `.proto` files and checked here to agree with the proxy on every file (a file
Envoy accepts and protojson refuses, or the reverse, is excluded and counted as disagreement, never trusted),
name the entities. The Go bindings are the schema's own, not an independent implementation of the format, so
tier A is claimed for definitions and references. What stays below A: the listener identity rule for an
unnamed listener (the bind port, then the pipe path: Envoy gives it no stable name), and the route naming
rule (name, else match prefix, path or regex): both are the dialect's definition of identity and the oracle
mirrors them, said here rather than hidden.

## What it writes

stdout (tier A): definitions: a `package` per accepted file (named by the file, at its first key), a
`resource` per listener, a `section` per route configuration, a `selector` per virtual host, a `route` per
route that has a name or match, a `service` per cluster. `-refs-out` (tier A): references (a route to the
cluster it names, weights or mirrors to, a listener to the route configuration its `rds` names; each only when
the target is declared in the same file), cross_references (a ConfigSource `path` that names a file of the
subject, file granularity; a route's cluster or a listener's rds name declared in a file a bootstrap's xDS
paths load, entity granularity; the target resolved by the same repo-path rule as the dialect). `-routes-out`
(bench-only): route_references (a cluster endpoint's address that joins a docker-compose service or a
Kubernetes Service of the subject) and route_unresolved, with reasons `external`, `ip-literal`, `variable`,
`unix-socket`, `ambiguous`, `undeclared`. They are in their own file because `lang-oracle` refuses a tool
oracle with a field it does not know; the route layer's scorer (fusion.ResolveRouteRefs at integration) reads
this one.

`ORACLE_TCC_RESET` (optional): a shell command run between retries of a file read or write that fails with
"operation not permitted", for a machine whose external volume access is revoked mid-run; a failure that
persists is fatal, never swallowed.
