// Package mcp — the elision disclosure block (docs/design/budgeted-answers.md
// §2, BA-1/BA-2).
//
// Every tool that can drop results to fit a budget must say so: "12 of 47"
// is only useful if the reader also knows the ranking and how to get the
// rest. elisionFor is the ONE constructor for that block — the arithmetic
// (count + returned == total) is enforced here, at construction, so it
// cannot drift from reality at a call site (BA-2). Silent truncation is a
// defect, always (design §2 rule 3).
package mcp

// elided is the disclosure block a truncating tool attaches to its response
// when it dropped results to fit a budget.
type elided struct {
	Count        int    `json:"count"`
	Of           int    `json:"of"`
	Ordering     string `json:"ordering"`
	HowToGetMore string `json:"how_to_get_more"`
}

// elisionFor constructs the disclosure block for a tool that returned
// `returned` of `total` available results. Returns nil when nothing was
// dropped (returned >= total) — a complete answer carries no elided block.
// ordering and howToGetMore are the per-tool declared ranking and remedy
// (design §2 rule 2); both are required so a caller cannot construct a
// disclosure that names a count without saying what it means.
func elisionFor(returned, total int, ordering, howToGetMore string) *elided {
	if total <= returned {
		return nil
	}
	return &elided{
		Count:        total - returned,
		Of:           total,
		Ordering:     ordering,
		HowToGetMore: howToGetMore,
	}
}
