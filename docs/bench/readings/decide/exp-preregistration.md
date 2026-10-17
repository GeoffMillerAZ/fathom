# Stage 0's pre-registration: E1, E2 and E3 (EXP-1)

EXP-1 pre-registered 2026-10-16 at fathom commit fd091c4f (fd091c4f0d99ae13b355cdb914ae6e0bca0681b5), before any call: the questions, the computed truth, the sample and its seed, the baselines, the bands, every bar and the $2 cap are fixed here, and nothing below changes after the first request is sent (K19).

## Verdict, plainly

Nothing has been asked of any model yet. This page fixes what stage 0 of
`docs/plans/agentic-decisions.md` will ask Jev (TypeSafe's fast decision model, over
OpenRouter, through `decide.Open`), what counts as the right answer, and what result
would justify going on, so the readings cannot be tuned after the fact.

The $0 dry run (section 7) already shows something worth knowing before any spend: the
greps a careful engineer would write are strong on these questions, 0.925 to 1.0
accurate on the drawn samples. A template qualifies only if Jev beats grep on a band
holding at least half its items, so stage 0 tests whether a fast model reads code
better than grep, not whether it reads code at all. The test-assertion template cannot
qualify: every drawn test asserts and grep reads all 200 right. The whole of E1 is
estimated at US$0.019, about one hundredth of the cap.

## 1. What is fixed, and in what order

1. E1 runs first (the harness `internal/decide/exp_live_test.go`, EXP-3), then E2, then
   E3. The $2 cap covers all three: each later run's cap is $2 minus the spend the
   earlier readings record, passed as `FATHOM_DECIDE_EXP_CAP_USD` (the harness takes a
   lower cap, never a higher one).
2. Every call goes through `decide.Open` with provider `typesafe` and
   `transport = "openrouter"`, the key read only by the adapter from the `api_key_file`
   path `FATHOM_DECIDE_EXP_OPENROUTER_KEY_FILE` names; no harness reads or prints it.
3. Nothing here is changed once a call is sent. A correction found later is a new
   pre-registration with a new date, and the readings say which one they ran under.

## 2. Subjects, pinned

| Subject | Repository and commit | License | Why |
|---|---|---|---|
| fathom | this repository at fd091c4f0d99ae13b355cdb914ae6e0bca0681b5 | this repository's | the code the questions are for; every template has a large pool |
| websocket | github.com/coder/websocket at 9c8faadccd1b679e811a79ce506f8a10237251ad (2026-06-14) | ISC | no dependencies; heavy `%w` wrapping (57 uses) and context threading (77), mutex-guarded connection state, 35 tests; type-checks in under a second |
| bbolt | github.com/etcd-io/bbolt (go.etcd.io/bbolt) at a4ef19710eb7ddc41fd1155336baf331b5c57ba8 (2026-10-16) | MIT | real error handling in a database (84 `%w` uses, 167 `fmt.Errorf`), mutex-guarded `DB` and `Tx` state, 289 tests in two styles (`t.Fatal` and testify's `require`); type-checks in about two seconds |

The two open-source repositories are cloned under the runner's scratch directory, never
into this repository, each at the commit above. Each subject is ingested into its own
scratch store (`fathom ingest --store <scratch>/<name>.sqlite --service <name> --path
<checkout>`), never the shared daemon's.

## 3. Span selection and the seed

- **Call-graph spans.** A subject's spans are its Go functions and methods (kinds
  `GoFunc` and `GoMethod` in a `.go` file) with at least one call edge in or out: the
  store's typed edges when it has any, else the syntax call graph (the rule
  `cmd/indexbench`'s oracle uses).
- **Truth first, model-free.** Every span's five facts are computed (section 4) by
  type-checking its package with `golang.org/x/tools/go/packages` (tests included,
  `GOWORK=off`, `GOTOOLCHAIN=local`). A span whose package fails to load or type-check,
  whose file no loaded package holds (a build constraint for the runner's platform, a
  nested module) or whose declaration is not found is excluded and counted, never
  guessed.
- **Pools.** A template's pool is every computed span its fact applies to, less a span
  whose question text would pass 16 KiB (excluded as `too_long` and counted).
- **The draw.** Per template and subject, the pool is sorted by (subject, file, start
  line, name), shuffled Fisher-Yates from the last index with j = next() mod (i+1) on a
  splitmix64 generator seeded with the seed XOR FNV-1a-64 of "template/subject", and the
  first quota taken. The seed is 20261016 (`ask.SampleSeed`). Quotas, about 200 spans
  per template: fathom: 100, websocket: 50, bbolt: 50 (`ask.E1Quotas`). A subject short
  of its quota leaves the shortfall to the next unused spans of the subjects in that
  order.

## 4. Truth: five computed facts over a Go span (EXP-2)

`internal/ask`'s `spanFacts`, pinned by hand-labelled fixtures under
`internal/ask/testdata/facts` (tricky negatives included). A fact applies when the span
has at least one site, except executes_only, which applies to every Test func. The
definitions, word for word as `ask.FactDefinitions` holds them:

- `unchecked_error`: A call whose result list includes the built-in error type, anywhere in the function (function literals included), is a site; it is unchecked when the call is an expression statement, or when an assignment or var declaration puts its error result into the blank identifier _. The call a defer or go statement makes is not an expression statement. The fact holds when at least one site is unchecked.
- `wrapped`: A call to fmt.Errorf with a constant format (no * width or precision, no explicit argument index) is examined; each argument whose type implements error is a site, wrapped when its verb is %w and not wrapped for any other verb, %v and %s among them. The fact holds when every site is wrapped.
- `executes_only`: It applies to a top-level func named Test (the next rune, if any, not lower-case) taking one *testing.T, in a _test.go file, whatever its body. The fact holds when its body, function literals in it included, calls no testing method named Error, Errorf, Fatal, Fatalf, Fail, FailNow, Skip, Skipf or SkipNow and passes no *testing.T or testing.TB value to any call.
- `locked_write`: Only a method whose receiver's struct type declares a sync.Mutex or sync.RWMutex field before other fields (Go's convention: a field declared after a mutex is guarded by the nearest mutex declared before it) has sites: each assignment (=, op=, ++, --) to such a guarded field of the receiver, through any index, field selection or dereference. A site is locked when, in source statement order within the method (function literals ignored), a receiver.<mutex>.Lock() call (the promoted Lock of an embedded mutex) precedes it with no receiver.<mutex>.Unlock() call between them. RLock is not a Lock and a deferred call is no event. The fact holds when every site is locked.
- `threaded_context`: Only a function with a context.Context parameter has sites: each argument it passes, function literals included, to a callee's non-variadic context.Context parameter. A site is threaded when it is the parameter, a function literal's own context.Context parameter, a variable assigned from a threaded value or from a call given one (ctx2, cancel := context.WithTimeout(ctx, d)), or such a call itself; context.Background(), context.TODO() and anything else are not. The fact holds when every site is threaded.

`threaded_context` has no E1 template; it is computed for `fathom ask` (AQ-4).

## 5. E1's four templates

Each is one `predicate` question per span. The text sent is the template's statement,
then, for the lock template only, the receiver type's declaration, then the function
from its `func` keyword to its closing brace (no doc comment), framed exactly as
`ask.QuestionText` writes it:

````text
<statement>

Receiver type:
```go
<the receiver's type declaration>
```

Function <subject>/<file>, line <func line>:
```go
<the function>
```
````

Every request's State, verbatim:

> Each question below is a statement about one Go function, shown in the question. Judge each statement independently and give the probability that it is true.

No `TrueCriteria` or `FalseCriteria` are sent. One batched request per chunk: a chunk is
the next spans in draw order up to 64 questions and 96 KiB of state plus questions
(the bounds the obligations re-rank's `decisionBatchPlan` uses); the templates run in
the order below.

### errcheck: does the caller check the callee's returned error

> Every error this Go function receives from a call is checked: no call whose result list includes the built-in error type is used as a bare expression statement, and no such call has its error result assigned to the blank identifier _. Calls inside function literals count; the call a defer or go statement makes is not a bare expression statement; fmt.Print, fmt.Fprintf and Write-style calls count like any other call.

Truth: the template applies when `unchecked_error` applies; the statement is true when
`unchecked_error` does not hold. Lexical baseline, false when any expression matches the function's text, true otherwise:

```text
(?m)^\s*_\s*=\s*[\w.]+\(
,\s*_\s*:?=\s*[\w.]+\(
(?m)^\s*fmt\.(Print|Fprint)\w*\(
(?m)^\s*[\w.]+\.(Close|Write|WriteString|Flush|Remove|RemoveAll|Encode|Sync|Shutdown|Rollback)\(.*\)\s*$
```

### wrap: are errors wrapped with `%w`

> Every error value this Go function formats with fmt.Errorf is formatted with the %w verb: no argument whose type implements error is formatted with %v, %s or any other verb.

Truth: applies when `wrapped` applies; true when it holds. Lexical baseline, false when the expression matches the function's text, true otherwise:

```text
fmt\.Errorf\("(?:[^"\\%]|\\.|%[^w"])*"\s*,[^\n]*\berr\w*\b
```

### assert: does a test assert or only execute

> This Go test asserts: its body, function literals in it included, calls a testing method named Error, Errorf, Fatal, Fatalf, Fail, FailNow, Skip, Skipf or SkipNow, or passes its *testing.T (or a testing.TB) to a function.

Truth: applies when `executes_only` applies; true when it does not hold. Lexical baseline, true when any expression matches the function's text, false otherwise:

```text
\bt\.(Error|Errorf|Fatal|Fatalf|Fail|FailNow|Skip|Skipf|SkipNow)\(
[(,]\s*t\s*[,)]
```

### lock: is the lock taken before a guarded field is written

> Every write in this method to a receiver field that the receiver type declares after a sync.Mutex or sync.RWMutex field (guarded by the nearest such mutex declared before it) is preceded, in statement order within the method, by a Lock call on that mutex through the receiver with no Unlock call on it in between. RLock is not a Lock, a deferred call is not in between, and function literals in the method are ignored.

Truth: applies when `locked_write` applies; true when it holds. Lexical baseline over the
method's text only, true when the first expression (a Lock call) matches before the second (a field write) first matches, or matches and the second never does; false otherwise:

```text
\.Lock\(\)
(?m)^\s*[A-Za-z_]\w*\.\w+(\[[^\]]*\])?\s*([-+*/%&|^]?=[^=]|\+\+|--)
```

## 6. The reading (E1)

- **The model's answer** is true when its probability that the statement is true (the
  predicate's `Noul`) is at least 0.5. An invalid, declined or failed answer is counted
  and left out of accuracy, never scored as wrong or right.
- **Bands**, by that probability: below 0.1, 0.1 to 0.5, 0.5 to 0.9, 0.9 and above (each
  band includes its lower edge; the last includes 1).
- **Per band:** n, the share of the template's answered items, the model's accuracy
  against the computed truth with its Wilson 95% bounds (z = 1.959963984540054), the
  lexical baseline's accuracy on the same items and the model's lift over it (model
  accuracy minus baseline accuracy).
- **Per template:** n, accuracy, AUROC (the share of true and false item pairs the true
  item outscores, a tie counting half; undefined when either class is empty), the
  baseline's accuracy and the lift, p50 and p95 latency per request (nearest rank), the
  spend and the cost per 100 spans.
- **QUALIFIES** when one band holding at least 50% of the template's answered items
  reads accuracy of at least 0.9 and strictly beats the baseline's accuracy on those
  items (`ask.Read`, unit-tested in the gate).

## 7. Known before any call: the $0 dry run

The harness in dry mode (`FATHOM_DECIDE_EXP_DRY=1`: the kit's mock decider answers the
baseline's answer at 0.95 or 0.05; `decide.Open` is never called, nothing is dialed)
ran on 2026-10-16 over the three subjects above. Everything in this table comes from
computed truth and the greps, none from a model:

| | fathom | websocket | bbolt |
|---|---|---|---|
| call-graph spans | 21,883 | 261 | 975 |
| computed (excluded: file in no loaded package) | 21,865 (18) | 225 (36) | 954 (21) |
| errcheck pool / drawn | 9,323 / 100 | 127 / 50 | 470 / 50 |
| wrap pool / drawn | 776 / 132 | 29 / 29 | 39 / 39 |
| assert pool / drawn | 7,794 / 121 | 29 / 29 | 280 / 50 |
| lock pool / drawn | 208 / 194 | 4 / 4 | 2 / 2 |

| Template | Drawn | Statement true | Baseline accuracy | Too long | Requests | Estimated cost |
|---|---|---|---|---|---|---|
| errcheck | 200 | 149 | 0.935 | 15 | 4 | US$0.0048 |
| wrap | 200 | 180 | 0.975 | 6 | 4 | US$0.0038 |
| assert | 200 | 200 | 1.000 | 2 | 4 | US$0.0042 |
| lock | 200 | 144 | 0.925 | 1 | 5 | US$0.0060 |

What this fixes in advance: `assert` reads NONE whatever the model answers (its
baseline is already perfect and its AUROC is undefined, one class only); `lock` draws
almost entirely from fathom (the two open-source repositories hold six applicable
methods between them). The whole of E1 is 17 requests, about US$0.019 at the conservative
estimate.

## 8. E2: questions with no computable truth

About 100 items, two questions, asked through the same `decide.Open` helper and the same
stop rule, with a request State of: "Each question below is about one Go change or one
Go test failure, shown in the question. Answer each independently."

- **E2a, 50 items, `predicate`:** "This caller depends on the behaviour the diff
  changes: for some input, what the caller returns or does would differ because of the
  change." Items: non-test fathom functions with at least one typed-edge caller at
  fd091c4f, in the seeded order of section 3 (stream "e2-caller/fathom"); for each, the
  last commit that changed it (`git log -L :<function>:<file> -n 1 fd091c4f`), kept when
  that commit is among the 300 non-merge commits before fd091c4f and its diff changes a
  non-comment line; one caller drawn by the same stream from its callers sorted by key.
  The item shows the function's diff (at most 80 lines) and the caller at fd091c4f.
- **E2b, 50 items, `choice`** (F35's question): "What kind of failure is this Go test
  failure?" with options `code_bug` (the code under test is wrong and the test is
  right), `environment` (the machine, a tool, a file, a permission or the network, not
  the code), `timing` (a deadline, an ordering or a race: the same code can pass on
  another run), `outdated_expectation` (the code changed on purpose and the test still
  expects the old behaviour) and `other` (none of these). Items: commits among the 300
  non-merge commits before fd091c4f whose subject starts with `fix` and that change a Go
  file, in seeded order (stream "e2-failure/fathom"); at the commit's parent, in a
  scratch clone, the tests `fathom tests select --safe` names for the commit's changed
  files are run, and the first that fails there and passes at the commit is kept
  (fail-then-green). The item shows the failing test's output (absolute paths and the
  user name redacted, at most 60 lines) and the failing test's function.
- **Labels:** a Claude reviewer reads each item on subscription time, with the source
  around it and, for E2b, the fix commit's diff (which the model never sees), and writes
  the label before any E2 call (provenance `frontier`, no API spend). The owner's
  spot-check of a 20% sample is recorded only if the owner chooses to give one
  (provenance `human`, Q-DM5).
- **Reading:** agreement with the frontier labels by band (for E2b the band is the
  chosen option's probability), labelled "agreement with frontier labels", never
  accuracy; the baseline E2 must beat is the majority frontier label within each
  question (a constant answer). The items' text is never committed; the reading holds
  aggregates only.

## 9. E3: the agentic query prototype

Five questions, each asked of fathom at fd091c4f, where each answer is in the code:

1. Q1: which call paths to `decide.Open` can read a key or reach a provider before any
   consent or kill check?
2. Q2: which callers of `fetchSymbolFile` accept a `find_symbol` hit without checking
   `matched_via`?
3. Q3: which tests reach the obligations adr-rules assembly and would fail if a rule
   compiled from a stale section were served?
4. Q4: which call sites derive a language name from a `SourceKind` that `.js` and `.ts`
   share?
5. Q5: which functions decide whether a typed edge into an excluded file counts as a
   discovery gap?

- **Arms.** The table arm: the agent writes a typed spec (design section 6), fathom
  selects the rows (callers, blast radius, tests reaching) and computes what the
  section 4 facts answer, and Jev labels only the residue, one batched request per
  question. The reading arm: the agent answers by reading the files.
- **Grading.** Before either arm runs, a Claude reviewer reads the files and writes each
  question's reference answer (the complete set of call paths, callers, tests, call sites
  or functions, each with file and line). Each arm's answer is graded against it:
  correct (every element of the reference and nothing the files contradict), partial (at
  least half the elements and at most one wrong), wrong (anything less). "At least as
  correct" compares the grades in the order correct, partial, wrong.
- **Recorded per question:** rows selected, the computed and residue shares, judged
  rows, each arm's grade, each arm's context tokens (the input tokens the arm's session
  reports for that question, less the shared system prompt and tool definitions; where a
  session reports none, the bytes of tool output it read divided by four) and dollars.

## 10. The bars, copied verbatim from the plan

- EXP-M1, E1: per template, n, accuracy by probability band with each band's share of items and its Wilson bounds, AUROC, the lexical baseline's accuracy on the same items and the model's lift over it, p50 and p95 latency, cost per 100 spans and the spend; a template QUALIFIES when one probability band holding at least 50% of its items reads accuracy of at least 0.9 against the computed truth and the model beats the lexical baseline on those items; the verdict names the qualifying templates or NONE
- EXP-M2, E2: about 100 items with no computable truth (does this caller depend on the changed behaviour; what kind of failure is this test failure), labelled by a Claude reviewer reading the span on subscription time (provenance `frontier`, no API spend), with the owner's spot-check of a 20% sample recorded only if the owner chooses to give one (provenance `human`, Q-DM5); agreement by band under EXP-M1's bar, labelled agreement with frontier labels, never accuracy
- EXP-M3, E3, the agentic query prototype: 5 real questions from recent waves; the agent writes a typed spec (design section 6), fathom selects rows (callers, blast radius, tests reaching) and computes first (EXP-2), Jev labels the residue; the other arm reads the files; a Claude reviewer grades both against the files; recorded per question: rows selected, computed and residue shares, judged rows, correctness, context tokens per arm and dollars; KEEP when the table is at least as correct as the reading arm on at least 4 of 5 questions at no more than half its context tokens, STOP otherwise
- EXP-M4, the stage gate: stage 0's total spend (at most $2), each template's EXP-M1 and EXP-M2 verdict and EXP-M3's; OPEN when at least one template QUALIFIES, naming the winners, which are the only templates stage 1 may ask; CLOSED otherwise; filed as an observation either way

## 11. Spend: the $2 cap, the stop rule and the cost basis

- **Cap.** $2 for all of stage 0 (`ask.DefaultCapUSD`); a run's cap is $2 minus the
  spend already recorded.
- **Stop rule.** Before each request, the harness prices the request's conservative
  estimate; when the spend so far plus that estimate would pass the cap, that request is
  not sent, nor is any after it, and the unsent items are counted per template. Three
  failed requests in a row also stop the run. A failed request is charged its estimate.
- **Cost basis per request.** The typesafe adapter returns the provider's token counts
  (`decision.Usage`: input and output tokens, from the wire's `usage`); it does not pass
  OpenRouter's own `usage.cost` through. A request is priced from those reported counts
  when either is non-zero, else from the conservative estimate (one token per three bytes
  of state and questions, rounded up, plus 32 output tokens per question), at the
  model's listed price: US$0.042 per million input tokens, output free (the kit's
  recorded vendor price, and what OpenRouter's `usage.cost` billed in the 2026-10-09
  smoke, `provider-smoke-2026-10-09.md`). If OpenRouter lists a higher price for
  `typesafe/jev-1.13` on the run date, the run does not start until this page and
  `ask.ListedPrice` are amended in a commit made before the first call.

## 12. Running it

```sh
FATHOM_DECIDE_EXP=1 FATHOM_DECIDE_EXP_OUT=<scratch>/exp-e1.json \
FATHOM_DECIDE_EXP_SUBJECTS='fathom|<checkout>|<store>;websocket|<checkout>|<store>;bbolt|<checkout>|<store>' \
FATHOM_DECIDE_EXP_OPENROUTER_KEY_FILE=<absolute path, chmod 600> \
  go test -tags decideexp -run TestDecideExpE1 -count=1 -timeout 60m -v ./internal/decide
```

Add `FATHOM_DECIDE_EXP_DRY=1` for the $0 dry run; `FATHOM_DECIDE=off` closes the run
before anything is opened. The output holds aggregates only: counts, the reading, latency
and spend, never a span's text, path or name. The E1 reading (`exp-e1.md`, EXP-M1)
quotes it.
