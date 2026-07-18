package prellmdlpgate_test

import (
	"strings"
	"testing"

	gate "github.com/geoffmilleraz/agentic-kit/foundations/security/pre-llm-dlp-gate"
)

// FuzzScan checks robustness of a gate compiled from ExtendedPolicy (a
// superset of DefaultPolicy):
//
//  1. Scan must never panic, whatever bytes arrive (including invalid UTF-8).
//  2. Redaction must be idempotent: scanning already-redacted output is a
//     fixed point. Labels start with "[", end with "]", and contain no rule
//     trigger substrings, so a second pass must find nothing new.
//
// The seed corpus includes a trigger for every shipped rule plus a 1 MiB
// input; seeds run as part of plain `go test`, so these properties are
// exercised even without -fuzz.
func FuzzScan(f *testing.F) {
	g, err := gate.NewGate(gate.ExtendedPolicy())
	if err != nil {
		f.Fatalf("NewGate(ExtendedPolicy()): %v", err)
	}

	megabyte := strings.Repeat("padding AKIAIOSFODNN7EXAMPLE secret ", 1<<15)[:1<<20]

	seeds := []string{
		"",
		"plain prose with no secrets at all",
		"Authorization: Bearer abc.def.ghi", // DLP-001
		"api_key=sk-abc123secret",           // DLP-002
		"password=hunter2",                  // DLP-003
		"Card: 4111 1111 1111 1111 on file", // DLP-004
		"-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA7bq6t8H\n-----END RSA PRIVATE KEY-----",                      // DLP-101
		"the key AKIAIOSFODNN7EXAMPLE was used",                                                                        // DLP-102
		"ghp_AbCd1234EfGh5678IjKl9012MnOp3456QrSt",                                                                     // DLP-103 classic
		"github_pat_11AAAAAAA0abcdefghijklmnopqrstuvwxyz",                                                              // DLP-103 fine-grained
		"xoxb-not-a-real-token-AbCdEfGhIjKlMnOp",                                                                       // DLP-104
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c", // DLP-105
		"key=AIza" + strings.Repeat("Ab-_9", 7) + " ok",                                                                // DLP-106
		"[REDACTED:JWT] pre-redacted label in user text",
		"AKIA short prefix and eyJword prose must survive",
		"\xff\xfe invalid utf-8 with password=p\xc3 tail",
		megabyte, // 1 MiB: throughput + excerpt/replacement robustness at size
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		once, _ := g.Scan(input) // property 1: must not panic
		twice, _ := g.Scan(once)
		if twice != once {
			t.Errorf("redaction not idempotent:\n input (%d bytes): %.200q\n  once: %.200q\n twice: %.200q",
				len(input), input, once, twice)
		}
	})
}
