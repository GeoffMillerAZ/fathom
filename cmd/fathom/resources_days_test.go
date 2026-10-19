package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// writeDays writes n complete day rows (09-19..) plus today's partial one.
func (f *resourcesFixture) writeDays(t *testing.T, mut func(i int, d *tenancy.DaySummary)) {
	t.Helper()
	log, err := tenancy.OpenDayLog(filepath.Join(f.dir, tenancy.DaysFileName))
	if err != nil {
		t.Fatal(err)
	}
	m := tenancy.DayMarks{Budget: 4 * tenancy.GiB, Shed: 3 * tenancy.GiB, Refuse: 4 * tenancy.GiB, Exit: 8 * tenancy.GiB, ExitEnabled: true}
	for i := 0; i < 8; i++ {
		day := time.Date(2026, 9, 19+i, 0, 0, 0, 0, time.UTC)
		for k := 0; k < 3; k++ {
			log.Observe(tenancy.RingRecord{At: day.Add(time.Duration(k) * time.Hour), Level: tenancy.LevelNormal, Footprint: 3 * tenancy.GiB, Children: -1}, m, nil)
		}
	}
	rows := log.Rows()
	for i := range rows {
		rows[i].Samples = 1440
		if mut != nil {
			mut(i, &rows[i])
		}
	}
	raw, _ := json.Marshal(map[string]any{"version": 1, "days": rows})
	if err := os.WriteFile(filepath.Join(f.dir, tenancy.DaysFileName), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func daysReport(t *testing.T, out string) (days []map[string]any, verdict map[string]any) {
	t.Helper()
	var rep struct {
		Days    []map[string]any `json:"days"`
		Verdict map[string]any   `json:"verdict"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	return rep.Days, rep.Verdict
}

// RT-185: `resources --days N` prints the day rows and RT-M1's verdict; the
// ledger supplies unclean exits and exit rungs per day at read time.
var pastShed = true

func TestResourcesDaysVerdict(t *testing.T) {
	startLedger := func(f *resourcesFixture, extra ...string) {
		rows := []string{`{"row":"start","at":"` + at(time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)) + `","pid":1}`}
		f.ledger(t, append(rows, extra...)...)
	}
	t.Run("PASS with a covered ledger", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
		f.writeDays(t, nil)
		startLedger(f)
		code, out := f.run(t, "--days", "8", "--json")
		days, v := daysReport(t, out)
		if code != 0 || len(days) != 8 || v["state"] != "PASS" {
			t.Fatalf("exit %d, %d days, verdict %v\n%s", code, len(days), v, out)
		}
		if days[0]["date"] != "2026-09-19" || days[7]["date"] != "2026-09-26" {
			t.Errorf("days not oldest first: %v .. %v", days[0]["date"], days[7]["date"])
		}
	})
	t.Run("an unclean exit from the ledger fails the day", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
		f.writeDays(t, nil)
		startLedger(f, `{"row":"exit","at":"`+at(time.Date(2026, 9, 22, 3, 0, 0, 0, time.UTC))+`","pid":1,"reason":"unclean"}`,
			`{"row":"exit","at":"`+at(time.Date(2026, 9, 23, 3, 0, 0, 0, time.UTC))+`","pid":2,"reason":"over_budget","code":70}`)
		_, out := f.run(t, "--days", "7", "--json")
		days, v := daysReport(t, out)
		if v["state"] != "FAIL" || !strings.Contains(v["detail"].(string), "2026-09-22") {
			t.Fatalf("verdict %v", v)
		}
		if len(days) != 7 || days[2]["unclean_exits"].(float64) != 1 || days[3]["exit_rungs"].(float64) != 1 {
			t.Errorf("ledger columns not merged: %v", days)
		}
	})
	t.Run("a ledger that begins after a day leaves it INCOMPLETE", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
		f.writeDays(t, nil)
		f.ledger(t, `{"row":"start","at":"`+at(time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC))+`","pid":1}`)
		_, out := f.run(t, "--days", "8", "--json")
		_, v := daysReport(t, out)
		if v["state"] != "INCOMPLETE" || !strings.Contains(v["detail"].(string), "2026-09-19") {
			t.Fatalf("verdict %v", v)
		}
	})
	t.Run("a rotated ledger whose kept generation begins mid-day leaves that day INCOMPLETE", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
		f.writeDays(t, nil)
		// The generation before .1 was dropped at rotation: exits on 09-19
		// before 08:00 are gone, so 09-19's exits are not known.
		old := `{"row":"start","at":"` + at(time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC)) + `","pid":1}` + "\n"
		if err := os.WriteFile(filepath.Join(f.dir, "lifecycle.jsonl.1"), []byte(old), 0o600); err != nil {
			t.Fatal(err)
		}
		f.ledger(t, `{"row":"start","at":"`+at(time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC))+`","pid":2}`)
		_, out := f.run(t, "--days", "8", "--json")
		_, v := daysReport(t, out)
		if v["state"] != "INCOMPLETE" || !strings.Contains(v["detail"].(string), "2026-09-19") {
			t.Fatalf("verdict %v", v)
		}
	})
	t.Run("an unreadable current ledger leaves exits unknown, never PASS (I38)", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
		f.writeDays(t, nil)
		old := `{"row":"start","at":"` + at(time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)) + `","pid":1}` + "\n"
		if err := os.WriteFile(filepath.Join(f.dir, "lifecycle.jsonl.1"), []byte(old), 0o600); err != nil {
			t.Fatal(err)
		}
		// A directory where the current generation belongs: open succeeds,
		// the read fails.
		if err := os.Mkdir(filepath.Join(f.dir, "lifecycle.jsonl"), 0o700); err != nil {
			t.Fatal(err)
		}
		_, out := f.run(t, "--days", "8", "--json")
		_, v := daysReport(t, out)
		if v["state"] != "INCOMPLETE" {
			t.Fatalf("verdict %v", v)
		}
	})
	t.Run("no day log is INCOMPLETE naming every day, not an error", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
		code, out := f.run(t, "--days", "8", "--json")
		_, v := daysReport(t, out)
		if code != 0 || v["state"] != "INCOMPLETE" || !strings.Contains(v["detail"].(string), "2026-09-19") {
			t.Fatalf("exit %d verdict %v", code, v)
		}
	})
	t.Run("text table and one verdict line", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
		f.writeDays(t, func(i int, d *tenancy.DaySummary) {
			if i == 2 {
				d.RefuseMinutes = 20
				d.RefuseWindows = []tenancy.RefuseWindow{{Start: "2026-09-21T04:00:00Z", End: "2026-09-21T04:19:00Z", Minutes: 20, Trigger: "disk floor: swap volume free 3.0 GiB < 5.0 GiB", PastShed: &pastShed}}
			}
		})
		startLedger(f)
		_, out := f.run(t, "--days", "8")
		for _, want := range []string{"2026-09-26", "2026-09-19", "disk floor: swap volume", "RT-M1 verdict  FAIL", "2026-09-21"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
		if strings.Count(out, "RT-M1 verdict") != 1 {
			t.Errorf("want one verdict line:\n%s", out)
		}
	})
	t.Run("--days outside 1..8 is exit 2", func(t *testing.T) {
		f := newResourcesFixture(t)
		for _, a := range []string{"0", "9", "-1"} {
			code, out := f.run(t, "--days", a)
			if code != 2 || !strings.Contains(out, "--days must be 1..8") {
				t.Errorf("--days %s: exit %d: %s", a, code, out)
			}
		}
	})
	t.Run("without --days the JSON has no days or verdict (additive)", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
		f.writeDays(t, nil)
		_, out := f.run(t, "--json")
		if strings.Contains(out, `"days"`) || strings.Contains(out, `"verdict"`) {
			t.Errorf("days leaked into the default report:\n%s", out)
		}
	})
}

// RT-187: --days shows the self-caused and host-starved refuse minutes per
// day in the table and in --json, and the verdict reads the amended clause.
func TestResourcesDaysSplit(t *testing.T) {
	setup := func(t *testing.T, ws ...tenancy.RefuseWindow) *resourcesFixture {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
		f.writeDays(t, func(i int, d *tenancy.DaySummary) {
			if i == 2 {
				d.RefuseWindows = ws
				for _, w := range ws {
					d.RefuseMinutes += w.Minutes
				}
			}
		})
		f.ledger(t, `{"row":"start","at":"`+at(time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC))+`","pid":1}`)
		return f
	}
	floor := tenancy.RefuseWindow{Start: "2026-09-21T01:00:00Z", End: "2026-09-21T01:26:00Z", Minutes: 27, Trigger: "disk floor: swap volume free 3.0 GiB < 5.0 GiB"}
	t.Run("the 10-09 shape passes and shows 27 host-starved minutes", func(t *testing.T) {
		f := setup(t, floor)
		code, out := f.run(t, "--days", "8", "--json")
		days, v := daysReport(t, out)
		if code != 0 || v["state"] != "PASS" || days[2]["host_starved_minutes"].(float64) != 27 || days[2]["self_refuse_minutes"].(float64) != 0 {
			t.Fatalf("exit %d verdict %v day %v", code, v, days[2])
		}
		_, table := f.run(t, "--days", "8")
		if !strings.Contains(table, "self") || !strings.Contains(table, "host-starved") {
			t.Errorf("table lacks the split columns:\n%s", table)
		}
	})
	t.Run("20 self-caused minutes fail", func(t *testing.T) {
		w := tenancy.RefuseWindow{Start: "2026-09-21T01:00:00Z", End: "2026-09-21T01:19:00Z", Minutes: 20, Trigger: "memory: footprint 4.1 GiB >= refuse mark"}
		_, out := setup(t, w).run(t, "--days", "8", "--json")
		_, v := daysReport(t, out)
		if v["state"] != "FAIL" || !strings.Contains(v["detail"].(string), "2026-09-21") {
			t.Fatalf("verdict %v", v)
		}
	})
	t.Run("61 host-starved minutes fail on availability", func(t *testing.T) {
		w := floor
		w.Minutes = 61
		_, out := setup(t, w).run(t, "--days", "8", "--json")
		_, v := daysReport(t, out)
		if v["state"] != "FAIL" || !strings.Contains(v["criterion"].(string), "host-starved") {
			t.Fatalf("verdict %v", v)
		}
	})
}

// RT-189 (§2.1, §13.1): --days prints RT-M13's verdict over the last seven
// complete UTC days (shed + refuse minutes at most 201.6, 2% of the week,
// and zero host-trigger rung entries at normal pressure). Each day row
// carries its rung entries by trigger with the pressure at entry, rolled on
// the sampler tick; a day logged before the field reads INCOMPLETE, named;
// --json adds rtm13; RT-M1's verdict is unchanged.
func TestResourcesDaysPrintsRTM13Verdict(t *testing.T) {
	m := tenancy.DayMarks{Budget: 4 * tenancy.GiB, Shed: 3 * tenancy.GiB, Refuse: 4 * tenancy.GiB, Exit: 8 * tenancy.GiB, ExitEnabled: true}
	// rise rolls one rung entry the way the governor does: the transition
	// record (carrying the sample's pressure) with the ladder's transition.
	rise := func(log *tenancy.DayLog, at time.Time, to tenancy.Level, kind tenancy.CauseKind, trig tenancy.RungTrigger, p tenancy.PressureLevel) {
		rec := tenancy.RingRecord{At: at, Transition: true, Level: to, Footprint: 70 * tenancy.MiB, Children: -1, Pressure: p}
		tr := tenancy.Transition{From: tenancy.LevelNormal, To: to, At: at, Changed: true, Cause: tenancy.Cause{Kind: kind, Detail: "fixture", Trigger: trig}}
		log.Observe(rec, m, &tr)
	}
	setup := func(t *testing.T, perDay func(i int, day time.Time, log *tenancy.DayLog), mut func(i int, d *tenancy.DaySummary)) *resourcesFixture {
		t.Helper()
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
		log, err := tenancy.OpenDayLog(filepath.Join(f.dir, tenancy.DaysFileName))
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 8; i++ {
			day := time.Date(2026, 9, 19+i, 0, 0, 0, 0, time.UTC)
			for k := 0; k < 3; k++ {
				log.Observe(tenancy.RingRecord{At: day.Add(time.Duration(k) * time.Hour), Level: tenancy.LevelNormal, Footprint: 70 * tenancy.MiB, Children: -1, Pressure: tenancy.PressureNormal}, m, nil)
			}
			if perDay != nil {
				perDay(i, day, log)
			}
		}
		rows := log.Rows()
		for i := range rows {
			rows[i].Samples = 1440
			if mut != nil {
				mut(i, &rows[i])
			}
		}
		raw, _ := json.Marshal(map[string]any{"version": 1, "days": rows})
		if err := os.WriteFile(filepath.Join(f.dir, tenancy.DaysFileName), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		f.ledger(t, `{"row":"start","at":"`+at(time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC))+`","pid":1}`)
		return f
	}
	report := func(t *testing.T, out string) (days []map[string]any, rtm1, rtm13 map[string]any) {
		t.Helper()
		var rep struct {
			Days    []map[string]any `json:"days"`
			Verdict map[string]any   `json:"verdict"`
			RTM13   map[string]any   `json:"rtm13"`
		}
		if err := json.Unmarshal([]byte(out), &rep); err != nil {
			t.Fatalf("not JSON: %v\n%s", err, out)
		}
		return rep.Days, rep.Verdict, rep.RTM13
	}
	shedOnWarn := func(i int, day time.Time, log *tenancy.DayLog) {
		rise(log, day.Add(4*time.Hour), tenancy.LevelShed, tenancy.CauseHost, tenancy.TriggerPressure, tenancy.PressureWarn)
		rise(log, day.Add(5*time.Hour), tenancy.LevelShed, tenancy.CauseMark, tenancy.TriggerOwnMark, tenancy.PressureNormal)
	}
	minutes := func(n int) func(int, *tenancy.DaySummary) {
		return func(_ int, d *tenancy.DaySummary) { d.ShedMinutes = n }
	}

	t.Run("PASS at 196 minutes, host entries only at warn", func(t *testing.T) {
		f := setup(t, shedOnWarn, minutes(28))
		code, out := f.run(t, "--days", "8", "--json")
		days, rtm1, v := report(t, out)
		if code != 0 || v["state"] != "PASS" || v["shed_refuse_minutes"].(float64) != 196 {
			t.Fatalf("exit %d rtm13 %v", code, v)
		}
		if rtm1["state"] != "PASS" || rtm1["detail"] != "seven complete UTC days, every criterion held" {
			t.Errorf("RT-M1's verdict moved: %v", rtm1)
		}
		entries, _ := days[2]["rung_entries"].([]any)
		var got []string
		for _, e := range entries {
			em := e.(map[string]any)
			got = append(got, fmt.Sprintf("%v/%v/%v/%v", em["rung"], em["trigger"], em["pressure"], em["count"]))
		}
		if strings.Join(got, " ") != "shed/own_mark/normal/1 shed/pressure/warn/1" || days[2]["rung_entries_known"] != true {
			t.Fatalf("day row rung entries %v known=%v", got, days[2]["rung_entries_known"])
		}
		_, table := f.run(t, "--days", "8")
		if !strings.Contains(table, "RT-M1 verdict  PASS: seven complete UTC days, every criterion held\n") {
			t.Errorf("RT-M1's line moved:\n%s", table)
		}
		if !strings.Contains(table, "RT-M13 verdict PASS:") || !strings.Contains(table, "shed by pressure at warn ×1") {
			t.Errorf("table lacks the RT-M13 line or the entries:\n%s", table)
		}
	})
	t.Run("210 shed minutes FAIL on the day the week crosses 201.6", func(t *testing.T) {
		_, out := setup(t, shedOnWarn, minutes(30)).run(t, "--days", "8", "--json")
		_, _, v := report(t, out)
		if v["state"] != "FAIL" || !strings.Contains(v["criterion"].(string), "201.6") || v["day"] != "2026-09-25" {
			t.Fatalf("rtm13 %v", v)
		}
	})
	t.Run("a host-trigger entry at normal pressure FAILs, naming the day", func(t *testing.T) {
		floor := func(i int, day time.Time, log *tenancy.DayLog) {
			shedOnWarn(i, day, log)
			if i == 2 {
				rise(log, day.Add(6*time.Hour), tenancy.LevelRefuse, tenancy.CauseHost, tenancy.TriggerSwapVolume, tenancy.PressureNormal)
			}
		}
		_, out := setup(t, floor, minutes(10)).run(t, "--days", "8", "--json")
		days, _, v := report(t, out)
		if v["state"] != "FAIL" || v["day"] != "2026-09-21" || !strings.Contains(v["criterion"].(string), "normal pressure") {
			t.Fatalf("rtm13 %v", v)
		}
		if !strings.Contains(fmt.Sprint(days[2]["rung_entries"]), "swap_volume_floor") {
			t.Fatalf("the floor entry is not on its day row: %v", days[2]["rung_entries"])
		}
	})
	t.Run("days logged before the field read INCOMPLETE, named", func(t *testing.T) {
		old := func(i int, d *tenancy.DaySummary) {
			d.ShedMinutes = 10
			if i < 2 {
				d.RungEntriesKnown = false
			}
		}
		f := setup(t, nil, old)
		code, out := f.run(t, "--days", "8", "--json")
		_, rtm1, v := report(t, out)
		detail, _ := v["detail"].(string)
		if v["state"] != "INCOMPLETE" || !strings.Contains(detail, "2026-09-19") || !strings.Contains(detail, "2026-09-20") || strings.Contains(detail, "2026-09-21") {
			t.Fatalf("rtm13 %v", v)
		}
		if code != 0 || rtm1["state"] != "PASS" {
			t.Errorf("RT-M1 reads the same rows as before: exit %d %v", code, rtm1)
		}
	})
	t.Run("JudgeRTM1 is blind to the new fields", func(t *testing.T) {
		now := time.Date(2026, 9, 26, 22, 0, 0, 0, time.UTC)
		var with, without []tenancy.DaySummary
		for i := 0; i < 8; i++ {
			d := tenancy.DaySummary{Date: time.Date(2026, 9, 19+i, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), Samples: 1440, ShedMinutes: 40, ExitsKnown: true}
			without = append(without, d)
			d.RungEntriesKnown = true
			d.RungEntries = []tenancy.RungEntry{{Rung: "refuse", Trigger: "swap_volume_floor", Pressure: "normal", Count: 3}}
			with = append(with, d)
		}
		a, _ := json.Marshal(tenancy.JudgeRTM1(with, now))
		b, _ := json.Marshal(tenancy.JudgeRTM1(without, now))
		if string(a) != string(b) {
			t.Fatalf("RT-M1 reads the rung entries:\n%s\n%s", a, b)
		}
	})
}
