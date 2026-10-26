// command-guard control contracts. Runtime additionally enforces byte limits,
// exact ID sets, path alias rules, effective intersections, and disclosure.
package commandguard

import (
	"list"
	"strings"
)

_id:   string & =~"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$"
_path: string & strings.MaxRunes(1024) & =~"^[ -~]+$" & !~"[\\\\:<>\"|?*]" & !~"(^/|/$|//|(^|/)\\.{1,2}(/|$)|[. ](/|$))" & !~"(?i)(^|/)(con|prn|aux|nul|com[1-9]|lpt[1-9])(\\.[^/]*)?(/|$)"
#ScopeNoClassifier: close({version: "command-guard.scope/1", requiredLayerIds: [..._id] & list.MinItems(1) & list.MaxItems(16) & list.UniqueItems(), requiredRuleIds: [..._id] & list.MinItems(1) & list.MaxItems(64) & list.UniqueItems(), requiredScannerIds?: [] | ["gitleaks"], requiredGitProfiles?: [] | ["git-read/1"], requiredClassifierIds?: [] & list.MaxItems(0), permittedClassifierSHA256?: [] & list.MaxItems(0), classifierPolicySHA256?: ""})
#ScopeWithClassifier: close({version: "command-guard.scope/1", requiredLayerIds: [..._id] & list.MinItems(1) & list.MaxItems(16) & list.UniqueItems(), requiredRuleIds: [..._id] & list.MinItems(1) & list.MaxItems(64) & list.UniqueItems(), requiredScannerIds?: [] | ["gitleaks"], requiredGitProfiles?: [] | ["git-read/1"], requiredClassifierIds!: [...(string & =~"^semantic-concern$")] & list.MinItems(1) & list.MaxItems(1), permittedClassifierSHA256!: [...(string & =~"^[0-9a-f]{64}$")] & list.MinItems(1) & list.MaxItems(32) & list.UniqueItems(), classifierPolicySHA256!: string & =~"^[0-9a-f]{64}$"})
#Scope: #ScopeNoClassifier | #ScopeWithClassifier
#Limits: close({maxFileBytes: int & >=1 & <=4194304, maxTotalBytes: int & >=1 & <=67108864, maxOutputBytes: int & >=1 & <=1048576, maxResults: int & >=1 & <=1000})
#Layer: close({id: _id, allowedOperations: [...("read-range" | "search-literal" | "git-status" | "git-object-diff" | "git-history")] & list.MinItems(1) & list.MaxItems(5) & list.UniqueItems(), limits: #Limits, deniedPathPrefixes: [..._path] & list.MaxItems(64)})
#Rule: close({
	id:      _id
	pattern: string & !="" & strings.MaxRunes(2048)
	_byteLimit: len(pattern) & <=2048
})
#ScannerPolicy: close({
	kind:       "gitleaks"
	profile:    "gitleaks-stdin-8.30.1"
	executable: string & !="" & strings.MaxRunes(4096)
	_byteLimit: len(executable) & <=4096
	sha256:         string & =~"^[0-9a-f]{64}$"
	timeoutMillis:  int & >=1 & <=30000
	maxReportBytes: int & >=1 & <=1048576
	reuseEqualInputs?: bool
})
_gitAbs: string & =~"^/" & strings.MaxRunes(4096)
_oid: string & =~"^([0-9a-f]{40}|[0-9a-f]{64})$"
#GitRepository: close({kind: "worktree" | "bare", root: _gitAbs, gitDir: _gitAbs, commonDir: _gitAbs, configSHA256: string & =~"^[0-9a-f]{64}$", worktreeConfigSHA256: "absent" | (string & =~"^[0-9a-f]{64}$"), commonDirFileSHA256: "absent" | (string & =~"^[0-9a-f]{64}$")})
#GitPolicy: close({profile: "git-read/1", executable: _gitAbs, sha256: string & =~"^[0-9a-f]{64}$", expectedVersion: string & =~"^git version [^\\x00-\\x1f\\x7f]+$" & strings.MaxRunes(128), timeoutMillis: int & >=1 & <=30000, repository: #GitRepository})
#ClassifierPolicy: close({profile!: "loopback-concern/1", endpoint!: string & =~"^http://(127\\.0\\.0\\.1|\\[::1\\]):(?:[1-9]|[1-9][0-9]{1,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5])/v1$", model!: string & =~"^[ -~]+$" & strings.MaxRunes(128), expectedResponseModel!: string & =~"^[ -~]+$" & strings.MaxRunes(128), criterion!: string & =~"\\S" & !~"[\\p{Cf}\\p{Cs}\\p{Co}\\p{Zl}\\p{Zp}]", criterionSHA256!: string & =~"^[0-9a-f]{64}$", timeoutMillis!: int & >=1 & <=30000, maxInputBytes!: int & >=1 & <=16384, maxResponseBytes!: int & >=1 & <=16384, maxTokens!: int & >=1 & <=1024, _criterionByteLimit: len(criterion) & <=4096})
#Policy: close({version: "command-guard.policy/1", layers: [...#Layer] & list.MinItems(1) & list.MaxItems(16), rules: [...#Rule] & list.MinItems(1) & list.MaxItems(64), scanner?: #ScannerPolicy, git?: #GitPolicy, classifier?: #ClassifierPolicy})
#File: close({path: _path, sha256: string & =~"^[0-9a-f]{64}$"})
#BaseRequest: {version: "command-guard.request/1", files: [...#File] & list.MinItems(1) & list.MaxItems(32)}
#ReadRequest: close({#BaseRequest, kind: "read-range", startLine: int & >=1 & <=1000000, endLine: int & >=startLine & <=1000000})
#SearchRequest: close({
	#BaseRequest
	kind:    "search-literal"
	literal: string & !="" & !~"[\\x00-\\x08\\x0a-\\x1f\\x7f]" & strings.MaxRunes(1024)
	_byteLimit: len(literal) & <=1024
})
#Request: #ReadRequest | #SearchRequest
#GitStatusRequest: close({version: "command-guard.git-request/1", kind: "git-status", paths: [...(_path | ".")] & list.MinItems(1) & list.MaxItems(32) & list.UniqueItems()})
#GitDiffRequest: close({version: "command-guard.git-request/1", kind: "git-object-diff", base: "HEAD" | (string & =~"^([0-9a-f]{40}|[0-9a-f]{64})$"), target: "HEAD" | (string & =~"^([0-9a-f]{40}|[0-9a-f]{64})$"), paths: [...(_path | ".")] & list.MinItems(1) & list.MaxItems(32) & list.UniqueItems()})
#GitHistoryRequest: close({version: "command-guard.git-request/1", kind: "git-history", target: "HEAD" | (string & =~"^([0-9a-f]{40}|[0-9a-f]{64})$"), maxCommits: int & >=1 & <=32})
#GitRequest: #GitStatusRequest | #GitDiffRequest | #GitHistoryRequest

#Row: close({file: int & >=0 & <32, line: int & >=1 & <=4194305, text: string})
#ScannerReuse: close({profile: "invocation-exact/1", files: int & >=0 & <=256, bytes: int & >=0 & <=67108864})
#ScannerCompleteCoverage: {
	expectedFiles: int
	completedFiles: int
	reuse?: #ScannerReuse
	if reuse != _|_ { _coverage: true & (completedFiles + reuse.files == expectedFiles) }
	if reuse == _|_ { completedFiles: expectedFiles }
	...
}
#ExternalScanner: close({id: "gitleaks", profile: "gitleaks-stdin-8.30.1", status: "not-invoked" | "complete" | "finding" | "incomplete", code: "not_invoked" | "verified" | "finding" | "binary_unavailable" | "hash_mismatch" | "version_mismatch" | "deadline" | "report_invalid" | "coverage_mismatch" | "execution_incomplete" | "output_limit" | "cleanup_failed", expectedFiles: int & >=0 & <=32, attemptedFiles: int & >=0 & <=32, invokedFiles: int & >=0 & <=32, completedFiles: int & >=0 & <=32, bytesSubmitted: int & >=0 & <=67108864, bytesObserved: int & >=0 & <=67108864, findings: int & >=0, reuse?: #ScannerReuse & {files: <=(expectedFiles - completedFiles)}})
_ClassifierReceiptBase: {profile!: "loopback-concern/1", expectedFiles!: int & >=0 & <=32, selectedFiles!: int & >=0 & <=32, selectedBytes!: int & >=0 & <=67108864, attemptedRequests!: int & >=0 & <=1, completedDecisions!: int & >=0 & <=1, requestBytes!: int & >=0 & <=131073, responseBytes!: int & >=0 & <=16385}
#ClassifierNotInvoked: close({_ClassifierReceiptBase, status!: "not-invoked", code!: "not_invoked", selectedFiles!: 0, selectedBytes!: 0, attemptedRequests!: 0, completedDecisions!: 0, requestBytes!: 0, responseBytes!: 0})
#ClassifierComplete: close({_ClassifierReceiptBase, status!: "complete", code!: "no_concern", expectedFiles!: int & >=0 & <=32, selectedFiles!: expectedFiles, selectedBytes!: >=1 & <=16384, attemptedRequests!: 1, completedDecisions!: 1, requestBytes!: >=1 & <=131072, responseBytes!: >=1 & <=16384})
#ClassifierConcern: close({_ClassifierReceiptBase, status!: "concern", code!: "concern", expectedFiles!: int & >=0 & <=32, selectedFiles!: expectedFiles, selectedBytes!: >=1 & <=16384, attemptedRequests!: 1, completedDecisions!: 1, requestBytes!: >=1 & <=131072, responseBytes!: >=1 & <=16384})
#ClassifierUnknown: close({_ClassifierReceiptBase, status!: "unknown", code!: "unknown", expectedFiles!: int & >=0 & <=32, selectedFiles!: expectedFiles, selectedBytes!: >=1 & <=16384, attemptedRequests!: 1, completedDecisions!: 1, requestBytes!: >=1 & <=131072, responseBytes!: >=1 & <=16384})
#ClassifierIncomplete: close({_ClassifierReceiptBase, status!: "incomplete", code!: "disclosure_denied", attemptedRequests!: 0, completedDecisions!: 0, requestBytes!: 0, responseBytes!: 0}) | close({_ClassifierReceiptBase, status!: "incomplete", code!: "input_limit", expectedFiles!: int & >=0 & <=32, selectedFiles!: expectedFiles, attemptedRequests!: 0, completedDecisions!: 0, requestBytes!: 0, responseBytes!: 0}) | close({_ClassifierReceiptBase, status!: "incomplete", code!: "request_limit", expectedFiles!: int & >=0 & <=32, selectedFiles!: expectedFiles, attemptedRequests!: 0, completedDecisions!: 0, requestBytes!: 131073, responseBytes!: 0}) | close({_ClassifierReceiptBase, status!: "incomplete", code!: "response_limit" | "connection_failed" | "http_error" | "response_invalid" | "model_mismatch" | "refused" | "unsupported_response", expectedFiles!: int & >=0 & <=32, selectedFiles!: expectedFiles, selectedBytes!: >=1 & <=16384, attemptedRequests!: 1, completedDecisions!: 0, requestBytes!: >=1 & <=131072}) | close({_ClassifierReceiptBase, status!: "incomplete", code!: "deadline" | "execution_incomplete", completedDecisions!: 0})
#ClassifierReceipt: #ClassifierNotInvoked | #ClassifierComplete | #ClassifierConcern | #ClassifierUnknown | #ClassifierIncomplete
_ReportBase: {version: "command-guard.report/1", noActionAuthority: true, filesRead: int & >=0 & <=32, bytesRead: int & >=0 & <=67108865, bytesScanned: int & >=0 & <=67108864, scope: "selected-files-only", scanner: "configured-regex-only" | "configured-regex-and-gitleaks", externalScanner?: #ExternalScanner, externalClassifier?: #ClassifierReceipt, limitation: "not-proof-of-secret-or-pii-absence", totalMatches: int & >=0, returned: int & >=0 & <=1000, omitted: int & >=0, truncated: bool, rows?: [...#Row] & list.MaxItems(1000)}
#CompleteReport: close({_ReportBase, status: "no-findings", code: "ok", externalScanner?: #ExternalScanner & #ScannerCompleteCoverage, externalClassifier?: #ClassifierComplete})
_HeldSource: {returned: 0, rows?: _|_}
#BlockedReport: close({_ReportBase, _HeldSource, status: "blocked", code: "rule" | "denied", externalClassifier?: #ClassifierNotInvoked}) | close({_ReportBase, _HeldSource, status: "blocked", code: "scanner_finding", externalScanner: #ExternalScanner & #ScannerCompleteCoverage, externalClassifier?: #ClassifierNotInvoked}) | close({_ReportBase, _HeldSource, status: "blocked", code: "classifier_concern", externalClassifier: #ClassifierConcern})
#IncompleteReport: close({_ReportBase, _HeldSource, status: "incomplete", code: "invalid" | "scanner_incomplete", externalClassifier?: #ClassifierNotInvoked}) | close({_ReportBase, _HeldSource, status: "incomplete", code: "output_limit", externalClassifier?: #ClassifierNotInvoked | #ClassifierComplete}) | close({_ReportBase, _HeldSource, status: "incomplete", code: "classifier_incomplete", externalClassifier: #ClassifierIncomplete | #ClassifierUnknown})
#UnsupportedReport: close({version: "command-guard.report/1", status: "incomplete", code: "unsupported", noActionAuthority: true, filesRead!: int & >=0 & <=0, bytesRead!: int & >=0 & <=0, bytesScanned!: int & >=0 & <=0, scope: "selected-files-only", scanner: "configured-regex-only", limitation: "not-proof-of-secret-or-pii-absence", totalMatches!: int & >=0 & <=0, returned!: int & >=0 & <=0, omitted!: int & >=0 & <=0, truncated!: false, externalScanner?: _|_, externalClassifier?: _|_, rows?: _|_})
#Report: #CompleteReport | #BlockedReport | #IncompleteReport | #UnsupportedReport
#GitRow: close({kind: "tracked" | "untracked" | "change" | "commit", path?: _path, xy?: string, oid?: _oid, parentIds?: [..._oid] & list.MaxItems(32), author?: string, committer?: string, message?: string, beforeOid?: _oid, afterOid?: _oid, before?: [...string] & list.MaxItems(8), after?: [...string] & list.MaxItems(8), beforeOmittedLines?: int & >=0, afterOmittedLines?: int & >=0, messageOmittedBytes?: int & >=0})
#GitExternalScanner: close({id: "gitleaks", profile: "gitleaks-stdin-8.30.1", status: "not-invoked" | "complete" | "finding" | "incomplete", code: "not_invoked" | "verified" | "finding" | "binary_unavailable" | "hash_mismatch" | "version_mismatch" | "deadline" | "report_invalid" | "coverage_mismatch" | "execution_incomplete" | "output_limit" | "cleanup_failed", expectedFiles: int & >=0 & <=256, attemptedFiles: int & >=0 & <=256, invokedFiles: int & >=0 & <=256, completedFiles: int & >=0 & <=256, bytesSubmitted: int & >=0 & <=67108864, bytesObserved: int & >=0 & <=67108864, findings: int & >=0, reuse?: #ScannerReuse & {files: <=(expectedFiles - completedFiles)}})
_GitReportBase: {version: "command-guard.git-report/1", noActionAuthority: true, operation?: "git-status" | "git-object-diff" | "git-history", scanner: "configured-regex-only" | "configured-regex-and-gitleaks", externalScanner?: #GitExternalScanner, attemptedCalls: int & >=0, startedCalls: int & >=0, completedCalls: int & >=0, bytesScanned: int & >=0, returned: int & >=0 & <=1000, omitted: int & >=0, omittedLines: int & >=0, omittedBytes: int & >=0, projectionTruncated: bool, hasMore: bool, nonAtomic: bool, limitation: string}
_GitSourceFields: {scope?: [...(_path | ".")] & list.MaxItems(32), baseCommit?: _oid, targetCommit?: _oid, rows?: [...#GitRow] & list.MaxItems(1000)}
#GitCompleteStatus: close({_GitReportBase, _GitSourceFields, status: "no-findings", code: "ok", operation: "git-status", scope: [...(_path | ".")] & list.MinItems(1) & list.MaxItems(32), baseCommit?: _|_, targetCommit?: _|_, externalScanner?: #GitExternalScanner & #ScannerCompleteCoverage})
#GitCompleteDiff: close({_GitReportBase, _GitSourceFields, status: "no-findings", code: "ok", operation: "git-object-diff", scope: [...(_path | ".")] & list.MinItems(1) & list.MaxItems(32), baseCommit: _oid, targetCommit: _oid, externalScanner?: #GitExternalScanner & #ScannerCompleteCoverage})
#GitCompleteHistory: close({_GitReportBase, _GitSourceFields, status: "no-findings", code: "ok", operation: "git-history", scope?: _|_, baseCommit?: _|_, targetCommit: _oid, externalScanner?: #GitExternalScanner & #ScannerCompleteCoverage})
#GitBlockedReport: close({_GitReportBase, status: "blocked", code: "rule" | "denied", returned: 0, scope?: _|_, baseCommit?: _|_, targetCommit?: _|_, rows?: _|_}) | close({_GitReportBase, status: "blocked", code: "scanner_finding", externalScanner: #GitExternalScanner & #ScannerCompleteCoverage, returned: 0, scope?: _|_, baseCommit?: _|_, targetCommit?: _|_, rows?: _|_})
#GitIncompleteReport: close({_GitReportBase, status: "incomplete", code: "invalid" | "binary_unavailable" | "hash_mismatch" | "deadline" | "execution_incomplete" | "output_limit" | "cleanup_failed" | "config_invalid" | "format_unsupported" | "git_incomplete" | "metadata_invalid" | "payload_limit" | "preparation_failed" | "result_limit" | "scanner_incomplete" | "source_invalid" | "state_drift" | "unsupported" | "version_mismatch", returned: 0, scope?: _|_, baseCommit?: _|_, targetCommit?: _|_, rows?: _|_})
#GitReport: #GitCompleteStatus | #GitCompleteDiff | #GitCompleteHistory | #GitBlockedReport | #GitIncompleteReport

// Compact variants close every header and tuple position. A native output is
// validated against #OutputReport or #OutputGitReport, respectively.
#CompactRow: [int & >=0 & <32, int & >=1 & <=4194305, string]
#CompactReport: close({
	version: "command-guard.compact-report/1"
	rowFormat: "file-line-text/1"
	status: "no-findings"
	code: "ok"
	noActionAuthority: true
	filesRead: int & >=0 & <=32
	bytesRead: int & >=0 & <=67108865
	bytesScanned: int & >=0 & <=67108864
	scope: "selected-files-only"
	scanner: "configured-regex-only" | "configured-regex-and-gitleaks"
	externalScanner?: #ExternalScanner & #ScannerCompleteCoverage
	externalClassifier?: #ClassifierComplete
	limitation: "not-proof-of-secret-or-pii-absence"
	totalMatches: int & >=0
	returned: int & >=0 & <=1000
	omitted: int & >=0
	truncated: bool
	rows: [...#CompactRow] & list.MaxItems(1000)
})
#OutputReport: #Report | #CompactReport

#StatusTuple: ["tracked" | "untracked", _path, string]
#DiffTuple: [_path, "" | _oid, "" | _oid, [...string] & list.MaxItems(8), [...string] & list.MaxItems(8), int & >=0, int & >=0]
#HistoryTuple: [_oid, [..._oid] & list.MaxItems(32), string, string, string, int & >=0]
_CompactGitBase: {
	version: "command-guard.compact-git-report/1"
	status: "no-findings"
	code: "ok"
	noActionAuthority: true
	scanner: "configured-regex-only" | "configured-regex-and-gitleaks"
	externalScanner?: #GitExternalScanner & #ScannerCompleteCoverage
	attemptedCalls: int & >=0
	startedCalls: int & >=0
	completedCalls: int & >=0
	bytesScanned: int & >=0
	returned: int & >=0 & <=1000
	omitted: int & >=0
	omittedLines: int & >=0
	omittedBytes: int & >=0
	projectionTruncated: bool
	hasMore: bool
	nonAtomic: bool
	limitation: string
}
#CompactGitStatus: close({_CompactGitBase, operation: "git-status", rowFormat: "status/1", scope: [...(_path | ".")] & list.MinItems(1) & list.MaxItems(32), rows: [...#StatusTuple] & list.MaxItems(1000)})
#CompactGitDiff: close({_CompactGitBase, operation: "git-object-diff", rowFormat: "object-diff/1", scope: [...(_path | ".")] & list.MinItems(1) & list.MaxItems(32), baseCommit: _oid, targetCommit: _oid, rows: [...#DiffTuple] & list.MaxItems(1000)})
#CompactGitHistory: close({_CompactGitBase, operation: "git-history", rowFormat: "history/1", targetCommit: _oid, rows: [...#HistoryTuple] & list.MaxItems(1000)})
#CompactGitReport: #CompactGitStatus | #CompactGitDiff | #CompactGitHistory
#OutputGitReport: #GitReport | #CompactGitReport
