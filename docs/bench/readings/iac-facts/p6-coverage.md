IF-P6 verdict: PENDING (sampling rule written 2026-11-04 before any lockfile was read; results follow below once measured)

# IF-P6 lockfile coverage

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
