# Terminal reference evidence

Observed 2026-09-13 on macOS 26.6.2 (25G83), arm64, Go 1.26.3. This assessment
supports the bounded [terminal catalog](../design/terminal-gallery.md), not full
[G5](../intent/scope.md#g5--advance-the-existing-terminal-target) completion.
No consumer files, terminal preferences, global config or dependencies were changed.

## Deterministic and process checks

`go test -race ./apps/terminal/... ./appkit/catalog ./cmd/appkit-mcp` checks the
example and shared discovery boundaries. The model tests exercise the actual
headless Bubble Tea loop as well as direct action/state assertions; they are not
screen captures. The command tests keep JSON independent of terminal input and
the runner, validate query/profile/parse errors, and check output failure/cancellation.

An actual built `appkit-terminal` binary additionally produced:

| Invocation | Observation |
|---|---|
| `--json --profile cli` through pipes | one JSON report, exit 0; five shared CLI capability IDs |
| `--json --profile invalid` | error JSON on stdout, diagnostic on stderr, exit 2 |
| no flags through pipes | explicit `--json` remedy, exit 2; no interactive start |
| `--help` | help, exit 0; no interactive start |

Existing `cmd/appkit` output contracts are not replaced. Catalog metadata changes
revise the shared digest; this example adds no second ID/revision authority.

## Actual PTY interaction

The command ran in a tool-allocated macOS PTY, initially 80 columns × 24 rows.
Injected SGR mouse press/release on the second row's visible `[Open]` opened
`analytics.comparison`, matching that row's exact ID. Resizing the owned PTY to
60 × 14 and notifying the owned child with SIGWINCH rewrapped the same detail.
A wheel-down event moved detail from lines 1–6 to 4–9. F2 disabled capture and
emitted mouse-mode disable sequences. Esc returned with
`analytics.comparison` still selected. `q` returned process exit 0.

A second run started with `--mouse --query 'forms validation'`; it selected
`interaction.forms`, quit with exit 0, and restored canonical input/echo. A
Ctrl+C key run also quit with exit 0. In a separate run, SIGTERM to the owned
child was handled as graceful Bubble Tea shutdown (exit 0); this is not evidence
of context cancellation or a promised exit code for every signal/host race.

Captured output showed alternate-screen exit, cursor show, bracketed-paste disable
and mouse disable on shutdown. These are **emitted protocol bytes**, not proof
that every emulator or screen reader interpreted them correctly.

A final rebuilt-binary run at 41 × 12 kept the end of a long CJK filter and its
insertion marker visible. Bracketed paste containing a newline became `hello world`
with a visible sanitization notice. New regressions first reproduced both gaps,
then passed across every width from 40–80 columns, including wide-grapheme cuts.
Quit returned 0; the subsequent canonical-line read restored exact terminal state.

## Context cancellation and restoration

A separate temporary Go harness loaded the same catalog/model with mouse enabled,
then ran the existing `tui.Run` with actual PTY stdin/stdout and a two-second
`context.WithTimeout`. The returned error matched **both**
`context.DeadlineExceeded` and `tea.ErrProgramKilled`; cleanup sequences were
emitted and normal input worked afterward. The harness exit 0 meant its expected
error assertions passed; it was not the interactive command reporting success.

For both graceful quit and context deadline, compare `stty -g` before/after in
the **same** PTY. The immediate result differed only by the local-flag bit
`0x20000000`: `lflag=5cb` became `lflag=200005cb`. This was not silently masked.

Two independent controls isolated the difference:

- Saving `stty -g`, toggling only echo, then restoring the saved value retained
  exact equality. Saving, setting `stty raw -echo`, and restoring reproduced the
  same one-bit difference **without app-kit or Bubble Tea**.
- A fresh PTY `tcsetattr` control reproduced raw→saved-state behavior. The local
  macOS SDK `sys/termios.h` identifies that bit as `PENDIN` (pending-input state).
  Reading one synthetic canonical line cleared it.

After typing `restoration-check` and reading it through the wrapper's normal
line input, the complete `stty -g` string matched the pre-run value exactly for
graceful quit, SIGTERM shutdown, and context deadline. No fields were excluded.
Pinned x/term restores the original termios structure; no kit workaround was added.

## Reproducible PTY lifecycle follow-up

The [repo-owned probe](../../apps/terminal/ptyprobe/README.md) now replaces the
temporary deadline harness and manual wrapper for repeatable lifecycle evidence.
It uses fresh controlling sessions, bounded transcripts/waits, exact shutdown-region
protocols and all-field termios equality after a canonical read, independently observed
by keeper and parent. It does not normalize PENDIN or repair the application's state.
Twelve Python oracle/preflight/cleanup tests, Go deadline-identity negative cases,
the full Go suite, `go vet ./...` and focused terminal/CLI race tests pass.

An actual OS SIGINT intermittently hung the old command for the six-second deadline
(`appkit-terminal-pty-2eonpyu8`); a later old-binary replay passed (`h48dtn59`). This is
not a deterministic red/green claim. Pinned Fang v2.0.1 cancels the command context
while Tea v2.0.9 independently sends InterruptMsg to an unbuffered event-loop channel;
shutdown waits for that sender. INC-17 makes Tea the terminal reference's sole signal
owner with the existing empty Fang option. Shared non-TUI command defaults stay intact.

Final macOS/Python 3.14.6 observations, using identical rebuilt binaries:

| Run | Result | Receipt directory suffix |
|---|---|---|
| Five lifecycle cases | 84 assertions pass; no forced cleanup | `atwc8mop` |
| 20 fresh OS-SIGINT cases | 280 assertions pass; every app exit 2, full restoration | `perancx3` |
| Deliberately toggle owned PTY ECHO after app exit | exit 1 at exact-restoration check; no forced cleanup | `deu1q9iv` |

Receipts and raw transcripts: `/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/appkit-terminal-pty-<suffix>/`.
SHA-256 identities (each receipt also hashes its raw transcripts and probe sources):

- Normal receipt: `cffb27d90bb3c7b75b90dfb295be9c21d1dcec48c455515f5173e1f25bbca0e5`.
- Repeated-signal receipt: `aefda06e748eb5e8a167448142f51c43b7327636fb2775872b35e716659f91ab`.
- Negative canary receipt: `24f00730ab1475eac34f6dcf38cbd886a396c0f6c10d9f38c5a8be220c3180f8`.
- Command: `229183c8080c76266b508e1850615cdac9b2ba5ef2b437101e112023f7a21eb1`; deadline fixture: `02fed662e6af331ae41d9e12f85338aa113762e07fb7d4e88310b4e71ec5685b`.
- Probe `run.py`: `372a0fbced93600a4944a75d7799f76d692b83baba87b66132dfc8f04e131681`; `session.py`: `d65701580a4cc473a23c5d36ed5a2598d4e8c49aabb67ac6aff2fd26461d64b9`.

Original binaries remain under `/tmp/appkit-terminal-lifecycle.CL0Pds`; final command
is `appkit-terminal-single-owner`. A repo-path replay (`dfgtzjqw`) failed two launches
with Python `Operation not permitted` before the app started. Final runs used the
byte-identical `/tmp/appkit-pty-probe.TxdaMu` script copy; no host permissions changed.
Earlier harness failures (old-frame/delta oracle, revoked PTY, pending-output drain)
remain failed observations, not product proof or omitted passes. Twenty successful
signals are scoped regression evidence, not statistical reliability certification.

A fresh isolated MCP process also returned exact updated `cli-kit` (5219 bytes,
`cd9f1b3750a93764cad779d106c7af38edfc4976c79123dfb797d97b40909360`) and
`terminal-gallery` (5430 bytes, `4a15c0031163e33391f27014c1da04a8caf5a0dc625ca405a77a6d38740878bd`)
through tool/resource/guidance routes. It does not prove installed hosts reloaded them.

## Limits and disposition

This is actual PTY input/lifecycle evidence plus headless tests, **not** a visual
Ghostty/Herdr/Windows Terminal matrix, light/dark contrast review, clipboard or
assistive-technology certification. No screenshot/physical mouse-device claim is
made. A terminal background response was not supplied by this PTY, so the initial
uncolored fallback—not theme-polarity correctness—was exercised here.

Two CLI consumer integrations remain unperformed. The reference is an admitted
experiment, not justification for permanent widgets or additional Charm packages.
Future consumer work must repeat the relevant host interaction/restoration checks;
it cannot inherit this dated PTY observation as blanket platform certification.
