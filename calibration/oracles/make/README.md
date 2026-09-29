# make oracles (LB-Q-makefile, design section 1.5 D2)

Two bench-only scripts, Python 3 standard library only, run against a checkout of the subject
(`task calibrate-clone -- makefile`). Neither is imported by fathom. Both write `indexbench
lang-oracle`'s neutral oracle JSON to stdout, and the reading is measured with both:

```sh
M=workspace/tools/make/bin/make        # GNU Make 4.4.1, built from the release tarball (below)
for s in cosmopolitan prometheus valkey; do
  d=workspace/calibration/makefile/$s
  python3 calibration/oracles/make/ctags_oracle.py -subject $d > workspace/tmp/oracle/ctags-$s.json
  python3 calibration/oracles/make/make_oracle.py  -subject $d -make $M \
      $( [ $s = cosmopolitan ] && echo -variant MODE=x86_64 ) > workspace/tmp/oracle/make-$s.json
done
go run ./cmd/indexbench lang-oracle -dialect makefile -subject $d -subject-name <owner/repo> \
    -tool-oracle workspace/tmp/oracle/ctags-$s.json -tool-oracle workspace/tmp/oracle/make-$s.json \
    -out docs/bench/readings/lang/makefile.json
```

GNU Make 4.4.1 (sha256 `dd16fb1d67bfab79a72f5e8390735c49e3e8e70b4945a15ab1f81ddb78658fb3`, the
published value) is built with `./configure --prefix=... --disable-nls --without-guile`. macOS ships
3.81, which cosmopolitan's own Makefile refuses (`$(error please use build/bootstrap/make)`) and
which has no grouped targets; GNU make is GPL-3 and is only run, never vendored.

| Script | Tier | Emits | What it does |
|---|---|---|---|
| `ctags_oracle.py` | B (definitions: L1) | `definitions` | universal-ctags' Make tags, with two corrections |
| `make_oracle.py` | A (references: L2) | `references` | prerequisite edges from `make -pn`, in the spelling the source uses |

## ctags_oracle.py: two corrections, both counted

The harness's own tier-B run scored the dialect at sym_recall .9164 and sym_precision .9438 (L1
unmet); the misses were read one by one, and two are defects of ctags 6.2.1's Make parser, not
disagreements:

1. **Line.** ctags reports every name of a backslash-continued statement at the statement's last
   physical line (`o/$(MODE)/a.o \` and `o/$(MODE)/b.o: private CFLAGS += -O3` are both line 2). A name
   that is not on its reported line but is on an earlier line of the same statement moves to the first
   such line (404 targets on cosmopolitan, none on the other two subjects).
2. **Modifier keywords.** `private CFLAGS += x` is tagged as a variable named `private` (likewise
   `override`, `export`, `unexport`); those tags are dropped (74 on cosmopolitan).

Everything else ctags says passes through, including what it gets wrong (`V ::= 1` tagged as a
target, `define D :=` tagged with the operator in its name, a target-specific assignment tagged as
both a target and a variable). The uncorrected numbers are in the reading's notes.

## make_oracle.py: what tier A means here, and what it does not prove

`make -pn -r` evaluates variables, functions, conditionals and includes and prints every rule's
prerequisites, expanded. It does not say which file a rule came from (only a rule with a recipe carries
a location) and it prints `$(OBJS)` as `a.o b.o`, so the pairs are put back in the form the source spells
them, per file:

1. Each Makefile-like file no earlier run has read is an entry point (`make -pn -r -f FILE`, in the
   file's directory, then the subject root; both under `sandbox-exec` with no network and writes only
   inside the subject and the temp dirs, because `-n` still runs `$(shell)` and the recipes that remake
   included makefiles). `MAKEFILE_LIST` names every file a run read, so an included fragment is measured
   in its includer's context. `-variant "VAR=value"` evaluates the subject again with extra command-line
   assignments (cosmopolitan: `MODE=x86_64`, because its `ifeq ($(ARCH), aarch64)` branch is dead on an
   arm64 host) and takes the union.
2. Each file is scanned for rule lines (continuations joined; recipes, comments, `define` bodies, special
   targets, static pattern rules and target-specific assignments are not rule lines). A pair (target,
   prerequisite) is kept when the run's database holds the edge. The prerequisite must be spelled without
   `$` (a value written with a variable or a function is not a name and is out of the truth, as it is out
   of the dialect's references); a target spelled with variables matches every database target of that
   shape (each `$(..)` is `.*`) that lists the prerequisite.
3. A pair on a rule line the database does not confirm (a dead conditional branch in every evaluated
   configuration) is not in the truth, so the dialect's collection of it is a precision miss: what a static
   extractor cannot avoid.

Read the metrics accordingly. **ref_recall is near 1 by construction**: the truth is the literal pairs make
confirmed, so recall says the extractor sees every literal prerequisite of a live rule line. **ref_precision
is the informative number** (pairs on lines make never evaluated). And the truth is a small part of what make
knows: on cosmopolitan (default configuration) make's database holds 238,335 edges, of which 51,820 are
under a spelled literal and 186,515 (78%) have a variable- or function-produced prerequisite that no static
extractor sees (valkey 12,275 / 577 / 11,698; prometheus 128 / 40 / 88). The dialect claims none of those.

stderr summarises each run: files, entry runs, files make could not evaluate (0 on all three subjects),
truth pairs, database edges aligned and expanded-only (summed over variants), and the spelled pairs the
database did not confirm.
