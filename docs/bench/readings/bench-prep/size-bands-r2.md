# Large-band search and remaining gaps

Historical round-2 screen. Current product-only C6 counts and dispositions are
in `c6-r2.json` and `README.md`; C5 estimates are superseded by `c5-r3.json`.

Measured on 2026-10-26 (UTC). The selection target was fixed by review: add a
75k–150k primary-language candidate per language where one exists. Trpc and
redux-toolkit cover TypeScript in `typescript-r2.md`; this reading adds Go and
Python. `size-bands-r2.json` binds the following numbers to full-history pins,
licence hashes, line counts, history estimates, commands and D24 witnesses.
No source or patch is committed. C4 remains unrun.

| Language | Candidate / pin | C1 | C2 lines | C5 co-changes / fix-message subset | D24 witnesses | Screen |
|---|---|---|---:|---|---:|---|
| Go | go-git/go-git / `bdae57cc427f94a6f7c627201bfe14508b5e19bd` | Apache-2.0 | 99,417 | 81 / 10 | 85 | refuse C3 |
| Python | pytest-dev/pytest / `082ed70e314ee8e1ab117763d17fe801648f7508` | MIT | 117,244 | 320 / 83 | 12 | retained draft |

The current go-git v6 head exceeded the size ceiling in the initial screen;
the latest v5 tag, v5.19.3, is the measured large-band candidate. Its frozen
Go module cache is warmed before `GOPROXY=off GOFLAGS=-mod=readonly go test
-count=1 -timeout=14m ./...`. Its warm-up reached `TestFetchExactSHA1`, which
hardcodes an upstream GitHub fixture URL; the request failed with network denied.
Other tests also failed because the shared mount does not preserve native Git
ownership semantics. The warm-up remainder was interrupted after those witnessed
failures. No complete warm-up time and no three timed trial results are claimed;
its refusal is scoped to the full suite in this isolation setup. No passing
subset was substituted. **Go's retained 75k–150k band remains empty.**

Pytest uses its upstream `uv.lock`, uv 0.13.5, `sync --frozen
--no-default-groups --group dev`, and the pinned Python 3.12.15 arm64 image.
The dev group includes `pytest[dev]`. The initial shared-mount warm-up had
ownership/hardlink failures; it is invalid preparation evidence, not a subject
refusal. The corrected command places `--basetemp` beneath native Linux tmpfs
at `/work/tmp/pytest-temp`. No subject file changed. The final full-suite result
and individual raw-log hashes follow.

Pytest final warm-up: 161.968 seconds / exit 0. Timed trials: 160.138s/exit 0, 165.92s/exit 0, 150.788s/exit 0.


C1 reads root licence blobs; C2 counts every tracked primary-language file
(including tests/generated/examples), with archive completeness checked against
the tracked tree. C5 uses the same fixed UTC window and word-bounded fix-message
surrogate, never a replayed-fix claim. C6 uses the committed resolver and D24
recount, excluding defining units. Go has 85 witnesses and pytest has 12.
C7 grepped full repository names and distinctive short names in calibration,
scenario tasks and bench readings. Go-git mentions in oracle module locks are
dependencies, and the `go-gitea` URL is a substring collision; generic pytest
runner mentions are not a read of pytest as a subject. No prior subject read
was found. Organization counts are one each; fleet exclusion remains unknown.

Commands are the `inventory.py`, `remeasure.py`, `go-c6`, `c6_python.py` and
`recount.py` invocations documented in the tool READMEs. These new retained
candidates still need D23 metadata before admission manifests can be written;
no stars, age, model cutoff or held-out split was invented. Native macOS,
twelve cores, actual C5 fix replay and post-cutoff yield after Q2, formal C4
acceptance and the live C7 fleet check remain unmeasured. The original reading's
five Python drafts and any new retained draft are preparation candidates, not
BX-44 subjects. Class-transformer/class-validator remain BX-M0-only.

Python now has a retained large-band preparation candidate (pytest).
