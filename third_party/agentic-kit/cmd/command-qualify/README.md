# Native file-command qualification

`command-qualify` runs a fixed, owned synthetic roster against a pinned
`command-guard` executable. `files-core/1` has 18 cases. `files-gitleaks/1`
adds nine cases for the exact `gitleaks-stdin-8.30.1` profile. The latter needs
an independently trusted scanner executable and its SHA-256. The command does
not install either program, change hooks, or attest provenance. Child processes
retain ordinary host authority. Use only executables you trust.

`files-loopback-fixture/1` runs the 18 core cases and 12 classifier client
boundary cases against an owned, ephemeral IPv4 loopback HTTP peer. The peer
returns fixed synthetic JSON and performs no inference. This profile needs no
scanner, endpoint, model, credentials, or caller-selected response. Its receipt
sets `syntheticPeerOnly: true` and contains no request or response bodies.
The peer derives the sole answer ID from the validated fixture state, criterion
and ordered choices using `decision.RequestKey`; it rejects an unkeyed or
unrelated ID.
Build its guard with `go build -tags command_classifier -o ABS ./cmd/command-guard`.
The ordinary guard intentionally excludes that experimental client and rejects
selected classifier controls before source/scanner work. Inspect `help`, then pin
and qualify the actual artifact. Use the ordinary build for core-only adoption.

Build the qualification command and candidate separately. Give each binary an
absolute path. Obtain each lowercase SHA-256 independently. On macOS/Linux,
`shasum -a 256 /absolute/path/to/binary` prints the hash. On Windows,
`(Get-FileHash -Algorithm SHA256 C:\absolute\path\to\binary.exe).Hash.ToLowerInvariant()`
does the same. These commands inspect files; they do not establish provenance.

```text
command-qualify run --profile files-core/1 --guard ABS --guard-sha256 HASH --format json
command-qualify run --profile files-gitleaks/1 --guard ABS --guard-sha256 HASH --scanner ABS --scanner-sha256 HASH --format text
command-qualify run --profile files-loopback-fixture/1 --guard ABS --guard-sha256 HASH --format json
```

`command-qualify help` (also `--help`, `-h`, or `run --help`) prints fixed
usage without opening files or running a candidate.
Invalid arguments return 2 and may emit no report bytes.

`json` is compact JSON. `text` is indented JSON. Exit 0 means every required
case passed, 1 means an observed oracle contradiction, and 2 means at least one
required case is unknown or report output failed. Read every ordered case result;
the aggregate alone does not identify the missing witness. The `rosterSHA256`
binds the versioned descriptor: ordered case IDs, fixture body hashes and selectors,
request kind/ranges/literal, fixture flags/limits, and variable expected rows,
counters and outcomes. It does not hash every serialized control byte or hardcoded
report expectation. Retain the runner binary hash and source revision to identify
the exact implementation. The public receipt carries only the descriptor digest.
Go validation additionally checks order and exact
semantic counters; `schema.cue` constrains the closed receipt shape and counts.

The command verifies and privately copies each executable (at most 128 MiB),
uses an owned working tree and a minimal child environment, limits each output
channel to 1 MiB, each child to 15 seconds, and the suite to five minutes. These
limits do not confine descendant processes or impose a filesystem deadline.
Only synthetic source fixtures are used. The fictitious scanner canary follows
the same seeded generator as `command-guard/testdata/scanner-canary.go`.

An actual native run covers only its reported host OS and architecture. Other
host combinations, real repositories, installed hooks, production interception,
and general absence of sensitive content need separate evidence. The loopback
fixture witnesses native HTTP framing, response handling, disclosure holds and
receipt counters for fixed cases. It does not qualify IPv6 runtime behavior,
Ollama, an installed model, semantic judgment, or residency and egress.
