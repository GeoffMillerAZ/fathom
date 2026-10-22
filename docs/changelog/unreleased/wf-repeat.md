### Fixed

- Stop hooks report claims about uncommitted code as unknown, exclude them from refusal counts, and report unchanged failure or unknown sets once per session.
- Claude Code and Codex post-edit and post-bash hooks deliver each governing rule once per context, with a count of rules already shown and a reset on compaction.
