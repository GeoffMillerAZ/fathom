// Package hostwire implements the private control channel used by the Wails
// gallery spike. It is intentionally small and is not an app-kit API.
package hostwire

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	protocolVersion = 1
	maxFrameSize    = 2048
	writeTimeout    = 3 * time.Second
	launchMarker    = "/auth/launch?token="
)

var (
	errInvalidMessage = errors.New("invalid host control message")
	errReadMessage    = errors.New("cannot read host control message")
	errWriteMessage   = errors.New("cannot write host control message")
	errClose          = errors.New("cannot close host control connection")
)

// Message is one private host-control protocol message.
type Message struct {
	Version int    `json:"version"`
	Kind    string `json:"kind"`
	Origin  string `json:"origin,omitempty"`
	Launch  string `json:"launch,omitempty"`
}

// Conn frames private host-control messages over conn.
type Conn struct {
	conn net.Conn
	mu   sync.Mutex
}

// New wraps conn as a private host-control connection.
func New(conn net.Conn) *Conn {
	return &Conn{conn: conn}
}

// Read reads and validates one message. A zero timeout waits indefinitely for
// the first byte, then bounds completion to three seconds. A positive timeout
// bounds the entire frame. An idle channel is valid; a stalled partial frame is not.
func (c *Conn) Read(timeout time.Duration) (Message, error) {
	if c == nil || c.conn == nil || timeout < 0 {
		return Message{}, errReadMessage
	}
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	if err := c.conn.SetReadDeadline(deadline); err != nil {
		return Message{}, errReadMessage
	}

	var header [4]byte
	if _, err := io.ReadFull(c.conn, header[:1]); err != nil {
		return Message{}, errReadMessage
	}
	if timeout == 0 {
		if err := c.conn.SetReadDeadline(time.Now().Add(writeTimeout)); err != nil {
			return Message{}, errReadMessage
		}
	}
	if _, err := io.ReadFull(c.conn, header[1:]); err != nil {
		return Message{}, errReadMessage
	}
	size := binary.BigEndian.Uint32(header[:])
	if size == 0 || size > maxFrameSize {
		return Message{}, errReadMessage
	}
	payload := make([]byte, int(size))
	if _, err := io.ReadFull(c.conn, payload); err != nil {
		return Message{}, errReadMessage
	}

	var message Message
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&message); err != nil {
		return Message{}, errInvalidMessage
	}
	canonical, err := json.Marshal(message)
	if err != nil || !bytes.Equal(payload, canonical) || validateMessage(message) != nil {
		return Message{}, errInvalidMessage
	}
	return message, nil
}

// Write validates and writes one message using a three-second write deadline.
// Concurrent writes are serialized so frames cannot interleave.
func (c *Conn) Write(message Message) error {
	if c == nil || c.conn == nil || validateMessage(message) != nil {
		return errInvalidMessage
	}
	payload, err := json.Marshal(message)
	if err != nil || len(payload) == 0 || len(payload) > maxFrameSize {
		return errInvalidMessage
	}
	frame := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(frame[:4], uint32(len(payload)))
	copy(frame[4:], payload)

	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return errWriteMessage
	}
	if _, err := writeFull(c.conn, frame); err != nil {
		return errWriteMessage
	}
	return nil
}

// Close closes the underlying transport.
func (c *Conn) Close() error {
	if c == nil || c.conn == nil {
		return errClose
	}
	if err := c.conn.Close(); err != nil {
		return errClose
	}
	return nil
}

// ValidateOrigin accepts only the canonical IPv4 loopback HTTP origin used by
// the synthetic spike.
func ValidateOrigin(origin string) error {
	portText, ok := strings.CutPrefix(origin, "http://127.0.0.1:")
	if !ok || portText == "" || strings.Contains(portText, ":") {
		return errInvalidMessage
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != portText {
		return errInvalidMessage
	}
	return nil
}

// ValidateLaunch accepts exactly one canonical, 32-byte launch credential for
// origin. It never includes the credential in an error.
func ValidateLaunch(origin, launch string) error {
	if ValidateOrigin(origin) != nil {
		return errInvalidMessage
	}
	token, ok := strings.CutPrefix(launch, origin+launchMarker)
	if !ok || token == "" {
		return errInvalidMessage
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != token {
		return errInvalidMessage
	}
	return nil
}

func validateMessage(message Message) error {
	if message.Version != protocolVersion {
		return errInvalidMessage
	}
	switch message.Kind {
	case "init":
		if message.Origin == "" || message.Launch == "" || ValidateLaunch(message.Origin, message.Launch) != nil {
			return errInvalidMessage
		}
	case "launch":
		if message.Origin != "" || validateStandaloneLaunch(message.Launch) != nil {
			return errInvalidMessage
		}
	case "ready", "launch-request", "browser", "browser-ok", "browser-failed", "closed":
		if message.Origin != "" || message.Launch != "" {
			return errInvalidMessage
		}
	default:
		return errInvalidMessage
	}
	return nil
}

func validateStandaloneLaunch(launch string) error {
	marker := strings.Index(launch, launchMarker)
	if marker < 0 {
		return errInvalidMessage
	}
	origin := launch[:marker]
	return ValidateLaunch(origin, launch)
}

func writeFull(writer io.Writer, payload []byte) (int, error) {
	written := 0
	for written < len(payload) {
		n, err := writer.Write(payload[written:])
		written += n
		if err != nil {
			return written, err
		}
		if n == 0 {
			return written, io.ErrShortWrite
		}
	}
	return written, nil
}
