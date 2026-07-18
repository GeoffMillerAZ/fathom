package awscred

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// imdsHandler builds a scripted IMDSv2 handler: PUT /latest/api/token issues
// a token; the role-list GET and the role-credentials GET both require that
// exact token in X-aws-ec2-metadata-token, mirroring the real service's
// token-gating.
func imdsHandler(t *testing.T, role, credJSON string) http.Handler {
	t.Helper()
	const token = "imds-test-token"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/latest/api/token":
			if r.Header.Get(imdsTokenTTLHeader) == "" {
				t.Errorf("token PUT missing %s header", imdsTokenTTLHeader)
			}
			w.Write([]byte(token))
		case r.Method == http.MethodGet && r.URL.Path == "/latest/meta-data/iam/security-credentials/":
			if r.Header.Get(imdsTokenHeader) != token {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Write([]byte(role + "\n"))
		case r.Method == http.MethodGet && r.URL.Path == "/latest/meta-data/iam/security-credentials/"+role:
			if r.Header.Get(imdsTokenHeader) != token {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Write([]byte(credJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func TestResolveIMDS_HappyPath(t *testing.T) {
	srv := httptest.NewServer(imdsHandler(t, "my-instance-role",
		`{"Code":"Success","AccessKeyId":"AKIDIMDS","SecretAccessKey":"secretimds","Token":"tokenimds","Expiration":"2030-01-01T00:00:00Z"}`))
	defer srv.Close()

	cfg := Config{IMDSBaseURL: srv.URL}
	creds, ok, err := resolveIMDS(context.Background(), cfg, time.Now)
	if err != nil {
		t.Fatalf("resolveIMDS: %v", err)
	}
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if creds.AccessKeyID != "AKIDIMDS" || creds.SecretAccessKey != "secretimds" || creds.SessionToken != "tokenimds" {
		t.Errorf("creds = %+v", creds)
	}
	if creds.Source != SourceIMDS {
		t.Errorf("Source = %q, want imds", creds.Source)
	}
	if creds.Expiry.IsZero() {
		t.Error("Expiry not parsed")
	}
}

func TestResolveIMDS_Unreachable_ReturnsNotOK_NotError(t *testing.T) {
	// Point at a base URL nothing is listening on (a closed httptest server)
	// — the local-metadata-service-absent case, which is the common case off
	// EC2 and must be a clean "not found," not an error.
	srv := httptest.NewServer(imdsHandler(t, "role", `{}`))
	srv.Close() // closed immediately: connections to srv.URL now fail fast.

	cfg := Config{IMDSBaseURL: srv.URL}
	_, ok, err := resolveIMDS(context.Background(), cfg, time.Now)
	if err != nil {
		t.Fatalf("resolveIMDS: %v, want nil error for an unreachable IMDS", err)
	}
	if ok {
		t.Fatal("ok = true, want false for an unreachable IMDS")
	}
}

func TestResolveIMDS_TokenRequestForbidden_ReturnsNotOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	cfg := Config{IMDSBaseURL: srv.URL}
	_, ok, err := resolveIMDS(context.Background(), cfg, time.Now)
	if err != nil {
		t.Fatalf("resolveIMDS: %v", err)
	}
	if ok {
		t.Fatal("ok = true, want false")
	}
}

func TestResolveIMDS_Disabled(t *testing.T) {
	t.Setenv(envIMDSDisabled, "true")
	if !imdsDisabled() {
		t.Error("imdsDisabled() = false, want true when AWS_EC2_METADATA_DISABLED=true")
	}
	t.Setenv(envIMDSDisabled, "")
	if imdsDisabled() {
		t.Error("imdsDisabled() = true, want false when unset")
	}
}

func TestResolveIMDS_TimeoutBound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1500 * time.Millisecond)
		w.Write([]byte("too-slow"))
	}))
	defer srv.Close()

	cfg := Config{IMDSBaseURL: srv.URL}
	start := time.Now()
	_, ok, err := resolveIMDS(context.Background(), cfg, time.Now)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("resolveIMDS: %v", err)
	}
	if ok {
		t.Fatal("ok = true, want false for a timed-out IMDS call")
	}
	// The tight guard is the ok=false above (a working timeout means the
	// slow response never arrives as a success). The wall-clock bound is
	// deliberately loose — CI schedulers add hundreds of ms of noise — and
	// only catches a gross regression (e.g. the per-call timeout removed
	// entirely, making this call ride the server's full sleep).
	if elapsed > 2500*time.Millisecond {
		t.Errorf("resolveIMDS took %s, want loosely bounded near imdsTimeout (%s)", elapsed, imdsTimeout)
	}
}
