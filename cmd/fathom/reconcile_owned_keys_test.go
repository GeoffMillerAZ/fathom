package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// Exercise the CLI's real plan/apply writers, including the pre-apply
// baseline pass which must not turn a missing ledger into an existing one
// before the legacy migration is selected.
func TestReconcileOwnedKeys(t *testing.T) {
	for _, tc := range []struct {
		name, ledger, command, neighbourCommand, asyncJSON string
		owned, update, adopt, collision, baseline          bool
		extraField                                         bool
	}{
		{name: "recorded key updates", ledger: "state", command: "legacy", owned: true, update: true},
		{name: "unrecorded fathom spelling is user owned", ledger: "state", command: "legacy", collision: true},
		{name: "colliding user key", ledger: "state", command: "printf '%s' 'fathom edit-hook'", collision: true},
		{name: "empty legacy ledger prevents adoption", ledger: "legacy", command: "legacy", collision: true},
		{name: "missing ledger adopts fathom hook", command: "legacy", update: true, adopt: true},
		{name: "missing ledger adopts before first baseline", command: "legacy", update: true, adopt: true, baseline: true},
		{name: "missing ledger adopts current hook", command: "current", update: true, adopt: true},
		{name: "missing ledger preserves extra arguments", command: "fathom edit-hook --verbose", collision: true},
		{name: "missing ledger preserves wrong event arguments", command: "fathom edit-hook --pre", collision: true},
		{name: "missing ledger preserves continued arguments", command: "fathom edit-hook \\\n --verbose", collision: true},
		{name: "missing ledger preserves async override", command: "fathom edit-hook", asyncJSON: "true", collision: true},
		{name: "missing ledger preserves invalid async", command: "fathom edit-hook", asyncJSON: `"true"`, collision: true},
		{name: "missing ledger preserves null async", command: "fathom edit-hook", asyncJSON: "null", collision: true},
		{name: "missing ledger preserves trailing comment", command: "fathom edit-hook # edit-hook", collision: true},
		{name: "missing ledger leaves colliding user hook", command: "printf '%s' 'fathom edit-hook'", collision: true},
		{name: "missing ledger leaves similarly named binary", command: "custom-fathom edit-hook", collision: true},
		{name: "missing ledger leaves unreadable command", command: "fathom edit-hook; 'unterminated", collision: true},
		{name: "missing ledger preserves preceding lint", command: "./scripts/lint.sh && fathom edit-hook", collision: true},
		{name: "missing ledger preserves following notify", command: "fathom edit-hook; ./scripts/notify.sh", collision: true},
		{name: "missing ledger preserves extra entry field", command: "fathom edit-hook", extraField: true, collision: true},
		{name: "old ledger preserves chained command", ledger: "old", command: "fathom edit-hook; ./scripts/notify.sh", collision: true},
		{name: "old ledger preserves extra entry field", ledger: "old", command: "fathom edit-hook", extraField: true, collision: true},
		{name: "unreadable ledger never adopts", ledger: "corrupt", command: "legacy", collision: true},
		{name: "colliding user key before fathom entry", command: "printf '%s' 'fathom edit-hook'", neighbourCommand: "legacy", collision: true},
		{name: "fathom entry before colliding user key", command: "legacy", neighbourCommand: "printf '%s' 'fathom edit-hook'", update: true, adopt: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			root := t.TempDir()
			hs := hookSpec{event: "PostToolUse", matcher: "Edit|Write|MultiEdit", marker: "fathom edit-hook", sub: "edit-hook", timeout: 5}
			command := tc.command
			switch command {
			case "legacy":
				command = filepath.Join(t.TempDir(), "bin", "fathom") + " edit-hook"
			case "current":
				command = hookCommand("fathom", hs.sub)
			}
			entry := map[string]any{"type": "command", "command": command, "timeout": float64(5)}
			if tc.extraField {
				entry["statusMessage"] = "Checking the edit"
			}
			if tc.asyncJSON != "" {
				var value any
				if err := json.Unmarshal([]byte(tc.asyncJSON), &value); err != nil {
					t.Fatal(err)
				}
				entry["async"] = value
			}
			neighbourCommand := tc.neighbourCommand
			if neighbourCommand == "" {
				neighbourCommand = "./scripts/check"
			}
			if neighbourCommand == "legacy" {
				neighbourCommand = filepath.Join(t.TempDir(), "bin", "fathom") + " edit-hook"
			}
			neighbour := map[string]any{"type": "command", "command": neighbourCommand, "timeout": float64(73), "custom": true}
			userValue := map[string]any{"choice": "keep", "enabled": true}
			settings := map[string]any{"userSetting": userValue, "hooks": map[string]any{
				hs.event: []any{map[string]any{"matcher": hs.matcher, "hooks": []any{entry, neighbour}}},
			}}
			body, err := json.Marshal(settings)
			if err != nil {
				t.Fatal(err)
			}
			writeSettings(t, root, body)
			if tc.baseline {
				writeRepoFile(t, root, ".mcp.json", `{"mcpServers":{"fathom":{"command":"fathom","args":["mcp"]}}}`)
				if len(analyzeWiring(root).WiringUnrecorded) == 0 {
					t.Fatal("fixture has no first-sight baseline to record")
				}
			}
			k := hookKeyFor(hs, command)
			ledger := wiring.NewLedger()
			ledger.Version = 2 // These fixtures represent a completed migration.
			if tc.owned {
				value, err := json.Marshal(entry)
				if err != nil {
					t.Fatal(err)
				}
				ledger.Record(k.File, wiring.OwnedKey{ID: k.ID(), Digest: wiringDigest(string(value)), Wrote: "2026-01-01T00:00:00Z"})
			}
			switch tc.ledger {
			case "state":
				if err := wiring.SaveLedger(root, ledger); err != nil {
					t.Fatal(err)
				}
			case "legacy", "old":
				if tc.ledger == "old" {
					ledger.Version = 1
				}
				b, err := json.Marshal(ledger)
				if err != nil {
					t.Fatal(err)
				}
				writeRepoFile(t, root, ".fathom/owned.json", string(b))
			case "corrupt":
				path := wiring.LedgerPath(root)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			run := func(apply bool) string {
				t.Helper()
				args := []string{"--repo", root, "--url", "://unavailable"}
				if apply {
					args = append(args, "--apply")
				}
				var out, errb strings.Builder
				if code := cmdFleetReconcile(args, &out, &errb); code != 0 {
					t.Fatalf("exit %d: %s%s", code, &out, &errb)
				}
				return out.String()
			}
			dry := run(false)
			if tc.adopt && strings.Count(dry, "would adopt 1 fathom keys into the ledger") != 1 {
				t.Errorf("dry-run must name pending adoption once:\n%s", dry)
			}
			if tc.collision && strings.Count(dry, "collision:") != 1 {
				t.Errorf("dry-run must report collision once:\n%s", dry)
			}
			out := run(true)
			var after map[string]any
			if err := json.Unmarshal([]byte(readRepoFile(t, root, k.File)), &after); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after["userSetting"], userValue) {
				t.Errorf("user setting changed: %v", after["userSetting"])
			}
			groups := after["hooks"].(map[string]any)[hs.event].([]any)
			entries := groups[0].(map[string]any)["hooks"].([]any)
			if tc.collision && !reflect.DeepEqual(entries[0], entry) {
				t.Errorf("colliding entry changed: got %v, want %v", entries[0], entry)
			}
			if !reflect.DeepEqual(entries[1], neighbour) {
				t.Errorf("neighbour changed: %v", entries[1])
			}
			got := entries[0].(map[string]any)["command"]
			want := command
			if tc.update {
				want = hookCommand("fathom", hs.sub)
			}
			if got != want {
				t.Errorf("command = %q, want %q", got, want)
			}
			if tc.collision && strings.Count(out, "collision:") != 1 {
				t.Errorf("apply must report collision once:\n%s", out)
			}
			if tc.adopt && strings.Count(out, "adopted 1 fathom keys into the ledger") != 1 {
				t.Errorf("adoption must be reported once:\n%s", out)
			}
			ledger, err = wiring.LoadLedger(root)
			if err != nil {
				t.Fatal(err)
			}
			if owned := ledgerDigest(ledger, k.File, k.ID()) != ""; owned != (tc.owned || tc.adopt) {
				t.Errorf("key ownership = %v, want %v", owned, tc.owned || tc.adopt)
			}
			// A missing template key is always added, and recorded from its bytes.
			stop := hookKeyFor(hookSpec{event: "Stop", marker: "fathom stop-hook"}, "")
			stopEntry := after["hooks"].(map[string]any)["Stop"].([]any)[0].(map[string]any)["hooks"].([]any)[0]
			stopValue, err := json.Marshal(stopEntry)
			if err != nil {
				t.Fatal(err)
			}
			if ledgerDigest(ledger, stop.File, stop.ID()) != wiringDigest(string(stopValue)) {
				t.Error("missing template key was not recorded from the bytes written")
			}
			if tc.adopt {
				if again := run(true); strings.Contains(again, "adopted ") {
					t.Errorf("adoption repeated on the next run:\n%s", again)
				}
			}
		})
	}
}

func TestReconcileAdoptsLegacyLedgerOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	var keys []wiring.Key
	for i, hs := range fathomHookSpecsAt(root) {
		command := "fathom " + hs.sub
		if i%2 == 0 {
			command = hookCommand("fathom", hs.sub)
		}
		keys = append(keys, hookKeyFor(hs, command))
	}
	if len(keys) <= 7 {
		t.Fatal("fixture needs more than seven hook keys")
	}
	clock := func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	if _, err := wiring.ApplyWiring(root, wiring.Spec{Keys: keys}, wiring.ApplyOptions{Now: clock}); err != nil {
		t.Fatal(err)
	}
	ledger, err := wiring.LoadLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	ledger.Version = 1
	ledger.Files[keys[0].File] = wiring.OwnedKeys(ledger, keys[0].File)[:7]
	legacy, err := json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, root, ".fathom/owned.json", string(legacy))
	if err := os.Remove(wiring.LedgerPath(root)); err != nil {
		t.Fatal(err)
	}
	run := func(apply bool) string {
		t.Helper()
		args := []string{"--repo", root, "--url", "://unavailable"}
		if apply {
			args = append(args, "--apply")
		}
		var out, errb strings.Builder
		if code := cmdFleetReconcile(args, &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s%s", code, &out, &errb)
		}
		return out.String()
	}
	if dry := run(false); strings.Contains(dry, "collision:") {
		t.Errorf("legacy fathom hooks reported as user collisions:\n%s", dry)
	}
	if got := readRepoFile(t, root, ".fathom/owned.json"); got != string(legacy) {
		t.Fatal("dry run rewrote the legacy ledger")
	}
	out := run(true)
	want := fmt.Sprintf("adopted %d fathom keys into the ledger", len(keys)-7)
	if strings.Count(out, want) != 1 || strings.Contains(out, "collision:") {
		t.Errorf("migration did not report exactly one adoption and no collisions:\n%s", out)
	}
	ledger, err = wiring.LoadLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Version != 2 || len(wiring.OwnedKeys(ledger, keys[0].File)) != len(keys) {
		t.Fatalf("migration ledger version=%d hooks=%d", ledger.Version, len(wiring.OwnedKeys(ledger, keys[0].File)))
	}
	if again := run(true); strings.Contains(again, "adopted ") {
		t.Errorf("adoption repeated:\n%s", again)
	}
	if dry := run(false); !strings.Contains(dry, "wiring current") || strings.Contains(dry, "collision:") {
		t.Errorf("migrated repository does not read current:\n%s", dry)
	}
	// The old on-disk ledger must not reopen adoption after migration.
	ledger, err = wiring.LoadLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	ledger.Forget(keys[0].File, keys[0].ID())
	if err := wiring.SaveLedger(root, ledger); err != nil {
		t.Fatal(err)
	}
	if after := run(true); strings.Count(after, "collision:") != 1 || strings.Contains(after, "adopted ") {
		t.Errorf("current ledger reopened legacy adoption:\n%s", after)
	}
}

func TestAdoptOnlyExactFathomInvocation(t *testing.T) {
	for _, tc := range []struct {
		command string
		want    bool
	}{
		{"fathom edit-hook", true},
		{"./bin/fathom edit-hook --quiet", false},
		{"fathom edit-hook --pre", false},
		{"fathom edit-hook # edit-hook", false},
		{"fathom # ignored edit-hook", false},
		{hookCommand("fathom", "edit-hook --verbose"), false},
		{"fathom edit-hook \\\n --verbose", false},
		{"\"./bin with spaces/fathom\" edit-hook", true},
		{hookCommand("fathom", "edit-hook --pre"), true},
		{hookCommand("./bin/fathom", "edit-hook"), true},
		{"command -v ./scripts/check; ./bin/fathom >/dev/null 2>&1 || exit 0; './scripts/check; ./bin/fathom' edit-hook", false},
		{"./scripts/lint.sh && fathom edit-hook", false},
		{"fathom edit-hook; ./scripts/notify.sh", false},
		{hookCommand("fathom", "edit-hook") + "; ./scripts/notify.sh", false},
		{"fathom edit-hook | cat", false},
		{"fathom edit-hook &", false},
		{"(fathom edit-hook)", false},
		{"bash -c 'fathom edit-hook'", false},
		{"exec fathom edit-hook", false},
		{"MODE=check fathom edit-hook", false},
		{"fathom edit-hook >result.txt", false},
		{"fathom edit-hook $(./scripts/notify.sh)", false},
		{"fathom edit-hook $EXTRA", false},
		{"command -v fathom || exit 1; fathom edit-hook", false},
		{"command -v fathom >/dev/null 2>&1 || exit 0; custom-fathom edit-hook", false},
		{"custom-fathom edit-hook", false},
		{"printf '%s' 'fathom edit-hook'", false},
	} {
		t.Run(tc.command, func(t *testing.T) {
			subcommand := "edit-hook"
			if tc.command == hookCommand("fathom", "edit-hook --pre") {
				subcommand = "edit-hook --pre"
			}
			if got := invokesFathom(tc.command, subcommand); got != tc.want {
				t.Errorf("invokesFathom(%q) = %v, want %v", tc.command, got, tc.want)
			}
		})
	}
}
