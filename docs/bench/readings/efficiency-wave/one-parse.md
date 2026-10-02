# One parse per file: the ingest reading (EW-54, wave ic, O361)

EW-54 reading: a tags-dispatched file and a CGO incumbent's file are no longer parsed twice, and ingest costs 16 to 45 percent fewer instructions on every polyglot subject (31.0 percent over the seven summed: 582.0 G before, 401.8 G after) with byte-identical output; whole tree 1,415.3 G to 1,180.7 G (-16.6 percent), peak RSS +1.2 percent, so LB-M5 does not turn on this item: the whole tree still costs 2.94 times its subjects summed (O362).

## What changed

- **EW-52.** `TSExtractor.ExtractWithFileSpecs` returns a file's symbols and its `FileSpecs` from one parse and one tags-query run. The extract stage calls it for a file of a file-resolve language and keeps the specs in a map that lives for one ingest call (internal/fusion/specmemo.go, on the ingest's sibling index): keyed by path, language and the content hash the file is fingerprinted under. `resolveLang` takes the entry before parsing, still writes `file_imports` as before (so `FileResolveReused` and every report counter stay), and parses only a file the stage did not see. A header the router sent to the other language is read as foreign and keeps its parse; a dialect whose `preprocess` swapped the tree, or a references-query fault, leaves no entry and the resolve pass parses it as before. No table, no persistent cache, no background job.
- **EW-53.** C#, Java, Rust, PHP and C++ run their wasm dialect's tags and references queries over the go-tree-sitter tree the incumbent already parsed (the same grammar commit, tsengine/grammars/PROVENANCE.md): `nativeFileSpecs` converts the cursor's matches to the engine's match shape and hands them to the capture-to-spec code. All five land: the probe found no difference on any file (Polly 801 C#, retrofit 306 and okhttp 71 Java, ripgrep 110 Rust, guzzle 70 PHP, fmt 47 C++ files), and the five sit behind the same per-ingest map. C# reads the tree of the source as written, never the O148 directive-blanked re-parse. No O415..O419 entry was needed.
- No `ExtractorEpoch` bump: symbols, `typed_edges` and `file_imports` are byte-identical (below).

## Method

Before is the 0fd24ac2 build, after is this branch (15a37789), both `GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 go build ./cmd/fathom`. Each reading is `fathom ingest --json --store <fresh sqlite> --service poly --path <subject, nested one level>` under `nice -n 10 /usr/bin/time -l`, with a fresh store, `FATHOM_HOME` and `FATHOM_CACHE_ROOT` per run, base and head interleaved, the second valid run of each the reading (footprint.md's method). The set is the w6 polyglot set (17 repositories at the SHAs the LB-M5 reading records). Load average (1 min) 8.4 to 28 over the runs (sibling waves' gates). A run is valid only with `read_failed` 0, no doc failures, no "doc index not finished", and the same `symbols` and `file_resolved` as the other valid runs: the volume's EPERM bursts make the file-resolved pass's own reads fail silently (`Gated`, not `read_failed`), which dropped `file_resolved` from 27,368 to 19,503 and 27,042 in two whole-tree base runs and was discarded; that exposure is one the memo removes (the pass no longer re-reads a changed file).

## Per subject (instructions retired, user + sys CPU, peak RSS)

| Subject | instr before | instr after | delta | CPU before | CPU after | RSS before | RSS after |
|---|--:|--:|--:|--:|--:|--:|--:|
| libuv | 54.14 G | 38.72 G | -28.5% | 3.66 s | 2.81 s | 356 MiB | 348 MiB |
| okhttp | 119.68 G | 89.81 G | -25.0% | 8.30 s | 6.06 s | 313 MiB | 292 MiB |
| Polly | 85.57 G | 51.23 G | -40.1% | 8.46 s | 3.91 s | 395 MiB | 511 MiB |
| ripgrep | 36.87 G | 22.63 G | -38.6% | 3.08 s | 1.88 s | 295 MiB | 233 MiB |
| fmt | 89.38 G | 74.97 G | -16.1% | 8.04 s | 5.06 s | 509 MiB | 436 MiB |
| guzzle | 12.57 G | 6.86 G | -45.4% | 1.09 s | 0.62 s | 144 MiB | 67 MiB |
| retrofit | 183.78 G | 117.60 G | -36.0% | 14.62 s | 9.64 s | 576 MiB | 459 MiB |
| sum of the seven | 581.99 G | 401.84 G | -31.0% | | | | |

Instructions are the reading; CPU seconds move with load. Peak RSS per subject is noisy under load (Polly's own before runs span 392 to 545 MiB); the whole-tree figure below is the bound.

## Whole tree (6,524 files, 75,971,918 bytes)

| Build | instructions | user + sys CPU | wall | peak RSS |
|---|--:|--:|--:|--:|
| before 0fd24ac2 (second valid run) | 1,415.3 G | 84.3 s | 90.0 s | 846.6 MiB |
| after (second valid run) | 1,180.7 G | 71.9 s | 79.1 s | 857.0 MiB |

Peak RSS moved +1.2 percent (the two valid after runs read 742.6 and 857.0 MiB, the before runs, valid and discarded, spanning 762 to 847): inside the 5 percent bound, so the map keeps whole `FileSpecs` (their rows total 6.2 MB of JSON over the whole tree) and nothing was narrowed. The map is emptied per language as `resolveLang` finishes.

Whole over the sum of the seven subjects ingested alone: 1,180.7 G against 401.8 G, **2.94 times**: the superlinear component of O362 is untouched by this item and now dominates (before: 1,415.3 G against 582.0 G, 2.43 times). Nothing here changes it.

## Byte identity

Sorted dumps taken from the second runs' stores (`typed_edges`: from_file, from_symbol, to_symbol, kind, pkg_path; `file_imports`: path, content_hash, imports_json, fault; `observations`: every column but the timestamp, hashed):

| Subject | typed_edges rows | file_imports rows | before = after |
|---|--:|--:|---|
| libuv | 5,266 | 374 | identical, identical, observations identical |
| okhttp | 11,800 | 689 | identical, identical |
| Polly | 77 | 802 | identical, identical, observations identical |
| ripgrep | 1,938 | 111 | identical, identical, observations identical |
| fmt | 1,723 | 83 | identical, identical, observations identical |
| guzzle | 266 | 70 | identical, identical, observations identical |
| retrofit | 1,458 | 950 | identical, identical, observations identical |
| whole tree | 30,135 | 4,198 | identical, identical, observations identical |

The parse-count tests: `TestExtractWithFileSpecsOneParse` (one parse where Extract plus ExtractFileSpecs ran two, nodes and specs deep-equal), `TestIncumbentFileSpecsFromNativeTree` (an incumbent's specs equal the wasm dialect's, no wasm parse) and `TestOneParsePerDispatchedFile` (an ingest of C, Python and the five incumbents' files runs 7 wasm parses, 24 before: one per tags-dispatched file, none per incumbent file, and the one foreign header).

## LB-M5: the command sequence for the integrator's reading

LB-M5's bound is 1.5 times the LB-M1 baseline (base 56c18502) on the polyglot set; the 2026-10-01 reading was 4.84 times CPU per byte. This item takes the second parse out (the C switch's +1.9 percent and the incumbents' wasm re-parse); O362 is the rest. The integrator reads LB-M5 on the merged tree after item linear lands, flips it only on a PASS, and on a FAIL updates LB-Q-c-rest, LB-Q-cpp-rest and O362 with the remaining causes and numbers. All scratch under `<scratch>` (the external volume, never the internal disk), with `tcc-keeper.sh` running; a read refused with "Operation not permitted" is the TCC wedge: reset it and discard the run.

```sh
W=<scratch>/lbm5; mkdir -p $W/{poly,bin,runs}
export GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0

# 1. The polyglot set: LB-M1's subjects at the SHAs its reading records, shallow-fetched by SHA and written
#    with git archive (one tree, 17 directories, 6,548 files).
while read -r repo sha; do
  n=$(echo "$repo" | tr / _); [ -d $W/poly/$n ] && continue
  d=$(mktemp -d -p $W); (cd $d && git init -q && git remote add origin https://github.com/$repo.git &&
    git fetch -q --depth 1 origin $sha && mkdir -p $W/poly/$n && git archive $sha | tar -x -C $W/poly/$n); rm -rf $d
done <<'LIST'
nvm-sh/nvm a885b885fef16fac4bc544188fb25e9e37ae83e8
libuv/libuv abe835d41317b55b16260990821f89b4ee9e437b
fmtlib/fmt 5da4e9a3fb15626ba92969199061e0169ebc8a00
App-vNext/Polly dc933fd172cbfae51b4ebc419014f4fed695cbb6
spf13/cobra adbc8813901bba65827259daa8e22ff94ec1f30e
square/retrofit e27d855b7b029e7f907dfc52205fdb3e9c603c16
expressjs/express 9a34acf03cb818ff3f8bc40e44176e277a25cbb9
square/okhttp 40a3b8749deaacf60a04c890aed052cb14ad36ee
nvim-telescope/telescope.nvim 40aedd8a68c78a656a10a8d62d80c54af59420fb
guzzle/guzzle 93939470950a9b11e2e84204166ef5e048c55fe4
bufbuild/protovalidate d06a71c9ba254a56c10a7085e1dd44d33cedeb56
pallets/flask d73fa1cdcbd8b1465c151db8924ba58b1dd14e35
sinatra/sinatra cb22afd7902b566b6eaba6c4ea89739494a65d12
BurntSushi/ripgrep 3fce3b5bb0236da2df6d99672afb8a719642eca7
lerocha/chinook-database 7f67772503d71ba90f19283c38e93923addb43fa
terraform-aws-modules/terraform-aws-vpc b3abd6df2ecf052451a361ed55b8f06f8742a795
colinhacks/zod 2bf7b0630d5378033e90bcee82cb32b0fe04628e
LIST

# 2. The builds: base from a worktree at 56c18502, head from the merged tree (no -ldflags).
git -C <integrator-clone> worktree add --detach $W/base-src 56c18502
(cd $W/base-src && go build -o $W/bin/fathom-base ./cmd/fathom)
(cd <merged-tree> && go build -o $W/bin/fathom-head ./cmd/fathom)

# 3. Same input: both builds discover the same files and bytes (6,524 files, 75,971,918 bytes expected).
for b in base head; do FATHOM_HOME=$W/dry-$b FATHOM_CACHE_ROOT=$W/dry-$b/c $W/bin/fathom-$b ingest --dry-run --json \
  --store $W/dry-$b.sqlite --service poly --path $W/poly | grep -E '"(files|bytes)"'; done

# 4. One run (a function; a fresh store, FATHOM_HOME and FATHOM_CACHE_ROOT every time).
run() { # run <base|head> <poly|subject-dir-name> <tag>
  local b=$1 s=$2 t=$3 R=$W/runs/$1-$2-$3 src=$W/poly/$2; [ "$s" = poly ] && src=$W/poly
  rm -rf $R; mkdir -p $R/home $R/cache
  echo "load $(sysctl -n vm.loadavg | awk '{print $2}')" > $R/load.txt
  FATHOM_HOME=$R/home FATHOM_CACHE_ROOT=$R/cache nice -n 10 /usr/bin/time -l $W/bin/fathom-$b ingest --json \
    --store $R/s.sqlite --service poly --path $src > $R/out.json 2> $R/time.txt
}

# 5. Whole tree: base head base head (interleaved). Then every subject, nested one level, in the same
#    order. Wait for a quiet machine first (load average under about 4 on the 10-core box) and read
#    the load into each run's load.txt; if siblings keep it high, say so in the reading.
for t in 1 2; do run base poly w$t; run head poly w$t; done
for s in $(ls $W/poly); do for t in 1 2; do run base $s r$t; run head $s r$t; done; done

# 6. Validity: discard (and rerun) a run with "read_failed" other than 0, a "failures" array under "docs",
#    docs.skipped "doc index not finished", "Operation not permitted" in time.txt, or a "symbols",
#    "file_resolved" or docs.units_written different from the run's twin (a refused read in the
#    file-resolved pass gates the file silently and lowers file_resolved).

# 7. The reading is the second valid run of each build. From time.txt: "instructions retired",
#    user + sys CPU, "maximum resident set size". LB-M5 = (head CPU / head bytes) / (base CPU / base bytes)
#    on the whole tree; also report instructions, wall, and head whole over the head subjects' sum
#    (the O362 ratio: 2.94 here). PASS needs the CPU ratio at or under 1.5; write the verdict line in
#    footprint.md and flip LB-M5 only on a PASS.
```

Reading at this item's branch, for comparison: before (0fd24ac2 whole) 1,415.3 G, after 1,180.7 G, subjects after 401.8 G summed; the 2026-10-01 LB-M1 base (56c18502) whole was 238.1 G, so this item alone leaves instructions about 4.96 times the base (not a CPU-per-byte ratio; the integrator's sequence measures that).
