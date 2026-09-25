# CLI invocation provenance: isolated candidate, not adoption

G5 remains incomplete. [ADR-0048](../adr/0048-prove-cli-invocation-origins-before-adoption.md)
extends the reporting decision without changing root dependencies or consumer exits.
The only production API addition is `command.ExplicitCode`, preserving existing
`errors.As` selection semantics, including explicit zero/one and ordered joins.

## Current default-off candidate — ADR-0052

[ADR-0052](../adr/0052-default-off-cli-invocation-origins.md) responds to the four
compatibility failures below. The private Cobra candidate now requires explicit
`SetInvocationErrorOrigins(true)`; the nearest explicit command/ancestor setting wins.
Default errors retain concrete types and pointer identity. Explicit false overrides
parent true; true overrides false. Reparented/generated commands use current ancestry.
Opting in preserves raw callback input, diagnostic text and causes, but wraps selected
returned errors. It does not change exits or ship an automatic classifier.

Retained root: `/Volumes/m2ext/appkit-cli-optin.WWGmDq`. Preparations `candidate-a`,
`candidate-b` and a post-test `candidate-c` are byte-identical; the latter revalidates
both original cached source sums after upstream/oracle execution. Each candidate has
156 manifest files, all rehashed. Receipt SHA256:
`4af4f21d8ee27f4bd403df24136825ac72edf995207e0c1f7b29aab942dbc7c5`.
Cobra candidate sum: `h1:CTdeZp7SmSi17fS21nFYL0V1l61I6gdSdRdhDRLlqmE=` (4 of67
files changed); pflag candidate sum and its 3 of89 changed files match the historical
candidate below. Root pins, consumer dependencies and global settings are unchanged.
`verification.json` in that retained root binds source/log hashes, exact inventories
and counts; SHA256 `1d78e2b9393ff951308e6b8d43069323e710134310956abc84d9aebb4e850421`.

| Executed race suite | Result |
|---|---|
| Same final tagged oracle, unchanged baseline | 50 passing /76 failing leaf tests; zero skips; exit1 |
| Same final tagged oracle, opted-in candidate | 132 passing leaf tests; zero failures/skips; exit0 |
| Unchanged kit checks, baseline and default-off candidate | nine packages each; 250 passing leaf tests each; zero failures; seven unrelated licensed-asset tests skip in each |
| Full upstream Cobra/pflag package suites, candidate | 555 passing leaf tests; zero failures/skips, including child-build dead-code elimination |

Missing baseline setter assertions fail before some nested cases are created, hence
different leaf totals. The baseline compiles; missing features are not skipped.
The four unchanged `TestFlagParseErrorsRetainTypedProvenanceThroughExecute` cases now
pass without relaxing their identity expectations. Additional checks exercise actual
child execution, overrides/reparenting, direct validators, generated completion and
nil-command success/refusal. A review suspicion of nil-receiver panic was disproven:
the ancestor walk checks nil before field access; the new tests pass on both graphs.

| Current log | SHA256 |
|---|---|
| `baseline-oracle-final.jsonl` | `816ff1ec55afbe82129dc524cd6a3622b1ddda419685fb851f7372cbf20e550c` |
| `candidate-oracle-final.jsonl` | `b9b65cbb4b3e0799a1f51add05e4f8b728234c3f85093eacaea7c0476945c021` |
| `baseline-kit-final.jsonl` | `e7799b1892306d9b9c3af5120a9c693fa3aac4cf9ea6d320c3c78c0fa853e814` |
| `candidate-kit-final.jsonl` | `80d7fb3de271f1a42a0a2782d32e5d06dd0aada8bcf62aa9c8a0b5d86ee8d46e` |
| `candidate-upstream.jsonl` | `d1caddd84ff71f6968a29847671fa7506a214cf4a6dfadf4b1abf02843566410` |
| `mcp-docs.jsonl` | `f2217332e25874560671e70e6070535c32817f3e3b3f6a40bc06686b95837aa8` |

The MCP race probe serves exact current intent/CLI documents and ADR-0052 through the
protocol; indexes/status/guidance pass. It does not prove a real agent host loaded them.
Initial `candidate-kit.jsonl` retains an orchestration failure: a nonexistent extra
`cmd/terminal-gallery` path was supplied. Corrected final runs cover the actual
`apps/terminal/cmd/terminal` through `./apps/terminal/...`; no failed test was dropped.
The seven asset skips are not delivery proof; this slice touches no paid assets or UI.

Remaining gate: review dependency-patch/upstream maintenance cost and obtain scoped
authority before any real dependency or consumer exit migration. The API is unavailable
on root pins, historical Bosun/Surveyor usage failures remain, and G5 is not complete.

## Historical always-on candidate and replay

The [maintained preparer and tagged tests](fixtures/cli-consumers/cobra-origin/README.md)
bind Cobra v1.10.2 and pflag v1.0.10 by full module sums. Sources are snapshotted and
verified before fresh output creation; the cache and root pins are not edited. The
candidate adds a typed Cobra refusal wrapper at actual construction/dispatch origins,
and captures built-in pflag conversion provenance before the exact `Value.Set` call.
Func/BoolFunc/TextVar/Go flag bridges/custom values remain application failures.

Root review caught two issues before adoption: tagging before FlagErrorFunc changes
callback input, and a test using a Go flag that also implements pflag.Value bypasses
the Go bridge. Final code tags only after an identity-preserving callback; a separate
Go-only value, synchronous value mutation, embedded/direct values and persistent-hook
precedence now have explicit canaries. No callback is replayed; public ParseFlags stays
raw. Unknown marker kinds and mixed marked joins remain unclassified in the test policy.

Retained root: `/tmp/appkit-cobra-origin.DV21da`. Preparations `candidate-c` and
`candidate-d` have byte-identical receipts and per-file manifests. All 156 manifest
files were rehashed after upstream tests and still matched; unrelated upstream files
and licenses remain intact. Earlier `candidate-a`/`b` are superseded experiments.

| Module | Input Go sum | Candidate Go sum | Changed files |
|---|---|---|---|
| Cobra | `h1:DMTTonx5m65Ic0GOoRY2c16WCbHxOOw6xxezuLaBpcU=` | `h1:xlFkKrebMuUoBcGHpCLVv7GypSJBVCajEK9wLwoq0AI=` | 4 of67 |
| pflag | `h1:4EBh2KAYBwaONj6b2Ye1GiHfwjqyROoF4RwYO+vPwFk=` | `h1:tP4kGLHg6bnDYk7Q7oR4yqEh8P98811oIRAQLphJfSI=` | 3 of89 |

Candidate receipt SHA256 `a2eec3b244712e68bf84ff5f757691a05a683cec51d75c1a0ae69debcd2ac5ab`.
These are private modified module trees, not versioned forks, releases or supply-chain certification.

## Historical execution evidence

- Same final tagged race oracle: baseline45 passing/69 failing leaf tests; candidate
  **114/114 leaf tests pass** (128 pass events including containers/package). Baseline
  compiles; missing typed-origin features fail assertions rather than disappearing.
- Includes all35 error-capable built-in flag converters; two string success paths;
  framework argument/required/group/Traverse/completion failures; application callbacks,
  explicit exits and error compositions. This is a bounded matrix, not every Cobra API.
- Complete upstream Cobra and pflag package suites pass with race detection, including
  dead-code elimination. Initial subprocess build failed offline because CLI `-modfile`
  did not propagate; inherited `GOFLAGS=-modfile=...` resolves the same pinned graph.
  That initial operational failure is retained in tool history; no test was skipped.
- Root baseline race checks pass across all five CLI-kit packages, terminal app/command/
  deadline probe and `cmd/appkit` (nine packages). Preparer canaries cover source drift,
  unsafe files, output collisions, patch context and deterministic hashes.

| Retained log | SHA256 |
|---|---|
| `candidate-final.jsonl` | `4f4f646724ec2a065e2139e8bb18f5b8c796e245b246d884aa8a5a595c682e56` |
| `baseline-final.jsonl` | `32610a7db81d0da5881f4c98049de12e7833e9cea6f727f883d5869749597635` |

## Historical compatibility finding and surviving adoption gate

The unchanged command package under the initial candidate fails four assertions in
`TestFlagParseErrorsRetainTypedProvenanceThroughExecute`: hook error and rendered
error are no longer pointer-equal because the returned error gains a Cobra wrapper.
The callback still receives the original concrete pflag error; text and `errors.Is`/`As`
survive. Keep `candidate-command-compatibility.txt` as a **failing compatibility check**,
not a pass or the historical four Bosun/Surveyor silent-usage failures.

Automatic classification is test-only. ADR-0052 now resolves candidate default-off
selection; two dependency patches, enabled error-type migration and authorized consumer
adoption still need resolution. Bosun/Surveyor adapters, module pins and usage exit1 behavior
are untouched; installed-host proof is still absent. No live-host checks, commits,
publication, deployment or global changes. Current Go1.26.3/macOS arm64 evidence is
not Windows or PTY-interaction coverage; this slice does not alter terminal rendering.

Root `go.mod` SHA256 remains `0fb99ae8e4595cc5d7fc8d4c3821cf9fec8efd6069251c97cfac132b1523ced1`;
`go.sum` remains `f25254821cdc4357d5502862de9a987439452328edb6e74d7e8a1cd682587114`.
