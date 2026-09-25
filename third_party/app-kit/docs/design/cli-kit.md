# cli-kit — building a kit CLI (the consumer guide)

Decision: [ADR-0018](../adr/0018-terminal-target-cli-kit.md) (D18). This
file is the how-to for a CLI that wants the kit's terminal chrome. Reference
consumer: env-kit's `bosun` (platform-engineering, `kits/env-kit`) — read it
when a snippet here isn't enough.

Before duplicating a terminal workaround or handing off material CLI work, use the
[contribution loop](contribution-loop.md). Return reusable input/output, config and
interaction regressions with synthetic evidence; operational commands and domain
policy stay in the app. Contribution and consumer upgrade are separate reviewed steps.

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

Ordinary unclassified errors still map to **1**. Default `Execute` does not classify
invocation errors. The historical [Bosun/Surveyor probe](../assessments/2026-09-14-cli-consumer-errors.md)
retains its four usage exit1 failures; diagnostic restoration alone did not fix them.
The owner-authorized [opt-in migration](../adr/0070-opt-in-cli-invocation-classification.md)
adds a separate selected policy without changing unselected consumers.
The [retained runtime probe](../assessments/fixtures/cli-consumers/README.md) now executes
private copies and reports diagnostic/exit-policy dimensions separately; nine synthetic
tests pass. Its [receipt](../assessments/2026-09-16-cli-retained-probe.md) is not a new
Bosun/Surveyor run or permission to execute either consumer.

`command.ExplicitCode(err)` returns `(code, true)` only when an `Exit` wrapper exists;
unlike `Code(err)`, it distinguishes explicit 1 (or 0) from a default. Ordered joins
retain the existing first `errors.As` match. A classifier must honor that policy first.
ADR-0070 retains ADR-0048/0052's default-off origin wrappers and nearest explicit
ancestor selection. `ExecuteWithClassifier` invokes a selected classifier once for
a non-nil error without an explicit exit; nil/declined classification uses `Code`.
`InvocationOriginCode` returns2 only for complete recognized invocation-origin
compositions, never message matches. Mixed or unknown errors remain unclassified.

Only a consumer that selects the verified patched Cobra/pflag snapshots may use:

```go
root.SetInvocationErrorOrigins(true) // requires the explicitly selected patched Cobra
os.Exit(command.ExecuteWithClassifier(ctx, root, command.InvocationOriginCode,
	fang.WithErrorHandler(command.UnreportedErrorHandler(nil))))
```

The root kit dependency graph remains upstream-compatible; the setter does not exist
there. Each opted-in consumer must explicitly select both generated local dependency
copies; Go does not inherit app-kit's replacements. The [delivery tool](../assessments/fixtures/cli-consumers/cobra-origin/README.md)
verifies their full inventories and module wiring. Do not hand-edit copied sources,
silently fall back when the setter is missing, or migrate another CLI implicitly.
Changing unknown-command/flag exit1 to2 is a breaking consumer policy change.

## Curated errors without losing unexpected diagnostics

When a leaf has already printed its curated diagnostic, explicitly acknowledge it:

```go
// Only after the diagnostic was actually reported:
return command.Reported(command.Exit(code, err))
// At the process entrypoint, opt in instead of suppressing every Fang error:
os.Exit(command.Execute(ctx, root,
	fang.WithErrorHandler(command.UnreportedErrorHandler(nil))))
```

Nil selects Fang's default rendering for unreported errors; pass a handler to retain
your own rendering policy. Marking preserves `errors.Is`/`As` and `Code`. A mixed
join containing any unreported error renders intact, even if that repeats an already
reported child. A blanket no-op handler still overrides kit defaults; do not use it
to solve duplicate output. [ADR-0039](../adr/0039-explicit-cli-error-reporting.md) keeps
this opt-in reporting helper separate from usage classification. Bosun and Surveyor
now use it in their entrypoint/leaf adapters; this does not prove installed-host delivery.

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
a table + flags genuinely can't carry the interaction; bosun ships none yet.

The experimental [terminal catalog](terminal-gallery.md) is a bounded interactive
reference with keyboard/mouse inspection, filtering and inert catalog detail. Its
model sets `View.MouseMode` itself so F2 can release capture live; **omit** the
runner's `WithMouse` option when that opt-out is required. A headless test is not
proof of terminal restoration or two consumer integrations; the guide links their
separate evidence and the bounded G5 acceptance, without broader host certification.

Give an interactive command **one signal owner**. The terminal reference calls
`command.Execute(ctx, root, fang.WithNotifySignal())` so Bubble Tea handles SIGINT/
SIGTERM; the empty Fang override disables its competing context cancellation, not
Tea's handler or explicit parent-context cancellation. Shared non-TUI defaults remain
unchanged. INC-17 records a real pinned-dependency shutdown race; repeat the
[owned-PTY probe](../../apps/terminal/ptyprobe/README.md), not just a typed Ctrl+C test.

## What NOT to do

- Don't import bubbles/huh/glamour/log "while you're at it" — each joins
  the kit only on a consumer's real need (ADR-0018 boundary).
- Don't copy code from crush (FSL-licensed) — it is a pattern reference
  only; the patterns worth stealing are already distilled here.
- Don't build a second status-table/config/exit-code implementation — if
  the kit's shape doesn't fit, that's a kit issue to raise, not a local
  fork (D17's divergence rule applies: declare it, with why).
