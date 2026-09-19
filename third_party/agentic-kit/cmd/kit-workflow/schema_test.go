package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"testing"
	"time"

	prose "github.com/geoffmilleraz/agentic-kit/kits/governed-prose-kit"
)

func TestActualCLIJSONMatchesCUE(t *testing.T) {
	cue, err := exec.LookPath("cue")
	if err != nil {
		t.Skip("CUE tool unavailable; task cue-vet has the same optional prerequisite")
	}
	vet := func(t *testing.T, schema string, raw []byte) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, cue, "vet", "schema.cue", "json:", "-", "-d", schema)
		cmd.Stdin = bytes.NewReader(raw)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("actual CLI JSON violates %s: %v\n%s", schema, err, out)
		}
	}
	for _, name := range []string{"check", "historical-only", "export", "public-redaction", "restricted-export"} {
		t.Run(name, func(t *testing.T) {
			request := fixtureInput(t)
			operation := "check"
			switch name {
			case "historical-only":
				request.Current = nil
				request.Request.Facts[0].Historical = true
				request.Draft = prose.ProseDraft{Units: []prose.ProseUnit{{ID: "heading", Text: "Summary", NonFactualReason: "reviewed heading"}}}
				request.NonFactual = []string{"Summary"}
			case "export", "public-redaction", "restricted-export":
				operation, request.Audience = "export", "public"
				if name != "export" {
					request.Sources[0].Access = "restricted"
				}
				if name == "restricted-export" {
					request.Audience = "restricted"
				}
			}
			raw, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			vet(t, "#Input", raw)
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), []string{"prose", operation}, bytes.NewReader(raw), &stdout, &stderr, func() time.Time { return fixtureNow })
			if code != 0 {
				t.Fatalf("command failed: %d %s %s", code, stdout.Bytes(), stderr.Bytes())
			}
			vet(t, "#Output", stdout.Bytes())
			if name == "public-redaction" {
				var result output
				if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.Export == nil || result.Export.Markdown != "[redacted]" || len(result.Export.SourceMap) != 0 {
					t.Fatal("expected public redaction with empty sidecar")
				}
			}
		})
	}
	t.Run("failure", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if run(context.Background(), nil, nil, &stdout, &stderr, nil) == 0 {
			t.Fatal("usage error succeeded")
		}
		vet(t, "#Output", stdout.Bytes())
	})
}
