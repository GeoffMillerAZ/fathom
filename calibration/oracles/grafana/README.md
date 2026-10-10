# grafana-oracle (tier C)

The language-breadth harness's oracle for the `grafana-dashboard` doc-engine dialect (LB-Q-grafana-dashboard, design
§1.3, §10). **Tier C, and the reading says so: every level it grades is capped at good.** No Grafana tool reads a
dashboard or a provisioning file offline, and Grafana's source is AGPL-3.0 (a behaviour reference only; nothing of it
is copied). This program is a walk of its own over the documented dashboard JSON model and provisioning files, sharing
no code with fathom's dialect, and it grades the PromQL a panel sends with the Prometheus parser's own
`ExtractSelectors` (prometheus v0.315.0, as `calibration/oracles/prometheus`). A separate Go module, never imported by
fathom (ADR-020, ADR-025).

## The instrument

- **Dashboards**: a JSON or YAML document whose top level (or a `dashboard` wrapper's) has `schemaVersion` with
  `panels` or `rows`, or a `dashboard.grafana.app` resource of kind `Dashboard`. Definitions: the dashboard (uid,
  else title), rows (`row/<id>`, `row/rows[i]`), panels (`panel/<id>`, nested in rows, a library panel reference
  included), library panel elements, template variables, dashboard links.
- **Provisioning**: a YAML document with `apiVersion: 1` and a section: providers, datasources (uid, else name),
  alert rule groups and rules (title), contact points, notification policies (`policies[i]`, `.routes[j]`), mute
  timings, templates.
- **References**: a panel, row or variable to every declared variable its strings interpolate; a panel to the
  library panel element it reuses.
- **obs_references**: a panel's or alert rule's PromQL metric to the recording rule of the subject's Prometheus rule
  files, read only for a Prometheus datasource (the target's, else the panel's, else every datasource variable and
  input of the dashboard Prometheus), macros substituted, a variable inside a range's brackets a duration and any other
  outside a string a placeholder no metric has, parsed and `ExtractSelectors`'d by the Prometheus parser; a `/d/<uid>`
  link or `dashboardUid` to the dashboard; a datasource uid to the provisioned datasource; a policy's receiver to the
  contact point and its mute time interval to the mute timing; a provider's `options.path` (relative, or a container
  path a compose bind mount maps) to every dashboard under it.

Lines come from yaml.v3: a map value at its key's line, a list item at its own.

`-derive-embedded <out>` writes every dashboard and provisioning file a ConfigMap (or ConfigMapList), Secret or
GrafanaDashboard embeds under `<out>/<manifest without extension>/<key>` and removes the manifest from `out`.

## Run

```
cd calibration/oracles/grafana && go build -o <scratch>/grafana-oracle .
<scratch>/grafana-oracle -subject <kube-prometheus>/manifests -derive-embedded <derived>/kube-prometheus/manifests
cd <subject> && <scratch>/grafana-oracle -subject . -refs-out <o>/<s>-refs.json -obs-out <o>/<s>-obs.json \
    -files-out <o>/<s>-files.txt > <o>/<s>-defs.json
indexbench lang-obs -dialect grafana-dashboard -subject <subject> -subject-name <s> \
    -tool-oracle <o>/<s>-defs.json -tool-oracle <o>/<s>-refs.json -tool-oracle <o>/<s>-obs.json \
    -only "$(paste -sd, <o>/<s>-files.txt)" -out docs/bench/readings/lang/grafana-dashboard.json
```

Subjects are pinned in `calibration/corpus.d/grafana-dashboard.yaml`.
