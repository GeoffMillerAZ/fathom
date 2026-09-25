# CV-44: Review Kit armed at warn on this repository (2026-09-25)

**Verdict (plain language). CV-44's mechanical arming works, and a real
sounding proves two of the three requirements genuinely fire at warn — but
the third, `testing.certified-targets`, currently reads a false "ok" here
for a reason worth fixing before anyone trusts it: review-kit's
`fathom-certify` adapter never passes `--service`, so it guesses the
service ID from the sounding directory's basename. This clone is named
`cv3-arming`, not `fathom`, so it silently queried an empty service and
found nothing to flag — and the SAME defect class will recur at the real
checkout, whose directory is literally named `sextant`, not `fathom` (this
repository's own code already names this failure mode as O15: "the
sextant→fathom rename left this repo's dir basename pointing at a service
that does not exist, silently killing obligations injection for six
days"). `fathom-conserve`'s adapter was already fixed to thread `--service`
explicitly (TC-18, platform-engineering commit `323e69f`); `fathom-certify`
was not. Until it is (or `certify gaps` grows the same daemon-backed
`resolveIndexedCandidate` identity resolution other fathom commands use
instead of a directory-name guess), `testing.certified-targets` cannot be
trusted to fire on this repository from its real working directory. The two
`fathom-conserve`-backed requirements have no such gap (their adapter
already names `--service` explicitly) and both fired correctly, at warn, on
a real historical range from this repository's own history.**

Bar (CV-44, `docs/plans/refactor-conservation.md`): "Review Kit's
`refactoring-conservation` pack and `fathom-conserve` adapter, armed at
warn on this repository (platform-engineering)." **Result: armed, wired,
and independently confirmed to fire — with the `testing.certified-targets`
service-identity caveat above recorded for whoever lands the fix.**

## What landed

Platform-engineering (`cv-envelope` branch, read-only checkout):
`f52b4f8` (test-kit `#EnvInfo` gains `commit`/`dirty`/`race`, ADR-0106),
`11d25d9` (review-kit `fathom-conserve` pack; surveyor runs carry
commit/dirty/race), `323e69f` (fix: thread `--service` through the
`fathom-conserve` adapter). Built `leadsman` from that checkout
(`cd kits/review-kit && go build -o /Volumes/m2ext/tmp/cv3-tools/leadsman
./cmd/leadsman`) — a plain `go build`, no vendoring changes needed.

In this fathom clone (no `.reviewkit/` existed before this change — CV-44
is the first time Review Kit is armed here at all, not a re-arm):

1. `leadsman catalog install refactoring-conservation` and
   `leadsman catalog install testing-certifications` — wrote
   `.reviewkit/packs/{refactoring-conservation,testing-certifications}/pack.cue`,
   the shipped catalog bytes verbatim, every requirement still `severity:
   "off"`.
2. `.reviewkit/config.cue` — `subject.service: "fathom"`, a `catalogs`
   entry pointing at `.reviewkit/catalog` (required: `#ProjectConfig`'s
   `catalogs` list must be non-empty, and this repository has no local
   Standard of its own), and `packs: ["refactoring-conservation",
   "testing-certifications"]`.
3. `.reviewkit/catalog/standard.cue` — a valid, empty local `#Standard`
   (`concepts: {}`, `requirements: {}`) so `config.catalogs` has a source
   to flatten; every concept, requirement and finding this repository
   evaluates comes from the two packs, never a local one (ADR-011: no
   invented org content here either).
4. `.reviewkit/dispositions/refactoring-conservation.cue` — three
   `#Tailor` dispositions raising `refactoring.no-undeclared-loss`,
   `refactoring.proof-integrity` and `testing.certified-targets` from the
   packs' shipped `off` to `warn` (`refactoring.uncovered-behaviour` stays
   `off` — advisory typed-judgment lane, never a gate, per the plan and
   design doc; nothing is armed at `deny`).
5. Top-level `CODEOWNERS` — did not exist before this change. Required:
   `leadsman` refuses (loudly, naming the missing coverage) to load ANY
   disposition unless the repository's CODEOWNERS carries a matching rule
   for `.reviewkit/dispositions/` (ADR-0096 D3, verified against
   platform-engineering's own `internal/dispose/codeowners.go`). Added a
   rule for `.reviewkit/dispositions/`, `.reviewkit/catalog/` and
   `.reviewkit/packs/`, mirroring platform-engineering's own dogfood
   `CODEOWNERS` verbatim in shape.

Built `bin/fathom` from this clone (`go build -o bin/fathom ./cmd/fathom`,
HEAD `5afeee42`, `v0.8.0`).

## What fired, and at what severity

Every requirement below is armed at **warn** (confirmed in the rendered
`sounding.json` cells — `"severity": "warn"` on all three; none at `deny`).

| requirement | real sounding (`--since origin/work`) | historical demo (`--since 81098d39`, real scratch index) |
|---|---|---|
| `refactoring.no-undeclared-loss` | `ok` — no diff yet (HEAD = upstream) | **`fail`** — 4 undeclared findings |
| `refactoring.proof-integrity` | `ok` — no diff yet | **`fail`** — 1 finding |
| `testing.certified-targets` | `ok` — **false pass, see caveat below** | `ok` — **false pass, same cause** |

### The real sounding: `leadsman sound --since origin/work`

This clone's `origin` is another scratch checkout in this same wave, and
`HEAD` is already up to date with `origin/work` (no local commits yet, only
the new `.reviewkit/`/`CODEOWNERS` files, which are additions, not losses).
`--since origin/main` does not resolve here (no `main` ref in this scratch
chain); `origin/work` is the documented equivalent
(`docs/use-cases/refactor-conservation.md` §6: "the branch's own upstream,
falling back to `origin/main`" — exactly what the pre-push hook's own
`"$since"` resolution does). Result: all three requirements read `ok` —
correct and expected for a freshly-armed repository with nothing yet to
compare against. This is not a null test of the arming; it is the honest
first reading.

### Proving the packs actually fire: a real historical range

To confirm the wiring does something besides sit green, I ran a second,
demonstrative sounding against `81098d39` (the commit before this
repository's own CV-M6 dogfood refactor — `docs/bench/readings/cv-m6-dogfood-2026-09-24.md`
— retired 18 unused `internal/analysis/*` packages), using a real scratch
index (`fathom ingest --service fathom --store workspace/tmp/fathom-cv44.sqlite`,
0 failed packages, per this task's environment rule against ingesting
`workspace/db/*` — see "Store, honestly" below).

```
fathom conserve --json --since 81098d39 --service fathom \
  --store workspace/tmp/fathom-cv44.sqlite
```

produced a real, non-fabricated finding on the `integrity` plane:

```json
{
  "plane": "integrity", "kind": "deleted", "key": "TestAnalyze_SortedDescending",
  "detail": "gone at head; no test in the changed-file set has a matching normalized body (guarded by: reaches_changed_symbol)",
  "base": "internal/analysis/cognitive/cognitive_test.go:127",
  "declared": "none",
  "remedy": "restore the test, or add `retires:`/a disposition to the plan row that removed it"
}
```

(plus three informational `edited` findings on unrelated tests, unchanged
by any capability loss). `TestAnalyze_SortedDescending` lived in
`internal/analysis/cognitive` — one of CV-M6's 18 legitimately, ratchet-
proven, `gone:`-declared retirements. This is `refactoring.proof-integrity`
working exactly as ADR-044 decision 3 designs it: proof integrity gates
**ahead of and independent from** self-declaration, so a properly-declared
package retirement still surfaces its deleted guarded test for a human (or
a `#Tailor`/`#Dispute`) to look at, rather than being silently excused
because the surrounding refactor was otherwise honest. With `leadsman
sound --since 81098d39` (same scratch index copied to the CLI's own fixed
default path — see below), the rendered `GAPS.md` shows both:

```
| refactoring.no-undeclared-loss | 4 undeclared loss(es) since 81098d39…: integrity/deleted:TestAnalyze_SortedDescending, integrity/edited:TestCmdIngest_ScipExplicitFlagJSON, integrity/edited:TestImportIndex_MapsOccurrencesToTypedEdges, integrity/edited:TestImportIndex_PackageGranularReplacement |
| refactoring.proof-integrity    | 1 proof-integrity finding(s) since 81098d39…: deleted:TestAnalyze_SortedDescending |
```

Both requirements' `sounding.json` cells: `"raw": "fail"`, `"severity":
"warn"`. **Both fire at warn**, from a real conserve report, not a
constructed fixture. (Observation, not a defect claim: `no-undeclared-loss`
scans every plane's findings for `declared != "prior"`, `integrity`
included, so on this range it flags literally the same four findings
`proof-integrity` does — a real overlap in what the two requirements catch
on this repository's data, worth knowing about, not this task's to
resolve.)

### The `testing.certified-targets` false pass — a real, reproducible cause

`leadsman sound`'s `fathom-certify` adapter shells `fathom certify gaps
--json --kind invariant` with **no `--service` flag**
(`platform-engineering/kits/review-kit/internal/evidence/certify.go:152`).
`cmd/fathom certify gaps` then falls back to guessing the service from the
current directory's basename (`cmd/fathom/certify_gaps.go`:
`defaultTestsCoverageService()` → `serviceCandidates(cwd)[0].name` →
`filepath.Base(cwd)`). This clone's directory is `cv3-arming`:

```
$ fathom certify gaps --json --kind invariant   # no --service, from this clone
{"service":"cv3-arming", "summary":{"declared":0,"live":0,"gaps":0, …}}
```

Zero declared targets under a service nobody ingested — an honest-looking
`ok` that is actually "nothing was asked." Passing `--service fathom`
explicitly (what the adapter should do, and what its sibling
`fathom-conserve` adapter already does since `323e69f`) against the same
scratch index shows the real state:

```
$ fathom certify gaps --json --kind invariant --service fathom \
    --store workspace/tmp/fathom-cv44.sqlite
{"service":"fathom","index_commit":"5afeee42b78b…","summary":{"declared":45,"live":0,"gaps":45,"joined_cases":0,"cases":0}}
```

**All 45 of this repository's declared invariant certification targets are
gaps** — some never declared at all (`I1`: `"certificates":[]`), others
declared and joined but `unproven` (`I11`: a certificate exists for
`TestMcpToolsDocMatchesRegistry` at tier `pre-scan`, state `unproven`) —
exactly ADR-043's amended O114 state: "existing certificates read unproven
until runs are re-attested with a commit," and `task tests:attest` has
never run in this scratch clone. This is the honest, real gap count
`testing.certified-targets` should be reporting at warn on every sounding
here; it currently does not, because of the missing `--service`, and this
is the SAME failure class (`cmd/fathom/serviceident.go`'s own comment
names it: "the sextant→fathom rename left this repo's dir basename
pointing at a service that does not exist, silently killing obligations
injection for six days" — O15) that will recur at the real checkout, whose
directory is `sextant`, not `fathom`. **This is this reading's one
actionable finding**: `testing-certifications`'s `fathom-certify` adapter
needs the same `--service` fix `fathom-conserve` already got before this
requirement's warn-severity firing means anything at the real checkout.

## Store, honestly

Per this task's environment rule (never shell `fathom ingest`/`compile`
against `workspace/db/*` or any shared store; scratch stores under this
clone's `workspace/tmp/` only), the scratch index used above lives at
`workspace/tmp/fathom-cv44.sqlite` — never at the CLI's own default
(`workspace/db/fathom.sqlite`, `internal/statedir`/`defaultpaths.go`).
Neither `fathom-conserve`'s nor `fathom-certify`'s `leadsman` adapters
accept a `--store` flag (both hardcode the CLI's default path, by design —
"an already-built index," per their own doc comments), so the historical
demonstration above (only) briefly **copied** (never ingested/compiled
against) the completed scratch `.sqlite` file to `workspace/db/fathom.sqlite`
purely so `leadsman sound` could exercise the real end-to-end evaluation
path once, for this reading; that copy and the generated `docs/review/`
output were deleted afterward and neither is committed. The `origin/work`
sounding above needed no index at all (nothing changed since HEAD, so
every plane read `conserved` before any adapter subprocess mattered).

## Reproduce

```sh
export GOCACHE=/Volumes/m2ext/tmp/cv3-build GOMODCACHE=/Volumes/m2ext/tmp/cv3-mod \
       GOTMPDIR=/Volumes/m2ext/tmp/cv3-tmp TMPDIR=/Volumes/m2ext/tmp/cv3-tmp \
       GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 GOFLAGS=-mod=mod

# platform-engineering, branch cv-envelope (read-only)
cd <platform-engineering checkout>/kits/review-kit
go build -o /Volumes/m2ext/tmp/cv3-tools/leadsman ./cmd/leadsman

# this fathom clone
cd <fathom clone>
go build -o bin/fathom ./cmd/fathom
export PATH="$PWD/bin:$PATH"

/Volumes/m2ext/tmp/cv3-tools/leadsman sound --since origin/work   # the real, first sounding

mkdir -p workspace/tmp
fathom ingest --service fathom --store workspace/tmp/fathom-cv44.sqlite --path .
fathom conserve --json --since 81098d39 --service fathom \
  --store workspace/tmp/fathom-cv44.sqlite
fathom certify gaps --json --kind invariant --service fathom \
  --store workspace/tmp/fathom-cv44.sqlite
```
