// valid.cue — concrete values that satisfy the #Template constraint defined in
// schema.cue. Running `cue vet ./...` from the package root validates these
// against the schema.
package investigationtemplate

// A standard gap-analysis template with four ordered steps.
_gapAnalysis: #Template & {
	Name: "gap-analysis"
	Steps: ["scope-definition", "evidence-collection", "gap-identification", "remediation-proposal"]
}

// A regression-analysis template for investigating regressions.
_regressionAnalysis: #Template & {
	Name: "regression-analysis"
	Steps: ["baseline-capture", "change-identification", "impact-analysis", "root-cause", "fix-proposal"]
}

// A minimal template with a single step.
_quickCheck: #Template & {
	Name:  "quick-check"
	Steps: ["verify-ownership"]
}
