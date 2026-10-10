# alertmanager-oracle (tier A)

The language-breadth harness's oracle for the `alertmanager` doc-engine dialect (LB-Q-alertmanager, design §1.3,
§10). A separate Go module (`go.mod`: `github.com/prometheus/alertmanager` v0.34.1, Apache-2.0; yaml.v3), never
imported by fathom and never a product dependency (ADR-020, ADR-025); it reads fathom's own `internal/promql` through a
local `replace` only for the matcher differential below.

## The instrument

amtool v0.34.1 (`GOBIN=<scratch>/gobin go install github.com/prometheus/alertmanager/cmd/amtool@v0.34.1`, module sum
checked by the Go toolchain) decides which files are configurations, and Alertmanager's own packages name what each
declares and where an alert goes:

- **A configuration** (a top-level `route` and `receivers`, never a Kubernetes object) is a subject when `amtool
  check-config` accepts it. `config.LoadFile` gives its route tree (each route named by its path, `route`,
  `route.routes[0]`), receivers, inhibit rules (`inhibit_rules[<i>]`), time intervals (`time_intervals` and
  `mute_time_intervals`) and template globs (as written: LoadFile makes them absolute); a route's receiver and time
  intervals are its references.
- **Alerting rules** of the subject's Prometheus rule files (`promtool check rules` 3.15.0 acceptance with
  `-promtool`): the alert name and static labels (the group's, then the rule's; a templated value is not static).
- **obs_references**: a route's alertname equality matcher (`dispatch.NewRoute` merges `matchers`, `match` and
  `match_re` as Alertmanager does) and an inhibit rule's source and target alertname to every alerting rule of that
  name; and, for every rule and configuration, the receivers `dispatch.Route.Match` (amtool `config routes test`'s own
  code) routes its static labels to, where the rule is **decided** (`obs_decided`): every matcher Match evaluates
  reads a static label, or a route already fails on one. An undecided rule's answer for its static labels alone is
  disclosed (`obs_undecided`, with the labels it lacks), never scored. Each decided routing is run through `amtool
  config routes test` as a cross-check (`obs_crosscheck`).

Lines are not Alertmanager output: they come from yaml.v3, a map value at its key's line (the root `route`), a list
item at its own; an end line is the last line any node under it is written on.

`-list-secret-fields` prints, by reflection over `config.Config`, every field whose type is a secret (a named type
`Secret*` of prometheus/common or Alertmanager: `Secret`, `SecretURL`, `SecretTemplateURL`) as a YAML path
(`receivers[].slack_configs[].api_url`; `[]` a list item, `{}` a map value), committed as `secret_fields.json` for
TestIngest_AlertmanagerSecretsNeverStored. `-derive-embedded <out>` writes each configuration a Secret or ConfigMap
embeds as a file at the manifest's path, so a manifest tree is measured as extracted configurations.

## Run

```
cd calibration/oracles/alertmanager && go build -o <scratch>/alertmanager-oracle .
<scratch>/alertmanager-oracle -list-secret-fields > secret_fields.json
<scratch>/alertmanager-oracle -subject <kube-prometheus>/manifests -derive-embedded <derived>/kube-prometheus/manifests
cd <subject> && <scratch>/alertmanager-oracle -subject . -amtool <amtool> -promtool <promtool> \
    -refs-out <o>/<s>-refs.json -obs-out <o>/<s>-obs.json -files-out <o>/<s>-files.txt > <o>/<s>-defs.json
indexbench lang-obs -dialect alertmanager -subject <subject> -subject-name <s> \
    -tool-oracle <o>/<s>-defs.json -tool-oracle <o>/<s>-refs.json -tool-oracle <o>/<s>-obs.json \
    -only "$(paste -sd, <o>/<s>-files.txt)" -out docs/bench/readings/lang/alertmanager.json
```

Subjects are pinned in `calibration/corpus.d/alertmanager.yaml`. prometheus/alertmanager is read as a derived tree:
`doc/examples` and `examples/ha` as they are, `cli/testdata` and `config/testdata` copied to `cli-testdata/` and
`config-testdata/` (fathom's connector skips a `testdata` directory); the configurations amtool refuses (26 of 52,
each a deliberately invalid test configuration) are out. kube-prometheus is read with its PrometheusRule specs
derived by `calibration/oracles/prometheus -derive-prometheusrules` and its Secret's configuration by
`-derive-embedded`.
