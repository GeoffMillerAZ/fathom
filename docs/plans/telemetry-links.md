# Telemetry links: the plan (ADR-062)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --strict --service fathom --plan docs/plans/telemetry-links.md` proves it on a scratch ingest of the
committed HEAD and, for a `symbol:` row, a product entry point reaches it (the MCP `localize` tool, `fathom
prompt-hook`, ingest, `indexbench`). Decision: ADR-062. Design: `docs/design/telemetry-links.md` (section numbers
below are its). Goal G15 (tier T7). Futures: F66 (Stage B), F67 (TL-4). Readings: `docs/bench/readings/telemetry/`.

Every reading here is $0 (no provider or agent spend). No row adds an MCP tool, a verb or a schema field (law 4), or a
store table, cache or background job (ADR-049 decision 2). Payloads are parsed in the call and discarded; frame tuples
only (ADR-058 decision 3).

## Records (TL-0)

- [x] ADR-062 appended: Stage A is dialect work, identity joins only; F59 not fired; the Stage B gate; payloads parsed and discarded; owner decision D4 recorded and fired; tier T7 accepted {id: ADR-062, oracle: lexical, path: ADR.md, pattern: "^## ADR-062: Telemetry links"}
- [x] The design: Stage A and Stage B, the payload-shape registry, the TL-5 identity contract and the delivery reality {id: TL-0, oracle: lexical, path: docs/design/telemetry-links.md, pattern: "^## 4\. The telemetry identity contract"}
- [x] The decisions index row for ADR-062 {id: TL-0-index, oracle: lexical, path: docs/intent/decisions/README.md, pattern: "^\| ADR-062 \|"}
- [x] Scope: one G15 line, observability-as-code formats measured as dialects at tier T7 {id: TL-0-scope, oracle: lexical, path: docs/intent/scope/goals-1.md, pattern: "are G15 dialects at tier T7"}
- [x] Futures: F66, Stage B name joins (TL-2, TL-3), promoted only when census E is at least 25 percent AND the owner records an F59 amendment {id: TL-0-f66, oracle: lexical, path: docs/intent/futures/f62-f67.md, pattern: "^## F66\. Telemetry name joins"}
- [x] Futures: F67, the log-template index (TL-4, wave C2) {id: TL-0-f67, oracle: lexical, path: docs/intent/futures/f62-f67.md, pattern: "^## F67\. A log-template index"}

## Frames (TL-1, section 3)

- [x] `localize` seeds from frame tuples in depth order (PHP `#N path(line): fn` realigned, V8 `at fn (path:l:c)` and `at path:l:c`, JVM frames, Python, Go, chained causes root first) and weights the innermost resolved frame as the throw site; a payload seeds frame tuples only {id: TL-1, symbol: localizeFrameSeeds, package: internal/mcp, reachable_from: TestLocalize_ThrowSiteIsTheTopCandidateOnEveryFixtureShape}
- [x] The payload shapes are data (`shapes.json`: Bugsnag with `inProject`, OTLP `exception.stacktrace` and `code.*` old and new, Lambda, New Relic); message, breadcrumb, user and metaData fields are never read, and a sentinel placed in them never reaches an answer or a hook line {id: TL-1-payload, symbol: ExtractPayload, package: internal/stackframe, reachable_from: TestExtractReadsEveryFixtureShape}
- [x] A JVM frame's package picks its file: `at com.x.y.C.m(C.java:12)` resolves com/x/y/C.java over another `C.java` that sorts first {id: TL-1-jvm, symbol: findFileForFrame, package: internal/mcp, reachable_from: TestLocalize_ThrowSiteIsTheTopCandidateOnEveryFixtureShape}
- [x] The prompt hook detects V8 colon frames, PHP frames and frame-carrying payloads with the same parser (the hand mirror of localize's patterns is gone), over the one shared fixture `internal/stackframe/testdata/frames.json` {id: TL-1-hook, symbol: routeIssueShapedPrompt, package: cmd/fathom, reachable_from: TestPromptHookRoutesEveryFixtureFrameShapeToLocalize}
- [x] O615 filed: the hook names `localize`, which the default eight-tool surface does not serve {id: TL-1-o615, oracle: lexical, path: .fathom/observations/O615.md, pattern: "^### O615 "}

## Telemetry identity (TL-5, section 4)

- [x] `localize` resolves a `service` that names no indexed service to the one service declaring that TelemetryIdentity, says so, refuses two or more with the candidates, and never matches an interpolated value; no new tool, schema or surface change {id: TL-5, symbol: resolveTelemetryIdentity, package: internal/mcp, reachable_from: TestLocalize_TelemetryIdentityResolvesToItsService}
- [x] The docker-compose (`OTEL_SERVICE_NAME`, `NEW_RELIC_APP_NAME`), serverless (function `name:`) and cloudformation (`FunctionName` on Lambda and Serverless functions) dialects emit the contract as data, with their ceilings in PROVENANCE.md (the Dockerfile `ENV` form a ceiling); TelemetryIdentity is a contract source kind several dialects declare {id: TL-5-dialects, symbol: isContractSourceKind, package: internal/extractor, reachable_from: TestTelemetryIdentityIsAContractSourceKind}
- [-] The dynamic oracle (a running app's OTLP resource `service.name` and its scrape target's `job` label) and the New Relic agent's own files (`newrelic.yml`, `.ini`, `.js`, not dispatched this wave) {id: TL-5-rest, disposition: deferred, reason: "no running app or committed agent file to read here; Bugsnag configuration is skipped under K15", trigger: "the cloud environment (dynamic oracle); an adopter commits a newrelic.yml, newrelic.ini or newrelic.js (agent files)"}

## Stage B, gated (F66, section 2)

- [-] TL-2: metric and span names joined to the code that emits them {id: TL-2, disposition: deferred, reason: "Stage B waits for the census", trigger: "census E at least 25 percent AND an owner F59 amendment (ADR-062 decision 3)"}
- [-] TL-3: the join stored as `telemetry-ref` typed edges (pre-scan, literal, never gating; query-time across repositories until SG-2) {id: TL-3, disposition: deferred, reason: "Stage B waits for the census", trigger: "census E at least 25 percent AND an owner F59 amendment (ADR-062 decision 3)"}

## Next (wave C2)

- [x] TL-4: a log-template index, a pasted log line to the call whose format string wrote it (F67; ADR-064, design section 9): `localize`'s `error` input matches each line against the service's stored templates, a matched line seeds a typed `log_template` seed per best template (longest coverage with placeholders as wildcards, literal length breaking ties, deterministic) and is consumed, the call's enclosing symbol ranks first above the literal-grep hop and below a trace's throw site, and nothing of the line beyond the template's own text reaches a hop, why-line, answer or call-log row (a sentinel in a placeholder position); no new tool, schema field or surface {id: TL-4, needs: TL-1, symbol: localizeLogTemplateSeeds, package: internal/mcp, reachable_from: TestLocalize_LogLineSeedsItsTemplate}
- [x] TL-10: the template reader (`internal/logtemplate`): Python `logging` and `structlog`, SLF4J, log4j2 and `java.util.logging`, pino, winston and the console, PSR-3 (Monolog), a formatter inside the call (`String.format`, `sprintf`, `util.format`), concatenated and interpolated operands as placeholders; the logger API list is embedded data with universal defaults (`apis.json`, K15), never a dialect's queries {id: TL-10, symbol: Read, package: internal/logtemplate, reachable_from: TestReadPerLogger}
- [x] TL-11: ingest stores each template as a `LogTemplate` observation of its own file (ADR-064; no new table, column, cache or background job), so a changed or deleted file's purge takes it; a store without templates (none from before, or an ingest that died between a file's symbols and its templates) is swept once {id: TL-11, symbol: enrichLogTemplates, package: internal/fusion, reachable_from: TestIngest_StoresLogTemplatesAsObservations}
- [x] TL-11-parity: `ReingestFile` writes exactly the template rows a fresh ingest of the same tree writes {id: TL-11-parity, symbol: logTemplateObservations, package: internal/fusion, reachable_from: TestReingestFile_LogTemplateParity}
- [x] (op) TL-12: `indexbench localize-replay -logs` runs each fixture app's log capture runner through the real logging libraries (`calibration/incidents/install-log-deps.sh`, pinned, installed under scratch), the truth the call site the runtime reports, each line pasted as `error`; it tallies by logger API and an echo fails the verdict {id: TL-12, symbol: captureCommand, package: cmd/indexbench, reachable_from: TestLocalizeReplayScoresTheLogLine}
- [x] (op) TL-13: the reading (`tl4-replay.md`): 24 logging cases a language (96), top-1 before 0.021 (2 of 96) and after 0.958 (92), 0.958 in every language, 0 echoes (18 answers carried the pasted line before), PASS at 0.90; the four misses are messages built into a variable before the call {id: TL-13, oracle: lexical, path: docs/bench/readings/telemetry/tl4-replay.json, pattern: "\x22verdict\x22: \x22PASS\x22"}
- [-] TL-14: ceilings: a message built into a variable before the call (4 of 96 in the replay); a logger reached through a chain the registry does not name (`request.app.logger`, structlog's `log.bind(...).info`), a fully qualified `java.util.logging.Logger.getLogger`, or a project's own logger names; Go is not read; the prompt hook does not route a pasted log line to `localize`; a `LogTemplate` node meets line-to-symbol readers other than `localize` at its call's line (O700) {id: TL-14, disposition: deferred, reason: "no consumer has measured these yet; top-1 is 0.958 without them", trigger: "an adopter's replay where variable-built messages or unnamed logger chains are a measurable share; ADR-061's zones ship (the registry override); a Go adopter's incident replay; a prompt-hook wave opens; RT-M1 reads PASS (ADR-064's move out of the symbol rows)"}

## Readings ($0)

- [x] (op) TL-6: the census reading, the share of telemetry emitters a static pass can name (item prom produces it): pooled emitter_share .1451 (85 of 586 metric names), under the .25 gate, so Stage B does not clear {id: TL-6, oracle: lexical, path: docs/bench/readings/telemetry/census-c0.json, pattern: "\x22emitter_share\x22"}
- [x] (op) TL-7: the TL-1 instrument; fault-injected apps in Java, Node, PHP and Python (22 cases each, five shapes each, truth from the runtime's own trace), top-1 before TL-1 0.259 and after 0.984, PASS at 0.90 per language and overall {id: TL-7, oracle: lexical, path: docs/bench/readings/telemetry/tl1-replay.json, pattern: "\x22verdict\x22: \x22PASS\x22"}
- [x] (op) TL-7's harness: `indexbench localize-replay` captures, scratch-ingests one app at a time and asks the MCP `localize` tool per case and shape {id: TL-7-harness, symbol: cmdLocalizeReplay, package: cmd/indexbench, reachable_from: TestLocalizeReplayScoresTheThrowSite}
- [x] (op) TL-8: Stage A's static deploy half, label-join recall (item mesh produces it): .583 over 235 kube-prometheus expressions, precision 1.0 (137 of 137), under .60, so the PrometheusRule join is withheld (LB-F312) {id: TL-8, oracle: lexical, path: docs/bench/readings/telemetry/stage-a-static.json, pattern: "\x22label_join_recall\x22"}

## Ceilings

- [-] TL-9: the document-path engine gains an entity `pattern` (an RE2 capture over the selected scalar, nothing on no match), lifting the compose list form, `OTEL_RESOURCE_ATTRIBUTES` and the interpolated-value ceilings at extraction (section 4) {id: TL-9, disposition: deferred, reason: "TL-5 was data only by brief; the join already refuses interpolated values", trigger: "an adopter's compose files use the list form or OTEL_RESOURCE_ATTRIBUTES, or a doc-engine wave opens"}
