package main

// tests_select_embed_test.go — TQ-21 (docs/plans/test-intelligence.md,
// O658): a file a //go:embed directive embeds maps to no symbol of its own,
// so an edit to dist/agents/skills/<name>/SKILL.md selected no test, not
// even TestPluginTemplateDigestPinned, which pins those bytes. The changed
// embedded file now counts its embedding variable, and every function that
// reads that variable, as changed.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// embedFixture is a module shaped like fathom's own skills embed: a
// skillsdist package embedding fathom-plan, a renderSkill that reads it, a
// pluginTemplateDigest over that, and the test that pins the digest.
func embedFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/embedfix\n\ngo 1.22\n")
	write("dist/agents/skills/dist.go", "package skillsdist\n\nimport \"embed\"\n\n// FS is the shipped skills.\n//\n//go:embed fathom-plan\nvar FS embed.FS\n\n// Names lists them.\nvar Names = []string{\"fathom-plan\"}\n")
	write("dist/agents/skills/fathom-plan/SKILL.md", "# plan\n")
	write("dist/agents/skills/README.md", "not embedded\n")
	write("cmd/fathom/skillsinstall.go", "package main\n\nimport skillsdist \"example.com/embedfix/dist/agents/skills\"\n\nfunc renderSkill(name string) ([]byte, error) {\n\treturn skillsdist.FS.ReadFile(name + \"/SKILL.md\")\n}\n\nfunc skillNames() []string { return skillsdist.Names }\n")
	write("cmd/fathom/pluginrender.go", "package main\n\nfunc pluginTemplateDigest() string {\n\tb, _ := renderSkill(\"fathom-plan\")\n\treturn string(b)\n}\n\nfunc unrelated() int { return len(skillNames()) }\n\nfunc main() { _ = pluginTemplateDigest(); _ = unrelated() }\n")
	write("cmd/fathom/pluginrender_test.go", "package main\n\nimport \"testing\"\n\nfunc TestPluginTemplateDigestPinned(t *testing.T) { _ = pluginTemplateDigest() }\n\nfunc TestUnrelated(t *testing.T) { _ = unrelated() }\n")
	return root
}

func TestTestsSelectGoEmbedEditSelectsEmbeddingTests(t *testing.T) {
	root := embedFixture(t)
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	var ing bytes.Buffer
	if code := run([]string{"ingest", "--service", "embed-svc", "--path", root, "--store", storePath}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}
	old := testsSelectRoot
	testsSelectRoot = root
	t.Cleanup(func() { testsSelectRoot = old })
	setEconomicsLedger(t, filepath.Join(t.TempDir(), "econ.jsonl"))

	selectFor := func(changed ...string) string {
		t.Helper()
		var out, errBuf bytes.Buffer
		args := []string{"--since", "HEAD~1", "--service", "embed-svc", "--store", storePath, "--safe"}
		if code := runTestsSelect(args, fakeGitRunner(changed), &out, &errBuf); code != 0 {
			t.Fatalf("exit %d: %s", code, errBuf.String())
		}
		return out.String()
	}

	text := selectFor("dist/agents/skills/fathom-plan/SKILL.md")
	if !strings.Contains(text, "TestPluginTemplateDigestPinned") {
		t.Fatalf("an edit to an embedded skill must select the test that reads it:\n%s", text)
	}
	if strings.Contains(text, "TestUnrelated") {
		t.Errorf("a test reaching only another var of the embedding package was selected:\n%s", text)
	}

	// A file beside the embed that no pattern names is not embedded.
	if text := selectFor("dist/agents/skills/README.md"); strings.Contains(text, "TestPluginTemplateDigestPinned") {
		t.Errorf("a file no //go:embed pattern names selected an embedding test:\n%s", text)
	}
}

// TestEmbedChangedSymbols pins the mapping itself: the embedding variable
// and every function reading it, for a file under an embedded directory;
// nothing for a file no pattern names or a Go file.
func TestEmbedChangedSymbols(t *testing.T) {
	root := embedFixture(t)
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	var ing bytes.Buffer
	if code := run([]string{"ingest", "--service", "embed-svc", "--path", root, "--store", storePath}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}
	nodes := listNodesForTest(t, storePath, "embed-svc")

	got := embedChangedSymbols(root, nodes, []string{"dist/agents/skills/fathom-plan/SKILL.md"})
	joined := strings.Join(got, " ")
	wants := []string{".renderSkill"}
	for _, n := range nodes {
		if n.Name == "FS" { // the variable itself, where the index carries it
			wants = append(wants, ".FS")
		}
	}
	for _, want := range wants {
		if !strings.Contains(joined, want) {
			t.Errorf("embedChangedSymbols = %v, want a key ending %s", got, want)
		}
	}
	for _, not := range []string{".Names", ".skillNames", ".pluginTemplateDigest"} {
		if strings.Contains(joined, not+" ") || strings.HasSuffix(joined, not) {
			t.Errorf("embedChangedSymbols = %v, must not name %s", got, not)
		}
	}
	if got := embedChangedSymbols(root, nodes, []string{"dist/agents/skills/README.md", "cmd/fathom/pluginrender.go"}); len(got) != 0 {
		t.Errorf("no embedded file changed, got %v", got)
	}
}

// listNodesForTest reads every symbol of service from the store at path.
func listNodesForTest(t *testing.T, path, service string) []core.SymbolNode {
	t.Helper()
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	c := client.New(connector.NewLocalConnector(), st)
	t.Cleanup(func() { c.Close() })
	nodes, err := c.ListAllSymbols(context.Background(), service)
	if err != nil {
		t.Fatal(err)
	}
	return nodes
}
