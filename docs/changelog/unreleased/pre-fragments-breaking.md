### Breaking

- `fathom intent render --playbook` is removed after its 0.15.0 deprecation: `fathom intent render` now exits 1 as an unknown `intent` subcommand (ADR-074, IV-10). Migration: drop any call of it from scripts and gate steps; `docs/intent/README.md` is the router.
