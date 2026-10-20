package project

// INVALID examples, kept commented so `cue vet ./...` stays green. The
// `project-negatives` step of `task cue-vet` asserts each fails.
//
// 1. A key #Project does not define (#Project is closed):
// bad1: #Project & {
// 	made_up_key: true
// }
//
// 2. A key that fails its sub-schema (budget must be > 0):
// bad2: #Project & {
// 	budgets: list: 0
// }
//
// 3. A concern with an unknown check kind:
// bad3: #Project & {
// 	concerns: c: check: kind: "nope"
// }
//
// 4. A generated declaration with no source (the tech pack shape needs one):
// bad4: #Project & {
// 	generated: [{output: "ui/app.css", source: [], regenerate: "task ui:css"}]
// }
