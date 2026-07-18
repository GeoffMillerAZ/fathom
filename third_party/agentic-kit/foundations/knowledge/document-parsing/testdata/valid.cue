// valid.cue — concrete values that satisfy the #Segment constraint defined in
// schema.cue. Running `cue vet ./...` from the package root or the repo root
// validates these against the schema.
package documentparsing

// A preamble segment: heading is empty, text is present.
_preamble: #Segment & {
	Heading: ""
	Text:    "Introduction text before any heading."
	Order:   0
}

// A heading segment with body text.
_section1: #Segment & {
	Heading: "Overview"
	Text:    "This section describes the overview of the system."
	Order:   1
}

// A heading segment with no body text (heading only).
_section2: #Segment & {
	Heading: "References"
	Text:    ""
	Order:   2
}

// A deeply nested heading (###).
_section3: #Segment & {
	Heading: "Deep Nested Section"
	Text:    "Content under a deeply nested heading."
	Order:   3
}

// Minimum valid segment: all zero values (empty doc produces this).
_empty: #Segment & {
	Heading: ""
	Text:    ""
	Order:   0
}
