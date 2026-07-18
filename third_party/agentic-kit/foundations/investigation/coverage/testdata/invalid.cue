// invalid.cue — NEGATIVE fixtures, documented (not live-vetted).
//
// These shapes intentionally violate schema.cue. They are kept commented so the
// repo gate (`cue vet ./...`) stays green; their REJECTION is asserted by the Go
// tests in coverage_test.go (Validate() must reject each). To verify a case at
// the CUE layer manually, uncomment it and run:
//   cue vet ./testdata   (expect errors)
//
// 1. _badEmptySourceID — SourceID violates _nonEmpty:
//      SourceID: "" -> field is required to be non-empty
//
// 2. _badNegativeKnown — RecordsKnown violates int & >=0:
//      RecordsKnown: -1 -> value must be >= 0
//
// 3. _badNegativeScanned — RecordsScanned violates int & >=0:
//      RecordsScanned: -1 -> value must be >= 0
//
// 4. _badNegativeFailed — RecordsFailed violates int & >=0:
//      RecordsFailed: -1 -> value must be >= 0
//
// 5. _badNegativePending — RecordsPending violates int & >=0:
//      RecordsPending: -1 -> value must be >= 0
//
// 6. _badFloatKnown — RecordsKnown violates int (float not allowed):
//      RecordsKnown: 3.14 -> conflicting values 3.14 and int (mismatched types float and int)
//
// 7. _badInvariantViolation — Scanned+Failed+Pending > Known violates _sumGuard:
//      RecordsScanned(7) + RecordsFailed(2) + RecordsPending(3) = 12 > RecordsKnown(10)
//      -> _sumGuard: conflicting values false and true

package coverage

import "github.com/geoffmilleraz/agentic-kit/foundations/investigation/coverage"

// _badEmptySourceID: empty SourceID violates the _nonEmpty constraint.
// _badEmptySourceID: coverage.#SourceCoverage & {
// 	SourceID:       ""
// 	RecordsKnown:   10
// 	RecordsScanned: 5
// 	RecordsFailed:  0
// 	RecordsPending: 5
// }

// _badNegativeKnown: RecordsKnown < 0 violates the int & >=0 constraint.
// _badNegativeKnown: coverage.#SourceCoverage & {
// 	SourceID:       "src-x"
// 	RecordsKnown:   -1
// 	RecordsScanned: 0
// 	RecordsFailed:  0
// 	RecordsPending: 0
// }

// _badNegativeScanned: RecordsScanned < 0 violates the int & >=0 constraint.
// _badNegativeScanned: coverage.#SourceCoverage & {
// 	SourceID:       "src-x"
// 	RecordsKnown:   10
// 	RecordsScanned: -1
// 	RecordsFailed:  0
// 	RecordsPending: 0
// }

// _badNegativeFailed: RecordsFailed < 0 violates the int & >=0 constraint.
// _badNegativeFailed: coverage.#SourceCoverage & {
// 	SourceID:       "src-x"
// 	RecordsKnown:   10
// 	RecordsScanned: 5
// 	RecordsFailed:  -1
// 	RecordsPending: 0
// }

// _badNegativePending: RecordsPending < 0 violates the int & >=0 constraint.
// _badNegativePending: coverage.#SourceCoverage & {
// 	SourceID:       "src-x"
// 	RecordsKnown:   10
// 	RecordsScanned: 5
// 	RecordsFailed:  0
// 	RecordsPending: -1
// }

// _badFloatKnown: RecordsKnown: 3.14 violates int — float values are rejected.
// Without the int constraint (old >=0-only schema), this would have silently
// accepted the float and truncated it to 3 on Go unmarshal.
// _badFloatKnown: coverage.#SourceCoverage & {
// 	SourceID:       "src-x"
// 	RecordsKnown:   3.14
// 	RecordsScanned: 0
// 	RecordsFailed:  0
// 	RecordsPending: 0
// }

// _badInvariantViolation: Scanned(7) + Failed(2) + Pending(3) = 12 > Known(10).
// The cross-field _sumGuard constraint in schema.cue encodes this as:
//   _sumGuard: RecordsScanned + RecordsFailed + RecordsPending <= RecordsKnown
//   _sumGuard: true
// CUE evaluates _sumGuard to false and rejects the unification.
// _badInvariantViolation: coverage.#SourceCoverage & {
// 	SourceID:       "src-x"
// 	RecordsKnown:   10
// 	RecordsScanned: 7
// 	RecordsFailed:  2
// 	RecordsPending: 3
// }
