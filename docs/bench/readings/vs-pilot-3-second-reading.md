## Where fathom stands (vs-pilot-3, second reading)

### Read the caveats first

This pilot asked fourteen lookup questions about the fathom repository itself, twice each, with Sonnet, in five setups: fathom wired in ("full"), gopls, graphify, Serena, and plain grep. The ledger says fathom passed 26 of 28, gopls 24, grep 23, graphify 22 and Serena 21, and that fathom cost $1.57 in total against $2.67 for grep and $2.83 to $3.11 for the rest. Four things have to be said before those numbers mean anything.

1. **The competitors were handicapped by the harness, not by their tools.** For gopls, graphify and Serena the bench replaced the repo's CLAUDE.md with a 16-line routing note (the corpus says it leaves their files untouched; the code does not). Two of the fourteen questions have their answer inside CLAUDE.md. That one defect explains all six competitor failures on H1-T08 and two more on H1-T12. Void those eight trials and fathom's lead over gopls disappears; its lead over grep shrinks to nothing.

2. **The competitors never used their tools.** gopls made zero tool calls in 28 trials, graphify one, Serena one (a handshake). Their routing notes told them most of these questions were "grep's job" and the model believed it, even on the two caller/implementer questions where the notes named the right tool. So gopls, graphify and Serena were scored as "Sonnet with grep plus a note", not as gopls, graphify or Serena. Nothing here says fathom beats those tools on their own ground.

3. **fathom's obligations tool never served the rule it was asked for.** In all ten trials on "what governs this file" questions it returned displaced root-tier rules (displaced: 31) instead of the file's rule. fathom still passed most of them, but by search plus a plain file read, the same route everyone else took. The predictions written into the corpus ("answered directly by the obligations tool") were wrong ten times out of ten. That is a product bug to fix, not a bench artifact.

4. **The grader is a substring match and the answer key is in the repo.** Two gopls passes on H1-T13 are wrong answers that happen to quote the right token; one fathom pass on H1-T05 bolds the wrong function and mentions the right one in parentheses; four failures on H1-T02 are right answers that wrote "Edge" instead of "core.Edge". The file the questions were copied from (testdata/contextbench/tasks-home.json) was checked out in every worktree and showed up in tool output in 41 of the 108 trials we could verify. Separately, 32 of the 140 raw transcripts on disk are not the sessions the ledger scored (a later session overwrote the same path), so per-trial transcript claims on H1-T06, T07, T08, T10 and T11 have to be read against the cell records, not the files.

### What is left after the corrections

Voiding the corrupted trials, removing the two false gopls passes and discounting fathom's hedged pass: fathom 23 of 26, gopls 22 of 25, grep 23 of 26, graphify 22 of 26, Serena 21 of 25. Fathom, gopls and grep tie at about 88 percent; graphify and Serena sit a few points behind, mostly because two of their trials ran out of turns before answering. Scoring H1-T02 by substance adds a point to every arm and changes no ordering.

Where fathom genuinely earned something:

- **The verify gate (H1-T04).** fathom answered with one or two verify_plan calls for $0.06. Every other setup had to build fathom's own binary, ingest the repo and run the gate by hand for $0.33 to $0.51, and Serena ran out of turns doing it. Nobody else has this capability. It is the one clean mechanism win in the pilot.
- **Cost.** $1.57 for 26 passes against $2.67 for grep's 23: about six cents per pass against twelve. Fewer detours, fewer turns. Six competitor or baseline trials hit the 20-turn cap with no answer, several with the answer already in hand; none of fathom's did. Part of that is the tools; part is sandbox friction (`find` is `fd`, BSD grep swallows `\|`) that hit the exploring arms harder.
- **Callers and concern search (H1-T05, T06, T09, T10).** fathom's find_callers, find_symbol, query_symbols and search produced the gold in 6 of 8 trials. But grep produced it in 8 of 8, and find_callers merged two same-named RenderMarkdown functions on H1-T05 t2, which is how fathom's own arm gave the wrong headline answer there.

Where fathom did not: obligations, claim provenance and staleness, the categories the product exists for. The obligations tool served displaced rules every time; staleness_report was never called by fathom's own arm; the freshness task (FRESH-M7-01) was answered by every arm with `cat`, so the live-index-versus-precomputed-graph argument was never tested at all.

### Pairs well with, equal to, loses to

- **Pairs well with grep and a file read.** That is what fathom's own arm actually did on most questions, and it is where fathom's routing stanza sends them. fathom's search tool plus Read was the working combination.
- **Equal to gopls and grep** on pass rate after correction; ahead on cost per pass.
- **Ahead of everything on the verify-gate question**, the only place a fathom-exclusive tool decided a row.
- **Not shown better or worse than graphify or Serena on their home turf**, because their tools were never exercised. This pilot cannot say.

### Keep or not

Keep developing, but do not cite this pilot as the reason. The durable result is verify_plan; the cost advantage is real but partly sandbox-shaped; the obligations layer failed on its own bench, which is the most useful thing the run found. Criterion M6-1 (fathom at least equal to grep on passes, at no more tokens per pass) still passes after correction, as a tie on count with a cost edge.

The falsifier that would change this verdict either way: rerun with the competitor notes appended to CLAUDE.md instead of replacing it, the answer-key file excluded from the subject, raw transcripts written to an immutable per-cell path, the grep arm cut off from the live daemon, and the obligations displacement bug fixed. If fathom's obligations, claim-provenance and staleness rows then beat grep by a clear margin, the "what governs this file" thesis has bench support. If they still tie, the product's core claim has no bench evidence and the roadmap should say so.