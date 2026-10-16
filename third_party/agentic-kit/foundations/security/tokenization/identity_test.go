package tokenization_test

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/tokenization"
)

func TestVaultStoredIdentity(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		name := "memory"
		if encrypted {
			name = "encrypted"
		}
		t.Run(name, func(t *testing.T) {
			ev, tz, _ := newEncryptedTestVault(t)
			var v tokenization.Vault = ev
			if !encrypted {
				v = tokenization.NewMemoryVault(tz, tokenization.NewRolePolicy(map[string][]tokenization.TokenClass{"admin": {tokenization.ClassPII, tokenization.ClassOther}}), fixedClock(time.Unix(0, 0)))
			}
			tok, err := v.Tokenize("private", tokenization.ClassPII, "tenant-a", encVaultAuthor)
			if err != nil {
				t.Fatal(err)
			}
			for _, forged := range []tokenization.Token{
				{Value: tok.Value, Class: tokenization.ClassOther, Namespace: tok.Namespace},
				{Value: tok.Value, Class: tok.Class, Namespace: "tenant-b"},
				{Value: "tok_pii_unknown", Class: tok.Class, Namespace: tok.Namespace},
			} {
				if value, err := v.Detokenize(forged, encAdminPrincipal, "test"); err == nil || value != "" {
					t.Errorf("forged identity resolved: %+v -> %q, %v", forged, value, err)
				}
				log := v.AuditLog()
				if log[len(log)-1].Allowed {
					t.Error("failed lookup audited as allowed")
				}
			}
			parsed, err := tokenization.Parse(tok.Value)
			if err != nil {
				t.Fatal(err)
			}
			if value, err := v.Detokenize(parsed, encAdminPrincipal, "test"); err != nil || value != "private" {
				t.Fatalf("parsed token failed: %q, %v", value, err)
			}
		})
	}
}
