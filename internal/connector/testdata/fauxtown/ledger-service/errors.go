// Package ledger — sentinel errors.
package ledger

import "errors"

var (
	errMissingAccount = errors.New("ledger: account ID is required")
	errZeroAmount     = errors.New("ledger: amount must be non-zero")
)
