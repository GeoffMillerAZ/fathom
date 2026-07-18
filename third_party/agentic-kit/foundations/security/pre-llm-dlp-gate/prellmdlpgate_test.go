package prellmdlpgate_test

import (
	"strings"
	"testing"

	gate "github.com/geoffmilleraz/agentic-kit/foundations/security/pre-llm-dlp-gate"
)

func TestScanRedactsBearerToken(t *testing.T) {
	t.Parallel()

	g, err := gate.NewGate(gate.DefaultPolicy())
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	text := "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig"
	redacted, findings := g.Scan(text)
	if strings.Contains(redacted, "eyJ") {
		t.Errorf("token must be redacted, got: %q", redacted)
	}
	if len(findings) == 0 {
		t.Error("expected at least one Finding")
	}
	if findings[0].RuleID != "DLP-001" {
		t.Errorf("expected DLP-001, got %q", findings[0].RuleID)
	}
}

func TestScanRedactsAPIKey(t *testing.T) {
	t.Parallel()

	g, err := gate.NewGate(gate.DefaultPolicy())
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	text := "api_key=sk-abc123secret"
	redacted, findings := g.Scan(text)
	if strings.Contains(redacted, "sk-abc123secret") {
		t.Errorf("api key must be redacted, got: %q", redacted)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "DLP-002" {
			found = true
		}
	}
	if !found {
		t.Error("expected DLP-002 to fire")
	}
}

func TestScanRedactsCardNumber(t *testing.T) {
	t.Parallel()

	g, err := gate.NewGate(gate.DefaultPolicy())
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	text := "Card: 4111 1111 1111 1111 for payment"
	redacted, findings := g.Scan(text)
	if strings.Contains(redacted, "4111") {
		t.Errorf("card number must be redacted, got: %q", redacted)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "DLP-004" {
			found = true
		}
	}
	if !found {
		t.Error("expected DLP-004 to fire")
	}
}

func TestScanCleanTextProducesNoFindings(t *testing.T) {
	t.Parallel()

	g, err := gate.NewGate(gate.DefaultPolicy())
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	text := "Hello, please summarise the following document for me."
	redacted, findings := g.Scan(text)
	if redacted != text {
		t.Errorf("clean text must pass through unchanged, got: %q", redacted)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings for clean text, got: %v", findings)
	}
}

func TestScanIsDeterministic(t *testing.T) {
	t.Parallel()

	g, err := gate.NewGate(gate.DefaultPolicy())
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	text := "password=hunter2 and api_key=xyz"
	r1, f1 := g.Scan(text)
	r2, f2 := g.Scan(text)
	if r1 != r2 {
		t.Errorf("Scan not deterministic: %q vs %q", r1, r2)
	}
	if len(f1) != len(f2) {
		t.Errorf("Scan findings not deterministic: %d vs %d", len(f1), len(f2))
	}
}

func TestNewGateRejectsInvalidPattern(t *testing.T) {
	t.Parallel()

	bad := gate.RedactPolicy{
		Name: "bad", Version: "0.0.1",
		Rules: []gate.RedactRule{
			{ID: "DLP-BAD", Pattern: `(?P<unclosed`, Label: "[X]", Description: "bad"},
		},
	}
	_, err := gate.NewGate(bad)
	if err == nil {
		t.Error("expected error for invalid pattern, got nil")
	}
}

func TestNewGateRejectsEmptyLabel(t *testing.T) {
	t.Parallel()

	bad := gate.RedactPolicy{
		Name: "bad", Version: "0.0.1",
		Rules: []gate.RedactRule{
			{ID: "DLP-BAD", Pattern: `test`, Label: "", Description: "empty label"},
		},
	}
	_, err := gate.NewGate(bad)
	if err == nil {
		t.Error("expected error for empty Label, got nil")
	}
}

// ---- MustDefaultGate --------------------------------------------------------

func TestMustDefaultGateReturnsWorkingGate(t *testing.T) {
	t.Parallel()

	g := gate.MustDefaultGate() // must not panic: shipped policy always compiles
	redacted, findings := g.Scan("password=hunter2")
	if strings.Contains(redacted, "hunter2") {
		t.Errorf("MustDefaultGate gate must redact, got: %q", redacted)
	}
	if len(findings) == 0 {
		t.Error("expected findings from MustDefaultGate gate")
	}
}

// ---- ExtendedPolicy ---------------------------------------------------------

func TestExtendedPolicyCompilesAndSupersetsDefault(t *testing.T) {
	t.Parallel()

	ext := gate.ExtendedPolicy()
	if _, err := gate.NewGate(ext); err != nil {
		t.Fatalf("NewGate(ExtendedPolicy()): %v", err)
	}
	def := gate.DefaultPolicy()
	if len(ext.Rules) <= len(def.Rules) {
		t.Fatalf("ExtendedPolicy must extend DefaultPolicy: %d vs %d rules",
			len(ext.Rules), len(def.Rules))
	}
	for i, r := range def.Rules {
		if ext.Rules[i].ID != r.ID {
			t.Errorf("ExtendedPolicy rule %d = %q, want default rule %q", i, ext.Rules[i].ID, r.ID)
		}
	}
}

// TestExtendedPolicyRules covers every high-precision rule with a
// true-positive case AND false-positive-resistance cases (prose, short
// prefixes, wrong case, off-by-one lengths must pass through untouched).
func TestExtendedPolicyRules(t *testing.T) {
	t.Parallel()

	g, err := gate.NewGate(gate.ExtendedPolicy())
	if err != nil {
		t.Fatalf("NewGate(ExtendedPolicy()): %v", err)
	}

	googleKey := "AIza" + strings.Repeat("Ab-_9", 7) // AIza + exactly 35 chars

	cases := []struct {
		name     string
		text     string
		wantRule string // "" = no rule may fire at all (FP-resistance)
		gone     string // substring that must be absent after redaction
	}{
		// --- DLP-101 PEM private key ---
		{
			name:     "pem/tp-rsa-block",
			text:     "config:\n-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA7bq6t8H\n-----END RSA PRIVATE KEY-----\ndone",
			wantRule: "DLP-101",
			gone:     "MIIEowIBAAKCAQEA7bq6t8H",
		},
		{
			name:     "pem/tp-pkcs8-block",
			text:     "-----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49\n-----END PRIVATE KEY-----",
			wantRule: "DLP-101",
			gone:     "MIGHAgEAMBMGByqGSM49",
		},
		{
			name:     "pem/tp-pgp-block",
			text:     "-----BEGIN PGP PRIVATE KEY BLOCK-----\nlQOYBGRk\n-----END PGP PRIVATE KEY BLOCK-----",
			wantRule: "DLP-101",
			gone:     "lQOYBGRk",
		},
		{
			name: "pem/fp-prose-mention",
			text: "never commit a PRIVATE KEY; rotate the private key quarterly",
		},
		{
			name: "pem/fp-begin-without-end",
			text: "the file started with -----BEGIN RSA PRIVATE KEY----- but was truncated",
		},
		{
			name: "pem/fp-certificate-block",
			text: "-----BEGIN CERTIFICATE-----\nMIICmzCC\n-----END CERTIFICATE-----",
		},
		// --- DLP-102 AWS access key ID ---
		{
			name:     "aws/tp-key-id",
			text:     "the key AKIAIOSFODNN7EXAMPLE was used",
			wantRule: "DLP-102",
			gone:     "AKIAIOSFODNN7EXAMPLE",
		},
		{
			name: "aws/fp-bare-prefix",
			text: "ids starting with AKIA are AWS access keys",
		},
		{
			name: "aws/fp-too-short",
			text: "AKIAIOSFODNN7EXAMPL", // 15 chars after AKIA
		},
		{
			name: "aws/fp-too-long",
			text: "AKIAIOSFODNN7EXAMPLES", // 17 chars after AKIA
		},
		{
			name: "aws/fp-lowercase-body",
			text: "AKIAiosfodnn7example",
		},
		// --- DLP-103 GitHub tokens ---
		{
			name:     "github/tp-classic-ghp",
			text:     "token ghp_AbCd1234EfGh5678IjKl9012MnOp3456QrSt configured",
			wantRule: "DLP-103",
			gone:     "ghp_AbCd1234EfGh5678IjKl9012MnOp3456QrSt",
		},
		{
			name:     "github/tp-server-ghs",
			text:     "ghs_AbCd1234EfGh5678IjKl9012MnOp3456QrSt",
			wantRule: "DLP-103",
			gone:     "ghs_AbCd1234EfGh5678IjKl9012MnOp3456QrSt",
		},
		{
			name:     "github/tp-fine-grained-pat",
			text:     "github_pat_11AAAAAAA0abcdefghijklmnopqrstuvwxyz",
			wantRule: "DLP-103",
			gone:     "github_pat_11AAAAAAA0",
		},
		{
			name: "github/fp-short-body",
			text: "ghp_short123 is not a real token",
		},
		{
			name: "github/fp-bare-prefix-prose",
			text: "use a ghp_ or github_pat_ token for the API",
		},
		{
			name: "github/fp-unknown-prefix-letter",
			text: "ghx_AbCd1234EfGh5678IjKl9012MnOp3456QrSt",
		},
		// --- DLP-104 Slack tokens ---
		{
			name:     "slack/tp-bot-token",
			text:     "SLACK_TOKEN=xoxb-not-a-real-token-AbCdEfGhIjKlMnOp end",
			wantRule: "DLP-104",
			gone:     "xoxb-not-a-real-token-AbCdEfGhIjKlMnOp",
		},
		{
			name: "slack/fp-short-body",
			text: "xoxb-test is a placeholder",
		},
		{
			name: "slack/fp-prefix-prose",
			text: "Slack tokens start with xoxb- or xoxp- prefixes",
		},
		// --- DLP-105 JWT ---
		{
			name:     "jwt/tp-three-segments",
			text:     "session eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c expired",
			wantRule: "DLP-105",
			gone:     "SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
		},
		{
			name: "jwt/fp-eyj-prefixed-word",
			text: "the eyJness of the encoding is a giveaway in prose",
		},
		{
			name: "jwt/fp-short-segments",
			text: "eyJabc.def.ghi looks like a path, not a token",
		},
		{
			name: "jwt/fp-header-only",
			text: "headers often start with eyJhbGciOiJIUzI1NiJ9 alone",
		},
		// --- DLP-106 Google API key ---
		{
			name:     "google/tp-api-key",
			text:     "key=" + googleKey + " ok",
			wantRule: "DLP-106",
			gone:     googleKey,
		},
		{
			name: "google/fp-too-short",
			text: "AIzaShort is not a key",
		},
		{
			name: "google/fp-too-long",
			text: "AIza" + strings.Repeat("Ab-_9", 7) + "Z", // 36 chars after AIza
		},
		{
			name: "google/fp-bare-prefix-prose",
			text: "Google keys start with the AIza prefix",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			redacted, findings := g.Scan(tc.text)
			if tc.wantRule == "" {
				if redacted != tc.text {
					t.Errorf("false positive: text must pass through unchanged\n got: %q\nwant: %q", redacted, tc.text)
				}
				if len(findings) != 0 {
					t.Errorf("false positive: expected no findings, got %v", findings)
				}
				return
			}
			if strings.Contains(redacted, tc.gone) {
				t.Errorf("secret %q must be redacted, got: %q", tc.gone, redacted)
			}
			var fired bool
			for _, f := range findings {
				if f.RuleID == tc.wantRule {
					fired = true
				}
			}
			if !fired {
				t.Errorf("expected rule %s to fire, findings: %v", tc.wantRule, findings)
			}
		})
	}
}

// google/tp-api-key relies on key=... NOT also matching DLP-002 (api key
// assignment); "key=" alone does not satisfy (api[_-]?key|apikey). This test
// pins that DLP-106 is the rule doing the work there.

// ---- Fixpoint / hardening regressions ----------------------------------------

// TestScanFixpointNestedPII verifies that a value whose first-pass redaction
// label exposes another PII match is fully redacted in a single Scan call.
// The canonical failure mode: "password=Bearer tok" — after DLP-003 fires and
// rewrites the password value, the "Bearer tok" inside the replacement could
// expose a DLP-001 match on the next sweep.  More directly: a crafted input
// where one rule's label uncovers a match for an earlier rule.
//
// Concrete example: a card number glued to a password value so that the card
// rule fires first and the leftover text then matches the password rule.
func TestScanFixpointNestedPII(t *testing.T) {
	t.Parallel()

	// Custom two-rule policy where rule A's label contains rule B's trigger.
	// RuleA matches "SECRET" and replaces with "[password=SAFE]".
	// RuleB matches "password=<non-space>+" (DLP-003-like) and would catch
	// "[password=SAFE]" in the next sweep.
	// After fixpoint the output must be "[REDACTED:PASSWORD]" (RuleB fires in
	// pass 2, then no more matches in pass 3 → converged).
	policy := gate.RedactPolicy{
		Name:    "nested-test",
		Version: "0.0.1",
		Rules: []gate.RedactRule{
			{ID: "T-001", Pattern: `SECRET`, Label: "password=FOUND", Description: "exposes password-like label"},
			{ID: "T-002", Pattern: `(?i)password=\S+`, Label: "[REDACTED:PASSWORD]", Description: "catches the exposed label"},
		},
	}
	g, err := gate.NewGate(policy)
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}

	redacted, findings := g.Scan("value: SECRET end")
	if strings.Contains(redacted, "SECRET") {
		t.Errorf("SECRET must be gone: %q", redacted)
	}
	if strings.Contains(redacted, "password=FOUND") {
		t.Errorf("intermediate label must not survive fixpoint: %q", redacted)
	}
	if redacted != "value: [REDACTED:PASSWORD] end" {
		t.Errorf("unexpected final output: %q", redacted)
	}
	// Both rules must appear in findings (both fired at least once).
	rulesFired := map[string]bool{}
	for _, f := range findings {
		rulesFired[f.RuleID] = true
	}
	if !rulesFired["T-001"] {
		t.Error("expected T-001 in findings")
	}
	if !rulesFired["T-002"] {
		t.Error("expected T-002 in findings")
	}
}

// TestScanLabelWithDollarSign verifies that a label string containing "$1"
// (or any $ backreference syntax) is treated as a literal replacement and
// never expanded by the regexp engine. A label like "[REDACTED:$1]" must
// appear verbatim in the output, not as a capture-group substitution.
func TestScanLabelWithDollarSign(t *testing.T) {
	t.Parallel()

	policy := gate.RedactPolicy{
		Name:    "dollar-label-test",
		Version: "0.0.1",
		Rules: []gate.RedactRule{
			{
				ID:          "T-DOLLAR",
				Pattern:     `(secret\d+)`,
				Label:       "[REDACTED:$1]", // "$1" must be literal, not a backreference
				Description: "label with dollar sign",
			},
		},
	}
	g, err := gate.NewGate(policy)
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}

	redacted, findings := g.Scan("value: secret42 here")
	// With ReplaceAllLiteralString the label is "[REDACTED:$1]" verbatim.
	// With ReplaceAllString (the old code) it would expand to "[REDACTED:secret42]"
	// — the secret would leak back into the output.
	if strings.Contains(redacted, "secret42") {
		t.Errorf("secret must not leak via $1 expansion: %q", redacted)
	}
	if redacted != "value: [REDACTED:$1] here" {
		t.Errorf("label must be literal (no backreference expansion): %q", redacted)
	}
	if len(findings) == 0 || findings[0].RuleID != "T-DOLLAR" {
		t.Errorf("expected T-DOLLAR finding, got: %v", findings)
	}
}

func TestFindingExcerptIsPopulated(t *testing.T) {
	t.Parallel()

	g, err := gate.NewGate(gate.DefaultPolicy())
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	_, findings := g.Scan("password=hunter2")
	for _, f := range findings {
		if f.Excerpt == "" {
			t.Errorf("Excerpt must not be empty for finding %q", f.RuleID)
		}
	}
}
