// invalid.cue — demonstrates #Signal and #Assessment values that VIOLATE the
// schema. This file is intentionally NOT included in `cue vet` (it is
// intentionally invalid CUE). It documents the constraints enforced by
// schema.cue and their Go-level counterparts.
//
// To verify the schema rejects a binding, temporarily uncomment it and run:
//
//	cue vet ./testdata/   (expect errors for that binding)
//
// or:
//
//	cue eval -e '<name>' ./foundations/evidence/authenticity/testdata/
//
// The following errors would be produced by `cue vet ./testdata/`:
//
//	_badMethod: "x-ray_scan" is not one of the seven recognised #Method values.
//	_badVerdict: "forgery" is not one of the six recognised #Verdict values.
//	_badSignalEmptyID: "" violates string & !="" (ID must be non-empty).
//	_badSignalEmptyRecordID: "" violates string & !="" (RecordID must be non-empty).
//	_badSignalScoreHigh: Score 1.5 violates <=1.
//	_badSignalScoreLow: Score -0.1 violates >=0.
//	_badSignalEmptyAt: "" violates string & !="" (At must be non-empty).
//	_badAssessmentConfHigh: Confidence 1.1 violates <=1.
//	_badAssessmentConfLow: Confidence -0.05 violates >=0.
//	_badAssessmentEmptyRecordID: "" violates string & !="" (RecordID must be non-empty).
package authenticity

// _badMethod: "x-ray_scan" is not a recognised #Method.
// _badMethod: #Method & "x-ray_scan"

// _badVerdict: "forgery" is not a recognised #Verdict.
// _badVerdict: #Verdict & "forgery"

// _badSignalEmptyID: Signal.ID must be non-empty.
// _badSignalEmptyID: #Signal & {
// 	ID:       ""
// 	RecordID: "rec-1"
// 	Method:   "hash_match"
// 	Verdict:  "authentic"
// 	Score:    0.9
// 	Detail:   ""
// 	At:       "2026-06-08T10:00:00Z"
// }

// _badSignalEmptyRecordID: Signal.RecordID must be non-empty.
// _badSignalEmptyRecordID: #Signal & {
// 	ID:       "s-bad"
// 	RecordID: ""
// 	Method:   "hash_match"
// 	Verdict:  "authentic"
// 	Score:    0.9
// 	Detail:   ""
// 	At:       "2026-06-08T10:00:00Z"
// }

// _badSignalScoreHigh: Score > 1 violates the <=1 constraint.
// _badSignalScoreHigh: #Signal & {
// 	ID:       "s-bad-hi"
// 	RecordID: "rec-1"
// 	Method:   "c2pa"
// 	Verdict:  "authentic"
// 	Score:    1.5
// 	Detail:   ""
// 	At:       "2026-06-08T10:00:00Z"
// }

// _badSignalScoreLow: Score < 0 violates the >=0 constraint.
// _badSignalScoreLow: #Signal & {
// 	ID:       "s-bad-lo"
// 	RecordID: "rec-1"
// 	Method:   "c2pa"
// 	Verdict:  "tampered"
// 	Score:    -0.1
// 	Detail:   ""
// 	At:       "2026-06-08T10:00:00Z"
// }

// _badSignalEmptyAt: Signal.At must be a non-empty string (RFC 3339 timestamp).
// _badSignalEmptyAt: #Signal & {
// 	ID:       "s-bad-at"
// 	RecordID: "rec-1"
// 	Method:   "manual_review"
// 	Verdict:  "inconclusive"
// 	Score:    0.5
// 	Detail:   ""
// 	At:       ""
// }

// _badAssessmentConfHigh: Assessment.Confidence > 1 violates <=1.
// _badAssessmentConfHigh: #Assessment & {
// 	RecordID:     "rec-bad"
// 	Overall:      "authentic"
// 	Confidence:   1.1
// 	Signals:      []
// 	Conservative: false
// }

// _badAssessmentConfLow: Assessment.Confidence < 0 violates >=0.
// _badAssessmentConfLow: #Assessment & {
// 	RecordID:     "rec-bad"
// 	Overall:      "tampered"
// 	Confidence:   -0.05
// 	Signals:      []
// 	Conservative: true
// }

// _badAssessmentEmptyRecordID: Assessment.RecordID must be non-empty.
// _badAssessmentEmptyRecordID: #Assessment & {
// 	RecordID:     ""
// 	Overall:      "inconclusive"
// 	Confidence:   0.5
// 	Signals:      []
// 	Conservative: false
// }
