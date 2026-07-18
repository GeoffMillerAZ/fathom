package messagecapture

// valid_message is a well-formed #Message used to verify the schema
// accepts correct values.
valid_message: #Message & {
	ID:      "msg-001"
	Channel: "general"
	Author:  "alice"
	Text:    "Hello, team!"
	At:      "2024-03-01T09:00:00Z"
}

// empty_text is valid — attachment-only messages may have no body text.
empty_text_message: #Message & {
	ID:      "msg-002"
	Channel: "uploads"
	Author:  "bob"
	Text:    ""
	At:      "2024-03-01T09:01:00Z"
}
