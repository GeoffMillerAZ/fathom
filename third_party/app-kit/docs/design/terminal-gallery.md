# Terminal catalog reference

`apps/terminal` is an experimental, read-only Charm application under
[G5](../intent/scope.md#g5--advance-the-existing-terminal-target) and
[ADR-0018](../adr/0018-terminal-target-cli-kit.md). It answers a bounded task:
find a kit capability, inspect its ownership/evidence requirements, and return without
losing the selection. It is an example, not a new shared widget framework.

## Run and controls

From the kit root:

```sh
go run ./apps/terminal/cmd/terminal
go run ./apps/terminal/cmd/terminal --mouse --query 'forms validation'
go run ./apps/terminal/cmd/terminal --json --profile cli
```

Build the command for process exit checks; `go run` reports failed child exits itself.
Interactive mode requires terminal input **and** output. Pipes receive an exit-2
refusal unless `--json` is selected. Help/version do not start an interactive program.

| Task | Keyboard | Mouse, when capture is on |
|---|---|---|
| Select and scroll list | arrows, Home/End, PgUp/PgDn | row selects; wheel moves three rows |
| Inspect exact selected ID | Enter | `[Open]` on that row |
| Scroll detail | arrows, Home/End, PgUp/PgDn | wheel |
| Back, preserving filter/selection | Esc | `[Esc Back]` |
| Edit text filter | `/`; Enter/Esc/Tab finishes; Ctrl+U clears | `[/ Find]`, then type; `[Clear]` |
| Release/re-enable capture | F2 | `[F2 Mouse on/off]` while captured |
| Quit | q outside editing; Ctrl+C anywhere | `[Quit]` |

Mouse defaults off so selection/copy stays with the terminal. Capture changes live;
use F2 to re-enable it while mouse events belong to the terminal. No hover-only actions.
Rows have explicit selection markers; color is supplemental. The design inherits the
terminal background, uses kit role styles after background-polarity detection, and
respects the selected output color profile. It never guesses a separate canvas theme.

## Boundaries

- Reads the same CUE-validated first-party capability IDs, contracts and revision as
  Gallery/MCP. It does **not** browse purchased sources or conversion attempts;
  the web Gallery and existing source tools retain that separate coverage.
- Each search returns at most 50 entries. If more match, the UI says to narrow the
  filter. `--profile` selects capability compatibility, not the host running this TUI.
- A detail is inert metadata: usage, paths and proof commands are never executed.
  Private files, apps and customer data are not opened. The model has no network work.
- Query input is at most 256 UTF-8 bytes; paste scanning is capped at 4096 bytes.
  Newline/tab become spaces, control characters and the complete Unicode `Bidi_Control`
  property are removed (not Arabic/Hebrew letters, combining marks or join controls), and a
  visible notice reports sanitization/truncation. Backspace removes one grapheme;
  a long filter scrolls horizontally to keep its insertion marker visible.
- Layout is bounded to 300 columns/120 rows. Below 40 columns/10 rows it asks for a
  resize and suspends mouse capture; F2 can change the saved preference, which a usable
  resize restores. Clipped controls have no hit region; clipped revision text has an ellipsis,
  while detail and JSON retain the complete digest.
- A mouse callback belongs to its rendered frame; stale callbacks after filter,
  scroll, resize or capture changes cannot activate a different item.

The model owns `View.MouseMode` and `View.AltScreen`. Do not pass `tui.WithMouse`
when the application needs a live opt-out: that runner option deliberately forces
capture on each view. Use the existing runner/config/output packages where relevant;
this reference needs no CUE user configuration or additional Charm dependencies.

## Machine contract

`--json` writes one newline-terminated object: `schema_version: 1`, `outcome`,
`exit_code`, and either `page` (the shared catalog search response) or `error`.
It does not probe the terminal, read input or start the runner. No matches is a
successful observation (`0`); invalid query/profile, cancellation before inspection,
or write failure is `2`. Root flag/argument parse errors can precede JSON dispatch
and use stderr with exit `2`; callers must always check the process exit code.
Graceful interactive quit returns `0`; runner failure/cancellation returns `2`.
Only plain `q` quits outside editing; application/window modifiers do not. Lock-state
bits alone do not change the meaning of an otherwise plain key. Ctrl+C remains available.
The reference disables Fang's signal subscription so Bubble Tea alone owns signals:
typed Ctrl+C is a graceful key quit (`0`), OS SIGINT is interruption (`2`), and the
pinned Tea SIGTERM path quits gracefully (`0`). Parent context cancellation still
works. Do not compose competing signal owners; INC-17 records the observed race.
The existing `cmd/appkit` commands/output contracts are unchanged.

## Evidence and remaining work

`go test -race ./apps/terminal/...` covers shared identity, fixed forms lookup,
keyboard/mouse action parity, stale callbacks, scroll/resize, Unicode/input bounds,
inert metadata, machine output and failures. The real headless Bubble Tea loop
receives key/paste/mouse/resize events; a separate runner test checks cancellation.
These tests do not claim terminal-emulator rendering or terminal-mode restoration.

The opt-in [repo-owned PTY probe](../../apps/terminal/ptyprobe/README.md) checks actual
keyboard/mouse/resize, repeated signals, deadlines and full restoration, with a failing
ECHO-drift canary. See the [dated assessment](../assessments/2026-09-13-terminal-reference.md)
for observed receipts and limits. The [G5 acceptance matrix](../assessments/2026-09-16-terminal-acceptance-matrix.md)
combines current-code PTY/machine checks and the two scoped consumer integrations.
It does not justify permanent widgets, another TUI stack, installed-binary delivery
or broader terminal accessibility/emulator certification.

[Input hardening follow-up](../assessments/2026-09-15-terminal-input-controls.md) retains
old failures for omitted direction controls and undisclosed small-window capture;
seven final PTY journeys pass144 assertions, including PageDown/PageUp. Frame-readiness canaries and graceful
test-loop cleanup address a separately reproduced headless-test timing failure.
