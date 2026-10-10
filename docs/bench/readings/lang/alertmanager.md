# Alertmanager configuration — reading of 2026-10-10 (wave c3, LB-Q-alertmanager)

**Verdict.** fathom now reads an Alertmanager configuration as code and connects it to the alerts it routes:
every route, receiver, inhibit rule, time interval and template glob, and for each Prometheus alerting rule the
receivers its alerts reach. Measured against Alertmanager's own tools (amtool v0.34.1, its config and dispatch
packages; tier A) it agrees on everything: 155 definitions, 57 references, 140 cross-file pairs, L3 excellent. Keep
it. It pairs with the prometheus dialect (the rules it routes) and with `symbol_context`/`blast_radius` on a rule
("which pager does this alert page?"). Nothing comparable is read by the generic YAML extractor it replaces, which
read the file as nothing. What it does not do: read template files, AlertmanagerConfig custom resources, or know
which Alertmanager a Prometheus sends to (`LB-Q-alertmanager-rest`).

| Subject | Files | Definitions (fathom = oracle) | References | Pairs (routing, alertname) |
|---|---|---|---|---|
| prometheus/alertmanager v0.34.1 (derived: doc/examples, examples/ha, cli-testdata, config-testdata) | 26 accepted of 52 | 143 | 53 of 53 | none (no rule file) |
| prometheus-operator/kube-prometheus v0.19.0 (derived: the Secret's configuration, the PrometheusRule specs) | 1 | 12 | 4 of 4 | 140 of 140 (125 rule names decided; amtool routes test agrees on 139 of 139 routings) |

The JSON beside this file is the reading (`indexbench lang-obs`); the commands are in
`calibration/oracles/alertmanager/README.md`. Secrets: no secret-bearing field is selected or stored
(TestIngest_AlertmanagerSecretsNeverStored plants a sentinel in all 226 fields Alertmanager types a secret).
