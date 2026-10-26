# Command lab security review

Scope: `cmd/command-lab/**` against `docs/designs/command-governance.md` CG-1..7 and CG-A01..09. Reviewed untracked implementation files, fixtures, tests, and the local `decision` answer contract. This is a new command, so there is no committed command-lab baseline or removed security code to blame. Fathom `review_diff` reported a leaf surface with zero indexed callers; its coverage, integrity, and architecture dimensions were unchecked. The review below is source analysis plus focused offline reproduction, not runtime adoption proof.

**Recommendation: hold CG-A02, CG-A04, and CG-7 acceptance until F1-F4 are fixed.** The command has no action authority, which limits immediate effect, but `audit` currently makes false complete-coverage claims.

## Findings

**F1 - High - Dynamic JS can appear fully covered.** `cmd/command-lab/audit.go:190-209`. A `functions.exec` record whose code is `tools.exec_command({cmd: "git status"}); tools.exec_command({cmd: dynamic})` returns `status:"complete"`, one Git candidate, and zero `unsupported` failures. The regular expression finds one literal and returns without accounting for the second dynamic call. This violates CG-3/CG-A04: mixed wrappers have unknown coverage. Mark the record incomplete whenever the wrapper cannot be fully bounded, even if it contains one literal candidate. A focused CLI reproduction confirmed the output.

**F2 - Medium - Wrong command type becomes a valid proposal.** `cmd/command-lab/audit.go:225-245,211-220`. Normalized JSONL `{"tool":"shell_command","command":42}` produces `status:"complete"`, `proposals.shell:1`, and no malformed/unsupported failure. `getString` discards the decode error and the tool name supplies a fallback family. CG-2/CG-7 require malformed coverage to be incomplete. Require string types for both structured fields before counting. A focused CLI reproduction confirmed this result.

**F3 - Medium - Total intake reads past the stated overflow byte.** `cmd/command-lab/audit.go:68-69,81-92,139-150`. The total 64 MiB check occurs after `readLine` consumes a whole next line. With 16 complete 4 MiB sources followed by a 17th source beginning with a 1 MiB line, the command reads roughly 1 MiB beyond the 64 MiB total before it reports `total_oversize`. CG-2 says never read beyond a cap plus one overflow byte. Bound each source reader by the remaining total budget plus one before calling `readLine`.

**F4 - Medium - CUE-valid capture is rejected by Go.** `cmd/command-lab/schema.cue:15` and `cmd/command-lab/codec.go:127-135`. `#PortAnswer` declares `Score?` and `Noul?`, but `fields` treats those untagged `decision.Answer` fields as required. Removing those two irrelevant score/predicate fields from the supplied choice capture passes `cue vet ... -d '#Capture'`, while `assess --capture` exits 2 with `input_missing`. Align the schema and decoder; for a choice answer, absence of these fields is the natural safe shape. CG-7 requires CUE/Go parity.

**F5 - Low - Empty invocation omits its mandatory safety statement.** `cmd/command-lab/main.go:13-16`. Running with no subcommand returns exit 2 before `emit`, so no `noActionAuthority:true` or fixed diagnostic is printed. CG-1 says output always states no action authority. Emit the same fixed `invalid_subcommand` envelope used by the default branch.

**R1 - Performance - Byte-at-a-time file reads.** `cmd/command-lab/audit.go:139-155` issues approximately one `Read` per byte. A maximum-size source entails about 4 million reads; the 64 MiB total can entail about 67 million. The limit is finite, but a routine audit can be unnecessarily slow. A buffered reader can preserve exact counters and overflow semantics.

## Verification and limits

`TMPDIR=/Volumes/m2ext/tmp/agentic-kit-build go test ./cmd/command-lab` passed. Focused offline CLI inputs reproduced F1, F2, and F4. I inspected fixed-code diagnostics, source ordinal output, independent source/layer set validation, allowed-operation intersection, classifier provider/model/key checks, probability checks, and deterministic blocker precedence. I found no raw source, path, name, or digest emitted by the normal audit report, and no provider, network, credential, installer, hook, or command execution path in this package. The tests do not establish exhaustive privacy or live provider behavior. No source files or hooks were changed for this review.

## 2026-10-23 re-review of fixes

Focused package tests pass. A freshly built offline binary confirmed these dispositions:

- **F1 remains open, narrower.** The original mixed literal/dynamic case now exits 2 with `unsupported:1`. However `const run = tools.exec_command; run(dynamic); tools.exec_command({cmd: "git status"})` exits 0 with `status:"complete"`, one Git candidate and no failures. `audit.go:236` counts only direct `exec_command(` calls and `cmd:` keys, missing the aliased dynamic invocation. Count the literal candidate but mark the wrapper incomplete unless its calls are conservatively bounded.
- **F2 fixed.** `{"tool":"shell_command","command":42}` now exits 2, `malformed:1`, zero proposals.
- **F3 fixed for the total cap.** Sixteen 4 MiB sources plus a 17th source exit 2 at exactly 67,108,865 bytes (64 MiB + one), with `total_oversize:1` and 64 retained records.
- **F4 fixed.** A CUE-valid choice capture without `Score` or `Noul` now exits 0 as advisory with `classifierStatus:"answered"`.
- **F5 fixed.** Empty invocation exits 2 with `invalid_subcommand` and `noActionAuthority:true`.
- **R1 fixed.** `audit.go:82-83,163-175` now uses buffered reads; no byte-at-a-time file loop remains.

**F6 - Medium - New line-cap overread.** `audit.go:82-85,163-170`. A synthetic line of 1 MiB + 100,000 bytes exits 2 for `line_oversize:1`, but reports 1,081,344 bytes read: 32,768 beyond the 1 MiB line cap. The buffered reader prefetches an entire chunk before checking line length, contrary to CG-2's cap-plus-one rule. Bound the next read to the remaining line budget plus one while retaining buffering. No source files were modified in this re-review.

## 2026-10-23 final narrow review

The intervening native command-type and bounded line-reader changes close F1 and F6; see `final-review.md` for their direct proof. I independently reviewed the subsequent Unicode guard in `codec.go:36` and `audit.go:221,238,307-317`. Both control input and outer JSONL are checked before `encoding/json`; Codex string-encoded `arguments` receive a second check after decoding. `TestUnicodeWireBeforeDecode` covers isolated high/low and wrong surrogate pairs, plus valid pairs, escaped backslashes and literal replacement characters. `go test -count=1 ./cmd/command-lab` passed. Fresh binary probes rejected an isolated high surrogate in control input (`input_utf8`, exit 2), normalized JSONL (`malformed:1`, exit 2), and Codex encoded arguments (`malformed:1`, exit 2), with zero proposals in both audit cases. No new finding in these two files and the focused test. The root full gate remains separate.

## 2026-10-23 model reference portability review

Reviewed the three-file diff in `assess.go`, `schema.cue`, and `main_test.go`. Go and CUE now use the same ASCII 1-256 character model reference shape, permitting `/`, `:`, `@`, and `+` in pinned models and captured `Model`. Provider and request IDs retain the 64-character `_id` rule. `checkResponse` still requires exact provider and model equality, and `RequestKey` still binds the policy model. The named test covers three native reference forms, wrong captured identity, malformed/control-character/space/oversize refs, and CUE policy/capture shapes. `cue` was present; `TMPDIR=/Volumes/m2ext/tmp/agentic-kit-build go test -race -count=1 -run '^TestCapturedNativeModelReferences$' ./cmd/command-lab` passed. No new finding or authority relaxation observed. Root full gate and consumer profile 4 remain pending outside this review.
