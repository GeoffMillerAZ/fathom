# Security

How fathom behaves on a machine, how to check each claim, and how to harden further.
Reporting a vulnerability: [SECURITY.md](../SECURITY.md). Decision record: ADR-083 in
[ADR.md](../ADR.md).

## Threat model

Fathom is a local, single-user tool. One user runs one daemon on their own machine, and the
daemon indexes repositories that user chooses to ingest.

- **Trusted:** the user, and every process running as that user.
- **Untrusted:** the network, other machines, and the contents of an ingested repository
  (a cloned repository may be hostile).
- **Daemon:** serves loopback only, and checks the peer address as well as the `Host` and
  `Origin` headers. A non-loopback peer gets 403 whatever `Host` it sends.
- **No authentication yet.** Any local process can call the loopback daemon, including
  `ingest`, `remove_service` and `rename_service`. A per-machine bearer token is designed
  and not built. It waits on open question **Q5** (`docs/intent/questions.md`): accept
  single-user in writing, or make hardening a goal before a second adopter. Until Q5 is
  answered, do not run fathom on a shared multi-user host.
- **Not a boundary:** MCP surfaces and obligations are guidance for agents, not access control
  (`docs/mcp-tools.md`).

## Where data is stored

| Data | Location |
|---|---|
| Index, ledgers, logs, daemon state | `~/.fathom/` (all of it moves with `FATHOM_HOME`; see `docs/usage.md`) |
| MCP call log | `~/.fathom/logs/mcp-calls.jsonl` |
| Repository config, guards, rules | `.fathom/` inside each repository |
| Build artifacts, scratch | `workspace/` in the repository (gitignored) |
| Daemon definition (macOS) | a launchd plist from `fathom daemon install` |

Nothing is sent anywhere to produce these. Index contents are derived from your source, so
treat `~/.fathom/` with the same sensitivity as the repositories.

## Egress paths

Fathom has no telemetry and no update check. Every way data or requests can leave the machine:

| Path | Default | Gate |
|---|---|---|
| LLM lanes (hosted APIs, plus `claude` and `codex` child processes) | Off until a provider is configured. An ambient `ANTHROPIC_API_KEY` or `FATHOM_LLM_PROVIDER` counts as configuring one | Used only by compile, smoke, providers and doctor pings |
| Decision port | Provider `none` | `FATHOM_DECIDE=off`, `~/.fathom/decide.off`; a provider named only by a repository needs user consent |
| Go module fetches (`go list` in the typed pass, ask, layer rules) | Offline: `GOPROXY=off`, `GOSUMDB=off`, `GONOPROXY=,`, `GOTELEMETRY=off`, `GOTOOLCHAIN=local`, `-mod=mod` stripped | `[ingest] go_network = true`, user config only |
| Bedrock `credential_process` | Off | Provider `bedrock` |
| `scip-typescript`, `scip-python` | Only during `init`, only if already on `PATH` | You run `init` |
| `receipt certify`, `doctor --stranger` | Not run | Explicit verbs, your environment |
| CUE module registry | Closed | Refused in code |
| git | Read-only commands | No fetch, clone or push in the binary |
| `FATHOM_HTTP` | Unset | You point a client at a host; the caller header goes only there |
| Claude Code plugin MCP | `http://localhost:7575/rpc/agent` | Loopback |

A test (`TestEgressImportAllowlist`) lists every package in the release binaries that imports
`net`, `net/http`, `os/exec` or similar, with its reason and gate, in
`cmd/fathom/testdata/egress-allowlist.txt`. A new edge fails `task validate`.

## Reviewer checklist

Each row is a claim and the command that checks it.

| Claim | Check |
|---|---|
| **Bind.** The daemon listens on loopback only | `lsof -nP -iTCP:7575 -sTCP:LISTEN` shows `127.0.0.1`. `fathom doctor` row `daemon_bind` passes. `daemon install` refuses a non-loopback `--addr`, and `serve-http` given one binds `127.0.0.1` on the same port instead and logs one warning. No flag or key widens the bind |
| **Peer check.** A network peer is refused | From another host: `curl -H 'Host: 127.0.0.1:7575' http://<host-ip>:7575/healthz` fails or returns 403 |
| **Auth.** There is none | `curl -s http://127.0.0.1:7575/healthz` answers with no credential. This is the known gap (Q5) |
| **Model calls.** None without a configured provider | `env -u ANTHROPIC_API_KEY -u FATHOM_LLM_PROVIDER fathom doctor`; `fathom config --explain` lists providers and the decide provider |
| **Telemetry.** None | `go list -deps ./cmd/fathom` and the allowlist file above; run the daemon under a firewall rule or `sandbox-exec` that blocks outbound and ingest still works |
| **Go module egress.** Typed pass is offline | Ingest a Go repository with `GOPROXY` set to a loopback listener that logs requests; it receives none. Missing modules are named with `run go mod download` |
| **CUE registry.** Closed | `internal/cuerefuse` tests; `go test ./internal/cuerefuse/...` |
| **Plugin.** Loopback MCP, hooks run local `fathom` | Read `plugins/fathom/.mcp.json` and `hooks/hooks.json`; `fathom plugin render` reproduces them byte for byte (gate test) |
| **Provenance.** The binary is the release | `sha256sum -c` against `checksums.txt`; `gh attestation verify <archive> --repo <owner>/fathom --signer-workflow <owner>/fathom/.github/workflows/release.yml` (attestations start after v0.23.0). `fathom version` reports the revision, or the module version and sum for a `go install` build |
| **Egress guard.** Advisory by default | `fathom doctor` row `egress_guard`; `[guards] egress` in `~/.fathom/config.toml` |
| **Data handling.** Store is local | `ls ~/.fathom`; set `FATHOM_HOME` to a scratch directory and confirm nothing else is written |

## The egress guard and its limits

Before a Bash command (Claude Code and Codex) or a Claude Code `WebFetch` or `WebSearch`,
fathom matches commands that send data off the machine: `curl`/`wget` uploads, `nc`/`socat`,
`scp`/`rsync`/`ssh` to a non-loopback host, `git push` to a remote off the allowlist,
`gh gist create`, and cloud storage uploads.

- `[guards] egress = "warn"` (default) adds one advisory line. `"enforce"` denies while guard
  enforcement is on. `"off"` silences it.
- `egress_allow` lists permitted hosts. The user sets it. A repository may tighten the mode,
  never loosen it.
- **Limit:** this is string matching, a tripwire. A script, `python -c` or an encoded payload
  gets past it. It is not data-loss prevention. Codex hooks see only Bash and `apply_patch`,
  so Codex web search and MCP calls are not intercepted. Claude Code's MCP fetch and browser
  tools are not intercepted either.

## Harden further

The harness boundary is the real one. Use it.

- **Claude Code:** `permissions.deny` for network tools you do not want (`WebFetch`,
  `Bash(curl:*)`), and the sandbox network allowlist so only named hosts are reachable.
- **Codex:** keep the sandbox network setting off (the default for workspace-write), and
  review hooks before trusting them with `/hooks`.
- **Fathom:** keep `go_network` unset, keep `[daemon].addr` on loopback, set
  `egress = "enforce"` with a short `egress_allow`, and set `FATHOM_DECIDE=off` if you want no
  decision-model calls at all.
- **Machine:** do not expose port 7575 through port forwarding, a dev container with host
  networking, or a tunnel. There is no authentication behind it.
