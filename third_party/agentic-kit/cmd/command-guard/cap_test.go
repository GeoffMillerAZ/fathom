package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func inspectArgs(t *testing.T, root string, s scope, p policy, q request, format string) []string {
	t.Helper()
	dir := t.TempDir()
	for name, v := range map[string]any{"scope": s, "policy": p, "request": q} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".json"), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return []string{"inspect", "--root", root, "--scope", filepath.Join(dir, "scope.json"), "--policy", filepath.Join(dir, "policy.json"), "--request", filepath.Join(dir, "request.json"), "--format", format}
}

func TestInspectFailureOutputCaps(t *testing.T) {
	for _, format := range []string{"json", "text", "compact"} {
		for _, failure := range []string{"projection", "digest", "denied", "read"} {
			t.Run(format+"/"+failure, func(t *testing.T) {
				root, s, p, q := fixture(t)
				p.Layers[0].Limits.MaxOutputBytes = 1
				switch failure {
				case "digest":
					q.Files[0].SHA256 = strings.Repeat("0", 64)
				case "denied":
					p.Layers[0].DeniedPathPrefixes = []string{"a.txt"}
				case "read":
					q.Files[0].Path = "missing.txt"
				}
				var out bytes.Buffer
				if code := execute(inspectArgs(t, root, s, p, q, format), &out); code != 2 || out.Len() != 0 {
					t.Fatalf("code=%d bytes=%d", code, out.Len())
				}
			})
		}
	}
}

func TestInspectFailureExactOutputBoundary(t *testing.T) {
	for _, format := range []string{"json", "text", "compact"} {
		t.Run(format, func(t *testing.T) {
			root, s, p, q := fixture(t)
			q.Files[0].SHA256 = strings.Repeat("0", 64)
			r := run(root, s, p, q, format)
			want := append(encode(r, format), '\n')
			for _, delta := range []int{-1, 0, 1} {
				p.Layers[0].Limits.MaxOutputBytes = len(want) + delta
				var out bytes.Buffer
				if code := execute(inspectArgs(t, root, s, p, q, format), &out); code != 2 {
					t.Fatalf("delta %d code %d", delta, code)
				}
				if delta < 0 {
					if out.Len() != 0 {
						t.Fatalf("one byte short emitted %d", out.Len())
					}
				} else if !bytes.Equal(out.Bytes(), want) {
					t.Fatalf("delta %d changed report", delta)
				}
			}
		})
	}
}

func TestInspectSuccessAndWriterAfterCapCheck(t *testing.T) {
	root, s, p, q := fixture(t)
	p.Layers[0].Limits.MaxOutputBytes = 4096
	for _, format := range []string{"json", "text", "compact"} {
		r := run(root, s, p, q, format)
		want := append(encode(r, format), '\n')
		var out bytes.Buffer
		args := inspectArgs(t, root, s, p, q, format)
		if code := execute(args, &out); code != 0 || !bytes.Equal(out.Bytes(), want) {
			t.Fatalf("%s code %d changed success", format, code)
		}
		if code := execute(args, shortWriter{}); code != 2 {
			t.Fatalf("%s short writer code %d", format, code)
		}
	}
}
