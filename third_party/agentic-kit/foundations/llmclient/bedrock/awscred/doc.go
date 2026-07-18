// Package awscred implements the AWS default credential-resolution chain for
// the Bedrock adapter (foundations/llmclient/bedrock): explicit Config
// fields, then the conventional AWS_* environment variables, then the shared
// config/credentials files (static keys, credential_process, and SSO — both
// the modern sso_session form and the legacy inline sso_* form), then
// IMDSv2 last. Resolve is the single entry point; every other exported and
// unexported symbol in this package supports one leg of that chain.
//
// # Resolution order
//
//  1. Explicit — Config.AccessKeyID/SecretAccessKey (+ Config.SessionToken).
//  2. Environment — AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY/AWS_SESSION_TOKEN.
//  3. Shared files — AWS_PROFILE (default "default") selects a profile from
//     the shared config file (AWS_CONFIG_FILE, else ~/.aws/config) and
//     credentials file (AWS_SHARED_CREDENTIALS_FILE, else
//     ~/.aws/credentials). Within this leg: static aws_access_key_id/
//     aws_secret_access_key first, then credential_process (executed via
//     stdlib argv tokenizing — no shell is invoked, see splitCommandLine —
//     time-bounded, with captured stderr capped so a misbehaving tool cannot
//     leak secrets into error messages), then SSO (see below). A missing
//     file at any of these paths is treated as an empty document, not an
//     error — the common case this whole leg exists to degrade gracefully
//     past is a machine with no AWS CLI ever configured. A file that exists
//     but cannot be READ, by contrast, is a hard error: only IsNotExist is
//     forgiven, so a chain user genuinely depending on this leg hears about
//     an unreadable config instead of silently skipping to IMDS.
//  4. IMDSv2 — the EC2 instance-metadata service, token-based flow only
//     (IMDSv1's tokenless GETs are never attempted), honoring
//     AWS_EC2_METADATA_DISABLED=true. Bounded to a ~1s timeout per call: off
//     an EC2 instance this leg is expected to fail fast, not hang the whole
//     chain.
//
// Legs 1 and 2 are atomic and short-circuiting: a complete pair on either
// leg resolves immediately, before ANY ambient state (files,
// credential_process, SSO cache, IMDS) is consulted — so explicit and
// env-credentialed callers can never be broken, or silently re-identified,
// by whatever happens to live in ~/.aws. A PARTIAL pair on either leg
// (exactly one half set) is a hard *PartialCredentialsError, never
// completed from elsewhere and never fallen through: silently signing as
// the ambient default profile's identity instead of the principal the
// caller half-named is request-level misattribution, and the AWS SDKs
// hard-error on it too. See Resolve.
//
// Region resolves independently of credentials, in its own priority order:
// explicit Config.Region, then AWS_REGION, then AWS_DEFAULT_REGION, then the
// selected profile's "region" key (the profile leg only when the shared
// files were consulted at all — a short-circuited explicit/env resolution
// never reads them). See resolveRegion.
//
// # SSO
//
// A profile names SSO either the modern way (sso_session pointing at a
// [sso-session <name>] section carrying sso_start_url/sso_region) or the
// legacy way (sso_start_url/sso_region inline on the profile itself, no
// sso_session key). Either way, the resolved access token is read from the
// on-disk cache AWS CLI/SDK tools write on `aws sso login`:
// ~/.aws/sso/cache/<sha1-hex>.json. The two forms hash different inputs into
// that filename — see resolveSSO's doc comment for the exact lookup order —
// and an absent or expired cached token returns a typed *ExpiredSSOTokenError
// whose message names the exact fix (`aws sso login --profile <name>`)
// rather than a bare "not found." A live token is exchanged for role
// credentials via one HTTPS GET (GetRoleCredentials) against the SSO portal;
// Config.SSOPortalBaseURL overrides that origin for tests.
//
// # Expiry
//
// Every Credentials carries Expiry (zero for static/explicit/env credentials
// that never expire on their own). Resolve does not itself judge expiry —
// callers holding onto a resolved Credentials past a construction-time
// snapshot (as the bedrock Client does) are responsible for re-invoking
// Resolve before Expiry, using an injected clock rather than time.Now so
// expiry judgment is deterministic in tests.
//
// # Concurrency
//
// Resolve and every unexported helper in this package are pure functions of
// their arguments plus ambient OS state (env vars, the filesystem, the
// network) — there is no package-level mutable state, so concurrent callers
// are safe. A caller that wants a single cached, expiry-aware credential
// across concurrent use (the bedrock Client's use case) must add its own
// synchronization around the cache, not around Resolve itself.
//
// # No aws-sdk-go dependency
//
// Every leg is hand-rolled on the standard library, deliberately: ini
// parsing (~50 lines), one HTTPS GET for the SSO leg, two HTTP calls for
// IMDSv2, and a stdlib os/exec invocation (argv-tokenized, no shell) for
// credential_process. This mirrors the parent bedrock package's SigV4 and
// event-stream framing, which are hand-rolled for the same reason.
//
// # Package placement (D8)
//
// This package is nested under foundations/llmclient/bedrock, not a
// standalone foundations/awscred, because it has exactly one named consumer
// (the bedrock adapter) and D8 forbids speculative generality: a shared
// top-level location is premature until a second consumer — another kit
// package or a fleet repo — names a need for AWS credential resolution
// outside Bedrock. That is the graduation trigger; until it fires, this
// package stays exactly where its one consumer lives.
package awscred
