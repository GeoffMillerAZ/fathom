package modspec

// INVALID examples — kept commented so `cue vet ./...` stays green; the Go
// test (internal/modspec/schema_test.go) asserts each fails when uncommented
// into a unification.
//
// 1. Empty corpus include (a module must select a corpus, mirroring
//    #Layer's own rule):
// bad1: #Module & {
// 	name: "x", version: "v1", description: "x"
// 	corpus: include: []
// 	watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
// 	eval_evidence: evaled: false
// 	recompile: allowed: false
// 	provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
// }
//
// 2. Missing watermark — docs/spec/expert-module-v0.md §3.3's freshness
//    doctrine: "a module without a watermark is invalid." Checked with
//    cue.Concrete(true) (this repo's convention for a missing-required-
//    field negative, see schema/config/testdata/invalid.cue case 4 /
//    internal/config/schema_test.go), since a plain Validate() does not by
//    itself reject a merely-abstract (unset) required field:
// bad2: #Module & {
// 	name: "x", version: "v1", description: "x"
// 	corpus: include: ["docs/**"]
// 	eval_evidence: evaled: false
// 	recompile: allowed: false
// 	provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
// }
//
// 3. A malformed watermark (neither a 40-hex git SHA nor a
//    manifest:-prefixed 64-hex content hash):
// bad3: #Module & {
// 	name: "x", version: "v1", description: "x"
// 	corpus: include: ["docs/**"]
// 	watermark: "not-a-real-watermark"
// 	eval_evidence: evaled: false
// 	recompile: allowed: false
// 	provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
// }
//
// 4. evaled:true with no eval_evidence.path — the D3 trust seam requires an
//    on-disk pointer, never a bare self-declaration, for a module shipped
//    outside the fathom binary (docs/design/pack-taxonomy.md):
// bad4: #Module & {
// 	name: "x", version: "v1", description: "x"
// 	corpus: include: ["docs/**"]
// 	watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
// 	eval_evidence: evaled: true
// 	recompile: allowed: false
// 	provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
// }
//
// 5. recompile.allowed:true with no prompt_identity — a manifest that
//    permits recompilation must name the exact prompt identity a compliant
//    recompile is expected to use (docs/spec/store-format-v0.md §9's claim-
//    identity reproducibility contract):
// bad5: #Module & {
// 	name: "x", version: "v1", description: "x"
// 	corpus: include: ["docs/**"]
// 	watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
// 	eval_evidence: evaled: false
// 	recompile: allowed: true
// 	provenance: {compiled_by: "deterministic", compiled_at: "2026-07-28T00:00:00Z"}
// }
//
// 6. A malformed provenance.compiled_at (not RFC3339-shaped):
// bad6: #Module & {
// 	name: "x", version: "v1", description: "x"
// 	corpus: include: ["docs/**"]
// 	watermark: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
// 	eval_evidence: evaled: false
// 	recompile: allowed: false
// 	provenance: {compiled_by: "deterministic", compiled_at: "not-a-date"}
// }
