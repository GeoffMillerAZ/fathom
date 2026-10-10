# Grafana dashboards and provisioning — reading of 2026-10-10 (wave c3, LB-Q-grafana-dashboard)

**Verdict.** fathom now reads a Grafana dashboard and its provisioning as code: dashboards (claimed by content,
whatever their file name, also inside Kubernetes ConfigMaps), rows, panels, variables, links, datasources, alerting
and providers, and it connects a panel to the Prometheus recording rule its query reads, a link to the dashboard it
opens and a provider to the dashboards it loads. Against a tier-C oracle (a walk of its own plus the Prometheus
parser; no Grafana tool reads a dashboard offline) every metric reads 1 — 1,038 definitions, 2,156 references, 230
cross-file pairs — and every level is capped at good by the oracle's tier: L3 good. Keep it; it pairs with the
prometheus and alertmanager dialects. It never joins a metric name to the code that emits it (Stage B is gated) and
never joins a label to a `deploy:` key (K19). What it does not do: a tier-A grade, a provider path a Kubernetes volume
mounts, a datasource named only by a string (`LB-Q-grafana-dashboard-rest`).

| Subject | Files | Definitions (fathom = oracle) | References | Pairs |
|---|---|---|---|---|
| prometheus-operator/kube-prometheus v0.19.0 (derived: 33 embedded dashboards, 2 provisioning files) | 35 | 540 | 1,164 | 158 (metric to recording rule, uid links) |
| thanos-io/thanos v0.42.4 (examples/dashboards, legacy rows) | 9 | 255 | 412 | none |
| open-telemetry/opentelemetry-demo 3.1.0 (src/grafana with compose) | 17 | 243 | 580 | 72 (datasource uids, provider to dashboards, contact points) |

The JSON beside this file is the reading (`indexbench lang-obs`); the commands are in
`calibration/oracles/grafana/README.md`.
