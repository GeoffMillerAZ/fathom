package correctionledger

import "fmt"

// validate returns an error if the Correction violates any invariant enforced
// by schema.cue: all required string fields must be non-empty, Confidence must
// be in [1,5], and StableTarget must be non-empty.
//
// This mirrors the CUE constraints exactly so that runtime-constructed values
// receive the same protection as CUE-vetted testdata.
func (c Correction) validate() error {
	if c.FindingID == "" {
		return fmt.Errorf("correctionledger: Correction.FindingID must not be empty")
	}
	if c.Target.StableTarget == "" {
		return fmt.Errorf("correctionledger: Correction.Target.StableTarget must not be empty")
	}
	if c.Original == "" {
		return fmt.Errorf("correctionledger: Correction.Original must not be empty")
	}
	if c.Corrected == "" {
		return fmt.Errorf("correctionledger: Correction.Corrected must not be empty")
	}
	if c.CorrectedBy == "" {
		return fmt.Errorf("correctionledger: Correction.CorrectedBy must not be empty")
	}
	if c.Reason == "" {
		return fmt.Errorf("correctionledger: Correction.Reason must not be empty")
	}
	if c.Confidence < 1 || c.Confidence > 5 {
		return fmt.Errorf("correctionledger: Correction.Confidence must be 1–5, got %d", c.Confidence)
	}
	return nil
}

// validate returns an error if the Dismissal violates any invariant enforced
// by schema.cue: FindingID, DismissedBy, and Reason must all be non-empty.
func (d Dismissal) validate() error {
	if d.FindingID == "" {
		return fmt.Errorf("correctionledger: Dismissal.FindingID must not be empty")
	}
	if d.DismissedBy == "" {
		return fmt.Errorf("correctionledger: Dismissal.DismissedBy must not be empty")
	}
	if d.Reason == "" {
		return fmt.Errorf("correctionledger: Dismissal.Reason must not be empty")
	}
	return nil
}
