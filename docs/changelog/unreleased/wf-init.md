### Fixed
- `fathom init --no-user-writes` previews user configuration, ignore, prompt and PATH shim writes while applying project wiring. `--harness none` implies it unless a user-scope flag is requested.
- Declared `fathom:adapted` skill copies are preserved by init and reconcile. Doctor passes adaptations from the current shipped plugin version and warns with a diff command for other versions.
- The activate skill retires only fathom-generated intent playbooks and explicitly preserves consumer-authored intent.
