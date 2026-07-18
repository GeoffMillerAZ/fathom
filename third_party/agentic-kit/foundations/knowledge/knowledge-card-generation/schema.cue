// Package knowledgecardgeneration defines the CUE-first contract for a
// KnowledgeCard. The Go struct KnowledgeCard in types.go mirrors this
// definition. Any drift between the two is a bug.
package knowledgecardgeneration

// #KnowledgeCard is a typed, provenance-tagged unit of knowledge derived from
// one or more parsed document segments.
#KnowledgeCard: {
	// Title is a non-empty human-readable summary of the card.
	Title: string & !=""

	// Fields holds the extracted key/value pairs. Keys and values are strings.
	// The map may be empty (no fields extracted) but must not be absent.
	Fields: {[string]: string}

	// SourceID is the non-empty identifier of the source document.
	SourceID: string & !=""
}
