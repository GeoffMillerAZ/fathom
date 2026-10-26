//go:build !harness_maintenance

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHooksDefaultBuildUnsupportedBeforeInput(t *testing.T) {
	var out, errOut bytes.Buffer
	args := []string{"evaluate", "--controller", "/does-not-exist", "--controller-sha256", strings.Repeat("a", 64), "--project", "/also-missing", "--host", "codex", "--input", "/missing"}
	if code := runHooks(args, &out, &errOut); code != 2 || !bytes.Contains(out.Bytes(), []byte(`"code":"unsupported"`)) || errOut.Len() != 0 {
		t.Fatalf("evaluate code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	args[0] = "run"
	if code := runHooks(args, &out, &errOut); code != 2 || out.Len() != 0 || errOut.String() != "harnessctl spawn check blocked\n" {
		t.Fatalf("run code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}
