package httpauth

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// Middleware wraps an http.Handler with an additional check, in the
// standard net/http decorator shape.
type Middleware func(next http.Handler) http.Handler

// bearerPrefix is the required Authorization header scheme, with the
// trailing space that separates it from the token.
const bearerPrefix = "Bearer "

// Bearer builds a Middleware that requires a constant-time-matched bearer
// token on every request it wraps. The token is read from the environment
// variable named tokenEnvName, once, at construction time — never accepted
// as a literal argument, so it never has to travel through a flag, a config
// file, or a process's argv.
//
// tokenEnvName must name a non-empty environment variable: an empty
// tokenEnvName, or one naming an unset/empty variable, is a constructor
// error. This is deliberate — a caller who intended to protect a handler
// must find out immediately, not discover later that the gate was silently
// open. There is no "auth disabled" zero value returned from Bearer; a
// caller that wants an optional gate decides that at the call site (e.g.
// skip calling Bearer, or wrap conditionally) rather than depending on this
// constructor to signal it.
//
// The returned Middleware answers a missing or mismatched
// "Authorization: Bearer <token>" header with 401 Unauthorized and a
// WWW-Authenticate: Bearer response header, without invoking next. The
// comparison is constant-time; the configured token is never logged,
// returned in an error, or echoed into any response.
func Bearer(tokenEnvName string) (Middleware, error) {
	if tokenEnvName == "" {
		return nil, fmt.Errorf("httpauth: token env var name must not be empty")
	}
	token := os.Getenv(tokenEnvName)
	if token == "" {
		return nil, fmt.Errorf("httpauth: environment variable %q is empty or unset — refusing to build a bearer gate that would silently accept every request", tokenEnvName)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := r.Header.Get("Authorization")
			authorized := strings.HasPrefix(got, bearerPrefix) &&
				subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(got, bearerPrefix)), []byte(token)) == 1
			if !authorized {
				w.Header().Set("WWW-Authenticate", `Bearer realm="restricted"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}
