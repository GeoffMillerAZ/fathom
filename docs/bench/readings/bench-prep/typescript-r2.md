# TypeScript lock-bound retry and candidate search

Measured on 2026-10-26 (UTC). These C3 measurements supersede round 1. C4
remains unrun. No paid API was called. Full pins, licence hashes, lock hashes,
commands, times, exit codes and log hashes are in `typescript-r2.json`.

Every candidate uses its own tracked lock. Corepack prepares Yarn 1.22.22,
pnpm 11.23.0 (Redux and redux-toolkit), pnpm 12.4.1 (trpc), or npm 10.9.9
(the class-* packages and Remeda) before offline trials. Yarn and pnpm use
`install --frozen-lockfile`; npm uses `ci`. Tests are the package's own scripts:
Yarn `test`, pnpm `test`, the class-* `test:ci`, or Remeda's workspace `test`
with `--run`. Trpc's test also receives `--run` to prevent watch mode.
Class-transformer retains its explicit `--legacy-peer-deps --ignore-scripts`
compatibility flags; no subject or lock was patched.

The four-CPU limit and Node 22 image match round 1 except trpc and redux-toolkit, which use Node 24.21.0 to match their engine constraint or upstream CI. `--network none`, cleared
proxy variables, prepared Corepack caches and offline environment variables
apply inside each trial. Direct HTTPS probes exit 7. Trial processes are fresh;
setup and one warm-up are excluded. A job-local lock serializes offline runs.
Only the final valid environment contributes to the table. Trpc is refused after its complete failed warm-up; its three timed trials are unmeasured.

```sh
python3 calibration/bench/tools/inventory.py "$subject" TypeScript > "$inventory"
BENCH_JOB="$job" python3 calibration/bench/tools/remeasure.py "$config"
```

`calibration/bench/tools/README-r2.md` describes the configuration and C6
commands. C1 reads the pinned root licence. C2 counts tracked primary-language
lines, including tests/generated/examples. C5 counts non-merge test/non-test
co-changes over the fixed UTC window, with word-bounded fix messages; zero fixes
were replayed. C6 uses D24, with the witnesses in `c6-r2.json`. C7 grepped each
full repository name and distinctive short name through calibration, scenario
banks and bench readings. No previous subject read was found for these new
candidates; generic words and dependency mentions were inspected and excluded.
No organization exceeds two candidates. Fleet-index membership is unknown.

| Candidate | SHA | C1 | C2 lines | C3 seconds / exits | C5 co-change / fix-message | C6 witnesses | Screen |
|---|---|---|---:|---|---|---:|---|
| immerjs/immer | `75a290e44ba14bd723c8ef8546b6559c8ef3ae88` | MIT | 5,422 | 14.473, 14.658, 16.141 / 1,1,1 | 37 / 30 | 0 | refuse C3, C6 |
| microsoft/tsyringe | `78222334f49265ea2874fac2c73284345c1124d9` | MIT | 3,809 | 11.467, 11.6, 11.782 / 0,0,0 | 3 / 0 | 0 | refuse C2, C6 |
| reduxjs/redux | `61ebf8596654cf3c4b197e426e302fcb3c2bc13e` | unknown, MIT | 8,303 | 7.482, 4.173, 4.238 / 0,0,0 | 5 / 3 | 0 | refuse C6 |
| typestack/class-transformer | `a2734a5d154c9b0beb9a10555a04416bc371de06` | MIT | 8,125 | 20.106, 25.188, 21.01 / 0,0,0 | 0 / 0 | 4 | retained draft |
| typestack/class-validator | `2e1a5c27dbd65b80e27fe96b49bd6e6641fa3603` | MIT | 14,455 | 24.944, 14.961, 17.528 / 0,0,0 | 6 / 1 | 12 | retained draft |
| trpc/trpc | `8d09833c76791b2ea7270af174fdab9f1408aeb8` | MIT | 129,355 | warm-up 295.65; trials unrun / 1 | 278 / 145 | 13 | refuse C3 |
| remeda/remeda | `d02c3bdb5e18cfa53e76f33f4ae525adce515379` | MIT | 65,796 | 12.62, 14.416, 13.375 / 0,0,0 | 56 / 21 | 0 | refuse C6 |
| reduxjs/redux-toolkit | `0fd611cc79d44990ac9a040dec4d5cb446233440` | MIT | 86,935 | 92.494, 73.5, 78.397 / 0,0,0 | 96 / 30 | 26 | retained draft |

Immer's valid full-script failure is its locked Flow binary's unsupported Linux
arm64 platform. Its source and build tests pass before Flow fails. Tsyringe's
C2 and C6 refusals and Redux's C6 refusal stand independently of their corrected
C3 results. Remeda's npm workspace script was selected from its own manifest;
its root has no test script. The three added candidates are trpc, Remeda and
redux-toolkit; trpc (129,355) and redux-toolkit (86,935) meet the preferred large
band, while Remeda (65,796) is in the middle band.

Preparation failures are retained outside the table. Trpc first hit shared-mount
`fchown` errors during node-gyp setup (fixed by using the image's matching Node
headers), then missing pnpm shims (fixed with container-local Corepack enable).
pnpm 11/12 ignore npm's old offline variable and default to an implicit install
before scripts; explicit pnpm offline configuration and a warning-only
pre-script dependency check prevent a trial from reinstalling. A subsequent
trpc warm-up hit Unix-socket EPERM on the shared temp filesystem. That invalid
attempt was interrupted, logged and replaced by an ephemeral Linux tmpfs at
`/work/tmp`. Interrupted attempts are not counted as completed C3 failures. The Node 22 warm-up is also invalid for trpc's `engines.node: ^24.0.0`. It was replaced with the exact `.nvmrc` runtime, Node 24.21.0, whose image digest is recorded in the JSON. The final complete Node 24 warm-up fails the full default script; no further trial is needed to disprove three green runs, and no passing subset was substituted.

Class-transformer has zero qualifying commits since 2025-01-17 and
class-validator has six in the entire window since 2024-10-25. Both fail C5's
post-cutoff half for any model cutoff after 2024-10-25; they remain candidates
for BX-M0, which excludes C5. **TypeScript: no viable BX-44 subject yet**;
new candidates still need actual fix replays, C4, Q2 and controller vetting.
New candidates have no invented age or star observations and no held-out split;
the controller must collect D23 metadata before adding their admission manifests.
Native macOS and the exact twelve-core condition remain unmeasured.
