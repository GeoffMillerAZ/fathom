// Package prompttemplate defines the CUE-first contract for versioned prompt
// templates. Go types in types.go mirror these definitions; any drift is a
// bug. Behaviour (rendering, hashing, provenance mapping) lives entirely in
// Go — CUE owns only the shape of a Template.
package prompttemplate

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #Template is a named, versioned Go text/template source for a prompt.
// Go mirror: Template in types.go.
#Template: {
	// Name identifies the template (e.g. "summarize-incident").
	Name: _nonEmpty

	// Version is the semver-style version of the template text. It flows
	// through rendering into aiprovenance.AIAuthor.PromptTemplateVersion.
	Version: _nonEmpty

	// Text is the Go text/template source. Whether Text parses as a valid
	// text/template is a behavioural concern checked by Go
	// (Template.Validate); CUE only requires it to be non-empty.
	Text: _nonEmpty
}
