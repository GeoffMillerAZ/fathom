package main

import (
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/qualityplan"
	"github.com/geoffmilleraz/app-kit/internal/twplus/index"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestInstalledMCPProbe is an explicit, black-box check of the exact
// appkit-mcp executable a host would launch. Ordinary tests never discover or
// launch an installed binary. Licensed sources require a second opt-in and no
// source body is written to the test log.
func TestInstalledMCPProbe(t *testing.T) {
	binary := os.Getenv("APPKIT_PROBE_BINARY")
	root := os.Getenv("APPKIT_PROBE_ROOT")
	if binary == "" && root == "" {
		t.Skip("opt-in installed MCP probe: set absolute APPKIT_PROBE_BINARY and APPKIT_PROBE_ROOT paths")
	}
	if binary == "" || root == "" {
		t.Fatal("APPKIT_PROBE_BINARY and APPKIT_PROBE_ROOT must be set together")
	}
	if !filepath.IsAbs(binary) || !filepath.IsAbs(root) {
		t.Fatal("APPKIT_PROBE_BINARY and APPKIT_PROBE_ROOT must both be absolute paths")
	}
	if info, err := os.Stat(binary); err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		t.Fatal("APPKIT_PROBE_BINARY must name an existing executable regular file")
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		t.Fatal("APPKIT_PROBE_ROOT must name an existing directory")
	}

	binarySHA := installedProbeFileSHA256(t, binary)
	moduleVersion, vcsRevision, vcsModified := installedProbeBuildIdentity(binary)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "-kit-root", root)
	cmd.Dir = t.TempDir()
	cmd.Env = installedProbeEnvironment()
	cmd.Stderr = io.Discard
	client := mcp.NewClient(&mcp.Implementation{Name: "appkit-installed-stdio-probe", Version: "test"}, nil)
	timings := make([]installedProbeTiming, 0, 16)
	started := time.Now()
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd, TerminateDuration: time.Second}, nil)
	timings = append(timings, installedProbeTiming{Name: "initialize", Duration: time.Since(started)})
	if err != nil {
		t.Fatal("installed appkit-mcp initialize failed; subprocess diagnostics intentionally omitted")
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			if err := session.Close(); err != nil {
				t.Errorf("installed appkit-mcp cleanup failed")
			}
		}
	})

	init := session.InitializeResult()
	if init == nil || init.ProtocolVersion == "" || init.ServerInfo == nil || init.ServerInfo.Name != "appkit" || init.ServerInfo.Version == "" {
		t.Fatal("installed appkit-mcp returned incomplete initialize identity")
	}
	if !strings.Contains(init.Instructions, "consumer's intent") || !strings.Contains(init.Instructions, "search_capabilities") {
		t.Fatal("installed appkit-mcp initialize instructions omit required adoption routing")
	}

	started = time.Now()
	listed, err := session.ListTools(ctx, nil)
	timings = append(timings, installedProbeTiming{Name: "list_tools", Duration: time.Since(started)})
	if err != nil || listed == nil {
		t.Fatal("installed appkit-mcp tool discovery failed")
	}
	tools := make(map[string]bool, len(listed.Tools))
	for _, tool := range listed.Tools {
		tools[tool.Name] = true
	}
	for _, name := range []string{"get_kit_guidance", "get_kit_decision", "search_capabilities", "get_capability", "list_quality_work", "get_quality_work"} {
		if !tools[name] {
			t.Fatalf("installed appkit-mcp is missing required tool %q", name)
		}
	}

	guidance := installedProbeCallText(t, ctx, session, &timings, "get_kit_guidance", map[string]any{})
	if !strings.Contains(guidance, "get_kit_decision(\"intent\")") {
		t.Fatal("installed guidance does not route to the intent document")
	}

	wantIntent, err := os.ReadFile(filepath.Join(root, "docs", "intent", "README.md"))
	if err != nil || len(wantIntent) == 0 {
		t.Fatal("cannot read the selected checkout's intent router")
	}
	decision := installedProbeCallText(t, ctx, session, &timings, "get_kit_decision", map[string]any{"id": "intent"})
	started = time.Now()
	resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "kit://intent"})
	timings = append(timings, installedProbeTiming{Name: "read_resource:kit://intent", Duration: time.Since(started)})
	if err != nil || resource == nil || len(resource.Contents) != 1 {
		t.Fatal("installed appkit-mcp intent resource read failed")
	}
	if decision != string(wantIntent) || resource.Contents[0].Text != string(wantIntent) {
		t.Fatal("installed decision/resource intent bytes differ from APPKIT_PROBE_ROOT")
	}
	intentSHA := sha256.Sum256(wantIntent)

	var page catalog.Page
	installedProbeCallJSON(t, ctx, session, &timings, "search_capabilities", map[string]any{"limit": 2}, &page)
	if page.Revision == "" || page.Total < 2 || len(page.Items) != 2 {
		t.Fatal("installed capability search did not return the requested bounded page")
	}
	const capabilityID = "foundation.tokens"
	var capability capabilityOutput
	installedProbeCallJSON(t, ctx, session, &timings, "get_capability", map[string]any{"id": capabilityID}, &capability)
	if capability.Revision != page.Revision || capability.Entry.ID != capabilityID {
		t.Fatal("installed exact capability retrieval disagrees with the catalog revision")
	}

	var quality qualityplan.ListResult
	installedProbeCallJSON(t, ctx, session, &timings, "list_quality_work", map[string]any{
		"capability_id": "analytics.comparison", "scope": "core",
	}, &quality)
	if quality.Revision == "" || len(quality.Items) != 1 {
		t.Fatal("installed quality lookup did not return the expected one-item bounded selection")
	}
	var qualityDetail qualityplan.DetailResult
	installedProbeCallJSON(t, ctx, session, &timings, "get_quality_work", map[string]any{
		"id": quality.Items[0].ID, "expected_revision": quality.Revision,
	}, &qualityDetail)
	if qualityDetail.Revision != quality.Revision || qualityDetail.Item.ID != quality.Items[0].ID {
		t.Fatal("installed exact quality retrieval disagrees with its list revision")
	}

	started = time.Now()
	invalid, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_capability", Arguments: map[string]any{"id": "not.a.real.capability"},
	})
	timings = append(timings, installedProbeTiming{Name: "get_capability:invalid", Duration: time.Since(started)})
	if err != nil || invalid == nil || !invalid.IsError {
		t.Fatal("installed appkit-mcp did not return an MCP tool error for an invalid capability")
	}
	var recovered capabilityOutput
	installedProbeCallJSON(t, ctx, session, &timings, "get_capability", map[string]any{"id": capabilityID}, &recovered)
	if recovered.Revision != capability.Revision || recovered.Entry.ID != capabilityID {
		t.Fatal("installed appkit-mcp did not recover after a tool error")
	}

	if os.Getenv("APPKIT_PROBE_LICENSED") == "1" {
		installedProbeLicensedComponent(t, ctx, session, &timings, "tailwindplus", filepath.Join(root, "data", "tailwindplus", "index", "components.sqlite"))
		installedProbeLicensedComponent(t, ctx, session, &timings, "matdash", filepath.Join(root, "data", "matdash", "index", "components.sqlite"))
		assetCatalog, err := designassets.Load(filepath.Join(root, "data", "design-assets"))
		if err != nil {
			t.Fatal("selected checkout's private design-asset catalog failed validation; source details omitted")
		}
		archiveCount := len(assetCatalog.List())
		var assets designassets.Page
		installedProbeCallJSON(t, ctx, session, &timings, "search_design_assets", designassets.Query{Limit: 1}, &assets)
		if assets.Revision == "" || archiveCount < 1 || assets.Total != archiveCount || len(assets.Items) != 1 || assets.Items[0].ID == "" {
			t.Fatal("installed private design-asset discovery disagrees with the selected checkout's metadata inventory")
		}
	}

	if ctx.Err() != nil {
		t.Fatal("installed appkit-mcp probe exceeded its 60-second bound")
	}
	started = time.Now()
	err = session.Close()
	timings = append(timings, installedProbeTiming{Name: "close", Duration: time.Since(started)})
	closed = true
	if err != nil {
		t.Fatal("installed appkit-mcp did not shut down cleanly")
	}
	t.Logf("installed_mcp binary_sha256=%s module_version=%s vcs_revision=%s vcs_modified=%t server_version=%s protocol=%s tools=%d catalog_revision=%s quality_revision=%s intent_sha256=%s licensed=%t",
		binarySHA, moduleVersion, vcsRevision, vcsModified, init.ServerInfo.Version, init.ProtocolVersion, len(listed.Tools), page.Revision, quality.Revision, hex.EncodeToString(intentSHA[:]), os.Getenv("APPKIT_PROBE_LICENSED") == "1")
	for _, timing := range timings {
		t.Logf("installed_mcp_call name=%s duration=%s response_bytes=%d response_sha256=%s", timing.Name, timing.Duration, timing.Bytes, timing.SHA256)
	}
}

type installedProbeTiming struct {
	Name     string
	Duration time.Duration
	Bytes    int
	SHA256   string
}

func installedProbeEnvironment() []string {
	env := make([]string, 0, 2)
	for _, key := range []string{"PATH", "HOME"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func TestInstalledProbeEnvironmentAllowlist(t *testing.T) {
	t.Setenv("PATH", "/probe/bin")
	t.Setenv("HOME", "/probe/home")
	t.Setenv("APPKIT_ROOT", "/must/not/pass")
	t.Setenv("APPKIT_TWPLUS_INDEX", "/must/not/pass.sqlite")
	want := []string{"PATH=/probe/bin", "HOME=/probe/home"}
	if got := installedProbeEnvironment(); !reflect.DeepEqual(got, want) {
		t.Fatalf("installed probe environment = %q, want strict allowlist %q", got, want)
	}
}

func installedProbeCall(t *testing.T, ctx context.Context, session *mcp.ClientSession, timings *[]installedProbeTiming, name string, args any) *mcp.CallToolResult {
	t.Helper()
	started := time.Now()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	elapsed := time.Since(started)
	if err != nil || result == nil || result.IsError || len(result.Content) != 1 {
		t.Fatalf("installed MCP tool %s failed; response intentionally omitted", name)
	}
	content, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("installed MCP tool %s did not return one text block", name)
	}
	sum := sha256.Sum256([]byte(content.Text))
	*timings = append(*timings, installedProbeTiming{Name: name, Duration: elapsed, Bytes: len(content.Text), SHA256: hex.EncodeToString(sum[:])})
	return result

}

func installedProbeCallText(t *testing.T, ctx context.Context, session *mcp.ClientSession, timings *[]installedProbeTiming, name string, args any) string {
	t.Helper()
	result := installedProbeCall(t, ctx, session, timings, name, args)
	return result.Content[0].(*mcp.TextContent).Text
}

func installedProbeCallJSON(t *testing.T, ctx context.Context, session *mcp.ClientSession, timings *[]installedProbeTiming, name string, args, out any) {
	t.Helper()
	result := installedProbeCall(t, ctx, session, timings, name, args)
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || string(body) == "null" || json.Unmarshal(body, out) != nil {
		t.Fatalf("installed MCP tool %s returned invalid structured content", name)
	}
}

func installedProbeLicensedComponent(t *testing.T, ctx context.Context, session *mcp.ClientSession, timings *[]installedProbeTiming, source, localPath string) {
	t.Helper()
	var found searchOutput
	installedProbeCallJSON(t, ctx, session, timings, "search_components", searchInput{Source: source, Limit: 1}, &found)
	if found.Count < 1 || len(found.Results) != 1 {
		t.Fatalf("installed %s search did not return one real reference", source)
	}
	hit := found.Results[0]
	if hit.FullName == "" || hit.Framework == "" || hit.TailwindVersion == 0 || hit.Mode == "" {
		t.Fatalf("installed %s search omitted reference axes", source)
	}
	var got index.GetResult
	installedProbeCallJSON(t, ctx, session, timings, "get_component", getInput{
		FullName: hit.FullName, Framework: hit.Framework, TailwindVersion: hit.TailwindVersion, Mode: hit.Mode, Source: source,
	}, &got)
	if got.FullName != hit.FullName || got.Framework != hit.Framework || got.TailwindVersion != hit.TailwindVersion || got.Mode != hit.Mode || got.Code == "" {
		t.Fatalf("installed %s exact reference omitted code or disagreed with search axes", source)
	}
	local, err := index.Open(localPath)
	if err != nil {
		t.Fatalf("open local %s index for byte comparison: %v", source, err)
	}
	defer local.Close()
	want, err := local.GetComponent(hit.FullName, hit.Framework, hit.TailwindVersion, hit.Mode)
	if err != nil || !reflect.DeepEqual(got, *want) {
		t.Fatalf("installed %s source differs from APPKIT_PROBE_ROOT index; source bodies omitted", source)
	}
}

func installedProbeFileSHA256(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal("open installed binary for hashing failed")
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatal("hash installed binary failed")
	}
	return hex.EncodeToString(h.Sum(nil))
}

func installedProbeBuildIdentity(path string) (moduleVersion, vcsRevision string, vcsModified bool) {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return "unknown", "unknown", false
	}
	moduleVersion = info.Main.Version
	if moduleVersion == "" {
		moduleVersion = "unknown"
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			vcsRevision = setting.Value
		case "vcs.modified":
			vcsModified = setting.Value == "true"
		}
	}
	if vcsRevision == "" {
		vcsRevision = "unknown"
	}
	return moduleVersion, vcsRevision, vcsModified
}
