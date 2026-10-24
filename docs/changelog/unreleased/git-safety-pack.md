### Added

- Default git-safety guards block bypassing push or commit hooks, force-pushing or deleting remote main/master, and force or mirror pushes without a named target branch, with a remedy. Disabled Lefthook and hook-path overrides count too.
- Unreadable commands, `xargs git`, unknown force targets, git alias configuration and `--config-env` warn without blocking. Local git operations, push dry runs and forced tags-only pushes remain allowed.
- Set `[guards] git_safety = false` to opt out; the nearest project setting wins. The enforcement switch makes denials advisory, with the nearest project `[guards] enforce` setting deciding for the pack.
