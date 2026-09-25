# ADR-0071: Deliver the static reading reference as plain files

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-16
- **Extends:** ADR-0023 and ADR-0069

## Context

The static-content starter currently proves Go/HTTP reading and recovery. It does
not yet deliver files usable without Gallery or Go at runtime. This fires ADR-0069's
missing-starter-task trigger. The owner wants static sites to benefit from the same
kit without inheriting a server or client framework.

## Decision

Add an experimental, fixed export of the existing first-party reading/FAQ composition
and profile article. Generate system/light/dark variants and matching 404 documents,
one explicitly supplied compiled CSS file, and a deterministic byte manifest.
No licensed source or font is included. Go runs at build time, not for the visitor.

Use a small export-only shell and typed destination table. Ordinary page links/assets
are relative and preserve the selected page/scheme; fragments target actual focusable
headings. The static guide's onward links go to relevant article sections, not absent
Gallery catalog/quality routes. Omit the Gallery state form and global navigation;
never export an inert control that appears to require a working backend.

Build all bounded documents before creating a fresh absolute output directory.
Existing targets and symlinks refuse; files are exclusively created within the owned
directory. Do not overwrite or delete user files, even after partial I/O failure.
Retain such failed output for diagnosis. No timestamps or private source paths enter
the manifest; repeated identical inputs produce identical bytes.

Record a canonical hosting base path (default `/`). Error documents use that prefix
for assets and recovery/theme links: a host may serve the same 404 body at any missing
path depth, where relative URLs would resolve incorrectly. Re-export for a different
mount prefix; never rely on a base element or script to infer it at runtime.

CSS is an explicit trusted compiled input, not arbitrary input claimed to have been
sanitized. The documented build uses the kit's existing pinned compiler and selected
source scan. Bound its size, record exact bytes and verify the actual asset closure.
No inline script, inline style, base element, form endpoint or client router is needed.
The generated meta CSP denies scripts and nonlocal styles. It does not replace a
host's HTTP security, cache, MIME or 404-status configuration.

The fixed exporter lives with the Gallery example, not as a general public site
generator. A separate build-only command leaves the Gallery server CLI unchanged.
This is additive experimental availability, not permanent component promotion.

## Proof and limits

Require deterministic repeat exports, exact file/hash/link/fragment closure,
input/existing-target refusal and unchanged Gallery regression checks. Exercise the
actual files under a non-root static-host prefix with no JavaScript: reading, outline,
FAQ, article/back, theme/page preservation and unknown-path recovery. Freeze binaries,
CSS, harness/config and declared budgets before acquisition; retain failures.

A static host must return an appropriate 404 status for an unknown path; directly
opening a 404 document is not proof of that behavior. Consumer upgrades, public
hosting, full accessibility and all core quality gates remain separately proven.

## Alternatives rejected

- Export the whole Gallery: leaks server assumptions and expands the bounded task.
- Rewrite arbitrary rendered HTML: needs a general URL/control transformation policy.
- Add a CMS, universal generator or hydration framework: no present task needs it.
- Duplicate the reading content/template: loses the reuse and correction path.

## Revisit when

- Two real consumers need configurable content/routes beyond the fixed example.
- A host's path/404/CSP behavior cannot preserve the reading and recovery task.
- A measured payload, interaction or accessibility requirement fails this profile.
