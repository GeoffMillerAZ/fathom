### Fixed

- The edit hook no longer labels a file of an indexed repository a `worktree` when the repository is opened through a symlinked path (for example macOS `/var` or `/tmp`). The destination class and clone fold now compare canonical paths on both sides.
