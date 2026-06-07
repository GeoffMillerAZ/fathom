// Package payment — sentinel errors.
package payment

import "errors"

var (
	errMissingAccount    = errors.New("payment: account ID is required")
	errNonPositiveAmount = errors.New("payment: amount must be positive")
)
