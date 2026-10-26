//go:build harness_maintenance && (darwin || linux)

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func storeRoot(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "state")
	if e := os.Mkdir(p, 0700); e != nil {
		t.Fatal(e)
	}
	return p
}
func openStoreTest(t *testing.T, path string, fault func(string) error) *maintenanceFileStore {
	t.Helper()
	s, e := openMaintenanceFileStore(path, fault)
	if e != nil {
		t.Fatalf("open: %s", maintenanceStoreCode(e))
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func storeBinding(t *testing.T, path string, state maintenanceState) {
	t.Helper()
	raw, e := json.Marshal(maintenanceBindingFor(state, path))
	if e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(path, "BINDING"), raw, 0600); e != nil {
		t.Fatal(e)
	}
}
func storeArmed(t *testing.T) maintenanceState {
	t.Helper()
	ctx, _ := engineContext(t)
	return *engineInit(t, ctx)
}
func TestMaintenanceStoreInitCommitLoad(t *testing.T) {
	path := storeRoot(t)
	s := openStoreTest(t, path, nil)
	if got, e := s.Load(); e != nil || got != nil {
		t.Fatalf("virgin %v %v", got, e)
	}
	armed := storeArmed(t)
	if e := s.Commit(armed, "init"); e != nil {
		t.Fatal(e)
	}
	got, e := s.Load()
	if e != nil || got == nil || got.InstanceID != armed.InstanceID || got.Sequence != 1 {
		t.Fatalf("load %v %v", got, e)
	}
	if e := s.Commit(armed, "init"); maintenanceStoreCode(e) != "state_exists" {
		t.Fatalf("repeat init %v", e)
	}
	next := *got
	next.Sequence++
	next.Stamp.UTCMicros++
	if e := s.Commit(next, "check"); e != nil {
		t.Fatal(e)
	}
	again, e := s.Load()
	if e != nil || again.Sequence != 2 {
		t.Fatalf("second load %v %v", again, e)
	}
	if e := s.Close(); e != nil {
		t.Fatal(e)
	}
	s2 := openStoreTest(t, path, nil)
	persisted, e := s2.Load()
	if e != nil || persisted.Sequence != 2 {
		t.Fatalf("restart %v %v", persisted, e)
	}
}
func TestMaintenanceStoreConcurrentLockAndIdentity(t *testing.T) {
	path := storeRoot(t)
	s := openStoreTest(t, path, nil)
	if other, e := openMaintenanceFileStore(path, nil); other != nil || maintenanceStoreCode(e) != "state_busy" {
		t.Fatalf("second lock %v %v", other, e)
	}
	if e := s.Close(); e != nil {
		t.Fatal(e)
	}
	next := openStoreTest(t, path, nil)
	if e := next.Close(); e != nil {
		t.Fatal(e)
	}
	linked := filepath.Join(t.TempDir(), "alias")
	if e := os.Symlink(path, linked); e != nil {
		t.Fatal(e)
	}
	if _, e := openMaintenanceFileStore(linked, nil); maintenanceStoreCode(e) != "state_invalid" {
		t.Fatalf("symlink root %v", e)
	}
}
func TestMaintenanceStoreVirginAndPartialHold(t *testing.T) {
	for _, leaf := range []string{"BINDING", "BINDING.tmp", "CURRENT", "PENDING", "state-a.json", "state-b.json", "CURRENT.tmp", "PENDING.tmp", "state-a.json.tmp", "state-b.json.tmp", "recovery.json"} {
		t.Run(leaf, func(t *testing.T) {
			path := storeRoot(t)
			if e := os.WriteFile(filepath.Join(path, leaf), []byte("partial"), 0600); e != nil {
				t.Fatal(e)
			}
			s := openStoreTest(t, path, nil)
			if _, e := s.Load(); e == nil {
				t.Fatal("partial load accepted")
			}
			if e := s.Commit(storeArmed(t), "init"); e == nil {
				t.Fatal("partial init accepted")
			}
		})
	}
}
func TestMaintenanceStoreSymlinkLeafHold(t *testing.T) {
	path := storeRoot(t)
	if e := os.WriteFile(filepath.Join(path, "target"), []byte("x"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink("target", filepath.Join(path, "CURRENT")); e != nil {
		t.Fatal(e)
	}
	s := openStoreTest(t, path, nil)
	if _, e := s.Load(); maintenanceStoreCode(e) != "state_ambiguous" {
		t.Fatalf("symlink accepted %v", e)
	}
	if e := s.Commit(storeArmed(t), "init"); e == nil {
		t.Fatal("symlink init accepted")
	}
}
func TestMaintenanceStoreRecoverLockedOnly(t *testing.T) {
	path := storeRoot(t)
	if e := os.WriteFile(filepath.Join(path, "PENDING"), []byte("bad intent"), 0600); e != nil {
		t.Fatal(e)
	}
	s := openStoreTest(t, path, nil)
	if _, e := s.Load(); maintenanceStoreCode(e) != "state_ambiguous" {
		t.Fatalf("pending accepted %v", e)
	}
	armed := storeArmed(t)
	storeBinding(t, path, armed)
	if e := s.Recover(armed); maintenanceStoreCode(e) != "state_invalid" {
		t.Fatalf("armed recover %v", e)
	}
	locked := armed
	locked.Phase = "locked"
	locked.InstanceID = "1111111111111111111111111111111111111111111111111111111111111111"
	if e := s.Recover(locked); e != nil {
		t.Fatal(e)
	}
	got, e := s.Load()
	if e != nil || got.Phase != "locked" || got.InstanceID != locked.InstanceID {
		t.Fatalf("recovery %v %v", got, e)
	}
	if e := s.Recover(locked); maintenanceStoreCode(e) != "state_invalid" {
		t.Fatalf("second recovery %v", e)
	}
	raw, e := os.ReadFile(filepath.Join(path, "recovery.json"))
	if e != nil || !bytes.Contains(raw, []byte("YmFkIGludGVudA==")) {
		t.Fatal("missing bounded evidence")
	}
}
func TestMaintenanceStoreFaultStages(t *testing.T) {
	for _, stage := range []string{"before:PENDING.tmp:write", "after:PENDING.tmp:write", "before:PENDING.tmp:sync", "after:PENDING.tmp:sync", "before:PENDING:rename", "after:PENDING:rename", "before:DIR:sync", "before:state-a.json.tmp:write", "after:state-a.json:rename", "before:CURRENT.tmp:write", "after:CURRENT:rename", "before:PENDING:remove", "after:PENDING:remove"} {
		t.Run(stage, func(t *testing.T) {
			path := storeRoot(t)
			count := 0
			enabled := false
			s := openStoreTest(t, path, func(at string) error {
				if enabled && at == stage && count == 0 {
					count++
					return errors.New("fault")
				}
				return nil
			})
			if _, e := s.Load(); e != nil {
				t.Fatal(e)
			}
			enabled = true
			if e := s.Commit(storeArmed(t), "init"); maintenanceStoreCode(e) != "storage_incomplete" {
				t.Fatalf("stage %s result %v", stage, e)
			}
			if count != 1 {
				t.Fatal("fault not reached")
			}
			if _, e := s.Load(); maintenanceStoreCode(e) != "state_ambiguous" {
				t.Fatalf("uncertain load %v", e)
			}
			_ = s.Close()
			reopened := openStoreTest(t, path, nil)
			got, e := reopened.Load()
			if e == nil && got != nil && got.Phase != "armed" {
				t.Fatal("fault widened authority")
			}
		})
	}
}

func TestMaintenanceStoreCorruptSlotHold(t *testing.T) {
	for _, leaf := range []string{"state-a.json", "state-b.json"} {
		t.Run(leaf, func(t *testing.T) {
			path := storeRoot(t)
			s := openStoreTest(t, path, nil)
			if _, e := s.Load(); e != nil {
				t.Fatal(e)
			}
			armed := storeArmed(t)
			if e := s.Commit(armed, "init"); e != nil {
				t.Fatal(e)
			}
			loaded, e := s.Load()
			if e != nil {
				t.Fatal(e)
			}
			next := *loaded
			next.Sequence++
			next.Stamp.UTCMicros++
			if e := s.Commit(next, "check"); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(path, leaf), []byte("corrupt"), 0600); e != nil {
				t.Fatal(e)
			}
			if got, e := s.Load(); got != nil || maintenanceStoreCode(e) != "state_ambiguous" {
				t.Fatalf("corrupt slot accepted: %v %v", got, e)
			}
		})
	}
}
func TestMaintenanceStoreRecoverRejectsOldInstance(t *testing.T) {
	path := storeRoot(t)
	s := openStoreTest(t, path, nil)
	if _, e := s.Load(); e != nil {
		t.Fatal(e)
	}
	armed := storeArmed(t)
	if e := s.Commit(armed, "init"); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(path, "PENDING"), []byte("bad"), 0600); e != nil {
		t.Fatal(e)
	}
	locked := armed
	locked.Phase = "locked"
	if e := s.Recover(locked); maintenanceStoreCode(e) != "state_invalid" {
		t.Fatalf("old instance recovery accepted %v", e)
	}
}

func TestMaintenanceStoreNonzeroLockRefused(t *testing.T) {
	path := storeRoot(t)
	if e := os.WriteFile(filepath.Join(path, "LOCK"), []byte("pid"), 0600); e != nil {
		t.Fatal(e)
	}
	if s, e := openMaintenanceFileStore(path, nil); s != nil || maintenanceStoreCode(e) != "state_ambiguous" {
		t.Fatalf("nonzero lock accepted %v %v", s, e)
	}
}

func TestMaintenanceStoreRootReplacementHold(t *testing.T) {
	path := storeRoot(t)
	s := openStoreTest(t, path, nil)
	moved := path + "-moved"
	if e := os.Rename(path, moved); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(path, 0700); e != nil {
		t.Fatal(e)
	}
	if got, e := s.Load(); got != nil || maintenanceStoreCode(e) != "state_ambiguous" {
		t.Fatalf("replaced root accepted %v %v", got, e)
	}
}

func TestMaintenanceStoreInterruptedRecoveryResumes(t *testing.T) {
	for _, stage := range []string{"after:recovery.json:rename", "after:PENDING:remove", "after:state-a.json:rename", "after:CURRENT:rename"} {
		t.Run(stage, func(t *testing.T) {
			path := storeRoot(t)
			if e := os.WriteFile(filepath.Join(path, "PENDING"), []byte("bad"), 0600); e != nil {
				t.Fatal(e)
			}
			active := false
			count := 0
			s := openStoreTest(t, path, func(at string) error {
				if active && at == stage && count == 0 {
					count++
					return errors.New("fault")
				}
				return nil
			})
			first := storeArmed(t)
			storeBinding(t, path, first)
			first.Phase = "locked"
			first.InstanceID = "1111111111111111111111111111111111111111111111111111111111111111"
			active = true
			if e := s.Recover(first); maintenanceStoreCode(e) != "storage_incomplete" || count != 1 {
				t.Fatalf("fault %s: %v count %d", stage, e, count)
			}
			_ = s.Close()
			restart := openStoreTest(t, path, nil)
			got, e := restart.Load()
			if e == nil && got != nil {
				if got.Phase != "locked" || got.InstanceID != first.InstanceID {
					t.Fatal(got)
				}
				return
			}
			if maintenanceStoreCode(e) != "state_ambiguous" {
				t.Fatalf("unexpected load %v", e)
			}
			second := first
			second.InstanceID = "2222222222222222222222222222222222222222222222222222222222222222"
			if e := restart.Recover(second); e != nil {
				t.Fatalf("resume %s: %v", stage, e)
			}
			got, e = restart.Load()
			if e != nil || got == nil || got.Phase != "locked" || got.InstanceID != first.InstanceID {
				t.Fatalf("resumed state %v %v", got, e)
			}
		})
	}
}

func TestMaintenanceStoreRecoveryEvidenceRejectsNewAmbiguity(t *testing.T) {
	path := storeRoot(t)
	if e := os.WriteFile(filepath.Join(path, "PENDING"), []byte("old"), 0600); e != nil {
		t.Fatal(e)
	}
	s := openStoreTest(t, path, nil)
	first := storeArmed(t)
	storeBinding(t, path, first)
	first.Phase = "locked"
	first.InstanceID = "1111111111111111111111111111111111111111111111111111111111111111"
	if e := s.Recover(first); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(path, "PENDING"), []byte("new"), 0600); e != nil {
		t.Fatal(e)
	}
	second := first
	second.InstanceID = "2222222222222222222222222222222222222222222222222222222222222222"
	if e := s.Recover(second); maintenanceStoreCode(e) != "state_ambiguous" {
		t.Fatalf("new ambiguity reused old evidence: %v", e)
	}
}

func TestMaintenanceStoreRecoveryTempResumes(t *testing.T) {
	for _, stage := range []string{"before:recovery.json.tmp:write", "after:recovery.json.tmp:write", "before:recovery.json.tmp:sync", "after:recovery.json.tmp:sync", "before:recovery.json:rename"} {
		t.Run(stage, func(t *testing.T) {
			path := storeRoot(t)
			if e := os.WriteFile(filepath.Join(path, "PENDING"), []byte("old"), 0600); e != nil {
				t.Fatal(e)
			}
			enabled := false
			count := 0
			s := openStoreTest(t, path, func(at string) error {
				if enabled && at == stage && count == 0 {
					count++
					return errors.New("fault")
				}
				return nil
			})
			first := storeArmed(t)
			storeBinding(t, path, first)
			first.Phase = "locked"
			first.InstanceID = "1111111111111111111111111111111111111111111111111111111111111111"
			enabled = true
			if e := s.Recover(first); maintenanceStoreCode(e) != "storage_incomplete" || count != 1 {
				t.Fatalf("fault %v count %d", e, count)
			}
			_ = s.Close()
			restart := openStoreTest(t, path, nil)
			second := first
			second.InstanceID = "2222222222222222222222222222222222222222222222222222222222222222"
			if e := restart.Recover(second); e != nil {
				t.Fatalf("retry %s: %v", stage, e)
			}
			got, e := restart.Load()
			if e != nil || got == nil || got.Phase != "locked" {
				t.Fatalf("state %v %v", got, e)
			}
		})
	}
}

func TestMaintenanceStoreZeroLengthTempEvidenceResumes(t *testing.T) {
	path := storeRoot(t)
	if e := os.WriteFile(filepath.Join(path, "CURRENT.tmp"), nil, 0600); e != nil {
		t.Fatal(e)
	}
	trigger := true
	s := openStoreTest(t, path, func(at string) error {
		if trigger && at == "after:recovery.json:rename" {
			trigger = false
			return errors.New("fault")
		}
		return nil
	})
	first := storeArmed(t)
	storeBinding(t, path, first)
	first.Phase = "locked"
	first.InstanceID = "1111111111111111111111111111111111111111111111111111111111111111"
	if e := s.Recover(first); maintenanceStoreCode(e) != "storage_incomplete" {
		t.Fatal(e)
	}
	_ = s.Close()
	restart := openStoreTest(t, path, nil)
	second := first
	second.InstanceID = "2222222222222222222222222222222222222222222222222222222222222222"
	if e := restart.Recover(second); e != nil {
		t.Fatal(e)
	}
	got, e := restart.Load()
	if e != nil || got == nil || got.Phase != "locked" {
		t.Fatal(got, e)
	}
}

func TestMaintenanceStoreImmutableBinding(t *testing.T) {
	path := storeRoot(t)
	s := openStoreTest(t, path, nil)
	if _, e := s.Load(); e != nil {
		t.Fatal(e)
	}
	armed := storeArmed(t)
	if e := s.Commit(armed, "init"); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(filepath.Join(path, "BINDING"))
	if e != nil || len(raw) > 4096 {
		t.Fatal(e)
	}
	var binding maintenanceReceiverBinding
	if strictJSON(raw, &binding) != nil || !maintenanceBindingMatches(binding, armed, path) {
		t.Fatal("bad binding")
	}
	if e := os.WriteFile(filepath.Join(path, "PENDING"), []byte("bad"), 0600); e != nil {
		t.Fatal(e)
	}
	foreign := armed
	foreign.Phase = "locked"
	foreign.InstanceID = "2222222222222222222222222222222222222222222222222222222222222222"
	foreign.ControllerSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if e := s.Recover(foreign); maintenanceStoreCode(e) != "binding_mismatch" {
		t.Fatalf("foreign recovery %v", e)
	}
	same := foreign
	same.ControllerSHA256 = armed.ControllerSHA256
	if e := s.Recover(same); e != nil {
		t.Fatalf("original recovery %v", e)
	}
	after, e := os.ReadFile(filepath.Join(path, "BINDING"))
	if e != nil || !bytes.Equal(raw, after) {
		t.Fatal("binding overwritten")
	}
}
func TestMaintenanceStoreMissingOrCorruptBindingHolds(t *testing.T) {
	for _, mode := range []string{"missing", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			path := storeRoot(t)
			s := openStoreTest(t, path, nil)
			if _, e := s.Load(); e != nil {
				t.Fatal(e)
			}
			armed := storeArmed(t)
			if e := s.Commit(armed, "init"); e != nil {
				t.Fatal(e)
			}
			if mode == "missing" {
				if e := os.Remove(filepath.Join(path, "BINDING")); e != nil {
					t.Fatal(e)
				}
			} else {
				if e := os.WriteFile(filepath.Join(path, "BINDING"), []byte("corrupt"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			if got, e := s.Load(); got != nil || maintenanceStoreCode(e) != "state_ambiguous" {
				t.Fatal(got, e)
			}
			locked := armed
			locked.Phase = "locked"
			locked.InstanceID = "2222222222222222222222222222222222222222222222222222222222222222"
			if e := s.Recover(locked); maintenanceStoreCode(e) != "state_ambiguous" {
				t.Fatalf("rebound without binding %v", e)
			}
		})
	}
}
func TestMaintenanceStoreBindingTempPromotion(t *testing.T) {
	for _, complete := range []bool{true, false} {
		t.Run(strconv.FormatBool(complete), func(t *testing.T) {
			path := storeRoot(t)
			locked := storeArmed(t)
			locked.Phase = "locked"
			raw, _ := json.Marshal(maintenanceBindingFor(locked, path))
			if !complete {
				raw = []byte("partial")
			}
			if e := os.WriteFile(filepath.Join(path, "BINDING.tmp"), raw, 0600); e != nil {
				t.Fatal(e)
			}
			s := openStoreTest(t, path, nil)
			if got, e := s.Load(); got != nil || maintenanceStoreCode(e) != "state_ambiguous" {
				t.Fatal(got, e)
			}
			e := s.Recover(locked)
			if complete {
				if e != nil {
					t.Fatal(e)
				}
				got, e := s.Load()
				if e != nil || got.Phase != "locked" {
					t.Fatal(got, e)
				}
			} else if e == nil {
				t.Fatal("partial binding promoted")
			}
		})
	}
}
func TestMaintenanceStoreBindingBeforeStateFault(t *testing.T) {
	path := storeRoot(t)
	trigger := true
	s := openStoreTest(t, path, func(at string) error {
		if trigger && at == "after:BINDING:rename" {
			trigger = false
			return errors.New("fault")
		}
		return nil
	})
	if _, e := s.Load(); e != nil {
		t.Fatal(e)
	}
	armed := storeArmed(t)
	if e := s.Commit(armed, "init"); maintenanceStoreCode(e) != "storage_incomplete" {
		t.Fatal(e)
	}
	_ = s.Close()
	restart := openStoreTest(t, path, nil)
	if got, e := restart.Load(); got != nil || maintenanceStoreCode(e) != "state_ambiguous" {
		t.Fatal(got, e)
	}
	locked := armed
	locked.Phase = "locked"
	locked.InstanceID = "2222222222222222222222222222222222222222222222222222222222222222"
	if e := restart.Recover(locked); e != nil {
		t.Fatal(e)
	}
}

func TestMaintenanceReceiverBindingWire(t *testing.T) {
	path := storeRoot(t)
	armed := storeArmed(t)
	binding := maintenanceBindingFor(armed, path)
	raw, _ := json.Marshal(binding)
	if len(raw) > 4096 || strictJSON(raw, new(maintenanceReceiverBinding)) != nil {
		t.Fatal("valid binding rejected")
	}
	var fields map[string]any
	if e := json.Unmarshal(raw, &fields); e != nil {
		t.Fatal(e)
	}
	for field, value := range fields {
		delete(fields, field)
		broken, _ := json.Marshal(fields)
		if strictJSON(broken, new(maintenanceReceiverBinding)) == nil {
			t.Errorf("missing %s accepted", field)
		}
		fields[field] = value
	}
	cue, e := exec.LookPath("cue")
	if e != nil {
		t.Skip("cue unavailable")
	}
	file := filepath.Join(t.TempDir(), "binding.json")
	if e := os.WriteFile(file, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if out, e := exec.Command(cue, "vet", "-c", "schema.cue", "maintenance_schema.cue", "maintenance_store_schema.cue", file, "-d", "#MaintenanceReceiverBinding").CombinedOutput(); e != nil {
		t.Fatalf("cue: %v %s", e, out)
	}
}

func TestMaintenanceStoreForeignLowerSequenceSlotHolds(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*maintenanceState)
	}{{"instance", func(s *maintenanceState) {
		s.InstanceID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	}}, {"controller", func(s *maintenanceState) {
		s.ControllerSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	}}, {"registration", func(s *maintenanceState) {
		s.RegistrationSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	}}, {"project", func(s *maintenanceState) { s.ProjectID = "foreign" }}} {
		t.Run(tc.name, func(t *testing.T) {
			path := storeRoot(t)
			store := openStoreTest(t, path, nil)
			if _, e := store.Load(); e != nil {
				t.Fatal(e)
			}
			armed := storeArmed(t)
			if e := store.Commit(armed, "init"); e != nil {
				t.Fatal(e)
			}
			first, e := store.Load()
			if e != nil {
				t.Fatal(e)
			}
			next := *first
			next.Sequence++
			next.Stamp.UTCMicros++
			if e := store.Commit(next, "check"); e != nil {
				t.Fatal(e)
			}
			if _, e := store.Load(); e != nil {
				t.Fatal(e)
			}
			stale := armed
			tc.change(&stale)
			raw, e := json.Marshal(stale)
			if e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(path, "state-a.json"), raw, 0600); e != nil {
				t.Fatal(e)
			}
			if got, e := store.Load(); got != nil || maintenanceStoreCode(e) != "state_ambiguous" {
				t.Fatalf("foreign stale slot accepted: %v %v", got, e)
			}
			third := next
			third.Sequence++
			if e := store.Commit(third, "check"); maintenanceStoreCode(e) != "state_ambiguous" {
				t.Fatalf("commit bypassed stale slot: %v", e)
			}
		})
	}
}
