// Package reliabilitystats — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidObservation_EmptyTask:
//     task: "" violates string & !="" — a trial must name its cluster
//
//   invalidInterval_HighBelowLow:
//     high: 0.1 violates number & >=low — an interval cannot invert
//
//   invalidInterval_ZeroTasks:
//     tasks: 0 violates int & >=1 — an interval over no clusters is not an interval
//
//   invalidWilcoxon_PAboveOne:
//     p: 1.5 violates number & >=0 & <=1 — a p-value is a probability
//
//   invalidWilcoxon_ZeroN:
//     n: 0 violates int & >=1 — the test is undefined on no pairs (ErrNoPairs)
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package reliabilitystats

// // invalidObservation_EmptyTask — a trial must name its cluster
// invalidObservation_EmptyTask: #Observation & {
// 	task: ""
// 	diff: 0.1
// }

// // invalidInterval_HighBelowLow — high must be >= low
// invalidInterval_HighBelowLow: #Interval & {
// 	low:   0.3
// 	high:  0.1
// 	point: 0.2
// 	tasks: 4
// }

// // invalidInterval_ZeroTasks — tasks must be >= 1
// invalidInterval_ZeroTasks: #Interval & {
// 	low:   0.0
// 	high:  0.0
// 	point: 0.0
// 	tasks: 0
// }

// // invalidWilcoxon_PAboveOne — p must be in [0, 1]
// invalidWilcoxon_PAboveOne: #WilcoxonResult & {
// 	w:       3
// 	w_plus:  12
// 	w_minus: 3
// 	n:       5
// 	zeros:   0
// 	p:       1.5
// 	exact:   true
// }

// // invalidWilcoxon_ZeroN — n must be >= 1
// invalidWilcoxon_ZeroN: #WilcoxonResult & {
// 	w:       0
// 	w_plus:  0
// 	w_minus: 0
// 	n:       0
// 	zeros:   3
// 	p:       1
// 	exact:   true
// }
