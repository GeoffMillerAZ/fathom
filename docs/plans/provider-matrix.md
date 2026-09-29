# Provider matrix — every kit lane reachable from the compile seam

`buildLLMClient` (cmd/fathom/compile.go) switches on `FATHOM_LLM_PROVIDER`
and today knows `anthropic` and `claude-cli`. The kit is landing three more
lanes (agentic-kit docs/plans/{aws-credchain,azure-openai,codexcli-adapter}.md,
owner-ordered 2026-07-16): a full AWS credential chain under bedrock (SSO,
shared profiles, credential_process, IMDSv2, expiry-aware refresh), Azure
OpenAI in the openai adapter, and a codexcli adapter riding ChatGPT-plan
OAuth. fathom consumes all of them plus the already-shipped vanilla openai
adapter, completing the matrix: two API-key families, two subscription-CLI
families, and Bedrock/Azure enterprise endpoints. Fathom will serve
Codex-based customers; the codex-cli lane is a product requirement, not a
convenience.

Scope note: the seam governs the compile path (CLI `--store` mode and the
serve-http daemon) — the only LLM construction site in production.
`internal/llm` enrichment has zero production callers (verified 2026-07-16)
and stays out of scope until a consumer revives it.

Env contract (all read at construction, never mid-run):
- `openai` — `OPENAI_API_KEY`; optional `FATHOM_OPENAI_BASE_URL` for
  Ollama/vLLM/compatible hosts (key optional for local hosts, matching the
  adapter).
- `azure-openai` — `AZURE_OPENAI_API_KEY`, `AZURE_OPENAI_ENDPOINT`
  (resource base URL), `AZURE_OPENAI_DEPLOYMENT`, optional
  `AZURE_OPENAI_API_VERSION` (defaults to the adapter's pinned GA const).
  Missing endpoint or deployment → typed construction error naming the
  variable.
- `bedrock` — no fathom-specific env: the kit chain resolves explicit env
  keys, `AWS_PROFILE`/shared files (incl. `aws sso login` sessions),
  credential_process, and IMDSv2, in that order. `FATHOM_COMPILE_MODEL`
  must carry a Bedrock model or inference-profile ID; the error for a
  missing/rejected model names that requirement.
- `codex-cli` — rides the local `codex login` (ChatGPT plan); optional
  `FATHOM_CODEX_BIN` path override mirroring `FATHOM_CLAUDE_BIN`.
- Unknown provider values keep failing with the typed error, now listing
  all six lanes.

## Tasks

- [x] `openai` branch: construct the kit openai client from env; base-URL override honored {id: PM-1, symbol: buildOpenAIClientProfile, package: cmd/fathom}
- [x] `azure-openai` branch: endpoint/deployment/api-version wiring with typed missing-var errors {id: PM-2, symbol: buildAzureOpenAIClientProfile, package: cmd/fathom}
- [x] `bedrock` branch: kit chain-resolver construction; no static keys required; region resolution delegated to the kit {id: PM-3, symbol: buildBedrockClientProfile, package: cmd/fathom}
- [x] `codex-cli` branch: codexcli adapter with bin override, mirroring the claude-cli branch {id: PM-4, symbol: buildCodexCLIClientProfile, package: cmd/fathom}
- [x] Seam dispatch covers all six lanes; unknown-provider error text lists them; per-branch construction tests (API lanes construct-level with env fixtures, CLI lanes through fake shims) {id: PM-5, symbol: buildLLMClientForProfile, package: cmd/fathom}
- [x] Docs: provider matrix table in docs/use-cases/coding-agents.md §5 (auth mode, env vars, marginal-cost story per lane); CHANGELOG entry; kit go.mod bump to the commit carrying all three kit slices {id: PM-6, symbol: buildLLMClientForProfile, package: cmd/fathom, skip_test: true, skip: "docs + dependency-pin deliverable anchored to the seam it documents; prose is not index-provable"}

## Deferred

- [-] Failover lane composition (subscription-first → API-key) via the kit's failoverClient {id: PM-7, disposition: deferred, reason: "kit primitive exists and composes at the WithLLM call site in minutes; not ordered, and compile workloads tolerate a manual provider flip today", trigger: "first plan-limit exhaustion observed mid-compile, or owner orders resilient lanes"}
- [-] Embedding provider seam (FATHOM_EMBEDDING_PROVIDER) {id: PM-8, disposition: deferred, reason: "embeddings-first retrieval is a stated non-goal (docs/vision/01); vectors are reserved as an additive column (D-EM3)", trigger: "grep-miss taxonomy shows intent-search misses that joins cannot answer"}
