package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestScriptScratchDaemonsNameTheirCallLog (RT-22 review, I52): a script
// that starts a scratch serve-http passes --call-log. Without it the log
// defaults through the working directory's state dir — in this repository
// that is workspace/logs/mcp-calls.jsonl, the managed daemon's own — and
// stranger-check (run by `task validate` from the real checkout, under a
// throwaway HOME that holds no record to refuse against) appended its
// probe calls to the operator's call log.
func TestScriptScratchDaemonsNameTheirCallLog(t *testing.T) {
	scripts, err := filepath.Glob(filepath.Join("..", "..", "scripts", "*.sh"))
	if err != nil || len(scripts) == 0 {
		t.Fatalf("no scripts found: %v", err)
	}
	// A command line that starts serve-http: "serve-http" followed by a
	// --store/-store flag (the plist heredoc and pgrep patterns have none).
	start := regexp.MustCompile(`\bserve-http\b.*\s--?store\b`)
	for _, p := range scripts {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") || !start.MatchString(line) {
				continue
			}
			if !regexp.MustCompile(`\s--?call-log\b`).MatchString(line) {
				t.Errorf("%s:%d starts serve-http without --call-log (its default is the working directory's state dir — the operator's log here):\n  %s", p, i+1, strings.TrimSpace(line))
			}
		}
	}
}
