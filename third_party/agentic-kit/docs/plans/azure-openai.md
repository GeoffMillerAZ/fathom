# Azure OpenAI — close the endpoint-shape gap in the openai adapter

The openai adapter (`foundations/llmclient/openai`) covers vanilla OpenAI and
OpenAI-compatible hosts (Ollama, vLLM) via `Config.BaseURL`, but Azure OpenAI
changes three things at once and is currently unusable: auth header is
`api-key: <key>` (not `Authorization: Bearer`), routing is by deployment name
in the URL path (`/openai/deployments/{deployment}/chat/completions`), and an
`api-version` query parameter is mandatory. Named consumer: fathom's compile
path (`azure-openai` branch of `FATHOM_LLM_PROVIDER`, spec'd fathom-side in
docs/plans/provider-matrix.md); owner ordered the gap closed 2026-07-16.

Design constraints:
- **Extend the existing package, do not fork it.** The wire mapping
  (`wire.go` request/response bodies, tool_call accumulation, typed errors,
  SSE streaming) is 100% reusable; only auth-header construction
  (`setAuthHeaders`) and endpoint construction (`do`/stream URL building)
  vary. Azure support = new `Config` fields + a branch in those two seams.
- `Config` gains `AzureDeployment string` and `AzureAPIVersion string`
  (default to a current GA version const; document where the default is
  pinned and that callers on newer API features override it). Non-empty
  `AzureDeployment` selects Azure mode: deployment-path URL,
  `api-version` query param on every request (completions AND the models
  ping), `api-key` header. `BaseURL` becomes the resource endpoint
  (`https://<resource>.openai.azure.com`).
- Entra-ID bearer auth: when Azure mode is combined with the existing bearer
  scheme, send `Authorization: Bearer` instead of `api-key` — one `if`, and
  it covers managed-identity tokens minted outside the kit. No token minting
  in the kit.
- Azure's error envelope matches OpenAI's `{"error":{...}}` shape for the
  status codes the adapter maps; assert that in a conformance test rather
  than assuming it (one fixture per mapped status).
- Model-in-body: Azure ignores it (deployment routes). Keep sending it when
  set — harmless, and keeps `Rung.Apply`/model-catalog behavior uniform —
  but document the routing reality on the Config field.
- Same test rigor as the existing suite: table-driven URL/header building,
  api-version presence on every path (including Ping), streaming through the
  Azure path, key redaction in errors.

## Tasks

- [x] Azure config surface: `AzureDeployment`/`AzureAPIVersion` fields, GA-version default const, validation (deployment without version → default; version without deployment → typed InvalidRequestError) {id: AZ-1, symbol: Config, package: foundations/llmclient/openai, skip_test: true, skip: "struct type anchor — behavior is pinned by the AZ-2/AZ-3 URL/header tests which construct Configs directly"}
- [x] Azure endpoint construction: deployment path + api-version query on completions, streaming, and ping URLs {id: AZ-2, symbol: buildEndpoint, package: foundations/llmclient/openai}
- [x] Azure auth: `api-key` header in Azure mode; bearer scheme passthrough for Entra tokens {id: AZ-3, symbol: setAuthHeaders, package: foundations/llmclient/openai, skip_test: true, skip: "method on Client shared with the vanilla path; ambiguous method-call edges are the documented O9 residue — behavior pinned by direct header-assertion tests in client_test.go"}
- [x] Azure error-envelope conformance: fixture-asserted mapping for 400/401/429/5xx through the Azure path {id: AZ-4, symbol: mapHTTPError, package: foundations/llmclient/openai, skip_test: true, skip: "reached only through Client.Complete/do, whose method names are defined by every adapter — the documented O9 ambiguous-method-edge residue severs the static test path one hop up; behavior is pinned by this package's direct shim/fixture tests"}
  (contract repair at merge: the spec anchor named mapAPIError, a symbol that never existed — the real, cross-adapter-consistent name is mapHTTPError; the implementation targeted reality and the anchor now matches)
- [x] doc.go Azure section: endpoint shape, auth modes, api-version pinning doctrine, what is NOT handled (no token minting, no region failover) {id: AZ-5, symbol: New, package: foundations/llmclient/openai, skip_test: true, skip: "docs deliverable anchored to the constructor its examples exercise; prose is not index-provable"}

## Deferred

- [-] Azure Entra token acquisition (managed identity / client-credential flows) {id: AZ-6, disposition: deferred, reason: "token minting is an auth product, not an adapter concern; bearer passthrough covers externally-minted tokens", trigger: "a consumer that cannot mint tokens out-of-band names the need"}
