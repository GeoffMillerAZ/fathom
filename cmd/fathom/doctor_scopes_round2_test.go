package main

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorGuardLiteralPrefixes(t *testing.T) {
	root := t.TempDir()
	writeGuardCue(t, root, `guards: [{scopes: ["assets/**"]}]`)
	doctorWriteFixture(t, root, "assets/ui.css", "fixture")
	// Unrelated paths must not consume the scope's discovery work.
	for i := 0; i < 8200; i++ {
		doctorWriteFixture(t, root, fmt.Sprintf("a/file%04d", i), "")
	}
	row := checkGuardDeadScopes(&http.Client{Transport: doctorDeliveryTransport{}}, "http://daemon.invalid", root, "fixture", true, context.Background())
	if row.Status != doctorPass {
		t.Fatalf("literal-prefix scope was not completed: %+v", row)
	}
}

func TestDoctorGuardCheckoutScopes(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	hc := &http.Client{Transport: doctorDeliveryTransport{body: `{"result":{"content":[{"type":"text","text":"{\"sources\":[]}"}]}}`}}
	row := checkGuardDeadScopes(hc, "http://daemon.invalid", root, "fixture", true, context.Background())
	if row.Status != doctorPass {
		t.Fatalf("checkout guard scopes: %+v", row)
	}
	if !strings.Contains(row.Reason, "waiting for files") {
		t.Errorf("empty receipt scope not explained: %+v", row)
	}
}

func TestDoctorGuardIgnoredScopesWaitForFiles(t *testing.T) {
	for _, tc := range []struct {
		name, ignore, scope string
		want                doctorStatus
	}{
		{"ignored directory", "workspace/\n", "workspace/receipts/**", doctorPass},
		{"ignored file", "pending.json\n", "pending.json", doctorPass},
		{"tracked path typo", "workspace/\n", "src/missing/**", doctorWarn},
		{"mixed ignored and tracked", "workspace/\n", "workspace/receipts/**", doctorWarn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			c := doctorGitCommand(t.Context(), root, "init", "-q")
			if out, err := c.CombinedOutput(); err != nil {
				t.Fatalf("git init: %v: %s", err, out)
			}
			doctorWriteFixture(t, root, ".gitignore", tc.ignore)
			scopes := fmt.Sprintf("%q", tc.scope)
			if tc.name == "mixed ignored and tracked" {
				scopes += `, "src/missing/**"`
			}
			writeGuardCue(t, root, `guards: [{scopes: [`+scopes+`]}]`)
			hc := &http.Client{Transport: doctorDeliveryTransport{body: `{"result":{"content":[{"type":"text","text":"{\"sources\":[]}"}]}}`}}
			row := checkGuardDeadScopes(hc, "http://daemon.invalid", root, "fixture", true, t.Context())
			if row.Status != tc.want {
				t.Fatalf("scope = %+v, want %s", row, tc.want)
			}
			if tc.want == doctorPass && !strings.Contains(row.Reason, "waiting for files") {
				t.Errorf("waiting scope not explained: %+v", row)
			}
		})
	}
}
