### Fixed
- Resolve Go field deletion targets through symbol lookup name matching, including bare names and field/method collisions, and disclose that field references are not indexed.
- Withhold the zero-caller proof for a Go field in `gone: callers` rows (Unknown, naming that field references are not indexed) and in `fathom export receipt --kind callers` (refused with the same reason).
