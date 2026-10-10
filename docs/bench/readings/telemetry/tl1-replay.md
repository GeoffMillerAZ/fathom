# TL-7: localize on pasted traces and error payloads, before and after TL-1

**Verdict: PASS. Keep it.** Paste a stack trace or an error payload from Bugsnag, an
OpenTelemetry exporter, AWS Lambda or New Relic into `localize`, and it now names the
function that threw first in 98.4% of 440 answers (433), up from 25.9% (114) before TL-1. Every
language clears the 0.90 bar: Java 1.000, Python 0.991, PHP 0.991, Node 0.955. Before,
localize usually found the right frames and then ranked a caller first; a pasted payload
(as opposed to a raw trace) was close to useless (Bugsnag 4 of 88). Where it pays: an agent
handed an alert or an APM error goes straight to the throwing function, with no grep and
no reading of the trace by hand. What it does not replace: the error tracker itself (fathom
reads one pasted payload in the call and discards it, never fetches, never stores), and a
debugger for why the value was wrong. It pairs with the tracker the team already runs (the
paste is the integration). It costs nothing to run: no model, no daemon, a few seconds of
CPU per arm (four captures, four scratch ingests, 440 answers).

Row TL-7, `docs/plans/telemetry-links.md`. Decision: ADR-062. Design:
`docs/design/telemetry-links.md`. Data: `tl1-replay.json` beside this file.

## Method

- **Subjects.** `calibration/incidents/{java,node,php,python}/app`: one small shop app per
  language, 22 fault-injected cases each (88 cases), written to be hard where real code is:
  duplicate basenames (`billing/Util` and `shipping/Util`, both with a `round`), same-named
  methods in several classes (`validate`), constructors, a lambda or closure or nested
  function, recursion, a five-frame chain, chained causes (`Caused by:`, `raise ... from`,
  PHP's previous exception), and library errors raised under an app function.
- **Truth, tier A.** Each app's capture runner runs natively on the host (Java 27 through its
  multi-file source launcher with `-Xmx256m`, Node 26, PHP 8.5, Python 3.14) and reads the
  truth from the runtime's own structured trace, never from text: the innermost frame inside
  the app of the deepest error that reaches the app (JVM `StackTraceElement`, V8 call sites,
  PHP `getTrace()`, Python `extract_tb`). The bare name is the one the index gives the
  function: a JVM `lambda$summarize$0` is `summarize`, `<init>` the class, a V8 constructor
  `constructor`, a PHP 8.4+ closure its enclosing method.
- **Shapes.** Each case is captured five ways, built from the same runtime frames the way
  each sender lays them out: the raw trace as the runtime prints it; Bugsnag's event
  (`events[].exceptions[].stacktrace[]`, `inProject` set, the error first and its causes
  after); OTLP JSON (a span's `exception` event, or a log record, with
  `exception.stacktrace` and `code.*` attributes, old names for Java and PHP, new for Node
  and Python); the Lambda runtime's error response (Node `trace`, Python and Java
  `stackTrace`, Java causes nested; PHP, which has no managed runtime, a runtime layer's
  `stack` of `getTraceAsString` lines); and a New Relic error event with
  `error.stack_trace`. The deploy root is rewritten to `/app` (`/var/task` for Lambda), as a
  production trace carries the deploy root, not the repository's path. The payload layouts
  are emulated from each sender's documented fields, not captured from the SDKs themselves
  (the PHP OpenTelemetry stack text in particular follows that SDK's Java-style formatter).
- **Run.** `indexbench localize-replay` scratch-ingests each app alone (one probe store at a
  time, deleted after), then calls the MCP `localize` tool through the server's JSON-RPC
  handler with `{service, stack_trace: <payload>}` at the default budget. Correct means the
  top candidate's file is the truth's and its symbol's bare name is the injected function.
- **Arms.** Baseline: the same harness built against `localize` at 2b80f5f9 (before TL-1).
  After: 964ec6e9. Both arms ran the same capture runners on the same host the same day.

Reproduce: `go build -o <scratch>/indexbench ./cmd/indexbench && <scratch>/indexbench
localize-replay -work <scratch dir> -java /opt/homebrew/opt/openjdk/bin/java -out <file>`
(the baseline arm: the same file built in a worktree at 2b80f5f9, then `-baseline`).

## Results

Top-1 by language and shape (correct of 22 per cell):

| arm | language | raw | Bugsnag | OTLP | Lambda | New Relic | all |
|---|---|---|---|---|---|---|---|
| baseline | Java | 2 | 2 | 2 | 2 | 2 | 10/110 = .091 |
| baseline | Node | 2 | 1 | 2 | 2 | 2 | 9/110 = .082 |
| baseline | PHP | 22 | 1 | 17 | 13 | 22 | 75/110 = .682 |
| baseline | Python | 14 | 0 | 2 | 2 | 2 | 20/110 = .182 |
| baseline | all | 40/88 | 4/88 | 23/88 | 19/88 | 28/88 | **114/440 = .259** |
| after | Java | 22 | 22 | 22 | 22 | 22 | 110/110 = 1.000 |
| after | Node | 21 | 21 | 21 | 21 | 21 | 105/110 = .955 |
| after | PHP | 22 | 22 | 22 | 21 | 22 | 109/110 = .991 |
| after | Python | 22 | 22 | 22 | 21 | 22 | 109/110 = .991 |
| after | all | 87/88 | 87/88 | 87/88 | 85/88 | 87/88 | **433/440 = .984** |

Why the baseline missed. The frames were mostly found; the ranking was the defect. Every
resolved frame scored the same 10, a frame whose function was also named scored 8 more,
and graph convergence lifted the middle of the chain, so a caller (often the capture
runner's `main`) outranked the throw site; the truth sat at rank 2 or 3 in most misses. A
JVM frame matched the first sorted `Util.java` of two. V8's `at fn (path:l:c)` was read
only through the generic `path:line` match with no function, PHP's `#N path(line): fn`
not at all (PHP's raw and New Relic cells passed through the message line's
`in path:line`), and a JSON payload was read by the same regular expressions over its
escaped text: Bugsnag's `{file, lineNumber}` objects have no `path:line` in them at all.

What TL-1 changed (`internal/stackframe`, `internal/mcp/localize_tool.go`): one parser for
every grammar and payload shape, frames in depth order with the throw site first and
chained causes root first; the innermost resolved frame weighted as the throw site (a
bonus of 20, sized by this replay: at 10 a JavaScript constructor, which the trace names by
its class and so is never "named", fell below two callers); a function a trace names twice
counted once; a JVM frame's package choosing its file; a PHP frame known only by name
(`getTraceAsString` has no throw location) resolved by name and class.

## The seven misses after TL-1

- **Node `orders-summarize-callback`, all five shapes (5).** The throw is inside a named
  function expression passed to `map` (`function lineShare(q) {...}`). The JavaScript
  dialect indexes no symbol for it, so the enclosing indexed symbol of the throw line is
  `Order.summarize`, which localize names first, correctly for the index it has. The
  ceiling is extraction, not localize: a nested named function expression is not a symbol.
- **PHP `orders-checkout-wrapped`, Lambda (1).** A PHP runtime layer reports only the outer
  exception's `getTraceAsString`; the previous exception, where the throw happened, is not
  in the payload at all. Nothing that reads only the payload can name `Order::reserve`.
- **Python `orders-checkout-chained`, Lambda (1).** The same: the Lambda Python runtime's
  `stackTrace` is the outer exception's traceback only (`raise CheckoutFailed(...) from e`
  leaves the cause out).

No ceiling is owed (the verdict is PASS); these three are the next work if the bar rises:
a JavaScript symbol for named function expressions in call arguments, and nothing for the
two Lambda payloads (the information is absent from what the runtime sends).

## Threats to validity

- The apps are written for this replay (88 cases) and the payload layouts are emulated
  from documented fields; a real notifier's quirks (Bugsnag's code snippets, source maps,
  an SDK that truncates frames) are not exercised. The adopter census (TL-6) and a real
  payload from a consumer would move this toward a field reading.
- The harness reads one payload per call, as `localize` does; delivery is a separate
  question: `localize` is served on `/rpc/extended` only (ADR-062, O615).
