# Kubernetes per-container facts against k8s.io/api's typed structs (LB-F451, LB-F553; wave lf8, item k8s)

**Verdict: field parity 1 on every pod path class. On kubernetes.json's six subjects and the two
supplementary subjects of `calibration/corpus.d/kubernetes-facts.yaml`, every fact k8s.io/api's typed
decode reads is a K8sFact row a scratch ingest stored, and every stored row on an entity the typed decode
read is a typed fact: 2,332 of 2,332 both ways (`spec` 164, `spec.template.spec` 2,161,
`spec.jobTemplate.spec.template.spec` 7). The first run read 2,326: three kubernetes/examples documents
that repeat a mapping key were skipped whole by the extractor (O1034), fixed at 0a6507a9 before this
reading.**

LB-F553 verdict: PARITY (2,332 of 2,332; spec 164 of 164, template 2,161 of 2,161, jobTemplate 7 of 7)

Measured 2026-10-15 at fathom 0a6507a9 (the extractor) and 5465809d (the oracle), against the bar the shard
pre-registers: parity 1 on every class.

## Subjects

kubernetes.json's six at their pins (`calibration/corpus.d/kubernetes.yaml`, the subpath subjects as cone
clones, O1032): kubernetes/examples d6b8cd27, GoogleCloudPlatform/microservices-demo 38e7348e, istio/istio
190ae921 (1.31.1), kubernetes-sigs/gateway-api ca6c2a65 (v1.6.2), kubernetes/ingress-nginx 0a5901f3
(controller-v1.15.1), prometheus-operator/kube-prometheus 799f3d73 (v0.19.0). None holds a Job or a
CronJob, so `calibration/corpus.d/kubernetes-facts.yaml` adds two, pinned (and its licence read) before
the first run: GoogleCloudPlatform/kubernetes-engine-samples 2e79a7b3 and aws-samples/eks-workshop-v2
1b0f95ab, both Apache-2.0, their YAML files.

## Method

- *Stored.* `fathom ingest` (the build at 0a6507a9) of each subject into a fresh scratch store, an empty home
  and cache, then `sqlite3 -json` over its observations of source kind `K8sFact` (path, symbol, claim) and
  over `fingerprints` (the files ingest read).
- *Truth.* `calibration/oracles/kubernetes -facts-out` (facts.go): each document split and decoded by
  k8s.io/apimachinery v0.37.0 (`utilyaml.NewYAMLReader`, `utilyaml.Unmarshal`), its apiVersion and kind
  looked up in a scheme of the built-in workload groups (core/v1; apps/v1, v1beta1, v1beta2;
  extensions/v1beta1; batch/v1, v1beta1) and converted to k8s.io/api v0.37.0's typed object. The pod spec
  is the kind's own. The facts are LB-F451's: requests and limits of cpu, memory and ephemeral-storage
  (a quantity canonical, `resource.Quantity.String`), each probe's handler, port, httpGet path and the five
  timings (an int32 timing only where the document writes it), the container and init-container lists,
  and replicas.
- *Scope.* A pair is (file, entity, key, value). A truth fact on a file ingest did not read (discovery's
  exclusions) is out of scope, counted; a stored row on an entity the typed decode did not read (another
  kind, or a document it refuses) is counted, never scored (§1.4).
- *Withheld.* Neither side reads env, envFrom, args, command or a probe's exec command or headers
  (ADR-069 decision 2); the extractor's tests plant a sentinel in each and find it in no node and no
  stored row.

## Results

| Subject | Documents | Typed `spec` | Typed template | Typed job template | Refused | Truth facts | Matched / stored | Truth only | Stored on untyped entities | Truth off read files |
|---|---|---|---|---|---|---|---|---|---|---|
| kubernetes/examples | 272 | 55 | 68 | 0 | 1 | 329 | 329 / 329 | 0 | 7 | 0 |
| GoogleCloudPlatform/microservices-demo | 40 | 0 | 12 | 0 | 0 | 124 | 124 / 124 | 0 | 0 | 0 |
| istio/istio | 122 | 0 | 32 | 0 | 0 | 64 | 64 / 64 | 0 | 0 | 0 |
| kubernetes-sigs/gateway-api | 114 | 0 | 0 | 0 | 0 | 0 | 0 / 0 | 0 | 0 | 0 |
| kubernetes/ingress-nginx | 53 | 0 | 10 | 0 | 1 | 36 | 36 / 36 | 0 | 2 | 0 |
| prometheus-operator/kube-prometheus | 92 | 0 | 6 | 0 | 0 | 77 | 77 / 77 | 0 | 0 | 0 |
| GoogleCloudPlatform/kubernetes-engine-samples | 1,102 | 33 | 253 | 2 | 3 | 1,446 | 1,446 / 1,446 | 0 | 16 | 7 |
| aws-samples/eks-workshop-v2 | 272 | 7 | 66 | 1 | 0 | 256 | 256 / 256 | 0 | 0 | 0 |
| **All** | 2,067 | 95 | 447 | 3 | 5 | **2,332** | **2,332 / 2,332** | 0 | 25 | 7 |

By fact family (truth = stored): `containers` 524, `initContainers` 15, `replicas` 317, requests 534, limits
450, livenessProbe 58, readinessProbe 62, startupProbe 4, and the probes' port 94, path 60, periodSeconds
83, initialDelaySeconds 70, timeoutSeconds 26, failureThreshold 26, successThreshold 9.

## Disclosed

- *Refused by the typed decode (5 documents, their 25 stored rows counted, not scored):* a placeholder
  where k8s.io/api types an int32 (kubernetes/examples `_archived/volumes/fibre_channel/fc.yaml`'s
  `lun: <ADD LUN ID Here>`, kubernetes-engine-samples `networking/sctp/sctp-deployment.yaml`'s
  `containerPort: PORT`, and `ai-ml/infw-gw/.../inference-pool-variables.yaml`, whose Deployment is named
  and namespaced by `${VAR:-default}` placeholders), a `${ACCELERATOR_QUANTITY}` quantity on
  `nvidia.com/gpu` (`autoscaling/custom-compute-classes/reservations/deployment-gpu.yaml`), and an unquoted
  `true` env value where it types a string (ingress-nginx `auth/oauth-external-auth/vouch-proxy.yaml`; an
  env value neither side stores). The typed decode refuses a document whole; the extractor keeps each
  literal value of it as written, as LB-F451's rule says.
- *Not a typed kind:* kubernetes/examples' two vitess `{{cell}}` templates (no YAML parser outside
  yaml.v3 reads them: `invalid map key`) and a CloudNativePG `Pooler` (a custom resource with a pod
  template) carry stored facts no typed struct reads.
- *Out of scope:* 7 truth facts in kubernetes-engine-samples `cost-optimization/gke-shift-left-cost/api/testdata/`,
  a directory ingest's discovery excludes.
- *The first run (O1034):* 2,326 of 2,332. Three kubernetes/examples Deployments
  (`_archived/openshift-origin/etcd-controller.yaml`, `etcd-discovery-controller.yaml`,
  `openshift-controller.yaml`) write `spec.selector` twice; kubectl and the typed decode read them, the last
  value winning, while yaml.v3's struct decode refused them and the extractor skipped each document whole.
  The extractor now reads such a document from its node, last key winning
  (TestK8sContainerFactsDuplicateKeyLastWins); the six facts, and the three workloads and images, are
  stored.

## Cost (K26, K28, K30)

LB-F553 cost: under the bar (summed store +8.5%, cold wall +6.9%, warm wall +3.6%; K26's bars +20% and +15%)

`fathom ingest` of each subject into a fresh scratch store, base origin/main 99aa205a against 0a6507a9, under
`ingest.lock`, five interleaved rounds (the build order alternating), medians. *Cold* is a fresh store with an empty
home and cache each run; *warm* a fresh store with a home and cache kept from an uncounted warm-up (K30). Subjects:
kubernetes/examples (the whole checkout) and prometheus-operator/kube-prometheus (its cone clone). The store grows by
the K8sFact rows (kubernetes/examples 336, kube-prometheus 77) and, on kubernetes/examples, the three duplicate-key
workloads and their images (O1034): 1,293 to 1,637 observations there. The host ran other waves' tests throughout
(load near 7), so single runs spread by up to 0.9 s; the medians are the reading.

| Subject, mode | Wall base | Wall head | Δ | CPU base | CPU head | Δ | Store base (B) | Store head (B) | Δ |
|---|---|---|---|---|---|---|---|---|---|
| kubernetes/examples cold | 1.728 s | 1.755 s | +1.6% | 1.808 s | 1.874 s | +3.7% | 2,478,080 | 2,736,128 | +10.4% |
| kubernetes/examples warm | 1.796 s | 1.809 s | +0.8% | 1.912 s | 1.928 s | +0.9% | 2,478,080 | 2,736,128 | +10.4% |
| prometheus-operator/kube-prometheus cold | 1.957 s | 2.185 s | +11.7% | 2.038 s | 2.158 s | +5.9% | 1,396,736 | 1,466,368 | +5.0% |
| prometheus-operator/kube-prometheus warm | 1.996 s | 2.121 s | +6.2% | 2.103 s | 2.192 s | +4.2% | 1,396,736 | 1,466,368 | +5.0% |
| both, summed cold | 3.684 s | 3.940 s | +6.9% | 3.845 s | 4.033 s | +4.9% | 3,874,816 | 4,202,496 | +8.5% |
| both, summed warm | 3.792 s | 3.930 s | +3.6% | 4.015 s | 4.120 s | +2.6% | 3,874,816 | 4,202,496 | +8.5% |
