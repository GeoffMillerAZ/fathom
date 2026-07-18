// Package investigationtemplate defines the CUE-first contracts for named
// investigation templates. Go types in types.go mirror these definitions; any
// drift is a bug.
package investigationtemplate

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #Template is a named, reusable investigation structure. Templates codify the
// expected steps for a class of investigation (e.g. "gap-analysis",
// "regression-analysis") so investigations start from a consistent baseline.
#Template: {
	// Name uniquely identifies this template.
	Name: _nonEmpty

	// Steps is an ordered list of investigation step labels. Each step label
	// must be non-empty. The list itself may not be empty.
	Steps: [_nonEmpty, ...string & !=""]
}
