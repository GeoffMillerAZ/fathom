// Package rulecore CUE contract.
// #PatternSpec and #Finding mirror the Go structs in types.go.
// CUE is the authority for shape; Go is the authority for behaviour.
package rulecore

// #PatternSpec is the input to Compile: a rule ID and a Go regexp pattern.
#PatternSpec: {
	// id is a unique, stable rule identifier (e.g. "INJ-001", "DLP-002").
	id: string & =~"^[A-Z]+-[0-9]+"

	// pattern is a valid Go regexp string.
	pattern: string & !=""
}

// #Finding is one rule match returned by ScanFirst.
#Finding: {
	// ruleID matches #PatternSpec.id of the rule that fired.
	ruleID: string & =~"^[A-Z]+-[0-9]+"

	// start and end are byte offsets into the scanned text.
	start: int & >=0
	end:   int & >=start

	// excerpt is a UTF-8-safe extract capped at 120 bytes.
	excerpt: string
}
