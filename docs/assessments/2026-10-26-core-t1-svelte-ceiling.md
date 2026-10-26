# svelte named ceiling routes (X-R1)

Measured baseline: [svelte reading](../bench/readings/lang/svelte.json). No new reading is claimed.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| Svelte own compiler parser | svelte/compiler 5.57.2 | Existing first-party reading already measures components: symbol precision .3816. It parses let declarations as variables while the shared dialect names constants under M1/S3a. |
| Shared JS/TS canonical kind change | M1/S3a/K19; ruling C-R1 (#20, 2026-10-26T01:44:25Z) | Owner keeps the current kind rules and ceiling. Canonical-variable changes would affect JS/TS plus all hosted frameworks and require their K29 regrades. |
| Component-specific kind admission | K19 current rule / C-R1 | Changing the intersection only for components is an owner-rule change. The owner has not authorized it. It is not hidden inside an oracle adapter. |
| Component-to-TS file universe | current hosted file resolver / O979 | The written limit for imports to .ts targets remains. The measured host-only file edges are not proof of ingest edges to those files. |

Ruling C-R1 (#20) retains the existing ceiling. The brief unmeasured label was wrong: the committed compiler reading already exists.

Governing row: `LB-Q-svelte-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.
