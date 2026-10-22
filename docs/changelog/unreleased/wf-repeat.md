### Changed

- Stop hooks report claims about uncommitted code as unknown, exclude them from refusal counts, and report unchanged failure or unknown sets once per session.
- Obligations are shown once per session per rule on Claude Code and Codex; `[hooks] dedupe = false` restores full injections.
