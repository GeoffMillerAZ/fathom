# Unreleased — enforce the configured findings producer policy

**Breaking:** manual imports and session outbox sync now honor the repository's
`[findings] producers` list. Earlier binaries parsed it without enforcing it.
This is an unreleased change, not a release-readiness claim.

| Configuration | Import behavior |
|---|---|
| `producers` omitted | Any trusted-catalog producer, preserving the legacy default |
| `producers = []` | No producer admitted |
| Nonempty list | Only exact IDs named; unknown catalog IDs remain refused |
| Explicit `--producer ID` | Further restricts the configured list; never bypasses it |

Before upgrading, inspect empty lists that may have meant “not configured.”
Replace them with the reviewed producer IDs the repository trusts, or omit the
field only if catalog-wide admission is intended. Do not add a broad wildcard:
IDs match exactly. Native criteria exports use
`reviewkit.criteria-import.deterministic`, `.human`, and `.model`; caps remain
`pre-scan`, `assumed`, and `assumed` respectively. An incomplete child observation
cannot close a whole rule.

The CLI reads configuration even when `--subject` is explicit and refuses
malformed configuration before opening the store. Automatic sync runs its
subprocess in the indexed project directory. Disallowed decoded events produce
`producer-not-allowed`, write no claim or cursor, and yield exit 1; operational
configuration failures yield exit 2. A mixed batch can import its allowed events
and reject the rest; inspect the receipt. Replay, ordering, dry-run, subject
binding, and catalog caps keep their existing contracts.

Go consumers gain the optional `ImportFindingsRequest.AllowedProducers []string`
field. Nil preserves catalog-wide admission; an explicitly empty non-nil slice
denies all. Callers that use unkeyed struct literals must convert them to keyed
literals. The client does not load a filesystem policy on the caller's behalf:
pass the desired boundary explicitly.

No on-disk schema or remote API changes accompany this fix. Do not infer support
for arbitrary mixed-version daemons or deploy over a running executable; test a
disposable store before an intentional binary upgrade.
