---
name: catalog-doctor
description: Diagnose catalog resolution, provenance, drift and evidence preparation. Use for catalog cycles, conflicts, stale patches, missing items, integrity failures or unexpected model packets.
---

# Diagnose Catalogs

Read ../../README.md. Run diagnostics against copies or existing read-only inputs. Preserve the first refusal and the exact source revisions.

1. Validate authored CUE before exporting. If CUE rejects a contradiction, inspect the actual inherited constraint. Do not replace a hard value with a permissive default to make it pass.
2. Run catalogctl doctor against -spec with its receiver -policy/-profile, or -bundle. Search by concept with catalogctl search -text when the ID is unknown. Classify the failure: JSON/schema shape, graph/cycle, duplicate identity, undeclared precedence, implicit scalar replacement, stale digest, missing/unused grant, or resource cap. Fix the declaration or receiver policy only within existing authorization.
3. Use catalogctl explain on the last verified bundle for the target ID. Report its source, contributing layers, changes and NodeDigest. If integrity fails, rebuild from trusted inputs; do not recompute the digest on edited output to bless it.
4. Compare verified bundles with catalogctl diff. Include hidden entries and presentation fields. The old Node.Hash contract omits some of these; do not use it as the new bundle identity.
5. For prepare failures, compare the binding node/version and definition_digest with the exact current -definition bytes, then check every required witness. Report missing, empty, extra, duplicate or hash-mismatched evidence separately from model uncertainty. Never silently drop context or truncate a packet to get under a cap.
6. Run the failing case through the real CLI or consumer entrypoint after repair. Report what ran and what remains untested. Keep model quality, source authenticity and consumer adoption separate from deterministic integrity.

Do not weaken a gate, automatically refresh a stale patch, enable a provider or grant a broader override as a diagnostic fix.
