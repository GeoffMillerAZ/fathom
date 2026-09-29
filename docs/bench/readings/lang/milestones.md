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
