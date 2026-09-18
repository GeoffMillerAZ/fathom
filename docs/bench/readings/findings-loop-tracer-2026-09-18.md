# The findings loop, traced end to end (2026-09-18)

## Verdict, in plain language

The loop works on real binaries, with no model and no network between the two
systems. The review kit published five findings from its outbox into a fathom
store through `leadsman findings publish --target fathom` and got a receipt
(five delivered); a coding agent's pre-edit hook on the anchored file then
showed the open finding beside the governing rules, with its class, producer,
rule id, remedy, citation, observation time and source revision; a closure
event published the same way retired it, and the next pre-edit hook on that
file served nothing about it; replaying the original outbox produced four
duplicates and one stale event, none delivered; the neighborhood query still
lists the closed finding as precedent, labeled superseded. This is the tracer
platform-engineering ADR-0103 asks for before any "implemented" claim (RP-9),
and the first executable proof of fathom ADR-040 decisions 2, 3, 5 and 7. It
is a mechanism proof on one repository with one producer. It is not a
measurement of whether an agent acts on the finding: that is the
`findings-loop` scenario corpus (FL-24), committed and not yet run.

## What ran

- fathom built from the integration branch (`fathom-merge`), a scratch SQLite
  store ingested from the same tree, a scratch `serve-http` daemon on a local
  port, and the pre-edit hook invoked exactly as Claude Code invokes it
  (PreToolUse JSON on stdin).
- `leadsman` built from platform-engineering `kits/review-kit` at the
  findings-projection change, run from a scratch git repository holding the
  outbox the `findings-loop` corpus materializes.

## Receipts

Publish (five observations):

```
{"schemaVersion":1,"target":"fathom","delivered":5,"duplicate":0,"stale":0,"unclassified":0,"noFingerprint":0,"rejected":[],"truncated":false,"cursors":[{"producer":"reviewkit.system-review","latestSourceVersion":1}],"publishedAt":"2026-09-18T09:34:12.593167Z"}
```

Closure of `RK-PURGE-ERRWRAP-1` (source version 2):

```
{"schemaVersion":1,"target":"fathom","delivered":1,"duplicate":0,"stale":0,"unclassified":0,"noFingerprint":0,"rejected":[],"truncated":false,"cursors":[{"producer":"reviewkit.system-review","latestSourceVersion":2}],"publishedAt":"2026-09-18T09:34:18.010159Z"}
```

Replay of the original outbox (idempotent; the closed finding's version 1 is stale):

```
{"schemaVersion":1,"target":"fathom","delivered":0,"duplicate":4,"stale":1,"unclassified":0,"noFingerprint":0,"rejected":[],"truncated":false,"cursors":[],"publishedAt":"2026-09-18T09:34:18.363826Z"}
```

## The hook, before and after closure

Before (pre-edit hook on `cmd/fathom/purgecmd.go`, after the governing rules):

```
open findings on this file (1, showing 1):
- [finding · error-handling · reviewkit.system-review · RK-PURGE-ERRWRAP-1] recordPurge returns the raw json.Marshal/file-I/O error verbatim, with no purge-record path or step named, so a failed purge-history write cannot be diagnosed from the error message alone. — remedy: Wrap each returned error with fmt.Errorf and %w, naming the purge-record path and which step failed (marshal / mkdir / open / write). — cmd/fathom/purgecmd.go#L418-L420 — observed 2026-09-17T18:00:00Z at 687ce6a9
```

After the closure event, the same hook on the same file:

```
(no findings block)
```

## Precedent survives closure

```
findings near "recordPurge" (hops<=2)
  hops=0 [error-handling] recordPurge returns the raw json.Marshal/file-I/O error verbatim, with no purge-record path or step named, so a failed purge-history write cannot be diagnosed from the error message alone. [SUPERSEDED — precedent, not live]
    subject=ext:reviewkit.system-review/geoffmilleraz/fathom/fathom/system-review/2026-09-17/purge-record-error-wrap scope=[cmd/fathom/purgecmd.go] status=superseded
  hops=2 [performance] cmdGenClaudeMd's ListAllSymbols call loads a service's entire symbol table into memory in one round trip before any output is written; a service with a very large symbol count has no incremental or paged path.
    subject=ext:reviewkit.system-review/geoffmilleraz/fathom/fathom/system-review/2026-09-17/genclaudemd-page scope=[cmd/fathom/genclaudemd.go] status=fresh
```

## What this does not show

- Agent behaviour: whether a capable model applies the remedy when shown it.
  That is FL-24's paired criterion `finding_addressed`, unmeasured.
- Two producers disagreeing on one anchor; a moved anchor at serve time
  (`anchor_current: false`); a revocation. Each is covered by conformance
  fixtures under `testdata/contracts/findings-import/`, not by this run.
- Organizational admission: a receipt is delivery evidence, never approval
  (platform-engineering ADR-0103).

## Reproduce

```
go build -o <bin>/fathom ./cmd/fathom
fathom ingest --store <scratch>.sqlite --service fathom --path <checkout>
(cd <platform-engineering>/kits/review-kit && go build -o <bin>/leadsman ./cmd/leadsman)
mkdir -p <scratch-repo>/.reviewkit/outbox && cd <scratch-repo> && git init
# copy the outbox the findings-loop corpus materializes (testdata/scenariobench/findings-loop.json, subject.setup)
leadsman findings publish --target fathom --outbox .reviewkit/outbox/projection.json --receipt-out .reviewkit/outbox/receipt.json \
  --fathom-bin <bin>/fathom --fathom-store <scratch>.sqlite --fathom-service fathom \
  --organization geoffmilleraz --repository fathom --component fathom
fathom serve-http --store <scratch>.sqlite --addr 127.0.0.1:<port> &
printf '{"tool_name":"Edit","tool_input":{"file_path":"<checkout>/cmd/fathom/purgecmd.go"},"cwd":"<checkout>"}' | fathom edit-hook --pre --url http://127.0.0.1:<port>
```
