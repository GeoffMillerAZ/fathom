package refresh

import (
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TestTenancyGate reads the process's governor state: shed pauses background
// work, refuse and store distress refuse everything, safe mode and a
// quarantine refuse background work, and an explicit ingest passes both.
func TestTenancyGate(t *testing.T) {
	now := time.Now()
	t.Cleanup(func() {
		tenancy.PublishLevel(tenancy.LevelNormal)
		tenancy.ResetLifecycleStateForTest()
		tenancy.SetQuarantined(nil)
	})
	tenancy.PublishLevel(tenancy.LevelNormal)
	tenancy.ResetLifecycleStateForTest()
	tenancy.SetQuarantined(nil)
	if v := TenancyGate("a", false, now); v != (Verdict{}) {
		t.Fatalf("normal: %+v", v)
	}
	tenancy.PublishLevel(tenancy.LevelShed)
	if v := TenancyGate("a", false, now); v.Pause == "" || v.Refuse != "" {
		t.Fatalf("shed, background: %+v; want paused", v)
	}
	if v := TenancyGate("a", true, now); v != (Verdict{}) {
		t.Fatalf("shed, explicit: %+v; want open", v)
	}
	tenancy.PublishLevel(tenancy.LevelRefuse)
	if v := TenancyGate("a", true, now); !strings.Contains(v.Refuse, "refuse") {
		t.Fatalf("refuse, explicit: %+v; want refused", v)
	}
	tenancy.PublishLevel(tenancy.LevelNormal)
	tenancy.SetPausedSafeMode(true)
	if v := TenancyGate("a", false, now); !strings.Contains(v.Refuse, "safe mode") {
		t.Fatalf("safe mode, background: %+v", v)
	}
	if v := TenancyGate("a", true, now); v != (Verdict{}) {
		t.Fatalf("safe mode, explicit: %+v; want open", v)
	}
	tenancy.SetPausedSafeMode(false)
	tenancy.SetQuarantined([]tenancy.Quarantine{{Service: "q", Since: now, Reason: tenancy.QuarantineReasonIngest}})
	if v := TenancyGate("q", false, now); !strings.Contains(v.Refuse, "quarantined") {
		t.Fatalf("quarantined, background: %+v", v)
	}
	if v := TenancyGate("q", true, now); v != (Verdict{}) {
		t.Fatalf("quarantined, explicit: %+v; want open (only it lifts one)", v)
	}
	tenancy.NoteStoreDistress("SQLITE_FULL", now)
	if v := TenancyGate("a", true, now); !strings.Contains(v.Refuse, "distress") {
		t.Fatalf("store distress: %+v", v)
	}
}
