package bedrock

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"io"
	"testing"
)

// encodeEventStreamFrame builds one AWS event-stream frame with the given
// string headers and payload, computing both CRCs exactly as a real Bedrock
// response does. It is the test-side inverse of eventStreamReader.next, so the
// decoder is exercised against bytes built independently of it.
func encodeEventStreamFrame(headers map[string]string, payload []byte) []byte {
	var hb bytes.Buffer
	for name, value := range headers {
		hb.WriteByte(byte(len(name)))
		hb.WriteString(name)
		hb.WriteByte(headerTypeString)
		var l [2]byte
		binary.BigEndian.PutUint16(l[:], uint16(len(value)))
		hb.Write(l[:])
		hb.WriteString(value)
	}
	headerBytes := hb.Bytes()

	totalLen := uint32(preludeLen + len(headerBytes) + len(payload) + 4)

	var prelude [preludeLen]byte
	binary.BigEndian.PutUint32(prelude[0:4], totalLen)
	binary.BigEndian.PutUint32(prelude[4:8], uint32(len(headerBytes)))
	binary.BigEndian.PutUint32(prelude[8:12], crc32.ChecksumIEEE(prelude[0:8]))

	var frame bytes.Buffer
	frame.Write(prelude[:])
	frame.Write(headerBytes)
	frame.Write(payload)
	msgCRC := crc32.ChecksumIEEE(frame.Bytes())
	var crcBytes [4]byte
	binary.BigEndian.PutUint32(crcBytes[:], msgCRC)
	frame.Write(crcBytes[:])
	return frame.Bytes()
}

// chunkFrame wraps an inner Anthropic event (raw JSON) as a Bedrock "chunk"
// event: payload is {"bytes":"<base64 of inner>"}, headers mark it event/chunk.
func chunkFrame(t *testing.T, innerJSON string) []byte {
	t.Helper()
	env := map[string]string{"bytes": base64Std.EncodeToString([]byte(innerJSON))}
	payload, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return encodeEventStreamFrame(map[string]string{
		":message-type": "event",
		":event-type":   "chunk",
		":content-type": "application/json",
	}, payload)
}

// exceptionFrame builds a Bedrock exception event frame.
func exceptionFrame(t *testing.T, exceptionType, message string) []byte {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"message": message})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return encodeEventStreamFrame(map[string]string{
		":message-type":   "exception",
		":exception-type": exceptionType,
		":content-type":   "application/json",
	}, payload)
}

// TestEventStreamRoundTrip proves the decoder reads back exactly what the
// encoder wrote: headers and payload survive the binary framing + CRCs.
func TestEventStreamRoundTrip(t *testing.T) {
	frames := bytes.Join([][]byte{
		chunkFrame(t, `{"type":"message_start"}`),
		exceptionFrame(t, "ThrottlingException", "slow down"),
	}, nil)

	r := newEventStreamReader(bytes.NewReader(frames))

	m1, err := r.next()
	if err != nil {
		t.Fatalf("frame 1: %v", err)
	}
	if m1.eventType != "chunk" || m1.messageType != "event" {
		t.Errorf("frame 1 headers wrong: %+v", m1)
	}
	inner, err := unwrapChunk(m1.payload)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	if string(inner) != `{"type":"message_start"}` {
		t.Errorf("inner = %q", inner)
	}

	m2, err := r.next()
	if err != nil {
		t.Fatalf("frame 2: %v", err)
	}
	if m2.messageType != "exception" || m2.exceptionType != "ThrottlingException" {
		t.Errorf("frame 2 headers wrong: %+v", m2)
	}

	if _, err := r.next(); err != io.EOF {
		t.Errorf("want io.EOF at end, got %v", err)
	}
}

// TestEventStreamCorruptCRC proves a tampered frame fails the CRC check rather
// than silently decoding garbage.
func TestEventStreamCorruptCRC(t *testing.T) {
	frame := chunkFrame(t, `{"type":"ping"}`)
	// Flip a byte in the payload region (after the prelude) so the message CRC
	// no longer matches.
	frame[len(frame)-6] ^= 0xff
	r := newEventStreamReader(bytes.NewReader(frame))
	if _, err := r.next(); err == nil {
		t.Fatal("want CRC error, got nil")
	}
}

// TestEventStreamTruncated proves a frame cut short surfaces as
// io.ErrUnexpectedEOF (a truncated frame, not a clean boundary).
func TestEventStreamTruncated(t *testing.T) {
	frame := chunkFrame(t, `{"type":"ping"}`)
	r := newEventStreamReader(bytes.NewReader(frame[:len(frame)-3]))
	if _, err := r.next(); err != io.ErrUnexpectedEOF {
		t.Errorf("want io.ErrUnexpectedEOF, got %v", err)
	}
}
