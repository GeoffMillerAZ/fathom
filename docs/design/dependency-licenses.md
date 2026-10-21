# Dependency licenses: map every dependency's license and enforce an org's policy at the edit

Status: proposed 2026-10-21 (the owner's idea, the owner-architect's design). Plan: `docs/plans/dependency-licenses.md`
(DL-1 to DL-11). Roadmap: `docs/vision/38-proof-layer-roadmap.md` section 9.11. Built after RT-M1 PASS. An ADR records
sections 3, 4 and 7 when wave 1 starts.

## 1. The problem

Many organizations forbid whole families of licenses in what they ship: network copyleft (AGPL, SSPL), often strong
copyleft (GPL), non-commercial terms, and anything unidentified. Where a check exists today it runs in CI or in a
vendor scanner, after a pull request opens. A coding agent adds dependencies faster than a person, and nothing stops
it at the moment it edits a manifest.

Fathom has no license data for adopters. It already enforces a license allowlist on its own inputs: embedded grammars
(ADR-048 decision 6, I55) and the kit mirrors (ADR-052 decision 4). This design offers the same control to adopters.

## 2. What fathom adds, and what it does not build

Fathom does not build a license detector from scratch, a license database, a network lookup or a legal verdict. It
reuses SPDX license identifiers and expressions, Google's `licensecheck` (pure Go, BSD-3-Clause) to identify LICENSE
texts, and an SPDX or CycloneDX SBOM that the operator names (the output of Syft, ScanCode or ORT).

What fathom adds that a CI scanner lacks:

1. **At the edit.** A guard check denies a manifest edit that adds a disallowed dependency, before the agent builds on
   it (DL-7).
2. **Reach.** Each dependency says whether shipped code, only tests, or nothing imports it, and which files would change
   to drop it (DL-6).
3. **One policy across the fleet.** The org sets it once through a platform-module binding, and every indexed repository
   answers to it (DL-4, DL-10).
4. **Receipts.** Each license names its source (file, field, classifier score) and the policy entry that judged it.

## 3. Sources: on disk only (I56)

The lockfile or manifest gives each dependency's name, version and scope (`internal/analysis/deps`, unwired today). The
license comes only from what the toolchain already placed on disk, found through the dependency plane's cache locator
(SG-36):

| Ecosystem | Declared license | License text |
| --- | --- | --- |
| Go | none, modules carry no field | `$GOMODCACHE/<module>@<version>/LICENSE*` |
| npm, pnpm, yarn | `license` in the installed `package.json` | `node_modules/<package>/LICENSE*` |
| Cargo | `license` in the registry copy's `Cargo.toml` | the same directory |
| Python | `License-Expression` (PEP 639) or classifiers in `METADATA` | `*.dist-info/licenses/` |
| Maven, Gradle | `<licenses>` in the cached `pom.xml` | rarely shipped |
| Ruby | `licenses` in the installed gemspec | the gem directory |

Order of evidence: a declared SPDX expression, then a LICENSE text that `licensecheck` matches above its coverage
threshold, then an entry in an operator-named SBOM (DL-9). When the declared expression and the text disagree, the
answer shows both and resolves neither. A dependency with nothing on disk reads `unknown (not on disk)` and is never
fetched.

## 4. The policy: configuration (ADR-011)

A `dependencies.licenses` section joins `#Config` (`internal/config/config_schema.cue`). An org sets it through a
platform-module binding, and a repository narrows it under scoped-config precedence (law 13):

```toml
[dependencies.licenses]
allow   = ["MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC"]
review  = ["MPL-2.0", "LGPL-2.1-only", "LGPL-3.0-only"]
deny    = ["AGPL-3.0-only", "AGPL-3.0-or-later", "SSPL-1.0", "GPL-3.0-only"]
unknown = "review"     # allow | review | deny
scopes  = ["runtime"]  # dependencies reached only from tests are exempt unless "test" is listed

[[dependencies.licenses.exceptions]]
package = "pkg:npm/example-lib@2.1.0"
reason  = "legal approved, ticket 1234"
expires = "2027-03-31"
```

Evaluation is deterministic. An `OR` expression passes when any branch passes, `AND` needs every branch, and a `WITH`
exception is part of the identifier. Each dependency gets one verdict, `allow`, `review`, `deny` or `unknown`, plus the
identifier and the policy entry behind it. An expired exception stops applying and says so. The section is named
`dependencies` so advisory policy (SG-41, SG-42) can join it later at no cost now.

## 5. Where it is served

No new MCP tool (P4). The verdicts reach agents and people through surfaces that exist:

- **The guard** (`.fathom/guard.cue`, `checks: ["dep_policy"]`). An edit to a recognized manifest is parsed before and
  after. A `deny` on an added dependency blocks the edit and names the dependency, its license and the policy entry. A
  `review` lets it through with a note. Off by default, and it fails open like every hook.
- **`review_diff`** gains a dependency dimension: dependencies added, removed or re-versioned, each with its verdict
  (DL-8).
- **`fathom deps licenses`** prints the inventory and verdicts for one service, as text or JSON. `--check` exits 1 on any
  `deny`, so CI can gate on it (DL-5).

## 6. Reach

Fathom already knows which files import a package. A verdict carries that reach: imported by shipped code, imported
only by tests, or declared and never imported. `scopes` decides whether test-only reach counts. Dropping a denied
dependency starts from the files that import it, and `blast_radius` over them answers what else moves. SG-38's usage
map deepens this to typed references when it lands.

## 7. Non-goals

- **No legal verdict or compatibility judgment.** License questions are never judged (ADR-060 decision 6). Every answer
  says it is evidence for the org's policy, not legal advice.
- **No network lookup and no license database kept by fathom** (I56).
- **No snippet provenance scanning.** Copied code inside a repository is ScanCode's job.
- **No advisory policy here.** SG-41 and SG-42 own vulnerabilities.
- **No replacement for CI scanners.** Fathom imports their SBOMs.

## 8. Freeze and sequencing

Wave 1 derives on read and adds no table, column, cache or background job (ADR-049 decision 2). A persisted license
fact waits for RT-M1 PASS and for DL-M2 to show that reading from disk is too slow. The work shares SG-36's cache
locator with wave 4's vuln reach.

## Decision models

None applies. License identification is deterministic: a declared SPDX expression, or a `licensecheck` match above its
coverage threshold, and below it the answer is `unknown`, never a guess. A model reading a license text or judging
compatibility would judge a license question, which ADR-060 decision 6 forbids, and would serve judgment as fact
(I62). The measurement that proves the deterministic path is DL-M1.
