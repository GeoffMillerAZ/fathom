# Terminal input boundaries and frame readiness

Date: 2026-09-15. Kit-owned G5 hardening; no consumer edits, dependencies,
installed-host changes, commits or releases. G5 remains incomplete.

## Implemented repairs

- `apps/terminal/model.go` now uses Go's complete `unicode.Bidi_Control` property
  instead of a hand-maintained partial list. U+061C/U+200E/U+200F previously survived
  startup queries, paste/key text and metadata output. Startup rejects these queries;
  interactive input removes the controls with notice. Arabic/Hebrew letters, combining
  marks, emoji joiners and Persian non-joiners remain unchanged.
- Only plain `q` quits outside editing. Shift/Alt/Ctrl/Meta/Hyper/Super modifiers
  no longer trigger that action; lock-state bits alone remain compatible. Ctrl+C and
  existing exit codes are unchanged.
- Below40 columns or10 rows, `View.MouseMode` and callbacks suspend capture while
  retaining the user's preference. F2 can change that preference while small; a usable
  resize restores the chosen state. The fallback discloses mouse release and F2.

These close existing documented interaction boundaries, not a new public widget API,
CLI framework migration or broadened platform claim. `cmd/appkit` remains unchanged.

## Model red/green evidence

New independent tests enumerate all12 direction controls across startup, paste and key
text; metadata tests inspect emitted frame bytes, not an ANSI-stripped projection.
Four multilingual examples protect ordinary writing and joined emoji. Separate tests
cover tiny-view capture/preferences and seven modified-q combinations plus five
ordinary/lock-state combinations. The original three omitted controls fail the new
input/metadata tests; the partial-list controls and multilingual examples already pass.
The old small-view test fails at retained capture; Alt/Ctrl/Meta+q each request quit.

Same-oracle Unicode output remains under `/tmp/appkit-terminal-input.tCj405gJ/`:
`model-before.txt` SHA256
`c940fa49914d5426d6908842a79176f07d3a5091a3b86b36c6340d34823ca85f`;
`model-after.txt` records the three new Unicode test groups passing. The pre-fix model
was `ce11166797cc64f8212714ee0a1833786d39b5300a4c98aacc54c03d9f9f1a2c`.
The final model is `c552be77ba1645b757bc9be1cc15a44cffe246502562ad520041ae8567413cd4`;
view is `8569c450234659d25ce9b186e868c443140d19ef6b9f6f3220d3315d210aeb62`.

## Actual built-command PTY evidence

[Repo-owned probe](../../apps/terminal/ptyprobe/README.md), Go1.26.3,
macOS26.6.2 arm64, Python3.14.6. Fresh owned sessions only; NO_COLOR/xterm-256color.
New cases use actual bracketed paste, Escape/Enter/F2, Alt+q/Ctrl+q bytes and resize.
Normal completion includes bounded quit, shutdown protocols and complete termios
equality after canonical input, independently observed by keeper and parent.

Receipts live under system temporary `appkit-terminal-pty-<suffix>/receipt.json`:

| Suffix | Source / result | Receipt SHA-256 |
|---|---|---|
| `s74k4so0` | Old Unicode input: fails exact sanitized filter after7 passes | `c1a5306dc6f1a0ac938eb02a1749fa830d507c7c7953f32ef6d905e697df105c` |
| `5uik8t88` | Unicode-only repair:28 checks pass | `09e5435ab425df0b4a7df5762b4326f0bb6db9c689f0fa4e5b40bfe5859c2c26` |
| `i9j7414t` | Old small viewport: fails mouse-release protocol after5 passes | `de318175ed4f3d6900669102e58b704e39e7641106bfeff28213c5135c74761a` |
| `vp767mzy` | Final command, earlier deadline binary:142 checks pass | `415c5019f873e92f438fb9b14950803c811c25973c5447561e14e53c46318916` |
| `k9vwa0m6` | Final command and rebuilt deadline binary:142 checks pass | `38da8967ab0122840eab78fd874f8455481c716cc22a2ba29caafc2656e63fe3` |
| `er392onk` | Same final binaries; additional PageDown/PageUp input:144 checks pass | `7c881339235ca2a03a683bf0f2471ae7545b9ba29c7c81a71da6432a32f8eba2` |

Both old failed assertions required forced test cleanup; their receipts also retain
`Operation not permitted` cleanup errors. They prove neither normal shutdown nor old
restoration. Subsequent exact-PID checks found neither keeper nor application alive
(95140/95141 and95752/95753). Final seven-case runs have no forced cleanup or cleanup
errors: keyboard/mouse27, Unicode28, small-viewport30, Ctrl+C14, SIGINT14, SIGTERM14,
context-deadline15. No failure was normalized into a pass.

Built artifacts in `/tmp/appkit-terminal-input.tCj405gJ`:

| Artifact | SHA-256 |
|---|---|
| `terminal-before` | `cbb15c621b7f02d00df537a5ad2945e120e7bc9464d69703d0e9a853835b9747` |
| `terminal-after` (Unicode-only) | `04c7c3ead66191f95027976920f7432ae47707578f3afedde6f47c3224653c61` |
| `terminal-final` | `8a26d19424197a8566df8fcf4e239f4a82f3736468ad306facd60d5cac3dea7a` |
| `deadline-final` | `7c1e932a88bf8bab185bc90453436fe4c11aaf0706527144afc0ef8d0317db3f` |

The two142-check receipts bind probe `run.py`
`9f9e14325f9df4f92eedf5be2da63c3d2935916998d1215244dc46c9398d00a3`,
session keeper `d65701580a4cc473a23c5d36ed5a2598d4e8c49aabb67ac6aff2fd26461d64b9`,
and deadline source `f1d403df348e5ec007e4540dac7915ac82d04e914fde21164963b52423eb9d73`.
Earlier Unicode-only runs bind earlier probe `f9b4a26d1c4e4357c83f65af2072920c9dcc0cd978749559618faf562ac05cfe`.

The subsequent `er392onk` seven-case run binds probe
`6d01cf7f5b07d87976e1321c2e4b3723d002d48848d9839a95624b347ed69a99`.
It adds actual xterm PageDown/PageUp bytes at60/61x14: detail rows4-9 advance
to10-15 and return to4-9, each confirmed by a full redraw before the next key.
All144 checks pass, with no forced cleanup or cleanup error; only the keyboard/mouse
case count increases27→29. Production binaries and other probe sources are unchanged.

## Paged navigation regression proof

`apps/terminal/paging_test.go` covers list and detail PageDown/PageUp, top/bottom
clamping, the resized list's visible page size, return selection and catalog immutability.
Detail assertions compare actual content rows to slices from a same-width120x120
complete reference, not only the production position label. SHA256:
`74c8ad39652c1397fbdd67010f30297f7ba0ac701d663432765d774a3afe17d9`.

Three isolated Go overlays under `/tmp/appkit-terminal-paging.fpY0e9` prove detection
with `go test -overlay <name>-overlay.json ./apps/terminal -run '^TestTerminal(List|Detail)PageKeys' -count=1`:
`down` makes PageDown move zero and fails both tests; `up` makes PageUp move zero
and fails both; `frozen-detail` leaves the position label advancing but renders rows
from zero, and fails the detail-content check. These are deliberate negative canaries,
not product defects. The unchanged production model/view pass the final eight-package
race suite, including both new tests. No working production source was mutated.

## Headless frame synchronization

Independent review and the root race suite both reproduce the old3-second timeout
waiting for detail after mouse input. An intermediate typing frame already contains
`interaction.forms`; that was insufficient acknowledgement of the final filtered
frame. A click could therefore hit a stale callback, which the model correctly refuses.
Deferred context cancellation also caused teatest's late `Fail in goroutine after
TestTerminalHeadlessProgramJourney has completed` on the first root failure.

The test now acknowledges an exact completed editing row, then an exact non-editing
row before clicking. A120→121→120 width sequence forces complete redraws at these
boundaries. New oracle canaries reject intermediate matches, editing carets, longer
queries and another cursor row. Deferred graceful quit/join precedes cancellation.
No production stale-frame rule or3-second deadline was relaxed.

Two intermediate harness attempts also failed: a matcher incorrectly required trailing
space, and a naive editing-row wait could not reconstruct insertion patches such as
`ESC[4;26H ... n ESC[4h_`. After a one-sided redraw, the non-editing update could instead
use `CR ESC[3A Filter: forms validation`. Failure excerpts are recorded here; complete
stdout for those intermediate test-only attempts was not saved. The final full-redraw
sequence avoids pretending ANSI stripping is a terminal emulator.

Final review also found that the row matcher accepted `forms validation extra`:
its one-space suffix did not constrain the rest of the row. Two independent negative
fixtures fail before repair and pass after requiring optional spaces followed by
end-of-buffer, newline or erase-to-end. This strengthens the oracle; it does not
change production filtering. Final test sources, SHA256:
`model_test.go` `8d09a8bcae93847c020454e558c9a6b23057ab62c3fa6407e5bf7b4f516090a3`;
`frame_readiness_test.go` `d094a0254431f259924fa45a0ea5bb9eb7655be3c4fd05abc554e4354712493b`.

## Verification and limits

Final local verification uses `GOWORK=off GOPROXY=off GOTOOLCHAIN=local`:

- `go test -race ./apps/terminal/... ./appkit/cli/...`: all eight packages pass.
- `go test -race ./apps/terminal -run 'TestTerminal(HeadlessProgramJourney|FilterFrameReadiness)$' -count=20`: passes with the final matcher and redraw sequence.
- `go test ./cmd/appkit-mcp ./appkit/catalog`: both packages pass.
- `go vet ./apps/terminal/... ./appkit/cli/...`: passes.
- `python3 -m unittest discover -s apps/terminal/ptyprobe -p 'test_*.py'`: all12 oracle/preflight/cleanup tests pass.

No graphical emulator, physical input device,
font/shaping/contrast, clipboard or assistive-technology claim follows from PTY bytes.
There is no new consumer integration, installed-host adoption or Windows evidence.
Temporary receipts are retained local evidence, not permanent backups.

INC-34/35 record the defects and their regression outcomes. Closed INC-9/15 were
compressed to linked summaries; their owning assessments retain details. No accepted
ADR was superseded, future promoted, exit-code contract changed or goal checked off.
