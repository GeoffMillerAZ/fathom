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

// The extended posture adds the kit's high-precision credential shapes
// (DLP-101..106) for callers that send arbitrary source off-host — a
// committed AWS key ID or GitHub token passes the generic policy untouched.
// RedactCount reports how many spans a pass replaced.
func TestExtendedPostureRedactsCredentialShapesAndCounts(t *testing.T) {
	const aws, gh = "AKIAIOSFODNN7EXAMPLE", "ghp_0123456789abcdefghijklmnopqrstuvwxyzAB"
	src := "const k = \"" + aws + "\"\nconst g = \"" + gh + "\"\nconst k2 = \"" + aws + "\"\n"

	if out, n := llmsec.NewRedactor(llmsec.DefaultPosture()).RedactCount(src); n != 0 || out != src {
		t.Fatalf("the generic policy is expected to miss these shapes (n=%d) — if it now catches them, this test's premise changed", n)
	}
	out, n := llmsec.NewRedactor(llmsec.ExtendedPosture()).RedactCount(src)
	if strings.Contains(out, aws) || strings.Contains(out, gh) {
		t.Fatalf("extended posture leaked a credential shape:\n%s", out)
	}
	if n != 3 {
		t.Errorf("RedactCount = %d, want 3 (two AWS key IDs, one GitHub token)", n)
	}
	again, n2 := llmsec.NewRedactor(llmsec.ExtendedPosture()).RedactCount(out)
	if n2 != 0 || again != out {
		t.Errorf("an already-redacted text counted %d new redactions", n2)
	}
	if _, n := llmsec.NewRedactor(llmsec.Posture{Passthrough: true, Extended: true}).RedactCount(src); n != 0 {
		t.Errorf("passthrough counted %d", n)
	}
}
