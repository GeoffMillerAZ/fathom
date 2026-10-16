package analytics

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
)

func TestLocalTimeZoneIsRejectedBeforeOutput(t *testing.T) {
	for _, kind := range []WindowKind{RangeWindow, SnapshotWindow} {
		t.Run(string(kind), func(t *testing.T) {
			d := fixture()
			d.Contract.Window.Kind = kind
			d.Contract.Window.TimeZone = "Local"
			if kind == SnapshotWindow {
				d.Contract.Window.End = time.Time{}
			}

			if err := d.Validate(testNow); err == nil || !strings.Contains(err.Error(), "explicit IANA") {
				t.Fatalf("Validate accepted process-local timezone: %v", err)
			}
			if output, err := RenderBars(d, viz.BarsOptions{}, testNow); err == nil || output != "" {
				t.Fatalf("RenderBars emitted process-dependent output: bytes=%d err=%v", len(output), err)
			}
			if output, err := RenderDivergingBars(d, viz.BarsOptions{}, testNow); err == nil || output != "" {
				t.Fatalf("RenderDivergingBars emitted process-dependent output: bytes=%d err=%v", len(output), err)
			}
			var output bytes.Buffer
			if err := WriteCSV(&output, d, testNow); err == nil || output.Len() != 0 {
				t.Fatalf("WriteCSV emitted process-dependent output: bytes=%d err=%v", output.Len(), err)
			}
		})
	}
}

func TestExplicitTimeZonesRetainExpectedLabels(t *testing.T) {
	tests := []struct {
		name, zone, want string
		kind             WindowKind
	}{
		{name: "UTC range", zone: "UTC", kind: RangeWindow, want: "2026-09-01T00:00:00Z to 2026-11-01T00:00:00Z (end exclusive; UTC)"},
		{name: "Phoenix snapshot", zone: "America/Phoenix", kind: SnapshotWindow, want: "Snapshot at 2026-08-31T17:00:00-07:00 [America/Phoenix]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := fixture()
			d.Contract.Window.Kind = tt.kind
			d.Contract.Window.TimeZone = tt.zone
			if tt.kind == SnapshotWindow {
				d.Contract.Window.End = time.Time{}
			}
			html, err := RenderBars(d, viz.BarsOptions{}, testNow)
			if err != nil || !strings.Contains(html, tt.want) {
				t.Fatalf("HTML label mismatch: want %q err=%v", tt.want, err)
			}
			var csv bytes.Buffer
			if err := WriteCSV(&csv, d, testNow); err != nil || !strings.Contains(csv.String(), tt.want) {
				t.Fatalf("CSV label mismatch: want %q err=%v", tt.want, err)
			}
		})
	}
}
