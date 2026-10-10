# istio (tier A acceptance and cross-check, pinned)

`istioctl` is not an oracle of its own here: the kubernetes oracle's `-istioctl` flag runs it
(`calibration/oracles/kubernetes/routes.go`, `istioCrossCheck`) over every subject file that holds an
Istio resource, offline, and reads two of its analyzers' IST0101 messages against the oracle's own pairs:
`virtualservice.DestinationRuleAnalyzer` ("Referenced host+subset in destinationrule not found") for each
VirtualService destination that names a subset, and `virtualservice.GatewayAnalyzer` ("Referenced gateway not
found") for each `spec.gateways` entry other than `mesh` (LB-F315). Offline (`--use-kube=false`, 1.31.1
`--verbose`) istioctl skips its destination-host and authorization-policy analyzers, so a route to a Service
and an AuthorizationPolicy's selector stay the walk's (tier C); its schema validation of every Istio kind runs.

## The pinned binary

istioctl 1.31.1, `istioctl-1.31.1-osx-arm64.tar.gz` from github.com/istio/istio releases 1.31.1, sha256
`3f37edda91e51a5948d688681c9db64971ee3abb38cda65d797dec51f46fecd1`, checked against the release's `.sha256`
asset before extraction:

```
curl -fsSLO https://github.com/istio/istio/releases/download/1.31.1/istioctl-1.31.1-osx-arm64.tar.gz
curl -fsSLO https://github.com/istio/istio/releases/download/1.31.1/istioctl-1.31.1-osx-arm64.tar.gz.sha256
shasum -a 256 -c istioctl-1.31.1-osx-arm64.tar.gz.sha256 && tar -xzf istioctl-1.31.1-osx-arm64.tar.gz
kubernetes-oracle -subject <checkout> -kubectl <kubectl> -refs-out <refs.json> -istioctl ./istioctl > <defs.json>
```

## Known disagreement

istioctl 1.31.1's gateway analyzer reads a legacy `<name>.<namespace>.svc.cluster.local` gateway entry as a
Gateway named so in the VirtualService's namespace, while its own schema validation warns that the form means
`<namespace>/<name>` (pilot's resolution, which fathom and the oracle follow). No measured subject writes the
legacy form; one would be reported in the cross-check's `disagree` list, not hidden.
