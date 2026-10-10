# alertmanager — provenance and divergences

LB-Q-alertmanager, design `docs/design/language-breadth.md` §4 ("Observability documents"), §18.3 (T7); wave c3,
item obs. A doc-engine dialect (no `tags.scm`, no upstream grammar, no incumbent: an Alertmanager configuration was
unclassified YAML). The fixtures are written for this dialect from the configuration file Alertmanager documents
(prometheus/alertmanager `docs/configuration.md`, Apache-2.0); nothing is copied from its source.

## How a file is claimed: a name OR the content

Manifest basenames `alertmanager.yml`, `alertmanager.yaml`. `detect.content_sniff` claims a file no name claims: a
top-level `route` and a top-level `receivers`, in either order, a key quoted or not (kube-prometheus writes
`"route":`). The late-YAML hub (`fusion/lateyamldispatch.go`) asks this dialect after grafana-dashboard and before
prometheus, never for a Kubernetes object or a workflow. A configuration a Kubernetes Secret (`stringData` or base64
`data`) or ConfigMap embeds under `alertmanager.yaml`/`alertmanager.yml` is read through this dialect as a document
of its own, its entities qualified by the key (`alertmanager.yaml/route.routes[0]`, `fusion/k8sembed.go`).

## What it models

- **The configuration** (`AlertmanagerConfig`, `package`), named by the file.
- **The route tree** (`AlertmanagerRoute`, `route`): the root `route` and every child route, named by its path
  (`route.routes[1].routes[0]`, the engine's `$ipath`), signed by its receiver, matchers (both syntaxes), continue,
  group_by and time intervals.
- **Receivers** (`AlertmanagerReceiver`, `target`), named by `name`, signed by their integration kinds alone
  (`slack_configs, webhook_configs`): `signature_keys` names a key it writes, never its value.
- **Inhibit rules** (`AlertmanagerInhibitRule`, `rule`), named `inhibit_rules[<i>]`, signed by their matchers and
  `equal`.
- **Time intervals** (`AlertmanagerTimeInterval`, `variable_decl`): `time_intervals` and the deprecated
  `mute_time_intervals`, named by `name`.
- **Template globs** (`AlertmanagerTemplate`, `import`), named by the glob as written.

## References

- L2: a route's `receiver` and its `mute_time_intervals`/`active_time_intervals` (declared only).
- Matcher sets (the engine's `matchers`): a route's `matchers` (Alertmanager's matcher syntax, an unquoted value
  allowed) and legacy `match`/`match_re`, an inhibit rule's `source_*`/`target_*`, each read by `promql.ParseMatchers`
  (the PromQL selector scanner's own brace grammar, never a second matcher parser); a malformed one is unresolved.
  Facts: a route's `receiver` and `continue`.
- L3 (`fusion/alertroute.go`, the doc-xref pass): a route's or inhibit rule's `alertname` equality matcher joins
  every Prometheus alerting rule of that name; every alerting rule joins the receivers its alert name and static
  labels (its own and its group's, a templated value left out) route to by Alertmanager's own semantics: depth
  first, the first matching child ends the search unless it sets `continue`, a route none of whose children matched
  is itself the match, the receiver inherited, `=~`/`!~` anchored RE2. `routes_to` when the static labels decide
  every matcher on the way; `may_route_to` when a matcher reads a label the rule does not set statically (a series'
  label, the evaluating Prometheus' external labels: unknown until runtime). Which Alertmanager a Prometheus sends
  to is not read: every configuration of the repository routes every rule. K19: no identity-label join to a
  `deploy:` key.

## Secrets

No entity, signature, reference, fact or matcher reads a secret-bearing field: global SMTP, Slack, OpsGenie,
VictorOps, WeChat, Telegram and Rocket.Chat credentials, every integration's URL, key and token, `http_config`
credentials and TLS keys. TestIngest_AlertmanagerSecretsNeverStored plants a sentinel in every field Alertmanager's
config package types a secret (listed by reflection, `calibration/oracles/alertmanager/secret_fields.json`) and finds
it in no store table and no MCP answer; every entity is `sensitive`, so symbol_context serves no source snippet.

## Not read

- Template files (`*.tmpl`, Go text/template): no connector rule discovers them and no dialect reads them, so a
  template glob is an entity joined to nothing (LB-Q-alertmanager-rest).
- AlertmanagerConfig custom resources (`monitoring.coreos.com/v1alpha1`, a camel-cased schema of their own) and the
  Alertmanager custom resource's own route (LB-Q-alertmanager-rest).
- Route `labels` (v0.34: attached to notifications, never matched) and `group_wait`-style timing fields: data on the
  configuration, not modelled.
