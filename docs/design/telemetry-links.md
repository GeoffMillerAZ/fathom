# Telemetry links: from a pasted frame, payload or identity to the code that threw

Status: Stage A built (TL-1, TL-5, TL-7), Stage B gated. Decision: ADR-062. Plan:
`docs/plans/telemetry-links.md` (TL-). Futures: F66 (Stage B), F67 (the log-template index).
Readings: `docs/bench/readings/telemetry/` (TL-7 here; TL-6 and TL-8 from wave C1's other items).

## 1. The problem

An incident reaches an agent as an artifact another system produced: a stack trace pasted from
a log, an error event from Bugsnag or New Relic, an OpenTelemetry span or log record, a Lambda
error response, an alert naming a service. Two things must happen before any reasoning about
the bug: the artifact's frames must be read, in whatever grammar or JSON layout it carries,
and the service it names must be mapped to the repository fathom indexed. Fathom does neither
by connecting to those systems (I56; ADR-058 decision 6). It reads what the agent pastes.

## 2. Stage A and Stage B

**Stage A (built): identity joins.** An observability-as-code format is a G15 dialect at tier
T7, "observability as code", measured like any dialect (ADR-048). What Stage A joins is
identity: which service a telemetry name (an OpenTelemetry `service.name`, a New Relic app
name, a Lambda function name, a Prometheus `job`) belongs to, and which deployed workload a
label matcher selects. A label matcher maps to the existing ADR-050 `deploy:` keys; its edge is
stored the way route references are, as a file-tier typed edge under the interim divergence
`internal/fusion/routeref.go` records (lines 14 to 24): no new key kind, table, cache or
background job.

**Stage B (gated, F66): name joins.** A metric name in an alert's PromQL, or a span name in a
trace, joined to the call in some service's code that emits it (TL-2), stored as a
`telemetry_ref` (TL-3). Its gate: the census reading E (TL-6, the share of emitters a static
pass can name) at 25 percent or more AND an F59 amendment the owner records. Built, its storage
follows the same interim divergence: typed_edges key `telemetry-ref`, tier pre-scan, a
`literal` link that never gates (I57); cross-repository joins are made at query time until
SG-2 lands. F59 (promises as code) has not fired and stays unamended until the census.

## 3. Frames and the payload-shape registry (TL-1)

One parser, `internal/stackframe`, serves `localize` and the prompt hook. It returns frame
tuples: a path, a line, the function enclosing that place as the trace names it, a path hint
when the trace implies a longer path, and a depth, 0 being the throw site. It never returns
message text.

Raw grammars, each read line by line:

| Grammar | Shape | Notes |
|---|---|---|
| JVM | `at [module/]pkg.Class.method(File.java:12)`, or without `at` (Lambda's Java runtime) | the package is a path hint (`pkg/Class.java`): of two `Util.java` files, the one in the package's directory |
| V8 | `at fn (path:l:c)`, `at path:l:c` | `node:` and `<anonymous>` places dropped; `[as alias]` and `file://`, `webpack:///` prefixes stripped |
| PHP | `#N path(line): callee(args)`, the header `... in path:line`, `thrown in path on line N` | realigned: frame #N's place lies inside #N+1's callee, the header's inside #0's; arguments never read; `Next X:` ends the root-cause section |
| Python | `File "path", line N, in fn` | oldest call first, so reversed; chained exceptions print the cause first |
| Ruby, Go, any `path.ext:line` | `path.rb:12:in 'm'`; Go's function line above `\tpath.go:N +0x1d` | the generic match last |

Chained causes are read root cause first: the JVM's last `Caused by:` section (and any other
grammar under such sections), Python's first section, PHP's first exception.

Payloads are JSON and their shapes are data, in `internal/stackframe/shapes.json` (universal
defaults; a `.fathom/` override waits for ADR-061's zones, K15):

| Shape | Keys read | Notes |
|---|---|---|
| Bugsnag | `stacktrace[]` of `{file, lineNumber, method, inProject}` | `inProject` filters frames; `exceptions[]` lists the error first, so it is read last-first |
| OTLP JSON | `exception.stacktrace` (a raw trace); `code.filepath`/`code.lineno`/`code.function` and `code.file.path`/`code.line.number`/`code.function.name` | attribute lists (`[{key, value: {stringValue}}]`) read as keys; `code.*` frames rank after every stack frame (they name the instrumented call, not the throw) |
| Lambda | `stackTrace`, `trace` or `stack` beside `errorType` | Node writes `trace`, Python and Java `stackTrace` (Java nests `cause`, read first), a PHP runtime layer `stack` |
| New Relic | `error.stack_trace` | a raw trace |

The registry also names what is never read (`never_read`: `metaData`, `breadcrumbs`, `user`,
`message`, `errorMessage`, `exception.message`, `error.message`, `body`, `request`), and a JSON
value no shape matches is a payload with no frames, never a trace to mine. A payload is parsed
in the call and discarded: this is the "pasted trace or alert" delivery of
`docs/design/incident-maps.md` section 5.1 (line 123), and ADR-058 is cited, not amended.

`localize` seeds the walk from the frames in depth order. The first frame that resolves to an
indexed symbol is weighted as the throw site (a bonus sized by the TL-7 replay to outrank any
caller's sum); a function a trace names twice counts once; a PHP frame known only by its
function (`getTraceAsString` has no throw location) resolves by name and class. A payload is
never mined for quoted literals and never reformulated: its free text is someone's message.
The prompt hook counts V8, PHP, JVM, Python, Ruby and Go frame lines with the parser's own
`FrameLineRE`, and a frame-carrying payload is issue-shaped; the hook line names the shape and
`localize`, never the payload's text.

## 4. The telemetry identity contract (TL-5)

The contract items prom and mesh emit to as well:

- A doc entity of source kind `TelemetryIdentity`, canonical kind `key`.
- `name` is the literal value; `signature` is `<scheme>=<value>`, with scheme one of
  `service.name`, `newrelic.app_name`, `aws.lambda.function_name`, `prometheus.job` (a dialect
  writes it as `fmt:<scheme>={selector}`).
- A value carrying `${`, `{{`, `$(`, `!Ref` or `!Sub` yields nothing: it is decided at deploy
  time and never guessed.
- Several dialects declare the one source kind; the dialect loader admits it through
  `contractSourceKinds`.

This wave's dialects, data only: docker-compose (a service's `environment` map:
`OTEL_SERVICE_NAME`; `NEW_RELIC_APP_NAME`, its first name), serverless (an explicit function
`name:`), cloudformation (a literal `FunctionName` on `AWS::Lambda::Function` and
`AWS::Serverless::Function`). Ceilings, in each dialect's PROVENANCE.md: the compose list form
(`- OTEL_SERVICE_NAME=x`) and `OTEL_RESOURCE_ATTRIBUTES` (`service.name=x,...`) need a cut after a
marker, which the document-path engine lacks; an interpolated value is emitted as written
because a `where` filter cannot test a substring of a value (the join below never matches it,
so the never-guessed rule holds at the answer, not at extraction); the Dockerfile `ENV` form
waits for a signature template in the tags engine. One engine capability (an entity
`pattern`, an RE2 capture over the selected scalar that yields nothing on no match) lifts the
first three (TL-9).

`localize` resolves additively, with no new tool and no schema or surface change: when its
`service` names no indexed service, every indexed service is asked for a TelemetryIdentity of
that literal name. Exactly one: the answer is that service's, with `service_resolved` naming
the identity and where it is declared. Two or more: refused, with the candidates. Deferred
(TL-5-rest): the dynamic oracle (a running app's OTLP resource `service.name` and its scrape
target's `job` label), whose trigger is the cloud environment; and the New Relic agent's own
files (`newrelic.yml`, `.ini`, `.js`), whose trigger is an adopter committing one. Bugsnag's
configuration is skipped (K15): it names no identity the payload does not carry.

## 5. Not built

LogScale CQL, LogsQL, NRQL, CloudWatch EMF, VictoriaMetrics (MetricsQL, VMRule), the Istio
Telemetry CRD, mirror and delegate rules, and Bugsnag as an extractor: no consumer, or no
identity a dialect could add (ADR-062 decision 10).

## 6. Delivery reality

`localize` is not on the default eight-tool agent surface (ADR-053; law 4) and stays on
`/rpc/extended`. Its call share is small: 40 of about 98,000 logged calls when ADR-058 was
written, against hooks and forced channels that survive. So the incident path reaches two
kinds of session: one the prompt hook routes (a pasted trace or payload reads issue-shaped,
and the hook names `localize` as the opening move) whose registration serves `/rpc/extended`,
and one registered on `/rpc/extended` directly. A session on the default surface is pointed at
a tool it does not have: O615 records that open defect. The measurement that would move
`localize` onto the default surface is a share of incident-path calls, never this replay.

## 7. Measurements

- TL-7 (`docs/bench/readings/telemetry/tl1-replay.md`): fault-injected apps in Java, Node, PHP
  and Python, 22 cases each, five shapes per case, truth from each runtime's own structured
  trace. Top-1 0.259 before TL-1, 0.984 after; PASS at 0.90 per language and overall.
- TL-6 (`census-c0.json`, item prom): the emitter share E that gates Stage B.
- TL-8 (`stage-a-static.json`, item mesh): Stage A's static deploy half, label-join recall.

## 8. Jev, weighed

The decision port could order candidates when frames are ambiguous, or choose among several
services declaring one identity. Neither is a site: the ordering is deterministic and measured,
and the TL-7 misses are extraction and payload gaps, not ranking choices; an ambiguous identity
is refused, never chosen, since a judgment never asserts a code fact (I62). With provider none,
the default, every path here runs unchanged. What would admit Jev: a replay over a consumer's
real payloads where top-1 falls below 0.90 because of ranking.
