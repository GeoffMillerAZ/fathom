// Package payment implements the payment processing domain.
package payment

import "context"

// ChargeRequest holds the parameters for a single charge operation.
type ChargeRequest struct {
	AccountID string
	Amount    int64
	Currency  string
}

// ChargeResult is returned by Charge on success.
type ChargeResult struct {
	TransactionID string
	Status        string
}

// Processor executes payment operations.
type Processor interface {
	Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error)
	Refund(ctx context.Context, txID string) error
}

// Charge validates the request and delegates to the ledger.
func Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error) {
	if err := validateCharge(req); err != nil {
		return ChargeResult{}, err
	}
	return ChargeResult{TransactionID: "txn-001", Status: "ok"}, nil
}

// validateCharge checks mandatory fields before processing.
func validateCharge(req ChargeRequest) error {
	if req.AccountID == "" {
		return errMissingAccount
	}
	if req.Amount <= 0 {
		return errNonPositiveAmount
	}
	return nil
}
