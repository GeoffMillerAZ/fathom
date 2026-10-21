package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

const preActionPrefix = "tenancy: pre-action memo "

func TestPreActionMemoIsSampledBeforeUpwardActionsOnly(t *testing.T) {
	r := newGovRig(t)
	reads := 0
	r.g.memoStats = func() []memo.GroupStats {
		reads++
		r.record("read memo")
		return []memo.GroupStats{{Name: "graphstore", Entries: 7, Bytes: 12345,
			Hits: 8, Misses: 4, Flights: 4, Evictions: map[memo.DropReason]int64{memo.ReasonBudget: 2}}}
	}
	var records []preActionRecord
	r.g.logf = func(line string) {
		if !strings.HasPrefix(line, preActionPrefix) {
			return
		}
		r.record("emit memo")
		var rec preActionRecord
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, preActionPrefix)), &rec); err != nil {
			t.Fatalf("pre-action record: %v", err)
		}
		records = append(records, rec)
	}
	r.ticks(3)
	if reads != 0 || len(records) != 0 {
		t.Fatalf("normal samples read or emit memo stats: reads=%d records=%d", reads, len(records))
	}
	r.host.set(3200*tenancy.MiB, 0)
	r.ticks(2)
	if reads != 1 || len(records) != 1 {
		t.Fatalf("shed must capture once: reads=%d records=%d", reads, len(records))
	}
	if got := records[0]; got.FootprintBytes != 3200*tenancy.MiB || got.From != "normal" || got.To != "shed" || got.Trigger != "own_mark" ||
		len(got.Groups) != 1 || got.Groups[0].Name != "graphstore" || got.Groups[0].EstimatedBytes != 12345 || got.Groups[0].EvictedBudget != 2 {
		t.Fatalf("wrong pre-action shed evidence: %+v", got)
	}
	r.ticks(5)
	if reads != 1 || len(records) != 1 {
		t.Fatalf("steady shed sampled memo: reads=%d records=%d", reads, len(records))
	}
	r.host.set(4500*tenancy.MiB, 0)
	r.ticks(2)
	if reads != 2 || len(records) != 2 || records[1].To != "refuse" || records[1].FootprintBytes != 4500*tenancy.MiB {
		t.Fatalf("refuse pre-action evidence missing: reads=%d records=%+v", reads, records)
	}
	r.mu.Lock()
	events := append([]string(nil), r.events...)
	r.mu.Unlock()
	// Each record precedes that transition's cache drops and scavenging.
	for len(events) > 0 && events[0] != "read memo" {
		events = events[1:] // a footprint-threshold capture can be queued first
	}
	want := []string{"read memo", "emit memo", "drop console", "drop fleet", "pause", "free",
		"read memo", "emit memo", "drop console", "drop fleet", "drop graph", "drop symbols", "free"}
	if len(events) < len(want) {
		t.Fatalf("missing actions: %v", events)
	}
	for i, e := range want {
		if events[i] != e {
			t.Fatalf("event %d=%q, want %q: %v", i, events[i], e, events)
		}
	}
	r.host.set(tenancy.GiB, 0)
	r.ticks(70)
	if reads != 2 || len(records) != 2 {
		t.Fatalf("downward/normal samples read or emitted memo: reads=%d records=%d", reads, len(records))
	}
}

func TestPreActionMemoLineBoundAndUnknownEvictions(t *testing.T) {
	stats := make([]memo.GroupStats, 100)
	for i := range stats {
		stats[i] = memo.GroupStats{Name: fmt.Sprintf("%03d-%s", 99-i, strings.Repeat("x", 500)),
			Entries: 1, Bytes: 32, Evictions: map[memo.DropReason]int64{"future": 7}}
	}
	line := preActionMemoLine(tenancy.Sample{At: time.Unix(1, 0), Footprint: 2}, tenancy.Budget{},
		tenancy.Transition{From: tenancy.LevelNormal, To: tenancy.LevelShed}, stats)
	if len(line) > preActionMemoBytes {
		t.Fatalf("line has %d bytes, cap %d", len(line), preActionMemoBytes)
	}
	var record preActionRecord
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, preActionPrefix)), &record); err != nil {
		t.Fatal(err)
	}
	if record.OmittedGroups != 100-len(record.Groups) || len(record.Groups) > preActionMemoGroups || record.OmittedEvictionReasons != len(record.Groups) ||
		record.Groups[0].Name[:3] != "000" || record.Groups[0].EvictedOther != 7 {
		t.Fatalf("bounded group disclosure wrong: %+v", record)
	}
}

func TestPreActionMemoDoesNotDelayEmergencyExit(t *testing.T) {
	r := newGovRig(t)
	reads, emissions := 0, 0
	r.g.memoStats = func() []memo.GroupStats { reads++; return nil }
	r.g.logf = func(line string) {
		if strings.HasPrefix(line, preActionPrefix) {
			emissions++
		}
	}
	r.host.set(9*tenancy.GiB, 0)
	r.ticks(2)
	if tenancy.CurrentLevel() != tenancy.LevelExit || reads != 0 || emissions != 0 {
		t.Fatalf("exit must not read or emit memo: level=%s reads=%d emitted=%d", tenancy.CurrentLevel(), reads, emissions)
	}
}

func memoDiagnosticFixture(tb testing.TB) *memo.Registry {
	tb.Helper()
	reg := memo.NewRegistry()
	for group := range 4 {
		g := memo.NewGroupIn[int, int](reg, fmt.Sprintf("group-%d", group), memo.Options{})
		for key := range 1024 {
			if _, err := g.Get(context.Background(), key, func(context.Context) (int, int64, error) {
				return key, 64, nil
			}); err != nil {
				tb.Fatal(err)
			}
		}
	}
	return reg
}

func TestPreActionMemoRealResidentStats(t *testing.T) {
	reg := memoDiagnosticFixture(t)
	stats := reg.Stats()
	if len(stats) != 4 || stats[0].Entries != 1024 || stats[0].Bytes != 1024*64 || stats[0].Misses != 1024 || stats[0].Flights != 1024 {
		t.Fatalf("real memo summaries differ: %+v", stats)
	}
	line := preActionMemoLine(tenancy.Sample{At: time.Unix(1, 0), Footprint: 512 * tenancy.MiB}, tenancy.Budget{},
		tenancy.Transition{From: tenancy.LevelShed, To: tenancy.LevelRefuse}, stats)
	if !strings.Contains(line, `"estimated_bytes":65536`) || !strings.Contains(line, `"flights":1024`) {
		t.Fatalf("resident memo evidence lost: %s", line)
	}
}

func BenchmarkPreActionMemoRepresentative(b *testing.B) {
	reg := memoDiagnosticFixture(b)
	sample := tenancy.Sample{At: time.Unix(1, 0), Footprint: 512 * tenancy.MiB}
	tr := tenancy.Transition{From: tenancy.LevelShed, To: tenancy.LevelRefuse}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = preActionMemoLine(sample, tenancy.Budget{}, tr, reg.Stats())
	}
}
