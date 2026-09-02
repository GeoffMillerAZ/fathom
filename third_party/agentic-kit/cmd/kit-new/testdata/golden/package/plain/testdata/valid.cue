// valid.cue — positive fixtures for the widgetgizmo schema; `cue vet ./testdata/`
// from the package directory checks them against ../schema.cue.
package widgetgizmo

minimal: #Config & {
	name:  "example"
	limit: 1
}
