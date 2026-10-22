package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

func TestAD5EditHookInstructionsAndWithheldLog(t *testing.T) {
	for _, instruction := range []bool{false, true} {
		t.Run(fmt.Sprint(instruction), func(t *testing.T) {
			dir := t.TempDir()
			log := filepath.Join(t.TempDir(), "hooks.jsonl")
			t.Setenv("FATHOM_HOOKS_LOG", log)
			layer, err := semantic.LoadLayer(semantic.LayerObligations)
			if err != nil {
				t.Fatal(err)
			}
			body := "# Heading only\n"
			if instruction {
				body += "\n# Current rules\nAlways sort outputs.\n"
			}
			rules, err := semantic.RawFallbackRules(nil, layer, []core.SourceFile{{Path: "AGENTS.md", Content: []byte(body)}}, "pkg/a.go", 2, "coverage gap", nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			http.DefaultTransport = ad5RoundTripper(func(r *http.Request) (*http.Response, error) {
				payload := map[string]any{"service": strings.ToLower(filepath.Base(dir)), "file": "pkg/a.go", "rules": rules, "raw_withheld": 1}
				envelope := map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": mustJSON(payload)}}}}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(mustJSON(envelope))), Header: make(http.Header)}, nil
			})
			hookStdin(t, fmt.Sprintf(`{"tool_input":{"file_path":%q},"cwd":%q}`, filepath.Join(dir, "pkg/a.go"), dir))
			var out, stderr bytes.Buffer
			if code := runWithStdin([]string{"edit-hook", "--url=http://127.0.0.1:0"}, nil, &out, &stderr); code != 0 {
				t.Fatalf("exit=%d: %s", code, &stderr)
			}
			if instruction && !strings.Contains(out.String(), "Always sort outputs.") {
				t.Errorf("missing instruction: %s", &out)
			}
			if !instruction && out.Len() != 0 {
				t.Errorf("heading-only output: %s", &out)
			}
			if strings.Contains(out.String(), "Heading only") || strings.Contains(out.String(), "raw_withheld") {
				t.Errorf("diagnostic leaked to payload: %s", &out)
			}
			beats, err := readRetroBeats(log)
			if err != nil {
				t.Fatal(err)
			}
			answered := 0
			for _, b := range beats {
				if b.Channel == "edit-hook" && b.Kind == "answered" {
					answered++
					if b.Meta["raw_withheld"] != "1" {
						t.Errorf("answered row = %+v", b)
					}
				}
			}
			if answered != 1 {
				t.Errorf("answered rows=%d, want one existing row", answered)
			}
		})
	}
}

type ad5RoundTripper func(*http.Request) (*http.Response, error)

func (f ad5RoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
