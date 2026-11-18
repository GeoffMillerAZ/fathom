# DEP-M1 · TypeScript and JavaScript dependency edges

Protocol `dep-m1/typescript-program-trace-v1`. The [preregistration](../../../../calibration/oracles/deps-tsjs/pins.json) fixes the public URLs, full commit IDs, byte-sorted tracked-file selections, tool version and admission bars. Each dialect has its own admission. The combined arm pools the evidence for reporting.

## TypeScript population completion preregistration

The architect authorized one added subject and one product attempt. The selection source is the pinned TypeScript corpus list in repository order. A candidate needs at least 100 tracked TypeScript files in the documented scope, config chains wholly inside the tracked tree, and a successful run of the pinned compiler oracle. Only Git and the compiler oracle informed this choice.

| Corpus ordinal | Public repository | Decision |
|---|---|---|
| 1 | gcanti/fp-ts | Existing DEP-M1 subject |
| 2 | ReactiveX/rxjs | Existing DEP-M1 subject |
| 3 | react-bootstrap/react-bootstrap | Existing DEP-M1 subject. All 233 files and their original unknowns remain in the reading. |
| 4 | tailwindlabs/headlessui | First eligible subject |
| 5 | palantir/blueprint | After the first eligible subject |

Headless UI is pinned at `d13526d02a2de92c4ad7b62c15cd980636543fe2` from `https://github.com/tailwindlabs/headlessui`. The documented React package scope selects all regular tracked `.cts`, `.mts`, `.ts` and `.tsx` files under `packages/@headlessui-react/src/` and `packages/@headlessui-react/types/`, including declarations and tests. It selects 201 files (86 `.ts`, 115 `.tsx`), the complete TypeScript tracked set of that package. Selection SHA-256 over byte-sorted paths followed by NUL is `e0ce54fbac2d399937ed46bfd69e474f3b9d521b8ebfcfdf4b2ff1c72140f3fb`. Other packages and playgrounds are outside this scope. Resolution sees the whole pinned repository.

Every selected file uses the tracked `packages/@headlessui-react/tsconfig.json`. It has no extends or project references. Node v26.12.0 and TypeScript 6.0.3 with the original compiler digest ran the tracked-only oracle successfully. The compiler listed all 201 selected roots and emitted truth for all 201. The probe recorded one config group, zero config or syntax losses, 785 in-repository import resolutions and 385 unresolved bare operands. The oracle truth JSON SHA-256 is `cb8a77e4670f7cdadf87f33323caeb541a3b4f6b6e0bce3d440be4352ee0e605`. The compiler reported option diagnostics TS5053, TS5101 and TS5107. The oracle grades file and resolution evidence.

The original population floor remains three oracle-covered subjects and 300 files per dialect. B1 allows zero missed repository dependencies on complete files. B2 allows zero wrong or ambiguous bindings on complete files. Any complete file lacking oracle coverage makes the candidate inconclusive. B3 requires byte-identical output on two runs at the primary path and another at the relocated path. B4 reports complete share, unknown codes and extra-edge rate, with complete share below 80% reported as low coverage. B5 retains detect-only on a B1 or B2 failure. The original subjects, selection, compiler and JavaScript admission remain in force.

## Original admission

| Dialect | B1 and B2 on complete files | B3 | B4 complete share | B5 admission |
|---|---|---|---:|---|
| JavaScript | Pass across the three oracle-covered subjects, 0 missing, wrong or ambiguous bindings | Pass | 153/1,508 (10.15%) | Measured, sound, low coverage |
| TypeScript | 0 missing, wrong or ambiguous bindings in the covered candidate subset. Two of the three selected subjects have oracle truth | Pass | 250/1,023 (24.44%) | Inconclusive population. Detect-only |
| Combined | 0 missing, wrong or ambiguous bindings in the pooled candidate subset | Pass | 403/2,531 (15.92%) | Reporting arm only |

fp-ts, react-bootstrap and webpack had zero candidate-complete files, so the soundness bars are inconclusive for those individual subjects. RxJS, ESLint and OpenLayers passed the candidate checks, each with low coverage. The JavaScript verdict uses all three preregistered subjects and the pooled complete-file bar. TypeScript needs native coverage of its third subject to qualify. Independent reproduction from a clean clone is the integration review step.

## Reproduce

Use Node v26.12.0 and the TypeScript 6.0.3 distribution. The SHA-256 of `lib/typescript.js` is `569177652966bd528c319171c7dd22860dbf72bde116cbc4f644f1d02bb12e39`. The runner checks the digest and both versions. The npm archive's integrity is recorded in the preregistration.

From a clean checkout of this revision with the Go module cache prepared, run the following in Bash. Use `--cache "$CORPUS_CACHE"` on the acquisition command when the six pinned bare Git repositories are available locally. If TypeScript is already available, set `FATHOM_DEP_TSJS_COMPILER` to its absolute `lib/typescript.js` path and omit the archive acquisition. The test requires its inputs when enabled and works offline with prepared checkouts and compiler.

```sh
dep_scratch="$PWD/workspace/artifacts/dep-m1"
mkdir -p "$dep_scratch"/{compiler,first,second,tmp,gotmp}
npm pack typescript@6.0.3 --ignore-scripts --cache "$dep_scratch/npm-cache" \
  --pack-destination "$dep_scratch"
python3 - "$dep_scratch/typescript-6.0.3.tgz" <<'PY'
import base64, hashlib, json, pathlib, sys
pins = json.loads(pathlib.Path("calibration/oracles/deps-tsjs/pins.json").read_text())
archive = pathlib.Path(sys.argv[1]).read_bytes()
assert "sha512-" + base64.b64encode(hashlib.sha512(archive).digest()).decode() == pins["npmIntegrity"]
PY
tar -xzf "$dep_scratch/typescript-6.0.3.tgz" -C "$dep_scratch/compiler"
python3 calibration/oracles/deps-tsjs/prepare.py "$dep_scratch/subjects"
export TMPDIR="$dep_scratch/tmp" GOTMPDIR="$dep_scratch/gotmp"
export GOPROXY=off GOSUMDB=off
export FATHOM_DEP_TSJS_NATIVE=corpus
export FATHOM_DEP_TSJS_COMPILER="$dep_scratch/compiler/package/lib/typescript.js"
export FATHOM_DEP_TSJS_SUBJECTS="$dep_scratch/subjects"
for dep_run in first second; do
  export FATHOM_DEP_TSJS_SCRATCH="$dep_scratch/$dep_run"
  go test -race -count=1 -v -run '^TestTSJSNativeOracle$' ./pkg/lang/deps \
    > "$FATHOM_DEP_TSJS_SCRATCH/native.log" 2>&1 || exit 1
done
diff -u <(sed -n 's/.* DEP-M1 /DEP-M1 /p' "$dep_scratch/first/native.log") \
  <(sed -n 's/.* DEP-M1 /DEP-M1 /p' "$dep_scratch/second/native.log")
```

In a shared lane, wrap each direct `go test` in the package lease. The fixture-only native control uses `FATHOM_DEP_TSJS_NATIVE=control`. Ordinary `go test` skips the native reader. Each corpus run verifies both checkout identities, membership digests and clean Git status before and after. It takes two observations at the primary path and one at the relocated path and checks byte equality. The [tracked-set recipe](../../../design/language-breadth.md#1331-freeze-the-measured-set-and-holdout) provides the wider before/after worktree receipt using `SUBPATH=.` for each subject checkout.

## Resolution and oracle scope

The pure Go reader uses the pinned Tree-sitter grammars and existing import and re-export bindings. The grammar lock pins TypeScript v0.23.2 at blob SHA-256 `30080208c90581bf3b29b412e9ee2e9a11c89b36153b6a94cc07da7b5000afcb` and JavaScript v0.25.0 at `7245ee58564f991827498e36b5ec14420b99759596490e3a74ed0993b24ed904`. It supplements those bindings with physical operand lines, dynamic import expressions, import types, import-equals, CommonJS calls and `require.resolve`. It reads the caller's repository view, resolves supported relative files, extension substitution and permitted directory indexes, then records consulted config files. Bare operands try exact aliases, the most specific single-wildcard alias, ordered substitutions and the configured base URL before package lookup. The nearest `tsconfig.json` takes precedence over a same-directory `jsconfig.json`. Node-style ESM requires explicit relative extensions. Bundler and legacy lookup can try implicit extensions and indexes.

Supported package exports use exact subpaths before wildcard subpaths, then matching condition keys in manifest order. Active conditions are `types`, `default`, the applicable `import` or `require`, and `node` for Node modes. The reader discloses export arrays and version-qualified conditions, unsupported config inheritance and references, custom conditions, path suffix overrides, unprovable symlinks, parser recovery, implicit JSX runtime imports, JSDoc imports, triple-slash references, computed operands and ambiguous loaders. It recognizes direct npm manifest version pins and matching v2/v3 `package-lock.json` entries for direct packages. Other lock formats, dependency links, visible declaration-package overlays, unresolved subpaths and unpinned packages leave the file incomplete. Bare Node built-ins have a runtime identity, so an npm manifest pin for the same name cannot pin them. It does not evaluate repository code or run the compiler.

The Go harness invokes `node calibration/oracles/deps-tsjs/trace.cjs <compiler> <subject> <selected-paths.json> <trace.log>` with absolute paths and the subject as the child process working directory. It saves the JSON truth independently of the product output.

The test oracle builds TypeScript `Program` instances with the tracked selected roots and the nearest project's parsed compiler options. It records compiler file lists and native resolution traces, including compiler resolver calls for literal CommonJS and dynamic operands the Program did not enumerate. The adapter walks compiler ASTs for import identity, including JSDoc and reference directives. The host confines reads to the pinned checkout. It enables JavaScript reading and disables emit, compiler library loading and ambient type auto-discovery. It performs no dependency install or project build, and the tally includes selected files outside native project membership. A missing inherited config or syntax-faulting root contributes no truth. Bare and relative operands with no native target and computed requests are counted separately from resolved edges.

The native control checks alias, package export and extension substitution bindings. Dropping the Go reader's edges makes the B1 control fail with missing edges and leaves compiler truth unchanged. Removing the control's source import changes compiler truth. This exercises evidence independence without running corpus code.

One correction followed the first corpus run. New fixtures reproduced seven loader and package ambiguity failures with `go test -race -count=1 -run '^TestTSJS(LoaderUncertainty|CompilerPackageAmbiguity)$' ./pkg/lang/deps`. Handling those cases passed the offline package suite. The corpus pins, tracked selection, oracle adapter and bars stayed fixed. `TestTSJSResolutionCaps` confirms refusal at 4,097 alias entries and beyond the 16-level package export traversal budget. `TestDependencyReadCapAndCancellation` covers the shared 2 MiB source cap.

The native adapter's options produced TS5053 where a subject supplied `lib` and the probe disabled compiler library loading. The compiler also reported TS5101/TS5107 for deprecated RxJS and fp-ts options. These diagnostics do not grade a build. React Bootstrap's inherited configs could not load from a tracked-only checkout, leaving all 233 selected files without truth. RxJS had one syntactically invalid native file and 504 local resolution results beyond the Program file list. webpack had one such result. Those trace targets were checked against the repository view. The pinned grammar also reports recoverable syntax gaps in sources accepted by the compiler, including fp-ts generic overloads and the fixture's `export type *`.


## Counts

Candidate completeness uses explicit trust for the trial. Its counts include only the listed tracked inputs. Complete oracle coverage is shown explicitly because an uncovered complete file makes B1 and B2 inconclusive. Expected and read are comparable repository edges in complete files. The B4 extras denominator includes every comparable repository edge in oracle-covered files, including incomplete files.

| Subject or pool | Files | Oracle | Complete | Complete with oracle | Expected | Read | Missing | Wrong | B2 ambiguous | Extra | B4 extras / comparable |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| fp-ts | 204 | 204 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 204/2213 (9.22%) |
| rxjs | 586 | 585 | 250 | 250 | 1296 | 1546 | 0 | 0 | 0 | 250 | 585/2590 (22.59%) |
| react-bootstrap | 233 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0/0 (n/a) |
| eslint | 389 | 389 | 59 | 59 | 112 | 112 | 0 | 0 | 0 | 0 | 0/1055 (0.00%) |
| openlayers | 373 | 373 | 94 | 94 | 347 | 347 | 0 | 0 | 0 | 0 | 7/2527 (0.28%) |
| webpack | 746 | 746 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 746/4769 (15.64%) |
| **TypeScript pooled** | 1023 | 789 | 250 | 250 | 1296 | 1546 | 0 | 0 | 0 | 250 | 789/4803 (16.43%) |
| **JavaScript pooled** | 1508 | 1508 | 153 | 153 | 459 | 459 | 0 | 0 | 0 | 0 | 753/8351 (9.02%) |
| **Combined** | 2531 | 2297 | 403 | 403 | 1755 | 2005 | 0 | 0 | 0 | 250 | 1542/13154 (11.72%) |

Each unknown count uses the same evidence denominator for its row (reported edges plus unknowns). Codes absent from the table were zero throughout.

| Subject or pool | Complete share | Edges | Unknowns / evidence | dynamic-import | external-unpinned | parse-error | unresolved-alias | unsupported-construct | read-limit |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| fp-ts | 0.00% | 2213 | 933/3146 (29.66%) | 0 | 0 | 666 | 24 | 243 | 0 |
| rxjs | 42.66% | 2592 | 1151/3743 (30.75%) | 0 | 0 | 15 | 944 | 192 | 0 |
| react-bootstrap | 0.00% | 1227 | 1113/2340 (47.56%) | 0 | 0 | 3 | 676 | 434 | 0 |
| eslint | 15.17% | 1170 | 552/1722 (32.06%) | 3 | 115 | 0 | 6 | 428 | 0 |
| openlayers | 25.20% | 2534 | 3518/6052 (58.13%) | 0 | 0 | 0 | 0 | 3518 | 0 |
| webpack | 0.00% | 4769 | 4067/8836 (46.03%) | 8 | 0 | 0 | 201 | 3858 | 0 |
| **TypeScript pooled** | 24.44% | 6032 | 3197/9229 (34.64%) | 0 | 0 | 684 | 1644 | 869 | 0 |
| **JavaScript pooled** | 10.15% | 8473 | 8137/16610 (48.99%) | 11 | 115 | 0 | 207 | 7804 | 0 |
| **Combined** | 15.92% | 14505 | 11334/25839 (43.86%) | 11 | 115 | 684 | 1851 | 8673 | 0 |

### Native coverage and build losses

| Subject | Compiler listed | Config missing | Syntax lost | Outside build roots | In-repo trace edges | Native external | Unresolved relative | Unresolved bare | Computed | Trace fallback | Resolved outside file list |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| fp-ts | 204 | 0 | 0 | 0 | 1805 | 20 | 0 | 4 | 0 | 0 | 0 |
| rxjs | 586 | 0 | 1 | 0 | 1915 | 0 | 0 | 447 | 0 | 3 | 504 |
| react-bootstrap | 0 | 233 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| eslint | 389 | 0 | 0 | 0 | 1018 | 32 | 6 | 170 | 3 | 0 | 0 |
| openlayers | 373 | 0 | 0 | 0 | 6962 | 0 | 0 | 73 | 0 | 0 | 0 |
| webpack | 746 | 0 | 0 | 0 | 5853 | 107 | 231 | 382 | 9 | 5 | 1 |
| **TypeScript pooled** | 790 | 233 | 1 | 0 | 3720 | 20 | 0 | 451 | 0 | 3 | 504 |
| **JavaScript pooled** | 1508 | 0 | 0 | 0 | 13833 | 139 | 237 | 625 | 12 | 5 | 1 |
| **Combined** | 2298 | 233 | 1 | 0 | 17553 | 159 | 237 | 1076 | 12 | 8 | 505 |

### B3 output digests

- fp-ts `e1b4e0f019a406e56aae73d30e44e31165729a501d7f494aecd84ef0c77ad3fe`
- rxjs `e4f5bf3e42094f01be4476a91f55065f47f314323f4d3e2fdc0eebdc62691016`
- react-bootstrap `4c6e2271282ac750f45465c6fe380940168c6b1e78b34efa36599a625631af19`
- eslint `01ade6c0fc6b00f207a995cf62beb27ff9385a0ba23354e459eaf97132150766`
- openlayers `d39f1401a3aa58996330f704617650e926a8be9f5b98d378bf3fa4a0b554a648`
- webpack `784990f26e6fbc071122405349b71eb78b24516107aa1a6a63ab8984b675815a`
- **TypeScript pooled** `52a40e4fc4dbb7e6fde15fcde6efc7d9f913e0c09a3af8fc1b8096941f9093ed`
- **JavaScript pooled** `ed72b3de907ff4d8cd2188e568f54eeb2b2aac932043cdb30bbefe4a7f9fd781`
- **Combined** `a9dbf338ffb38b8420d22d37082e138a970c652ccd33988496f7459338749e18`

## Tracked-set receipts

The whole tracked set is wider than the frozen prefixes and suffixes in the preregistration. The counts below include regular entries across the entire subject. The selection hashes in the preregistration cover the measured files. Before and after receipts matched across relocated copies. No untracked or ignored inputs appeared. Symlinks and gitlinks are outside the regular-file selection and submodules were never checked out. Raw manifests, compiler traces and per-file paths stay in local scratch.

| Subject | Regular files | Bytes | Symlinks | Gitlinks | Whole tracked-set SHA-256 |
|---|---:|---:|---:|---:|---|
| fp-ts | 431 | 4499544 | 0 | 0 | `017969c26316b57cb482af3234d950f8fc69c375b9fb80a5d6197816e9c6f262` |
| rxjs | 1308 | 13661631 | 0 | 0 | `416e6e854c92cb53d1f6b65f51dc90e96ee427403c296ed1366c3fc4226421f1` |
| react-bootstrap | 629 | 2380329 | 0 | 0 | `85eba50d1305d863c1f812265291310c742a0635d4d63560e6fa2a1ae0c6c6c8` |
| eslint | 2362 | 23722405 | 0 | 0 | `36b6ace510566946895f850465ee5572120426b0a4c29b228e5f8e304272966e` |
| openlayers | 2330 | 60741228 | 2 | 0 | `13367383d1029f69f983d749024dc7606d347d34ab51c2e3f61f6047e04788b0` |
| webpack | 19668 | 52140763 | 1 | 4 | `e92d9f9540a24d6056dc21460c68bd09b9042a3b0ee4c409b0b6ffd66b69315f` |
