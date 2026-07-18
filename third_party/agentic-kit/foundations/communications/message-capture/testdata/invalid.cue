// This file documents values that must NOT satisfy #Message.
// It is intentionally commented out so that `cue vet ./...` passes on
// the whole module while still recording the invalid shapes for reference.
//
// Uncomment individual blocks to confirm the schema rejects them.
//
// package messagecapture
//
// // missing_id — ID field is absent.
// missing_id: #Message & {
// 	Channel: "general"
// 	Author:  "alice"
// 	Text:    "hi"
// 	At:      "2024-03-01T09:00:00Z"
// }
//
// // empty_id — ID is present but empty string, violating !="".
// empty_id: #Message & {
// 	ID:      ""
// 	Channel: "general"
// 	Author:  "alice"
// 	Text:    "hi"
// 	At:      "2024-03-01T09:00:00Z"
// }
//
// // empty_channel — Channel is an empty string.
// empty_channel: #Message & {
// 	ID:      "msg-003"
// 	Channel: ""
// 	Author:  "alice"
// 	Text:    "hi"
// 	At:      "2024-03-01T09:00:00Z"
// }
//
// // empty_author — Author is an empty string.
// empty_author: #Message & {
// 	ID:      "msg-004"
// 	Channel: "general"
// 	Author:  ""
// 	Text:    "hi"
// 	At:      "2024-03-01T09:00:00Z"
// }
//
// // empty_at — At is an empty string.
// empty_at: #Message & {
// 	ID:      "msg-005"
// 	Channel: "general"
// 	Author:  "alice"
// 	Text:    "hi"
// 	At:      ""
// }
