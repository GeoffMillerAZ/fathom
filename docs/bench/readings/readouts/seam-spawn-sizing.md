RO-M16 verdict: STOP. Briefing a workflow subagent with the definitions its prompt names would rarely save the subagent any discovery. Of the 1,534 workflow subagents in 30 days, 152 (9.9%) open with three code-discovery calls (Read, Grep, Glob, or a shell search or source read), against a 30% bar. Seam H9 (the spawn brief) is re-rated Low. Its build row RO-45, its hook facts RO-V2 and its seam reading RO-M17 are deferred. The mods form of the same idea, MOD-X1, keeps its own deferred trigger, which is this bar.

# RO-M16: H9's sizing, MOD-X1's trigger reading

Taken 2026-10-18 with `fathom readout replay --days 30 --json` on the deployed binary (v0.15.6 at 518af7f6), over
every project's local Claude Code transcripts on the owner's machine. It prints aggregates only and is not committed.
The window runs from 2026-09-18T17:30Z to 2026-10-18T15:30Z. A subagent belongs to the window by its first
timestamp.

| Subagents | Count | Discovery-first | Share | Fewer than three calls (counted apart) |
|---|---:|---:|---:|---:|
| Workflow (`subagents/workflows/<run>/agent-*.jsonl`): the bar reads these | 1,534 | 152 | 9.9% (bar: at least 30%) | 30 |
| Other subagents (`subagents/agent-*.jsonl`) | 144 | 14 | 9.7% | 1 |

What the definition counts:
- Discovery-first means each of the first three tool calls is a Read, Grep or Glob, or a shell search or source read
  under RO-29's classification.
- Calls to fathom's own MCP tools are not discovery under that classification. Wave briefs in this repository send
  workers to `find_symbol` and `symbol_context` first, so the figure measures shell and file discovery, as the row
  defines it.
- A brief that also replaced MCP lookups would need its own sizing under a new bar, fixed before any run (K19). This
  reading moves no bar.
