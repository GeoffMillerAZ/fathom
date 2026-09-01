// Package expertchat is the governed answer loop for chatting against
// fathom "expert modules" — a module host serving citation-bearing claims
// over HTTP JSON-RPC (docs/spec/expert-module-v0.md, docs/spec/
// store-format-v0.md §3 Server conformance, in the fathom repo). It
// composes:
//
//   - foundations/llmclient    — the model turn (real Anthropic client in
//     production, MockClient/replay in tests — see the package doc for
//     record/replay cassettes)
//   - foundations/review/scoring — the evidence-tier vocabulary
//     (scoring.Tier / scoring.TierRank), reused rather than reinvented so a
//     module-host claim's tier string means the same thing on both sides of
//     the MCP boundary
//   - foundations/telemetry/sink (optional) — Deps.Sink, wired exactly like
//     kits/agent-harness's Deps.Sink; nil disables telemetry
//
// matching the kit's layering rule (CLAUDE.md: "kits compose foundations;
// imports point downward only"). expertchat imports NO fathom package —
// the module host is reached only through the Retriever interface, kept
// transport-thin. The HTTP implementation (HTTPRetriever, wire.go) speaks
// fathom's real HTTP MCP dispatch contract — the JSON-RPC "tools/call"
// envelope every fathom daemon actually answers with, not a bespoke verb —
// with one deliberately isolated unknown: the exact tool NAME on the
// not-yet-built `/rpc/modules` surface (docs/plans/gke-module-host.md,
// Phase G), kept behind a single, clearly-marked placeholder constant so
// reconciliation is a one-line change once the module-host worker finalizes
// it. See wire.go for exactly what is assumed and why.
//
// # The governed answer loop's contract (normative, docs/vision/
// 12-governed-knowledge-engine.md §3)
//
// retrieve via the module host → an answer MUST cite every claim it asserts
// with the claim's tier and staleness status → an answer with zero eligible
// evidence MUST refuse rather than answer from the model's own prior. Run is
// the loop; CitedAnswer is the citation-mandatory response type ("a response
// with an unsourced claim is unconstructable" — enforced by NewCitedAnswer,
// not a runtime linter: CitedAnswer's fields are unexported, and the only
// public constructor rejects zero citations); Refusal is the typed
// alternative outcome.
//
// # Refusal taxonomy
//
// Three reasons from the founding plan (docs/plans/gke-expertchat.md):
// RefusalNoEvidence (nothing came back from the retriever, nothing retrieved
// is in a citable status, or every citable item's identity is empty/unsafe
// to cite — see validCiteIdentity), RefusalBelowTierThreshold (evidence
// exists but none meets Deps.Policy.TierThreshold), RefusalStaleOnlyEvidence
// (Deps.Policy.RequireFresh is set and only stale evidence survived the tier
// filter). Two more are added beyond the plan's named three, each because
// folding it into an existing reason would misreport what actually
// happened, contradicting Refusal's own contract of naming what was
// searched/tried:
//
//   - RefusalFabricatedCitation covers the citation-validation loop in
//     requirement 4 of the plan ("validate the model's output cites only
//     supplied evidence; reject/retry-once then refuse on fabricated
//     citations") — there is no honest way to fold "the model invented a
//     citation" into any of the three evidence-shaped reasons above.
//   - RefusalEvidenceDoesNotAnswer covers the model's OWN honest verdict:
//     shown eligible, citable evidence, it truthfully reports (the
//     mandated "CITES: (none)" line) that none of it answers the query.
//     This is immediate and never retried — retrying an honest "(none)"
//     would pressure the model into fabricating a citation just to
//     satisfy the loop, and it is not RefusalFabricatedCitation because
//     nothing was fabricated, nor RefusalNoEvidence because evidence WAS
//     retrieved and WAS eligible.
//
// # Honest defaults (Policy)
//
// Policy.TierThreshold's zero value ("") applies no floor — every recognized
// tier is eligible. Policy.RequireFresh's zero value (false) is the freshness
// doctrine's honest default: stale evidence is still served, but its
// Citation.Status always says so — never silently promoted to look fresh.
// Set RequireFresh true to refuse rather than cite stale evidence at all.
//
// # Wiring point for app-kit's UI (not built here)
//
// app-kit's appkit/ui/chat (github.com/geoffmilleraz/app-kit, a separate
// repo/track) is the documented first-party UI consumer of Run's output: it
// renders CitedAnswer (text plus citations, each carrying module identity,
// tier, staleness status, and quoted anchor text) and Refusal (reason plus
// detail) directly. This is a wiring POINTER, not a code dependency in
// either direction — the loop's own contract above is UI-agnostic and
// MCP-native; a bare terminal client is an equally conforming consumer.
// app-kit's own track owns building the UI.
//
// # Concurrency
//
// CitedAnswer, Citation, Refusal, and EvidenceItem are immutable value types
// safe for concurrent read. FakeRetriever and HTTPRetriever are safe for
// concurrent use. Run itself holds no package-level mutable state and is
// safe to call concurrently with different Deps/query pairs; a single Deps
// value may be shared across concurrent Run calls provided its Retriever,
// Client, and Sink are themselves concurrency-safe (all three interfaces in
// this kit document that requirement, and llmclient.LLMClient already
// requires it).
package expertchat
