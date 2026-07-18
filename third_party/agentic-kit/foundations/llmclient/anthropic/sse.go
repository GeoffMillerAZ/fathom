package anthropic

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

// errSSETooLarge reports that the server sent a single SSE line, or
// accumulated more event data, than maxBodyBytes allows. consumeSSE maps it
// to a typed retryable ServerError; the cap exists so a malfunctioning (or
// malicious, proxied) server cannot grow memory without bound.
var errSSETooLarge = fmt.Errorf("SSE line or event data exceeds the %d-byte cap", maxBodyBytes)

// sseEvent is one parsed Server-Sent Event: the value of its last "event:"
// field and its "data:" lines joined with "\n" (per the SSE spec).
type sseEvent struct {
	name string
	data []byte
}

// sseReader is a hand-rolled SSE parser over a bufio.Reader. It deliberately
// avoids bufio.Scanner, whose default 64KB line cap would truncate large
// deltas. bufio.Reader.ReadBytes accumulates lines of any length and is
// insensitive to how the bytes arrive (arbitrary flush boundaries, UTF-8
// runes split across reads).
type sseReader struct {
	r *bufio.Reader
}

func newSSEReader(r io.Reader) *sseReader {
	return &sseReader{r: bufio.NewReader(r)}
}

var sseNewline = []byte("\n")

// next returns the next complete event (terminated by a blank line). Comment
// lines starting with ':' are skipped. It returns a read error — typically
// io.EOF on a severed stream — when the stream ends before an event
// boundary; a trailing partial event is dropped as incomplete. An event whose
// accumulated data exceeds maxBodyBytes fails with errSSETooLarge before more
// of it is buffered.
func (s *sseReader) next() (sseEvent, error) {
	var ev sseEvent
	var data [][]byte
	total := 0
	seen := false
	for {
		line, err := s.readLine()
		if err != nil {
			return sseEvent{}, err
		}
		if len(line) == 0 {
			if !seen {
				continue // blank line between events / leading blank lines
			}
			ev.data = bytes.Join(data, sseNewline)
			return ev, nil
		}
		if line[0] == ':' {
			continue // comment / heartbeat line
		}
		seen = true
		field, value := splitSSEField(line)
		switch field {
		case "event":
			ev.name = string(value)
		case "data":
			total += len(value) + 1 // +1 for the joining newline
			if total > maxBodyBytes {
				return sseEvent{}, errSSETooLarge
			}
			data = append(data, value)
		default:
			// id, retry, unknown fields: ignored.
		}
	}
}

// readLine reads one line, tolerating both \n and \r\n terminators. Lines of
// any length up to maxBodyBytes are accumulated (no bufio.Scanner 64KB trap);
// a longer line fails with errSSETooLarge before the rest of it is read or
// buffered. The returned slice is freshly allocated and safe to retain.
func (s *sseReader) readLine() ([]byte, error) {
	var line []byte
	for {
		frag, err := s.r.ReadSlice('\n')
		if len(line)+len(frag) > maxBodyBytes {
			return nil, errSSETooLarge
		}
		line = append(line, frag...) // copies: frag aliases the bufio buffer
		if err == nil {
			break
		}
		if err != bufio.ErrBufferFull {
			// A partial line at EOF belongs to an unterminated event; drop
			// it and surface the error.
			return nil, err
		}
	}
	line = line[:len(line)-1]
	if n := len(line); n > 0 && line[n-1] == '\r' {
		line = line[:n-1]
	}
	return line, nil
}

// splitSSEField splits "field: value", stripping the single optional space
// after the colon per the SSE spec.
func splitSSEField(line []byte) (string, []byte) {
	i := bytes.IndexByte(line, ':')
	if i < 0 {
		return string(line), nil
	}
	field := string(line[:i])
	value := line[i+1:]
	if len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}
	return field, value
}
