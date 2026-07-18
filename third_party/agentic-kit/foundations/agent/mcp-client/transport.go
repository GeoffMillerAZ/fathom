package mcpclient

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
)

// DefaultMaxMessageBytes is the message-size bound applied when
// IOTransportOptions.MaxMessageBytes is zero: 4 MiB, matching
// kits/mcp-server's DefaultMaxMessageBytes.
const DefaultMaxMessageBytes = 4 << 20

// Transport is the minimal seam over which Client speaks to an MCP server.
// Each call carries a context for cancellation; Send blocks until the message
// is accepted by the underlying I/O; Receive blocks until a complete message
// arrives or the context is cancelled. Close signals the transport to release
// resources; subsequent calls must return an error.
//
// Implementations must be safe for concurrent use within the constraints
// documented on Client.
type Transport interface {
	Send(ctx context.Context, msg []byte) error
	Receive(ctx context.Context) ([]byte, error)
	Close() error
}

// ioTransport implements Transport over an io.Reader / io.Writer pair using
// the same newline-delimited JSON-RPC 2.0 framing as kits/mcp-server.
// It is NOT safe for concurrent use; Client serializes access via its own
// mutexes.
type ioTransport struct {
	w      io.Writer
	br     *bufio.Reader
	max    int
	wmu    sync.Mutex // serialises Write; Receive has no concurrent path
	closed bool
}

// NewIOTransport constructs a Transport over r (from the server) and w (to
// the server). maxMessageBytes bounds one received line; zero means
// DefaultMaxMessageBytes. r and w are typically the stdin/stdout of a child
// process; the caller owns closing them after calling Close.
func NewIOTransport(r io.Reader, w io.Writer, maxMessageBytes int) Transport {
	max := maxMessageBytes
	if max <= 0 {
		max = DefaultMaxMessageBytes
	}
	return &ioTransport{
		w:   w,
		br:  bufio.NewReaderSize(r, 32*1024),
		max: max,
	}
}

// Send writes msg followed by '\n' to the underlying writer. It is safe for
// concurrent use (serialized by a mutex).
func (t *ioTransport) Send(_ context.Context, msg []byte) error {
	t.wmu.Lock()
	defer t.wmu.Unlock()
	if t.closed {
		return errors.New("mcpclient: transport closed")
	}
	buf := make([]byte, len(msg)+1)
	copy(buf, msg)
	buf[len(msg)] = '\n'
	_, err := t.w.Write(buf)
	if err != nil {
		return fmt.Errorf("mcpclient: send: %w", err)
	}
	return nil
}

// lineResult carries one readLine outcome across the goroutine boundary.
type lineResult struct {
	line []byte
	err  error
}

// Receive reads the next newline-delimited message from the underlying reader.
// It honours context cancellation: the blocking read runs in a separate
// goroutine; if the context is cancelled before the read completes, Receive
// returns ctx.Err() immediately. The reader goroutine is abandoned on
// cancellation — it will unblock when the underlying pipe/connection is closed
// or data arrives. The buffered channel prevents a goroutine leak: the
// abandoned goroutine eventually sends and exits.
func (t *ioTransport) Receive(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if t.closed {
		return nil, errors.New("mcpclient: transport closed")
	}

	// Buffered 1 so the goroutine can always send even if we have returned.
	ch := make(chan lineResult, 1)
	go func() {
		line, err := readLine(t.br, t.max)
		ch <- lineResult{line: line, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			if errors.Is(res.err, io.EOF) {
				return nil, fmt.Errorf("mcpclient: server closed connection: %w", io.EOF)
			}
			return nil, fmt.Errorf("mcpclient: receive: %w", res.err)
		}
		return res.line, nil
	}
}

// Close marks the transport closed. It does not close the underlying r/w;
// the caller owns those.
func (t *ioTransport) Close() error {
	t.wmu.Lock()
	defer t.wmu.Unlock()
	t.closed = true
	return nil
}

// readLine reads one newline-delimited message from br, trimming the trailing
// newline (and optional CR). Lines exceeding max bytes return an error.
func readLine(br *bufio.Reader, max int) ([]byte, error) {
	var buf []byte
	for {
		frag, err := br.ReadSlice('\n')
		buf = append(buf, frag...)
		switch {
		case err == nil:
			line := trimEOL(buf)
			if len(line) > max {
				return nil, fmt.Errorf("mcpclient: message exceeds maximum size of %d bytes", max)
			}
			return line, nil
		case errors.Is(err, bufio.ErrBufferFull):
			if len(buf) > max {
				return nil, fmt.Errorf("mcpclient: message exceeds maximum size of %d bytes", max)
			}
			// keep accumulating
		case errors.Is(err, io.EOF):
			if len(buf) == 0 {
				return nil, io.EOF
			}
			line := trimEOL(buf)
			if len(line) > max {
				return nil, fmt.Errorf("mcpclient: message exceeds maximum size of %d bytes", max)
			}
			return line, nil
		default:
			return nil, err
		}
	}
}

// trimEOL strips a trailing '\n' and then a trailing '\r'.
func trimEOL(b []byte) []byte {
	if n := len(b); n > 0 && b[n-1] == '\n' {
		b = b[:n-1]
	}
	if n := len(b); n > 0 && b[n-1] == '\r' {
		b = b[:n-1]
	}
	return bytes.TrimSpace(b)
}
