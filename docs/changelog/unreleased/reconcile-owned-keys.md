### Fixed

- Reconcile changes only keys it owns and adopts exact legacy fathom hooks once, even after init or other wiring updates. Commands with different arguments, shell steps, async settings or extra fields stay unchanged and are reported as collisions without counting as drift.
- Dry runs name pending hook adoptions, and the observatory reports completed adoptions. Ownership ledger saves sync a complete replacement before swapping it into place.
