### Added

- `docs/use-cases/library-agent-cookbook.md` (provisional) gives six agent recipes on `fathom-lang`: outline a file, which symbol holds a line or range, which symbols a diff touches, file dependencies, boundary check and ad hoc SQL over `extract -walk -rows`, each with what unknown means for that job.
- `examples/lang/agenttools/tools.json` (provisional) holds function-calling tool definitions for those jobs: name, description, input schema, command template, output schema and exit codes. It is data for your own runner, not a server.
