# Stage A static reading: PrometheusRule expressions to the Services they watch

**Verdict, in plain language.** Where a rule names a service the repository deploys, fathom's join is right
every time: on kube-prometheus v0.19.0 it joined all 137 such alert and recording expressions to the right
Service and made no wrong join (precision 1.0). But only 137 of the 235 expressions name such a service, so
label_join_recall over every expression is .583, under the .60 kill line, and no static join could do better:
the other 98 watch the cluster itself (58 read `job="apiserver"`, `kubelet`, `kube-scheduler` or
`kube-controller-manager`, whose Services the cluster creates, not the repository) or name no service at all
(40 with no job or service matcher). By the rule set for this reading the kill fires: PrometheusRule
expressions ship parsed, with no join, and the ceiling is recorded (LB-F312, deferred). Keep the reading; it is
the evidence for the owner's call on whether platform expressions (census partition P) belong in the
denominator, which would put recall at .774 (137 of 177), or 1.0 over the joinable ones.

- Reading: `stage-a-static.json` (TL-8's deploy half, Stage A, static; wave c1 item mesh). Measured
  2026-10-09 at fathom f0757ff9, where the kubernetes dialect's PrometheusRule entry reads each `expr` through
  the doc engine's promql mode (LB-F317) and the route pass resolves it (fusion.ResolveRouteRefs).
- Subject: prometheus-operator/kube-prometheus v0.19.0 (799f3d73), `manifests/`: 235 expressions (139 alerts,
  96 recording rules) in its PrometheusRules.
- Truth (tier C, so the reading caps at good): the kubernetes oracle's `-labeljoins-out`
  (`calibration/oracles/kubernetes/labeljoins.go`), from prometheus-operator's documented scrape semantics: a
  ServiceMonitor scrapes every Service its selector matches in the namespaces it names and gives each target
  the job label the Service's `jobLabel` label carries, else the Service's name; `service` is the Service's
  name. Expressions are read with the Prometheus parser itself (`promql/parser` v0.315.0), every one of the
  235 parsed. The namespace matcher of a selector does not narrow the truth (for an exporter that keeps its
  own labels, kube-state-metrics, it names the object's namespace, not the Service's).
- Scoring: an expression is recalled when fathom joins every Service its truth names, on the expression's own
  line; the denominator is every expression. label_join_recall .583 (137/235); label_join_precision 1.0
  (137/137); recall over the joinable partition 1.0 (137/137); platform 0/58; unlabelled 0/40.

## The ceiling

A static join can only reach a Service the repository declares. kube-prometheus is a monitoring stack for the
cluster: 58 of its expressions watch control-plane jobs whose Services are the cluster's own (`default/
kubernetes` for the API server, the kubelet Service the operator creates at run time, the scheduler and the
controller manager in kube-system), and 40 aggregate across jobs with no job or service matcher. Neither half
is a fathom miss: the maximum static recall on this subject is .583. A higher reading needs either a different
denominator (an owner decision: platform metrics as partition P, never "orphan alerts") or a runtime oracle
(Prometheus' `/api/v1/targets` and `/api/v1/series` on a running stack, tier A dynamic, which would also turn
the platform jobs into joins to the cluster's Services). Trigger to revisit: either of those.

## What did not change

The side pass's other joins ship: ServiceMonitor and PodMonitor selectors to the Services and workloads they
scrape, Probe targets, and the Istio references (LB-F307). They are measured in
`docs/bench/readings/lang/kubernetes.json` (LB-F315).
