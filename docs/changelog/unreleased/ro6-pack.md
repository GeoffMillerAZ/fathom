### Added

- Tech packs can carry add-only readout rules in a `readouts` key. A pinned pack's rule extracts records or adds lines under `[readouts] mode = rich` once its own fixtures pass; a rule with no fixture, a failing one or a stage that drops lines is withheld and named by doctor's readouts row.
