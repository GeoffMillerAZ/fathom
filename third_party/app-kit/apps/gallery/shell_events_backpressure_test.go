package gallery

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// Observe actual transport writes; net.Pipe itself supplies blocking and deadlines.
type shellEventsObservedPipe struct {
	net.Conn
	entered chan time.Time
	result  chan error
	once    sync.Once
}

func (c *shellEventsObservedPipe) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8765}
}
func (c *shellEventsObservedPipe) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
}
func (c *shellEventsObservedPipe) Write(body []byte) (int, error) {
	c.once.Do(func() { c.entered <- time.Now() })
	n, err := c.Conn.Write(body)
	select {
	case c.result <- err:
	default:
	}
	return n, err
}

type shellEventsPipeListener struct {
	connections chan net.Conn
	closed      chan struct{}
	once        sync.Once
}

func (l *shellEventsPipeListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connections:
		return conn, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}
func (l *shellEventsPipeListener) Close() error { l.once.Do(func() { close(l.closed) }); return nil }
func (l *shellEventsPipeListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8765}
}

func TestShellEventsActualBlockedTransportDeadlineReleasesAdmission(t *testing.T) {
	store, session, request := shellEventStoreRequest()
	serverPipe, clientPipe := net.Pipe()
	observed := &shellEventsObservedPipe{Conn: serverPipe, entered: make(chan time.Time, 1), result: make(chan error, 1)}
	listener := &shellEventsPipeListener{connections: make(chan net.Conn, 1), closed: make(chan struct{})}
	listener.connections <- observed
	handlerDone, serveDone := make(chan struct{}), make(chan error, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(handlerDone)
		store.serveShellEvents(w, r)
	})}
	go func() { serveDone <- server.Serve(listener) }()
	t.Cleanup(func() {
		_ = clientPipe.Close()
		_ = serverPipe.Close()
		_ = server.Close()
		select {
		case err := <-serveDone:
			if !errors.Is(err, http.ErrServerClosed) {
				t.Errorf("server exit: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("server did not close")
		}
	})
	_ = clientPipe.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := request.Write(clientPipe); err != nil {
		t.Fatalf("send real HTTP request: %v", err)
	}
	_ = clientPipe.SetWriteDeadline(time.Time{})
	// Never read clientPipe: even the small initial flush must block, without
	// depending on OS socket-buffer sizes or generating oversized projections.
	var started time.Time
	select {
	case started = <-observed.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("HTTP stream never attempted a transport write")
	}
	values := url.Values{"action": {"save-profile"}, "csrf": {session.CSRF}, "version": {"1"}, "display_name": {"Saved during blocked transport"}}
	post := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8765/shell/action", strings.NewReader(values.Encode()))
	post.RemoteAddr = request.RemoteAddr
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.Header.Set("Origin", "http://127.0.0.1:8765")
	post.AddCookie(request.Cookies()[0])
	posted := make(chan int, 1)
	go func() { response := httptest.NewRecorder(); store.serveHTTP(response, post); posted <- response.Code }()
	select {
	case status := <-posted:
		if status != http.StatusSeeOther {
			t.Fatalf("independent POST status = %d", status)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked transport prevented an independent POST")
	}
	store.mu.Lock()
	committed := session.State.Version == 2 && session.State.DisplayName == values.Get("display_name")
	admitted := store.streams == 1 && session.streams == 1
	store.mu.Unlock()
	if !committed || !admitted {
		t.Fatal("POST must commit while the transport still owns its admitted slot")
	}
	select {
	case <-handlerDone:
	case <-time.After(8*time.Second - time.Since(started)):
		t.Fatal("production write deadline failed to release blocked transport")
	}
	if err := <-observed.result; !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("actual pipe write must time out, not be canceled or closed: %v", err)
	}
	if elapsed := time.Since(started); elapsed < shellDemoStreamWriteTimeout-time.Second {
		t.Fatalf("transport exited before the configured deadline: %s", elapsed)
	} else {
		t.Logf("actual blocked HTTP write timed out and released admission after %s", elapsed)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.streams != 0 || session.streams != 0 {
		t.Fatal("timed-out transport leaked stream admission")
	}
}
