package commandguard

scope: #Scope & {
	version: "command-guard.scope/1"
	requiredLayerIds: ["local"]
	requiredRuleIds: ["token"]
}
policy: #Policy & {
	version: "command-guard.policy/1"
	layers: [{id: "local", allowedOperations: ["read-range", "search-literal"], limits: {maxFileBytes: 4096, maxTotalBytes: 8192, maxOutputBytes: 4096, maxResults: 10}, deniedPathPrefixes: ["private"]}]
	rules: [{id: "token", pattern: "SECRET"}]
}
request: #Request & {
	version: "command-guard.request/1"
	kind:    "read-range"
	files: [{path: "sample.txt", sha256: "0000000000000000000000000000000000000000000000000000000000000000"}]
	startLine: 1
	endLine:   3
}
report: #Report & {
	version:           "command-guard.report/1"
	status:            "no-findings"
	code:              "ok"
	noActionAuthority: true
	filesRead:         1
	bytesRead:         25
	bytesScanned:      25
	scope:             "selected-files-only"
	scanner:           "configured-regex-only"
	limitation:        "not-proof-of-secret-or-pii-absence"
	totalMatches:      2
	returned:          2
	omitted:           0
	truncated:         false
	rows: [{file: 0, line: 1, text: "alpha public"}, {file: 0, line: 2, text: "beta public"}]
}
scannerScope: #Scope & {
 version: "command-guard.scope/1"
 requiredLayerIds: ["local"]
 requiredRuleIds: ["token"]
 requiredScannerIds: ["gitleaks"]
}
scannerPolicy: #Policy & {
 version: "command-guard.policy/1"
 layers: [{id: "local", allowedOperations: ["read-range"], limits: {maxFileBytes: 4096, maxTotalBytes: 8192, maxOutputBytes: 4096, maxResults: 10}, deniedPathPrefixes: []}]
 rules: [{id: "token", pattern: "SECRET"}]
 scanner: {kind: "gitleaks", profile: "gitleaks-stdin-8.30.1", executable: "/trusted/gitleaks", sha256: "0000000000000000000000000000000000000000000000000000000000000000", timeoutMillis: 30000, maxReportBytes: 1048576}
}
scannerReport: #Report & {
 version: "command-guard.report/1"
 status: "incomplete"
 code: "scanner_incomplete"
 noActionAuthority: true
 filesRead: 1
 bytesRead: 25
 bytesScanned: 25
 scope: "selected-files-only"
 scanner: "configured-regex-and-gitleaks"
	externalScanner: {id: "gitleaks", profile: "gitleaks-stdin-8.30.1", status: "incomplete", code: "coverage_mismatch", expectedFiles: 1, attemptedFiles: 1, invokedFiles: 1, completedFiles: 0, bytesSubmitted: 25, bytesObserved: 0, findings: 0}
 limitation: "not-proof-of-secret-or-pii-absence"
 totalMatches: 0
 returned: 0
 omitted: 0
 truncated: false
}
gitScope: #Scope & {
 version: "command-guard.scope/1"
 requiredLayerIds: ["local"]
 requiredRuleIds: ["token"]
 requiredGitProfiles: ["git-read/1"]
}
gitPolicy: #Policy & {
 version: "command-guard.policy/1"
 layers: [{id: "local", allowedOperations: ["git-status", "git-object-diff", "git-history"], limits: {maxFileBytes: 4194304, maxTotalBytes: 67108864, maxOutputBytes: 1048576, maxResults: 1000}, deniedPathPrefixes: []}]
 rules: [{id: "token", pattern: "SECRET"}]
 git: {profile: "git-read/1", executable: "/trusted/git", sha256: "0000000000000000000000000000000000000000000000000000000000000000", expectedVersion: "git version 2.54.0", timeoutMillis: 30000, repository: {kind: "worktree", root: "/registered/tree", gitDir: "/registered/tree/.git", commonDir: "/registered/tree/.git", configSHA256: "0000000000000000000000000000000000000000000000000000000000000000", worktreeConfigSHA256: "absent", commonDirFileSHA256: "absent"}}
}
gitStatusRequest: #GitRequest & {version: "command-guard.git-request/1", kind: "git-status", paths: ["."]}
gitDiffRequest: #GitRequest & {version: "command-guard.git-request/1", kind: "git-object-diff", base: "HEAD", target: "0000000000000000000000000000000000000000", paths: ["a.txt"]}
gitHistoryRequest: #GitRequest & {version: "command-guard.git-request/1", kind: "git-history", target: "HEAD", maxCommits: 1}
gitReport: #GitReport & {version: "command-guard.git-report/1", operation: "git-status", status: "no-findings", code: "ok", noActionAuthority: true, scope: ["."], scanner: "configured-regex-only", attemptedCalls: 3, startedCalls: 3, completedCalls: 3, bytesScanned: 2, returned: 0, omitted: 0, omittedLines: 0, omittedBytes: 0, projectionTruncated: false, hasMore: false, nonAtomic: true, limitation: "live-worktree-non-atomic; ignored-and-submodules-excluded; no-clean-or-absence-inference"}
gitBlockedReport: #GitReport & {version: "command-guard.git-report/1", operation: "git-status", status: "blocked", code: "rule", noActionAuthority: true, scanner: "configured-regex-only", attemptedCalls: 4, startedCalls: 4, completedCalls: 4, bytesScanned: 20, returned: 0, omitted: 0, omittedLines: 0, omittedBytes: 0, projectionTruncated: false, hasMore: false, nonAtomic: true, limitation: "live-worktree-non-atomic; ignored-and-submodules-excluded; no-clean-or-absence-inference"}
gitIncompleteReport: #GitReport & {version: "command-guard.git-report/1", operation: "git-status", status: "incomplete", code: "state_drift", noActionAuthority: true, scanner: "configured-regex-only", attemptedCalls: 4, startedCalls: 4, completedCalls: 4, bytesScanned: 0, returned: 0, omitted: 0, omittedLines: 0, omittedBytes: 0, projectionTruncated: false, hasMore: false, nonAtomic: true, limitation: "live-worktree-non-atomic; ignored-and-submodules-excluded; no-clean-or-absence-inference"}
