# prometheus — provenance and divergences

LB-Q-prometheus, design `docs/design/language-breadth.md` §4 ("PromQL references"), §13, §18.3 (T7, observability
as code); wave c1, item prom. A doc-engine dialect (no `tags.scm`, no upstream grammar, no incumbent: a Prometheus
file was unclassified YAML). The fixtures are written for this dialect from the file formats Prometheus documents
(rule files, the configuration file, promtool unit tests; Apache-2.0, prometheus/prometheus `docs/`).

## How a file is claimed: a name OR the content

Manifest basenames `prometheus.yml`, `prometheus.yaml`; globs `**/*.rules.yml`, `**/*.rules.yaml`, `**/*_rules.yml`,
`**/*_rules.yaml` (Prometheus' own naming, `first_rules.yml`). The globs claim no path another manifest claims by
name (`TestPrometheusGlobsLeaveOtherClaims`: traefik's `rules/` and `dynamic*`, the gateways', gitlab-ci's and every
other dialect's fixture paths keep their verdicts). `detect.content_sniff` claims a file no name claims: a top-level
`groups:` whose rules write `alert:` or `record:` (never Grafana's alerting provisioning, whose rules are `uid`/`title`
maps), or a top-level `scrape_configs:` or `rule_files:` (a configuration, a promtool test file; never an
OpenTelemetry Collector's indented prometheus receiver). The late-YAML hub (`fusion/lateyamldispatch.go`, LB-F309)
claims a YAML file no earlier classifier signal claimed when either holds and it decodes to a root, never a
Kubernetes object (a PrometheusRule custom resource is the kubernetes side pass's, item mesh) or a workflow.
`require_keys` is not set: a rule file, a configuration and a test file share no key, and the spec's selectors are
the guard (an entity exists only under `groups`, `scrape_configs` or `tests`).

## What it models

- **The file** (`PrometheusFile`, `package`), named by the file: what `rule_files` resolves to.
- **Rule groups** (`PrometheusRuleGroup`, `section`), **alerting rules** (`PrometheusAlertingRule`, `rule`, named by
  `alert`) and **recording rules** (`PrometheusRecordingRule`, `rule`, named by `record`). Two rules of one name
  (one alert at two severities) are two symbols at their own lines.
- **Scrape jobs** (`PrometheusScrapeJob`, `job`, named by `job_name`), each with its **telemetry identity**
  (`TelemetryIdentity`, `key`, the literal `job_name`, signature `prometheus.job=<job_name>`: item incident's
  contract, ADR-062), the key a series' `job` label carries. A templated `job_name` (`${`, `{{`) is emitted as
  written, since a `where` filter cannot test a substring of a value; the identity join never matches it, so the
  contract's never-guessed rule holds at the answer (the docker-compose dialect's ceiling, the same).
- **promtool tests** (`PromtoolTest`, `test`), named `tests/<name>`, or `tests/<index>` for an unnamed one.

## References

- L2: a rule's `expr`, read by `internal/promql`'s selector scanner (the doc engine's `promql` mode), names every
  recording rule of the same file it reads (`uses_record`). A metric no rule records is no reference.
- L3, `repo-path`: a configuration's and a test file's `rule_files` (relative to the file's directory) to the rule
  file. A glob (`rules/*.yml`) is recorded as written, and fusion's doc-xref resolver matches globs to directories
  only, so it resolves to nothing at ingest (`LB-Q-prometheus-rest`).
- Route targets (`route-host`, `route_ref`): a scrape job's `static_configs[*].targets[*]` address, and each
  identifying-label equality matcher of a rule's `expr` (`job`, `service`, `app`, `destination_service`,
  `destination_workload`, `service_name`, scoped by the selector's `namespace` matcher), joined by
  `fusion.ResolveRouteRefs` to the compose service or Kubernetes Service whose `deploy:` key they name. A regex or
  negative matcher is counted in `DocResult.Unresolved`, never guessed. Route candidacy is path-only
  (`fusion.isRouteCandidate`): a file claimed by content alone is read but its targets reach the route pass only
  when item mesh's routeref change merges.

## Not read

- `.rules` files (Prometheus' older naming, `alert.rules`): `.rules` is no YAML extension classification routes,
  so no dialect reads it (`LB-Q-prometheus-rest`).
- Alertmanager targets, `remote_write`/`remote_read` endpoints and service-discovery configurations other than
  `static_configs`: no entity names them, so they are no route source.
- vmalert and MetricsQL (VictoriaMetrics' extensions); a Thanos ruler reads the same rule files, so Thanos is this
  dialect, deduplicated.
