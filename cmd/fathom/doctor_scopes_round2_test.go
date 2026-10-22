package main

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
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
}
