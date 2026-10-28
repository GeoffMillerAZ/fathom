### Fixed
- `blast_radius kind:"rename"` and `test_impact` on a bare name several definitions share answer `ambiguous: true` with the candidate keys and ask for a qualified name, instead of "0 call sites — nothing to rename" or "no tests reach"; `fathom refactor plan` refuses such a target naming its definitions instead of rendering an import plan.
