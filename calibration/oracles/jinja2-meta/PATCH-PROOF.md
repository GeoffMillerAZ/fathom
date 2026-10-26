# Jinja source patch proof

After committing the Jinja lock, patch, grammar blob and these proof entrypoints,
run `python3 calibration/oracles/jinja2-meta/prove_patch.py` from the repository
root. The two pinned subject checkouts must be present under `workspace/calibration/`.
The script requires the lock's build tools (`zig`, `wasm-opt`, `npx`) and the
production builder's tools (`jq`, `curl`, `git`, `go`) on PATH. It installs none.

The script archives committed HEAD into `workspace/tmp/`, uses the unmodified
production `build.sh` for both variants, and changes only the snapshot Jinja
lock row between builds. The baseline clears its patch list and records its
new blob digest and sizes so the embedded runtime can verify it. The patched
build must reproduce the committed Jinja lock bytes exactly. Both runs scan
`.j2`, `.jinja` and `.jinja2` through `tsengine.NewParser` and an
`[(ERROR) (MISSING)] @error` query. Subject HEADs must match the corpus pins, and each measured subtree must be clean
with no untracked or ignored files.
The proof fails on any new parse error, any engine fault, or a changed subject
file/content digest. Its artifact, `workspace/artifacts/jinja-patch-proof.json`,
contains aggregate counts and hashes only. File-level error sets exist only in
the temporary ignored workspace directory while the comparison runs.

Run `go run ./calibration/oracles/jinja2-meta/stress` separately to exercise
the production extractor with 2 MiB text, 256 nested blocks, 57,344 malformed
opener bytes, then a valid recovery case. It emits JSON resource readings and
fails unless the malformed case causes one contained fault and recovery works.

The corpus digest hashes each sorted shard-relative path, a NUL, its raw
SHA-256 digest, and a NUL. Error/fault/repaired set digests hash each sorted
shard-relative path followed by NUL. The committed receipt contains no subject
file list or content. Detailed sets never leave temporary `workspace/`.
Inherited cache/temp environment variables are preserved; otherwise local
`workspace/` defaults are used. Apply the lane's load/nice/one-heavy-job limits.


## Recorded review-fix receipts

`proof/parse-gate.json` and `proof/stress.json` are fresh outputs at source
`85b73ee11`. `proof/full-oracle-run{1,2,3}.json` are the three unchanged
full-oracle harness outputs from the assessment's two subject commands.
Run 1 is the median by pooled CPU throughput, and is byte-identical to
`docs/bench/readings/lang/jinja2.json`. No subject file list or content is
committed. Explanatory disclosures are in the assessment, not appended notes.
