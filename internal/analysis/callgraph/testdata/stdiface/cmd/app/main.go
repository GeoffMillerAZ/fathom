// Command app hands first-party values to the standard library, which calls
// their methods through its own interfaces (O1237): net/http calls
// RoundTrip and ServeHTTP, fmt calls String, errors calls Error. Nothing the
// index sees calls those methods; only the conversion to the interface
// shows the product runs them.
package main

import (
	"errors"
	"fmt"
	"net/http"

	"stdmod/rt"
)

// init installs a transport, as cmd/fathom installs the hook guard's.
func init() { http.DefaultTransport = &rt.Transport{} }

func main() {
	http.Handle("/", rt.Handler{})
	_ = rt.NewReader()
	_ = &http.Client{Transport: &rt.FieldRT{}}
	ch := make(chan error, 1)
	ch <- &rt.SentErr{}
	fmt.Println(fmt.Stringer(rt.Named{}))
	_ = errors.Join(&rt.JoinErr{})

	// Held and called directly, never converted: RoundTrip stays out.
	var u rt.Unconverted
	fmt.Println(u.Name())
	// Converted to an interface the module declares: a call site of
	// Local.RoundTrip would dispatch to it, and none exists.
	var l rt.Local = &rt.LocalOnly{}
	_ = l
	// Declared as the blank identifier: the value is discarded.
	var _ error = rt.Blank{}
}
