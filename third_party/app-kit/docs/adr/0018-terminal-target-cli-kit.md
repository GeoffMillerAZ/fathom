# ADR-0018: The terminal is a kit target — ship cli-kit on cobra + charm v2

- **Status:** Accepted
- **Date:** 2026-08-21
- **Decision drivers:** the owner directive to build one excellent CLI kit and
  dogfood it hard (2026-08-21, for env-kit's `bosun`); ADR-0017's own Context,
  which already named a terminal consumer ("fathom-studio… could consume
  nothing but a palette") and left the terminal outside the layer table; the
  cost of every fleet CLI reinventing help, theming, config, and exit-code
  conventions separately; the Charm v2 GA wave (Feb–Mar 2026) making a
  coherent, stable terminal stack pinnable for the first time.

## Context

The kit is multi-target and layered (ADR-0017): L0 CUE tokens flow outward to
CSS, browser JS, and Go renderers, and a consumer takes any prefix and stops.
Every target so far renders to the web. Meanwhile the fleet keeps growing
terminal programs — `appkit` itself, `appkit-mcp`, platform-engineering's
`plat` and now env-kit's `bosun` — and each one re-decides help text, version
plumbing, color handling, config loading, and what exit code 2 means. The
platform side has a hardened convention (0 ok / 1 checked-and-broken / 2
could-not-check, curated refusals, `--json` everywhere, doctor tables) with no
shared implementation.

External facts, dated 2026-08-21: Charm shipped GA v2 of its stack under
`charm.land/*` import paths (bubbletea/v2 v2.0.9, lipgloss/v2 v2.0.6 — now
"pure", no implicit terminal IO, explicit color profiles — bubbles/v2 v2.2.0,
fang/v2 v2.0.1, huh/v2 v2.0.3); fang provides styled help/errors, automatic
`--version`, `completion`, and a hidden `man` command over spf13/cobra
v1.10.2, with a fully injectable `ColorScheme` via `WithColorSchemeFunc`;
`colorprofile` v0.4.3 implements NO_COLOR/CLICOLOR/CLICOLOR_FORCE detection;
teatest/v2 (untagged x/exp, pseudo-version pinned) gives deterministic TUI
golden tests. cobra + pflag were already in this module's graph as indirect
dependencies of the CUE tool. charmbracelet/crush (v0.90.0) was studied as a
pattern reference for CLI craft — it is FSL-1.1 licensed, so it informed the
design and contributed no code.

## Decision

The terminal is a first-class kit target, and `appkit/cli/…` is its
realization — cli-kit. It consumes L0 exactly as the web targets do:

- **`appkit/cli/theme`** is the terminal token bridge. A generated palette
  (`palette_gen.go`, emitted by the same generation pipeline that emits
  `theme.css`, byte-identity-gated by `task check`) carries all 23
  `--ak-color-*` roles for light and dark as sRGB values converted from the
  CUE source's OKLCH. From it: lipgloss styles and a fang `ColorScheme`, so a
  kit CLI's help screen and a kit web page draw from one token source.
  Terminal light/dark follows the terminal background via
  `lipgloss.LightDarkFunc` — the terminal's analogue of `[data-theme]`.
- **`appkit/cli/command`** wraps cobra + fang with house defaults: styled
  help/errors, automatic `--version` (build info), `completion`, `man`,
  signal-aware context, and the typed exit-code convention (0 ok / 1
  checked-and-broken / 2 could-not-check) adopted fleet-wide from the
  doctor/fathom lineage.
- **`appkit/cli/output`** owns output discipline: color-profile detection
  (NO_COLOR et al. via colorprofile, never reimplemented), human/JSON/NDJSON
  modes, and the status-table renderer whose plain rendering is
  byte-compatible with the existing doctor-table format — styled output is
  progressive enhancement over the same geometry, so goldens and pipes see
  identical bytes.
- **`appkit/cli/config`** is the CUE-config pattern for CLIs: schema-validated
  layered config (schema defaults → system → XDG user → project file, with
  the project walk bounded at the git worktree root → explicit overrides),
  per-layer digests, and an `Explain` that names which layer set a field.
  CUE's refusal semantics are kept deliberately: a layer conflict is an
  error, not last-writer-wins.
- **`appkit/cli/tui`** is a thin bubbletea/v2 runner (context-tied, mouse
  opt-in via cell motion, explicit color profile) plus a teatest harness for
  deterministic goldens. It is a doorway, not a framework.

Boundaries: cli-kit adds five direct dependencies (cobra, fang/v2,
lipgloss/v2, bubbletea/v2, colorprofile) to the single root module (D13);
bubbles, huh, glamour, and log join only when a consumer needs them. The
kit's own `cmd/appkit` stays hand-rolled stdlib-flag for now — migrating it
is real work with its own diff, not a rider on this decision. The Loopback
Invariant (D5) governs kit *apps*; a CLI binary is not an app shell and is
out of D5's scope. First consumer: env-kit's `bosun`
(platform-engineering), which migrates onto all five packages and proves the
byte-compatibility claim against its existing goldens; the promotion bar for
the terminal target is therefore one consumer plus goldens, matching
ADR-0017 §5's low-layer bar, with the two-consumer bar (D13-style) applying
to any future opinionated component above these five.

## Alternatives considered

- **Keep hand-rolling per binary** (the current `cmd/appkit` and old `bosun`
  pattern). Attractive: zero dependencies, full control, proven. Lost
  because the fleet now has four terminal programs and a fifth convention
  would be written this quarter; help/completions/man/theming are exactly
  the undifferentiated chrome a kit exists to own; and the platform's exit
  code and curated-refusal conventions deserve one implementation with
  tests instead of four copies.
- **cobra alone, no charm.** Attractive: smallest dependency step, cobra is
  already in the graph. Lost because unstyled help throws away the token
  system's reach, and every consumer would still hand-roll color detection,
  tables, and TUI plumbing — the parts that actually drift.
- **urfave/cli or stdlib-flag conventions codified in docs.** Attractive:
  lighter than cobra. Lost because fang targets cobra (the styled chrome
  would be ours to build), the ecosystem gravity — including the owner's
  stated preference — is cobra, and docs-only conventions are wishes
  (nothing enforces them).
- **charm v1 (github.com/charmbracelet/*) instead of v2.** Attractive: older,
  more tutorials. Lost because v2 is the GA line (lipgloss v2's pure
  rendering makes deterministic tests dramatically simpler), mixing v1/v2
  in one binary double-loads rendering stacks, and pinning the legacy line
  in a new kit buys a migration for nothing.
- **Costs of the winner, honestly:** the charm graph lands in the single
  module's go.sum for every consumer (build pruning keeps it out of non-CLI
  builds, but the lockfile grows); charm v2 is six months old and moves
  fast (pins are exact; teatest is an untagged pseudo-version — churn
  expected); and a second CLI style now exists in-repo until `cmd/appkit`
  migrates, which is admitted drift with a named revisit below.

## Consequences

- Easier: every fleet CLI gets styled help, completions, manpages, one theme
  (the kit's actual tokens), one exit-code convention, one config pattern
  with provenance, and deterministic TUI tests — by importing, not copying.
  The terminal joins the web as a place the design system reaches.
- Harder: five new direct deps to watch (revisit trigger: any of them
  loses maintenance or breaks compatibility twice in a quarter);
  cross-target token changes now affect terminal output (the palette is
  generation-gated, so drift is loud); consumers outside this repo need
  the module resolvable (revisit trigger: the moment a second external
  consumer appears, cutting v0.1.0 per ADR-0017 §4 stops being optional).
- Risk accepted: fang's ColorScheme surface is young; if its API shifts,
  the theme bridge absorbs it in one file (trigger: a fang minor breaks
  `WithColorSchemeFunc` — pin until the bridge is updated, never fork).

## Revisit when

- `cmd/appkit` or `appkit-mcp` wants richer CLI UX — that is the trigger to
  migrate the kit's own binaries onto cli-kit and end the two-style drift.
- A consumer needs forms (huh), markdown rendering (glamour), or structured
  logging (log/v2) — each joins as a deliberate pin with a one-line note
  here, not as a drive-by.
- bubbles/v2 stabilizes past v2.2.x and a consumer needs an interactive
  component — the static status table stays; interactive tables come from
  bubbles then.
- Charm relicenses any pinned library away from MIT, or teatest/v2 gets a
  real tag (swap the pseudo-version pin the same week).
- A third fleet CLI adopts cli-kit — cut the app-kit release tag (v0.1.0)
  so consumers can drop `replace` directives (ADR-0017 §4 prerequisite).
