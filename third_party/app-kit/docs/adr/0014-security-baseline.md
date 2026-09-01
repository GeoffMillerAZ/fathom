# ADR-0014: Set the security baseline: token-bootstrapped loopback, eyes-open CSP, keychain secrets, signed updates

- **Status:** Accepted
- **Date:** 2026-06-10
- **Decision drivers:** the Loopback Invariant (ADR-0005) opens a real TCP
  port on every launch; Datastar makes `'unsafe-eval'` non-negotiable
  (ADR-0003); agents read everything (D15), so plaintext secrets are
  radioactive; the threat model is single-user machines, and pretending
  otherwise produces enterprise theater.

## Context

Every kit app listens on `127.0.0.1:0` ([ADR-0005](0005-loopback-invariant.md)).
A loopback port is not private: any process running as any local user can
connect to it, and a malicious *web page* can attack it from inside the
user's browser via CSRF or DNS rebinding. The research behind ADR-0005 and
ADR-0003 flags this explicitly: the loopback workaround "needs a per-session
auth token so other local processes can't hit the API."

Datastar evaluates `data-*` expressions through the `Function` constructor,
so `script-src` **must** include `'unsafe-eval'`; there is no nonce path for
expressions. The official guidance: escape user input in expressions, fence
untrusted DOM with `data-ignore`, keep sensitive data out of signals (they
are plain text and client-modifiable), validate server-side.
(https://data-star.dev/reference/security, Datastar v1.0.2, 2026-06-02.)

Wails v3 ships a built-in `app.Updater`: pluggable providers (GitHub
Releases, keygen.sh, Sparkle AppCast, custom), sha256/signature
verification, atomic binary swap
(https://github.com/wailsapp/wails/blob/master/docs/src/content/docs/guides/updater.mdx,
verified 2026-06-10). `wails3 setup signing` stores signing credentials in
the OS keychain.

The MCP go-sdk shipped DNS-rebinding and cross-origin validation patches in
v1.4.1 (2026-03-13); v1.6.1 (2026-05-22) includes an `MCPGODEBUG` escape
hatch that weakens those protections
(https://github.com/modelcontextprotocol/go-sdk/releases).

Licensed material is in play: the TailwindPlus corpus (336 MB dump and
everything derived from it, [ADR-0010](0010-component-data-pipeline.md)) and
Datastar Pro ($349 solo lifetime), whose license forbids use in open-source
projects and public redistribution (https://data-star.dev/pro, verified
2026-06-10).

**Threat model, honestly scoped.** We defend against: other local processes
poking the loopback port; malicious web pages mounting CSRF/DNS-rebinding
attacks against loopback servers; untrusted content rendered into pages;
tampered updates; secrets leaking through config files, repos, or agent
context. We do **not** defend against: a compromised OS user account
(same-user malware wins by definition), hostile multi-tenant deployment, or
targeted attackers with local execution. Kit apps run on the owner's
machines for the owner; the baseline is sized to that and nothing more.

## Decision

**1. Loopback trust bootstrap.** Each launch generates a random 256-bit
bearer token (32 bytes from `crypto/rand`, base64url-encoded, single-use,
120s TTL). The shell opens
`http://127.0.0.1:<port>/auth/launch?token=<t>`; the `GET /auth/launch`
exchange trades the token for a session cookie — `HttpOnly`,
`SameSite=Strict` — invalidates the token (strictly single-use), and
redirects to a clean URL so the token never lingers in history. Every
subsequent request requires the cookie. Kit middleware additionally rejects
any request whose `Host` is not exactly `127.0.0.1:<port>` — `localhost` is
rejected; shells always open `127.0.0.1`, never `localhost` — killing DNS
rebinding against the app server itself. The listener **refuses to bind
non-loopback addresses** unless the `app.cue` cloud profile explicitly
configures it — and that profile requires a real auth provider (its design
is deferred per the ROADMAP's continuous/deferred cloud-deployment entry;
this ADR covers local shells only).

**2. CSP, eyes open.** Default policy: `script-src 'self' 'unsafe-eval'`
(with `default-src 'self'` as the surrounding posture). `'unsafe-eval'` is a
**permanent** Datastar requirement — the Function constructor is how
expressions work — and we accept it deliberately rather than pretend a
future flag removes it. What makes it acceptable: no third-party script ever
loads (D2 — JS exists only as vendored `go:embed` artifacts), templ's
context-aware escaping covers interpolation ([ADR-0004](0004-templ-templates.md)),
and `'self'` still blocks injected external script sources.

**3. Untrusted content is fenced.** Any HTML not authored by us (clipboard
imports, fetched feeds, user-pasted content) renders inside a
`data-ignore` subtree so Datastar never evaluates attributes within it, and
is escaped by templ as text unless explicitly sanitized. Sensitive values
never go into signals.

**4. CSRF.** `SameSite=Strict` on the session cookie, plus a custom request
header (`X-AppKit-Cmd: 1`) required by kit middleware on every
state-changing request; the kit's templ helpers emit Datastar actions
configured to send it. Cross-origin HTML forms can set neither, so the pair
closes CSRF without token plumbing in every form.

**5. Secrets live in the OS keychain.** Behind the capability interface of
ADR-0005, with per-shell implementations (Keychain on macOS, Credential
Manager on Windows; env/secret-manager injection in headless cloud).
Plaintext secrets in config files are forbidden — `app.cue` and config hold
keychain item *references*, never values. This is a D15 consequence as much
as a security one: agents read and quote config files; they must have
nothing quotable.

**6. Updates are signed or they don't happen.** The only code-update path
on desktop is the signature-verified `app.Updater` (GitHub Releases provider
+ SHA256SUMS, per [ADR-0006](0006-wails-v3-desktop-shell.md)). The kit
exposes no other download-and-execute mechanism. Browser-tab and cloud tiers
update by deploy, not in-app.

**7. The Assistant's transport posture.** `appkit-mcp` runs stdio by
default — no network surface at all. HTTP mode (`-http`) uses the go-sdk's
`StreamableHTTPHandler` with its origin and DNS-rebinding protections
exactly as shipped (≥ v1.6.1, carrying the v1.4.1 patches); we never set
`MCPGODEBUG` or otherwise disable them, and we track SDK patch releases
because that surface is where its CVEs land.

**8. Licensed data stays home.** The TailwindPlus corpus, every artifact
derived from it, and all Datastar Pro code live only in private repos. If
any part of the kit is ever opened, a CI check proves these are absent —
Pro's license forbids OSS embedding, and we structure the repo so compliance
is mechanical, not vigilance.

**9. No telemetry.** Kit apps phone home for exactly one thing: the update
check. No analytics, no crash reporting, no usage pings — there is one user
and he has the logs.

Backup and restore are explicitly out of this baseline's scope — they are
owned by `appkit/store` (pre-upgrade snapshot + restore helpers,
[ADR-0009](0009-sqlite-default-store.md)).

## Alternatives considered

- **"localhost is trusted" — no auth on the loopback port.** Simplest, and
  common in dev tools. Lost because it is false: any local process and any
  malicious web page can reach loopback, and the app exposes real
  capabilities (files, secrets, commands). The token costs ~50 lines once,
  in the kit.
- **Unix domain sockets instead of TCP.** No port, no local-process
  exposure. Lost because webviews and browser tabs cannot navigate to a
  socket — it breaks the browser-tab tier and the entire shell model of
  ADR-0005.
- **TLS/mTLS on loopback.** Defense-in-depth on paper. Lost because cert
  provisioning on user machines is real ceremony (self-signed certs make
  webviews balk) for negligible gain on a loopback interface where the
  adversary already runs as the user. Token + cookie gets the same access
  control without the apparatus.
- **Strict CSP without `'unsafe-eval'`.** The "correct" posture by audit
  checklist. Lost because it is impossible with Datastar — no nonce path
  exists for expressions — and abandoning Datastar relitigates ADR-0003.
  Cost of the winner: strict-CSP web deployments are foreclosed and every
  scanner will flag us; we answer with this ADR.
- **Encrypted secrets file instead of keychain.** Portable, no OS APIs.
  Lost because the encryption key has to live somewhere, which recreates
  the problem one layer down; the OS keychain is the platform's answer and
  the capability interface already exists. Cost: per-OS implementations and
  occasional keychain prompts.
- **Crash reporting / telemetry.** Genuinely useful even solo. Rejected:
  crashes happen on the owner's machine where the owner can debug them; the
  no-phone-home property is worth more than remote stack traces.

## Consequences

- Easier: every app gets the full baseline by importing the kit — token
  bootstrap, CSP headers, CSRF middleware, keychain capability, updater
  wiring are written once. Security review of a new app reduces to "did you
  bypass the kit?"
- Harder: the token bootstrap adds a redirect to first paint (negligible on
  loopback); keychain capabilities need per-OS code and degrade explicitly
  in headless mode; `'unsafe-eval'` must be re-justified to every scanner
  forever (we link this ADR).
- Risk: the token's brief life in a URL (process args, history before
  scrub). Accepted: single-use + immediate exchange + same-machine scope
  makes interception equivalent to already running as the user. Tripwire:
  any shell that logs launch URLs persistently → switch that shell to a
  different handoff.
- Risk: `SameSite=Strict` + custom header assumes all commands flow through
  kit helpers. Tripwire: the first integration needing cross-origin calls
  into a kit app → design a scoped exception, never a blanket one.

## Revisit when

- Datastar ships a CSP-compatible expression path (none announced as of
  v1.0.2, 2026-06-02) — drop `'unsafe-eval'` the same week.
- Any kit app deploys multi-user on the public internet — that app gets a
  real auth ADR; this baseline does not stretch to cover it.
- The Assistant is ever exposed beyond localhost — adopt the go-sdk OAuth
  story (stabilizing since v1.5.0, 2026-04-07) before doing so.
- The kit gains a second human user with their own machines — the
  single-owner assumptions in the threat model expire.
