package main

import (
	"encoding/json"
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
				d.RefuseWindows = []tenancy.RefuseWindow{{Start: "2026-09-21T04:00:00Z", End: "2026-09-21T04:19:00Z", Minutes: 20, Trigger: "disk floor: swap volume free 3.0 GiB < 5.0 GiB"}}
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
