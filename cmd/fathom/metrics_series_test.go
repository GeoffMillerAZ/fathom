package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/metrics"
)

func TestCmdMetricsSeries_DayBucket_RollupsPlusLiveTail(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "workspace", "artifacts", "metrics", "daily")
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	if err := metrics.WriteDailyRollup(dir, metrics.DailyRollup{
		Date: yesterday.Format("2006-01-02"), SpendTotalUSD: 1.5,
	}); err != nil {
		t.Fatalf("WriteDailyRollup: %v", err)
	}

	var out, errBuf bytes.Buffer
	code := cmdMetricsSeries([]string{
		"--bucket", "day", "--since", "48h", "--root", root,
		"--call-log", filepath.Join(t.TempDir(), "none.jsonl"),
		"--json", "spend_total_usd",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	points, _ := payload["points"].([]any)
	if len(points) < 2 {
		t.Fatalf("points = %+v, want yesterday's rollup PLUS today's live tail", points)
	}
	var sawYesterday bool
	for _, p := range points {
		pt := p.(map[string]any)
		if pt["bucket"] == yesterday.Format("2006-01-02") {
			sawYesterday = true
			if pt["value"].(float64) != 1.5 {
				t.Errorf("yesterday's value = %v, want 1.5", pt["value"])
			}
		}
	}
	if !sawYesterday {
		t.Fatalf("points = %+v, want yesterday's rollup present", points)
	}
}

func TestCmdMetricsSeries_HourBucket_LiveOnly(t *testing.T) {
	root := t.TempDir()
	var out, errBuf bytes.Buffer
	code := cmdMetricsSeries([]string{
		"--bucket", "hour", "--since", "3h", "--root", root,
		"--call-log", filepath.Join(t.TempDir(), "none.jsonl"),
		"--json", "traffic_total_calls",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	json.Unmarshal(out.Bytes(), &payload)
	if payload["note"] == "" {
		t.Error("note is empty, want the hour-bucket honesty caveat")
	}
	points, _ := payload["points"].([]any)
	if len(points) == 0 {
		t.Fatal("points is empty, want at least one hour bucket")
	}
}

func TestCmdMetricsSeries_UnknownMetric_ListsValidNames(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := cmdMetricsSeries([]string{"not_a_real_metric"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !bytes.Contains(errBuf.Bytes(), []byte("spend_total_usd")) {
		t.Errorf("stderr = %q, want it to list valid metric names", errBuf.String())
	}
}

// TestCmdMetricsSeries_MetricFirst_AlsoAccepted: the natural
// "subcommand-first" typing (`metrics series <metric> --flags`) must work
// too, not just flag.Parse's native positional-after-flags order.
func TestCmdMetricsSeries_MetricFirst_AlsoAccepted(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := cmdMetricsSeries([]string{
		"traffic_total_calls", "--bucket", "hour", "--since", "1h", "--json",
		"--call-log", filepath.Join(t.TempDir(), "none.jsonl"),
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
	}
	if payload["metric"] != "traffic_total_calls" {
		t.Fatalf("metric = %v, want traffic_total_calls", payload["metric"])
	}
}

func TestCmdMetricsSeries_RequiresExactlyOnePositionalArg(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := cmdMetricsSeries(nil, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (missing <metric>)", code)
	}
}
