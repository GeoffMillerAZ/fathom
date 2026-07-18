package jsonllog_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	jsonllog "github.com/geoffmilleraz/agentic-kit/foundations/persistence/jsonl-log"
)

type sample struct {
	Name string
	N    int
}

// TestAppendWritesOneCompactLinePerValue verifies the canonical form: one
// compact json.Marshal encoding per line, newline-terminated, in append order.
func TestAppendWritesOneCompactLinePerValue(t *testing.T) {
	var buf bytes.Buffer
	w := jsonllog.NewWriter(&buf)

	values := []any{
		sample{Name: "alpha", N: 1},
		map[string]int{"b": 2, "a": 1}, // map keys sorted by json.Marshal
		nil,                            // any JSON value is accepted
	}
	for i, v := range values {
		if err := w.Append(v); err != nil {
			t.Fatalf("Append(%d): unexpected error: %v", i, err)
		}
	}

	want := `{"Name":"alpha","N":1}` + "\n" +
		`{"a":1,"b":2}` + "\n" +
		`null` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("output mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// TestAppendMarshalErrorWritesNothing verifies that an unmarshalable value is
// rejected before any bytes reach the underlying writer.
func TestAppendMarshalErrorWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	w := jsonllog.NewWriter(&buf)

	if err := w.Append(make(chan int)); err == nil {
		t.Fatal("expected marshal error for chan value, got nil")
	}
	if buf.Len() != 0 {
		t.Errorf("expected no bytes written after marshal failure, got %q", buf.String())
	}
}

// failingWriter always fails with a fixed error.
type failingWriter struct{ err error }

func (f failingWriter) Write(p []byte) (int, error) { return 0, f.err }

// TestAppendPropagatesWriteError verifies that the underlying writer's error
// is surfaced and remains matchable via errors.Is.
func TestAppendPropagatesWriteError(t *testing.T) {
	sentinel := errors.New("disk full")
	w := jsonllog.NewWriter(failingWriter{err: sentinel})

	err := w.Append(sample{Name: "x", N: 1})
	if err == nil {
		t.Fatal("expected write error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected errors.Is(err, sentinel), got %v", err)
	}
}

// TestConcurrentAppendsProduceWholeLines exercises the mutex guard: many
// goroutines appending concurrently must never interleave bytes within a
// line. Run with -race.
func TestConcurrentAppendsProduceWholeLines(t *testing.T) {
	const goroutines = 8
	const perGoroutine = 50

	var buf bytes.Buffer
	w := jsonllog.NewWriter(&buf)

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				if err := w.Append(sample{Name: fmt.Sprintf("g%d", g), N: i}); err != nil {
					t.Errorf("Append: %v", err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != goroutines*perGoroutine {
		t.Fatalf("expected %d lines, got %d", goroutines*perGoroutine, len(lines))
	}
	for i, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatalf("line %d is not valid JSON (interleaved write?): %q", i+1, line)
		}
	}
}
