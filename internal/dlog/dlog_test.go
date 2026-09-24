package dlog

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestNew_WritesTimestampedLeveledLines pins the core contract: every
// emitted line carries an RFC3339 timestamp (from the injected clock, not
// time.Now), the level, and the message.
func TestNew_WritesTimestampedLeveledLines(t *testing.T) {
	var buf bytes.Buffer
	fixed := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	logger := New(&buf, LevelInfo, func() time.Time { return fixed })

	logger.Info("scavenge: heap freed", "heap_sys_mb", 512)

	line := buf.String()
	wantTS := fixed.Format(time.RFC3339)
	if !strings.Contains(line, wantTS) {
		t.Errorf("line %q does not contain RFC3339 timestamp %q", line, wantTS)
	}
	if !strings.Contains(line, "level=INFO") {
		t.Errorf("line %q does not name the level", line)
	}
	if !strings.Contains(line, "scavenge: heap freed") {
		t.Errorf("line %q does not contain the message", line)
	}
	if !strings.Contains(line, "heap_sys_mb=512") {
		t.Errorf("line %q does not contain the key=value arg", line)
	}
}

// TestNew_LevelFiltersBelowThreshold pins that a Logger constructed at
// LevelWarn drops Debug/Info lines entirely rather than buffering them.
func TestNew_LevelFiltersBelowThreshold(t *testing.T) {
	var buf bytes.Buffer
	fixed := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	logger := New(&buf, LevelWarn, func() time.Time { return fixed })

	logger.Info("should not appear")
	logger.Debug("should not appear either")
	if buf.Len() != 0 {
		t.Fatalf("expected no output below the configured level, got %q", buf.String())
	}
	logger.Warn("should appear")
	if !strings.Contains(buf.String(), "should appear") {
		t.Errorf("expected the Warn line to be written, got %q", buf.String())
	}
}

// TestLevelFromEnv_DefaultsAndOverrides mirrors
// TestScavengeIntervalFromEnv_DefaultsAndOverrides's table style
// (cmd/fathom/memscavenge_test.go): unset/invalid defaults to LevelInfo; a
// recognized value (any case) overrides it. getenv is injected — no
// t.Setenv needed.
func TestLevelFromEnv_DefaultsAndOverrides(t *testing.T) {
	cases := []struct {
		name string
		val  string
		want Level
	}{
		{"unset defaults to info", "", LevelInfo},
		{"invalid value falls back to info", "verbose", LevelInfo},
		{"debug", "debug", LevelDebug},
		{"upper-case DEBUG", "DEBUG", LevelDebug},
		{"info", "info", LevelInfo},
		{"warn", "warn", LevelWarn},
		{"warning alias", "warning", LevelWarn},
		{"error", "error", LevelError},
		{"padded with whitespace", "  error  ", LevelError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(key string) string {
				if key != "FATHOM_LOG_LEVEL" {
					t.Fatalf("unexpected getenv key %q", key)
				}
				return tc.val
			}
			if got := LevelFromEnv(getenv); got != tc.want {
				t.Errorf("LevelFromEnv(%q) = %v, want %v", tc.val, got, tc.want)
			}
		})
	}
}

// TestLogger_DedupeCountsRepeatsAndFlushesSummary pins DG-2: the first
// occurrence of a key logs immediately; repeats are counted, not emitted;
// FlushDedupe emits exactly one summary line naming the repeat count.
func TestLogger_DedupeCountsRepeatsAndFlushesSummary(t *testing.T) {
	var buf bytes.Buffer
	fixed := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	logger := New(&buf, LevelInfo, func() time.Time { return fixed })

	for i := 0; i < 5; i++ {
		logger.Dedupe(LevelWarn, "fleet:sources:demo", "fleet: sources for %s: %v", "demo", "boom")
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected exactly one immediate line for 5 calls with the same key, got %d: %q", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], "fleet: sources for demo: boom") {
		t.Errorf("immediate line %q missing rendered message", lines[0])
	}

	buf.Reset()
	flushAt := fixed.Add(time.Minute)
	logger.FlushDedupe(flushAt)

	flushed := strings.TrimRight(buf.String(), "\n")
	if flushed == "" {
		t.Fatal("expected FlushDedupe to emit a summary line, got none")
	}
	if strings.Count(flushed, "\n") != 0 {
		t.Fatalf("expected exactly one flush line, got %q", flushed)
	}
	if !strings.Contains(flushed, "repeated 4 times") {
		t.Errorf("flush line %q does not say 'repeated 4 times'", flushed)
	}
	if !strings.Contains(flushed, fixed.UTC().Format(time.RFC3339)) {
		t.Errorf("flush line %q does not name the first-seen timestamp", flushed)
	}

	// A second flush with nothing new pending emits nothing further — keys
	// are cleared on flush.
	buf.Reset()
	logger.FlushDedupe(flushAt.Add(time.Minute))
	if buf.Len() != 0 {
		t.Errorf("expected no output on a flush with nothing pending, got %q", buf.String())
	}
}

// TestLogger_DedupeDistinctKeysEachGetTheirOwnFirstLine pins that
// dedup scoping is per-key, not global — a burst for service A does not
// suppress the first occurrence for service B.
func TestLogger_DedupeDistinctKeysEachGetTheirOwnFirstLine(t *testing.T) {
	var buf bytes.Buffer
	fixed := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	logger := New(&buf, LevelInfo, func() time.Time { return fixed })

	logger.Dedupe(LevelWarn, "fleet:sources:a", "fleet: sources for %s: %v", "a", "boom")
	logger.Dedupe(LevelWarn, "fleet:sources:b", "fleet: sources for %s: %v", "b", "boom")

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected one immediate line per distinct key, got %d: %q", len(lines), buf.String())
	}
}
