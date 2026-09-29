package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestHostMarshaledContractsMatchCUE(t *testing.T) {
	cue, err := exec.LookPath("cue")
	if err != nil {
		t.Skip("CUE unavailable; task cue-vet has the same optional prerequisite")
	}
	for _, host := range []string{"claude", "codex"} {
		f := newHostFixture(t, host)
		plan := f.install(t)
		receipt := inspectHost(f.root, f.dir, host, host, "", fakeHostDeps(host))
		for _, tc := range []struct {
			name  string
			value any
		}{
			{"#Policy", f.policy}, {"#Policy", hostPolicy{Version: 1}},
			{"#Plan", plan}, {"#Receipt", receipt},
		} {
			raw, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "actual.json")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			out, err := exec.CommandContext(ctx, cue, "vet", "schema.cue", path, "-d", tc.name).CombinedOutput()
			cancel()
			if err != nil {
				t.Fatalf("%s actual %s violates CUE: %v\n%s", host, tc.name, err, out)
			}
		}
	}
}
