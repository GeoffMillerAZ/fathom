### Added
- `fathom config audit` names every project-pack orientation anchor (a path or `path#Symbol`) that no longer resolves, and exits 1 on one.
- The custodian's shipped packs are validated as tech packs against the new `schema/techpack` `#Pack` before they load; `fathom config audit` output is unchanged.
