//go:build harness_maintenance

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// This opt-in test executes an operator-selected binary. Its hash pin detects
// substitution, but the operator must independently trust the binary itself.
// plan-check parses syntax only; it grants no authorization or enrollment proof.
func TestMaintenanceSwiftPlanSyntax(t *testing.T) {
	path, pin := os.Getenv("HARNESSCTL_NATIVE_PLAN_CHECK_PATH"), os.Getenv("HARNESSCTL_NATIVE_PLAN_CHECK_SHA256")
	if path == "" && pin == "" {
		t.Skip("native plan-check binary and SHA-256 pin not supplied")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("native plan-check requires macOS")
	}
	if path == "" || pin == "" || !filepath.IsAbs(path) || len(pin) != 64 {
		t.Fatal("native plan-check selection incomplete")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 64<<20 || info.Mode().Perm()&0111 == 0 {
		t.Fatal("native plan-check selection invalid")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal("native plan-check selection unavailable")
	}
	raw, readErr := io.ReadAll(io.LimitReader(f, 64<<20+1))
	closeErr := f.Close()
	if readErr != nil || closeErr != nil || int64(len(raw)) != info.Size() {
		t.Fatal("native plan-check selection unreadable")
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != pin {
		t.Fatal("native plan-check SHA-256 mismatch")
	}
	// Execute the verified bytes, not the source path which could be replaced.
	verifiedCopy := filepath.Join(t.TempDir(), "plan-check")
	if err := os.WriteFile(verifiedCopy, raw, 0700); err != nil {
		t.Fatal("native plan-check staging failed")
	}

	ctx, _ := engineContext(t)
	initial := engineInit(t, ctx)
	maintenanceRaw, err := json.Marshal(maintenanceTestCandidate())
	if err != nil {
		t.Fatal(err)
	}
	prepared := engineNext(t, maintenanceTransition(initial, maintenanceTransitionInput{Operation: "prepare", CandidateRaw: maintenanceRaw, Nonce: bytes.Repeat([]byte{1}, 32)}, ctx))
	maintenancePlan, err := canonicalMaintenancePlan(*prepared.Plan)
	if err != nil {
		t.Fatal(err)
	}
	locked := engineNext(t, maintenanceTransition(initial, maintenanceTransitionInput{Operation: "recover", InstanceEntropy: bytes.Repeat([]byte{2}, 32)}, ctx))
	rearm := maintenanceTestCandidate()
	rearm.Purpose, rearm.DurationSeconds, rearm.Controls = "rearm-baseline", 0, []maintenanceCandidateControl{}
	rearmRaw, err := json.Marshal(rearm)
	if err != nil {
		t.Fatal(err)
	}
	rearmed := engineNext(t, maintenanceTransition(locked, maintenanceTransitionInput{Operation: "prepare", CandidateRaw: rearmRaw, Nonce: bytes.Repeat([]byte{3}, 32)}, ctx))
	rearmPlan, err := canonicalMaintenancePlan(*rearmed.Plan)
	if err != nil {
		t.Fatal(err)
	}

	mutate := func(key string, replacement json.RawMessage) []byte {
		t.Helper()
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(maintenancePlan, &fields); err != nil {
			t.Fatal(err)
		}
		fields[key] = replacement
		result, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	duplicate := append([]byte(`{"version":"harnessctl.maintenance-plan/1",`), maintenancePlan[1:]...)
	unknown := append([]byte(`{"extra":0,`), maintenancePlan[1:]...)
	badUTF8 := mutate("reason", json.RawMessage(`"bad"`))
	badUTF8 = bytes.Replace(badUTF8, []byte(`"reason":"bad"`), []byte{'"', 'r', 'e', 'a', 's', 'o', 'n', '"', ':', '"', 0xff, '"'}, 1)
	wrongDeadline := mutate("deadlineContinuousNanos", json.RawMessage(`1`))
	cases := []struct {
		name     string
		body     []byte
		length   uint32
		valid    bool
		received int
	}{
		{"maintenance", maintenancePlan, uint32(len(maintenancePlan)), true, len(maintenancePlan)},
		{"rearm", rearmPlan, uint32(len(rearmPlan)), true, len(rearmPlan)},
		{"malformed", []byte(`{"`), 2, false, 2},
		{"duplicate keys", duplicate, uint32(len(duplicate)), false, len(duplicate)},
		{"unknown field", unknown, uint32(len(unknown)), false, len(unknown)},
		{"null", mutate("reason", json.RawMessage(`null`)), 0, false, 0},
		{"overflow", mutate("preparedUtcMicros", json.RawMessage(`9223372036854775808`)), 0, false, 0},
		{"float", mutate("durationSeconds", json.RawMessage(`1.5`)), 0, false, 0},
		{"invalid UTF-8", badUTF8, 0, false, 0},
		{"trailing JSON", append(append([]byte{}, maintenancePlan...), 'x'), 0, false, 0},
		{"wrong deadline", wrongDeadline, 0, false, 0},
		{"bidi", mutate("reason", json.RawMessage(`"bad\u202ereason"`)), 0, false, 0},
		{"line separator", mutate("reason", json.RawMessage(`"bad\u2028reason"`)), 0, false, 0},
		{"paragraph separator", mutate("reason", json.RawMessage(`"bad\u2029reason"`)), 0, false, 0},
		{"oversize", bytes.Repeat([]byte{'x'}, 65537), 65537, false, 0},
		{"truncated", []byte{'{'}, 2, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			length := tc.length
			if length == 0 {
				length = uint32(len(tc.body))
				tc.received = len(tc.body)
			}
			frame := make([]byte, 4+len(tc.body))
			binary.BigEndian.PutUint32(frame[:4], length)
			copy(frame[4:], tc.body)
			runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(runCtx, verifiedCopy, "plan-check")
			cmd.Env = []string{"PATH=/usr/bin:/bin", "TMPDIR=" + t.TempDir()}
			cmd.Stdin = bytes.NewReader(frame)
			stdout := &maintenanceSDKOutput{max: 512}
			stderr := &maintenanceSDKOutput{max: 512}
			cmd.Stdout, cmd.Stderr = stdout, stderr
			if err := cmd.Run(); err != nil || runCtx.Err() != nil || stdout.overflow || stderr.overflow {
				t.Fatal("native plan-check execution incomplete")
			}
			var response struct {
				Version       string `json:"version"`
				Status        string `json:"status"`
				ReceivedBytes int    `json:"receivedBytes"`
				NoAuthority   bool   `json:"noAuthority"`
			}
			if strictJSON(bytes.TrimSpace(stdout.Bytes()), &response) != nil || response.Version != "harnessctl.native-plan-check/1" || !response.NoAuthority || response.ReceivedBytes != tc.received {
				t.Fatal("native plan-check response invalid")
			}
			want := "invalid"
			if tc.valid {
				want = "valid"
			}
			if response.Status != want {
				t.Fatalf("native plan-check status=%s want=%s", response.Status, want)
			}
		})
	}
}

type maintenanceSDKOutput struct {
	bytes.Buffer
	max      int
	overflow bool
}

func (b *maintenanceSDKOutput) Write(p []byte) (int, error) {
	if len(p) > b.max-b.Len() {
		b.overflow = true
		return 0, errors.New("native output limit")
	}
	return b.Buffer.Write(p)
}
