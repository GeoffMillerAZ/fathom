### Changed
- Readouts read a command past its leading `&&`-joined `cd`, `export`, `mkdir -p`, `sleep N`, `set`, `source` and similar steps and classify it by its last segment, so a rule now matches `cd a && go test ./...` and those steps' own lines stay verbatim ahead of its view; `[readouts] quiet_prefixes` (user-only) names more such steps.
