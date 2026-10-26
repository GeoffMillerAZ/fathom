//go:build harness_maintenance

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func nativeFixture(t *testing.T, body string) maintenanceNative {
	t.Helper()
	path := filepath.Join(t.TempDir(), "helper")
	if e := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0700); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256(raw)
	root := nativeSafeRoot(t)
	return maintenanceNative{Profile: "darwin-protected-key/1", Executable: path, SHA256: hex.EncodeToString(h[:]), PreparationRoot: root, TimeoutMillis: 1000}
}

func nativeSafeRoot(t *testing.T) string {
	t.Helper()
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	root, e := os.MkdirTemp(home, "harnessctl-native-test-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := os.RemoveAll(root); e != nil {
			t.Error(e)
		}
	})
	if e := os.Chmod(root, 0700); e != nil {
		t.Fatal(e)
	}
	if _, ok := maintenanceNativeProtectedRoot(root); !ok {
		t.Fatalf("test root lacks protected ancestors: %s", root)
	}
	return root
}

func nativePrepareFixture(t *testing.T, selection maintenanceNative) *maintenanceNativeRuntime {
	t.Helper()
	n, e := prepareMaintenanceNativeWithHost(selection, "darwin")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := n.Close(); e != nil {
			t.Error(e)
		}
	})
	return n
}

func TestMaintenanceNativeStampBoundaries(t *testing.T) {
	stamp := `{"version":"harnessctl.host-stamp/1","os":"darwin","osUserId":"` + strconv.Itoa(os.Getuid()) + `","bootId":"boot","utcMicros":1,"continuousNanos":0}`
	selection := nativeFixture(t, "printf '%s\\n' '"+stamp+"'")
	n := nativePrepareFixture(t, selection)
	for i := 0; i < 4; i++ {
		got, e := n.Stamp(context.Background())
		if e != nil || got.BootID != "boot" {
			t.Fatalf("stamp %d: %+v %v", i, got, e)
		}
	}
	if _, e := n.Stamp(context.Background()); e == nil {
		t.Fatal("fifth stamp accepted")
	}
	if c := n.Counts(); c.StampAttempts != 4 || c.StampInvoked != 4 || c.StampCompleted != 4 {
		t.Fatalf("counts: %+v", c)
	}
}

func TestMaintenanceNativeSourceAndDestinationPins(t *testing.T) {
	s := nativeFixture(t, "exit 0")
	s.SHA256 = strings.Repeat("0", 64)
	if _, e := prepareMaintenanceNativeWithHost(s, "darwin"); e == nil {
		t.Fatal("wrong source pin accepted")
	}
	s = nativeFixture(t, "exit 0")
	if _, e := prepareMaintenanceNativeWithHost(s, "linux"); e == nil {
		t.Fatal("wrong host accepted")
	}
	rootLink := filepath.Join(t.TempDir(), "root-link")
	if e := os.Symlink(s.PreparationRoot, rootLink); e != nil {
		t.Fatal(e)
	}
	badRoot := s
	badRoot.PreparationRoot = rootLink
	if _, e := prepareMaintenanceNativeWithHost(badRoot, "darwin"); e == nil {
		t.Fatal("symlink preparation root accepted")
	}
	if e := os.Chmod(s.PreparationRoot, 0755); e != nil {
		t.Fatal(e)
	}
	if _, e := prepareMaintenanceNativeWithHost(s, "darwin"); e == nil {
		t.Fatal("shared preparation root accepted")
	}
	if e := os.Chmod(s.PreparationRoot, 0700); e != nil {
		t.Fatal(e)
	}
	link := filepath.Join(t.TempDir(), "link")
	if e := os.Symlink(s.Executable, link); e != nil {
		t.Fatal(e)
	}
	s.Executable = link
	if _, e := prepareMaintenanceNativeWithHost(s, "darwin"); e == nil {
		t.Fatal("source symlink accepted")
	}
	s = nativeFixture(t, "exit 0")
	n := nativePrepareFixture(t, s)
	if e := os.WriteFile(n.executable, []byte("#!/bin/sh\nexit 0\n# mutated\n"), 0700); e != nil {
		t.Fatal(e)
	}
	if _, e := n.Stamp(context.Background()); e == nil {
		t.Fatal("changed private snapshot accepted")
	}
	if c := n.Counts(); c.StampInvoked != 0 || c.StampCompleted != 0 {
		t.Fatalf("false invocation: %+v", c)
	}
}

func TestMaintenanceNativeAncestorWritabilityAndReplacement(t *testing.T) {
	stamp := `{"version":"harnessctl.host-stamp/1","os":"darwin","osUserId":"` + strconv.Itoa(os.Getuid()) + `","bootId":"boot","utcMicros":1,"continuousNanos":0}`
	selection := nativeFixture(t, "printf '%s\\n' '"+stamp+"'")
	parent := nativeSafeRoot(t)
	child, e := os.MkdirTemp(parent, "preparation-")
	if e != nil {
		t.Fatal(e)
	}
	selection.PreparationRoot = child
	if e := os.Chmod(parent, 0770); e != nil {
		t.Fatal(e)
	}
	if _, e := prepareMaintenanceNativeWithHost(selection, "darwin"); e == nil {
		t.Fatal("writable ancestor accepted at acquisition")
	}
	if e := os.Chmod(parent, 0700); e != nil {
		t.Fatal(e)
	}
	n := nativePrepareFixture(t, selection)
	if e := os.Chmod(parent, 0770); e != nil {
		t.Fatal(e)
	}
	if _, e := n.Stamp(context.Background()); e == nil {
		t.Fatal("writable ancestor accepted at execution")
	}
	if c := n.Counts(); c.StampInvoked != 0 {
		t.Fatalf("child started through writable ancestor: %+v", c)
	}
	if e := os.Chmod(parent, 0700); e != nil {
		t.Fatal(e)
	}
	moved := parent + "-moved"
	if e := os.Rename(parent, moved); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(parent, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Rename(filepath.Join(moved, filepath.Base(child)), child); e != nil {
		t.Fatal(e)
	}
	if _, e := n.Stamp(context.Background()); e == nil {
		t.Fatal("replaced ancestor accepted with original root identity")
	}
	if c := n.Counts(); c.StampInvoked != 0 {
		t.Fatalf("child started through replaced ancestor: %+v", c)
	}
	if e := os.Rename(child, filepath.Join(moved, filepath.Base(child))); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(parent); e != nil {
		t.Fatal(e)
	}
	if e := os.Rename(moved, parent); e != nil {
		t.Fatal(e)
	}
}

type nativeGrowingReader struct{ read int }

func (r *nativeGrowingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	r.read += len(p)
	return len(p), nil
}

func TestMaintenanceNativeGrowthAndOversizeStayBounded(t *testing.T) {
	growing := &nativeGrowingReader{}
	raw, e := maintenanceNativeReadBounded(growing)
	if e != nil || len(raw) != maintenanceNativeExecutableLimit+1 || growing.read != maintenanceNativeExecutableLimit+1 {
		t.Fatalf("growth read=%d captured=%d err=%v", growing.read, len(raw), e)
	}
	selection := nativeFixture(t, "exit 0")
	n := nativePrepareFixture(t, selection)
	if e := os.Truncate(n.executable, maintenanceNativeExecutableLimit+1); e != nil {
		t.Fatal(e)
	}
	if _, e := n.Stamp(context.Background()); e == nil {
		t.Fatal("oversized destination executed")
	}
	if c := n.Counts(); c.StampInvoked != 0 || c.StampCompleted != 0 {
		t.Fatalf("false invocation: %+v", c)
	}
	stamp := `{"version":"harnessctl.host-stamp/1","os":"darwin","osUserId":"` + strconv.Itoa(os.Getuid()) + `","bootId":"boot","utcMicros":1,"continuousNanos":0}`
	selection = nativeFixture(t, "printf '%s\\n' '"+stamp+"'")
	n2 := nativePrepareFixture(t, selection)
	// Source changes after snapshot cannot substitute the pinned private executable.
	if e := os.Truncate(selection.Executable, maintenanceNativeExecutableLimit+1); e != nil {
		t.Fatal(e)
	}
	if _, e := n2.Stamp(context.Background()); e != nil {
		t.Fatalf("private snapshot changed with source: %v", e)
	}
	if e := os.Chmod(selection.PreparationRoot, 0755); e != nil {
		t.Fatal(e)
	}
	if _, e := n2.Stamp(context.Background()); e == nil {
		t.Fatal("weakened preparation root accepted")
	}
	if c := n2.Counts(); c.StampInvoked != 1 {
		t.Fatalf("false invocation after root change: %+v", c)
	}
	if e := os.Chmod(selection.PreparationRoot, 0700); e != nil {
		t.Fatal(e)
	}
}

func TestMaintenanceNativeFastApprovalWriterWitness(t *testing.T) {
	selection := nativeFixture(t, "dd bs=1 count=6 of=/dev/null 2>/dev/null\nprintf '%s\\n' '{\"version\":\"harnessctl.native-approval/1\",\"status\":\"denied\",\"signature\":\"\",\"publicKey\":\"\"}'")
	a := maintenanceTestApprover()
	a.OSUserID = strconv.Itoa(os.Getuid())
	for i := 0; i < 32; i++ {
		n, e := prepareMaintenanceNativeWithHost(selection, "darwin")
		if e != nil {
			t.Fatal(e)
		}
		_, approveErr := n.Approve(context.Background(), []byte("{}"), a)
		closeErr := n.Close()
		if approveErr != nil || closeErr != nil {
			t.Fatalf("iteration %d approval=%v close=%v", i, approveErr, closeErr)
		}
		if c := n.Counts(); c.ApprovalInvoked != 1 || c.ApprovalCompleted != 1 {
			t.Fatalf("iteration %d counts %+v", i, c)
		}
	}
}

var _ io.Reader = (*nativeGrowingReader)(nil)

func TestMaintenanceNativeApprovalWireAndOneAttempt(t *testing.T) {
	s := nativeFixture(t, "dd bs=1 count=6 of=/dev/null 2>/dev/null\nprintf '%s\\n' '{\"version\":\"harnessctl.native-approval/1\",\"status\":\"denied\",\"signature\":\"\",\"publicKey\":\"\"}'")
	n := nativePrepareFixture(t, s)
	a := maintenanceTestApprover()
	a.OSUserID = strconv.Itoa(os.Getuid())
	raw, e := n.Approve(context.Background(), []byte("{}"), a)
	if e != nil {
		t.Fatal(e)
	}
	approval, e := decodeMaintenanceNativeApproval(raw)
	if e != nil || approval.Status != "denied" {
		t.Fatalf("approval: %+v %v", approval, e)
	}
	if _, e := n.Approve(context.Background(), []byte("{}"), a); e == nil {
		t.Fatal("second approval accepted")
	}
	if c := n.Counts(); c.ApprovalAttempts != 1 || c.ApprovalInvoked != 1 || c.ApprovalCompleted != 1 {
		t.Fatalf("counts: %+v", c)
	}
}

func TestMaintenanceNativeIncompleteOutputAndLifeline(t *testing.T) {
	a := maintenanceTestApprover()
	a.OSUserID = strconv.Itoa(os.Getuid())
	cases := []struct {
		name, body string
		approval   bool
	}{
		{"wrong-wire", "printf '%s\\n' '{}'", false},
		{"stdout-cap", "head -c 65537 /dev/zero", false},
		{"stderr-cap", "head -c 65537 /dev/zero >&2", false},
		{"start-failed", "exit 0", false},
		{"lifeline", "dd bs=1 count=6 of=/dev/null 2>/dev/null\ndd bs=1 count=1 of=/dev/null 2>/dev/null", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := nativeFixture(t, tc.body)
			if tc.approval {
				s.TimeoutMillis = 100
			}
			n := nativePrepareFixture(t, s)
			if tc.name == "start-failed" {
				if e := os.Remove(n.executable); e != nil {
					t.Fatal(e)
				}
			}
			if tc.approval {
				if _, e := n.Approve(context.Background(), []byte("{}"), a); e == nil {
					t.Fatal("lifeline exited as valid response")
				}
				if n.Counts().ApprovalCompleted != 0 {
					t.Fatal("false completed approval")
				}
			} else {
				if _, e := n.Stamp(context.Background()); e == nil {
					t.Fatal("bad stamp accepted")
				}
				if n.Counts().StampCompleted != 0 {
					t.Fatal("false completed stamp")
				}
			}
		})
	}
}

func TestMaintenanceNativeCancellation(t *testing.T) {
	s := nativeFixture(t, "while :; do :; done")
	s.TimeoutMillis = 100
	n := nativePrepareFixture(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, e := n.Stamp(ctx); e == nil {
		t.Fatal("canceled stamp succeeded")
	}
	if n.Counts().StampCompleted != 0 {
		t.Fatal("false completion")
	}
}
