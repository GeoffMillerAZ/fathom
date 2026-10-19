// Package rt declares the types cmd/app hands to the standard library.
package rt

import (
	"errors"
	"io"
	"net/http"
)

// Transport is cmd/app's http.DefaultTransport (an assignment in an init).
type Transport struct{}

func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) { return roundTripHelper(r) }

func roundTripHelper(*http.Request) (*http.Response, error) { return nil, errors.New("offline") }

// Close is no method of http.RoundTripper: converting a Transport reaches
// RoundTrip only.
func (t *Transport) Close() { closeHelper() }

func closeHelper() {}

// Handler is passed to http.Handle (a call argument).
type Handler struct{}

func (Handler) ServeHTTP(http.ResponseWriter, *http.Request) {}

// Reader is returned as an io.Reader (a return).
type Reader struct{}

func (*Reader) Read([]byte) (int, error) { return 0, io.EOF }

// NewReader returns a Reader as an io.Reader.
func NewReader() io.Reader { return &Reader{} }

// FieldRT is an http.Client's Transport field (a composite-literal field).
type FieldRT struct{}

func (*FieldRT) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }

// SentErr is sent on a chan error (a send to the universe's error).
type SentErr struct{}

func (*SentErr) Error() string { return "sent" }

// Named is converted to fmt.Stringer explicitly.
type Named struct{}

func (Named) String() string { return "named" }

// JoinErr is a variadic argument of errors.Join.
type JoinErr struct{}

func (*JoinErr) Error() string { return "join" }

// VarHandler is the initializer of a package variable of type http.Handler.
type VarHandler struct{}

func (VarHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

// Default holds a VarHandler from package initialization (O1124).
var Default http.Handler = VarHandler{}

// Unconverted has a RoundTrip method; cmd/app holds one and never converts it.
type Unconverted struct{}

func (Unconverted) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }

// Name is the method cmd/app calls.
func (Unconverted) Name() string { return "unconverted" }

// Orphan is converted only by a function nothing calls.
type Orphan struct{}

func (*Orphan) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }

func unusedInstall() { http.DefaultTransport = &Orphan{} }

// Asserted is converted only by a compile-time assertion to the blank
// identifier, which discards the value.
type Asserted struct{}

func (*Asserted) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }

var _ http.RoundTripper = (*Asserted)(nil)

// Blank is converted to error only in a blank declaration in main.
type Blank struct{}

func (Blank) Error() string { return "blank" }

// Local is an interface this module declares.
type Local interface {
	RoundTrip(*http.Request) (*http.Response, error)
}

// LocalOnly is converted only to Local.
type LocalOnly struct{}

func (*LocalOnly) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }

// TestOnlyRT is converted to http.RoundTripper only by rt_test.go.
type TestOnlyRT struct{}

func (*TestOnlyRT) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }
