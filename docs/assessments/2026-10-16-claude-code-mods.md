# Claude Code mods: what fathom should draw inside Claude Code

Owner-architect assessment, 2026-10-16, summarized into the repository the same day. Read at fathom `d2765564`, on
Claude Code 2.1.302 installed locally, against the plugin typings Claude Code 2.1.301 writes beside a mod and the
local changelog. The research behind it was four parallel passes (Claude Code's mod surfaces, its interface
surfaces, the plugin ecosystem, and fathom's own code); their notes sit in gitignored workspace files and are not the
record. The design is `docs/design/claude-code-mods.md`; this file is the reading that led to it.

## Verdict

Yes, in two steps, and only as a display. Fathom already tells the agent the rules for an edit, how far the edit
reaches, whether the index is behind and whether a claimed plan row is proven; the person sees none of it. Mods let
a plugin draw those facts inside Claude Code at no token cost, because they can read what fathom's hooks already
produced. Build the Go and markdown parts first (an install check, a scout subagent, a status line, a committed
marketplace manifest), then one small TypeScript module in a separate optional plugin. Never let a mod answer,
approve, rewrite or block anything: that would cross four non-goals and corrupt fathom's own measurements.

## What was read, graded

| Claim | Grade | Source |
|---|---|---|
| Mods are plugin hooks modules run in process, in an environment with no DOM and no Node | measured (read from the typings) | typings lines 14-20; changelog 2.1.287 |
| A mod can redraw a tool's result block on every surface, draw a band (terminal and desktop) and open a pane | measured | typings 9771, 10362-10369, 10673 |
| A mod's classic hook sees the settings hooks' `additionalContext` | measured (the type); whether a plugin's command hooks are among them is unmeasured | typings 1264-1327 |
| A failing or slow hook is skipped, so mods fail open | measured | typings 4052, 5243-5244 |
| Organizations can skip a mod's hooks | measured | changelog 2.1.299 |
| A managed-mods-only setting exists by that name | unmeasured locally | the research only |
| Claude Code already shows hook context to the person | unmeasured | MOD-M3 reads it |
| The status-line and HUD space is saturated | measured (public star counts on two projects, about 28,600 and 13,300, read by the research) | the research |
| No direct competitor draws code-intelligence evidence in a mod | hypothesis (negative search results) | the research |
| Evidence rows save the person a lookup | hypothesis | none yet; MOD-M3 tests the premise |
| The bare-identifier share of greps is 13.5% of 29,643, an upper bound on what the index could answer | measured (ADR-071's corpus reading) | `docs/design/readouts.md` §1 |

What this assessment does not claim: any token saving, any time saving, or that a mod's behaviour holds beyond
2.1.301's typings, which say the surface may change without notice.

## Corrections made while recording

The research called the Stop-time notice an `InfoNotice`; the typings define that as the line under the logo, so the
proof line uses `$.ui.log`. It said `/statusz` lacks per-service freshness; the JSON carries each service's indexed
commit, and only HEAD and the distance are missing. It rejected output summaries outright; readouts already plan an
outline view behind a measured contract, so the rejection is narrowed to a second engine inside a mod. It could not
find its 44% shell-read figure; F75 records it.

## Decisions it drove

ADR-072 (decisions 1 to 12, with Q1 to Q6 answered by the owner-architect under the owner's delegation), the
`No Node toolchain` carve-out and a note under `No grep-blocking` (`docs/intent/scope/non-goals.md`), ADR-029's
amendment, F2 promoted in part, F75 and F78 notes, goal G40, question Q36 (Q-MOD-1, Q-MOD-2), roadmap §9.7, and plan
`docs/plans/claude-code-mods.md` (MOD-).
