package researchbundle

// INVALID examples — kept commented so `cue vet ./...` stays green; the Go
// test (internal/research/bundle/schema_test.go) asserts each fails when
// uncommented into a unification.
//
// 1. An unknown field: #Bundle is closed, so anything outside the allowlist
//    (a stray repo/file path, here) is a hard validation error, not a
//    silently-ignored extra key:
// bad1: #Bundle & {
// 	schema_version: "v1"
// 	window_since:   "2026-07-21T00:00:00Z"
// 	window_until:   "2026-07-28T00:00:00Z"
// 	generated_at:   "2026-07-28T00:00:00Z"
// 	fathom_version: "fathom v0.1.0"
// 	commit_dirty:   false
// 	goos:           "darwin"
// 	goarch:         "arm64"
// 	total_calls:    1
// 	injection_seen:            0
// 	injection_answered:        0
// 	obligations_served:        0
// 	verify_claimed:                  0
// 	verify_verified:                 0
// 	verify_failing:                  0
// 	verify_instrumented_calls:       0
// 	verify_predates_instrumentation: 0
// 	review_active_findings:          0
// 	staleness_report_calls: 0
// 	file_path: "/Users/someone/repo/secret.go"
// }
//
// 2. A service_hash that is not a 64-hex-char sha256 digest (i.e. looks like
//    it could be a plaintext name instead of a hash):
// bad2: #Bundle & {
// 	schema_version: "v1"
// 	window_since:   "2026-07-21T00:00:00Z"
// 	window_until:   "2026-07-28T00:00:00Z"
// 	generated_at:   "2026-07-28T00:00:00Z"
// 	fathom_version: "fathom v0.1.0"
// 	commit_dirty:   false
// 	goos:           "darwin"
// 	goarch:         "arm64"
// 	total_calls:    1
// 	injection_seen:            0
// 	injection_answered:        0
// 	obligations_served:        0
// 	verify_claimed:                  0
// 	verify_verified:                 0
// 	verify_failing:                  0
// 	verify_instrumented_calls:       0
// 	verify_predates_instrumentation: 0
// 	review_active_findings:          0
// 	staleness_report_calls: 0
// 	service_activity: [{service_hash: "my-secret-service-name", calls: 3}]
// }
//
// 3. A negative call count:
// bad3: #Bundle & {
// 	schema_version: "v1"
// 	window_since:   "2026-07-21T00:00:00Z"
// 	window_until:   "2026-07-28T00:00:00Z"
// 	generated_at:   "2026-07-28T00:00:00Z"
// 	fathom_version: "fathom v0.1.0"
// 	commit_dirty:   false
// 	goos:           "darwin"
// 	goarch:         "arm64"
// 	total_calls:    -1
// 	injection_seen:            0
// 	injection_answered:        0
// 	obligations_served:        0
// 	verify_claimed:                  0
// 	verify_verified:                 0
// 	verify_failing:                  0
// 	verify_instrumented_calls:       0
// 	verify_predates_instrumentation: 0
// 	review_active_findings:          0
// 	staleness_report_calls: 0
// }
//
// 4. A required field missing entirely (total_calls omitted):
// bad4: #Bundle & {
// 	schema_version: "v1"
// 	window_since:   "2026-07-21T00:00:00Z"
// 	window_until:   "2026-07-28T00:00:00Z"
// 	generated_at:   "2026-07-28T00:00:00Z"
// 	fathom_version: "fathom v0.1.0"
// 	commit_dirty:   false
// 	goos:           "darwin"
// 	goarch:         "arm64"
// 	injection_seen:            0
// 	injection_answered:        0
// 	obligations_served:        0
// 	verify_claimed:                  0
// 	verify_verified:                 0
// 	verify_failing:                  0
// 	verify_instrumented_calls:       0
// 	verify_predates_instrumentation: 0
// 	review_active_findings:          0
// 	staleness_report_calls: 0
// }
