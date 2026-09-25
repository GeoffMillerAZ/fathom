package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestReportDeadlineResultRequiresBothErrorIdentities(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code int
	}{
		{"both", errors.Join(context.DeadlineExceeded, tea.ErrProgramKilled), 0},
		{"deadline only", context.DeadlineExceeded, 1},
		{"program killed only", tea.ErrProgramKilled, 1},
		{"nil", nil, 1},
		{"unrelated", errors.New("unexpected"), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := reportDeadlineResult(tc.err, &stdout, &stderr); got != tc.code {
				t.Fatalf("exit code = %d, want %d", got, tc.code)
			}
			if tc.code == 0 {
				if got := stdout.String(); got != deadlineMarker+"\n" || stderr.Len() != 0 {
					t.Fatalf("success output = stdout %q stderr %q", got, stderr.String())
				}
				return
			}
			if stdout.Len() != 0 || !strings.Contains(stderr.String(), "expected context deadline and tea program-killed errors") {
				t.Fatalf("failure output = stdout %q stderr %q", stdout.String(), stderr.String())
			}
		})
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestReportDeadlineResultRefusesUnwrittenSuccessMarker(t *testing.T) {
	markerErr := errors.New("broken PTY output")
	var stderr bytes.Buffer
	if got := reportDeadlineResult(errors.Join(context.DeadlineExceeded, tea.ErrProgramKilled), failingWriter{markerErr}, &stderr); got != 1 {
		t.Fatalf("exit code = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "write success marker") || !strings.Contains(stderr.String(), markerErr.Error()) {
		t.Fatalf("marker write failure was not reported: %q", stderr.String())
	}
}
