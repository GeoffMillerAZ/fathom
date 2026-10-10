# TL-4: localize on pasted log lines, before and after the log-template index

**Verdict: PASS. Keep it.** Paste a log line into `localize`'s `error` input, a line written by
Python's `logging` or `structlog`, SLF4J, log4j2 or `java.util.logging`, pino, winston or the
console, or Monolog, and it now names the function whose logger call wrote it first in 92 of 96
answers (0.958), up from 2 (0.021) before. Every language reads 0.958: Java, Node, PHP and
Python alike. The four misses are one case per language that no template reader can answer: the
message is built into a variable before the call (`msg = "..." + id; log.warning(msg)`). No answer
repeats the pasted line: 0 of 96 now, where 18 did before (quoted values in the line were grepped
as literals and cited back). Where it pays: an agent handed an alert or an operator's pasted log
line goes straight to the code that logged it, with no grep for a fragment of the message and no
guessing which of several similar messages it was. What it does not replace: the log system
(fathom reads one pasted line in the call and discards it, never fetches, never stores a line),
and a trace when there is one (a trace's throw site still ranks first when both are pasted). It
pairs with whatever log tooling the team already runs: the paste is the integration. It costs
nothing to run: no model, no daemon; the reader adds 32 ms per full pass over a 387-file Python
service and 0.44 percent to its store.

Row TL-4 (with TL-10 to TL-14), `docs/plans/telemetry-links.md`. Decisions: ADR-062, ADR-064.
Design: `docs/design/telemetry-links.md` section 9. Data: `tl4-replay.json` beside this file.

## Method

- **Subjects.** The TL-7 fixture apps, `calibration/incidents/{java,node,php,python}/app`, each
  given payments, refunds, warehouse and audit code that logs: 24 logging cases a language (96).
  The shapes are the ones real code writes: a format string with arguments, named placeholders
  (`%(name)s`, PSR-3 `{name}`, `{session.id}`), an f-string, a template literal, PHP
  interpolation, string concatenation, `String.format`/`sprintf`/`util.format` inside the call,
  `"..." % args` and `.format()`, implicit and multi-line literals, a level argument first
  (`log(level, msg)`), a pino merging object or an error object first, an SLF4J marker first, an
  escaped `\{}`, a MessageFormat `''`, `logger.exception` and a throwable last, the root logger,
  debug and fine levels, and one message built into a variable (the ceiling case).
- **Libraries.** The real ones, pinned, installed under scratch by
  `calibration/incidents/install-log-deps.sh` (never globally): structlog 26.1.0, SLF4J 2.0.17
  bound to log4j2 2.25.2, `java.util.logging`, pino 10.4.0, winston 3.19.0, the Node console,
  Monolog 3.12.1 with its PSR-3 message processor. Renderers as deployed: Python
  `%(asctime)s %(levelname)s [%(name)s] %(message)s`; structlog's console, JSON and key-value
  renderers; log4j2's `%d{ISO8601} %-5level [%t] %c{1.} - %msg`; a one-line `SimpleFormatter`;
  pino's JSON line; winston `timestamp splat printf`; Monolog's default line format.
- **Truth, tier A.** Each app's log capture runner runs natively (Java 27's multi-file source
  launcher with `-Xmx512m`, Node 26, PHP 8.5, Python 3.14) and records the call site the runtime
  itself reports when the line is written: Python's `LogRecord` (`pathname`, `lineno`,
  `funcName`) and structlog's `CallsiteParameterAdder`; the JVM's `StackWalker` inside the log4j2
  appender and the JUL handler; V8's structured call sites inside pino's destination, winston's
  format chain and the console; Monolog's `IntrospectionProcessor`. The rendered line never
  carries it. Every case logs a value carrying `SNTL`.
- **Run.** `indexbench localize-replay -logs` scratch-ingests each app alone (one probe store at
  a time, deleted after), then calls the MCP `localize` tool through the server's JSON-RPC
  handler with `{service, error: <the line>}` at the default budget. Correct means the top
  candidate's file is the truth's and its symbol's bare name is the logging function's. An echo
  is an answer that matched a template and still carries `SNTL` (it fails the verdict outright);
  `sentinel_answers` counts every answer carrying it, matched or not.
- **Arms.** Baseline: the same harness built against `localize` at 6280cfd4 (before TL-4).
  After: 580b6e89. Both arms ran the same runners and libraries on the same host the same day.

Reproduce: `calibration/incidents/install-log-deps.sh <deps>`, then
`go build -o <scratch>/indexbench ./cmd/indexbench && <scratch>/indexbench localize-replay -logs
-deps <deps> -work <scratch dir> -java /opt/homebrew/opt/openjdk/bin/java -out <file>` (the
baseline arm: the same `localizereplay.go` built in a worktree at 6280cfd4, then `-baseline`).

## Results

Top-1 by language (correct of 24):

| arm | Java | Node | PHP | Python | all | carrying the line |
|---|---|---|---|---|---|---|
| baseline | 0 | 0 | 1 | 1 | **2/96 = .021** | 18 |
| after | 23 | 23 | 23 | 23 | **92/96 = .958** | 0 |

By the API that wrote the line (correct of n, baseline then after):

| API | n | baseline | after |
|---|---|---|---|
| Python `logging` | 16 | 0 | 15 |
| `structlog` | 8 | 1 | 8 |
| SLF4J (on log4j2) | 12 | 0 | 11 |
| log4j2 API | 6 | 0 | 6 |
| `java.util.logging` | 6 | 0 | 6 |
| pino | 12 | 0 | 11 |
| winston | 6 | 0 | 6 |
| console (`error`, `warn`, `info`, `log`) | 6 | 0 | 6 |
| Monolog (PSR-3) | 24 | 1 | 23 |

The console row is the measurement the brief asked for: `console.error` and its siblings read
as well as the logging libraries (6 of 6), since `util.format` placeholders and template literals
are the same shapes pino and winston use.

Why the baseline missed. A log line carries no frame, so the only seeds were quoted literals and
`name(` tokens. Most lines quote nothing: 69 of the 94 misses had no candidate at all. Where a
line did quote something (Monolog's JSON context, structlog's key-value values), the quoted
runtime value was grepped and found where the test input was written (the capture runners), so
the truth ranked second or lower in 18 misses, and 18 answers cited a pasted value back.

The after arm's misses are the four `refunds-flag-dynamic` cases, one a language: the message is
assembled into a variable and the call logs the variable, so the reader has no literal to make a
template of, and the line matches nothing (no candidate). Every template-matched answer named the
logging function first, including the multi-line calls, the escaped and quoted forms, the
merging-object and marker-first calls and the structlog JSON and key-value renderings.

## Cost

| Subject | Arm | Ingest wall, 3 runs | Store | Observations | Templates |
|---|---|---|---|---|---|
| celery v5.4.0 (387 .py) | before | 3.50, 4.38, 5.40 s | 12,959,744 B | 11,224 | 0 |
| celery v5.4.0 | after | 3.10, 3.21, 3.43 s | 13,017,088 B (+0.44%) | 11,324 | 100 |
| flask 22d9247 (83 .py) | before | 0.91, 0.96, 0.97 s | 2,203,648 B | 2,127 | 0 |
| flask 22d9247 | after | 0.90, 0.92, 0.96 s | 2,203,648 B (+0) | 2,130 | 3 |

Fresh scratch stores, CLI ingest, the host shared with other work, so wall time is noisy and the
difference is inside it; the reader's own cost, benchmarked, is 32 ms for a full pass over
celery's 387 files (it was 1.35 s before the reader walked back from each `(` instead of running a
per-file receiver regex, which the first measurement caught). An unchanged re-ingest reads no
file again: only changed candidates are re-read, and every candidate once when no template pass
has completed for the service.

No language reading moves: `LogTemplate` is in no dialect's `oracle_kinds`, and `lang-oracle`
measures each dialect's own extractor, never ingest. Re-run at 6280cfd4 and at HEAD on one
subject a language (flask, square/retrofit, expressjs/express, Seldaek/monolog, at their reading
pins, universal-ctags as the oracle), every metric and count is identical; only the timing
fields differ.

## Ceilings and what would move them (TL-14)

- A message built into a variable before the call (4 of 96 here). Trigger: an adopter's replay
  where such calls are a measurable share; the fix is a same-function assignment read, not a
  guess.
- A logger reached through a chain the registry does not name (`request.app.logger`,
  structlog's `log.bind(...).info`), a fully qualified `java.util.logging.Logger.getLogger`, or a
  project's own logger names. Trigger: ADR-061's `.fathom/` zones (the override for the
  universal defaults), or a miss in an adopter's replay.
- Go is not read (no Go logger API in the registry). Trigger: a Go adopter's incident replay.
- The prompt hook does not yet route a pasted log line to `localize` (a follow-up; `cmd/fathom`
  was out of this item's files).
