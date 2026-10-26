package commandqualify

import "list"

_hash: string & =~"^[0-9a-f]{64}$"
_id: "read-json" | "read-text" | "read-compact" | "ordinary-compact-fallback" | "search-literal" | "late-regex" | "wrong-input-hash" | "missing-control" | "denied-prefix" | "traversal" | "case-alias" | "reserved-name" | "invalid-utf8" | "file-cap" | "symlink" | "tiny-json" | "tiny-text" | "tiny-compact" | "scanner-clean" | "scanner-empty" | "scanner-aba-disabled" | "scanner-aba-enabled" | "scanner-canary" | "scanner-late-pdf" | "scanner-wrong-hash" | "scanner-missing-scope" | "scanner-ambient-ignore" | "classifier-no-concern" | "classifier-concern" | "classifier-unknown" | "classifier-model-mismatch" | "classifier-refusal" | "classifier-duplicate-answer" | "classifier-truncated" | "classifier-unpinned-input" | "classifier-policy-drift" | "classifier-regex-hold" | "classifier-redirect" | "classifier-response-limit"

#Case: close({id: _id, status: "pass", code: "oracle_met"}) | close({id: _id, status: "fail", code: "oracle_mismatch"}) | close({id: _id, status: "unknown", code: "unrun" | "setup_failed" | "start_failed" | "wait_failed" | "deadline" | "output_limit" | "preparation_failed" | "cleanup_failed" | "binary_unavailable" | "hash_mismatch"})
#Report: {
	version: "command-qualify.report/1"
	profile: "files-core/1" | "files-gitleaks/1" | "files-loopback-fixture/1"
	rosterSHA256: _hash
	goos: string & !=""
	goarch: string & !=""
	guard: close({sha256: _hash})
	scanner?: close({sha256: _hash})
	syntheticPeerOnly?: true
	required: 18 | 27 | 30
	passed: int & >=0 & <=30
	failed: int & >=0 & <=30
	unknown: int & >=0 & <=30
	status: "pass" | "fail" | "unknown"
	cases: [...#Case] & list.MinItems(18) & list.MaxItems(30)
	_count: true & (required == len(cases))
	_sum: true & (required == passed + failed + unknown)
	if profile == "files-core/1" {
		required: 18
		scanner?: _|_
		syntheticPeerOnly?: _|_
	}
	if profile == "files-gitleaks/1" {
		required: 27
		scanner: close({sha256: _hash})
		syntheticPeerOnly?: _|_
	}
	if profile == "files-loopback-fixture/1" {
		required: 30
		scanner?: _|_
		syntheticPeerOnly!: true
	}
	if status == "pass" {
		passed: required
		failed: 0
		unknown: 0
	}
	if status == "fail" {
		failed: >=1
		unknown: 0
	}
	if status == "unknown" {
		unknown: >=1
	}
}
