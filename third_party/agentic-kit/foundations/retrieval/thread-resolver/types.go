// Package threadresolver groups messages by a thread key and returns ordered
// threads. Grouping and ordering are fully deterministic: no randomness, no ML.
// Messages within a thread preserve their original slice order; threads
// themselves are ordered by the first occurrence of their key in the input.
//
// Concurrency: Resolve is a pure function and safe for concurrent use.
package threadresolver

// Message is a single message to be resolved into a thread.
type Message struct {
	// ThreadKey is the value used to group messages into a thread.
	// Messages with the same ThreadKey belong to the same thread.
	ThreadKey string

	// ID is a stable, unique identifier for the message.
	ID string

	// Body is the text content of the message.
	Body string
}

// Thread is an ordered group of Messages sharing the same ThreadKey.
type Thread struct {
	// Key is the shared ThreadKey for all messages in this thread.
	Key string

	// Messages is the ordered list of messages, preserving original order.
	Messages []Message
}

// Resolve groups msgs into threads keyed by Message.ThreadKey.
// Threads are returned in the order their key first appeared in msgs.
// Messages within a thread appear in their original slice order.
// Messages with an empty ThreadKey are grouped under the "" key.
// An empty (non-nil) slice is returned for an empty input.
func Resolve(msgs []Message) []Thread {
	if len(msgs) == 0 {
		return []Thread{}
	}

	// keyOrder tracks first-seen ordering of thread keys.
	keyOrder := make([]string, 0)
	groups := make(map[string][]Message)

	for _, m := range msgs {
		if _, exists := groups[m.ThreadKey]; !exists {
			keyOrder = append(keyOrder, m.ThreadKey)
		}
		groups[m.ThreadKey] = append(groups[m.ThreadKey], m)
	}

	threads := make([]Thread, 0, len(keyOrder))
	for _, key := range keyOrder {
		threads = append(threads, Thread{
			Key:      key,
			Messages: groups[key],
		})
	}
	return threads
}
