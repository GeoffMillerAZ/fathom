### Fixed

- Subagent discovery briefs now keep only the parent's current, content-checked file ranges, excluding hook echoes, instruction files and shell fragments. Index errors no longer fill the brief.
- Completed Claude Code `symbol_context`, `find_symbol` and `read_source` lookups retain their harness session and agent identity in the discovery log, including plugin tool names, so the parent's lookups remain eligible when the daemon connection lacks those tags.
- Earlier session-end notes no longer close maps filed after the parent resumes.
- Claude Code spawn hooks append up to 1,500 bytes of live exploration-map excerpts and why lines to `Agent` and `Task` prompts, preserving other input and permission decisions. `fathom map show ID --brief` prints the same facts for orchestrators. Relocated logs require the same `FATHOM_THREADS` setting in the harness environment.
