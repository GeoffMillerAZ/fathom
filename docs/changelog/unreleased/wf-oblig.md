### Fixed

- Extract up to two bounded sentences from raw governing sections and ADR Decision blocks. Preserve code spans and identifiers, skip tables and examples, and withhold sections without instructions.
- Apply ADR standing and completed-plan filters to raw and compiled obligations. Tag partly superseded rules and count inactive rules on the existing hook log row.
- Re-find raw citations in live content once per file per call, retain relevance tokens, and report citation freshness.
- Isolate verdict and rule-outcome memos by agent, and suppress user verdict requests in subagents.
- Correct Bash write targets for sed expressions and script files, remote copies, Perl and Ruby in-place edits, nested command-substitution quotes, and argument-taking flag clusters.
