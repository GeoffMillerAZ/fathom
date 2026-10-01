# Conflict diet 2: the four append-heavy hotspots (EW-32)

Free reading: byte counts from `wc -c` on the tree before and after the split, no model spend.
"Per full read" is what an agent reads to take in the whole document; "per edit" is the size of
the one file a writer touches when it adds an entry, a verb or a tool.

| Hotspot | Conflicts | Before (one file) | After: entry point | After: full read (index plus parts) | Parts | Median part | Largest part |
|---|---|---|---|---|---|---|---|
| `.fathom/observations.md` | 70 | 605,907 B | legacy log 605,907 B, frozen, plus the directory | 605,907 B plus one file per new entry | one per entry | 1,386 B (O290 1,641 B, O291 1,130 B) | n/a |
| `docs/usage.md` | 65 | 248,523 B | 27,569 B (index) | 254,960 B | 69 verb files | 1,916 B | 15,860 B |
| `docs/mcp-tools.md` | 32 | 169,237 B | 1,649 B (index) | 170,259 B | 11 group files | 3,556 B | 85,214 B (`shared.md`) |
| `docs/bench/README.md` | n/a | 416,239 B | 61,668 B (README) | 416,385 B | `scenario-runs.md` 354,717 B | n/a | 354,717 B |

What changed for a writer: a new observation is a new file (`O<n>.md`, exclusive create, the next
id minted over the legacy log and the directory together); a new verb section edits one
`docs/usage/<verb>.md`; a new tool row edits one `docs/mcp-tools/<group>.md`; a scenario run
re-renders `docs/bench/scenario-runs.md`. A full read is not smaller (it is 0.04 to 2.6 percent
larger by the index and stub lines), but the entry point an agent opens first is: 27.6 KB against
248.5 KB for usage, 1.6 KB against 169.2 KB for the tool reference, 61.7 KB against 416.2 KB for
the bench README.

Honest limits: `docs/mcp-tools/shared.md` is still 85 KB because the Shared group is one table
of long rows and the registry test reads the table by group; a further split by tool family is a
follow-up if it conflicts. The legacy observations log is unchanged (605,907 B) because the lc
wave and the compile item still edit it; the 70-conflict surface drops only as writers move to
the directory.
