# Durable approval, commit and recovery

GW-11 extends `kits/hitl` inside the approved G8 composition. `Engine` retains its
pure preview/apply contract and existing defaults. `Committer[State, Op]` is an
opt-in host composition; it does not install a new harness gate or a bearer token.
The workflow design §3 extends decision 0002's synchronous approval path with
durable decisions; the host still owns its approval endpoint and persistence.

The trusted host supplies `Authenticate(ctx)` and `Approve(ctx, actor, binding,
preview)`. Identity comes from authenticated context, never model-provided fields.
Approval denial, errors, cancellation or missing dependencies cannot create a
decision. The decision ID locates a trusted stored decision; presenting an ID or
a fabricated `Decision` grants no authority without the matching authenticated
actor and current binding. Hosts must protect this store and these callbacks from
model access. Serialized snapshots are not signed portable authorization proofs.

`Propose` owns the operation and state before preview. The binding contains action,
resource, canonical operation digest, expected target version, evidence digest and
configuration digest. The host may place an artifact digest, audience or target
details in the operation payload; no dependency on another kit is required.

`OperationDigest` v1 is lowercase hex SHA-256 of the byte prefix
`hitl-operation-v1\0` followed by the compact `encoding/json` encoding of the typed
operation. Map keys use Go's JSON ordering; use the same Go operation type after
restart. This is not RFC 8785 canonicalization. Operations and state must round-trip
losslessly through JSON; custom codecs must be pure and deterministic. Every preview,
apply and send receives a newly decoded owned value. Preview and approval callbacks
cannot mutate the caller's operation or stored state through supplied references.

`TransactionStore` binds one resource to its authoritative state, monotonic version,
evidence/configuration digests, decisions, receipts and attempts. The host increments
the version on every target mutation and updates digests when evidence or policy
changes. `Commit` reloads the decision, checks actor, exact binding and operation,
current target/evidence/configuration, expiry and revocation, consumes the decision,
and applies the Engine operation within the same isolated transaction. It publishes
the new state, incremented version and bound receipt only on success. A failed
applier, even one that mutates before returning an error, leaves persisted state and
decision unchanged. Concurrent replay cannot publish a second effect.

The adapter must invoke transaction callbacks exactly once, serialize them, roll
back on error/cancellation and acknowledge success only after durable commit. A
transaction callback cannot reenter the same store. A real adapter that cannot
determine its own commit outcome must reconcile that outcome before admitting more
work; the memory reference cannot prove a database driver's guarantees.

`MemoryCommitStore` bounds decision count using its required capacity and serialized
snapshot bytes to 16 MiB. Each decision owns at most one local receipt or one remote
attempt. Exhaustion returns `ErrCommitCapacity`; nothing is silently evicted, because
losing consumption records would allow replay. `View` returns an owned snapshot for
caller-managed serialization; `RestoreMemoryCommitStore` validates format version 1,
duplicates, bindings, timestamp relationships, consumption/receipt/attempt links and
capacity. It is a reconstruction reference, not a production storage adapter.

For remote effects, `Prepare` consumes the decision into a durable prepared attempt
without applying local state or sending. `Send` verifies the current authorization
again and durably transitions to sending before invoking the sender once with the
stable `hitl-<decision ID>` idempotency key. Expiry is checked at that reservation
boundary. An error, empty receipt or cancellation after reservation is unknown; if
outcome persistence fails, sending remains durable. After restart, sending is treated
as unresolved. The host's sender must honor context and must not silently retry.

```
prepared -> sending -> succeeded
                   -> unknown -> reconciled succeeded / not-applied / unknown
```

Each decision has a fixed bound of one send. No state returns to prepared. A
reconciler queries the destination without sending: not-applied requires authoritative
settled non-application, not an eventually consistent absence or timeout. Unknown or
sending attempts block later commits/preparations/sends for the same resource until
reconciled. Reconciliation may observe a historical outcome after expiry or revocation;
it grants no new execution authority. A further send requires a fresh decision after
reconciliation. Hosts own in-flight fencing, destination deduplication, authorization,
data retention and backup integrity. There is no generic exactly-once guarantee.

Revisit when: a consumer needs cross-resource transactions, repeated sends under one
decision, signed portable receipts or another persistence format. None is pre-built.
