IF-P6 verdict: FAIL. A catalog of the 12 newest stable versions binds 3% to 60% of public lockfile selections, under the 80% bar for all five providers.

# IF-P6 lockfile coverage

## Summary

Exact-version binding on a recent-N catalog would rarely fire for the three cloud providers. Public repositories lock old
versions, spread across hundreds of releases: for `aws`, 80% of selections need the newest 242 to 258 of 512 stable
releases. Only a catalog that holds essentially every release binds (100% by construction, since every locked version was a
real release). Kubernetes and Helm are close at N = 50 (90% and 100%). The design's section 5 therefore needs a different
coverage or binding rule, or a catalog whose per-release cost is small enough to hold every release. IF-P1's size per N
was not available to this lane, so the cost of "every release" is not measured here.

## Numbers

Sample A: 250 repositories (pool 6,583 repositories, 3 skipped), 560 selections, 135 repositories pushed within 365 days.
Sample B: 60 repositories per provider (pools 1,170 to 1,401; 1 skip, google). Registry versions fetched 2026-11-04.
Source of the graded figure: A for aws (115 selections) and azurerm (33), B for google (A had 29), kubernetes (A had 24) and
helm (A had 19). Format: bound share (bound/selections).

| Provider (graded) | N = 12 | N = 50 | every release | stable releases | A at N=12 / 50 | B at N=12 / 50 |
|---|---|---|---|---|---|---|
| aws (A) | 13.9% (16/115) | 18.3% (21/115) | 100% | 512, newest 6.72.0 | 13.9% / 18.3% | 10.0% / 16.7% |
| azurerm (A) | 3.0% (1/33) | 27.3% (9/33) | 100% | 413, newest 5.12.0 | 3.0% / 27.3% | 3.3% / 30.0% |
| google (B) | 5.0% (3/60) | 23.3% (14/60) | 100% | 435, newest 8.10.0 | 6.9% / 24.1% | 5.0% / 23.3% |
| kubernetes (B) | 48.3% (29/60) | 90.0% (54/60) | 100% | 96, newest 3.4.0 | 50.0% / 91.7% | 48.3% / 90.0% |
| helm (B) | 60.0% (36/60) | 100% (60/60) | 100% | 65, newest 3.3.0 | 52.6% / 100% | 60.0% / 100% |

Wilson 95% intervals at N = 12 (graded): aws 9 to 21, azurerm 1 to 15, google 2 to 14, kubernetes 36 to 61, helm 47 to 71.
All five fail the bar at N = 12 with the upper bound of the interval under 80%.

Smallest N of newest stable versions that reaches 80% (A / B): aws 258 / 242 of 512, azurerm 180 / 206 of 413, google 254 /
189 of 435, kubernetes 41 / 45 of 96, helm 28 / 25 of 65.

Informational, not graded:

- Pushed within 365 days only (A / B, N = 12 and 50): aws 27%, 36% / 22%, 37%; azurerm 8%, 62% / 7%, 55%; google 13%, 47% / 8%,
  39%; kubernetes 70%, 100% / 77%, 96%; helm 75%, 100% / 88%, 100%. Recent repositories help, and the three cloud
  providers still fail at N = 12.
- A catalog of the 12 most-selected versions (fit to the same data, an upper bound): aws 39% (A) and 42% (B), azurerm 48% and
  47%, google 45% and 37%, kubernetes 75% and 70%, helm 100% and 90%.
- Latest patch of each minor does not shrink the problem: aws has 450 minors for 512 stable releases, azurerm 361 of 413,
  google 375 of 435, kubernetes 59 of 96, helm 32 of 65. At that set aws binds 96%.
- Selections are spread: 82 distinct aws versions in 115 selections (A). The most common single aws version is 5.100.0
  (19 of 115, the last 5.x). By major in A: 5.x 46, 4.x 30, 6.x 26, 3.x 12, 2.x 1.
- Unlisted and pre-release selections: none of the five providers had either.
- Source hosts in A: 502 `registry.terraform.io`, 55 `registry.opentofu.org`, 3 other (local or third-party). The first two
  were counted together, as the rule says.

## What it means for the design

- Exact-version binding with a recent-N catalog fails the bar, as the previous design did. Locked versions trail the newest
  by hundreds of releases.
- Three ways forward the owner-architect can weigh. None was tested here.
  1. Hold every release: needs IF-P1's per-release cost under blob deduplication to be small. IF-P1 decides this.
  2. Bind to the operator's own export: `fathom iac import` (design section 4) builds the catalog from the version the repo
     actually uses, so coverage is the share of repositories where an operator can run their own provider. This is not
     measured here.
  3. A different binding rule for unbound versions (nearest version within a major, labelled, with a disclosure): the design
     rejects a silent nearest version, and a labelled one is untested here.
- Kubernetes and Helm bind at N = 50 (90% and 100%). The failure is specific to the fast-moving cloud providers.

## Limits

- The pool is what code search exposes: relevance order inside a band, 1000 hits per band, 300 read per band. It is not a
  uniform draw of GitHub.
- Lockfiles are read at the current default branch, not at a past commit.
- Tutorial copies stay in the sample. One lockfile per repository and two repositories per owner cap the larger orgs.
- "Every release" reads 100% because every selected version is in the registry list. It measures no coverage risk by itself.
- N = 12 and 50 use the newest stable versions on 2026-11-04. A catalog built earlier would fare worse on newer locks.
- IF-P1's size per N was not available when this was written. The size side of the bar is unmeasured.

## Method and pins

Pool, walk and analysis scripts were throwaway Python (not committed) driving `gh api` (code search and contents
endpoints), run on 2026-11-04. Order key `sha256("ifp6|" + owner/name)`. Version lists from
`registry.terraform.io/v1/providers/hashicorp/<type>/versions`. The rule below was committed before any lockfile was read
(commit before the first fetch; the rule text is unchanged since).

## What this reads

Exact-version binding (design section 5) answers nothing when no catalog holds the version a repository locked. This reading
asks: of the provider versions that public repositories actually commit in `.terraform.lock.hcl`, what share would a
catalog of N versions bind? Bar (design section 10): a catalog within IF-P1's size budget binds at least 80% of selections
for the first five providers (`hashicorp/aws`, `azurerm`, `google`, `kubernetes`, `helm`).

## Sampling rule (frozen before reading any lockfile)

Sample date 2026-11-04. Listing hits (repository and path) does not read a lockfile. The rule is not changed after data.

Sample A, provider-agnostic, at least 200 repositories:

1. Pool: GitHub code search `filename:.terraform.lock.hcl size:LO..HI` over fixed size bands: 100-byte bands from 1 to 3000,
   then 3001..4000, 4001..6000, 6001..10000, 10001..20000, over 20000. Pages 1 to 3 (300 hits) per band. Keep hits whose
   file name is exactly `.terraform.lock.hcl` (the `filename:` qualifier also matches `x.terraform.lock.hcl.backup`).
2. Repository key is lowercase `owner/name`. Order repositories ascending by `sha256("ifp6|" + key)`. At most 2 repositories
   per owner, taken in that order.
3. One lockfile per repository: the hit with the fewest path segments, then the lexicographically first path.
4. Walk the order, fetch each lockfile at the default branch, skip it when the fetch fails, the file exceeds 256 KiB, or no
   provider block parses. Count every skip. Stop at 250 kept repositories.
5. A selection is one (repository, provider source, version) per `provider` block. Source hosts `registry.terraform.io` and
   `registry.opentofu.org` are stripped to `namespace/type` and counted together (IF-P2 measures whether the builds differ).
   Other hosts are kept apart and are not graded.

Sample B, per provider, for providers that Sample A leaves thin: for each of the five providers P, the same steps 2 to 4 over
the pool from code search `filename:.terraform.lock.hcl "hashicorp/P"` (bands 1..2000, 2001..3000, 3001..4000, 4001..6000,
6001..10000, over 10000; pages 1 to 3), stopping at 60 kept repositories whose parsed lockfile contains P.

Which sample grades a provider: Sample A when it holds at least 30 selections of that provider, else Sample B. Both are
reported for every provider.

## Version sets

Version lists come from the public registry (`registry.terraform.io/v1/providers/hashicorp/<type>/versions`) on the sample
date. A catalog of N is the N highest stable versions (no pre-release suffix) by semantic-version order. "Every release" is all
listed versions, pre-releases included. A selection is bound by a catalog when its version is in the set. A selected version
the registry does not list is counted as unlisted and is unbound under every N.

## Grade

Per provider, per N in 12, 50 and every release: bound share of selections with a 95% Wilson interval. Verdict:

- PASS: every one of the five providers binds at least 80% of its graded selections at N = 12 (point estimate).
- FAIL: any provider binds under 80% at N = 12. The reading then names the smallest N that reaches 80% per provider.
- INCONCLUSIVE: a provider has under 30 graded selections even in Sample B.

N = 50 and every release are reported beside IF-P1's size per N when that reading exists. A size that is not measured is
stated as not measured, never assumed.

Informational, not graded: the smallest N reaching 80% per provider; the share when the catalog holds the N most-selected
versions (an upper bound fit to the same data); the share among repositories pushed within 365 days of the sample date.

## Known limits of the rule

Code search returns hits in its own relevance order inside a band and caps a band at 1000, so the pool is what search
exposes, not a uniform draw of GitHub. Lockfiles are read at today's default branch. Repositories that copy a tutorial
lockfile are not removed.
