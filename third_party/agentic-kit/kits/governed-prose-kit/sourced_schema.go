package governedprosekit

import _ "embed"

// sourcedDraftSchema is the provider-facing mirror of #ProseDraft. Strict
// providers require all properties in required, so optional prose metadata is
// emitted as an empty value; CheckDraft remains the authoritative runtime gate.
//
//go:embed sourced-draft.schema.json
var sourcedDraftSchema []byte
