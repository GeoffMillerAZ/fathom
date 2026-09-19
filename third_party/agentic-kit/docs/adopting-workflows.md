# Adopting governed workflows outside Go

`kit-workflow` is a small, local JSON front door for sourced prose and portable
lesson checks. Its two prose operations are:

```sh
kit-workflow prose check < request.json > checked.json
kit-workflow prose export < export-request.json > export.json
```

Use `kit-workflow --help` to discover both command families; `-h`/`--help` also
work at family and operation levels, print usage and exit zero without reading input.

It does not call a model, reach a network, execute a command from input, write
to a destination, or issue a publication approval. `check` verifies a supplied
draft; `export` first performs that same fresh check and then returns Markdown
and a source-map sidecar as local stdout bytes. An export result is not
authenticated publication, permission to send/write, or durable approval.

## Build and run

Build a standalone binary from the repository root:

```sh
go build -o ./bin/kit-workflow ./cmd/kit-workflow
./bin/kit-workflow prose check < request.json > checked.json
./bin/kit-workflow prose export < export-request.json > export.json
```

The prose commands read one UTF-8 JSON value from standard input and write one JSON
value to standard output. Success exits zero; invalid/evidence/unsupported
failures exit one; an unsupported command exits two. JSON error output includes
its stable error kind. Inputs are capped at 32 MiB, nesting at 64, and reject
duplicate or unknown fields and trailing JSON. Prose limits in `config` apply
again to facts, drafts, sources, and final exports.

## Envelope

The exact CUE shape is [`cmd/kit-workflow/schema.cue`](../cmd/kit-workflow/schema.cue).
Every request is self-contained and versioned:

```json
{
  "version": "kit-workflow/prose/v1",
  "sources": [{"ref": {"id": "source-a", "version": "v1", "digest": "..."}, "content": "12", "media_type": "text/plain", "acquired_by": "caller", "acquired_at": "2026-09-13T09:00:00Z", "access": "public", "retain_until": "2026-09-13T11:00:00Z", "revoked": false}],
  "current": [{"id": "source-a", "ref": {"id": "source-a", "version": "v1", "digest": "..."}}],
  "request": {"principal": "reader", "topic": "example", "facts": ["... exact evidence-span fact ..."]},
  "draft": {"claims": ["... exact claim ..."], "units": ["... exact UTF-8 byte ranges ..."]},
  "config": {"policy_version": "policy/v1", "prompt_version": "prompt/v1", "model": "supplied-draft", "limits": {"max_facts": 256, "max_units": 256, "max_claims": 512, "max_bytes": 1048576, "max_tokens": 8192}},
  "non_factual": []
}
```

`export` additionally requires `"audience": "public"` or `"restricted"`.
For public export, any `restricted` source causes the governed prose kit to
redact the whole artifact and omit source-map entries. Create a public-only
artifact from public sources when publishable text is required.

The tool validates declared digest/reference/current-link consistency, source
retention/revocation fields, and exact UTF-8 evidence offsets. It cannot verify
that the caller's supplied bytes came from the named system, that the declared
principal/access rights are true, or that a supplied current pointer is the
real current revision. The JSON result repeats those boundary assertions so a
consumer cannot mistake local consistency for authenticated source access.

## Python producer example

Use ordinary JSON serialization; do not construct JSON through shell quoting.
The sample fixture contains the complete evidence shape and is a useful starting
point:

```sh
cue export ./cmd/kit-workflow/testdata/valid.cue --out json > request.json
python3 - <<'PY'
import json
from datetime import datetime, timedelta, timezone

with open("request.json", encoding="utf-8") as f:
    envelope = json.load(f)["valid"]
# Refresh this public synthetic example, not a real expired source.
now = datetime.now(timezone.utc)
stamp = lambda value: value.isoformat().replace("+00:00", "Z")
for source in envelope["sources"]:
    source["acquired_at"] = stamp(now)
    source["retain_until"] = stamp(now + timedelta(hours=1))
for fact in envelope["request"]["facts"]:
    fact["span"]["OccurredAt"] = stamp(now - timedelta(hours=1))
    fact["span"]["ObservedAt"] = stamp(now)
with open("request.json", "w", encoding="utf-8") as f:
    json.dump(envelope, f, ensure_ascii=False, separators=(",", ":"))
PY
./bin/kit-workflow prose check < request.json > checked.json
```

For an export, add `"audience": "restricted"` or `"public"` in the Python
object before writing it, then invoke `prose export`. Store or publish neither
output without a separate destination-specific authorization and transaction.
Real sources retain their authentic timestamps and authorization; never renew an
expired source merely to pass a check.

## Portable lesson checks

The same binary exposes a deliberately non-executing front door for one
`lesson-transfer/v1` public-synthetic bundle:

```sh
./bin/kit-workflow lesson validate \
  --lesson lesson.json --reproducer reproducer.json --attestations attestations.json
./bin/kit-workflow lesson adoption-check \
  --lesson lesson.json --reproducer reproducer.json --attestations attestations.json \
  --receiver receiver.json --assertion outcome.json
```

`validate` checks exact UTF-8 bytes, resource bounds, strict JSON,
domain-separated raw hashes, and the inert public-synthetic shape. It returns
`integrity: "bound"` and `reproduction: "not-run"`; it does not execute a
reproducer, command, or model request. `adoption-check` additionally checks
supplied receiver and outcome JSON, but returns `reproduction:
"asserted-unverified"` and `fresh: false`: those are caller assertions, not
trusted review, current-host, revocation, or reproduction evidence.

Version 1 has no free-form reproducer payload: its closed `input.case` selector
is only `unknown-family` or `unobserved-control`. This is a format boundary,
not a general-purpose DLP claim. Candidate, superseded, and revoked bundles may
be inspected for lifecycle/provenance, but only a reviewed, active bundle can
enter a receiver-owned adoption check.

Use the public synthetic fixture only as local test data:

```sh
./bin/kit-workflow lesson validate \
  --lesson kits/lesson-transfer/testdata/public-synthetic/lesson.json \
  --reproducer kits/lesson-transfer/testdata/public-synthetic/reproducer.json \
  --attestations kits/lesson-transfer/testdata/public-synthetic/attestations.json
```

Only a receiver-owned Go integration calling `lessontransfer.CheckAdoption`
with its own review-acceptance, current receiver, revocation, and reproduction
callbacks can produce a fresh adoption receipt. Neither CLI operation writes a
receipt, applies a remedy, changes policy, grants authority, or distributes a
lesson.

The public fixture has sanitized, real code-byte provenance recorded beside it;
the reproducer remains synthetic. A receiving host must nevertheless compute
and retain its own revision, configuration, oracle, and result evidence before
it can claim a fresh receipt.
