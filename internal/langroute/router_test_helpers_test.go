package langroute

import "testing"

// newTestRouter is a Router with the library's default registrations, closed
// when the test ends.
func newTestRouter(t testing.TB, opts ...Option) *Router {
	t.Helper()
	r, err := NewRouter(opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	return r
}
