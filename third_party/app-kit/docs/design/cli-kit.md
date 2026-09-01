# cli-kit — building a kit CLI (the consumer guide)

Decision: [ADR-0018](../adr/0018-terminal-target-cli-kit.md) (D18). This
file is the how-to for a CLI that wants the kit's terminal chrome. Reference
consumer: env-kit's `bosun` (platform-engineering, `kits/env-kit`) — read it
when a snippet here isn't enough.

## The five packages, and when you reach for each

| Package | You want | You get |
|---|---|---|
| `appkit/cli/command` | a cobra root that behaves like a kit CLI | fang-styled help/errors, `--version`, `completion`, `man`, signal-aware ctx, the 0/1/2 exit convention |
| `appkit/cli/theme` | colors that match every other kit surface | the generated 23-role palette (light+dark), `Styles(isDark)`, `FangColorScheme()` |
| `appkit/cli/output` | correct output in pipes, CI, and terminals | color-profile detection (NO_COLOR et al.), `RenderStatusTable`, `WriteNDJSON` |
| `appkit/cli/config` | config that explains itself | layered CUE (schema→system→XDG user→project→overrides), per-layer digests, `Explain` |
| `appkit/cli/tui` | an interactive screen | a bubbletea v2 runner with house defaults + a deterministic teatest harness |

## Minimal CLI

```go
root := command.New(command.App{
	Name:    "mycli",
	Short:   "one line, ends with a period.",
	Version: version, Commit: commit,
})
root.AddCommand(newDoctorCmd(), newRunCmd())
os.Exit(command.Execute(ctx, root)) // fang chrome + theme applied here
```

That alone buys styled `--help`, `--version`, `completion <shell>`, a
hidden `man` command, and Ctrl-C flowing through the context.

## Exit codes are a contract, not a convention

`0` ok · `1` checked-and-broken · `2` could-not-check (or usage). Return
`command.Exit(2, err)` when a check could not run; never conflate it with
broken — agents and CI branch on the difference. (Lineage: fathom / plat
doctor; see `command/exit.go`.)

## Diagnostic tables (the doctor shape)

```go
rows := []output.StatusRow{{ID: "git", Status: "ok", Detail: "found"}}
output.RenderStatusTable(w, rows, output.Detect(w, os.Environ()))
```

The plain rendering (Ascii/NoTTY) is **byte-stable and gate-worthy** —
bosun's migration kept its goldens unchanged through it. Styled rendering
colors only the status word and remedies over identical geometry, so your
goldens force a plain profile and never see ANSI. `--json` surfaces use
`output.WriteNDJSON` with the same rows.

## Config that answers "where did this value come from"

```go
res, err := config.Load("mycli", schemaCUE, config.Options{Host: config.NewOSHost(), Overrides: fromFlags})
res.Explain(os.Stdout, "someField")
```

Layers unify lowest→highest; a conflict is a refusal (CUE semantics — no
silent last-writer-wins). `Explain` lists **absent** layers too, teaching a
fresh machine where config could live. Ship a `config explain` subcommand;
your users will thank you exactly once, at the worst possible moment, which
is the point.

## Theming rules

- Never hand-pick hex in a kit CLI — take `theme.Styles(isDark)` or map
  roles. The palette is generated from the same CUE tokens as `theme.css`;
  `task check` refuses drift.
- fang's chrome is themed by `FangColorScheme()` automatically through
  `command.Execute`; the role mapping is fixed on purpose (a second
  terminal palette is the drift the bridge exists to prevent).
- Respect the profile: anything you print goes through `output` or lipgloss
  writers — raw ANSI by hand defeats NO_COLOR and pipes.

## TUI, when you actually need one

`tui.Run(ctx, model, tui.WithAltScreen(), tui.WithMouse())` — mouse is
opt-in cell-motion; the runner ties program lifetime to the context. Test
with `tui.NewTest` (80x24, Ascii, deterministic). Reach for a TUI only when
a table + flags genuinely can't carry the interaction; bosun ships none yet
and is better for it.

## What NOT to do

- Don't import bubbles/huh/glamour/log "while you're at it" — each joins
  the kit only on a consumer's real need (ADR-0018 boundary).
- Don't copy code from crush (FSL-licensed) — it is a pattern reference
  only; the patterns worth stealing are already distilled here.
- Don't build a second status-table/config/exit-code implementation — if
  the kit's shape doesn't fit, that's a kit issue to raise, not a local
  fork (D17's divergence rule applies: declare it, with why).
