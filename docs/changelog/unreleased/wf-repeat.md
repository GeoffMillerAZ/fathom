### Changed

- Stop and claim hooks report absent-index criteria as unknown only when the contract path has uncommitted files, preserving the original criterion and adding a freshness note. Other failures remain failures; proven failures keep blocking every Stop with `--block`, while advisory sets are reported once per change.
- Obligations are shown per rule and context on Claude Code and Codex, with immediate delivery when rendered text, tags or staleness markers change. Capacity evicts the oldest rule entries; suppressed counts include distinct rules with correct pluralization. `[hooks] dedupe = false` restores full injections.
