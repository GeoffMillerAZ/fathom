# Issue 215: Telescope's native Lua import trace clears 90%

The separate [Telescope reading](../bench/readings/lang/tuned/lua-native-215.json)
is **L3 excellent**, with L1 excellent and L2 good. Native execution covers
**215/237 (90.72%)** of the unchanged literal in-repo statement population,
versus the historical 192/237 (81.01%). The issue's minimum of 213 alone would
be below the design's 90% bar: 214 sites are required; the observed 215 clears
both. No bar, denominator, corpus shard, grammar or canonical reading changed.

This is a calibration-subject remeasurement, not a fresh holdout. Telescope
was already read. It is stored outside canonical aggregation and is not pooled
with a newly selected subject. Kong's tier-B evidence remains unchanged;
canonical Lua therefore remains L3 good. L2 remains capped at C under OL-R2.
No SCIP result, Kong native result, or whole-language excellent claim follows.

## Native execution and per-subject grade

Neovim 0.12.6 executes the original minimal init, sequential Plenary tests,
75 original Lua-file loads in separate processes, then a native public-API
[exercise](data/lua-oracle-2026-10-25/exercise_telescope.lua). No source,
loader, API, platform guard or global is stubbed or patched. The exercise
runs setup/documentation, history, sorters, command completion, previewers,
headless pickers and test-state APIs. Its protected calls retain one failed
keymap-entry exercise; 12 per-file loads fail and stay recorded. Plenary exits
0; macOS picker-suite guards remain intact. Process exit alone is not coverage.
Only successful requires with the exact source path, line, literal module and
native target matching the frozen population become truth. Unexecuted sites
stay in the 237 denominator; only the 215 executed sites are exported.

| Metric | Numerator / denominator | Ratio / tier |
|---|---|---|
| Native literal execution | 215 / 237 | .9072; imports A |
| Import resolution | 215 / 215 | 1; A |
| File-edge precision | 198 / 207 | .9565; A |
| File-edge recall | 198 / 198 | 1; A |
| Import-edge precision (disclosure, not the file-edge bar) | 215 / 235 | .9149; 20 extracted imports unattested |
| Definition precision/recall/kind/range | 941 / 941 each | 1; independent luaparse B |
| Signed callables | 941 / 941 | 1 |
| Calls precision / recall | 3235 / 3383; 3235 / 3319 | .9563 / .9747; C, caps L2 at good |

Harness population: 75 files / 703,625 bytes, 0 faults, ERROR-or-MISSING
file union 0, 3.93 MB/CPU-s, max file 14 ms, 301 peak module pages,
0 compiled bytes reported. The grammar is unchanged (64,669 gzip bytes).
The instrument is the committed native runner at `4839e5101`; the harness
was built from the same unchanged Go sources. The separate reading records
that revision. This is a single measured throughput run, not a retiming study.

## Measured-set receipts and independence

Recipe section 13.3.1's exact manifest command was extracted without modification
into the ignored local runner and used before, after, and after the final
committed-instrument replay. All three tracked-set digests are
`52e0ad4e7e35d494335f90423c9be360ced642689050f046a8f84da992ea5fb3`:
111 regular files / 1,508,436 bytes, 0 symlinks, 0 gitlinks. Before/after/final
ignored-plus-untracked receipts have zero entries. No build generated subject
inputs. The 36 non-Lua files / 804,811 bytes explain the difference from the
harness; all 75 Lua files are measured. Full path lists stay ignored under
`workspace/artifacts/lang-lua/telescope-{before,after,final}.*`.

Discovery uses the unchanged connector's tracked/nonignored-untracked list,
regular files only, skip-directory/default-cache/scoped-ignore rules from
`internal/connector/{connector,defaultignore,gitdiscover,local}.go`, and the
Lua extension/ownership, binary/size/generated/minified gates. No per-run
`-only`, `-skip` or custom ignore was supplied. None removes a Lua file in
this subject. Native scanning finds 290 require statements: 286 static and
4 dynamic, 0 parse errors, 237 native in-repo literal sites. The 49 static
names not resolving inside the subject and 4 computed statements are not
silently turned into in-repo truth. The harness's 235 extracted import sites
and native 237 statement sites remain separate disclosed counts.

Raw native population canonical-JSON digest
`5c6b223110b29121693349aef903d1af3d30d94e52b5a49e4e917a5834b52e2f`
is unchanged with the scratch Fathom output replaced by an empty-node output
and then removed. No native runner consumes Fathom nodes. In a separate authored
control, changing `require("a")` to `require("b")` changes the native target
and digest (`68baae980f1856cdc8c42caeb0262a9ae4b93f99927edda47eba258bc0fab830`
to `c97ecc9673761a52ce69f667e74ea699fd4efc8bc55fc7f582334dc0e5b88929`).
These are input-sensitivity controls, not a fresh subject confirmation.
Local commands/results remain in `workspace/artifacts/lang-lua/independence.json`.

## Runnable reproduction and checks

The pinned subject, Plenary, runtime and tools are recorded in the existing
[Lua provenance location](../../internal/extractor/dialects/lua/PROVENANCE.md).
All tools are external processes under `workspace/tools`; no root dependency
or production package was added. Run heavy commands through the common lane
wrapper after sourcing the lane environment, without another lease inside it.

```sh
python3 docs/assessments/data/lua-oracle-2026-10-25/trace_telescope.py --exercise \
  --nvim workspace/tools/nvim-macos-arm64/bin/nvim \
  --subject workspace/corpora/telescope.nvim \
  --luaparse workspace/tools/luaparse/node_modules \
  --out workspace/artifacts/lang-lua/fresh-replay
python3 docs/assessments/data/lua-oracle-2026-10-25/export_native_trace.py \
  --trace workspace/artifacts/lang-lua/fresh-replay --out workspace/artifacts/lang-lua/replay-imports.json
LUA_TRACE_RESULT=workspace/artifacts/lang-lua/fresh-replay \
  python3 docs/assessments/data/lua-oracle-2026-10-25/test_native_trace.py
NODE_PATH=workspace/tools/luaparse/node_modules node calibration/oracles/luaparse/luaparse_dump.js \
  workspace/corpora/telescope.nvim > workspace/artifacts/lang-lua/definitions.json
workspace/tools/pyts312/bin/python calibration/oracles/ts-tags/tags_oracle.py \
  --lang lua --query calibration/oracles/ts-tags/lua-tags.scm --ext .lua --module-caller '<module>' \
  workspace/corpora/telescope.nvim > workspace/artifacts/lang-lua/calls.json
workspace/tools/indexbench lang-oracle -dialect lua -subject workspace/corpora/telescope.nvim \
  -subject-name nvim-telescope/telescope.nvim \
  -tool-oracle workspace/artifacts/lang-lua/definitions.json \
  -tool-oracle workspace/artifacts/lang-lua/calls.json \
  -tool-oracle workspace/artifacts/lang-lua/replay-imports.json \
  -commit 4839e5101 -out workspace/artifacts/lang-lua/replay-reading.json
python3 docs/assessments/data/lua-oracle-2026-10-25/test_export_native_trace.py
```

Mutation: in `data/lua-oracle-2026-10-25/export_native_trace.py`, change
`covered * 10 >= 237 * 9` to `covered >= 213`; change `actual != expected or`
to `False or`; replace `if key in seen` with a three-field match omitting the
target. For each, `python3 docs/assessments/data/lua-oracle-2026-10-25/test_export_native_trace.py`
reports FAIL (tier boundary, frozen scope, wrong-target tests respectively).
Restoring bytes reports OK. Full transcripts: `workspace/artifacts/lang-lua/mutation.log`
and `final-oracles.log`. The native target test was RED at 192/237 before the
exercise, then green at 215/237, including the exact frozen population match.

Caps: the same test command proves whole-input refusal at 8 MiB + 1 byte
(per-file and aggregate), and 100,001 events against the 100,000 cap.
No output oracle is published on aggregate refusal. Inputs are trusted local
trace artifacts; no claim of a new general-purpose hostile JSON parser follows.

The new nested Reading has no base version; its full sorted JSON was reviewed
and retained locally. Existing canonical Reading values/bytes were untouched.
Run `go test -race -count=1 ./internal/langbench -run
'^(TestCommittedReadingsRoundTrip|TestCommittedReadingsAreCanonical|TestLanguagesTrackerIsGenerated)$'`
for strict top-level values, recursive canonical bytes and the unchanged tracker.
Scoped Lua fusion and import-scoring race tests, release-notes check, merge-debris
(including staged content), and the org-name test are recorded in the lane proof.
No full gate or plan-checkbox verification is claimed; no checkbox changed.

## Five-source refresh (2026-11-16 UTC)

| Source | Query / result |
|---|---|
| Own runtime | [Neovim 0.12.6 Lua documentation](https://github.com/neovim/neovim/blob/v0.12.6/runtime/doc/lua.txt), native loaders and the actual trace above. Measured; tier A only for executed literal sites after coverage clears 90%. |
| SCIP / LSIF | [SCIP catalog](https://github.com/scip-code/scip), [Sourcegraph navigation catalog](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), `https://api.github.com/search/repositories?q=scip-lua`: 6 unrelated spelling/script hits, no measured producer found. This is a bounded search, not an absence proof. |
| Language servers | GitHub latest-release metadata for LuaLS and EmmyLua still gives 3.19.2 and 0.25.1. The named, licensed candidates and concrete exported interfaces remain in the [prior audit](2026-10-25-lua-oracle-ceiling.md). OL-R2 keeps their references C; no new L2 measurement or ceiling is claimed. |
| Package registry | `https://registry.npmjs.org/luaparse/0.3.1`: 0.3.1, MIT. Installed independent parser supplies literal sites and L1; not native call truth. |
| Runtime tracing | [Lua require manual](https://www.lua.org/manual/5.4/manual.html#pdf-require), Neovim's loaders, actual native API exercise above. Kong 3.9.2 remains unmeasured natively under OL-R3 (no Docker; patched OpenResty must install entirely inside lane roots). |

No lookup above failed. Raw response hashes are retained in the lane's ignored
search receipt. A concrete next trigger is a full pinned Kong native execution
environment inside lane roots followed by its unchanged-population trace and
remeasurement; the Telescope coverage trigger has fired and is now measured.
Controller owns the fresh critic and post-critic corpus cleanup; corpora remain
ignored for that review. Generated shared tables need no regeneration because
the canonical Lua reading is unchanged.
