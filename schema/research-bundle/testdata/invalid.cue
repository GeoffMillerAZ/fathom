package researchbundle

// INVALID examples — kept commented so `cue vet ./...` stays green; the Go
// test (internal/research/bundle/schema_test.go) asserts each fails when
// uncommented into a unification.
//
// 1. An unknown field: #Bundle is closed, so anything outside the allowlist
//    (a stray repo/file path, here) is a hard validation error, not a
//    silently-ignored extra key:
// bad1: #Bundle & {
// 	schema_version: "v2"
// 	window_since:   "2026-07-21T00:00:00Z"
// 	window_until:   "2026-07-28T00:00:00Z"
// 	generated_at:   "2026-07-28T00:00:00Z"
// 	fathom_version: "fathom v0.1.0"
// 	commit_dirty:   false
// 	goos:           "darwin"
// 	goarch:         "arm64"
// 	file_path:      "/Users/someone/repo/secret.go"
// }
//
// 2. A service_hash that is not a 64-hex-char sha256 digest (i.e. looks like
//    it could be a plaintext name instead of a hash):
// bad2: #Bundle & {
// 	schema_version: "v2"
// 	window_since:   "2026-07-21T00:00:00Z"
// 	window_until:   "2026-07-28T00:00:00Z"
// 	generated_at:   "2026-07-28T00:00:00Z"
// 	fathom_version: "fathom v0.1.0"
// 	commit_dirty:   false
// 	goos:           "darwin"
// 	goarch:         "arm64"
// 	service_activity: [{service_hash: "my-secret-service-name", calls: 3}]
// }
//
// 3. A negative call count inside call_traffic:
// bad3: #Bundle & {
// 	schema_version: "v2"
// 	window_since:   "2026-07-21T00:00:00Z"
// 	window_until:   "2026-07-28T00:00:00Z"
// 	generated_at:   "2026-07-28T00:00:00Z"
// 	fathom_version: "fathom v0.1.0"
// 	commit_dirty:   false
// 	goos:           "darwin"
// 	goarch:         "arm64"
// 	call_traffic: {
// 		total_calls:             -1
// 		staleness_report_calls: 0
// 	}
// }
//
// 4. A required field missing entirely (goarch omitted):
// bad4: #Bundle & {
// 	schema_version: "v2"
// 	window_since:   "2026-07-21T00:00:00Z"
// 	window_until:   "2026-07-28T00:00:00Z"
// 	generated_at:   "2026-07-28T00:00:00Z"
// 	fathom_version: "fathom v0.1.0"
// 	commit_dirty:   false
// 	goos:           "darwin"
// }
//
// 5. A tool name outside #Label's shape (critic Finding 1: a call-log Tool
//    value is client-controlled, not fathom-controlled — a path-shaped or
//    otherwise hostile string must never validate):
// bad5: #Bundle & {
// 	schema_version: "v2"
// 	window_since:   "2026-07-21T00:00:00Z"
// 	window_until:   "2026-07-28T00:00:00Z"
// 	generated_at:   "2026-07-28T00:00:00Z"
// 	fathom_version: "fathom v0.1.0"
// 	commit_dirty:   false
// 	goos:           "darwin"
// 	goarch:         "arm64"
// 	call_traffic: {
// 		total_calls:             1
// 		tool_calls: [{tool: "/Users/someone/repo/secret.go", calls: 1}]
// 		staleness_report_calls: 0
// 	}
// }
//
// 6. A surface value outside #Label's shape (or empty) — e.g. one carrying
//    a Markdown table-breaking character:
// bad6: #Bundle & {
// 	schema_version: "v2"
// 	window_since:   "2026-07-21T00:00:00Z"
// 	window_until:   "2026-07-28T00:00:00Z"
// 	generated_at:   "2026-07-28T00:00:00Z"
// 	fathom_version: "fathom v0.1.0"
// 	commit_dirty:   false
// 	goos:           "darwin"
// 	goarch:         "arm64"
// 	call_traffic: {
// 		total_calls:             1
// 		surface_traffic: [{surface: "agent | injected", calls: 1}]
// 		staleness_report_calls: 0
// 	}
// }
//
// 7. A bench arm label outside #Label's shape:
// bad7: #Bundle & {
// 	schema_version: "v2"
// 	window_since:   "2026-07-21T00:00:00Z"
// 	window_until:   "2026-07-28T00:00:00Z"
// 	generated_at:   "2026-07-28T00:00:00Z"
// 	fathom_version: "fathom v0.1.0"
// 	commit_dirty:   false
// 	goos:           "darwin"
// 	goarch:         "arm64"
// 	bench_arms: [{arm: "fathom\nEVIL", task_count: 1, success_count: 1, success_rate: 1.0, mean_tokens: 0, mean_wall_ms: 0}]
// }
//
// 8. IR-5: a retro outcome label outside #Label's shape (a raw mechanical
//    outcome string with a hyphen must be mapped, never passed through):
// bad8: #Bundle & {
// 	schema_version: "v2"
// 	window_since:   "2026-07-21T00:00:00Z"
// 	window_until:   "2026-07-28T00:00:00Z"
// 	generated_at:   "2026-07-28T00:00:00Z"
// 	fathom_version: "fathom v0.1.0"
// 	commit_dirty:   false
// 	goos:           "darwin"
// 	goarch:         "arm64"
// 	retros: {
// 		retros: 1, unreadable: 0, off_arm_retros: 0, guard_fires: 1, obligation_injections: 0
// 		silent_on_clean: 0, unjoinable: 0, custodian_remedied: 0, custodian_unremedied: 0
// 		outcomes: [{channel: "guard", outcome: "modified-uncommitted", firings: 1}]
// 	}
// }
//
// 9. IR-5: a file or session smuggled into the retros group (closed struct):
// bad9: #Bundle & {
// 	... common fields as above ...
// 	retros: { ...required counts..., file: "internal/secret.go" }
// }
