package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/scenariobench"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

type wiringFaultCase struct {
	Directory             string   `json:"directory"`
	Fault                 string   `json:"fault"`
	HealthyTwin           string   `json:"healthy_twin,omitempty"`
	TouchedFiles          []string `json:"touched_files"`
	DetectableFromRecords bool     `json:"detectable_from_records"`
	RecordsNote           string   `json:"records_note,omitempty"`
	RecordScope           string   `json:"record_scope,omitempty"`
	RemovedRegistration   string   `json:"removed_registration,omitempty"`
}

type wiringFaultManifest struct {
	Cases []wiringFaultCase `json:"cases"`
}

// Regenerate with go test -count=1 -p 4 ./cmd/fathom -run WiringFault -update.
func TestWiringFaultCorpus(t *testing.T) {
	generated := t.TempDir()
	wiringFaultCorpus(t, generated)
	got := wiringFaultFiles(t, generated)
	golden := filepath.Join("testdata", "wiring-faults")
	if *updateGolden {
		wiringFaultWriteGolden(t, golden, got)
	}
	want := wiringFaultFiles(t, golden)
	if !reflect.DeepEqual(wiringFaultPaths(got), wiringFaultPaths(want)) {
		t.Fatalf("corpus paths differ: generated %v, committed %v", wiringFaultPaths(got), wiringFaultPaths(want))
	}
	for _, name := range wiringFaultPaths(want) {
		if !bytes.Equal(got[name], want[name]) {
			t.Errorf("%s differs from generated corpus (regenerate with -update)", name)
		}
	}
}

func wiringFaultWriteGolden(t *testing.T, golden string, files map[string][]byte) {
	t.Helper()
	if err := os.RemoveAll(golden); err != nil {
		t.Fatal(err)
	}
	for _, name := range wiringFaultPaths(files) {
		wiringFaultWrite(t, golden, name, files[name])
	}
}

func TestWiringFaultTwins(t *testing.T) {
	root := filepath.Join("testdata", "wiring-faults")
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest wiringFaultManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	wantFaults := []string{"channel-off", "daemon-down", "hook-unregistered", "index-behind-head", "mcp-not-connected", "rules-empty", "stale-binary", "stale-hook-binary"}
	faults := []string{}
	healthy := map[string]bool{}
	directories := []string{}
	for _, c := range manifest.Cases {
		if !fs.ValidPath(c.Directory) || c.Directory == "." || filepath.Base(c.Directory) != c.Directory {
			t.Fatalf("invalid case directory %q", c.Directory)
		}
		for _, previous := range directories {
			if previous == c.Directory {
				t.Fatalf("duplicate case directory %q", c.Directory)
			}
		}
		directories = append(directories, c.Directory)
		if c.Fault == "healthy" {
			healthy[c.Directory] = true
			if c.HealthyTwin != "" || len(c.TouchedFiles) != 0 {
				t.Fatalf("healthy case has an injection: %+v", c)
			}
		} else {
			faults = append(faults, c.Fault)
		}
	}
	if !sort.StringsAreSorted(directories) {
		t.Fatal("manifest cases must be sorted")
	}
	sort.Strings(faults)
	if len(healthy) < 2 || !reflect.DeepEqual(faults, wantFaults) {
		t.Fatalf("want two healthy cases and exactly the eight injected faults, got %d healthy and %v", len(healthy), faults)
	}
	for _, c := range manifest.Cases {
		if c.Fault == "healthy" {
			continue
		}
		t.Run(c.Directory, func(t *testing.T) {
			if !healthy[c.HealthyTwin] {
				t.Fatalf("unknown healthy twin %q", c.HealthyTwin)
			}
			if !c.DetectableFromRecords && c.RecordsNote == "" {
				t.Fatal("undetectable injection must explain the missing record")
			}
			got := wiringFaultFiles(t, filepath.Join(root, c.Directory))
			want := wiringFaultFiles(t, filepath.Join(root, c.HealthyTwin))
			paths := map[string][]byte{}
			for name := range got {
				paths[name] = nil
			}
			for name := range want {
				paths[name] = nil
			}
			different := []string{}
			for _, name := range wiringFaultPaths(paths) {
				g, existsG := got[name]
				w, existsW := want[name]
				if existsG != existsW || !bytes.Equal(g, w) {
					different = append(different, name)
				}
			}
			if len(different) == 0 || !reflect.DeepEqual(different, c.TouchedFiles) {
				t.Fatalf("changed files = %v, injection declared %v", different, c.TouchedFiles)
			}
		})
	}
}

func wiringFaultFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			t.Fatalf("non-regular corpus file %s", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = raw
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func wiringFaultPaths(files map[string][]byte) []string {
	paths := make([]string, 0, len(files))
	for name := range files {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	return paths
}

func wiringFaultCorpus(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("FATHOM_HOME", t.TempDir())
	t.Setenv("FATHOM_CHANNELS", "")
	oldHarness := beatHarness
	beatHarness = harness.ClaudeCode
	t.Cleanup(func() { beatHarness = oldHarness })
	now := func() time.Time { return time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC) }
	parent, head := wiringFaultRevisions(t, now())
	stampPath := "session-stamps/session-001.json"
	// This is the injection ledger, not a classifier. File lists and labels
	// are declared before any fixture is rendered or read.
	manifest := wiringFaultManifest{Cases: []wiringFaultCase{
		{Directory: "channel-off", Fault: "channel-off", HealthyTwin: "healthy-001", TouchedFiles: []string{"hooks.jsonl", "mcp-calls.jsonl", stampPath}, RecordsNote: "FATHOM_CHANNELS=off exits before any hook record; absence alone cannot identify the kill switch. statusz reflects only the daemon's environment, never the harness's."},
		{Directory: "daemon-down", Fault: "daemon-down", HealthyTwin: "healthy-001", TouchedFiles: []string{"hooks.jsonl", "mcp-calls.jsonl", stampPath, "statusz.json"}, DetectableFromRecords: true},
		{Directory: "healthy-001", Fault: "healthy", TouchedFiles: []string{}, DetectableFromRecords: true},
		{Directory: "healthy-002", Fault: "healthy", TouchedFiles: []string{}, DetectableFromRecords: true},
		{Directory: "hook-unregistered", Fault: "hook-unregistered", HealthyTwin: "healthy-001", TouchedFiles: []string{".claude/settings.json", "hooks.jsonl", stampPath, "stream.jsonl"}, DetectableFromRecords: true, RemovedRegistration: "SessionStart:startup|resume|clear|compact"},
		{Directory: "index-behind-head", Fault: "index-behind-head", HealthyTwin: "healthy-001", TouchedFiles: []string{stampPath, "statusz.json"}, DetectableFromRecords: true},
		{Directory: "mcp-not-connected", Fault: "mcp-not-connected", HealthyTwin: "healthy-001", TouchedFiles: []string{"stream.jsonl"}, DetectableFromRecords: true, RecordScope: "bench-only", RecordsNote: "The bench captures Claude Code's init event, which explicitly records an empty MCP server list; a normal session writes no init capture."},
		{Directory: "rules-empty", Fault: "rules-empty", HealthyTwin: "healthy-001", TouchedFiles: []string{"hooks.jsonl", "mcp-calls.jsonl"}, DetectableFromRecords: true},
		{Directory: "stale-binary", Fault: "stale-binary", HealthyTwin: "healthy-001", TouchedFiles: []string{"hooks.jsonl", "mcp-calls.jsonl", "statusz.json"}, DetectableFromRecords: true, RecordsNote: "Models a stale daemon and hooks. doctor daemon_fresh compares statusz binary_commit with the local binary running doctor, not repository HEAD; that local binary is assumed current here."},
		{Directory: "stale-hook-binary", Fault: "stale-hook-binary", HealthyTwin: "healthy-001", TouchedFiles: []string{"hooks.jsonl"}, DetectableFromRecords: true, RecordsNote: "Only the hook binary is stale: its beats stamp the parent revision while the daemon's statusz and MCP calls remain current."},
	}}
	for _, c := range manifest.Cases {
		root := filepath.Join(dir, c.Directory)
		seed := 1
		if c.Directory == "healthy-002" {
			seed = 2
		}
		service := fmt.Sprintf("service-%03d", seed)
		project := "/repos/" + service
		session := fmt.Sprintf("session-%03d", seed)
		write := func(name string, raw []byte) { wiringFaultWrite(t, root, name, raw) }

		settings, _, skipped, err := mergeHookSettings(nil, "/tools/fathom")
		if err != nil || len(skipped) != 0 {
			t.Fatalf("render settings: skipped=%v err=%v", skipped, err)
		}
		if c.Fault == "hook-unregistered" {
			var settingsObject map[string]any
			if err := json.Unmarshal(settings, &settingsObject); err != nil {
				t.Fatal(err)
			}
			// Remove precisely the primer's registration. Other hook entries
			// still run, so edit-hook traffic remains in this session.
			delete(settingsObject["hooks"].(map[string]any), "SessionStart")
			var out bytes.Buffer
			encoder := json.NewEncoder(&out)
			encoder.SetEscapeHTML(false)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(settingsObject); err != nil {
				t.Fatal(err)
			}
			settings = out.Bytes()
		}
		write(".claude/settings.json", settings)

		indexed, binary, behind := head, head, 0
		if c.Fault == "index-behind-head" {
			indexed, behind = parent, 1
		}
		if c.Fault == "stale-binary" {
			binary = parent
		}
		hookVersion := binary[:12]
		if c.Fault == "stale-hook-binary" {
			hookVersion = parent[:12]
		}
		if c.Fault != "daemon-down" {
			clean, known := false, true
			write("statusz.json", wiringFaultJSON(t, StatuszReport{
				SchemaVersion: statuszSchemaVersion, GeneratedAt: now().Format(time.RFC3339),
				StatuszBuildInfo: StatuszBuildInfo{BinaryCommit: binary, BuiltAt: now().Format(time.RFC3339)},
				Root:             &StatuszRoot{Dir: project, Indexed: &known, Service: service},
				Services: []StatuszServiceRow{{Service: service, IndexedCommit: indexed,
					StatuszHead: &StatuszHead{HeadCommit: &head, Behind: &behind, Dirty: &clean}}},
			}))
		}

		var beats, calls bytes.Buffer
		log, err := heartbeat.NewLog(&beats, now)
		if err != nil {
			t.Fatal(err)
		}
		beat := func(channel, kind string, meta map[string]string) {
			if meta == nil {
				meta = map[string]string{}
			}
			// Caller overrides are stampBeatMeta's supported seam. No build,
			// working-directory or hook-invocation identity can leak in.
			meta["project_dir"], meta["fathom_version"] = project, hookVersion
			meta["hook_inv"], meta["harness"] = channel+"-"+session, harness.ClaudeCode
			if err := log.Beat(channel, kind, stampBeatMeta(meta)); err != nil {
				t.Fatal(err)
			}
		}
		channels := ""
		if c.Fault == "channel-off" {
			channels = "off"
		}
		t.Setenv("FATHOM_CHANNELS", channels)
		if !fathomChannelsDisabled() {
			if c.Fault != "hook-unregistered" {
				beat("session-hook", "fired", nil)
				t.Setenv("FATHOM_SESSION_STAMPS", filepath.Join(root, "session-stamps"))
				stamp := sessionStamp{SchemaVersion: 1, SessionID: session, Source: "startup", At: now().Format(time.RFC3339), Cwd: project}
				if c.Fault != "daemon-down" {
					stamp.Identity = &identityResult{Schema: 1, Service: service, Root: project, Path: project, MatchedBy: identityByRoot, IndexedCommit: indexed}
				}
				writeSessionStamp(stamp)
			}
			file := project + "/entry.go"
			beat("edit-hook", "seen", firingMeta(map[string]string{"phase": "post"}, session, file))
			switch c.Fault {
			case "daemon-down":
				failure := wiringFaultDaemonError(t)
				beat("edit-hook", "error", firingMeta(map[string]string{"phase": "post", "reason": truncateBytes(failure.Error(), reasonMetaMaxBytes)}, session, file))
				beat("edit-hook", "answered", firingMeta(withDestination(silentAnswerMeta(map[string]string{"phase": "post"}, silenceDaemonError), project, file), session, file))
			case "rules-empty":
				beat("edit-hook", "skipped", firingMeta(map[string]string{"emitted": "false", "reason": "not-ready:no-instructions", "capability": string(harness.CapObligationsPostEdit)}, session, ""))
			default:
				beat("edit-hook", "answered", firingMeta(withDestination(map[string]string{"emitted": "true", "phase": "post", "claims": "0"}, project, file), session, file))
			}
			if c.Fault != "daemon-down" {
				count := 1
				if c.Fault == "rules-empty" {
					count = 0
				}
				calls.Write(wiringFaultCall(t, service, indexed, binary[:12], count, now()))
			}
		}
		write("hooks.jsonl", beats.Bytes())
		write("mcp-calls.jsonl", calls.Bytes())

		// Claude Code produces this external format. Its only fathom type
		// (scenariobench.streamEvent) is private and has no encoder. Keep
		// the external envelope typed and round-trip through that reader.
		init := wiringFaultInit{Type: "system", Subtype: "init", MCPServers: []wiringFaultMCPServer{{Name: "fathom", Status: "connected"}}, Tools: []string{"mcp__fathom__obligations"}}
		if c.Fault == "mcp-not-connected" {
			init.MCPServers, init.Tools = []wiringFaultMCPServer{}, []string{}
		}
		stream := wiringFaultJSON(t, init)
		if c.Fault != "hook-unregistered" {
			stream = append(stream, wiringFaultJSON(t, wiringFaultHookResponse{
				Type: "system", Subtype: "hook_response", HookEvent: "SessionStart", HookName: "SessionStart:startup",
			})...)
		}
		write("stream.jsonl", stream)
		parsed, err := scenariobench.ParseSessionStream(filepath.Join(root, "stream.jsonl"))
		if err != nil || parsed.MCPServers == nil || len(parsed.MCPServers) != len(init.MCPServers) || parsed.MCPTools != len(init.Tools) {
			t.Fatalf("stream init did not round-trip: servers=%v tools=%d err=%v", parsed.MCPServers, parsed.MCPTools, err)
		}
	}
	var manifestJSON bytes.Buffer
	if err := json.Indent(&manifestJSON, wiringFaultJSON(t, manifest), "", "  "); err != nil {
		t.Fatal(err)
	}
	wiringFaultWrite(t, dir, "manifest.json", manifestJSON.Bytes())
}

type wiringFaultMCPServer struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type wiringFaultInit struct {
	Type       string                 `json:"type"`
	Subtype    string                 `json:"subtype"`
	MCPServers []wiringFaultMCPServer `json:"mcp_servers"`
	Tools      []string               `json:"tools"`
}

type wiringFaultHookResponse struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	HookEvent string `json:"hook_event"`
	HookName  string `json:"hook_name"`
}

// Only obligations and citation metadata are used by this request.
// Embedding the unused port makes an unexpected extra query fail immediately.
type wiringFaultBackend struct {
	mcp.QueryInterface
	watermark string
	ruleCount int
}

func (b *wiringFaultBackend) Obligations(_ context.Context, service, file string, _ int) (semantic.Obligations, error) {
	rules := []semantic.Obligation{}
	if b.ruleCount > 0 {
		rules = append(rules, semantic.Obligation{Rule: "Sort outputs.", Src: "AGENTS.md#L1-L1", Tier: "doc-claimed"})
	}
	return semantic.Obligations{Service: service, File: file, Rules: rules, Watermark: b.watermark}, nil
}

func (*wiringFaultBackend) ObligationsIndexStale(context.Context, string, string) bool { return false }

func (b *wiringFaultBackend) ServiceMeta(_ context.Context, service string) (string, string, bool, error) {
	return "/repos/" + service, b.watermark, true, nil
}

func wiringFaultCall(t *testing.T, service, watermark, version string, count int, at time.Time) []byte {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "calls-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	srv := mcp.NewServerWithLogger(&wiringFaultBackend{watermark: watermark, ruleCount: count}, mcp.NewCallLogger(file))
	request := wiringFaultJSON(t, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "obligations", "arguments": map[string]any{
			"service": service, "file": "entry.go", "include_snippets": false,
		}},
	})
	response, ok := srv.HandleRequest(mcp.WithOrigin(context.Background(), mcp.OriginEditHook), request)
	var envelope struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(response, &envelope); err != nil || !ok || len(envelope.Error) != 0 {
		t.Fatalf("obligations call failed: response=%s ok=%v err=%v", response, ok, err)
	}
	raw, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]json.RawMessage
	if err := json.Unmarshal(raw, &row); err != nil {
		t.Fatal(err)
	}
	// The writer has no injected clock/build identity. Normalize only
	// those process-dependent values, preserving its fields and order.
	for _, field := range []struct {
		key   string
		value any
	}{{"ts", at.Format(time.RFC3339Nano)}, {"fathom_version", version}, {"latency_ms", 0}} {
		if _, ok := row[field.key]; !ok {
			t.Fatalf("writer omitted %s", field.key)
		}
		prefix := []byte(`"` + field.key + `":`)
		old := append(append([]byte{}, prefix...), row[field.key]...)
		next := append(append([]byte{}, prefix...), bytes.TrimSpace(wiringFaultJSON(t, field.value))...)
		raw = bytes.Replace(raw, old, next, 1)
	}
	return raw
}

func wiringFaultJSON(t *testing.T, value any) []byte {
	t.Helper()
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func wiringFaultWrite(t *testing.T, root, name string, raw []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func wiringFaultDaemonError(t *testing.T) error {
	t.Helper()
	previous := http.DefaultTransport
	http.DefaultTransport = testRunRoundTrip(func(*http.Request) (*http.Response, error) {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 80}, Err: &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED}}
	})
	defer func() { http.DefaultTransport = previous }()
	_, _, _, ok, err := fetchObligations("http://daemon.example.invalid", "service-001", "entry.go", 0, time.Second, "")
	if ok || err == nil {
		t.Fatalf("injected daemon failure: ok=%v err=%v", ok, err)
	}
	return err
}

// Real, deterministic commits make "older than HEAD" ancestry true by
// construction. The generated repository's machine path is never recorded.
func wiringFaultRevisions(t *testing.T, at time.Time) (parent, head string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required to generate wiring-fault revisions")
	}
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(context.Background(), "git", append([]string{"-c", "core.hooksPath=" + os.DevNull}, args...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_COUNT=0",
			"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
			"GIT_AUTHOR_DATE="+at.Format(time.RFC3339), "GIT_COMMITTER_DATE="+at.Format(time.RFC3339))
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "--initial-branch=main", "--object-format=sha1", ".")
	for _, revision := range []string{"first", "second"} {
		wiringFaultWrite(t, root, "entry.go", []byte("package fixture\n// "+revision+"\n"))
		git("add", "entry.go")
		args := []string{"commit-tree", git("write-tree"), "-m", revision}
		if head != "" {
			args = append(args, "-p", head)
		}
		parent, head = head, git(args...)
		git("update-ref", "HEAD", head)
	}
	return parent, head
}
