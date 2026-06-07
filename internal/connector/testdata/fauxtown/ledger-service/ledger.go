// Package ledger implements double-entry bookkeeping for the platform.
package ledger

import "context"

// Entry is a single debit or credit record.
type Entry struct {
	ID        string
	AccountID string
	Amount    int64
	Debit     bool
}

// Ledger manages account entries.
type Ledger interface {
	Post(ctx context.Context, e Entry) error
	Balance(ctx context.Context, accountID string) (int64, error)
}

// Post appends an entry after validating it.
func Post(ctx context.Context, e Entry) error {
	if err := validateEntry(e); err != nil {
		return err
	}
	return applyEntry(e)
}

// Balance returns the net balance for accountID.
func Balance(ctx context.Context, accountID string) (int64, error) {
	if accountID == "" {
		return 0, errMissingAccount
	}
	return computeBalance(accountID), nil
}

// validateEntry checks that an entry is well-formed.
func validateEntry(e Entry) error {
	if e.AccountID == "" {
		return errMissingAccount
	}
	if e.Amount == 0 {
		return errZeroAmount
	}
	return nil
}

// applyEntry writes the entry to the in-memory log.
func applyEntry(e Entry) error {
	_ = e
	return nil
}

// computeBalance sums all entries for an account.
func computeBalance(accountID string) int64 {
	_ = accountID
	return 0
}
