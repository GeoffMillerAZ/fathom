//// A NEGATIVE fixture, fully commented so cue-vet never evaluates it —
//// TestInvalidFixtureRefused strips one comment level and asserts the
//// loader refuses the result (these //// prose lines survive the strip
//// as comments; an uncheckable concern is a wish, and wishes are refused).
//
// concerns: {
// 	vibes: {
// 		want:  "the docs feel fresh"
// 		check: {kind: "feels_right"}
// 	}
// 	//// allow_import is agents_md_reaches_claude's field only (ADR-059):
// 	import_on_a_budget: {
// 		want:  "AGENTS.md stays short"
// 		check: {kind: "max_lines", file: "AGENTS.md", lines: 120, allow_import: false}
// 	}
// }
