# Catalog doctor skill trial

Date: 2026-10-20. One independent read-only task using the new catalog-doctor skill,
as prescribed by the skill-creator validation workflow. This is an advisory-skill
usability check, not a production consumer or model-quality study.

The task received only the skill location, a raw two-layer catalog and receiver policy,
the repository/toolchain locations, and a read-only instruction. It was not given the
expected diagnosis. The input renamed an inherited title through an ordinary node
declaration; the policy granted no overrides.

The task validated both inputs against CUE, ran the real `catalogctl resolve` command,
preserved its `implicit scalar replacement of review` refusal and identified the
distinction between dependency precedence and override permission. It recommended an
agreeing assertion under the existing policy or a separately reviewed explicit override.
It did not grant itself fields or modify files. It explicitly reported that no verified
bundle was supplied and downstream explain/diff/preparation behavior was untested.

Input SHA-256 values:

- Spec: `17ca8b13baad051bb0ec97affbb1755d55aedfee2c95a56762d48d1d8cff4a26`.
- Policy: `ca3d163bab055381827226140568f3bef7eaac187cebd76878e0b63e9e442356`.

One trial is insufficient evidence for broad skill reliability. The resolver's
deterministic tests, rather than the trial's prose, establish the refusal behavior.
