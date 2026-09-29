# Language breadth: milestone counts (LB-M6)

Counts of languages and tool formats at each level, taken only from the
committed readings in this directory (`level_met` and `grade`); a
`tolerated` or `unsupported` tracker row counts as nothing. The tracker's
generated milestone line (`docs/languages.md`) is the gated reading; this
file records it per wave.

## Wave 1, 2026-09-28 (a15b29a3)

| Level | Languages | Tool formats |
|---|---|---|
| L1 | 2 — Python (excellent), Go (good) | 1 — Terraform (excellent) |
| L2 | 0 | 0 |
| L3 | 0 | 0 |
| L4 | 0 | 0 |

Measured below L1 (readings committed): TypeScript, JavaScript, HTML, CSS,
package.json, Python packaging (its L2 include-group metric graded good,
not claimable without L1).

Tracker line: `Milestone M1 progress: languages at L1 excellent 1/20, at L2
0/12, at L3 0/8, at L4 0/6; tool formats at target 0/15`.

## Wave 1b, 2026-09-28 (20304d19)

Highest `level_met` per committed reading.

| Level | Languages | Tool formats |
|---|---|---|
| L1 | 4 — Python (excellent), Go (good), HTML (good), Bash (good) | 2 — Terraform (excellent), package.json (excellent) |
| L2 | 0 | 4 — Dockerfile (excellent), GitHub Actions (good), Kubernetes (good), Python packaging (good) |
| L3 | 0 | 0 |
| L4 | 0 | 0 |

Measured below L1 (readings committed): C (its L1 metrics grade good, but
`level_met` is none on the throughput floor), Java, C#, SQL, JavaScript,
TypeScript, CSS. C++ has no reading (blocked before dialect work).

Tracker line: `Milestone M1 progress: languages at L1 excellent 1/20, at L2
0/12, at L3 0/8, at L4 0/6; tool formats at target 1/15`.

## Round 2, 2026-09-28 (eb96fcbc)

Highest `level_met` per committed reading after the owner's round-2 decisions (D1 CPU-time throughput, D2
stronger oracles, D3 oracle-deficiency disclosure and its audit, D5 an L2 calls oracle for every dialect, D7
wildcard imports; `docs/design/language-breadth.md` section 1.5). The grade in parentheses is the grade at the
level met; the L1 grade is added where it differs.

| Level | Languages | Tool formats |
|---|---|---|
| L1 | 6: C (good), C# (excellent), Go (good), HTML (good), Python (excellent), TypeScript (good) | 2: package.json (excellent), Terraform (excellent) |
| L2 | 3: Bash (good), Java (good; L1 excellent), SQL (good) | 2: Dockerfile (excellent), Python packaging (good) |
| L3 | 0 | 2: GitHub Actions (good), Kubernetes (good) |
| L4 | 0 | 0 |

Measured below L1 (readings committed): CSS, JavaScript. The D3 audit re-measured TypeScript from L1 excellent
to L1 good and JavaScript from L1 excellent to no level met
(`docs/bench/readings/lang/d3-audit.md`).

Tracker line: `Milestone M1 progress: languages at L1 excellent 3/20, at L2 3/12, at L3 0/8, at L4 0/6; tool
formats at target 3/15`.

## The lq wave, 2026-09-28 (6674b551)

Six items landed on the integration line after round 2, each from its committed reading:

| Item | Result | Commit |
|---|---|---|
| C++ | L1 excellent | `b93e5760` |
| PHP | L1 excellent | `9e1219c3` |
| Rust | L2 excellent | `00d6aad9` |
| PowerShell | L2 good (L1 excellent) | `83e2cafd` |
| Docker Compose | L2 excellent, L3 deferred | `668d5263` |
| Helm | L3 good | `e40cb0e5` |

Highest `level_met` per committed reading at `6674b551`:

| Level | Languages | Tool formats |
|---|---|---|
| L1 | 8: C (good), C++ (excellent), C# (excellent), Go (good), HTML (good), PHP (excellent), Python (excellent), TypeScript (good) | 2: package.json (excellent), Terraform (excellent) |
| L2 | 5: Bash (good), Java (good; L1 excellent), PowerShell (good; L1 excellent), Rust (excellent), SQL (good) | 3: Docker Compose (excellent), Dockerfile (excellent), Python packaging (good) |
| L3 | 0 | 3: GitHub Actions (good), Helm (good), Kubernetes (good) |
| L4 | 0 | 0 |

Measured below L1 (readings committed): CSS, JavaScript.

Tracker line: `Milestone M1 progress: languages at L1 excellent 7/20, at L2 5/12, at L3 0/8, at L4 0/6; tool
formats at target 4/15`.

LB-M2-1 (the wave 1 re-measure: every reading regenerated at HEAD, no `level_met` or grade regressed, the
wave's token spend per item recorded) is not met, because the D3 audit moved two readings down (TypeScript and
JavaScript); `waves.md` is therefore not written and LB-M2-1 stays open in `docs/plans/language-breadth.md`.

## Wave 2, 2026-09-29 (lq-integ 88fe67a5, after the integration review)

Highest `level_met` per committed reading; the grades are the per-level grades up to the level met. A
refused item (design §14.2) counts at no level whatever its reading measured.

| Level | Languages | Tool formats |
|---|---|---|
| L1 | 8: C++ (excellent), C# (excellent), PHP (excellent), Python (excellent), C (good), Go (good), HTML (good), TypeScript (good) | 2: package.json (excellent), Terraform (excellent) |
| L2 | 9: Dart (excellent, excellent), Rust (excellent, excellent), Java (excellent, good), Kotlin (excellent, good), Lua (excellent, good), PowerShell (excellent, good), Ruby (excellent, good), Bash (good, good), SQL (good, good) | 4: Docker Compose (excellent, excellent), Dockerfile (excellent, excellent), Makefile (excellent, excellent), Python packaging (good, good) |
| L3 | 0 | 8: Cargo (excellent ×3), Go modules (excellent ×3), MSBuild (excellent ×3), GitLab CI (excellent, good, good), Maven (excellent, good, good), GitHub Actions (good ×3), Helm (good ×3), Kubernetes (good ×3) |
| L4 | 0 | 0 |

At or above each level: languages 17 at L1 (11 excellent), 9 at L2, none at L3 or L4; tool formats 14
at L1, 12 at L2, 8 at L3. Tool formats at their target: 10 (the eight at L3, Makefile and Python
packaging at L2).

Measured below L1 (readings committed): CSS, JavaScript. Refused: Swift (its reading measures L2, L1
excellent and L2 good, but 5.6% of its calibration files parse with an error, over §14.2's 5% line;
`LB-Q-swift-rest`), so the worker's count of 18 languages at L1 (12 excellent) and 10 at L2 is 17 (11)
and 9.

Not all of the movement since wave 1b is wave 2's. Round 2's instruments (D1 CPU-time throughput, D2
oracles, D3 adjudication; readings committed before the wave's base, eb96fcbc) moved C, C# and
TypeScript to L1, Java and SQL to L2, Bash from L1 to L2, and GitHub Actions and Kubernetes from L2 to
L3. Wave 2's seventeen items added C++ and PHP at L1; Dart, Rust, Kotlin, Lua, PowerShell and Ruby at
L2; Docker Compose and Makefile at L2; and Cargo, Go modules, MSBuild, GitLab CI, Maven and Helm at L3.
C++, PHP and Rust are measured dialects not yet dispatched (their CGO incumbents stay live), and Helm's
and Docker Compose's dispatch stays on their incumbents (`LB-Q-helm-rest`, `LB-Q-docker-compose-rest`);
Maven's L3 holds for apache/maven's reactor POMs only, and Cargo's for rust-lang/cargo's workspace
manifests only.

The integration review's other adjudications left every level as the worker recorded it: Makefile's
two ctags corrections are oracle-bug fixes (L1 excellent kept), Kotlin's PSI dump is tier B by D2's
Roslyn precedent, Rust's shared SCIP rules are now scoped to rust-analyzer (no earlier reading moves),
and Dart's engine and scip-dart changes move no other golden (each reading carries its one-line
disposition). The critic pass kept Lua's luaparse dump at tier B, the one tier its reading flagged for
review, with PowerShell's and PHP's parser dumps (ctags is verified deficient for all three; design
§1.5 D2), so their L1 excellent grades stand.

Tracker line: `Milestone M1 progress: languages at L1 excellent 11/20, at L2 9/12, at L3 0/8, at L4
0/6; tool formats at target 10/15`.

## L3 re-grade, 2026-09-29 (regrade wave; harness 3929c383, LB-F40)

The harness computes `import_resolution` (LB-F40); each of the six languages whose L3 waited on it was re-run
with the oracle tools that exist on the measuring machine, on the pinned calibration subjects. No level and no
L3 row moved: the L3 grade also needs the two SCIP-derived file-edge metrics, and every one of the six fails or
lacks them.

| Language | `import_resolution` | Oracle and tier | L3 grade | Why L3 is not met |
|---|---|---|---|---|
| Ruby | 1 (144 of 144) | ruby-require, ruby 2.6.10, B (good at most) | unmet (was not attempted) | file edges: scip-ruby is absent here; its disclosed reading is .8 and .0124 against .90 and .25 |
| C++ | 1 (218 of 218) | clang-includes, Apple clang 21.0.0, B (good at most) | unmet | file_edge_precision .5309 and file_edge_recall .1114 against .90 and .25 |
| Rust | not measurable | rust-analyzer and cargo absent | unmet | file_edge_recall .0134 against .25 |
| PHP | not measurable | php, composer and scip-php absent | unmet | file edge precision and recall read 0 |
| Dart | not measurable | dart and scip-dart absent | unmet | file_edge_recall .108 against .25 |
| Kotlin | not measurable | kotlinc and scip-java absent; the PSI dump writes no imports | not attempted | no SCIP index with occurrence ranges, so no file-edge metric |

Swift stays refused (no import oracle, and its grammar is over §14.2's 5% line). The two measured numbers are
tier B, so each is "good at most" whatever it reads, and each carries a large unattested count that is the
oracle's blind spot (a `lib`-only load path for Ruby; an include the compiler does not enter for C++, O146). The
import counts were spliced into each recorded subject beside the families measured earlier (Ruby at 303b2bdb, C++
at eb96fcbc), because a fresh `lang-oracle` run without scip-clang or the tree-sitter-ruby wheel would have replaced
those families; regrading changed no other metric, and both readings' L1 numbers re-measure identically.

Tracker line, unchanged: `Milestone M1 progress: languages at L1 excellent 11/20, at L2 9/12, at L3 0/8, at L4
0/6; tool formats at target 10/15`.

## Import strategies, 2026-09-29 (O145 fixes 928ec49a and 9de28530; LB-F43 to LB-F45)

O145's three strategy gaps are fixed and `import_resolution` re-measured on round 2's SCIP indexes (tier A) and
pinned clones. No level and no L3 row moved: TypeScript and JavaScript still miss L3 on file_edge_recall, and
C# on `import_resolution` itself.

| Language | Subject | Before | After | L3 grade |
|---|---|---|---|---|
| TypeScript | zod | 0 (0 of 504) | 1 (504 of 504) | unmet (file_edge_recall .0224) |
| TypeScript | nest | 1 (3,271) | 1 (3,271) | |
| JavaScript | express | .3419 (53 of 155) | 1 (155 of 155) | unmet (file_edge_recall .0897) |
| JavaScript | axios | 1 (232) | 1 (232) | |
| C# | Polly | 0 (0 of 15) | 1 (15 of 15) | unmet (was not attempted): .0854 overall |
| C# | Newtonsoft.Json | 0 (0 of 313) | .0415 (13 of 313) | 300 name a type in a file the dialect parses without its namespace (O148) |

The counts were spliced into each recorded subject beside the families measured earlier; regrading changed no
other metric. The tracker is unchanged.
