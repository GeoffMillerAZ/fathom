package rt

import (
	"net/http"
	"testing"
)

var testTransport http.RoundTripper = &TestOnlyRT{}

func init() { http.DefaultTransport = &TestOnlyRT{} }

func TestTestOnlyRT(t *testing.T) {
	var c http.RoundTripper = &TestOnlyRT{}
	_ = c
	_ = testTransport
	_ = &Unconverted{}
	_, _ = (&Transport{}).RoundTrip(nil)
	(&Transport{}).Close()
}
