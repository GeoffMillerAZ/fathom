package main

// json_envelopes_test.go — RP-14..RP-17 (docs/plans/reach-parity.md): the
// `--json` envelopes the compatibility surface promises, each parsed back
// and checked for the fields CI and non-Claude harnesses read.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIngestJSONEnvelope(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Hello() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(t.TempDir(), "s.sqlite")
	var stdout, stderr bytes.Buffer
	code := cmdIngest([]string{"--service", "jsonprobe", "--path", root, "--store", store, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s %s", code, stdout.String(), stderr.String())
	}
	var env ingestJSON
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout.String())
	}
	if env.Service != "jsonprobe" || env.Outcome != "ok" || env.ExitCode != 0 || env.Discovered < 1 || env.ReadFailed != 0 || env.ReadFailures == nil {
		t.Errorf("envelope = %+v", env)
	}
	// The errno class is derived from the connector's text, never guessed.
	if errnoClass("open x: operation not permitted") != "EPERM" || errnoClass("weird") != "" {
		t.Error("errnoClass")
	}
}

func TestInitJSONEnvelope(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	srv := fakeInitDaemon(t, strings.ToLower(filepath.Base(dir)), 5)
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := cmdInit([]string{"--path", dir, "--url", srv.URL, "--bin", testBinPath, "--skills=agents", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s %s", code, stdout.String(), stderr.String())
	}
	var env initJSON
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout.String())
	}
	if env.Service == "" || env.Root != dir || env.ExitCode != 0 {
		t.Errorf("identity = %+v", env)
	}
	var sawStanza, sawSkill bool
	for _, w := range env.Writes {
		if w.Path == "CLAUDE.md" {
			sawStanza = true
		}
		if strings.HasSuffix(w.Path, filepath.Join(".agents", "skills", "fathom-plan", "SKILL.md")) && w.Action == "created" {
			sawSkill = true
		}
	}
	if !sawStanza || !sawSkill {
		t.Errorf("writes must name the stanza and the skills: %+v", env.Writes)
	}
	if env.Receipt == nil || env.Receipt.ID == "" || len(env.Receipt.Files) == 0 {
		t.Errorf("receipt = %+v", env.Receipt)
	}
	var sawMCPAdd bool
	for _, s := range env.ManualSteps {
		if strings.HasPrefix(s.Command, "claude mcp add") {
			sawMCPAdd = true
		}
	}
	if !sawMCPAdd || len(env.ManualSteps) == 0 {
		t.Errorf("manual steps must carry the mcp add line as a command: %+v", env.ManualSteps)
	}
}

func TestThreadJSONEnvelopes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := cmdThreadList([]string{"--repo", repo, "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("list exit %d: %s", code, stderr.String())
	}
	var list threadListJSON
	if err := json.Unmarshal(stdout.Bytes(), &list); err != nil || list.Kind != "list" || list.Items == nil {
		t.Fatalf("list envelope: %v %+v\n%s", err, list, stdout.String())
	}

	stdout.Reset()
	if code := cmdThreadBrief([]string{"--repo", repo, "--source", "startup", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("brief exit %d: %s", code, stderr.String())
	}
	var brief threadBriefJSON
	if err := json.Unmarshal(stdout.Bytes(), &brief); err != nil || brief.Kind != "brief" || brief.Source != "startup" || brief.Bytes != len(brief.Text) {
		t.Fatalf("brief envelope: %v %+v", err, brief)
	}
	stdout.Reset()
	if code := cmdThreadBrief([]string{"--repo", repo, "--source", "resume", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("brief(resume) exit %d", code)
	}
	if err := json.Unmarshal(stdout.Bytes(), &brief); err != nil || brief.Text != "" || brief.Source != "resume" {
		t.Fatalf("resume must still be an envelope with empty text: %v %+v", err, brief)
	}
}

func TestDaemonStatusJSONEnvelope(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := cmdDaemonStatus([]string{"--url", "http://127.0.0.1:1", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("an unreachable daemon exits 1, got %d: %s", code, stdout.String())
	}
	var st daemonStatusJSON
	if err := json.Unmarshal(stdout.Bytes(), &st); err != nil || st.Reachable || st.ExitCode != 1 || st.Remedy == "" || st.Platform == "" {
		t.Fatalf("envelope: %v %+v", err, st)
	}
}

func TestLayersListJSONEnvelope(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := cmdLayersList([]string{"--root", t.TempDir(), "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var env layersListJSON
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout.String())
	}
	if env.StoreRead || len(env.Layers) == 0 {
		t.Fatalf("envelope = %+v", env)
	}
	var sawObligations bool
	for _, l := range env.Layers {
		if l.Name == "obligations" && l.Source == "embedded" && l.Claims == nil {
			sawObligations = true
		}
	}
	if !sawObligations {
		t.Errorf("the embedded obligations pack must be listed without store truth: %+v", env.Layers)
	}
	for i := 1; i < len(env.Layers); i++ {
		if env.Layers[i-1].Name > env.Layers[i].Name {
			t.Errorf("layers must be sorted by name")
		}
	}
}
