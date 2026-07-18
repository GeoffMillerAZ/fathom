package bedrock

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
)

// AWS event-stream (vnd.amazon.eventstream) frame decoder, hand-rolled on the
// standard library. InvokeModelWithResponseStream does NOT use Server-Sent
// Events — it uses AWS's binary "message" framing. Each frame is:
//
//	[total_len uint32][headers_len uint32][prelude_crc uint32]
//	[headers headers_len bytes][payload total_len-headers_len-16 bytes]
//	[message_crc uint32]
//
// all big-endian. prelude_crc is CRC32 of the first 8 bytes; message_crc is
// CRC32 of everything before it (the whole frame minus the trailing 4 bytes).
//
// Each header is [name_len uint8][name][value_type uint8][value...]. The only
// value type that appears on the Bedrock response stream is 7 (string:
// [len uint16][bytes]); we decode that and skip anything else defensively.
//
// The headers we care about: ":event-type" (e.g. "chunk"), ":message-type"
// ("event" or "exception"), and ":exception-type". The payload of a normal
// "chunk" event is JSON {"bytes":"<base64>"} whose decoded bytes are ONE
// Anthropic streaming event object (the same shape the public API delivers over
// SSE: message_start, content_block_delta, message_stop, ...). So the Bedrock
// stream is "Anthropic SSE JSON, re-framed in AWS binary envelopes": decode the
// envelope here, hand the inner JSON to the shared event handler.

const (
	// frameOverhead is the fixed bytes around the headers+payload: prelude
	// (12) + message CRC (4).
	frameOverhead = 16

	// preludeLen is total_len + headers_len + prelude_crc.
	preludeLen = 12

	// headerTypeString is the only event-stream header value type Bedrock
	// uses on the response stream.
	headerTypeString = 7

	// maxFrameLen caps a single event-stream frame. 32 MiB matches maxBodyBytes
	// — a larger frame is a malfunctioning/proxied server, not a real event.
	maxFrameLen = maxBodyBytes
)

// crc32IEEE is the polynomial AWS event-stream uses (the standard IEEE/zlib
// table, same as crc32.ChecksumIEEE).
var crc32Table = crc32.MakeTable(crc32.IEEE)

// eventMessage is one decoded event-stream frame.
type eventMessage struct {
	eventType     string // ":event-type", e.g. "chunk"
	messageType   string // ":message-type": "event" or "exception"
	exceptionType string // ":exception-type", set when messageType == "exception"
	payload       []byte // raw payload bytes (JSON)
}

// errFrameTooLarge reports an event-stream frame whose declared length exceeds
// the cap; like the SSE caps in the anthropic adapter it maps to a typed
// retryable ServerError rather than unbounded buffering.
var errFrameTooLarge = fmt.Errorf("event-stream frame exceeds the %d-byte cap", maxFrameLen)

// eventStreamReader decodes consecutive event-stream frames from an io.Reader.
type eventStreamReader struct {
	r io.Reader
}

func newEventStreamReader(r io.Reader) *eventStreamReader {
	return &eventStreamReader{r: r}
}

// next reads and decodes the next frame. It returns io.EOF at a clean frame
// boundary when the stream ends. A malformed frame (bad CRC, short read,
// oversize) returns a non-EOF error.
func (e *eventStreamReader) next() (eventMessage, error) {
	prelude := make([]byte, preludeLen)
	if _, err := io.ReadFull(e.r, prelude); err != nil {
		// io.EOF here is a clean boundary; io.ErrUnexpectedEOF is a truncated
		// prelude (surfaced as-is so the producer can classify it).
		return eventMessage{}, err
	}

	totalLen := binary.BigEndian.Uint32(prelude[0:4])
	headersLen := binary.BigEndian.Uint32(prelude[4:8])
	preludeCRC := binary.BigEndian.Uint32(prelude[8:12])

	if crc32.Checksum(prelude[0:8], crc32Table) != preludeCRC {
		return eventMessage{}, fmt.Errorf("event-stream: prelude CRC mismatch")
	}
	if totalLen > maxFrameLen {
		return eventMessage{}, errFrameTooLarge
	}
	if uint64(headersLen)+frameOverhead > uint64(totalLen) {
		return eventMessage{}, fmt.Errorf("event-stream: headers length %d overflows frame length %d", headersLen, totalLen)
	}

	// Remaining = everything after the prelude: headers + payload + message
	// CRC.
	remaining := make([]byte, int(totalLen)-preludeLen)
	if _, err := io.ReadFull(e.r, remaining); err != nil {
		if err == io.EOF {
			return eventMessage{}, io.ErrUnexpectedEOF
		}
		return eventMessage{}, err
	}

	// Verify the message CRC over the whole frame except its trailing 4 bytes.
	msgCRC := binary.BigEndian.Uint32(remaining[len(remaining)-4:])
	hasher := crc32.New(crc32Table)
	hasher.Write(prelude)
	hasher.Write(remaining[:len(remaining)-4])
	if hasher.Sum32() != msgCRC {
		return eventMessage{}, fmt.Errorf("event-stream: message CRC mismatch")
	}

	headerBytes := remaining[:headersLen]
	payload := remaining[headersLen : len(remaining)-4]

	msg := eventMessage{payload: payload}
	if err := parseEventHeaders(headerBytes, &msg); err != nil {
		return eventMessage{}, err
	}
	return msg, nil
}

// parseEventHeaders walks the header block, populating the fields of msg we
// care about. Unknown header names and non-string value types are skipped (the
// value is consumed so parsing stays in sync).
func parseEventHeaders(b []byte, msg *eventMessage) error {
	for len(b) > 0 {
		nameLen := int(b[0])
		b = b[1:]
		if len(b) < nameLen+1 {
			return fmt.Errorf("event-stream: truncated header name")
		}
		name := string(b[:nameLen])
		b = b[nameLen:]
		valueType := b[0]
		b = b[1:]
		if valueType != headerTypeString {
			// Only string-valued headers appear on this stream; bail rather
			// than guess the width of an unexpected type and desync.
			return fmt.Errorf("event-stream: unexpected header value type %d for %q", valueType, name)
		}
		if len(b) < 2 {
			return fmt.Errorf("event-stream: truncated header value length")
		}
		valLen := int(binary.BigEndian.Uint16(b[:2]))
		b = b[2:]
		if len(b) < valLen {
			return fmt.Errorf("event-stream: truncated header value")
		}
		value := string(b[:valLen])
		b = b[valLen:]

		switch name {
		case ":event-type":
			msg.eventType = value
		case ":message-type":
			msg.messageType = value
		case ":exception-type", ":error-code":
			msg.exceptionType = value
		}
	}
	return nil
}
