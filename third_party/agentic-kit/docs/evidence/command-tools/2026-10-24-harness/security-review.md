# HC8 security review and dispositions

An independent critic used differential-review/sharp-edges over the new command and
owning specification, then reread root integration changes. Fathom review_diff was
resource-refused; conclusions came from current files, concrete counterexamples and
bounded probes. Final source identities are in [the qualification record](README.md).
No confirmed security blocker remained in the final reread.

| Finding | Evidence and resulting control |
|---|---|
| HC-F1: case aliases bypassed registration placement | Two native counterexamples previously authenticated a duplicate physical project or candidate-owned registration under different case spelling. Ancestor filesystem identity and registered root identity/case-fold checks now hold both. |
| HC-F2: selected root could change after verification | Final root canonical/Stat/SameFile checks now accompany envelope/registration byte-and-identity rereads. ABA and atomicity remain unproven. |
| HC-F3: unknown signed tuner was silently dropped | Every tuner is checked against the selected independent roster; a ghost in an authenticated layer holds rather than quietly using a default. Losing preferences are also validated. |
| HC-F4: CUE encodings/calendar/path shapes diverged | Invalid key/signature/base64/calendar counterexamples originally passed CUE. Closed encoding/UTC calendar constraints and portable lexical absolute-path forms now reject them; Go still owns crypto and native path behavior. |
| HC-F5: unselected registry shape was skipped | Two native cases originally authenticated despite an unknown unselected layer kind or explicitly empty organization ID. Go now validates all declared kind enums and distinguishes optional absence from present-empty with a pointer. |
| HC-F6: monotonic metadata masked wall rollback | Go 1.26.3's local time implementation confirms Before prefers monotonic data. Both clock samples now use Round(0) before the wall comparison. This closes endpoint detection, not persistent antirollback. |
| HC-F7: blanket data placement restricted project adoption | Explicitly registered signed snapshots of any kind may reside in a project. Protected registration/pins/key/target remain authority; candidate removal can deny availability. In-project positives and tamper/wrong-key/pin/symlink negatives cover the boundary. |
| HC-F8: constant failure count impeded diagnosis | Fixed reason codes, truthful expected/loaded/authenticated counts and optional bounded roster ordinal replace the constant count. Late failure tests forbid raw IDs, paths, hashes, keys, hints or successful source prefixes. |

Root integration also added read-free help and newline-inclusive output accounting.
The toy generator exclusively creates a new directory, writes 0600 files with O_EXCL,
and emits public keys, payloads, envelopes and registration. Six synthetic private
keys are used only in memory; the generator is not a production enrollment tool.

Reviewer runtime scope did not include the full gate, actual OS clock mutation,
hardware-backed key/prompt qualification, installation, other native operating systems
or host hook interception. Separate executed evidence and remaining limits appear in
the qualification record. Source verification and a rule's presence do not establish
OS protection, actor authority, approval consent or measured engineering benefit.
