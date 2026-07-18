// Package messagecapture defines the CUE-first contract for captured
// channel messages in the communications foundation. The Go types in
// types.go mirror these definitions. CUE is authoritative for valid value
// spaces; Go is authoritative for runtime behaviour.
package messagecapture

// #Message is a single message received from a communication channel.
// Every captured message in the system must satisfy this definition.
#Message: {
	// ID is a unique, non-empty identifier for the message within its channel.
	ID!: string & !=""

	// Channel names the source channel from which the message was captured.
	Channel!: string & !=""

	// Author identifies the sender of the message. Must be non-empty.
	Author!: string & !=""

	// Text is the message body. May be empty (e.g. attachment-only posts) but
	// the field must be present.
	Text!: string

	// At is an RFC 3339 timestamp recording when the message was sent.
	At!: string & !=""
}
