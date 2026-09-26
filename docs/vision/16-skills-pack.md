# 16 — The skills pack: adoption as one install

> Status: accepted 2026-08-07 — the same owner directive as vision 14 promoted
> it. Plan: `docs/plans/impact-observatory-activation.md` (§G, IO-23..26, all
> proven).
> Drafted for review 2026-08-07 · Non-binding, companion to 14/15.
> Owner directive: a fathom skills pack that works on major code assistants
> (`/fathom:xyz`), turning the rewire/activate prompts into reusable
> skills, "super easy to adopt and keep fathom working great." Grounded in
> a same-day recon of the shipped skill machinery and a dated harness
> radar (Claude Code plugins reference, agentskills.io, Codex/Cursor/
> Gemini first-party docs).

## 0. What already exists (more than expected)

fathom already ships **dual-format skills**: six skills embedded in the
binary (`dist/agents/skills/`, go:embed), each as a Claude `SKILL.md` plus
a Codex `codex-prompt.md` twin, installed by `fathom init
--skills[=claude|codex|all]` with the `fathom:owned` marker guarding
overwrites, one stanza written to both CLAUDE.md and AGENTS.md, and
`doctor --harness=codex` honestly skipping Claude-only checks. The
cross-harness *content* problem is solved. What's missing is
**packaging**: namespaced invocation, one-install adoption, and an update
channel — plus the two newest skills.

One vocabulary guard: the vision corpus's earlier rejection of a "plugin
system" (02 §3, 07 R3) rejected a *Go code-plugin API* for extending
fathom. This document is about a *distribution* plugin — data files in a
harness's packaging format. product-vision.md already wished for exactly
this: "plugin bundle: MCP + hooks + /verify command — adoption as one
install." The radar says every piece of that wish is now real.

## 1. The pack: eight skills, one canonical source

`fathom-plan · verify · onboard · feedback · doctor · impact` plus the two
born as prompts in docs/adoption/rewire-and-activate-prompts.md:
**`fathom-rewire`** (the reconcile pass with doctor's false signals called
out) and **`fathom-activate`** (guards, layer packs, bindings, skills,
loud gates). The prompts doc becomes the skill bodies' source of truth;
the skills carry the same template-version stamp vision 14's tier-0 gives
hooks, so a drifted skill is detectable like a drifted hook.

## 2. Three renderings of the one source

**R1 — the Claude Code plugin (`/fathom:xyz`, the headline).** One plugin
directory: `plugin.json {name: "fathom", version: semver}` + `skills/`
(all eight → `/fathom:rewire`, `/fathom:doctor`, … exactly the owner's
syntax; namespacing is collision-proof by construction) + **`hooks/
hooks.json` carrying the entire hook template** (SessionStart, PostToolUse,
PreToolUse ×3, Stop) + **`.mcp.json` registering the daemon endpoint** +
optionally `bin/` for a bundled shim. Confirmed consequences (radar,
first-party docs, 2026-08):

- A plugin's hooks are wired the moment the plugin is enabled — **per-repo
  settings.json hook stanzas disappear**; the one line a repo needs is
  `enabledPlugins`. The five-wiring-shapes drift class (the fleet audit's
  headline) becomes structurally impossible on Claude Code.
- **Updates are the reconcile channel**: bump `version` in plugin.json,
  and Claude Code's background marketplace check notifies /
  `/reload-plugins` applies. Honest caveat: third-party marketplaces have
  auto-update OFF by default (user toggles once; org-forceable via
  managed settings) — the session-hook tripwire (14 §7 tier-1) covers
  the gap by naming the update when it detects skew.
- Distribution: a git marketplace (`.claude-plugin/marketplace.json` in a
  repo we own; users run `/plugin marketplace add GeoffMillerAZ/<repo>`
  then `/plugin install fathom@<marketplace>` — two commands, once per
  machine). Plugin-hook matchers that target the bundled MCP server must
  use the scoped `mcp__plugin_fathom_*` names — a documented footgun,
  handled in the template, tested in CI.

**R2 — the open-standard rendering (`.agents/skills/`).** Agent Skills is
now an open standard (agentskills.io, stewarded by the Linux-Foundation
AAIF since 2025-12), and **Codex and Gemini CLI both discover the shared
`$HOME/.agents/skills` and `<repo>/.agents/skills` paths natively**;
Cursor reads its own `.cursor/skills/`. `fathom init
--skills=agents` writes the same eight skills, spec-conformant
(name/description frontmatter, dir-name match, progressive disclosure),
to the shared path — one rendering, multiple harnesses, zero per-tool
work. The codex-prompt.md twins stay as the legacy/manual lane until the
shared-path discovery is verified per harness (calibration row, not
assumption). Codex sidecar metadata (`agents/openai.yaml`) is optional
polish, not required.

**R3 — the pinned rendering (today's mechanism).** Per-repo
`.claude/skills/` via `fathom init --skills` remains for air-gap,
no-marketplace, and version-pinned setups. Nothing is deprecated; the
plugin is the paved road, not the only road.

## 3. What init and reconcile become

With R1 live, Claude-Code adoption collapses to: (1) install the plugin
(marketplace add + install, once per machine); (2) `fathom init` scaffolds
only the *repo-owned* parts — `.fathom/` (config, guards, layers,
observations), `.fathomignore`, the CLAUDE.md/AGENTS.md stanza, service
registration; (3) `fleet reconcile` (14 §7) audits the remainder and
retires its own hook-rewriting duties on Claude Code to "is the plugin
enabled and current?". Codex keeps init-based wiring (its hooks system is
real but narrow today — PreToolUse/PostToolUse intercept only Bash calls,
~10 events, maturity-gated; a watch row, not a dependency). The honest
capability table in docs/adoption/codex.md §6 extends to a three-column
matrix: Claude Code (plugin) / open-standard skills path / manual.

> **2026-09-02 note (RP-19).** The "only Bash calls, ~10 events" sentence
> above is superseded by the verified contract: Codex documents **eleven**
> Claude-shaped events and its tool hooks intercept shell, `apply_patch`
> (matchable as `Edit|Write`), MCP tools and local function tools
> (`docs/adoption/codex.md` §6, *Verified against*). The registry carries
> that contract as data (`fathom harness list --json`); the codec that
> drives it is wave 2 of `docs/plans/harness-port.md`. The three-column
> matrix landed as `docs/adoption/other-harnesses.md`, generated by
> `fathom harness matrix`.

## 4. Sequencing (verb-first, law 10)

1. Add `fathom-rewire` + `fathom-activate` to the embedded pack (both
   renderings) — the prompts become skills with no new machinery.
2. `--skills=agents` rendering + per-harness discovery verification
   (Codex, Gemini; Cursor's path as a follow-on).
3. The plugin directory + marketplace repo, generated from the same
   embedded sources by `fathom plugin render` (a build artifact, never
   hand-maintained — a hand-edited plugin is the schema-copy bug class);
   CI validates with `claude plugin validate --strict` and tests the
   scoped-MCP-name footgun.
4. Flip the fleet: enable the plugin in Geoff's repos, delete the
   per-repo hook stanzas the plugin now owns, and let doctor confirm the
   collapse from five wiring shapes to one.
5. Version-skew wiring: plugin version lands in the beats (14 tier-0) so
   instrument/doctor can say "plugin 1.3 behind embedded template 1.5 —
   /plugin update fathom".


## 5. Kit-shipped packs (owner directive, same day)

Obligation/guard packs ship **with the kits whose law they encode** — the
kit's repo carries `fathom-pack/` (a README, a `guard-pack.cue` with
labeled kit-self and consumer groups, and a `layers/<pack>/` compile pack),
versioned with the kit itself. First two authored 2026-08-07: **app-kit**
(the UI doctrine: corpus-first via the appkit MCP, role tokens, the
@source contract, no-Node on path-A surfaces, D13 contribute-back, and
"a 200 is not a rendering") and **agentic-kit** (stdlib-only, layering,
the schema.cue↔types.go shape, determinism seams, TRACKING honesty,
_reference porting, plus consumer discipline: extend-the-kit-not-the-app,
mocks-are-adapters, instruments-get-existence-checks). Consumers install
by provenance-marked copy (guards) + `fathom layers install` (the compile
pack); `/fathom:activate` re-syncs by the provenance markers, so a
consumer lagging its kit's pack is *drift*, detected like hook drift.
Guards lead because they are deterministic and work with obligations
dark; the compile layer deepens them where a daemon has an LLM profile.
The pattern is the catalog seed for 15 §1's pack matrix: every kit that
states law ships it as a pack, and no project starts from scratch.

The end state is the owner's sentence made literal: adoption is one
install, staying current is one notification, and every rewire/activate/
doctor motion is `/fathom:verb` in whichever assistant is open.
