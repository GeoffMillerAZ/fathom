### `map` — exploration maps (ADR-056, experimental)

```sh
fathom map new      ID --purpose "the question it answers" [--when "…"] [--completes-when …] [--agent ID]
fathom map append   ID --anchor <symbol|path#Lx-Ly> --why "…" [--agent ID] [--service S] [--store DB]   # = thread note --map ID …
fathom map name     ID --purpose "the question it answers" [--as NAME] [--when "…"]
fathom map include  ID BASE… [--drop BASE…]
fathom map compose  ID… [--agent ID] [--id-only | --json]    # the ONE id a handoff carries
fathom map show     ID [--budget 1200] [--json] [--store S] [--service X]   # re-resolves every anchor now, within the budget
fathom map list     [--all] [--json]
```

A map is anchored entries plus `includes` references to other maps, stored as
rows in the threads log (no new table; the thread verbs never serve them). An
entry is its anchor, the span it resolves to, that span's content hash
(`core.HashLines`, the layer claims' contract) and a one-line why; `--agent`
records the appending sub-agent's id, and an append never edits an existing
entry. `new` creates a map with its purpose line (required, one line of at most
200 bytes: the question the map answers) and refuses an id that exists; the first
`append` to an unknown id creates the map without one. A new map ends with the
`completes_when` given, else with the end of the current session. `include`
refuses a cycle and a chain deeper than 4. `compose` mints a composite made only
of references (one id comes back as itself), expands it once and prints its
counts, or with `--id-only` prints just the id so a brief can carry it.
A symbol anchor resolves the way `find_symbol` does (XM-31): the first tier that
matches decides, exact (`Name`, `dir.Name`, the full key), a bare method name,
`package.Name`, then case-insensitive, so `ResolveImportSpecs`,
`fusion.ResolveImportSpecs` and `internal/fusion.ResolveImportSpecs` reach the same
symbol and the entry stores the one canonical `dir.Name` key. A name that means
several symbols at its tier lists them (`dir.Name (file:line)`, up to 8) and files
nothing, not even the map; a name no tier matches says so. The service is
`--service`, else the one the daemon names for this repository; a daemon that does
not answer is waited for up to 8 s (a deploy leaves it warming) and then named as
unreachable, with the retry and the two ways round it: `--store` a scratch ingest
with `--service`, or a `path#Lx-Ly` anchor, which needs no index. A store is
tried once. `map show` and `map compose` take the same `--service` for the
read-time re-resolution.
An anchor the map already carries (the same symbol or span in the same content
state) files no second entry (XM-33): the same why files nothing and says so, and
another why is filed as a note on the first entry, with its agent id, which `show`
serves as another annotation; the first entry is never edited.
`name` names a map and refuses one without a purpose line. A named map is
durable whatever condition it was filed under (XM-32, O143): the session-end
default or a `--completes-when` no longer ends it, whether the name comes before
that condition held or after (naming a map its session already ended brings it
back). `list` shows each map's lifetime: pinned while an alive map
includes it, permanent when named, ended once its condition held on an unnamed
map, else open with the time its idle clock started (a dropped include restarts it).

`show` is the read: it expands the map's include DAG once (a base reached by two
routes is one set of entries), then re-resolves **every** entry against the code
as it is now, at read time, never from a cache. An entry is `fresh` (its span
still hashes to what was filed), `moved from <span>` (it drifted and its symbol
key found the same text at a new span), or `STALE +A/-R lines since filing` (the
text changed or is gone; served flagged with its diff size, never as if it were
still true). When a drifted symbol entry needs the index and it cannot be reached
(no daemon, no `--store`), the read says so beside the stale entry. Conflicting
why lines for one entry are all kept, the nearest include's first, each with its
map and agent. Output is deterministic, nearest entries first, and never exceeds
`--budget` bytes (default 1200, the size of a handoff; `0` lifts the cap): the
header and the counts over the whole map always lead, and what does not fit is
named in one `cut N of M entries` line with the stale ones marked. `--json` gives
the same facts as data (the budget still picks which entries are shown; the JSON
itself is not capped).

**Passing one map id to a sub-agent (the orchestrator's recipe).** The parent
builds the map, hands the sub-agent one id, and the sub-agent reads it at the
start and files what it finds:

```sh
# parent, once per run
fathom map new  run-42 --purpose "Where does token validation happen?"
fathom map append run-42 --anchor auth.Validate --why "the entry point"
ID=$(fathom map compose run-42 prior-run --id-only)    # several maps become one id; one map is itself
# … the brief carries: "Exploration map $ID: run `fathom map show $ID --budget 1200` before reading files."

# sub-agent
fathom map show $ID --budget 1200                      # what is already known, each entry re-checked now
fathom map new  sub-7 --purpose "…" --agent "$AGENT_ID"
fathom map append sub-7 --anchor path/x.go#L10-L40 --why "…" --agent "$AGENT_ID"
CHILD=$(fathom map compose $ID sub-7 --agent "$AGENT_ID" --id-only)   # pass CHILD down; a composite refuses entries
```

The sub-agent never edits an entry of the parent's: it appends to its own map and
composes. `--agent` is optional (provenance is then the session), but Claude Code
does not pass the agent id to a sub-agent's shell: it is in the SubagentStart
payload only, which is why the SubagentStart line (XM-8, below) names it to the
sub-agent.

**Surface.** The CLI and the hooks below, no MCP tool. The default MCP surface
stays at its eight tools (`TestDefaultSurfaceUnchangedByMap`); a map tool, when
one is built, lands on the extended surface and never on the default one.

**How maps arrive (XM-8, XM-11 to XM-13).** Nothing here needs an agent to
look: four hooks the harness already fires carry a map to whoever needs it, each
deterministic with provider none, silent when it has nothing true to say, and
free (no daemon call) for a repository whose threads log holds no map.

- **Handoff.** `SubagentStart` puts one map line at the head of the parent's
  discoveries inside RC-28's 1,200-byte cap: the one map the parent session
  filed, or, when it filed several, a minted composite of references that ends
  with the session. The line gives `fathom map show <id> --budget 1200` and
  the sub-agent's own id for `--agent`, which the payload alone carries (Claude
  Code exports no agent id to a sub-agent's shell). The brief yields bytes
  before the id does. The hook input carries no prompt (XM-V1), so the choice
  is the parent's state, never the task.
- **Relevance by location.** When a prompt names, or an edit touches, code
  inside an alive map's own anchored entries (a file, a directory, or a symbol),
  the prompt or edit hook adds one line per map, at most two per firing: the
  purpose, the entry count, how many entries changed since filed (a file re-hash,
  no index and no git), and the expand command. Once per session per map, kept
  as a marker under `$TMPDIR/fathom-map-pointer` that `fathom gc` prunes; a
  firing with no session id says nothing.
- **Session brief.** `thread brief` (startup and clear only) names at most three
  alive maps, one bullet each, ranked by being named by an open note of the
  thread, then by overlap with the thread's recent files (the working tree's
  changed files and the file pointers its open notes carry), then by recent use,
  and counts the rest (`N more: fathom map list`).
- **Threads.** `thread note "…" --map-ref ID` references maps (each must exist);
  the pre-compact pointer carries the ids of the alive maps the session filed or
  added to (`active maps: …`); `thread brief` and `thread handoff` show a note's
  references as `[maps …]`, so a session that resumes the thread finds them.

#### Facts in a worker brief

`fathom map show ID --brief` prints at most 1,500 bytes of current anchored
excerpts and their why lines, newest entries first. `--budget N` can lower that
cap. Changed or missing ranges contribute nothing. This mode uses local files
and the threads log, with no index lookup.

Claude Code's `PreToolUse` hook for `Agent` and older `Task` calls appends the
same block to the spawn prompt from live maps filed in the parent session.
Every other tool-input field and the permission decision stay unchanged. No live
facts, an unsupported harness, malformed input or a hook error leaves the input
untouched and logs a skip reason. A session-end note closes only unnamed maps
filed before that note in the same session, so a resumed parent can file new maps.

An orchestrator that relocates its threads log must set `FATHOM_THREADS` to the
same location in the harness environment as in its map-writing commands. Passing
it only to a shell command does not configure the hooks. Orchestrators writing
their own prompts can paste `map show ID --brief` output directly.

The SubagentStart discovery section includes only the parent's lookups since its
recorded session start, with a recorded range hash that still matches the local
file. Hook and console calls, native instruction files and unverified legacy
pointers are excluded. MCP rows whose caller cannot prove parent agent identity
are also omitted. Claude Code's completed-tool hook records `symbol_context`,
`find_symbol` and `read_source` results in the existing discovery log with the
harness session and agent identity, including plugin tool names. Source answers
retain hashes of the returned lines. Locator answers snapshot the returned local
span at completion. This preserves the parent's lookups even when its MCP
connection sends no identity tags. Other harnesses without this capture must
supply tagged evidence themselves. With no qualifying range the section is absent.
