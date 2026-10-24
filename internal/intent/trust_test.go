package intent_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

// TestAnnotationsAreRootConfined (EP-37): an annotation is repository
// content — it arrives with a clone or a pull request. A hostile one must
// not be able to read outside the repo or query another repo's index.
func TestAnnotationsAreRootConfined(t *testing.T) {
	root := t.TempDir()
	local := map[string]bool{"fathom": true}

	for _, tc := range []struct {
		name   string
		ann    map[string]string
		reason string
	}{
		{"parent escape", map[string]string{"oracle": "lexical", "paths": "../other-repo/secrets.txt"}, "escapes the repository"},
		{"absolute path", map[string]string{"oracle": "lexical", "paths": "/etc/passwd"}, "absolute or home-relative"},
		{"home path", map[string]string{"oracle": "lexical", "paths": "~/.ssh/id_rsa"}, "absolute or home-relative"},
		{"comma absolute path", map[string]string{"paths": "docs/a, /etc/passwd"}, "absolute or home-relative"},
		{"comma parent escape", map[string]string{"paths": "docs/a,../x"}, "escapes the repository"},
		{"comma home path", map[string]string{"paths": "docs/a, ~/.ssh/id_rsa"}, "absolute or home-relative"},
		{"deep escape", map[string]string{"oracle": "plan", "plan": "docs/../../elsewhere/x.md"}, "escapes the repository"},
		{"foreign service", map[string]string{"oracle": "index", "service": "other-repo", "symbol": "X"}, "cross-repo refused"},
	} {
		err := intent.ConfineAnnotation(root, tc.ann, local)
		if err == nil {
			t.Errorf("%s: must be refused", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.reason) {
			t.Errorf("%s: reason %q does not name %q", tc.name, err.Error(), tc.reason)
		}
	}

	// Legitimate annotations pass untouched.
	for _, ok := range []map[string]string{
		{"oracle": "lexical", "paths": ".fathom/capabilities.cue", "pattern": "budgets:"},
		{"oracle": "index", "service": "fathom", "symbol": "Load", "present": "true"},
		{"oracle": "index", "service": "FaThOm", "symbol": "Load", "present": "true"},
		{"oracle": "lexical", "paths": "docs/a, docs/b; docs/c"},
		{"oracle": "plan", "plan": "docs/plans/custodian.md", "rows": "all-proven"},
		{"oracle": "date", "after": "2027-01-01"},
		{},
	} {
		if err := intent.ConfineAnnotation(root, ok, local); err != nil {
			t.Errorf("a legitimate annotation was refused: %v (%v)", err, ok)
		}
	}

	// With no local-service set known, a service is not second-guessed —
	// the caller that cannot enumerate services must not manufacture a
	// refusal it cannot justify.
	if err := intent.ConfineAnnotation(root, map[string]string{"service": "anything"}, nil); err != nil {
		t.Errorf("with no known service set, a service must not be refused: %v", err)
	}
}
