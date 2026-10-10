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
| Istio `ServiceEntry` (LB-F307) | `spec.hosts[*]`, `spec.endpoints[*].address` | each non-wildcard host is also a provider of its `deploy:` key (the side pass), so a route to it joins the entry and the entry's own host is a dropped self-join |
| prometheus-operator `Probe` (LB-F312) | `spec.targets.staticConfig.static[*]`, `spec.prober.url` | `targets.ingress` (a selector over Ingresses) is not read |

## The side pass (LB-F307, wave c1)

What the doc engine's selector subset cannot say is read by `fusion`'s side pass (`routeref_mesh.go`) from the
manifest itself and joined to the incumbent's nodes in the same route pass: a VirtualService's `spec.gateways` to
the Gateway (`<ns>/<name>`, a short name in the rule's namespace, the legacy `<name>.<ns>.svc.cluster.local` as
pilot reads it; `mesh` names none) and each destination's `subset` to the DestinationRule declaring it for the
same host; an AuthorizationPolicy's `selector.matchLabels` to the workloads whose pod templates carry them and
its `rules[].from[].source.principals` (`<trust domain>/ns/<ns>/sa/<name>`) to the ServiceAccount; a
ServiceMonitor's `selector` (matchLabels and matchExpressions) to the Services by their own labels and a
PodMonitor's to the workloads, in the namespaces `namespaceSelector` names. A selector joins every match in the
nearest file, directory or repository holding one. Divergences:

- **An object without `metadata.namespace` lives in `default`.** A kustomization's `namespace:` is not applied,
  so a policy whose principals name `ns/onlineboutique` does not reach a ServiceAccount written without a
  namespace beside a kustomization that sets it (counted undeclared).
- **An AuthorizationPolicy in the mesh's root namespace** applies to every namespace; the side pass reads its
  selector in its own namespace only.
- **`Gateway/<ns>/<name>`** names an Istio Gateway and a Gateway API Gateway of the same name alike (the
  incumbent's node name carries no API group).
- **istioctl 1.31.1's analyzer** reads a legacy FQDN gateway name as a name in the rule's namespace (its schema
  validation warns it is the `<ns>/<name>` form); the side pass follows pilot, and the oracle's cross-check
  reports such an entry as a disagreement.
- **A PrometheusRule's expressions are not joined** (LB-F312, deferred): the Stage A static reading's kill fired
  (`docs/bench/readings/telemetry/stage-a-static.md`: label_join_recall .583 under .60 on kube-prometheus, every
  join made right). The dialect has no PrometheusRule entry; the object alone is stored.
- **A container's telemetry identity** (`OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES` `service.name=`,
  `NEW_RELIC_APP_NAME`; LB-F314, TL-5) is read by the incumbent (`k8sextract.go`), not by this dialect: an
  identity is a literal value split out of a list (`k=v,k=v`, `a;b`) with templates refused, which no selector of
  §4's subset expresses. The dispatch switch needs it ported (an engine value filter and split, or the side pass).
- Out of scope (K15): the Istio Telemetry resource, VirtualService mirrors and delegates, VMRule and
  VMServiceScrape.

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
`route_references` and `route_unresolved` by the documented DNS rules and the side pass's
semantics (`mesh.go`), with an Istio cross-check from `istioctl analyze --use-kube=false`
(IST0101: host+subset and gateway references). The harness is `indexbench lang-routes` (LB-F98).
