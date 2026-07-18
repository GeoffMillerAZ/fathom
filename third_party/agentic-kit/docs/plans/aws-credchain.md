# AWS credential chain for bedrock — close the SSO gap

The bedrock adapter resolves credentials from explicit `Config` fields or four
env vars only (`bedrock/config.go` ~L144-158) — no SSO, no shared
config/credentials files, no `credential_process`, no IMDS, no expiry-aware
refresh. Anyone using `aws sso login` (the modern default) cannot use the
adapter at all, and anyone on temporary credentials gets silent signature
failures at expiry. Named consumer: fathom's compile path (bedrock branch of
`FATHOM_LLM_PROVIDER`, spec'd fathom-side in docs/plans/provider-matrix.md);
owner ordered the gap closed 2026-07-16.

Design constraints (kit DNA, keep them):
- **No aws-sdk-go dependency.** The adapter hand-rolls SigV4 deliberately;
  the chain resolver is stdlib + the kit's own HTTP discipline. The SSO leg
  is one HTTPS GET against the SSO portal; IMDSv2 is two HTTP calls; ini
  parsing is ~50 lines. All hermetically testable.
- **Subpackage, not a kit-wide foundation**: `foundations/llmclient/bedrock/awscred`.
  It graduates to a shared foundations package only when a second consumer
  names it (D8) — record that as the graduation trigger in the package doc.
- Resolution order mirrors the AWS default chain: explicit Config → env vars
  → shared config/credentials files (`AWS_PROFILE`, `AWS_CONFIG_FILE`,
  `AWS_SHARED_CREDENTIALS_FILE` respected): static keys, `credential_process`
  (exec, JSON contract, stderr surfaced on failure), SSO (`sso_session` and
  legacy inline `sso_*` profile forms; cached token from
  `~/.aws/sso/cache/<sha1>.json`; `GetRoleCredentials` against
  `portal.sso.<region>.amazonaws.com`; an expired/absent cached token returns
  a typed error whose text names the fix: `aws sso login --profile <p>`) →
  IMDSv2 last (token PUT + creds GET, ≤1s timeout, honors
  `AWS_EC2_METADATA_DISABLED`).
- Credentials carry `Expiry time.Time` (zero = static). The bedrock client
  re-resolves within a 5-minute margin of expiry — cached, mutex-guarded,
  injected clock (`now func() time.Time`), never `time.Now()` in logic.
- Region: env (`AWS_REGION`/`AWS_DEFAULT_REGION`) then profile `region` key;
  explicit `Config.Region` still wins.
- Hermetic tests only: temp HOME with fixture ini files, a fake
  `credential_process` script, `httptest` servers for the SSO portal and
  IMDS. No live AWS calls anywhere.

## Implementation notes (2026-07-16)

Landed as specified; two clarifications worth recording rather than leaving
implicit:

- **SSO token-cache lookup order (the spec asked for "both lookups,
  document which is tried first"):** for a modern `sso_session` profile, the
  SHA-1 of the **session name** is tried first (the AWS CLI/SDK's own
  derivation for that form), falling back to the SHA-1 of the **start URL**
  second — the referenced `[sso-session <name>]` section already carries the
  start URL, so this fallback is free and covers a token cached by a
  differently-behaved tool. A legacy inline profile (no `sso_session`) has
  only the start-URL hash to try. Documented on `resolveSSO`'s doc comment
  and the package doc.
- **`credential_process` is exec'd via stdlib argv tokenizing
  (`splitCommandLine` + `exec.CommandContext`), not a shell.** The design
  constraints said "stdlib + the kit's own HTTP discipline" but didn't
  specify shell-vs-argv for this leg; argv tokenizing (matching the real AWS
  CLI/SDK's own behavior) avoids shell-metacharacter surprises from a value
  that ultimately comes from a config *file*, not a trusted caller, at the
  cost of not supporting shell pipelines/redirection in a
  `credential_process` value — no real-world credential_process tool needs
  that, so this is a strict improvement, not a narrowing of the spec.
- **AC-5's existing-tests caveat required one addition, not a modification:**
  `client_test.go` gained a `TestMain` that pins a hermetic
  `HOME`/AWS-env baseline for the whole package (temp `HOME`,
  `AWS_EC2_METADATA_DISABLED=true`, every `AWS_*` var cleared) before
  `m.Run()`. Every existing test's *assertions* are byte-for-byte unchanged
  (two lines swapped `c.creds` → `c.credCache.creds`, a field rename with the
  same values, not a behavior change); the addition exists because the
  chain now legitimately consults the shared config/credentials files and
  IMDS when explicit/env credentials are incomplete, and without a hermetic
  baseline the three `Missing*` subtests would non-deterministically pick up
  a developer machine's real `~/.aws` (confirmed: they failed exactly this
  way against this machine's real AWS setup before the `TestMain` was
  added).

AC-7 and AC-8 are untouched by this change-set — no `role_arn`/`AssumeRole`
code exists anywhere in `awscred`, and the package stays nested under
`foundations/llmclient/bedrock` (no top-level `foundations/awscred`).

### Critic follow-up (2026-07-16, second commit)

The post-landing critique found five should-fixes; three shared one root
cause. All applied, TDD:

- **Explicit/env legs are now atomic and short-circuiting (the root fix).**
  A complete explicit pair resolves BEFORE any ambient state is consulted —
  no file read, no `credential_process`, no SSO cache, no IMDS — so an
  unreadable (permission-denied) `~/.aws` can never break an explicit- or
  env-credentialed caller. A **partial** pair (exactly one half set, on
  either leg) is a hard typed `*PartialCredentialsError`, never completed
  from the next leg and never fallen through: the first commit's per-field
  cfg-or-env mixing would have let `Config{AccessKeyID: "k"}` with a
  populated `~/.aws` silently sign as the default profile's identity —
  request-level misattribution. The behavioral guarantee, stated plainly:
  **explicit credentials never fall through to ambient state; partial
  explicit (and env) pairs hard-error** — matching both the AWS SDKs and
  the pre-awscred adapter (which errored on an incomplete pair). This
  retires the first commit's cfg-key+env-secret mixing support
  (`TestResolve_MixedExplicitAndEnvFields` replaced by the partial-pair
  hard-error tests). File-leg read errors other than IsNotExist remain hard
  failures for chain users who genuinely reach the file legs — now
  explicitly pinned by test in both directions.
- **`credential_process` stderr capped at 4 KiB at capture time**
  (`boundedBuffer`: bytes past the cap are counted and dropped, never
  buffered), with a truncation marker in the message — a tool tracing
  itself with `set -x` can no longer leak echoed credentials into
  `CredentialProcessError` and thence logs. The errors.go comment claiming
  the caller bounded it (nothing did) is fixed.
- **`credential_process` is time-bounded by default**: 1 minute
  (aws-sdk-go-v2 processcreds parity) when the caller's ctx has no earlier
  deadline, plus `cmd.WaitDelay` (2s) so a grandchild holding the output
  pipes cannot block `Run` past the kill. A hung tool now bounds — never
  hangs — `bedrock.New` and per-request re-resolution.
- **Unparseable `credential_process` `Expiration` is a typed error**, not
  silently-static: ignoring the parse failure cached the credential forever
  — the exact silent-signature-failure-at-expiry mode this plan exists to
  kill. RFC3339 is the accepted format (the ISO8601 profile the contract's
  own examples use) and the error text names it and the bad value.
- **SSO**: a 200 with empty `roleCredentials` is a typed `*SSOPortalError`
  (never a partial `Credentials` + nil error, per `Resolve`'s contract);
  non-200/non-401/403 portal failures use the same type. Token-cache
  lookup-order doc aligned with behavior: **first cache match wins — a
  token found-but-expired under the session-name hash does not fall through
  to the start-URL hash** (both filenames name the same logical session; a
  second at-best-equally-stale copy would only mask the `aws sso login`
  fix).
- **Test hygiene**: `ssoCacheKey` vector pins the exact lowercase hex
  digest (not just length 40); the bedrock `TestMain` was restructured so
  its temp-HOME cleanup actually runs (deferred inside a helper returning
  the exit code — a defer next to `os.Exit` never fires); the IMDS
  wall-clock assertion loosened to 2.5s (the `ok=false` check is the real
  guard; the wall clock only catches gross regressions, and the old 1.3s
  bound was CI-flake territory); the credentials-file-wins-over-config-file
  precedence (which existed unpinned in `loadSharedProfile`) is now pinned
  by test.

Accepted as-is per the critique: legacy botocore `UTC`-suffix cache
timestamps hard-erroring (old-CLI edge), `splitCommandLine`'s
no-backslash-escapes (documented choice), AuthError-on-refresh-failure
failover semantics.

## Tasks

- [x] Chain resolver: `Resolve(ctx, cfg) (Credentials, error)` walking explicit → env → shared files → IMDS, returning source-labeled credentials with expiry {id: AC-1, symbol: Resolve, package: foundations/llmclient/bedrock/awscred}
- [x] Shared-file leg: ini parsing of config+credentials files, profile selection, static keys and `credential_process` {id: AC-2, symbol: loadSharedProfile, package: foundations/llmclient/bedrock/awscred}
- [x] SSO leg: token-cache read + GetRoleCredentials, typed expired-token error naming `aws sso login` {id: AC-3, symbol: resolveSSO, package: foundations/llmclient/bedrock/awscred}
- [x] IMDSv2 leg: token PUT + role-creds GET, 1s timeout, AWS_EC2_METADATA_DISABLED honored {id: AC-4, symbol: resolveIMDS, package: foundations/llmclient/bedrock/awscred}
- [x] Bedrock client consumes the resolver: expiry-aware cached re-resolution replaces the static env read; existing explicit-Config behavior byte-preserved {id: AC-5, symbol: New, package: foundations/llmclient/bedrock}
- [x] Region resolution from profile when env absent {id: AC-6, symbol: resolveRegion, package: foundations/llmclient/bedrock/awscred}

## Deferred

- [-] Graduation to a shared foundations/awscred package {id: AC-7, disposition: deferred, reason: "one named consumer (bedrock); D8 forbids speculative generality", trigger: "a second kit package or fleet repo needs AWS credential resolution"}
- [-] AssumeRole / role-chaining (`role_arn` profiles) {id: AC-8, disposition: deferred, reason: "needs STS AssumeRole signing; no named consumer uses role-chaining profiles", trigger: "first consumer whose ~/.aws/config routes through role_arn"}
