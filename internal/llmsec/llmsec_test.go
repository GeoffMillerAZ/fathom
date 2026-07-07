package llmsec_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/llmsec"
)

func TestRedact_Clean(t *testing.T) {
	r := llmsec.NewRedactor(llmsec.DefaultPosture())
	out, changed := r.Redact("ordinary symbol name: ValidateToken")
	if changed {
		t.Errorf("expected no change for clean text, got %q", out)
	}
	if out != "ordinary symbol name: ValidateToken" {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestRedact_BearerToken(t *testing.T) {
	r := llmsec.NewRedactor(llmsec.DefaultPosture())
	input := "Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.secret"
	out, changed := r.Redact(input)
	if !changed {
		t.Error("expected changed=true for Bearer token")
	}
	if strings.Contains(out, "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.secret") {
		t.Errorf("bearer token not redacted in: %q", out)
	}
}

func TestRedact_APIKey(t *testing.T) {
	r := llmsec.NewRedactor(llmsec.DefaultPosture())
	input := `const apiKey = "sk-abc123secret"`
	out, changed := r.Redact(input)
	if !changed {
		t.Error("expected changed=true for api_key pattern")
	}
	if strings.Contains(out, "sk-abc123secret") {
		t.Errorf("api key not redacted in: %q", out)
	}
}

func TestRedact_Password(t *testing.T) {
	r := llmsec.NewRedactor(llmsec.DefaultPosture())
	input := "password=supersecret123"
	out, changed := r.Redact(input)
	if !changed {
		t.Error("expected changed=true for password pattern")
	}
	if strings.Contains(out, "supersecret123") {
		t.Errorf("password not redacted in: %q", out)
	}
}

func TestRedact_Idempotent(t *testing.T) {
	r := llmsec.NewRedactor(llmsec.DefaultPosture())
	input := "Authorization: Bearer tok123"
	once, _ := r.Redact(input)
	twice, changed := r.Redact(once)
	if once != twice {
		t.Errorf("redact not idempotent: %q != %q", once, twice)
	}
	// second pass should not report changed (no new secrets)
	if changed {
		t.Errorf("second pass on already-redacted text should not report changed")
	}
}

func TestRedact_PassthroughPosture(t *testing.T) {
	posture := llmsec.Posture{Passthrough: true}
	r := llmsec.NewRedactor(posture)
	input := "password=shouldnotberedacted"
	out, changed := r.Redact(input)
	if changed {
		t.Error("passthrough posture: expected changed=false")
	}
	if out != input {
		t.Errorf("passthrough posture: expected unchanged text, got %q", out)
	}
}

func TestDefaultPosture(t *testing.T) {
	p := llmsec.DefaultPosture()
	if p.Passthrough {
		t.Error("default posture should not be passthrough")
	}
}
