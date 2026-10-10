package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/config"
)

// fakeClaudeCLIHealthyPingScript reproduces claudecli.Client.Ping's HEALTHY
// outcome (agentic-kit foundations/llmclient/claudecli/ping.go): empty
// stdin, no prompt argument → the CLI rejects locally with this exact
// stderr text and exit 1, before any network activity. The
// compile_test.go fakeClaudeCLIScript instead always succeeds (it targets
// Complete, not Ping), so providers.go's Ping-driven health check needs
// this distinct fake.
const fakeClaudeCLIHealthyPingScript = `#!/bin/sh
cat > /dev/null
printf 'Input must be provided either through stdin or as a prompt argument' >&2
exit 1
`

func newFakeClaudeCLIForPing(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(path, []byte(fakeClaudeCLIHealthyPingScript), 0o755); err != nil {
		t.Fatalf("write fake claude ping script: %v", err)
	}
	return path
}

func TestCmdProviders_NoConfig_EnvOverlayOnly_OpenAI(t *testing.T) {
	withIsolatedConfigHome(t)
	var gotPath, gotQuery string
	srv := newFakeOpenAIServer(t, &gotPath, &gotQuery)
	t.Setenv("FATHOM_LLM_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("FATHOM_OPENAI_BASE_URL", srv.URL+"/v1")

	var stdout, stderr bytes.Buffer
	code := cmdProviders(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "(env-overlay)") {
		t.Errorf("output = %s, want the env-overlay row label", stdout.String())
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Errorf("output = %s, want an ok status", stdout.String())
	}
}

func TestCmdProviders_MultipleProfiles_MixedHealth(t *testing.T) {
	withIsolatedConfigHome(t)
	healthyBin := newFakeCodexCLI(t, fakeCodexCLIHealthyScript)
	degradedBin := newFakeCodexCLI(t, fakeCodexCLIDegradedScript)

	// A profile's bin is user-only (SEC-2): the user config names it.
	writeUserConfig(t, `
[profiles.good]
provider = "codex-cli"
bin = "`+escapeTOML(healthyBin)+`"

[profiles.bad]
provider = "codex-cli"
bin = "`+escapeTOML(degradedBin)+`"
`)

	var stdout, stderr bytes.Buffer
	code := cmdProviders([]string{"--json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected nonzero exit (one lane is degraded); stdout=%s", stdout.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %v", len(rows), rows)
	}
	byName := map[string]map[string]any{}
	for _, r := range rows {
		byName[r["profile"].(string)] = r
	}
	if byName["good"]["status"] != "ok" {
		t.Errorf("good status = %v, want ok", byName["good"]["status"])
	}
	if byName["bad"]["status"] != "error" {
		t.Errorf("bad status = %v, want error", byName["bad"]["status"])
	}
}

func TestCmdProviders_SingleProfileFlag(t *testing.T) {
	withIsolatedConfigHome(t)
	pingBin := newFakeClaudeCLIForPing(t)
	// A profile's bin is user-only (SEC-2): the user config names it.
	writeUserConfig(t, `
[profiles.only]
provider = "claude-cli"
bin = "`+escapeTOML(pingBin)+`"

[profiles.other]
provider = "codex-cli"
bin = "`+escapeTOML(newFakeCodexCLI(t, fakeCodexCLIHealthyScript))+`"
`)
	var stdout, stderr bytes.Buffer
	code := cmdProviders([]string{"--profile", "only"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "other") {
		t.Errorf("output = %s, want only the 'only' profile probed", stdout.String())
	}
	if !strings.Contains(stdout.String(), "only") || !strings.Contains(stdout.String(), "ok") {
		t.Errorf("output = %s, want profile 'only' reported ok", stdout.String())
	}
}

func TestCmdProviders_UnknownProfile_Errors(t *testing.T) {
	withIsolatedConfigHome(t)
	var stdout, stderr bytes.Buffer
	code := cmdProviders([]string{"--profile", "nope"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected nonzero exit for an unknown profile")
	}
}

func TestCmdProviders_ConstructionFailure_ReportedAsError(t *testing.T) {
	_, projectDir := withIsolatedConfigHome(t)
	writeProjectConfig(t, projectDir, `
[profiles.broken]
provider = "azure-openai"
`)
	var stdout, stderr bytes.Buffer
	code := cmdProviders([]string{"--profile", "broken"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected nonzero exit for a profile that fails to construct")
	}
	if !strings.Contains(stdout.String(), "error") {
		t.Errorf("output = %s, want status error", stdout.String())
	}
}

// escapeTOML escapes backslashes for a TOML basic string literal — macOS
// temp paths are already forward-slash, so this is a defensive no-op today
// but keeps the test helper correct if that ever changes.
func escapeTOML(s string) string {
	return strings.ReplaceAll(s, `\`, `\\`)
}

// nonPingerClient implements llmclient.LLMClient but deliberately NOT
// llmclient.Pinger — the stand-in for a future adapter without Ping.
type nonPingerClient struct{}

func (nonPingerClient) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	return llmclient.Response{Text: "pong"}, nil
}

func (nonPingerClient) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	ch := make(chan llmclient.StreamChunk)
	close(ch)
	return ch, nil
}

var _ llmclient.LLMClient = nonPingerClient{}

// TestProbeProfiles_NotProbeable_ReportedHonestly pins the "not probeable"
// branch (N6): today every real adapter implements Pinger, so the branch is
// unreachable through buildLLMClientForProfile — a stub builder returning a
// Ping-less client exercises the reporting contract directly: the profile
// is STILL LISTED (never silently dropped), with an honest status that is
// informational, not a failure.
func TestProbeProfiles_NotProbeable_ReportedHonestly(t *testing.T) {
	clearProfileEnv(t)
	loaded := config.Loaded{Config: config.Config{Profiles: map[string]config.Profile{
		"pingless": {Provider: config.ProviderAnthropic},
	}}}

	stub := func(p config.Profile) (llmclient.LLMClient, error) {
		return nonPingerClient{}, nil
	}
	rows := probeProfiles(loaded, []string{"pingless"}, stub)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1 — a not-probeable profile must never be dropped", len(rows))
	}
	r := rows[0]
	if r.Status != providerStatusNotProbeable {
		t.Errorf("Status = %q, want %q", r.Status, providerStatusNotProbeable)
	}
	if !strings.Contains(r.Detail, "does not implement Ping") {
		t.Errorf("Detail = %q, want it to explain the missing Ping", r.Detail)
	}
}

// TestCmdProviders_NotProbeableIsNotAFailure drives the FULL command (via
// the providerClientBuilder seam) with a Ping-less client: exit 0, because
// "not probeable" is a capability gap, not a broken credential — and the
// row still appears in the output.
func TestCmdProviders_NotProbeableIsNotAFailure(t *testing.T) {
	_, projectDir := withIsolatedConfigHome(t)
	writeProjectConfig(t, projectDir, `
[profiles.pingless]
provider = "anthropic"
`)
	orig := providerClientBuilder
	providerClientBuilder = func(p config.Profile) (llmclient.LLMClient, error) {
		return nonPingerClient{}, nil
	}
	t.Cleanup(func() { providerClientBuilder = orig })

	var stdout, stderr bytes.Buffer
	code := cmdProviders([]string{"--profile", "pingless"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 — not-probeable alone must not fail the command; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), string(providerStatusNotProbeable)) {
		t.Errorf("output = %s, want the not-probeable status printed", stdout.String())
	}
}
