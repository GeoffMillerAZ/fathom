package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestUsageDoesNotEmitAReceipt(t *testing.T) {
	for _, args := range [][]string{nil, {"-unknown"}, {"-root", "."}, {"extra"}} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), args, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("%q: code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestHelpAndPreparationFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"-help"}, &stdout, &stderr); code != 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "-out") {
		t.Fatalf("help: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stderr.Reset()
	if code := run(context.Background(), []string{"-root", t.TempDir(), "-out", t.TempDir()}, &stdout, &stderr); code != 1 || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("failure: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
