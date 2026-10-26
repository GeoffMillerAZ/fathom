package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Owner-authored scope pins are independent of the implementation's loader and
// of the generator. Changing an input requires review of this acceptance oracle.
const frozenV2Corpus = "da99d1c025764c445967dd8d71c56e63992108feaa682d1ba81685155c0688bd"
const frozenV2GoalScope = "41988f2f526415763d64faa7193e477f1370c56bfd2a0e269427dbbf0c004403"
const frozenV1Corpus = "f85612aa226eb4418d2fa11b4023ea53f99979934bd56780dd005d1b8c796893"

func TestDocumentV2FrozenScopeAcceptance(t *testing.T) {
	legacy, err := loadDocumentCorpus("testdata/document-corpus")
	if err != nil || legacy.Digest != frozenV1Corpus {
		t.Fatalf("legacy corpus changed: %s %v", legacy.Digest, err)
	}
	c, err := loadDocumentCorpus("testdata/document-corpus-v2")
	if err != nil || c.Digest != frozenV2Corpus {
		t.Fatalf("v2 corpus changed: %s %v", c.Digest, err)
	}
	scope := mustReadDocumentTest(t, "testdata/document-scopes/v2-length-goals.json")
	if hashBytes(scope) != frozenV2GoalScope {
		t.Fatal("independently frozen goal scope changed")
	}
	args := documentArgs(documentTestStudy(t, c), "testdata/document-corpus-v2", "freeform-brief-executive-r1")
	args = append(args, "--goal-scope", "testdata/document-scopes/v2-length-goals.json", "--goal-scope-digest", frozenV2GoalScope)
	var out, errOut bytes.Buffer
	if code := runDocumentWith(context.Background(), args, &out, &errOut, documentDeps{}); code != 0 {
		t.Fatalf("valid v2 prepare: %d %s", code, errOut.String())
	}
	var p map[string]any
	if err := json.Unmarshal(out.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p["version"] != "prose-lab.document-prepared/2" || p["goal_scope_digest"] != frozenV2GoalScope || p["corpus_digest"] != frozenV2Corpus || p["semantic_quality"] != "unassessed" || p["count_rule"] != "markdown-whitespace-fields/v1" {
		t.Fatalf("missing scope or honest assessment binding: %s", out.String())
	}
	goals, ok := p["length_goals"].([]any)
	if !ok || len(goals) != 6 {
		t.Fatalf("expected all six scope goals: %v", p["length_goals"])
	}
	advisory, required := 0, 0
	seen := map[string]bool{}
	for _, value := range goals {
		g, ok := value.(map[string]any)
		if !ok {
			t.Fatal("invalid fixed goal metadata")
		}
		id, _ := g["document_id"].(string)
		if seen[id] || id == "" || g["criterion_id"] != "g-length-"+id || len(g["reason_digest"].(string)) != 64 {
			t.Fatal("duplicate or incomplete goal metadata")
		}
		seen[id] = true
		switch g["class"] {
		case "advisory":
			advisory++
		case "required":
			required++
			if id != "technical-design" || g["min_words"] != float64(1200) || g["max_words"] != float64(1600) {
				t.Fatal("long capability floor was weakened")
			}
		default:
			t.Fatal("unknown length class")
		}
	}
	if advisory != 5 || required != 1 {
		t.Fatal("unexpected policy classes")
	}
	witness, ok := p["long_artifact"].(map[string]any)
	if !ok || witness["document_id"] != "technical-design" || witness["criterion_id"] != "g-length-technical-design" || witness["min_words"] != float64(1200) {
		t.Fatal("preparation omitted the required long witness")
	}
	for _, text := range []string{"fictional", "This is an invented draft", "Independent reviewer", "The headless proof must exercise"} {
		if strings.Contains(out.String(), text) {
			t.Fatal("prepare disclosed raw document or scope reason")
		}
	}
}

func TestDocumentV2ChangedScopeInvalidatesPreparation(t *testing.T) {
	c, err := loadDocumentCorpus("testdata/document-corpus-v2")
	if err != nil {
		t.Fatal(err)
	}
	base := documentArgs(documentTestStudy(t, c), "testdata/document-corpus-v2", "freeform-brief-executive-r1")
	oldArgs := append(append([]string{}, base...), "--goal-scope", "testdata/document-scopes/v2-length-goals.json", "--goal-scope-digest", frozenV2GoalScope)
	prepare := func(args []string) map[string]any {
		t.Helper()
		var out, errOut bytes.Buffer
		if code := runDocument(context.Background(), args, &out, &errOut); code != 0 {
			t.Fatalf("valid scope prepare: %d %s", code, errOut.String())
		}
		var p map[string]any
		if err := json.Unmarshal(out.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	old := prepare(oldArgs)
	b := append(mustReadDocumentTest(t, "testdata/document-scopes/v2-length-goals.json"), ' ')
	file := filepath.Join(t.TempDir(), "scope.json")
	if err := os.WriteFile(file, b, 0600); err != nil {
		t.Fatal(err)
	}
	newArgs := append(append([]string{}, base...), "--goal-scope", file, "--goal-scope-digest", hashBytes(b))
	updated := prepare(newArgs) // Same decoded values, independently approved new raw bytes.
	if old["study_digest"] == updated["study_digest"] || old["prepared_digest"] == updated["prepared_digest"] {
		t.Fatal("raw goal scope did not bind both identities")
	}
	root := t.TempDir()
	journal, artifacts := filepath.Join(root, "journal"), filepath.Join(root, "artifacts")
	capture := append(append([]string{}, newArgs...), "--live", "--journal", journal, "--artifacts", artifacts, "--prepared-digest", old["prepared_digest"].(string))
	capture[0] = "document-capture"
	spy := &documentSpy{}
	if code := runDocumentWith(context.Background(), capture, &bytes.Buffer{}, &bytes.Buffer{}, documentDeps{generation: spy, review: spy}); code != 2 || spy.calls != 0 {
		t.Fatal("new valid scope reused old preparation")
	}
	for _, path := range []string{journal, artifacts} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("stale preparation caused effects")
		}
	}
}

func scopeAcceptanceRefuses(t *testing.T, args []string) {
	t.Helper()
	// Exercise intake without a deliberately bad prepared digest first. Otherwise
	// a late pin mismatch could mask a missing scope validator.
	prepare := append([]string{}, args...)
	prepare[0] = "document-prepare"
	if code := runDocumentWith(context.Background(), prepare, &bytes.Buffer{}, &bytes.Buffer{}, documentDeps{}); code == 0 {
		t.Fatal("invalid scope was accepted by prepare")
	}
	root := t.TempDir()
	journal, artifacts := filepath.Join(root, "journal"), filepath.Join(root, "artifacts")
	args = append(append([]string{}, args...), "--live", "--journal", journal, "--artifacts", artifacts, "--prepared-digest", strings.Repeat("a", 64))
	args[0] = "document-capture"
	spy := &documentSpy{}
	var out, errOut bytes.Buffer
	if code := runDocumentWith(context.Background(), args, &out, &errOut, documentDeps{generation: spy, review: spy}); code == 0 || spy.calls != 0 {
		t.Fatalf("scope refusal failed: code=%d calls=%d out=%s", code, spy.calls, out.String())
	}
	for _, path := range []string{journal, artifacts} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("unapproved scope created %s before refusal: %v", path, err)
		}
	}
}

func TestDocumentV2ScopeRefusalsHaveNoEffects(t *testing.T) {
	c, err := loadDocumentCorpus("testdata/document-corpus-v2")
	if err != nil {
		t.Fatal(err)
	}
	base := documentArgs(documentTestStudy(t, c), "testdata/document-corpus-v2", "freeform-brief-executive-r1")
	path := "testdata/document-scopes/v2-length-goals.json"
	for name, flags := range map[string][]string{
		"missing-both":  nil,
		"missing-pin":   {"--goal-scope", path},
		"missing-file":  {"--goal-scope-digest", frozenV2GoalScope},
		"changed-pin":   {"--goal-scope", path, "--goal-scope-digest", strings.Repeat("a", 64)},
		"malformed-pin": {"--goal-scope", path, "--goal-scope-digest", "not-a-digest"},
	} {
		t.Run(name, func(t *testing.T) { scopeAcceptanceRefuses(t, append(append([]string{}, base...), flags...)) })
	}
	source := mustReadDocumentTest(t, path)
	for name, mutate := range map[string]func([]byte) []byte{
		"unknown-field": func(b []byte) []byte { return bytes.Replace(b, []byte("{\n"), []byte("{\n  \"unknown\": true,\n"), 1) },
		"duplicate-version": func(b []byte) []byte {
			return bytes.Replace(b, []byte("{\n"), []byte("{\n  \"version\": \"prose-lab.goal-scope/1\",\n"), 1)
		},
		"null-documents": func(b []byte) []byte {
			var v map[string]any
			_ = json.Unmarshal(b, &v)
			v["documents"] = nil
			return jsonBytes(v)
		},
		"missing-long-witness": func(b []byte) []byte {
			var v map[string]any
			_ = json.Unmarshal(b, &v)
			delete(v, "long_artifact")
			return jsonBytes(v)
		},
		"shortened-long-witness": func(b []byte) []byte {
			var v map[string]any
			_ = json.Unmarshal(b, &v)
			v["long_artifact"].(map[string]any)["min_words"] = 1199
			return jsonBytes(v)
		},
		"semantic-goal-omitted": func(b []byte) []byte {
			var v map[string]any
			_ = json.Unmarshal(b, &v)
			v["documents"].([]any)[0].(map[string]any)["required_semantic_goal_ids"] = []string{"g-fidelity"}
			return jsonBytes(v)
		},
	} {
		t.Run(name, func(t *testing.T) {
			b := mutate(append([]byte{}, source...))
			file := filepath.Join(t.TempDir(), "scope.json")
			if err := os.WriteFile(file, b, 0600); err != nil {
				t.Fatal(err)
			}
			// A valid raw hash cannot turn a malformed or weaker scope into an
			// accepted preparation. This also exercises same-byte decoding.
			args := append(append([]string{}, base...), "--goal-scope", file, "--goal-scope-digest", hashBytes(b))
			scopeAcceptanceRefuses(t, args)
		})
	}
	t.Run("symlink-scope", func(t *testing.T) {
		abs, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(t.TempDir(), "scope.json")
		if err := os.Symlink(abs, link); err != nil {
			t.Skipf("platform cannot create symlink: %v", err)
		}
		scopeAcceptanceRefuses(t, append(append([]string{}, base...), "--goal-scope", link, "--goal-scope-digest", frozenV2GoalScope))
	})
}

func TestDocumentV1RejectsExplicitScopeFlags(t *testing.T) {
	c, err := loadDocumentCorpus("testdata/document-corpus")
	if err != nil {
		t.Fatal(err)
	}
	base := documentArgs(documentTestStudy(t, c), "testdata/document-corpus", "freeform-brief-executive-r1")
	for _, flag := range []string{"--goal-scope", "--goal-scope-digest"} {
		t.Run(flag, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := runDocument(context.Background(), append(append([]string{}, base...), flag, ""), &out, &errOut); code == 0 {
				t.Fatal("v1 silently ignored an explicit scope flag")
			}
		})
	}
}

func TestDocumentV2CoordinatedLongDowngradeRefuses(t *testing.T) {
	// A prospective replacement fixture AND matching scope must still retain a
	// real long capability witness. This is stronger than a stale corpus hash.
	dir := t.TempDir()
	ids := []string{"architecture-decision", "draft-service-agreement", "freeform-brief", "incident-summary", "operational-runbook", "technical-design"}
	for _, file := range append([]string{"manifest.json"}, func() []string {
		var out []string
		for _, id := range ids {
			out = append(out, id+".json")
		}
		return out
	}()...) {
		b := mustReadDocumentTest(t, filepath.Join("testdata/document-corpus-v2", file))
		if file == "technical-design.json" {
			var d map[string]any
			if err := json.Unmarshal(b, &d); err != nil {
				t.Fatal(err)
			}
			d["length_band"] = "short"
			g := d["length_goal"].(map[string]any)
			g["class"], g["min_words"], g["max_words"] = "advisory", 100, 200
			b = jsonBytes(d)
		}
		if err := os.WriteFile(filepath.Join(dir, file), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Recompute the corpus identity independently from exact bytes. The refusal
	// must survive a coordinated definition/hash update, not rely on old hashes.
	raw := append([]byte{}, mustReadDocumentTest(t, filepath.Join(dir, "manifest.json"))...)
	for _, id := range ids {
		raw = append(raw, []byte(id+"\x00")...)
		raw = append(raw, mustReadDocumentTest(t, filepath.Join(dir, id+".json"))...)
	}
	var scope map[string]any
	if err := json.Unmarshal(mustReadDocumentTest(t, "testdata/document-scopes/v2-length-goals.json"), &scope); err != nil {
		t.Fatal(err)
	}
	scope["corpus_digest"] = hashBytes(raw)
	for _, item := range scope["documents"].([]any) {
		d := item.(map[string]any)
		if d["document_id"] == "technical-design" {
			d["length_class"], d["min_words"], d["max_words"] = "advisory", 100, 200
		}
	}
	scopeBytes := jsonBytes(scope)
	file := filepath.Join(t.TempDir(), "scope.json")
	if err := os.WriteFile(file, scopeBytes, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := loadDocumentCorpus("testdata/document-corpus-v2")
	if err != nil {
		t.Fatal(err)
	}
	args := documentArgs(documentTestStudy(t, c), dir, "technical-design-executive-r1")
	args = append(args, "--goal-scope", file, "--goal-scope-digest", hashBytes(scopeBytes))
	scopeAcceptanceRefuses(t, args)
}
