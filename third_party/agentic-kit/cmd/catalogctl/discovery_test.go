package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/catalog-kit"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestOutputFailureIsInability(t *testing.T) {
	var diagnostics bytes.Buffer
	if code := execute([]string{"help"}, failingWriter{}, &diagnostics); code != 2 || !strings.Contains(diagnostics.String(), "broken pipe") {
		t.Fatalf("bad I/O classification: %d %s", code, &diagnostics)
	}
	if _, ok := writeBundle(&bytes.Buffer{}, catalogkit.Bundle{}, "typo").(usageError); !ok {
		t.Fatal("unknown format is not a usage error")
	}
}

func TestPrepareCommandBindsCurrentDefinition(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.json")
	if err := os.WriteFile(specPath, []byte(`{"version":1,"layers":[{"name":"base","nodes":[{"slug":"criterion","source":"criterion.cue","stable_key":"criterion"}]}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run([]string{"resolve", "-spec", specPath}, &out); err != nil {
		t.Fatal(err)
	}
	bundle, err := catalogkit.DecodeBundle(out.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	bundlePath := filepath.Join(dir, "bundle.json")
	if err := os.WriteFile(bundlePath, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	node, _ := bundle.Catalog.Get("criterion")
	definition := []byte("Required passages must cite captured evidence.")
	hash := func(p []byte) string { h := sha256.Sum256(p); return hex.EncodeToString(h[:]) }
	bind := catalogkit.Binding{NodeID: node.ID(), NodeDigest: catalogkit.NodeDigest(node), DefinitionDigest: hash(definition), Version: "1", Question: decision.Question{ID: "criterion", Kind: decision.KindPredicate, Instructions: "Does this cite evidence?", Options: []decision.Option{}, Levels: []string{}}, RequiredEvidence: []string{"passage"}}
	witness := catalogkit.Evidence{ID: "passage", Source: "report.md", Revision: "fixture", Text: "Evidence: run 1", Digest: hash([]byte("Evidence: run 1"))}
	bindPath, definitionPath, evidencePath := filepath.Join(dir, "binding.json"), filepath.Join(dir, "definition.txt"), filepath.Join(dir, "evidence.json")
	bindJSON, _ := json.Marshal(bind)
	evidenceJSON, _ := json.Marshal([]catalogkit.Evidence{witness})
	for path, body := range map[string][]byte{bindPath: bindJSON, definitionPath: definition, evidencePath: evidenceJSON} {
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"prepare", "-bundle", bundlePath, "-binding", bindPath, "-definition", definitionPath, "-evidence", evidencePath}
	out.Reset()
	if err := run(args, &out); err != nil {
		t.Fatal(err)
	}
	var prepared catalogkit.Prepared
	if err := json.Unmarshal(out.Bytes(), &prepared); err != nil || prepared.DefinitionDigest != hash(definition) || !strings.Contains(prepared.Request.State, string(definition)) {
		t.Fatalf("unbound CLI packet: %s %v", &out, err)
	}
	identityPath := filepath.Join(dir, "identity.json")
	identity := catalogkit.DecisionIdentity{Provider: "fixture", RequestedModel: "requested", ResolvedModel: "resolved", AdapterVersion: "1", PolicyVersion: "1"}
	identityJSON, _ := json.Marshal(identity)
	if err := os.WriteFile(identityPath, identityJSON, 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := run(append(args, "-identity", identityPath), &out); err != nil {
		t.Fatal(err)
	}
	var keyed struct {
		Prepared catalogkit.Prepared `json:"prepared"`
		CacheKey string              `json:"cache_key"`
	}
	if err := json.Unmarshal(out.Bytes(), &keyed); err != nil {
		t.Fatal(err)
	}
	expected, err := catalogkit.DecisionCacheKey(keyed.Prepared, identity)
	if err != nil || keyed.CacheKey == "" || keyed.CacheKey != expected {
		t.Fatalf("CLI reuse identity wrong: %s %v", &out, err)
	}
	if err := os.WriteFile(identityPath, []byte(`{"provider":"fixture"}`), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := run(append(args, "-identity", identityPath), &out); err == nil || out.Len() != 0 {
		t.Fatalf("incomplete identity produced output: %s %v", &out, err)
	}
	if err := os.WriteFile(definitionPath, []byte("Changed rule with the same version label."), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	var diagnostics bytes.Buffer
	if code := execute(args, &out, &diagnostics); code != 1 || out.Len() != 0 || !strings.Contains(diagnostics.String(), "definition digest mismatch") {
		t.Fatalf("stale CLI definition accepted: %d %s %s", code, &out, &diagnostics)
	}
}

func TestDiscoveryProfileAndDoctor(t *testing.T) {
	d := t.TempDir()
	spec := filepath.Join(d, "spec.json")
	profile := filepath.Join(d, "profile.json")
	bundle := filepath.Join(d, "bundle.json")
	for path, body := range map[string]string{
		spec:    `{"version":1,"layers":[{"name":"base","nodes":[{"slug":"review","title":"Review","source":"base.cue","stable_key":"review","children":[{"slug":"evidence","title":"Evidence","source":"base.cue","stable_key":"evidence"},{"slug":"secret","title":"Classified","source":"base.cue","stable_key":"secret","hidden":true}]}]},{"name":"team","requires":["base"]},{"name":"other","nodes":[{"slug":"unselected","source":"other.cue","stable_key":"other"}]}]}`,
		profile: `{"name":"project","roots":["team"]}`,
	} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := run([]string{"resolve", "-spec", spec, "-profile", profile}, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "unselected") {
		t.Fatal("profile leaked unrelated layer")
	}
	if err := os.WriteFile(bundle, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"doctor", "-bundle", bundle},
		{"doctor", "-spec", spec, "-profile", profile},
	} {
		out.Reset()
		if err := run(args, &out); err != nil {
			t.Fatal(err)
		}
		var report struct {
			Status string `json:"status"`
			Layers int    `json:"layers_checked"`
			Nodes  int    `json:"nodes_checked"`
		}
		if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Status != "ok" || report.Layers != 2 || report.Nodes != 3 {
			t.Fatalf("bad doctor report: %s", &out)
		}
	}
	for _, tc := range []struct {
		query  string
		hidden bool
		want   int
	}{
		{"evidence", false, 1}, {"classified", false, 0}, {"classified", true, 1}, {"nomatchtoken", false, 0},
	} {
		out.Reset()
		args := []string{"search", "-bundle", bundle, "-text", tc.query}
		if tc.hidden {
			args = append(args, "-include-hidden")
		}
		if err := run(args, &out); err != nil {
			t.Fatal(err)
		}
		var result struct {
			Query   string            `json:"query"`
			Matched int               `json:"matched"`
			Results []json.RawMessage `json:"results"`
		}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Query != tc.query || result.Matched != tc.want || len(result.Results) != tc.want || result.Results == nil {
			t.Fatalf("unexpected search: %s", &out)
		}
	}
	out.Reset()
	if err := run([]string{"explain", "-bundle", bundle, "-id", "review/evidence"}, &out); err != nil || !strings.Contains(out.String(), `"policy_digest"`) || !strings.Contains(out.String(), `"sources"`) {
		t.Fatalf("incomplete explain: %s %v", &out, err)
	}
}

func TestExitCodesHelpAndRefusals(t *testing.T) {
	d := t.TempDir()
	invalid := filepath.Join(d, "invalid.json")
	if err := os.WriteFile(invalid, []byte(`{"version":1,"layers":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"help"}, 0}, {[]string{"--help"}, 0}, {[]string{"resolve", "-h"}, 0},
		{nil, 2}, {[]string{"unknown"}, 2}, {[]string{"resolve"}, 2}, {[]string{"resolve", "-typo"}, 2},
		{[]string{"doctor", "-spec", invalid}, 1}, {[]string{"resolve", "-spec", invalid}, 1},
		{[]string{"resolve", "-spec", filepath.Join(d, "missing.json")}, 2},
	} {
		var out, diagnostics bytes.Buffer
		if got := execute(tc.args, &out, &diagnostics); got != tc.code {
			t.Fatalf("%v got %d want %d: %s", tc.args, got, tc.code, &diagnostics)
		}
		if tc.code != 0 && out.Len() != 0 {
			t.Fatalf("partial output on failure: %s", &out)
		}
		if tc.code == 0 && !strings.Contains(out.String(), "Usage:") {
			t.Fatal("help missing")
		}
	}
}
