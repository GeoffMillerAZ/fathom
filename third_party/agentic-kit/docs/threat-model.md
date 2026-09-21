# Threat model — kit-as-dependency

This is the honest threat model for "I imported agentic-kit into my agentic
workload." It covers what a malicious prompt, a malicious tool result, or a
malicious/misconfigured caller configuration **can** and **cannot** reach,
given the kit's actual, shipped security foundations
(`foundations/security/*`, plus the trifecta/tool-poisoning defenses in
`foundations/agent/tool-registry` and `foundations/agent/mcp-client`). Nothing
below invents a guarantee beyond what the code already does — where the kit
falls short, it says so.

## 1. What's in scope (mitigations that exist and are tested)

**Prompt-injection detection.**
`foundations/security/injection-detector` scans untrusted content against a
data-driven rule set (no hard-coded org-specific patterns — callers supply
`[]Rule` from CUE or programmatically) and ranks severity. In
`kits/agent-harness`, **when a `Detector` is wired** (`Deps.Detector`
non-nil — a nil detector means no screening, as the harness package doc
states), every tool result is screened before it re-enters the
conversation: a flagged result is either blocked (replaced by an `is_error`
tool_result naming the detection — the raw payload never reaches the
transcript) or annotated, per harness configuration. Recall on the kit's
seeded corpus is 22/22 after an uplift pass; that corpus is necessarily finite
— see §2.

**Untrusted-content fencing.**
`foundations/security/untrusted-content-wrapper` fences untrusted external
content (user documents, tool output, fetched web pages) between per-wrap
unguessable nonce delimiters, so an embedded fake closing fence forged by the
content itself carries the wrong nonce and stays inert data. This makes the
*boundary* between trusted and untrusted content visible and hard to forge —
it does not stop a model from being persuaded by what's inside the fence (see
§2). Its own package doc states this outright: "This is defense-in-depth, NOT
a security boundary."

**Instruction precedence.**
`foundations/security/instruction-hierarchy` enforces
system > developer > user > untrusted: a lower tier cannot programmatically
override a higher one in code that consults it. It is a data structure and a
precedence rule, not an enforcement mechanism inside the model itself — a
sufficiently persuasive untrusted payload can still influence model output
directly; the hierarchy governs how the *kit's own code* treats content by
tier, not what the model chooses to attend to.

**DLP / secret-shaped redaction.**
`foundations/security/pre-llm-dlp-gate` scans and redacts secret-**shaped**
content before it reaches an LLM (PEM blocks, `AKIA`-prefixed AWS keys,
GitHub/Slack tokens, JWTs, `AIza`-prefixed Google keys under
`ExtendedPolicy`). Its own package doc is explicit: "this is best-effort,
regexp-based redaction of strings that look like credentials — it is NOT
DLP-as-guarantee. Base64/hex-encoded secrets, secrets split across multiple
segments, novel token formats, and anything without a recognizable shape pass
straight through." `foundations/security/redaction-executor` applies a
caller-supplied policy to a textual **fixpoint** (idempotent under repeated
application; fuzz-tested after a real single-pass leak was found), but the
policy's rule quality is entirely the caller's responsibility — the executor
guarantees convergence, not coverage. `foundations/security/rule-core` is the
shared engine under all four regex-based scanners (injection-detector,
pre-llm-dlp-gate, redaction-executor, `foundations/quality/dlp-evaluator`),
so this is one audited engine, not four independent ones.

**Resource bounds on subprocess execution.**
`foundations/agent/bounded-exec` runs subprocesses with a hard wall-clock
deadline, byte caps on captured stdout/stderr (with explicit
`StdoutTruncated`/`StderrTruncated` flags — truncation is surfaced, never
silent), an explicitly supplied environment (no ambient env inheritance by
default), and a fixed working directory. Its own package doc states plainly
what this is not — see §2.

**Deny-by-default tool execution.**
`foundations/environment/tool-execution-policy` provides a `Decide` function
consulted, when wired, before every tool handler dispatch via
`foundations/agent/tool-registry`'s `Options.Policy` hook — a tool with no
policy match is denied, not permitted. Every dispatch failure mode (unknown
tool, policy denial, handler error/panic, timeout, cancellation) surfaces
in-band to the model as an `is_error` tool_result, never as an uncaught Go
error or a crashed process.

**Lethal-trifecta capability tracking.**
`foundations/agent/tool-registry`'s `CapabilitySet` requires every registered
tool to declare `CapReadsUntrusted` / `CapReadsPrivate` / `CapWritesExternal`.
A nil capability set is treated as UNKNOWN, not clean — under
`TrifectaStrict`, an unknown tool is assumed to exercise all three
capabilities. When a toolset's declared-capability union would complete the
trifecta (a tool that reads untrusted content + a tool that reads private
data + a tool that writes externally, all registered together), `Register`
for the completing tool fails closed with a typed `*ErrTrifectaBlocked` —
before any token is spent. Overriding requires a named `TrifectaOverride`
struct with `AuthorizedBy` and `Reason` — a bare boolean cannot bypass it.
`foundations/agent/mcp-client`'s `BridgeOptions.CapabilityMap` extends this to
foreign MCP tools bridged into the registry. **This taxonomy is
self-declared** — see §2 for what that means.

**Tool-description integrity (tool poisoning).**
`foundations/agent/mcp-client` computes a SHA-256 hash of
(name ‖ description ‖ inputSchema) for every tool at first listing and
re-verifies it on every subsequent `ListTools` — so the description the
model sees (which always enters context via a listing) is pin-verified. A
changed description returns a typed `ToolChangedError` instead of silently
forwarding a modified instruction into the model's context. Rebinding
requires an explicit `Repin` call, so any drift is visible and auditable
rather than silently absorbed. **`CallTool` itself performs no pin
re-verification** — description drift between a listing and a call is caught
at the *next* `ListTools`, not at call time; a pre-call pin check is an open
hardening item tracked in TRACKING.md's pull queue, not a shipped behavior.

**Audit trail.**
`foundations/provenance/event-store` is an append-only, hash-chained event
store; `foundations/review/correction-ledger` and
`foundations/review/claim-lifecycle` are append-only finding/claim ledgers;
`foundations/investigation/investigation-journal` is an append-only step
journal. Every AI-authored artifact carries a reproduction key
(`foundations/provenance/ai-provenance`: model, prompt hash, params hash,
input digest) so a claim can be traced back to what produced it. The MCP call
log (`kits/mcp-server`) never records tool arguments or results unless the
caller opts in via `LogPayloads` — payload logging is off by default.

## 2. What's explicitly NOT guaranteed

[VISION.md](../VISION.md) states this as core belief 6, verbatim, and it is
the ceiling for every claim in this document — nothing here may promise more
than this:

> **Honest contracts only.** No capability theater: the wrapper is
> defense-in-depth, not a security boundary; bounded-exec is resource
> confinement, not a sandbox; DLP is best-effort shape redaction; append-only
> stores have no deletion story. Overclaiming is treated as a defect class
> equal to a failing test — it is how trust in a foundation dies.

Concretely, unpacked against the mitigations in §1:

- **The untrusted-content wrapper and instruction hierarchy are
  defense-in-depth, not a security boundary.** A sufficiently persuasive
  prompt-injection payload inside a fenced block, or inside untrusted-tier
  content, can still influence what the model does — fencing and precedence
  make the attempt more visible and more consistently handleable by the
  kit's own code, they do not make the model immune to persuasion.
- **`bounded-exec` is resource confinement, not a sandbox.** A subprocess it
  runs has the host's full ambient authority — filesystem, network, memory,
  CPU, privileges — bounded only in wall-clock time and captured-output
  bytes. There is no filesystem jail, no network policy, no privilege drop.
  Real isolation requires OS-level primitives (containers, seccomp,
  namespaces, a VM boundary) that a stdlib-only kit deliberately does not
  provide.
- **DLP is best-effort shape redaction, not a data-loss guarantee.** Anything
  that doesn't look like a recognized secret shape — including secrets
  encoded, split across segments, or in a novel format — passes straight
  through pre-llm-dlp-gate and redaction-executor untouched.
- **Append-only stores have no general deletion story.** Event stores,
  journals, and ledgers are append-only by design (VISION belief 2: replay
  requires an unbroken record). The one exception is narrower than it
  sounds: `foundations/security/tokenization`'s `EncryptedVault` /
  `ShreddableVault` gives **tokenized** values a crypto-erasure path
  (`Shred(namespace)` destroys the per-namespace AES-256-GCM key, making
  ciphertexts permanently unrecoverable) — but that only covers content that
  was routed through the vault's tokenization step in the first place.
  Anything written directly into a journal, event store, ledger, or replay
  cassette outside that vault has no deletion path at all.
- **The lethal-trifecta taxonomy is self-declared.** A tool that lies about
  its own capabilities (declares no `CapReadsPrivate` while actually reading
  secrets) defeats `TrifectaStrict` entirely — the mechanism's value is
  making a *truthfully declared* trifecta auditable and blockable at
  registration time, and forcing an explicit, provenance-bearing override
  when one is truly needed; it cannot detect a capability a tool author hid.
- **Tool-poisoning defense only re-verifies what it already pinned.** If an
  attacker controls the very first `ListTools` response a client ever sees
  (before any hash is pinned), the poisoned description is trusted from that
  point on — the mechanism defends against *drift after first contact*, not
  a compromised introduction.
- **Injection-detector recall is bounded by its rule corpus.** 22/22 on the
  kit's own seeded corpus is a real, tested number — it is not a claim about
  recall against payloads the corpus doesn't yet cover; it is a rule-based
  detector, not a guarantee of catching novel attack phrasing.
- **Dispatch-time capability re-checking does not exist.** Trifecta detection
  runs at tool *registration*, once, for the whole toolset — not per call.
  This is a deliberate seam (the trifecta is a property of the assembled
  toolset, not of any single invocation), not an oversight, but it means a
  runtime capability change after registration is not re-evaluated.

## 3. If you need more, you must add it yourself

- **True sandboxing.** If your workload runs a tool whose blast radius must
  be bounded by more than wall-clock time and output bytes — untrusted code
  execution, arbitrary shell commands from a less-trusted source, anything
  where "the subprocess could do real damage" is a live concern — put a real
  OS-level sandbox (a container, a VM, `seccomp`/namespaces, or an
  OS-native jail) around the process `bounded-exec` runs. The kit will not
  do this for you; it is explicitly out of scope for a stdlib-only
  dependency (VISION belief 7).
- **True secret deletion outside the vault.** If regulatory or contractual
  obligations require you to be able to permanently remove a specific
  record's contents from a journal, event store, or replay cassette, route
  that data through `foundations/security/tokenization`'s vault *before* it
  is written anywhere append-only, so `Shred` has something to erase. Do not
  assume any other append-only primitive in this kit can forget something
  once it is written — none of them can.
- **Guaranteed injection/DLP coverage.** If your threat model requires
  catching a specific, known attack pattern or secret shape, add it to your
  own rule set / policy — `injection-detector`, `pre-llm-dlp-gate`, and
  `redaction-executor` are all explicitly data-driven for exactly this
  reason. Do not treat the shipped default rules as exhaustive; they are a
  reasonable starting catalog, not a certification.
- **A truthful capability taxonomy.** `TrifectaStrict` only works if every
  tool's `CapabilitySet` is accurate. If you bridge in tools you do not
  control (foreign MCP servers, third-party integrations), audit or
  conservatively over-declare their capabilities yourself — an
  under-declared capability set is a hole the kit cannot see through.
- **Runtime, per-call re-authorization.** If your workload needs a
  capability check that reacts to *runtime* conditions (a tool that is safe
  most of the time but must be blocked given a specific conversation state),
  build that as a `tool-execution-policy` `Decide` rule wired at dispatch —
  the trifecta mechanism itself will not do this; it is a one-time
  registration-time gate by design.
