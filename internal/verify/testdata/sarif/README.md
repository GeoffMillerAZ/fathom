# Vendored SARIF 2.1.0 JSON schema

`sarif-schema-2.1.0.json` is the OASIS Static Analysis Results Interchange Format (SARIF) Version 2.1.0 JSON
schema, errata 01, byte for byte as published. Tests validate every SARIF log fathom writes against it, with
no network (EVX-2, `docs/plans/portable-receipts.md`), through CUE's JSON Schema decoder (`cuelang.org/go`,
already a module dependency).

- Source: https://docs.oasis-open.org/sarif/sarif/v2.1.0/errata01/os/schemas/sarif-schema-2.1.0.json
- Retrieved: 2026-10-10
- sha256: `c3b4bb2d6093897483348925aaa73af03b3e3f4bd4ca38cef26dcb4212a2682e`
- Specification: SARIF Version 2.1.0 Plus Errata 01, OASIS Standard incorporating Approved Errata, 28 August
  2023. Copyright (c) OASIS Open 2023. All Rights Reserved.

Licence: an OASIS work product, distributed under the OASIS IPR Policy's copyright terms. OASIS permits
copies and derivative works that implement the specification to be made and distributed without restriction,
provided its copyright notice and that permission paragraph are kept with every copy; this file is that
notice for the schema here. The schema itself is not modified: replace it only with a newer published OASIS
copy, and update the source line and digest above in the same change.
