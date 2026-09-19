# Adopt sourced prose with a guarded publication boundary

Start with the [offline scenario](../scenarios/sourced-prose/README.md):

```sh
go test -race ./scenarios/sourced-prose -count=1
```

Its importable entry point is:

```go
report, err := sourcedprose.Run(ctx)
// import sourcedprose "github.com/geoffmilleraz/agentic-kit/scenarios/sourced-prose"
```

Inspect `report.Export.Markdown`, `report.Export.SourceMap`, the HITL receipt,
invalidation closure and effect counts. This is a deterministic integration
fixture. No setup-time target, live model quality or real consumer adoption has
been measured by running it.

## Supply the authorities, not just IDs

`NewSourcedPipeline(client, sources, reviewer, config, clock)` needs your
`llmclient.LLMClient`, authenticated `SourceProvider`, independent
`SupportReviewer`, versioned policy/prompt/model configuration and explicit bounds.
The fixture's narrow semantic oracle is not a production reviewer.

Represent exact source versions with `SourceRef` and `SourceSnapshot`. Resolve the
requested bytes without substituting the latest version; implement `Current` so
stale evidence fails. Enforce principal access, revocation and retention. Facts
use canonical evidence-span provenance and inclusive-end byte locators; generated
claim ranges use exclusive ends. Extraction from PDFs or cells needs an authorized
text snapshot with preserved extraction provenance, not invented byte coordinates.

Use derived facts for exact supported arithmetic. A historical flag permits
resolving an older version; it does not establish that current-tense wording or a
comparison is supported. Supply an independently evaluated/human policy for those
claims. Keep reference integrity, semantic support and source authority separate.

## Prepare bytes before asking for approval

Call `Generate`, then `CheckDraft` for the actual edited draft, then `Export`.
The required `PublicationAuthorizer` checks who may prepare this audience's bytes:

```go
func(ctx context.Context, principal string, receipt prose.ProseReceipt, audience string) error
```

Keep this callback non-mutating. **Do not call HITL `Commit`, consume an approval,
or send externally inside it:** Export still performs checks and rendering after
authorization and can fail. An export receipt is not a signature or bearer token.

The fixture captures a target snapshot before generation and atomically installs
the completed export only if target version and evidence/config digests still
match. SourceProvider reads the same transaction store that owns publication state.
All correction, retention-policy and revocation updates must participate in that
authority boundary. An independently mutable source map plus a copied digest does
not prevent a correction race. Avoid reentering the transaction store from its
update callback: use the owned transaction snapshot for local checks.

Public export conservatively redacts the complete artifact if any supplied input
is restricted. Generate a fresh public-only artifact for mixed-access documents;
claim IDs or selected citations cannot declassify model context.

## Bind the exact publication to durable HITL

Build `hitl.Engine[State, PublishOp]` with a pure applier and deep cloner. The same
applier previews and commits; it must not send, increment an external counter or
perform I/O. In the scenario it requires the exact trusted prepared export and
mutates only transaction-owned publication history/effect state.

`PublishOp` contains the complete export plus config digest. HITL binds its canonical
typed-JSON operation digest, action, resource, authenticated actor, target version,
evidence/config digests and expiry. Supply authenticated `CommitConfig` callbacks,
then `Propose`; serialize a trusted store snapshot if needed; call `Commit` against
that store. The same operation type must round-trip losslessly after restart.

The transaction adapter must atomically verify, consume and apply, and roll back
on error or cancellation. The memory adapter plus JSON reconstruction is a
reference, not disk durability. Protect real persisted approvals and replay
tombstones from tampering; define bounded capacity and retention without silently
evicting replay protection.

Corrections must invalidate reverse dependencies, clear preparation, advance target
version and update evidence digest in the same transaction. Prior publications
remain historical audit records, not current authority. New wording needs new
checking and approval. Bound approval expiry by source retention/authorization
validity as well as your approval policy; later revocation must advance or revoke
the same authoritative state. Hashes alone cannot notice a source changing elsewhere.

## External destinations require reconciliation

Use `Prepare`, then `Send` with a destination-specific adapter that forwards the
stable idempotency key and does not silently retry. Sending is durably recorded
before calling the adapter. An error or missing receipt leaves unknown; recovered
sending/unknown records cannot send again. `Reconcile` must query authoritative
destination knowledge without repeating the effect. Eventual absence or a timeout
does not prove non-application. Destination deduplication and fencing remain your
responsibility; arbitrary external systems have no generic exactly-once promise.

The scenario's seeded remote result applies once, loses its receipt, restarts and
reconciles successfully with effect count still one. Its tests also show unsupported
wording, caller-asserted identity, changed export bytes, correction between export
and installation, stale approval and replay refusing without publication effects.
