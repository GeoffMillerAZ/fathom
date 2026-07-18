// valid.cue — concrete values that satisfy the #Template constraint defined
// in schema.cue. Running `cue vet ./...` from the package root validates
// these against the schema.
package prompttemplate

// A minimal well-formed template with a single placeholder.
_summarize: #Template & {
	Name:    "summarize-incident"
	Version: "1.0.0"
	Text:    "Summarize the following incident report:\n\n{{.incident}}"
}

// A template with multiple placeholders and template logic.
_triage: #Template & {
	Name:    "triage-alerts"
	Version: "2.3.1"
	Text:    "Hello {{.name}}, you have {{.count}} alerts.{{if .urgent}} Act now.{{end}}"
}

// A static template with no placeholders is still valid — rendering it
// against an empty params map yields a deterministic ParamsHash over "{}".
_static: #Template & {
	Name:    "system-preamble"
	Version: "0.1.0"
	Text:    "You are a careful analyst. Cite evidence for every claim."
}
