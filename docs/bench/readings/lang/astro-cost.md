# Astro's ingest cost (wave lf7, item angularastro; K26, K27, K28, K30)

**Verdict: the dialect's own work passes under K27; one finding needed an owner ruling or a fix under
K26 before this ships, and is fixed at integration (the last section: LB-F526, LB-F550). origin/main extracts no `.astro` file (Astro was disclosed unsupported), so the
astro dialect's own extraction is new work on files that were skipped, disclosed here and not counted
as a regression: on a tree of `.astro` files alone the 91 examples extract in 0.6 s of CPU, compiles
included, and astro.json's harness reads 1.23 MB per CPU-second. The finding: on a tree mixing `.astro`
with `.ts`, `.tsx`, `.mjs` and `.css` files (withastro/astro's `examples/`), the process's grammar-module
pool (six modules) thrashes, because a hosting extractor leases its injected TypeScript and CSS under its
own keys, and about 2 s of extract CPU appears that is neither half's work: that slows the `.ts`
extraction the base already did (cold +202% wall on that tree). The fix is a foundation change outside
this item's files. On a repository with no `.astro` file the dialect costs nothing measurable
(realworld: wall +4.5%, CPU +3%, store equal, within the run's noise; startup equal); warm re-ingest is
unchanged everywhere (stat-gated); store grows +1% to +75% with the new definitions; peak RSS +35% to
+68% on the Astro subjects (the modules an injecting host leases at once). K28: one Astro change; no
TypeScript, CSS or HTML query moved, so their sums do not. Angular: nothing changed (no dialect), so no
K26 reading is owed.**

Measured 2026-10-11, base `fathom` built at c39875f3 (origin/main), branch at fafa07bb (the astro
dialect), same machine, interleaved, three rounds (medians; five for the no-Astro check). Other wave
workers ran on the machine throughout, so single rounds vary; CPU time is given where wall is noisy.

## Method

`fathom ingest -service <s> -path <subject> -store <fresh sqlite>` under `/usr/bin/time -l`: **cold** is
the first ingest into an empty store (every file extracted; the CLI compiles each grammar it uses in
memory, as every non-daemon process does, so a new grammar's compile is in this number: K30), **warm**
an immediate re-ingest of the unchanged tree into the same store. Store size is the sqlite file with
its WAL. Subjects at their pins (`calibration/corpus.d/astro.yaml`): withastro/docs whole (81 `.astro`
of a mostly `.mdx` tree), withastro/astro's `examples/` (91 `.astro`, 34 `.ts`), astro-paper v6.1.0
(the tuning subject, 27 `.astro`); gothinkster/angular-realworld-example-app build-50 (no `.astro`).

| Subject | Phase | Wall base -> branch | Peak RSS base -> branch | Store base -> branch |
|---|---|---|---|---|
| withastro/docs | cold | 1.83 s -> 2.19 s (+20%) | 260 -> 363 MiB (+39%) | 4,116 -> 4,160 KiB (+1%) |
| withastro/docs | warm | 0.28 s -> 0.35 s | 67 -> 69 MiB | (same store) |
| withastro/astro `examples/` | cold | 1.50 s -> 4.53 s (+202%) | 316 -> 533 MiB (+68%) | 3,136 -> 4,160 KiB (+33%) |
| withastro/astro `examples/` | warm | 0.13 s -> 0.15 s | 62 -> 63 MiB | (same store) |
| astro-paper | cold | 0.97 s -> 1.39 s (+43%) | 252 -> 341 MiB (+35%) | 1,204 -> 2,112 KiB (+75%) |
| astro-paper | warm | 0.10 s -> 0.07 s | 58 -> 59 MiB | (same store) |
| realworld (no `.astro`) | cold, 5 rounds | 1.11 s -> 1.16 s (+4.5%); CPU 1.03 s -> 1.06 s | equal | 1,024 -> 1,024 KiB |

On `examples/` the extract pass (`pass-times`) goes from 1.28 s to 4.02 s of wall at 3.76 s of CPU.
Its two halves measured apart do not add up to that: the 91 `.astro` files alone (copied out of the tree)
extract in 0.60 s to 0.65 s of CPU, compiles included, and the base extracts the rest in 1.28 s, so about
2 s is neither half's work. The cause, measured by elimination and read from the code: the module pool
holds at most six live grammar modules (`tsengine.MaxLiveModules`) keyed by extractor and variant
(`TSExtractor.leaseFor`), and a hosting extractor leases its injected TypeScript and CSS under its own
keys (F2's `hostedVariant`), so on a tree that also holds `.ts`, `.tsx`, `.mjs` and `.css` files the
JavaScript extractor's TypeScript, TSX and JavaScript modules, the CSS extractor's, and the astro
extractor's own three (astro, hosted TypeScript, hosted CSS) exceed six, and files read in directory
order evict and re-instantiate the 2 MB TypeScript module over and over. A tree with no `.astro` file never crosses the cap (realworld: no change); withastro/docs, whose 81
`.astro` files sit among 60 `.ts` and 3 `.css` files and no `.tsx`, reads +0.36 s cold (`examples/`: 34 `.ts`, 12 `.tsx` or `.jsx`, 10 `.css`).
**This part is not K27's new work: it slows the extraction of `.ts` files the base already did, on a
mixed Astro tree, so under K26 it ships only with an owner ruling or a fix.** The fix is a foundation
change outside this item's files (a hosted range leasing the injected dialect's module under a key
shared with that dialect's own extractor, or a cap that counts a host's modules), and it serves Vue and
Svelte hosts too. The harness, which reads one file at a time, never crosses the cap: astro.json's
tenancy is 1.23 MB per CPU-second over both subjects (181 ms of CPU for 172 files), max 4 ms a file,
peak 376 module pages.

## Why K27, and what is disclosed

K27: a cost gate reads against a baseline that did the work; when the baseline's output for the
measured scope was empty (a skipped pass), the cost is read where the pass already worked and the
other is disclosed as previously skipped. No repository's `.astro` files were ever extracted before
this change, so there is no such repository: the table above is the disclosure, and the realworld row
is the reading on a repository the change could only have slowed by touching other work (it does not).
Binary size: 81,499,746 -> 81,600,354 bytes (+100,608, the astro blob's 61,599 bytes gzip among it).

## K26 fix, re-measured at integration (LB-F550, LB-F526)

K26 fix, 2026-10-11: a component host (Astro, Vue, Svelte) reads its hosted TypeScript, JavaScript and CSS ranges on the pooled parser of the pipeline's own `.ts`, `.js` and `.css` extractor (`extractor.LinkHostedParsers`, wired by `fusion.New`; a range shares a parser only when the peer compiles the very queries it runs), so a tree mixing components with those files holds one grammar module per variant; withastro/astro's `examples/` ingests cold in 1.65 s where e32430b5 took 4.82 s, against 1.26 s at c39875f3 (+31%: CPU 0.94 s to 1.51 s, the 91 components' own extraction, 0.47 s to 0.53 s of CPU alone, K27), and the output is byte-identical to e32430b5's on every subject below (observations, their calls, typed edges, file imports).

Measured 2026-10-11 at integration: `fathom` built at c39875f3 (origin/main), at e32430b5 (wave lf7's merge) and at
e32430b5 with the fix, same machine, interleaved, three rounds, cold first ingests into empty stores, medians; machine
load 13 to 19 from other waves throughout. In process, the mixed fixture of `TestHostedRangesShareTheirDialectsParsers`
(24 files of each of six kinds) instantiated 146 grammar modules before the fix and 5 after, the same 5 as at 2 files.

| Subject | Wall c39875f3 / e32430b5 / fix | CPU (user) | Extract-pass CPU | Peak RSS |
|---|---|---|---|---|
| withastro/astro `examples/` | 1.26 / 4.82 / 1.65 s | 0.94 / 4.56 / 1.51 s | 0.93 / 4.67 / 1.50 s | 317 / 556 / 472 MiB |
| withastro/docs | 1.24 / 1.52 / 1.43 s | 1.04 / 1.32 / 1.21 s | 1.06 / 1.29 / 1.16 s | 264 / 363 / 296 MiB |
| astro-paper | 0.70 / 1.00 / 0.80 s | 0.62 / 0.84 / 0.74 s | 0.57 / 0.79 / 0.69 s | 252 / 357 / 269 MiB |
| sveltejs/kit (F.69) | 4.00 / 5.79 / 4.02 s | 2.86 / 5.05 / 3.12 s | 2.70 / 4.89 / 2.89 s | 393 / 573 / 441 MiB |
| element-plus (F.69) | 4.88 / 7.47 / 6.99 s | 4.04 / 6.32 / 5.79 s | 3.88 / 6.04 / 5.53 s | 463 / 663 / 585 MiB |

What remains over c39875f3 is the components' own extraction, new work on files the base skipped (K27, disclosed): the
91 `examples/` components alone extract in 0.43 s to 0.52 s of CPU and element-plus's 1,008 `.vue` files alone in 2.45 s
to 2.9 s, each at or above its tree's whole remaining delta. A tree with no component file is untouched (only a host
leases a peer's parser). Store is unchanged from e32430b5 by construction (the output is identical).
