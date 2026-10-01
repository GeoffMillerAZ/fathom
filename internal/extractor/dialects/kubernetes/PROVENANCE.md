# kubernetes — provenance and divergences

LB-Q-kubernetes, design `docs/design/language-breadth.md` §4. A doc-engine dialect
(no `tags.scm`, no upstream grammar), the port of `internal/extractor/k8sextract.go`.
Its reading is measured on the dialect itself (`lang-oracle`'s dialect path); ingest
still dispatches Kubernetes manifests to the incumbent (`fusion` `toolExts[ToolK8sManifest]`).
Both name an object `Kind/<namespace>/<name>` (`Kind/<name>` with no namespace).

## Route references (LB-F96, wave px, 2026-10-01)

The `route-host` entries of `cross_file` name the host a route, mesh rule or proxy
route sends traffic to; `fusion.ResolveRouteRefs` joins it to the Service (or compose
service) declaring it, by the Kubernetes and Istio DNS rules (design §4). The referring
entity is the routing object itself: the dialect's other entities are not stored while
the incumbent is dispatched, so a per-rule entity would join no stored node. Until the
dispatch switch, ingest's route pass reads only these `route-host` references from the
dialect and joins them to the incumbent's nodes (`fusion.routePort`).

| Kind | Selected | Notes |
|---|---|---|
| `Ingress` (networking.k8s.io/v1, extensions and networking v1beta1) | `spec.rules[*].http.paths[*].backend` `.service.name` (v1) or `.serviceName` (v1beta1); `spec.defaultBackend.service.name` (v1); `spec.backend.serviceName` (v1beta1) | a `resource` backend names no Service and is not read |
| Gateway API `HTTPRoute`, `GRPCRoute`, `TLSRoute`, `TCPRoute`, `UDPRoute` | `spec.rules[*].backendRefs[*].name` | see divergences |
| Istio `VirtualService` | `spec.http[*]`, `spec.tcp[*]` and `spec.tls[*]` `.route[*].destination.host` | mirrors and delegates are not read |
| Istio `DestinationRule` | `spec.host` | |
| Istio `Gateway` | `spec.servers[*].hosts[*]` | `namespace/dnsName` is read as the dnsName; most are public hosts or `*` (external) |
| Traefik `IngressRoute`, `IngressRouteTCP` | `spec.routes[*].services[*].name` | |

Divergences, all forced by §4's selector subset (a value is one scalar; a `where`
filters the referring entity, never the selected node):

- **Gateway API backendRef `namespace` and `kind`.** A backendRef with an explicit
  `namespace` (a cross-namespace reference under a ReferenceGrant) is read as a name in
  the route's own namespace, and one whose `kind` is not `Service` (a `ServiceImport`,
  an implementation's own backend kind) is read as a Service name. The oracle reads
  both fields, so the measurement counts every such reference against fathom.
- **Traefik `services[].kind`.** A `TraefikService` (weighted or mirrored, declared by
  Traefik) is read as a Service name, likewise; `services[].namespace` likewise.

## Calibration

Subjects in `calibration/corpus.d/kubernetes.yaml`; the tier-C oracle is
`calibration/oracles/kubernetes` (an independent `gopkg.in/yaml.v3` walk), which writes
`route_references` and `route_unresolved` by the documented DNS rules, with an Istio
cross-check from `istioctl analyze --use-kube=false` (IST0101). The harness is
`indexbench lang-routes` (LB-F98).
