# Prose integration evidence, 2026-10-24

`task validate` passed for the additive source, document runner, reasoning projection
and final evidence/date-parity unit. The actual four-consumer `task downstream-check`
passed in602.413 seconds, exit0, against committed library source
`1c0ceff3249a3de6901dc183d27b3b362416b81c`. [Receipt](receipt.json).

The five exported library-root trees are byte-identical in the later runner/projection
commits. Those later units add command-local tooling, tests and documentation;
they do not substitute a different exported library implementation for this test.

| Prepared consumer | Exact source | Observed coverage |
|---|---|---|
| Fathom |787c146237199ff3eda69ae1fee49d0b9ea624cf | Exact mirror pin, owned vendor check, CLI/all-package build, race suite and separate performance guard |
| consumer-a |a007075fab6ac156be44de46df941cce4a0915ed | All-package build/tests with `-mod=readonly`; scratch vendor synchronized for nested `go list` |
| consumer-b |8b8620936afd2ebcff6810baec1ef37528699c1a | All-package build/tests, including exact copied local fixtures, retained privately |
| consumer-c |04be1238bae18e9542b135060233412b4abd72ca | Candidate pseudo-version re-pin/re-vendor, build and declared KitVendor/NoGoWork/VendorTree guards |

The app-kit prerequisite is9d3da97554ae9202b0240fb6ef076181d7121219.
Historical consumer manifests needed scratch-only graph materialization/tidy,
including uniseg and the selected app-kit transitive graph. The receipt binds before/
after manifest hashes; private preparation patches retain exact differences. The
original source checkouts were never written by these checks. Existing dirty work
is preserved; no consumer dependency upgrade or full current-workspace pass is claimed.

The first aggregate passed Fathom but stopped on consumer-a's readonly manifest.
Its first prepared run then failed on stale vendor metadata in architecture tests.
Both failures remain retained. Synchronizing the disposable vendor tree repaired
the prerequisite; individual remaining targets passed, then the unchanged complete
aggregate ran again and passed. No assertions, threshold, gate, `go.mod` constitution
or tests were weakened. Performance used default CPU capacity; only Fathom correctness
tests received its supported cap. The optional app-kit reverse-check is explicitly
unconfigured in the disposable Fathom clone and is not a fifth consumer pass.

Go/CUE timestamp parity tests cover calendar errors, canonical fractional seconds,
zero values and UTC formatting. Structural/static checks, synthetic control tests,
real provider observations, independent quality and hosted CI remain different
instruments. The failed [real document cell](../2026-10-24-document/README.md) stays
failed. No full HP/PH/HW, production host, publication, UI or cost-saving claim follows.

## Retained root pre-push failure

The first normal push of1952d7f was refused by validation despite the earlier
worktree gates. The harness configuration fixture used macOS's aliased temporary
path while the production contract requires canonical roots. The same test failed
with `/var` and passed with its canonical temporary parent. During controlled
absolute-Git-directory reproduction, one conflict-creation test inherited that
variable and inserted its fixture's foreign blob into the caller's index. The full
index was backed up; only the test-created absent-from-HEAD path was removed.
No HEAD, working file, approval rule or hook was changed. INC-26 records this escape.

The test-only repair canonicalizes the fixture root and routes conflict creation
through the existing environment-isolating Git builder. New red/green cases create
their own temp alias and poison only a disposable foreign repository. They assert
that its index bytes stay exact. Focused race tests and the final full normal-root gate pass;
normal publication is a separate hook-enforced operation. This does not weaken production
canonical-path, source-policy or unresolved-index refusal.

Selected strict HP plan verification exited0 with10 tasks, zero claimed, zero
verified and zero failing. It does not establish new runtime or workload closure.
The source-level Fathom review checked5/8 dimensions; architecture rules, coverage
profiles and certifications were unconfigured. Its reachability/conservation
metadata complements actual tests and review; it is not a full security certificate.
