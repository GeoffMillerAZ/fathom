# Opt-in terminal lifecycle probe

Exercises the actual built terminal catalog in fresh, owned PTYs. Python's standard
library is the only harness dependency; no emulator, package installation, production
data, existing terminal session or global configuration is involved. This is an
optional maintainer check, not a Python dependency for kit consumers or `task check`.

## Build and run

From the kit root, with the pinned Go toolchain and Python 3.10+:

```sh
appkit_pty_build=$(mktemp -d /tmp/appkit-terminal-pty-build.XXXXXX)
go build -o "$appkit_pty_build/appkit-terminal" ./apps/terminal/cmd/terminal
go build -o "$appkit_pty_build/deadline" ./apps/terminal/ptyprobe/deadline
python3 -m unittest discover -s apps/terminal/ptyprobe -p 'test_*.py' -v
python3 apps/terminal/ptyprobe/run.py \
  --binary "$appkit_pty_build/appkit-terminal" \
  --deadline-binary "$appkit_pty_build/deadline"
python3 apps/terminal/ptyprobe/run.py \
  --binary "$appkit_pty_build/appkit-terminal" \
  --deadline-binary "$appkit_pty_build/deadline" --case sigint --repeat 20
```

Use built binaries, not `go run`, for process exit assertions. Both paths must be
explicit, absolute executable files. The runner supports macOS/Linux termios APIs;
the [recorded host proof](../../../docs/assessments/2026-09-13-terminal-reference.md)
is macOS only. Missing prerequisites and failed assertions are not skips/passes.
Normal success is exit 0; test/cleanup failure is 1; invalid invocation is 2.

## What the matrix proves

| Case | Expected observation |
|---|---|
| `keyboard-mouse-resize` | Enter/mouse open the same capability; resize rewraps; wheel scroll persists; PageDown/PageUp move one visible detail page; F2 releases and restores capture; Back preserves selection; q exits 0 |
| `bidi-filter` | Native bracketed paste removes all12 Unicode direction controls with notice; intended detail/Back and multilingual text survive; q exits0 |
| `small-viewport` | Too-small view releases capture, resize/F2 preserve preference, Alt+q/Ctrl+q do not quit, and ordinary q still exits0 |
| `ctrl-c-byte` | Raw Ctrl+C key reaches the model's graceful quit, exit 0 |
| `sigint` | Actual OS interrupt reaches the single signal owner, runner failure exit 2 |
| `sigterm` | Actual OS termination follows the pinned Tea graceful-quit path, exit 0 |
| `context-deadline` | Existing runner returns both deadline and program-killed identities; private fixture emits its success marker and exits 0 |

Each case checks owned session/foreground identity, bounded exit, shutdown-region
alternate-screen/cursor/paste/mouse protocols, and complete termios restoration after
an actual canonical line read. Before/immediate/after-read snapshots preserve every
field and control character; PENDIN is never masked. A session keeper allows an
independent parent snapshot before macOS revokes the controlling PTY. Cleanup never
repairs the program's terminal state. Any forced cleanup is a failed case.

The ordinary matrix has mouse capture on at shutdown, including after demonstrating
F2 opt-out. Disabling it earlier cannot satisfy shutdown proof. Output oracles accept
only exact full rows or the pinned renderer's exact cursor-addressed delta, followed
by a full-redraw confirmation; old frames cannot satisfy later actions.

## Negative canary and retained evidence

This deliberately corrupts **only the newly owned PTY**, after the application exits:

```sh
python3 apps/terminal/ptyprobe/run.py \
  --binary "$appkit_pty_build/appkit-terminal" \
  --deadline-binary "$appkit_pty_build/deadline" \
  --case ctrl-c-byte --inject-echo-drift
```

Expect exit **1** at the exact-restoration assertion. Never count the negative canary
as a production failure or silently accept it as normal success.

Completed test runs leave a fresh temporary evidence directory with `receipt.json`
and bounded raw `.pty` transcripts (1 MiB/case); invalid invocation exits before a
receipt exists. Receipts retain binary/probe-source hashes,
host identity, case/iteration, snapshots, assertion results and failures. Reads have
six-second deadlines; repetition is explicitly bounded to 1–50. Review and retain
needed evidence yourself; the probe neither removes it nor publishes it.

If the host refuses to open a repo script, retain that operational failure. An exact
temporary copy of `run.py`, `session.py`, `test_probe.py` and `deadline/main.go` may be
used for diagnosis: compare bytes/hashes first and run the copy without editing it.
Do not loosen host privacy/global permissions or treat the failed run as successful.

## Limits

Protocol bytes are not graphical emulator, physical mouse, screen-reader, clipboard,
light/dark polarity or consumer integration proof. Multilingual byte preservation is
not bidirectional shaping/font correctness in a graphical emulator. Fixtures use `NO_COLOR=1` and
`TERM=xterm-256color`; no background-color reply is fabricated. The deadline helper's
zero means its cancellation assertions passed, not that a cancelled user operation
succeeded. Distinguish typed Ctrl+C, OS SIGINT and context cancellation. Shared CLI
defaults and two actual consumer integrations remain separate from this reference.

The [2026-09-15 input receipt](../../../docs/assessments/2026-09-15-terminal-input-controls.md)
records144 assertions across all seven cases with no forced cleanup, plus retained
old-binary failures. Run the complete matrix after changing model/input behavior;
a passed Unicode model test does not prove the native paste/event-loop path.
