### `obligations`

What governs a file, as the obligations tool serves it to an agent — and, with
`--explain`, why (OC-32).

```sh
fathom obligations --service <id> [--file <path>] [--layer rules|claims|all] [--root <dir>] [--explain] [--json] [--store <path> | --url <daemon>]
fathom obligations promote <layer-rule-id> [--root <dir>] [--write]
```

Without `--explain` it prints the served list: the raw-fallback rows the tool prepends
for an uncompiled governing guide (ADR-037), then the compiled rules, in the order the
agent reads them, with `displaced`, `deduped` and the layer's coverage. `--explain` adds
every scope-matched candidate with its scope tier (`exact_file`, `package`,
`parent_dir`, `root`) and rank, the cap and the nearest tier's reserved slots, and each
displaced rule's reason — `duplicate`, `tier`, `rank`, `freshness`, `role` or
`reserve` — naming the served rule that held its place; `--explain --json` is exactly
the console's `/explain` section. The explanation runs the serving engine itself
(`semantic.ExplainObligations`) and never calls a decision-port provider; when `[decide]
rank_within_tier` is on it says serving re-orders within a tier. `--file` omitted asks
the root-scope question. `--layer` defaults to `rules`, the obligations tool's own
default. CLI only: the MCP `obligations` tool's arguments are unchanged (the agent
surface's argument schemas are pinned and paid by every session). Serving serves one
statement once: two claims whose normalized statements are identical (case, backticks,
asterisks, whitespace and trailing punctuation aside) take one slot, and the duplicate
is not counted as displaced (OC-33).

Beside each served rule the verb shows what the edit hook recorded of it over the last
30 days — `served`, `top_changed`, `still_served` (SR-61, surface reckoning F-27/U4;
JF-1 renamed `complied` and `violated`) — read from
the hook's signal log (`~/.fathom/rule-outcomes.jsonl` and its rotated generation), and
the console's `/explain` shows the same line under each rule of "What the agent saw at
the edit". `still_served` counts the repeat edits where the same rule still topped the
file's served set, and `top_changed` the turns where another rule did: both measure scope
persistence and churn, never whether an edit complied (a checkable rule denies and
records `blocked`). Rows an older binary wrote as `complied` or `violated` are read as
`top_changed` and `still_served`, never dropped. `--json` carries it as `outcomes`
(`{window_days, rules: [{claim_id, served, top_changed, still_served, blocked, refuted,
corroborated, class?}]}`), a truth-stated value:
**absent** with its reason when no hook has recorded anything on this machine, an
**error** when the log exists and cannot be read, **partial** when malformed rows were
left out, and a measured zero only when the log is there and the hook never served that
rule. `class` is GL-11's proposal (`ignored`, `effective`, `contested`,
`never_triggered`), shown only when the log is old enough to support it and never acted
on. The rule's `claim_id` (what `fathom claim refute|corroborate --claim` takes) is in
this output; the MCP `obligations` payload an agent reads carries neither the id nor the
record — the id rides only on the edit hook's own request (origin `edit-hook`).

After the compiled rules the verb lists the `.fathom/guard.cue` layer rules that govern
the path (SR-45w, F-26), each marked **checkable** or **not checkable**: checkable means
its kind has a deterministic pre-write check (`import_direction`, `boundary`), so it can
become an `enforce: true` guard the PreToolUse hook denies on; `no_cycles` and
`cross_service_direction` need the whole import or fleet graph and stay advisory, with
that reason shown. A rule governs a file when the file's directory is its `from`
(`import_direction`), outside its `allow` list (`boundary`), inside its `scope`
(`no_cycles`), or the service is its `from_service`; the root-scope question lists every
rule. `--root` names the checkout whose guard file is read (default: the service's
recorded root, when it is a directory on this machine; otherwise the answer says it did
not read one). `--json` carries it as `layer_rules` (`{source, rules: [{id, kind,
sentence, severity, enforce, checkable, why?}], note?}`). `--explain --json` is unchanged
(the console's `/explain` section).

`fathom obligations promote <id>` compiles a checkable layer rule to an `enforce: true`
guard (`guardspec.GuardFromCheckableRule`). The default is a dry run that prints the
stanza as it would read; `--write` adds the one `enforce: true` line to that rule in
`<root>/.fathom/guard.cue` and changes no other byte, and the result must load back with
only that rule changed or nothing is written. A not-checkable rule is refused with its
reason, and a rule already enforced is a named no-op. `FATHOM_ENFORCE=off` or `[guards]
enforce = false` still turns every enforced rule back to advisory.

---
