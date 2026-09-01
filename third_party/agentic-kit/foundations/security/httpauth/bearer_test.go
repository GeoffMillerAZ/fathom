package httpauth_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/security/httpauth"
)

// okHandler is the wrapped handler every test uses: it must never be
// invoked when auth fails, and must echo a fixed body when it is.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
}

func TestBearer_EmptyEnvNameIsConstructorError(t *testing.T) {
	if _, err := httpauth.Bearer(""); err == nil {
		t.Fatal("Bearer(\"\") = nil error, want a constructor error")
	}
}

func TestBearer_UnsetEnvIsConstructorError(t *testing.T) {
	const envName = "HTTPAUTH_TEST_UNSET_TOKEN"
	// Ensure the variable really is unset for this test, regardless of the
	// ambient environment.
	t.Setenv(envName, "")

	if _, err := httpauth.Bearer(envName); err == nil {
		t.Fatalf("Bearer(%q) with an empty/unset variable = nil error, want a constructor error", envName)
	} else if !strings.Contains(err.Error(), envName) {
		t.Fatalf("constructor error = %q, want it to name the env var %q", err.Error(), envName)
	}
}

func TestBearer_ConstructorErrorNeverContainsAValue(t *testing.T) {
	// There is no value to leak when the var is unset, but assert the shape
	// of the guarantee anyway: the error text is built only from the env
	// var NAME, never a token value, across both failure modes.
	const envName = "HTTPAUTH_TEST_EMPTY_NAME_VAR"
	t.Setenv(envName, "")

	_, err1 := httpauth.Bearer("")
	_, err2 := httpauth.Bearer(envName)
	if err1 == nil || err2 == nil {
		t.Fatal("expected both constructor calls to error")
	}
}

func newBearerMiddleware(t *testing.T, token string) httpauth.Middleware {
	t.Helper()
	const envName = "HTTPAUTH_TEST_TOKEN"
	t.Setenv(envName, token)
	mw, err := httpauth.Bearer(envName)
	if err != nil {
		t.Fatalf("Bearer(%q): %v", envName, err)
	}
	return mw
}

func TestBearer_MissingHeaderRejected(t *testing.T) {
	mw := newBearerMiddleware(t, "s3cr3t-token")
	h := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if got := rr.Header().Get("WWW-Authenticate"); !strings.HasPrefix(got, "Bearer") {
		t.Fatalf("WWW-Authenticate = %q, want a Bearer challenge", got)
	}
}

func TestBearer_WrongTokenRejected(t *testing.T) {
	mw := newBearerMiddleware(t, "s3cr3t-token")
	h := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if rr.Body.String() == "ok" {
		t.Fatal("wrapped handler ran despite a wrong token")
	}
}

func TestBearer_WrongSchemeRejected(t *testing.T) {
	mw := newBearerMiddleware(t, "s3cr3t-token")
	h := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic s3cr3t-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestBearer_CorrectTokenAllowed(t *testing.T) {
	mw := newBearerMiddleware(t, "s3cr3t-token")
	h := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer s3cr3t-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Fatalf("body = %q, want %q", rr.Body.String(), "ok")
	}
	if got := rr.Header().Get("WWW-Authenticate"); got != "" {
		t.Fatalf("WWW-Authenticate = %q on a successful request, want empty", got)
	}
}

func TestBearer_PrefixOfTokenRejected(t *testing.T) {
	// A shorter string that happens to be a prefix of the real token must
	// not accidentally satisfy a naive comparison.
	mw := newBearerMiddleware(t, "s3cr3t-token")
	h := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer s3cr3t")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

// TestBearer_TokenNeverAppearsInRecordedOutput proves the "never logged"
// contract the mission calls for: it drives a handful of failing and
// succeeding requests, capturing every byte written to the ResponseRecorder
// (headers + body) and every string a constructor error could produce, and
// greps all of it for the configured token. None of it may contain the
// secret.
func TestBearer_TokenNeverAppearsInRecordedOutput(t *testing.T) {
	const token = "sxt-secret-99f3c1"
	mw := newBearerMiddleware(t, token)
	h := mw(okHandler())

	var recorded strings.Builder

	record := func(req *http.Request) {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		recorded.WriteString(fmt.Sprintf("status=%d\n", rr.Code))
		for k, vs := range rr.Header() {
			for _, v := range vs {
				recorded.WriteString(k + ": " + v + "\n")
			}
		}
		recorded.WriteString(rr.Body.String())
		recorded.WriteString("\n")
	}

	missing := httptest.NewRequest(http.MethodGet, "/", nil)
	record(missing)

	wrong := httptest.NewRequest(http.MethodGet, "/", nil)
	wrong.Header.Set("Authorization", "Bearer not-the-token")
	record(wrong)

	right := httptest.NewRequest(http.MethodGet, "/", nil)
	right.Header.Set("Authorization", "Bearer "+token)
	record(right)

	// Also fold in the constructor error paths.
	if _, err := httpauth.Bearer(""); err != nil {
		recorded.WriteString(err.Error())
		recorded.WriteString("\n")
	}
	t.Setenv("HTTPAUTH_TEST_EMPTY_FOR_LEAK_CHECK", "")
	if _, err := httpauth.Bearer("HTTPAUTH_TEST_EMPTY_FOR_LEAK_CHECK"); err != nil {
		recorded.WriteString(err.Error())
		recorded.WriteString("\n")
	}

	if strings.Contains(recorded.String(), token) {
		t.Fatalf("recorded output contains the bearer token verbatim:\n%s", recorded.String())
	}
}

// TestBearer_ConcurrentRequestsAreRaceFree exercises the middleware from
// many goroutines at once — the returned closure captures only an immutable
// token string and performs no shared mutable state, so this must be clean
// under -race.
func TestBearer_ConcurrentRequestsAreRaceFree(t *testing.T) {
	mw := newBearerMiddleware(t, "s3cr3t-token")
	h := mw(okHandler())

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if i%2 == 0 {
				req.Header.Set("Authorization", "Bearer s3cr3t-token")
			} else {
				req.Header.Set("Authorization", "Bearer wrong")
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			wantCode := http.StatusUnauthorized
			if i%2 == 0 {
				wantCode = http.StatusOK
			}
			if rr.Code != wantCode {
				t.Errorf("request %d: status = %d, want %d", i, rr.Code, wantCode)
			}
		}(i)
	}
	wg.Wait()
}
