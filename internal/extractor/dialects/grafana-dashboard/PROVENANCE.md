# grafana-dashboard — provenance and divergences

LB-Q-grafana-dashboard, design `docs/design/language-breadth.md` §4 ("Observability documents"), §18.3 (T7); wave c3,
item obs. A doc-engine dialect (no `tags.scm`, no incumbent: a dashboard was generic JSON and a provisioning file
unclassified YAML). One dialect decodes JSON and YAML (the CloudFormation precedent). The fixtures are written for
this dialect from Grafana's documented dashboard JSON model and provisioning files; Grafana's source (AGPL-3.0) is a
behaviour reference only and nothing of it is copied.

## How a file is claimed: by its content, never its name

- **A dashboard** (JSON, or YAML): `detect.content_sniff` and a root entity: a top-level `schemaVersion` with `panels`
  or `rows`, a `dashboard` wrapper holding one (the HTTP API's export), or a `dashboard.grafana.app` resource of kind
  `Dashboard` (v2). `fusion.grafanaDashboardJSON` asks the sniff and the decode, so a JSON document that only mentions
  the keys keeps the generic JSON extractor; ingest, ReingestFile and ContentDialect ask the same predicate.
- **A provisioning file** (YAML): `apiVersion: 1` with a `providers`, `datasources`, `groups`, `contactPoints`,
  `policies`, `muteTimes` or `templates` section, through the late-YAML hub, asked before prometheus (an alerting
  provisioning file has a top-level `groups` of rules too, LB-F365).
- **Embedded**: a ConfigMap's `.json` data value that is a dashboard (kube-prometheus' grafana-dashboardDefinitions),
  a grafana-operator GrafanaDashboard's `spec.json`, and a ConfigMap's or Secret's `.yaml` provisioning value are read
  as documents of their own (`fusion/k8sembed.go`).

## What it models

- Dashboards (`GrafanaDashboard`, `package`), named by `uid` (else `title`, else the file), signed by title; rows
  (`GrafanaRow`, `section`): a `type: row` panel `row/<id>`, a legacy `rows[i]`; panels (`GrafanaPanel`,
  `element`), nested in rows, named `panel/<id>` (else by path), signed by type and title, a library panel reference
  included; library panel elements (`GrafanaLibraryPanel`, `__elements` keys); template variables
  (`GrafanaVariable`); dashboard links (`GrafanaLink`, named by path); a v2 dashboard's elements, variables and links.
- Provisioning (`GrafanaProvisioning`, `package`): dashboard providers, datasources (named by `uid`, else `name`,
  signed by name and type), alert rule groups and rules (by title), contact points, notification policies (by path),
  mute timings and notification templates. Every provisioning entity is `sensitive`.

## References

- L2: a panel, row or variable to every declared template variable its strings interpolate (`$name`, `${name...}`,
  `[[name...]]`); a panel to the library panel element it reuses.
- PromQL (`promql: {grafana: true}`, `extractor/docengine_grafana.go`): a target's `expr` is read only when its
  datasource is Prometheus-typed: the target's own, else the panel's (a mixed panel's targets carry theirs), else the
  dashboard default (every datasource variable and `${DS_*}` input Prometheus); Grafana's macros are substituted
  first; a template variable outside a string marks a name, so a templated metric is refused (`template-variable`);
  Loki, Tempo and server-side expressions are counted `datasource-not-prometheus`, an undecidable one
  `datasource-unknown`, never scanned. A metric name joins the recording rule that records it (entity-name); any
  other metric name joins nothing (Stage B is gated: never emitter code). K19: no identity-label join.
- L3 (entity-name, repo-tree; `fusion/alertroute.go`): a `/d/<uid>` link (a dashboard link, any panel string) or
  an alert rule's `dashboardUid`/`__dashboardUid__` to the dashboard of that uid; a datasource uid to the provisioned
  datasource; a policy's receiver to the contact point, its mute time interval to the mute timing; a provider's
  `options.path`, relative or a container path a compose bind mount maps, to every dashboard under it.

## Secrets

No entity, signature or reference reads `secureJsonData`, `basicAuthPassword`, `password`, a datasource's `url` or a
contact point's `settings` (TestIngest_AlertmanagerSecretsNeverStored).

## Not read

- A provider path a Kubernetes volume mounts (kube-prometheus' `/grafana-dashboard-definitions/0`): only compose bind
  mounts map a container path (counted `unmounted`).
- A datasource named by a legacy string (`"datasource": "Prometheus"`): its type is declared in another file, so the
  query is `datasource-unknown`; a v2 dashboard in YAML is a Kubernetes object, not claimed (LB-Q-grafana-dashboard-rest).
