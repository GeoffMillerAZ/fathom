// invalid.cue — negative fixtures, documented commented-out so live
// `cue vet` stays green; TestConfigValidate in widget-gizmo_test.go asserts each
// is rejected in Go.
package widgetgizmo

// 1. empty name:
// bad1: #Config & {name: "", limit: 1}

// 2. non-positive limit:
// bad2: #Config & {name: "example", limit: 0}
