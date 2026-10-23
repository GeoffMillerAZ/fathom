### Fixed

- When the daemon is slow or cannot build its model, `fathom doctor` reports affected rows as unknown with the reason instead of failing your hook wiring. It reads hook wiring and the local call log without waiting on the daemon, and checks guard scopes against disk when the daemon's file list is cut short.
- In a clone or worktree that is not indexed, authored rules point to `fathom init` instead of failing as "never compiled", and doctor no longer recommends a paid compile when it cannot read the daemon's compile state.
