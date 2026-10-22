### Fixed

- Serve raw obligations only when their bodies yield up to two bounded instructions or ADR decision lines. Withhold heading-only sections, inactive ADR records, archived plans and completed plan rows.
- Re-find raw citations in the live document, report their freshness, and record instruction-free sections withheld on the existing edit-hook log row.
- Isolate verdict and rule-outcome memos by agent, and suppress user verdict requests in subagents.
- Correct Bash write targets for sed expressions and script files, remote copies, Perl and Ruby in-place edits, and nested command-substitution quotes.
