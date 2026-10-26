package commandguard

inspectCompact: #OutputReport & {
	version: "command-guard.compact-report/1"
	rowFormat: "file-line-text/1"
	status: "no-findings", code: "ok", noActionAuthority: true
	filesRead: 1, bytesRead: 3, bytesScanned: 3
	scope: "selected-files-only", scanner: "configured-regex-only"
	limitation: "not-proof-of-secret-or-pii-absence"
	totalMatches: 1, returned: 1, omitted: 0, truncated: false
	rows: [[0, 1, "a\t\"b"]]
}
statusCompact: #OutputGitReport & {
	version: "command-guard.compact-git-report/1", rowFormat: "status/1"
	operation: "git-status", status: "no-findings", code: "ok"
	noActionAuthority: true, scope: ["."], scanner: "configured-regex-only"
	attemptedCalls: 1, startedCalls: 1, completedCalls: 1, bytesScanned: 1
	returned: 1, omitted: 0, omittedLines: 0, omittedBytes: 0
	projectionTruncated: false, hasMore: false, nonAtomic: true, limitation: "selected"
	rows: [["untracked", "a.txt", "??"]]
}
diffCompact: #OutputGitReport & {
	version: "command-guard.compact-git-report/1", rowFormat: "object-diff/1"
	operation: "git-object-diff", status: "no-findings", code: "ok"
	noActionAuthority: true, scope: ["a.txt"], baseCommit: "0000000000000000000000000000000000000000", targetCommit: "1111111111111111111111111111111111111111"
	scanner: "configured-regex-only", attemptedCalls: 1, startedCalls: 1, completedCalls: 1, bytesScanned: 1
	returned: 1, omitted: 0, omittedLines: 0, omittedBytes: 0
	projectionTruncated: false, hasMore: false, nonAtomic: false, limitation: "selected"
	rows: [["a.txt", "", "1111111111111111111111111111111111111111", [], ["é"], 0, 0]]
}
historyCompact: #OutputGitReport & {
	version: "command-guard.compact-git-report/1", rowFormat: "history/1"
	operation: "git-history", status: "no-findings", code: "ok"
	noActionAuthority: true, targetCommit: "1111111111111111111111111111111111111111"
	scanner: "configured-regex-only", attemptedCalls: 1, startedCalls: 1, completedCalls: 1, bytesScanned: 1
	returned: 1, omitted: 0, omittedLines: 0, omittedBytes: 0
	projectionTruncated: false, hasMore: false, nonAtomic: false, limitation: "selected"
	rows: [["1111111111111111111111111111111111111111", [], "A", "C", "message", 0]]
}

// Negative shapes for `cue vet`: append any one to a copy under a distinct
// field constrained by #CompactGitReport or #CompactReport. Rejected: a
// status/1 tuple under git-history, an eighth diff element, a string where
// a line/count belongs, a missing position, or any extra header field.
