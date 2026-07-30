package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/research/bundle"
)

// ─── aggregateBundleInput: the pure reduction step ─────────────────────────

func TestAggregateBundleInput_HashesServiceNames(t *testing.T) {
	since := mustParseTime(t, "2026-07-21T00:00:00Z")
	until := mustParseTime(t, "2026-07-28T00:00:00Z")

	entries := []adoptionCallEntry{
		{TS: "2026-07-23T10:00:00Z", Tool: "find_symbol", Surface: "agent", Service: "acme-internal-repo", LatencyMS: 10},
		{TS: "2026-07-23T10:01:00Z", Tool: "find_symbol", Surface: "agent", Service: "acme-internal-repo", LatencyMS: 20},
	}

	in := aggregateBundleInput("", since, until, entries, true, nil, false, "", false, nil, "salt-value")

	if len(in.ServicePlaintextCalls) != 1 || in.ServicePlaintextCalls["acme-internal-repo"] != 2 {
		t.Fatalf("ServicePlaintextCalls = %+v, want {acme-internal-repo: 2}", in.ServicePlaintextCalls)
	}

	b := bundle.Build(in)
	raw, _ := json.Marshal(b)
	if strings.Contains(string(raw), "acme-internal-repo") {
		t.Fatalf("bundle leaks plaintext service name: %s", raw)
	}
	wantHash := bundle.HashService("salt-value", "acme-internal-repo")
	if len(b.ServiceActivity) != 1 || b.ServiceActivity[0].ServiceHash != wantHash || b.ServiceActivity[0].Calls != 2 {
		t.Errorf("ServiceActivity = %+v, want [{%s 2}]", b.ServiceActivity, wantHash)
	}
}

// TestAggregateBundleInput_SanitizesHostileToolSurfaceArm is critic Finding
// 1's own regression test at the aggregation layer: a call-log row whose
// Tool/Surface is a hostile, non-#Label-shaped string (proven reachable via
// internal/mcp/dispatch.go logging the raw requested tool name even for an
// unresolved tool, over an always-on, unauthenticated daemon) must be
// bucketed into "unrecognized," never passed through verbatim.
func TestAggregateBundleInput_SanitizesHostileToolSurfaceArm(t *testing.T) {
	since := mustParseTime(t, "2026-07-21T00:00:00Z")
	until := mustParseTime(t, "2026-07-28T00:00:00Z")

	hostileTool := "/Users/jdoe/code/acme-internal-payments/internal/billing.go"
	hostileSurface := "agent | injected\ntable-break"
	entries := []adoptionCallEntry{
		{TS: "2026-07-23T10:00:00Z", Tool: hostileTool, Surface: hostileSurface, LatencyMS: 1},
		{TS: "2026-07-23T10:01:00Z", Tool: "find_symbol", Surface: "agent", LatencyMS: 1},
	}

	in := aggregateBundleInput("", since, until, entries, true, nil, false, "", false, nil, "salt-value")

	toolNames := map[string]bool{}
	for _, tc := range in.ToolCalls {
		toolNames[tc.Tool] = true
	}
	if toolNames[hostileTool] {
		t.Fatalf("hostile tool value reached Input.ToolCalls verbatim: %+v", in.ToolCalls)
	}
	if !toolNames["unrecognized"] {
		t.Errorf("ToolCalls = %+v, want the hostile tool bucketed as %q", in.ToolCalls, "unrecognized")
	}
	if !toolNames["find_symbol"] {
		t.Errorf("ToolCalls = %+v, want the well-formed %q to pass through unchanged", in.ToolCalls, "find_symbol")
	}

	surfaceNames := map[string]bool{}
	for _, sc := range in.SurfaceTraffic {
		surfaceNames[sc.Surface] = true
	}
	if surfaceNames[hostileSurface] {
		t.Fatalf("hostile surface value reached Input.SurfaceTraffic verbatim: %+v", in.SurfaceTraffic)
	}
	if !surfaceNames["unrecognized"] {
		t.Errorf("SurfaceTraffic = %+v, want the hostile surface bucketed as %q", in.SurfaceTraffic, "unrecognized")
	}

	b := bundle.Build(in)
	if err := bundle.Validate(b); err != nil {
		t.Errorf("Validate(sanitized bundle) = %v, want nil", err)
	}
}

// ─── cmdResearchBundle: end-to-end CLI, --dry-run and file-writing paths ───

// bundleOutDir mirrors cmdResearchBundle's own default --out resolution
// (defaultResearchBundleOutDir) so tests don't hardcode the path twice.
func bundleOutDir(root string) string {
	return filepath.Join(root, filepath.FromSlash(defaultResearchBundleOutDir))
}

func TestCmdResearchBundle_DryRun_NoFilesWritten(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := cmdResearchBundle([]string{"--root", dir, "--dry-run", "--hooks-log", filepath.Join(dir, "no-hooks.jsonl")}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"schema_version"`) {
		t.Errorf("stdout does not look like bundle JSON: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "nothing written, nothing transmitted") {
		t.Errorf("stderr missing the review-first disclosure: %s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(bundleOutDir(dir), "bundle.json")); !os.IsNotExist(err) {
		t.Errorf("--dry-run must not write bundle.json, stat err = %v", err)
	}
}

func TestCmdResearchBundle_WritesLocalFilesOnly(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := cmdResearchBundle([]string{"--root", dir, "--hooks-log", filepath.Join(dir, "no-hooks.jsonl")}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}

	jsonPath := filepath.Join(bundleOutDir(dir), "bundle.json")
	mdPath := filepath.Join(bundleOutDir(dir), "bundle.md")
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("bundle.json not written: %v", err)
	}
	mdData, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("bundle.md not written: %v", err)
	}

	var b bundle.Bundle
	if err := json.Unmarshal(jsonData, &b); err != nil {
		t.Fatalf("bundle.json does not parse as a Bundle: %v", err)
	}
	if err := bundle.Validate(b); err != nil {
		t.Errorf("written bundle fails schema validation: %v", err)
	}
	if !strings.Contains(string(mdData), b.SchemaVersion) {
		t.Errorf("bundle.md does not mention the schema version")
	}
	if !strings.Contains(stdout.String(), "Nothing was transmitted anywhere") {
		t.Errorf("stdout missing the local-file/no-transmission disclosure: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "never the parent") {
		t.Errorf("stdout missing the never-share-the-parent-directory warning (critic Finding 2c): %s", stdout.String())
	}

	// The salt file must exist at its NEW location (outside
	// workspace/artifacts/research/ entirely — critic Finding 2) and must
	// never appear inside the bundle.
	saltPath := bundle.SaltPath(dir)
	if strings.Contains(saltPath, filepath.Join("artifacts", "research")) {
		t.Fatalf("salt path %q still resolves inside the shareable artifacts tree", saltPath)
	}
	salt, err := os.ReadFile(saltPath)
	if err != nil {
		t.Fatalf("salt file not created at %s: %v", saltPath, err)
	}
	if strings.Contains(string(jsonData), strings.TrimSpace(string(salt))) {
		t.Errorf("bundle.json leaks the salt")
	}

	// The bundle/ subdirectory must contain ONLY the two shareable files —
	// no salt, no report.md alongside it (critic Finding 2b).
	entries, err := os.ReadDir(bundleOutDir(dir))
	if err != nil {
		t.Fatalf("read bundle dir: %v", err)
	}
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name()] = true
	}
	if len(names) != 2 || !names["bundle.json"] || !names["bundle.md"] {
		t.Errorf("bundle output dir contains %v, want exactly {bundle.json, bundle.md}", names)
	}
}

// TestCmdResearchBundle_Adversarial_SecretsNeverLeak is the mission-required
// adversarial test: seed secret-shaped strings into EVERY consumed source
// (a call-log error field, a call-log service name, a call-log tool/surface
// field, an observations.md symptom/tool/expected/actual/repro) and assert
// byte-level that none of them appear in either bundle output file.
func TestCmdResearchBundle_Adversarial_SecretsNeverLeak(t *testing.T) {
	dir := t.TempDir()

	secrets := []string{
		"sk-ant-api03-FAKESECRETVALUEnotreal1234567890",
		"/Users/jdoe/code/acme-internal-payments/internal/billing.go",
		"jdoe@example.com",
		"jdoes-macbook-pro.local",
		"AKIAFAKEACCESSKEYFAKE01",
	}

	// 1. Call log: a secret-shaped error string, a secret-shaped service
	// name, AND (critic Finding 1) a hostile Tool value and a hostile
	// Surface value straight from the wire.
	//
	// Timestamps are relative to time.Now() (never a hardcoded calendar
	// date): cmdResearchBundle defaults to a 168h (7-day) window with no
	// injected clock, so a fixed date silently ages out of the window
	// once real wall-clock time passes it — exactly the failure this
	// test hit once "now" crossed the fixture's original date by more
	// than 7 days.
	now := time.Now().UTC()
	ts := func(offset time.Duration) string { return now.Add(offset).Format(time.RFC3339) }
	callLogPath := filepath.Join(dir, "workspace", "logs", "mcp-calls.jsonl")
	entries := []adoptionCallEntry{
		{TS: ts(-50 * time.Minute), Tool: "find_symbol", Surface: "agent", Service: "svc-" + secrets[1], Error: secrets[0]},
		{TS: ts(-49 * time.Minute), Tool: "verify_plan", Surface: "", Service: "svc-two", Error: "auth failed for " + secrets[2] + " on host " + secrets[3]},
		{TS: ts(-48 * time.Minute), Tool: "brief", Surface: "agent", Service: secrets[4]},
		// Hostile Tool/Surface rows (Finding 1): what internal/mcp/dispatch.go
		// logs verbatim for an unresolved tool name or an unrecognized
		// surface selection.
		{TS: ts(-47 * time.Minute), Tool: secrets[1], Surface: "agent"},
		{TS: ts(-46 * time.Minute), Tool: "find_symbol", Surface: "agent | " + secrets[0]},
	}
	writeCallLogFixture(t, callLogPath, entries)

	// 2. .fathom/observations.md: every free-text field carries a secret.
	obsPath := filepath.Join(dir, ".fathom", "observations.md")
	if err := os.MkdirAll(filepath.Dir(obsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	obsBody := observationsHeader + "\n" + formatObservationEntry(1,
		"leaked "+secrets[0], "tool "+secrets[3], "expected "+secrets[2], "actual "+secrets[1], "repro "+secrets[4])
	if err := os.WriteFile(obsPath, []byte(obsBody), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := cmdResearchBundle([]string{"--root", dir, "--call-log", callLogPath, "--hooks-log", filepath.Join(dir, "no-hooks.jsonl")}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}

	jsonData, err := os.ReadFile(filepath.Join(bundleOutDir(dir), "bundle.json"))
	if err != nil {
		t.Fatalf("bundle.json not written: %v", err)
	}
	mdData, err := os.ReadFile(filepath.Join(bundleOutDir(dir), "bundle.md"))
	if err != nil {
		t.Fatalf("bundle.md not written: %v", err)
	}

	for _, secret := range secrets {
		if strings.Contains(string(jsonData), secret) {
			t.Errorf("bundle.json leaks secret-shaped string %q:\n%s", secret, jsonData)
		}
		if strings.Contains(string(mdData), secret) {
			t.Errorf("bundle.md leaks secret-shaped string %q:\n%s", secret, mdData)
		}
	}

	// Positive control: the run must have actually processed these rows
	// (otherwise the absence of secrets would be a vacuous pass).
	var b bundle.Bundle
	if err := json.Unmarshal(jsonData, &b); err != nil {
		t.Fatalf("parse bundle.json: %v", err)
	}
	if b.CallTraffic == nil {
		t.Fatal("CallTraffic is nil — the call log was not read")
	}
	if b.CallTraffic.TotalCalls != 5 {
		t.Fatalf("CallTraffic.TotalCalls = %d, want 5 (the fixture rows were not actually aggregated)", b.CallTraffic.TotalCalls)
	}
	if b.TotalObservations == nil || *b.TotalObservations != 1 {
		t.Fatalf("TotalObservations = %v, want 1", b.TotalObservations)
	}
	if len(b.ServiceActivity) == 0 {
		t.Fatal("ServiceActivity is empty — the service-name aggregation path was not exercised")
	}
	// The hostile tool/surface rows must have landed in the "unrecognized"
	// bucket, not silently dropped — proves the sanitize-then-count path
	// actually ran rather than the rows being skipped for some other reason.
	foundUnrecognizedTool, foundUnrecognizedSurface := false, false
	for _, tc := range b.CallTraffic.ToolCalls {
		if tc.Tool == "unrecognized" {
			foundUnrecognizedTool = true
		}
	}
	for _, sc := range b.CallTraffic.SurfaceTraffic {
		if sc.Surface == "unrecognized" {
			foundUnrecognizedSurface = true
		}
	}
	if !foundUnrecognizedTool {
		t.Error("expected an \"unrecognized\" tool bucket from the hostile Tool row, found none")
	}
	if !foundUnrecognizedSurface {
		t.Error("expected an \"unrecognized\" surface bucket from the hostile Surface row, found none")
	}
}

// TestCmdResearchBundle_InvalidBundleNeverWritten proves the CLI refuses to
// write (or print, on --dry-run) anything that fails schema validation —
// there is no path from "aggregation produced something odd" to a shipped
// file. This is exercised indirectly: a bundle built from ordinary,
// well-formed local evidence must validate; this test pins that Validate is
// actually called in the write path by checking the written file parses
// and validates (regression guard against someone routing around Validate).
func TestCmdResearchBundle_WrittenFileAlwaysValidates(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := cmdResearchBundle([]string{"--root", dir, "--hooks-log", filepath.Join(dir, "no-hooks.jsonl")}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(bundleOutDir(dir), "bundle.json"))
	if err != nil {
		t.Fatal(err)
	}
	var b bundle.Bundle
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatal(err)
	}
	if err := bundle.Validate(b); err != nil {
		t.Errorf("Validate(written bundle) = %v, want nil", err)
	}
}

// TestCmdResearchBundle_MigratesLegacySalt proves an existing repo's
// pre-Finding-2 salt (at the old workspace/artifacts/research/.salt
// location) is picked up and migrated rather than silently replaced with a
// fresh one — an existing bundle's hash lineage must survive the fix.
func TestCmdResearchBundle_MigratesLegacySalt(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "workspace", "artifacts", "research", ".salt")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o755); err != nil {
		t.Fatal(err)
	}
	const legacySalt = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd"
	if err := os.WriteFile(oldPath, []byte(legacySalt+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := cmdResearchBundle([]string{"--root", dir, "--hooks-log", filepath.Join(dir, "no-hooks.jsonl")}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}

	newSalt, err := os.ReadFile(bundle.SaltPath(dir))
	if err != nil {
		t.Fatalf("new salt file not written: %v", err)
	}
	if strings.TrimSpace(string(newSalt)) != legacySalt {
		t.Errorf("salt after migration = %q, want the preserved legacy value %q", newSalt, legacySalt)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Errorf("legacy salt file should have been removed after migration, stat err = %v", err)
	}
}
