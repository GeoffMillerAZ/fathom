package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWiringFaultRecordShapes(t *testing.T) {
	root := t.TempDir()
	wiringFaultCorpus(t, root)
	files := wiringFaultFiles(t, root)
	var manifest struct {
		Cases []struct {
			Directory           string `json:"directory"`
			Fault               string `json:"fault"`
			RecordScope         string `json:"record_scope"`
			RecordsNote         string `json:"records_note"`
			RemovedRegistration string `json:"removed_registration"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(files["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	var current, stale StatuszReport
	for name, report := range map[string]*StatuszReport{"healthy-001": &current, "stale-binary": &stale} {
		if err := json.Unmarshal(files[name+"/statusz.json"], report); err != nil {
			t.Fatal(err)
		}
	}
	sawStaleHook := false
	for _, c := range manifest.Cases {
		if c.Fault == "stale-hook-binary" {
			sawStaleHook = true
		}
		t.Run(c.Directory, func(t *testing.T) {
			wantHook, wantDaemon := current.BinaryCommit[:12], current.BinaryCommit[:12]
			if c.Fault == "stale-binary" {
				wantHook, wantDaemon = stale.BinaryCommit[:12], stale.BinaryCommit[:12]
			}
			if c.Fault == "stale-hook-binary" {
				wantHook = stale.BinaryCommit[:12]
				var report StatuszReport
				if err := json.Unmarshal(files[c.Directory+"/statusz.json"], &report); err != nil {
					t.Fatal(err)
				}
				if report.BinaryCommit != current.BinaryCommit {
					t.Error("stale hook must leave the daemon current")
				}
			}
			for _, raw := range bytes.Split(bytes.TrimSpace(files[c.Directory+"/hooks.jsonl"]), []byte("\n")) {
				if len(raw) == 0 {
					continue
				}
				var beat struct {
					Meta map[string]string `json:"meta"`
				}
				if err := json.Unmarshal(raw, &beat); err != nil {
					t.Fatal(err)
				}
				if got := beat.Meta["fathom_version"]; got != wantHook {
					t.Errorf("beat fathom_version = %q, want %q", got, wantHook)
				}
			}
			call := files[c.Directory+"/mcp-calls.jsonl"]
			if len(call) != 0 {
				var row map[string]any
				if err := json.Unmarshal(call, &row); err != nil {
					t.Fatal(err)
				}
				for key, want := range map[string]any{"schema_version": float64(3), "fathom_version": wantDaemon, "outcome": "ok", "class": "tree", "queued_ms": float64(0), "bytes_read": float64(0), "ts": "2026-10-20T12:00:00Z", "latency_ms": float64(0)} {
					if row[key] != want {
						t.Errorf("call %s = %v, want %v", key, row[key], want)
					}
				}
				// obligations returns rules, not an answer-list key recognized
				// by the writer's outcomeFrom. Do not invent a result count.
				if _, exists := row["result_count"]; exists {
					t.Error("obligations call invents a result_count")
				}
			}
			switch c.Fault {
			case "hook-unregistered":
				if c.RemovedRegistration != "SessionStart:startup|resume|clear|compact" {
					t.Errorf("removed registration = %q", c.RemovedRegistration)
				}
				raw := files[c.Directory+"/.claude/settings.json"]
				var indented bytes.Buffer
				if err := json.Indent(&indented, raw, "", "  "); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(raw, indented.Bytes()) {
					t.Error("settings must preserve the product writer's indentation")
				}
			case "mcp-not-connected":
				if c.RecordScope != "bench-only" {
					t.Errorf("record_scope = %q, want bench-only", c.RecordScope)
				}
			case "channel-off":
				if !strings.Contains(c.RecordsNote, "daemon's environment") || !strings.Contains(c.RecordsNote, "harness") {
					t.Error("records_note must distinguish daemon and harness environments")
				}
			case "stale-binary":
				if !strings.Contains(c.RecordsNote, "local binary") || !strings.Contains(c.RecordsNote, "HEAD") {
					t.Error("records_note must name the daemon-versus-local-binary comparison")
				}
			}
		})
	}
	if !sawStaleHook {
		t.Error("missing stale-hook-binary case")
	}
}

func TestWiringFaultDaemonErrorShape(t *testing.T) {
	err := wiringFaultDaemonError(t)
	var op *net.OpError
	if !errors.As(err, &op) || op.Op != "dial" || op.Net != "tcp" || !strings.Contains(err.Error(), "connect: connection refused") {
		t.Fatalf("daemon error must wrap a real-shaped TCP dial failure: %v", err)
	}
}

func TestWiringFaultUpdateRemovesOrphans(t *testing.T) {
	root := t.TempDir()
	wiringFaultWrite(t, root, "renamed-case/orphan.json", []byte("{}\n"))
	wiringFaultWriteGolden(t, root, map[string][]byte{"current-case/record.json": []byte("{}\n")})
	if _, err := os.Stat(filepath.Join(root, "renamed-case")); !os.IsNotExist(err) {
		t.Fatalf("-update left an orphan case: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "current-case", "record.json")); err != nil {
		t.Fatal(err)
	}
}

func TestWiringFaultMissingGitSkips(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Run("missing-git", func(t *testing.T) {
		wiringFaultRevisions(t, time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC))
		t.Error("git-dependent fixture generation must skip when git is missing")
	})
}
