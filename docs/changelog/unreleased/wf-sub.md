### Fixed

- Subagent discovery briefs now keep only the parent's current, content-checked file ranges, excluding hook echoes, instruction files and shell fragments. Index errors no longer fill the brief.
- Earlier session-end notes no longer close maps filed after the parent resumes.
- Claude Code spawn hooks append up to 1,500 bytes of live exploration-map excerpts and why lines to `Agent` and `Task` prompts, preserving other input and permission decisions. `fathom map show ID --brief` prints the same facts for orchestrators. Relocated logs require the same `FATHOM_THREADS` setting in the harness environment.
