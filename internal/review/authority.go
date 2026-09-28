package review

// receiptAuthority is the fixed boundary statement every RiskReceipt
// carries (ADR-040; refused: CX-X2, "posting to a forge, approving or
// blocking on fathom's authority"): fathom's own authority stops at citing
// evidence and naming a band. It never posts a comment, approves, or
// blocks a change on its own — a receipt is evidence handed to something
// that decides, never the decision itself. CI (or whatever consumes
// `fathom review --receipt --format sarif|check`) decides what a `high`
// band does.
//
// This is enforced two ways: this string is carried on every
// RiskReceipt.Authority (BuildRiskReceipt never omits it), and structurally
// — TestReceiptNeverPostsOrBlocks asserts this package imports nothing
// capable of reaching a network or a forge API, so there is no code path
// here that COULD post, approve or block even if a caller tried to make it.
func receiptAuthority() string {
	return "fathom never posts, approves or blocks on its own authority; CI decides what a band does (ADR-040)"
}
