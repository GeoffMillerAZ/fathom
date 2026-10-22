### Fixed
- `fathom init --no-user-writes` preserves user configuration while applying project wiring and writing operational state to the previewed state directory, relocatable with `FATHOM_STATE_DIR` or `XDG_STATE_HOME`. Under `--harness none`, each user-scope flag enables only its requested write. JSON marks skipped user writes separately from dry runs, and manual config guidance appears only for an existing config file.
- Init and reconcile preserve real skills directories containing declared adaptations even without an ownership baseline, naming the skipped file and its diff command. Doctor ignores semver build metadata when matching adaptations, and warns on both older and newer versions.
- Receipt evidence retains both command and forbidden-path file guards. Doctor reports empty gitignored guard scopes as waiting for files.
- Doctor's channel report excludes its own invocation so an empty-history report agrees with the project wiring page.
- The activate skill retires only fathom-generated intent playbooks and explicitly preserves consumer-authored intent (plugin 0.3.19).
