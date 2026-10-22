### Fixed

- Subagent discovery briefs expand omitted ranges through `fathom discoveries --parent-session ID`, with the same ownership and content-hash filters, while keeping the parent-map id within the 1,200-byte primer.
- Subagent discovery briefs now keep only the parent's current, content-checked file ranges, excluding hook echoes, instruction files and shell fragments. Index errors no longer fill the brief.
- Completed Claude Code `symbol_context`, `find_symbol` and `read_source` lookups retain their harness session and agent identity in the discovery log, including plugin tool names, so the parent's lookups remain eligible when the daemon connection lacks those tags.
- Earlier session-end notes no longer close maps filed after the parent resumes.
- Claude Code spawn hooks opt in through `[hooks] subagent_map_facts = true` (default false, `"observe"` logs only). They append up to 1,500 bytes of live exploration-map excerpts to `Agent` and `Task` prompts, framed as data, preserving other input and permission decisions. An existing facts marker skips rewriting, including nested spawns. `fathom map show ID --brief` prints the same facts for orchestrators. Relocated logs require the same `FATHOM_THREADS` setting in the harness environment.
- Plugin 0.3.19 ships the anchored spawn matcher and completed-lookup observer, with a refreshed template pin and reconciled repository hooks.
- Exploration-map hooks read and decode one thread-log snapshot per map load, index session-end positions once, and skip excerpt reads when their byte budget or deadline is spent.
