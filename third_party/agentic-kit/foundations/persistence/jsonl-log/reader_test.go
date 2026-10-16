package jsonllog_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	jsonllog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/persistence/jsonl-log"
)

// collect runs ReadAll over input and returns the delivered raw lines plus
// the terminal error.
func collect(input string) (lines []string, err error) {
	err = jsonllog.ReadAll(strings.NewReader(input), func(raw json.RawMessage) error {
		lines = append(lines, string(raw))
		return nil
	})
	return lines, err
}

// TestReadAllHappyPath verifies in-order delivery of complete lines.
func TestReadAllHappyPath(t *testing.T) {
	input := `{"a":1}` + "\n" + `{"b":2}` + "\n" + `[1,2,3]` + "\n"
	lines, err := collect(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{`{"a":1}`, `{"b":2}`, `[1,2,3]`}
	if len(lines) != len(want) {
		t.Fatalf("expected %d lines, got %d", len(want), len(lines))
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line[%d]: want %q, got %q", i, want[i], lines[i])
		}
	}
}

// TestReadAllEmptyInput verifies a zero-byte stream is a clean, empty log.
func TestReadAllEmptyInput(t *testing.T) {
	lines, err := collect("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lines != nil {
		t.Errorf("expected no lines, got %v", lines)
	}
}

// TestReadAllErrorsCarryLineNumbers is the table-driven corruption suite:
// every failure names its 1-based line and stops delivery at that line.
func TestReadAllErrorsCarryLineNumbers(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantDelivered int    // complete lines delivered before the error
		wantInError   string // substring that must appear in the error
	}{
		{
			name:          "invalid JSON mid-file",
			input:         `{"a":1}` + "\n" + `{"broken` + "\n" + `{"c":3}` + "\n",
			wantDelivered: 1,
			wantInError:   "line 2: invalid JSON",
		},
		{
			name:          "invalid JSON first line",
			input:         "not json\n",
			wantDelivered: 0,
			wantInError:   "line 1: invalid JSON",
		},
		{
			name:          "blank line",
			input:         `{"a":1}` + "\n" + "\n" + `{"c":3}` + "\n",
			wantDelivered: 1,
			wantInError:   "line 2: blank line",
		},
		{
			name:          "whitespace-only line",
			input:         `{"a":1}` + "\n" + "   \n",
			wantDelivered: 1,
			wantInError:   "line 2: blank line",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, err := collect(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantInError) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantInError)
			}
			if len(lines) != tt.wantDelivered {
				t.Errorf("expected %d delivered lines before error, got %d", tt.wantDelivered, len(lines))
			}
		})
	}
}

// TestReadAllFnErrorWrappedWithLineNumber verifies callback errors propagate
// with the line number while staying matchable via errors.Is.
func TestReadAllFnErrorWrappedWithLineNumber(t *testing.T) {
	sentinel := errors.New("reject")
	calls := 0
	err := jsonllog.ReadAll(strings.NewReader("{\"a\":1}\n{\"b\":2}\n{\"c\":3}\n"), func(raw json.RawMessage) error {
		calls++
		if calls == 2 {
			return sentinel
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected errors.Is(err, sentinel), got %v", err)
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("expected line number in error, got %q", err.Error())
	}
	if calls != 2 {
		t.Errorf("expected fn to stop after failing call (2 calls), got %d", calls)
	}
}

// TestReadAllPartialTrailingLine verifies the documented recovery mode: a
// torn tail surfaces as *PartialLineError AFTER complete lines are delivered.
func TestReadAllPartialTrailingLine(t *testing.T) {
	input := `{"a":1}` + "\n" + `{"b":2}` + "\n" + `{"c":` // torn mid-write
	lines, err := collect(input)
	if err == nil {
		t.Fatal("expected partial-line error, got nil")
	}
	if !errors.Is(err, jsonllog.ErrPartialTrailingLine) {
		t.Fatalf("expected errors.Is(err, ErrPartialTrailingLine), got %v", err)
	}
	var ple *jsonllog.PartialLineError
	if !errors.As(err, &ple) {
		t.Fatalf("expected *PartialLineError via errors.As, got %T", err)
	}
	if ple.Line != 3 {
		t.Errorf("PartialLineError.Line: want 3, got %d", ple.Line)
	}
	if string(ple.Bytes) != `{"c":` {
		t.Errorf("PartialLineError.Bytes: want %q, got %q", `{"c":`, ple.Bytes)
	}
	if len(lines) != 2 {
		t.Errorf("expected 2 complete lines delivered before partial error, got %d", len(lines))
	}
}

// TestReadAllUnterminatedValidJSONStillPartial verifies that a final line
// which parses as valid JSON but lacks the newline terminator is still
// reported as partial — completeness cannot be confirmed without the
// terminator (a torn prefix can itself be valid JSON).
func TestReadAllUnterminatedValidJSONStillPartial(t *testing.T) {
	input := `{"a":1}` + "\n" + `{"n":12}` // could be a torn `{"n":123}`
	lines, err := collect(input)
	if !errors.Is(err, jsonllog.ErrPartialTrailingLine) {
		t.Fatalf("expected ErrPartialTrailingLine, got %v", err)
	}
	var ple *jsonllog.PartialLineError
	if !errors.As(err, &ple) {
		t.Fatalf("expected *PartialLineError, got %T", err)
	}
	if string(ple.Bytes) != `{"n":12}` {
		t.Errorf("PartialLineError.Bytes: want %q, got %q", `{"n":12}`, ple.Bytes)
	}
	if len(lines) != 1 {
		t.Errorf("expected 1 complete line delivered, got %d", len(lines))
	}
}

// TestReadAllLongLines verifies there is no token-size limit: lines larger
// than bufio.Scanner's default 64 KiB buffer must round-trip.
func TestReadAllLongLines(t *testing.T) {
	big := strings.Repeat("x", 200_000)
	input := `{"pad":"` + big + `"}` + "\n"
	lines, err := collect(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	var decoded struct{ Pad string }
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
		t.Fatalf("unmarshal long line: %v", err)
	}
	if decoded.Pad != big {
		t.Error("long line payload corrupted in transit")
	}
}
